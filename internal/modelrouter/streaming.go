package modelrouter

import (
	"context"
)

// StreamChunk represents a single chunk of streamed model output.
type StreamChunk struct {
	// Text is the incremental text delta.
	Text string

	// ReasoningText is the incremental reasoning/thinking content delta.
	// This is separate from Text so consumers can render it differently
	// (e.g. dimmed, in a collapsible section, or omitted entirely).
	ReasoningText string

	// Done indicates this is the final chunk.
	Done bool

	// Usage is populated only on the final chunk when available.
	Usage *Usage

	// RequestID is the provider's request identifier.
	RequestID string
}

// StreamResponse is the result of initiating a streaming completion.
type StreamResponse struct {
	// Chunks delivers streamed output. The channel is closed when streaming
	// completes or an error occurs. Errors are delivered via ErrCh.
	Chunks <-chan StreamChunk

	// ErrCh delivers any streaming error. Closed when streaming completes.
	ErrCh <-chan error
}

// StreamingProvider extends Provider with streaming completion support.
// Providers that do not support streaming should return false from
// SupportsStreaming; the router will fall back to Complete().
type StreamingProvider interface {
	Provider

	// StreamComplete sends a streaming completion request.
	// The caller must drain Chunks and ErrCh to avoid goroutine leaks.
	StreamComplete(ctx context.Context, req CompletionRequest) (StreamResponse, error)

	// SupportsStreaming returns true if this provider supports streaming.
	SupportsStreaming() bool
}

// StreamingModelRouter extends ModelRouter with streaming support.
type StreamingModelRouter interface {
	ModelRouter

	// StreamComplete resolves the provider/model and calls StreamComplete.
	// Falls back to non-streaming Complete if the provider does not support streaming.
	StreamComplete(ctx context.Context, req CompletionRequest) (StreamResponse, error)
}

// CollectStream drains a StreamResponse into a complete text and usage.
// It blocks until the stream completes or an error occurs.
// This is the non-streaming fallback path.
func CollectStream(ctx context.Context, resp StreamResponse) (string, Usage, error) {
	var text string
	var usage Usage
	var streamErr error

	for {
		select {
		case <-ctx.Done():
			return text, usage, ctx.Err()
		case err, ok := <-resp.ErrCh:
			if ok && err != nil {
				streamErr = err
			}
			if !ok {
				// ErrCh closed, drain remaining chunks
				for chunk := range resp.Chunks {
					text += accumulateChunk(chunk)
					if chunk.Done && chunk.Usage != nil {
						usage = *chunk.Usage
					}
				}
				return text, usage, streamErr
			}
		case chunk, ok := <-resp.Chunks:
			if !ok {
				// Chunks closed, drain ErrCh
				for err := range resp.ErrCh {
					if err != nil {
						streamErr = err
					}
				}
				return text, usage, streamErr
			}
			text += accumulateChunk(chunk)
			if chunk.Done && chunk.Usage != nil {
				usage = *chunk.Usage
			}
		}
	}
}

// accumulateChunk returns only the answer text from a chunk.
// Reasoning text is intentionally excluded so it does not leak into
// displayed responses or session memory.
func accumulateChunk(chunk StreamChunk) string {
	return chunk.Text
}

// NopStreamCloser is a no-op WriteCloser for discarding stream output.
type NopStreamCloser struct{}

func (NopStreamCloser) Write(p []byte) (int, error) { return len(p), nil }
func (NopStreamCloser) Close() error                { return nil }
