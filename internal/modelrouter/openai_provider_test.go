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
	"time"

	"github.com/mimoneko/mimoneko/internal/contextengine"
	"github.com/mimoneko/mimoneko/internal/prefix"
)

func TestOpenAIProviderMissingAPIKey(t *testing.T) {
	p := NewOpenAICompatibleProvider("test", "http://localhost:11434/v1", "NONEXISTENT_KEY_12345", []string{"model-a"}, nil)

	_, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "model-a",
		Bundle: makeTestBundle(),
	})
	if err == nil {
		t.Fatal("Complete() succeeded, want API key missing error")
	}

	// Error should mention the env var name but NOT the key value
	if !strings.Contains(err.Error(), "NONEXISTENT_KEY_12345") {
		t.Errorf("error should mention env var name, got %q", err.Error())
	}
	// Make sure no actual API key value leaked
	if strings.Contains(err.Error(), "Bearer ") {
		t.Errorf("error should not contain Authorization header, got %q", err.Error())
	}
}

func TestOpenAIProviderSendsCorrectRequest(t *testing.T) {
	var receivedBody openAIRequest
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}

		receivedAuth = r.Header.Get("Authorization")

		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}

		// Return valid response
		resp := openAIResponse{
			ID:    "resp-123",
			Model: "test-model",
			Choices: []openAIChoice{
				{Message: openAIChoiceMessage{Role: "assistant", Content: "test response"}},
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

	os.Setenv("TEST_API_KEY_ROUTER", "sk-test-key-12345")
	defer os.Unsetenv("TEST_API_KEY_ROUTER")

	p := NewOpenAICompatibleProvider("test", server.URL, "TEST_API_KEY_ROUTER", []string{"test-model"}, server.Client())

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
	if resp.RequestID != "resp-123" {
		t.Errorf("RequestID = %q, want resp-123", resp.RequestID)
	}

	// Verify request was correct
	if receivedBody.Model != "test-model" {
		t.Errorf("request model = %q, want test-model", receivedBody.Model)
	}
	if receivedBody.MaxTokens != 2048 {
		t.Errorf("request max_tokens = %d, want 2048", receivedBody.MaxTokens)
	}
	if receivedBody.Temperature != 0.5 {
		t.Errorf("request temperature = %f, want 0.5", receivedBody.Temperature)
	}
	if len(receivedBody.Messages) != 4 {
		t.Errorf("request messages count = %d, want 4", len(receivedBody.Messages))
	}

	// Verify Authorization header
	if !strings.HasPrefix(receivedAuth, "Bearer ") {
		t.Errorf("Authorization header = %q, want Bearer prefix", receivedAuth)
	}
}

func TestOpenAIProviderParsesUsageWithCachedTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			ID:    "resp-456",
			Model: "test-model",
			Choices: []openAIChoice{
				{Message: openAIChoiceMessage{Role: "assistant", Content: "ok"}},
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

	os.Setenv("TEST_API_KEY_CACHED", "sk-test")
	defer os.Unsetenv("TEST_API_KEY_CACHED")

	p := NewOpenAICompatibleProvider("test", server.URL, "TEST_API_KEY_CACHED", []string{"test-model"}, server.Client())

	resp, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		Bundle: makeTestBundle(),
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", resp.Usage.OutputTokens)
	}
	if resp.Usage.CachedTokens != 40 {
		t.Errorf("CachedTokens = %d, want 40", resp.Usage.CachedTokens)
	}
	if resp.Usage.Estimated != false {
		t.Errorf("Estimated = %v, want false (provider returned cached_tokens)", resp.Usage.Estimated)
	}
}

