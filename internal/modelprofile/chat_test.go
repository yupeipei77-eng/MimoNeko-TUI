package modelprofile

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/reasonforge/reasonforge/internal/config"
)

func TestModelChatSuccess(t *testing.T) {
	root := setupChatRoot(t)
	t.Setenv("CHAT_API_KEY", "sk-chat-ok")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
		}
		fmt.Fprint(w, `{"model":"chat-model","choices":[{"message":{"content":"你好，我在。"}}]}`)
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
