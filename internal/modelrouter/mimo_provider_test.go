package modelrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMimoProviderMissingAPIKey(t *testing.T) {
	p := NewMimoProvider("test", "http://localhost:11434/v1", "NONEXISTENT_MIMO_KEY_12345", []string{"model-a"}, nil)

	_, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "model-a",
		Bundle: makeTestBundle(),
	})
	if err == nil {
		t.Fatal("Complete() succeeded, want API key missing error")
	}

	// Error should mention the env var name but NOT the key value
	if !strings.Contains(err.Error(), "NONEXISTENT_MIMO_KEY_12345") {
		t.Errorf("error should mention env var name, got %q", err.Error())
	}
}

func TestMimoProviderSendsCorrectRequest(t *testing.T) {
	var receivedBody mimoRequest
	var receivedAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}

		receivedAPIKey = r.Header.Get("api-key")

		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}

		// Return valid response in Mimo format (delta.content)
		resp := mimoResponse{
			ID:    "resp-mimo-123",
			Model: "test-model",
			Choices: []mimoChoice{
				{Delta: &mimoDelta{Content: "test response"}},
			},
			Usage: openAIUsage{
				PromptTokens:     50,
				CompletionTokens: 20,
				TotalTokens:      70,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("TEST_MIMO_API_KEY", "sk-mimo-test-key-12345")
	defer os.Unsetenv("TEST_MIMO_API_KEY")

	p := NewMimoProvider("test", server.URL, "TEST_MIMO_API_KEY", []string{"test-model"}, server.Client())

	resp, err := p.Complete(context.Background(), CompletionRequest{
		Model:           "test-model",
		Bundle:          makeTestBundle(),
		MaxOutputTokens: 2048,
		Temperature:     0.5,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Text != "test response" {
		t.Errorf("Text = %q, want %q", resp.Text, "test response")
	}
	if resp.Provider != "test" {
		t.Errorf("Provider = %q, want test", resp.Provider)
	}
	if resp.RequestID != "resp-mimo-123" {
		t.Errorf("RequestID = %q, want resp-mimo-123", resp.RequestID)
	}

	// Verify request was correct
	if receivedBody.Model != "test-model" {
		t.Errorf("request model = %q, want test-model", receivedBody.Model)
	}
	if receivedBody.MaxCompletionTokens != 2048 {
		t.Errorf("request max_completion_tokens = %d, want 2048", receivedBody.MaxCompletionTokens)
	}
	if receivedBody.Temperature != 0.5 {
		t.Errorf("request temperature = %f, want 0.5", receivedBody.Temperature)
	}
	if len(receivedBody.Messages) != 4 {
		t.Errorf("request messages count = %d, want 4", len(receivedBody.Messages))
	}

	// Verify api-key header (not Authorization: Bearer)
	if receivedAPIKey != "sk-mimo-test-key-12345" {
		t.Errorf("api-key header = %q, want sk-mimo-test-key-12345", receivedAPIKey)
	}
}

func TestMimoProviderParsesMessageContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response with message.content (OpenAI-compatible format)
		resp := mimoResponse{
			ID:    "resp-msg",
			Model: "test-model",
			Choices: []mimoChoice{
				{Message: &mimoMessage{Role: "assistant", Content: "message content"}},
			},
			Usage: openAIUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("TEST_MIMO_MSG_KEY", "sk-test")
	defer os.Unsetenv("TEST_MIMO_MSG_KEY")

	p := NewMimoProvider("test", server.URL, "TEST_MIMO_MSG_KEY", []string{"test-model"}, server.Client())

	resp, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		Bundle: makeTestBundle(),
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Text != "message content" {
		t.Errorf("Text = %q, want %q", resp.Text, "message content")
	}
}

func TestMimoProviderParsesDeltaContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response with delta.content (Mimo-specific format)
		resp := mimoResponse{
			ID:    "resp-delta",
			Model: "test-model",
			Choices: []mimoChoice{
				{Delta: &mimoDelta{Content: "delta content"}},
			},
			Usage: openAIUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("TEST_MIMO_DELTA_KEY", "sk-test")
	defer os.Unsetenv("TEST_MIMO_DELTA_KEY")

	p := NewMimoProvider("test", server.URL, "TEST_MIMO_DELTA_KEY", []string{"test-model"}, server.Client())

	resp, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		Bundle: makeTestBundle(),
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Text != "delta content" {
		t.Errorf("Text = %q, want %q", resp.Text, "delta content")
	}
}

func TestMimoProviderHandlesMimoErrorFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"code":"invalid_request","message":"Invalid model specified"}`)
	}))
	defer server.Close()

	os.Setenv("TEST_MIMO_ERR_KEY", "sk-test")
	defer os.Unsetenv("TEST_MIMO_ERR_KEY")

	p := NewMimoProvider("test", server.URL, "TEST_MIMO_ERR_KEY", []string{"test-model"}, server.Client())

	_, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		Bundle: makeTestBundle(),
	})
	if err == nil {
		t.Fatal("Complete() succeeded, want error")
	}

	// Should contain Mimo error format details
	if !strings.Contains(err.Error(), "Invalid model specified") {
		t.Errorf("error should contain message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "invalid_request") {
		t.Errorf("error should contain code, got %q", err.Error())
	}
}

func TestMimoProviderHandlesOpenAIErrorFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"message":"Internal server error","type":"server_error"}}`)
	}))
	defer server.Close()

	os.Setenv("TEST_MIMO_OAI_ERR", "sk-test")
	defer os.Unsetenv("TEST_MIMO_OAI_ERR")

	p := NewMimoProvider("test", server.URL, "TEST_MIMO_OAI_ERR", []string{"test-model"}, server.Client())

	_, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		Bundle: makeTestBundle(),
	})
	if err == nil {
		t.Fatal("Complete() succeeded, want error")
	}

	if !strings.Contains(err.Error(), "Internal server error") {
		t.Errorf("error should contain message, got %q", err.Error())
	}
}