func TestOpenAIProviderUsageEstimatedWhenNoCachedTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			ID:    "resp-789",
			Model: "test-model",
			Choices: []openAIChoice{
				{Message: openAIChoiceMessage{Role: "assistant", Content: "ok"}},
			},
			Usage: openAIUsage{
				PromptTokens:     80,
				CompletionTokens: 30,
				TotalTokens:      110,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("TEST_API_KEY_NOCACHE", "sk-test")
	defer os.Unsetenv("TEST_API_KEY_NOCACHE")

	p := NewOpenAICompatibleProvider("test", server.URL, "TEST_API_KEY_NOCACHE", []string{"test-model"}, server.Client())

	resp, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		Bundle: makeTestBundle(),
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Usage.Estimated != true {
		t.Errorf("Estimated = %v, want true (no cached_tokens from provider)", resp.Usage.Estimated)
	}
	if resp.Usage.CachedTokens != 0 {
		t.Errorf("CachedTokens = %d, want 0", resp.Usage.CachedTokens)
	}
}

func TestOpenAIProviderUsageEstimatedWhenNoUsageReturned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			ID:    "resp-none",
			Model: "test-model",
			Choices: []openAIChoice{
				{Message: openAIChoiceMessage{Role: "assistant", Content: "ok"}},
			},
			Usage: openAIUsage{}, // zero usage
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("TEST_API_KEY_NOUSAGE", "sk-test")
	defer os.Unsetenv("TEST_API_KEY_NOUSAGE")

	bundle := makeTestBundle()
	bundle.Report.TotalTokens = 20

	p := NewOpenAICompatibleProvider("test", server.URL, "TEST_API_KEY_NOUSAGE", []string{"test-model"}, server.Client())

	resp, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		Bundle: bundle,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Usage.InputTokens != 20 {
		t.Errorf("InputTokens = %d, want 20 (estimated from bundle)", resp.Usage.InputTokens)
	}
	if resp.Usage.Estimated != true {
		t.Errorf("Estimated = %v, want true", resp.Usage.Estimated)
	}
}

func TestOpenAIProviderHandlesNon200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error": "internal server error"}`)
	}))
	defer server.Close()

	os.Setenv("TEST_API_KEY_500", "sk-test")
	defer os.Unsetenv("TEST_API_KEY_500")

	p := NewOpenAICompatibleProvider("test", server.URL, "TEST_API_KEY_500", []string{"test-model"}, server.Client())

	_, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		Bundle: makeTestBundle(),
	})
	if err == nil {
		t.Fatal("Complete() succeeded, want error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500, got %q", err.Error())
	}
}

func TestOpenAIProviderSupportsModel(t *testing.T) {
	p := NewOpenAICompatibleProvider("test", "http://localhost", "", []string{"model-a", "model-b"}, nil)

	if !p.Supports("model-a") {
		t.Error("Supports(model-a) = false, want true")
	}
	if !p.Supports("model-b") {
		t.Error("Supports(model-b) = false, want true")
	}
	if p.Supports("model-c") {
		t.Error("Supports(model-c) = true, want false")
	}
}

func TestOpenAIProviderAPIKeyStatus(t *testing.T) {
	p := NewOpenAICompatibleProvider("test", "http://localhost", "TEST_STATUS_KEY_XYZ", []string{"m1"}, nil)

	// Not set
	if p.APIKeyStatus() != "missing" {
		t.Errorf("APIKeyStatus() = %q, want missing", p.APIKeyStatus())
	}

	os.Setenv("TEST_STATUS_KEY_XYZ", "some-value")
	defer os.Unsetenv("TEST_STATUS_KEY_XYZ")

	if p.APIKeyStatus() != "configured" {
		t.Errorf("APIKeyStatus() = %q, want configured", p.APIKeyStatus())
	}
}

func TestOpenAIProviderNoAPIKeyEnv(t *testing.T) {
	p := NewOpenAICompatibleProvider("test", "http://localhost", "", []string{"m1"}, nil)

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

func TestOpenAIProviderContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response - use a short sleep instead of blocking on context
		// to avoid httptest.Server Close() blocking on active connections.
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	os.Setenv("TEST_API_KEY_CANCEL", "sk-test")
	defer os.Unsetenv("TEST_API_KEY_CANCEL")

	// Use a custom http client with short timeout
	client := &http.Client{Timeout: 50 * time.Millisecond}

	p := NewOpenAICompatibleProvider("test", server.URL, "TEST_API_KEY_CANCEL", []string{"test-model"}, client)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.Complete(ctx, CompletionRequest{
		Model:  "test-model",
		Bundle: makeTestBundle(),
	})
	if err == nil {
		t.Fatal("Complete() succeeded, want context cancellation error")
	}
}

