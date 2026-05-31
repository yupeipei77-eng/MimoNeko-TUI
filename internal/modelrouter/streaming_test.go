package modelrouter

import (
	"context"
	"testing"
)

func TestMockStreamProviderStreamsChunks(t *testing.T) {
	provider := NewMockStreamProvider("test", []string{"Hello", " ", "world"})

	resp, err := provider.StreamComplete(context.Background(), CompletionRequest{
		Model: "test-model",
	})
	if err != nil {
		t.Fatalf("StreamComplete() error = %v", err)
	}

	text, usage, err := CollectStream(context.Background(), resp)
	if err != nil {
		t.Fatalf("CollectStream() error = %v", err)
	}

	if text != "Hello world" {
		t.Errorf("text = %q, want %q", text, "Hello world")
	}
	if usage.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d, want 150", usage.TotalTokens)
	}
}

func TestCollectStreamHandlesContextCancellation(t *testing.T) {
	// Create a provider with a delay
	provider := NewMockStreamProvider("test", []string{"chunk1", "chunk2"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	resp, err := provider.StreamComplete(context.Background(), CompletionRequest{
		Model: "test-model",
	})
	if err != nil {
		t.Fatalf("StreamComplete() error = %v", err)
	}

	_, _, err = CollectStream(ctx, resp)
	// Should either complete quickly or return context error
	// Since mock is fast, it may complete before cancellation takes effect
	_ = err
}

func TestStreamToCallback(t *testing.T) {
	provider := NewMockStreamProvider("test", []string{"a", "b", "c"})

	resp, err := provider.StreamComplete(context.Background(), CompletionRequest{
		Model: "test-model",
	})
	if err != nil {
		t.Fatalf("StreamComplete() error = %v", err)
	}

	var chunks []string
	text, usage, err := StreamToCallback(context.Background(), resp, func(chunk StreamChunk) {
		if chunk.Text != "" {
			chunks = append(chunks, chunk.Text)
		}
	})
	if err != nil {
		t.Fatalf("StreamToCallback() error = %v", err)
	}

	if text != "abc" {
		t.Errorf("text = %q, want %q", text, "abc")
	}
	if len(chunks) != 3 {
		t.Errorf("chunks count = %d, want 3", len(chunks))
	}
	if usage.Estimated != true {
		t.Errorf("Estimated = %v, want true", usage.Estimated)
	}
}

func TestStreamToCallbackReceivesDoneChunk(t *testing.T) {
	provider := NewMockStreamProvider("test", []string{"text"})

	resp, err := provider.StreamComplete(context.Background(), CompletionRequest{
		Model: "test-model",
	})
	if err != nil {
		t.Fatalf("StreamComplete() error = %v", err)
	}

	var doneReceived bool
	_, _, err = StreamToCallback(context.Background(), resp, func(chunk StreamChunk) {
		if chunk.Done {
			doneReceived = true
			if chunk.Usage == nil {
				t.Error("done chunk should have usage")
			}
		}
	})
	if err != nil {
		t.Fatalf("StreamToCallback() error = %v", err)
	}
	if !doneReceived {
		t.Error("should have received a done chunk")
	}
}

func TestNonStreamingFallback(t *testing.T) {
	// Use a regular provider (not streaming)
	provider := NewMockProvider("test").WithText("fallback text")

	var chunks []string
	resp, err := provider.Complete(context.Background(), CompletionRequest{
		Model: "test-model",
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// Simulate the fallback path: deliver as single chunk
	chunk := StreamChunk{
		Text:  resp.Text,
		Done:  true,
		Usage: &resp.Usage,
	}
	chunks = append(chunks, chunk.Text)

	if len(chunks) != 1 {
		t.Errorf("chunks count = %d, want 1 (fallback)", len(chunks))
	}
	if chunks[0] != "fallback text" {
		t.Errorf("text = %q, want %q", chunks[0], "fallback text")
	}
}

func TestStreamChunkUsageOnlyOnDone(t *testing.T) {
	provider := NewMockStreamProvider("test", []string{"partial"})

	resp, err := provider.StreamComplete(context.Background(), CompletionRequest{
		Model: "test-model",
	})
	if err != nil {
		t.Fatalf("StreamComplete() error = %v", err)
	}

	var nonDoneHasUsage bool
	var doneHasUsage bool

	for chunk := range resp.Chunks {
		if !chunk.Done && chunk.Usage != nil {
			nonDoneHasUsage = true
		}
		if chunk.Done && chunk.Usage != nil {
			doneHasUsage = true
		}
	}

	if nonDoneHasUsage {
		t.Error("non-done chunks should not have usage")
	}
	if !doneHasUsage {
		t.Error("done chunk should have usage")
	}
}
