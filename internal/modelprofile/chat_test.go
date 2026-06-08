package modelprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
)

func TestModelChatSuccess(t *testing.T) {
	root := setupChatRoot(t)
	t.Setenv("CHAT_API_KEY", "sk-chat-ok")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
		}
		fmt.Fprint(w, `{"model":"chat-model","choices":[{"message":{"content":"你好，我在。"}}],"usage":{"prompt_tokens":12,"completion_tokens":5,"total_tokens":17}}`)
	}))
	defer server.Close()
	saveChatModel(t, root, server.URL)

	result, err := Chat(context.Background(), root, ChatOptions{Provider: "chat", Model: "chat-model", Prompt: "你好"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Response != "你好，我在。" || result.Provider != "chat" || result.Model != "chat-model" {
		t.Fatalf("result = %+v, want sanitized chat response", result)
	}
	if result.PromptTokens != 12 || result.CompletionTokens != 5 || result.TotalTokens != 17 {
		t.Fatalf("usage = %+v, want parsed usage", result)
	}
}

func TestModelChatParsesCachedTokens(t *testing.T) {
	root := setupChatRoot(t)
	t.Setenv("CHAT_API_KEY", "sk-chat-cache")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"model":"chat-model","choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":100,"completion_tokens":5,"total_tokens":105,"prompt_tokens_details":{"cached_tokens":40}}}`)
	}))
	defer server.Close()
	saveChatModel(t, root, server.URL)

	result, err := Chat(context.Background(), root, ChatOptions{Provider: "chat", Model: "chat-model", Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result.CachedTokens != 40 || !result.CachedTokensKnown {
		t.Fatalf("result = %+v, want cached token details", result)
	}
}

func TestModelChatOpenAICompatibleIgnoresMimoNativeCacheTokens(t *testing.T) {
	root := setupChatRoot(t)
	t.Setenv("CHAT_API_KEY", "sk-chat-cache")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"model":"chat-model","choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1000,"completion_tokens":5,"total_tokens":1005,"prompt_cache_hit_tokens":900,"prompt_cache_miss_tokens":100,"prompt_tokens_details":{"cached_tokens":40}}}`)
	}))
	defer server.Close()
	saveChatModel(t, root, server.URL)

	result, err := Chat(context.Background(), root, ChatOptions{Provider: "chat", Model: "chat-model", Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result.NativeCacheKnown {
		t.Fatalf("result = %+v, want native cache disabled for non-MIMO", result)
	}
	if result.CachedTokens != 40 || !result.CachedTokensKnown {
		t.Fatalf("result = %+v, want fallback cached token details", result)
	}
}

func TestModelChatDoesNotLeakAPIKey(t *testing.T) {
	root := setupChatRoot(t)
	secret := "sk-chat-secret"
	t.Setenv("CHAT_API_KEY", secret)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"model":"chat-model","choices":[{"message":{"content":"token %s"}}]}`, secret)
	}))
	defer server.Close()
	saveChatModel(t, root, server.URL)

	result, err := Chat(context.Background(), root, ChatOptions{Provider: "chat", Model: "chat-model", Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Response, secret) {
		t.Fatalf("chat response leaked API key: %q", result.Response)
	}
}

func TestModelChatMimoUsesAPIKeyHeaderAndDeltaContent(t *testing.T) {
	root := setupChatRoot(t)
	t.Setenv("MIMO_CHAT_API_KEY", "sk-mimo-chat")

	var receivedAPIKey string
	var receivedAuthorization string
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("api-key")
		receivedAuthorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		fmt.Fprint(w, `{"model":"mimo-v2.5-pro","choices":[{"delta":{"content":"mimo ok"}}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`)
	}))
	defer server.Close()
	saveMimoChatModel(t, root, server.URL)

	result, err := Chat(context.Background(), root, ChatOptions{Provider: "mimo", Model: "mimo-v2.5-pro", Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if receivedAPIKey != "sk-mimo-chat" {
		t.Fatalf("api-key header = %q, want configured key", receivedAPIKey)
	}
	if receivedAuthorization != "" {
		t.Fatalf("Authorization header = %q, want empty for mimo", receivedAuthorization)
	}
	if _, ok := receivedBody["max_completion_tokens"]; !ok {
		t.Fatalf("request body missing max_completion_tokens: %#v", receivedBody)
	}
	if _, ok := receivedBody["max_tokens"]; ok {
		t.Fatalf("request body had OpenAI max_tokens for mimo: %#v", receivedBody)
	}
	if result.Response != "mimo ok" || result.TotalTokens != 5 {
		t.Fatalf("result = %+v, want delta response and usage", result)
	}
}

func TestModelChatMimoParsesNativeCacheHitMissTokens(t *testing.T) {
	root := setupChatRoot(t)
	t.Setenv("MIMO_CHAT_API_KEY", "sk-mimo-native-cache")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"model":"mimo-v2.5-pro","choices":[{"delta":{"content":"ok"}}],"usage":{"prompt_tokens":1000,"completion_tokens":5,"total_tokens":1005,"prompt_cache_hit_tokens":900,"prompt_cache_miss_tokens":100,"prompt_tokens_details":{"cached_tokens":12}}}`)
	}))
	defer server.Close()
	saveMimoChatModel(t, root, server.URL)

	result, err := Chat(context.Background(), root, ChatOptions{Provider: "mimo", Model: "mimo-v2.5-pro", Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.NativeCacheKnown || !result.CachedTokensKnown {
		t.Fatalf("result = %+v, want native cache known", result)
	}
	if result.CacheHitTokens != 900 || result.CacheMissTokens != 100 {
		t.Fatalf("native cache = hit %d miss %d, want 900/100", result.CacheHitTokens, result.CacheMissTokens)
	}
	if result.CachedTokens != 900 {
		t.Fatalf("CachedTokens = %d, want native hit tokens 900", result.CachedTokens)
	}
}

func TestModelChatStreamMimoUsesAPIKeyHeaderAndMaxCompletionTokens(t *testing.T) {
	root := setupChatRoot(t)
	t.Setenv("MIMO_CHAT_API_KEY", "sk-mimo-stream")

	var receivedAPIKey string
	var receivedAuthorization string
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("api-key")
		receivedAuthorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"model\":\"mimo-v2.5-pro\",\"choices\":[{\"delta\":{\"reasoning_content\":\"thinking \"}}]}\n\n")
		fmt.Fprint(w, "data: {\"model\":\"mimo-v2.5-pro\",\"choices\":[{\"delta\":{\"content\":\"done\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":2,\"total_tokens\":6,\"prompt_tokens_details\":{\"cached_tokens\":1}}}\n\n")
	}))
	defer server.Close()
	saveMimoChatModel(t, root, server.URL)

	stream, err := ChatStream(root, ChatOptions{Provider: "mimo", Model: "mimo-v2.5-pro", Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	var chunks []string
	result, err := stream(context.Background(), func(chunk ChatStreamChunk) {
		chunks = append(chunks, chunk.ReasoningText+chunk.Text)
	})
	if err != nil {
		t.Fatal(err)
	}
	if receivedAPIKey != "sk-mimo-stream" {
		t.Fatalf("api-key header = %q, want configured key", receivedAPIKey)
	}
	if receivedAuthorization != "" {
		t.Fatalf("Authorization header = %q, want empty for mimo", receivedAuthorization)
	}
	if _, ok := receivedBody["max_completion_tokens"]; !ok {
		t.Fatalf("request body missing max_completion_tokens: %#v", receivedBody)
	}
	if _, ok := receivedBody["max_tokens"]; ok {
		t.Fatalf("request body had OpenAI max_tokens for mimo: %#v", receivedBody)
	}
	if got := strings.Join(chunks, ""); got != "thinking done" {
		t.Fatalf("streamed chunks = %q, want reasoning and text", got)
	}
	if result.TotalTokens != 6 || !result.CachedTokensKnown || result.CachedTokens != 1 {
		t.Fatalf("result = %+v, want parsed stream usage", result)
	}
}

func TestModelChatRejectsPlaceholderAPIKey(t *testing.T) {
	root := setupChatRoot(t)
	t.Setenv("MIMO_CHAT_API_KEY", "your-api-key-here")
	saveMimoChatModel(t, root, "http://127.0.0.1:1")

	_, err := Chat(context.Background(), root, ChatOptions{Provider: "mimo", Model: "mimo-v2.5-pro", Prompt: "hello"})
	if err == nil {
		t.Fatal("Chat succeeded, want placeholder API key error")
	}
	if !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("error = %q, want placeholder hint", err.Error())
	}
}