func TestMimoProviderSupportsModel(t *testing.T) {
	p := NewMimoProvider("test", "http://localhost", "", []string{"mimo-v2.5-pro", "mimo-v2.5"}, nil)

	if !p.Supports("mimo-v2.5-pro") {
		t.Error("Supports(mimo-v2.5-pro) = false, want true")
	}
	if !p.Supports("mimo-v2.5") {
		t.Error("Supports(mimo-v2.5) = false, want true")
	}
	if p.Supports("unknown-model") {
		t.Error("Supports(unknown-model) = true, want false")
	}
}

func TestMimoProviderAPIKeyStatus(t *testing.T) {
	p := NewMimoProvider("test", "http://localhost", "TEST_MIMO_STATUS_KEY", []string{"m1"}, nil)

	// Not set
	if p.APIKeyStatus() != "missing" {
		t.Errorf("APIKeyStatus() = %q, want missing", p.APIKeyStatus())
	}

	os.Setenv("TEST_MIMO_STATUS_KEY", "some-value")
	defer os.Unsetenv("TEST_MIMO_STATUS_KEY")

	if p.APIKeyStatus() != "configured" {
		t.Errorf("APIKeyStatus() = %q, want configured", p.APIKeyStatus())
	}
}

func TestMimoProviderNoAPIKeyEnv(t *testing.T) {
	p := NewMimoProvider("test", "http://localhost", "", []string{"m1"}, nil)

	_, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "m1",
		Bundle: makeTestBundle(),
	})
	if err == nil {
		t.Fatal("Complete() succeeded, want error for empty api_key_env")
	}
	if !strings.Contains(err.Error(), "no api_key_env") {
		t.Errorf("error should mention no api_key_env, got %q", err.Error())
	}
}

func TestMimoProviderName(t *testing.T) {
	p := NewMimoProvider("mimo", "http://localhost", "", []string{"m1"}, nil)
	if p.Name() != "mimo" {
		t.Errorf("Name() = %q, want mimo", p.Name())
	}
}

func TestMimoProviderSupportsStreaming(t *testing.T) {
	p := NewMimoProvider("test", "http://localhost", "", []string{"m1"}, nil)
	if !p.SupportsStreaming() {
		t.Error("SupportsStreaming() = false, want true")
	}
}

func TestMimoProviderStreamCompleteMissingAPIKey(t *testing.T) {
	p := NewMimoProvider("test", "http://localhost:11434/v1", "NONEXISTENT_STREAM_KEY", []string{"model-a"}, nil)

	_, err := p.StreamComplete(context.Background(), CompletionRequest{
		Model:  "model-a",
		Bundle: makeTestBundle(),
	})
	if err == nil {
		t.Fatal("StreamComplete() succeeded, want API key missing error")
	}

	if !strings.Contains(err.Error(), "NONEXISTENT_STREAM_KEY") {
		t.Errorf("error should mention env var name, got %q", err.Error())
	}
}

func TestMimoProviderDefaultMaxTokens(t *testing.T) {
	var receivedBody mimoRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		resp := mimoResponse{
			ID: "resp-default", Model: "m",
			Choices: []mimoChoice{{Delta: &mimoDelta{Content: "ok"}}},
			Usage:   openAIUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("TEST_MIMO_DEFAULT_KEY", "sk-test")
	defer os.Unsetenv("TEST_MIMO_DEFAULT_KEY")

	p := NewMimoProvider("test", server.URL, "TEST_MIMO_DEFAULT_KEY", []string{"m"}, server.Client())

	_, err := p.Complete(context.Background(), CompletionRequest{
		Model:           "m",
		Bundle:          makeTestBundle(),
		MaxOutputTokens: 0, // should default to 4096
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if receivedBody.MaxCompletionTokens != 4096 {
		t.Errorf("MaxCompletionTokens = %d, want 4096 (default)", receivedBody.MaxCompletionTokens)
	}
}

func TestMimoProviderParsesUsageWithCachedTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mimoResponse{
			ID:    "resp-cached",
			Model: "test-model",
			Choices: []mimoChoice{
				{Delta: &mimoDelta{Content: "ok"}},
			},
			Usage: openAIUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				PromptTokensDetails: &promptTokensDetails{
					CachedTokens: 40,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("TEST_MIMO_CACHED_KEY", "sk-test")
	defer os.Unsetenv("TEST_MIMO_CACHED_KEY")

	p := NewMimoProvider("test", server.URL, "TEST_MIMO_CACHED_KEY", []string{"test-model"}, server.Client())

	resp, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		Bundle: makeTestBundle(),
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Usage.CachedTokens != 40 {
		t.Errorf("CachedTokens = %d, want 40", resp.Usage.CachedTokens)
	}
	if resp.Usage.Estimated != false {
		t.Errorf("Estimated = %v, want false", resp.Usage.Estimated)
	}
}
