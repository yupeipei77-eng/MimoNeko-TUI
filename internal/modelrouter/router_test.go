package modelrouter

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/reasonforge/reasonforge/internal/cache"
	"github.com/reasonforge/reasonforge/internal/contextengine"
	"github.com/reasonforge/reasonforge/internal/prefix"
)

// stubCacheRegistry is a test CacheRegistry that records observations.
type stubCacheRegistry struct {
	observations []cache.Observation
}

func (s *stubCacheRegistry) Lookup(ctx context.Context, fingerprint prefix.Fingerprint) (cache.Entry, bool, error) {
	return cache.Entry{}, false, nil
}

func (s *stubCacheRegistry) Record(ctx context.Context, observation cache.Observation) error {
	s.observations = append(s.observations, observation)
	return nil
}

func makeTestBundle() contextengine.Bundle {
	return contextengine.Bundle{
		CurrentInput: []byte("test input"),
		Layers: []contextengine.ContextLayer{
			{Name: "immutable_prefix", Bytes: []byte("prefix"), Tokens: 10},
			{Name: "conversation_log", Bytes: []byte("conv"), Tokens: 5},
			{Name: "scratchpad", Bytes: []byte("scratch"), Tokens: 3},
			{Name: "current_input", Bytes: []byte("test input"), Tokens: 2},
		},
		CacheFingerprint: prefix.Fingerprint{SHA256: "abc123", Version: 1},
		Report: contextengine.ContextReport{
			PrefixTokens:       10,
			ConversationTokens: 5,
			ScratchpadTokens:   3,
			CurrentInputTokens: 2,
			TotalTokens:        20,
		},
	}
}

func TestRouterCompletesWithSingleProvider(t *testing.T) {
	provider := NewMockProvider("test-provider").WithText("hello")
	reg := &stubCacheRegistry{}

	router := NewDefaultModelRouter(
		map[string]Provider{"test-provider": provider},
		[]FallbackEntry{{Provider: "test-provider", Model: "test-model"}},
		"test-model",
		reg,
	)

	resp, err := router.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		Bundle: makeTestBundle(),
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if resp.Text != "hello" {
		t.Errorf("Text = %q, want %q", resp.Text, "hello")
	}
	if resp.Provider != "test-provider" {
		t.Errorf("Provider = %q, want %q", resp.Provider, "test-provider")
	}
}