func TestSanitizeTextPreservesAPIKeyEnvVarName(t *testing.T) {
	errText := "API key not found in environment variable MIMO_API_KEY"
	got := SanitizeText(errText)
	if got != errText {
		t.Fatalf("SanitizeText() = %q, want env var name preserved", got)
	}
}

func TestSanitizeTextRedactsAssignedAPIKeyValue(t *testing.T) {
	got := SanitizeText("using API_KEY=sk-secret-value")
	if strings.Contains(got, "sk-secret-value") {
		t.Fatalf("SanitizeText leaked key value: %q", got)
	}
	if !strings.Contains(got, "API_KEY=<redacted>") {
		t.Fatalf("SanitizeText() = %q, want assigned API_KEY redacted", got)
	}
}

func setupChatRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := config.InitDetailed(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func saveChatModel(t *testing.T, root, baseURL string) {
	t.Helper()
	models := config.ModelsConfig{
		Providers: []config.ProviderConfig{
			{
				Name:      "chat",
				Type:      "openai-compatible",
				BaseURL:   baseURL,
				APIKeyEnv: "CHAT_API_KEY",
				Models: []config.ModelConfig{
					{Name: "chat-model", Purpose: "coding", MaxOutputTokens: 4096},
				},
			},
		},
		Routing: config.RoutingConfig{DefaultModel: "chat-model"},
	}
	if err := Save(root, models); err != nil {
		t.Fatal(err)
	}
}

func saveMimoChatModel(t *testing.T, root, baseURL string) {
	t.Helper()
	models := config.ModelsConfig{
		Providers: []config.ProviderConfig{
			{
				Name:      "mimo",
				Type:      "mimo",
				BaseURL:   baseURL,
				APIKeyEnv: "MIMO_CHAT_API_KEY",
				Models: []config.ModelConfig{
					{Name: "mimo-v2.5-pro", Purpose: "coding", MaxOutputTokens: 4096},
				},
			},
		},
		Routing: config.RoutingConfig{DefaultModel: "mimo-v2.5-pro"},
	}
	if err := Save(root, models); err != nil {
		t.Fatal(err)
	}
}
