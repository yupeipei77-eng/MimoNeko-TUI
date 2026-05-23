package modelrouter

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMockProviderReturnsConfiguredText(t *testing.T) {
	p := NewMockProvider("test").WithText("hello world")

	resp, err := p.Complete(context.Background(), CompletionRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if resp.Text != "hello world" {
		t.Errorf("Text = %q, want %q", resp.Text, "hello world")
	}
	if resp.Provider != "test" {
		t.Errorf("Provider = %q, want %q", resp.Provider, "test")
	}
}

func TestMockProviderReturnsConfiguredUsage(t *testing.T) {
	usage := Usage{InputTokens: 200, OutputTokens: 100, TotalTokens: 300, CachedTokens: 50, Estimated: false}
	p := NewMockProvider("test").WithUsage(usage)

	resp, err := p.Complete(context.Background(), CompletionRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if resp.Usage.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200", resp.Usage.InputTokens)
	}
	if resp.Usage.CachedTokens != 50 {
		t.Errorf("CachedTokens = %d, want 50", resp.Usage.CachedTokens)
	}
	if resp.Usage.Estimated != false {
		t.Errorf("Estimated = %v, want false", resp.Usage.Estimated)
	}
}

func TestMockProviderReturnsError(t *testing.T) {
	p := NewMockProvider("test").WithError(errors.New("provider unavailable"))

	_, err := p.Complete(context.Background(), CompletionRequest{Model: "test-model"})
	if err == nil {
		t.Fatal("Complete() succeeded, want error")
	}
	if err.Error() != "provider unavailable" {
		t.Errorf("error = %q, want %q", err.Error(), "provider unavailable")
	}
}

func TestMockProviderSimulatesDelay(t *testing.T) {
	p := NewMockProvider("test").WithDelay(50 * time.Millisecond)

	start := time.Now()
	_, err := p.Complete(context.Background(), CompletionRequest{Model: "test-model"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("elapsed = %v, want at least ~50ms", elapsed)
	}
}

func TestMockProviderRespectsContextCancellation(t *testing.T) {
	p := NewMockProvider("test").WithDelay(5 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.Complete(ctx, CompletionRequest{Model: "test-model"})
	if err == nil {
		t.Fatal("Complete() succeeded, want context deadline error")
	}
}

func TestMockProviderSupportsModel(t *testing.T) {
	p := NewMockProvider("test").WithSupportedModels([]string{"model-a", "model-b"})

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

func TestMockProviderSupportsAllModelsByDefault(t *testing.T) {
	p := NewMockProvider("test")

	if !p.Supports("any-model") {
		t.Error("Supports(any-model) = false, want true (default)")
	}
}

func TestMockProviderTracksCallCount(t *testing.T) {
	p := NewMockProvider("test")

	if p.CallCount() != 0 {
		t.Errorf("CallCount() = %d, want 0", p.CallCount())
	}

	_, _ = p.Complete(context.Background(), CompletionRequest{Model: "m1"})
	_, _ = p.Complete(context.Background(), CompletionRequest{Model: "m2"})

	if p.CallCount() != 2 {
		t.Errorf("CallCount() = %d, want 2", p.CallCount())
	}
}

func TestMockProviderTracksLastRequest(t *testing.T) {
	p := NewMockProvider("test")

	if p.LastRequest() != nil {
		t.Error("LastRequest() should be nil initially")
	}

	req := CompletionRequest{Model: "test-model", MaxOutputTokens: 4096}
	_, _ = p.Complete(context.Background(), req)

	last := p.LastRequest()
	if last == nil {
		t.Fatal("LastRequest() is nil after call")
	}
	if last.Model != "test-model" {
		t.Errorf("LastRequest().Model = %q, want %q", last.Model, "test-model")
	}
}

func TestMockProviderName(t *testing.T) {
	p := NewMockProvider("my-provider")
	if p.Name() != "my-provider" {
		t.Errorf("Name() = %q, want %q", p.Name(), "my-provider")
	}
}