func TestRouterFallsBackOnError(t *testing.T) {
	primary := NewMockProvider("primary").WithError(errors.New("unavailable"))
	secondary := NewMockProvider("secondary").WithText("fallback response")
	reg := &stubCacheRegistry{}

	router := NewDefaultModelRouter(
		map[string]Provider{"primary": primary, "secondary": secondary},
		[]FallbackEntry{
			{Provider: "primary", Model: "model-a"},
			{Provider: "secondary", Model: "model-b"},
		},
		"model-a",
		reg,
	)

	resp, err := router.Complete(context.Background(), CompletionRequest{
		Model:  "model-a",
		Bundle: makeTestBundle(),
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if resp.Text != "fallback response" {
		t.Errorf("Text = %q, want fallback response", resp.Text)
	}
	if resp.Provider != "secondary" {
		t.Errorf("Provider = %q, want secondary", resp.Provider)
	}
}

func TestRouterReturnsFallbackErrorWhenAllFail(t *testing.T) {
	primary := NewMockProvider("primary").WithError(errors.New("unavailable"))
	secondary := NewMockProvider("secondary").WithError(errors.New("also down"))
	reg := &stubCacheRegistry{}

	router := NewDefaultModelRouter(
		map[string]Provider{"primary": primary, "secondary": secondary},
		[]FallbackEntry{
			{Provider: "primary", Model: "model-a"},
			{Provider: "secondary", Model: "model-b"},
		},
		"model-a",
		reg,
	)

	_, err := router.Complete(context.Background(), CompletionRequest{
		Model:  "model-a",
		Bundle: makeTestBundle(),
	})
	if err == nil {
		t.Fatal("Complete() succeeded, want FallbackError")
	}

	var fbErr *FallbackError
	if !errors.As(err, &fbErr) {
		t.Fatalf("error type = %T, want *FallbackError", err)
	}

	if len(fbErr.Attempts) != 2 {
		t.Fatalf("Attempts count = %d, want 2", len(fbErr.Attempts))
	}

	// Verify error message contains provider/model info but no keys
	msg := fbErr.Error()
	if !strings.Contains(msg, "primary/model-a") {
		t.Errorf("error message should contain primary/model-a, got %q", msg)
	}
	if !strings.Contains(msg, "secondary/model-b") {
		t.Errorf("error message should contain secondary/model-b, got %q", msg)
	}
}

func TestRouterUsesDefaultModelWhenModelEmpty(t *testing.T) {
	provider := NewMockProvider("test").WithText("ok")
	reg := &stubCacheRegistry{}

	router := NewDefaultModelRouter(
		map[string]Provider{"test": provider},
		[]FallbackEntry{{Provider: "test", Model: "default-model"}},
		"default-model",
		reg,
	)

	resp, err := router.Complete(context.Background(), CompletionRequest{
		Model:  "", // empty, should use default
		Bundle: makeTestBundle(),
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if resp.Model != "default-model" {
		t.Errorf("Model = %q, want default-model", resp.Model)
	}
}

func TestRouterRecordsUsageToCacheRegistry(t *testing.T) {
	usage := Usage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150, CachedTokens: 0, Estimated: true}
	provider := NewMockProvider("test").WithUsage(usage)
	reg := &stubCacheRegistry{}

	router := NewDefaultModelRouter(
		map[string]Provider{"test": provider},
		[]FallbackEntry{{Provider: "test", Model: "test-model"}},
		"test-model",
		reg,
	)

	_, err := router.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		Bundle: makeTestBundle(),
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if len(reg.observations) != 1 {
		t.Fatalf("observations count = %d, want 1", len(reg.observations))
	}

	obs := reg.observations[0]
	if obs.Provider != "test" {
		t.Errorf("observation Provider = %q, want test", obs.Provider)
	}
	if obs.Model != "test-model" {
		t.Errorf("observation Model = %q, want test-model", obs.Model)
	}
	if obs.InputTokens != 100 {
		t.Errorf("observation InputTokens = %d, want 100", obs.InputTokens)
	}
	if obs.Fingerprint.SHA256 != "abc123" {
		t.Errorf("observation Fingerprint.SHA256 = %q, want abc123", obs.Fingerprint.SHA256)
	}
	if obs.PrefixTokens != 10 {
		t.Errorf("observation PrefixTokens = %d, want 10", obs.PrefixTokens)
	}
	if obs.ConversationTokens != 5 {
		t.Errorf("observation ConversationTokens = %d, want 5", obs.ConversationTokens)
	}
	if obs.ScratchpadTokens != 3 {
		t.Errorf("observation ScratchpadTokens = %d, want 3", obs.ScratchpadTokens)
	}
	if obs.CurrentInputTokens != 2 {
		t.Errorf("observation CurrentInputTokens = %d, want 2", obs.CurrentInputTokens)
	}
}

func TestRouterSkipsProviderNotFoundInChain(t *testing.T) {
	provider := NewMockProvider("available").WithText("ok")
	reg := &stubCacheRegistry{}

	router := NewDefaultModelRouter(
		map[string]Provider{"available": provider},
		[]FallbackEntry{
			{Provider: "missing", Model: "m1"},
			{Provider: "available", Model: "m2"},
		},
		"m2",
		reg,
	)

	resp, err := router.Complete(context.Background(), CompletionRequest{
		Model:  "m2",
		Bundle: makeTestBundle(),
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if resp.Provider != "available" {
		t.Errorf("Provider = %q, want available", resp.Provider)
	}
}
