package model

import (
	"testing"
	"time"
)

func TestRoleConstants(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		expected string
	}{
		{"system", RoleSystem, "system"},
		{"user", RoleUser, "user"},
		{"assistant", RoleAssistant, "assistant"},
		{"tool", RoleTool, "tool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.role) != tt.expected {
				t.Errorf("Role %s = %q, want %q", tt.name, string(tt.role), tt.expected)
			}
		})
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    RoleUser,
		Content: "Hello, world!",
	}

	if msg.Role != RoleUser {
		t.Errorf("Message.Role = %v, want %v", msg.Role, RoleUser)
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("Message.Content = %q, want %q", msg.Content, "Hello, world!")
	}
}

func TestRouteRequest(t *testing.T) {
	req := RouteRequest{
		TaskID:        "task-123",
		Capability:    "coding",
		TokenBudget:   4096,
		RequiresCache: true,
	}

	if req.TaskID != "task-123" {
		t.Errorf("RouteRequest.TaskID = %q, want %q", req.TaskID, "task-123")
	}
	if req.Capability != "coding" {
		t.Errorf("RouteRequest.Capability = %q, want %q", req.Capability, "coding")
	}
	if req.TokenBudget != 4096 {
		t.Errorf("RouteRequest.TokenBudget = %d, want %d", req.TokenBudget, 4096)
	}
	if !req.RequiresCache {
		t.Error("RouteRequest.RequiresCache = false, want true")
	}
}

func TestRoute(t *testing.T) {
	route := Route{
		Provider: "openai",
		Model:    "gpt-4",
		BaseURL:  "https://api.openai.com/v1",
	}

	if route.Provider != "openai" {
		t.Errorf("Route.Provider = %q, want %q", route.Provider, "openai")
	}
	if route.Model != "gpt-4" {
		t.Errorf("Route.Model = %q, want %q", route.Model, "gpt-4")
	}
	if route.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("Route.BaseURL = %q, want %q", route.BaseURL, "https://api.openai.com/v1")
	}
}

func TestCompletionRequest(t *testing.T) {
	req := CompletionRequest{
		TaskID: "task-456",
		Route: Route{
			Provider: "mimo",
			Model:    "mimo-v1",
		},
		ImmutablePrefix: []byte("prefix"),
		VolatileMessages: []Message{
			{Role: RoleUser, Content: "Hello"},
			{Role: RoleAssistant, Content: "Hi there!"},
		},
		MaxOutputTokens: 2048,
	}

	if req.TaskID != "task-456" {
		t.Errorf("CompletionRequest.TaskID = %q, want %q", req.TaskID, "task-456")
	}
	if len(req.VolatileMessages) != 2 {
		t.Errorf("len(CompletionRequest.VolatileMessages) = %d, want 2", len(req.VolatileMessages))
	}
	if req.MaxOutputTokens != 2048 {
		t.Errorf("CompletionRequest.MaxOutputTokens = %d, want %d", req.MaxOutputTokens, 2048)
	}
}

func TestCacheUsage(t *testing.T) {
	usage := CacheUsage{
		InputTokens:  1000,
		CachedTokens: 500,
	}

	if usage.InputTokens != 1000 {
		t.Errorf("CacheUsage.InputTokens = %d, want %d", usage.InputTokens, 1000)
	}
	if usage.CachedTokens != 500 {
		t.Errorf("CacheUsage.CachedTokens = %d, want %d", usage.CachedTokens, 500)
	}
}

func TestCompletionResponse(t *testing.T) {
	now := time.Now()
	resp := CompletionResponse{
		ID:    "resp-789",
		Model: "mimo-v1",
		Text:  "Generated text",
		CacheUsage: CacheUsage{
			InputTokens:  800,
			CachedTokens: 400,
		},
		CompletedAt: now,
	}

	if resp.ID != "resp-789" {
		t.Errorf("CompletionResponse.ID = %q, want %q", resp.ID, "resp-789")
	}
	if resp.Model != "mimo-v1" {
		t.Errorf("CompletionResponse.Model = %q, want %q", resp.Model, "mimo-v1")
	}
	if resp.Text != "Generated text" {
		t.Errorf("CompletionResponse.Text = %q, want %q", resp.Text, "Generated text")
	}
	if !resp.CompletedAt.Equal(now) {
		t.Errorf("CompletionResponse.CompletedAt = %v, want %v", resp.CompletedAt, now)
	}
}