func TestOpenAIProviderName(t *testing.T) {
	p := NewOpenAICompatibleProvider("my-provider", "http://localhost", "", []string{"m1"}, nil)
	if p.Name() != "my-provider" {
		t.Errorf("Name() = %q, want my-provider", p.Name())
	}
}

func TestParseUsageWithCachedTokens(t *testing.T) {
	apiUsage := openAIUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		PromptTokensDetails: &promptTokensDetails{CachedTokens: 30},
	}

	usage := parseUsage(apiUsage, 0)

	if usage.CachedTokens != 30 {
		t.Errorf("CachedTokens = %d, want 30", usage.CachedTokens)
	}
	if usage.Estimated != false {
		t.Errorf("Estimated = %v, want false", usage.Estimated)
	}
}

func TestParseUsageWithoutCachedTokens(t *testing.T) {
	apiUsage := openAIUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	usage := parseUsage(apiUsage, 0)

	if usage.CachedTokens != 0 {
		t.Errorf("CachedTokens = %d, want 0", usage.CachedTokens)
	}
	if usage.Estimated != true {
		t.Errorf("Estimated = %v, want true", usage.Estimated)
	}
}

func TestParseUsageWithZeroUsage(t *testing.T) {
	apiUsage := openAIUsage{}

	usage := parseUsage(apiUsage, 200) // bundle total

	if usage.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200 (from bundle)", usage.InputTokens)
	}
	if usage.Estimated != true {
		t.Errorf("Estimated = %v, want true", usage.Estimated)
	}
}

func TestOpenAIProviderDefaultMaxTokens(t *testing.T) {
	var receivedBody openAIRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		resp := openAIResponse{
			ID: "resp-default", Model: "m",
			Choices: []openAIChoice{{Message: openAIChoiceMessage{Content: "ok"}}},
			Usage: openAIUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("TEST_API_KEY_DEFAULT", "sk-test")
	defer os.Unsetenv("TEST_API_KEY_DEFAULT")

	p := NewOpenAICompatibleProvider("test", server.URL, "TEST_API_KEY_DEFAULT", []string{"m"}, server.Client())

	_, err := p.Complete(context.Background(), CompletionRequest{
		Model:           "m",
		Bundle:          makeTestBundle(),
		MaxOutputTokens: 0, // should default to 4096
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if receivedBody.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096 (default)", receivedBody.MaxTokens)
	}
}

func TestOpenAIProviderConvertsBundleToMessages(t *testing.T) {
	var receivedBody openAIRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		resp := openAIResponse{
			ID: "resp-bundle", Model: "m",
			Choices: []openAIChoice{{Message: openAIChoiceMessage{Content: "ok"}}},
			Usage: openAIUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("TEST_API_KEY_BUNDLE", "sk-test")
	defer os.Unsetenv("TEST_API_KEY_BUNDLE")

	p := NewOpenAICompatibleProvider("test", server.URL, "TEST_API_KEY_BUNDLE", []string{"m"}, server.Client())

	bundle := contextengine.Bundle{
		Layers: []contextengine.ContextLayer{
			{Name: "immutable_prefix", Bytes: []byte("system"), Tokens: 5},
			{Name: "current_input", Bytes: []byte("input"), Tokens: 2},
		},
		CacheFingerprint: prefix.Fingerprint{SHA256: "test", Version: 1},
		Report:           contextengine.ContextReport{TotalTokens: 7},
	}

	_, err := p.Complete(context.Background(), CompletionRequest{
		Model:  "m",
		Bundle: bundle,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if len(receivedBody.Messages) != 2 {
		t.Fatalf("messages count = %d, want 2", len(receivedBody.Messages))
	}
	if receivedBody.Messages[0].Role != RoleSystem {
		t.Errorf("message 0 role = %q, want system", receivedBody.Messages[0].Role)
	}
	if receivedBody.Messages[1].Role != RoleUser {
		t.Errorf("message 1 role = %q, want user", receivedBody.Messages[1].Role)
	}
}
