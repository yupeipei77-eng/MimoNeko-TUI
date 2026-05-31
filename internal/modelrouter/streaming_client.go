package modelrouter

import (
	"context"
	"fmt"
	"time"

	"github.com/mimoneko/mimoneko/internal/events"
)

// StreamingClient wraps a ModelRouter and adds streaming support with event emission.
// It falls back to non-streaming when the provider does not support streaming.
type StreamingClient struct {
	router   ModelRouter
	emitter  events.EventEmitter
}

// NewStreamingClient creates a new streaming client.
func NewStreamingClient(router ModelRouter, emitter events.EventEmitter) *StreamingClient {
	return &StreamingClient{
		router:  router,
		emitter: emitter,
	}
}

// StreamResult contains the result of a streaming completion.
type StreamResult struct {
	Text      string
	Usage     Usage
	RequestID string
	Err       error
}

// CompleteStream performs a streaming completion and delivers chunks via callback.
// If the provider supports streaming, it uses the streaming API.
// Otherwise, it falls back to non-streaming Complete and delivers the full text as one chunk.
func (c *StreamingClient) CompleteStream(ctx context.Context, req CompletionRequest, onChunk func(StreamChunk)) (StreamResult, error) {
	startedAt := time.Now()

	// Emit stream started event
	c.emitSafe(ctx, events.RunEvent{
		ID:        mustGenerateStreamEventID(),
		Type:      "stream.started",
		Source:    "model",
		Status:    "started",
		Message:   "Streaming completion started",
		StartedAt: startedAt,
		Metadata:  map[string]string{"model": req.Model},
	})

	// Try streaming first
	if sp, ok := c.router.(StreamingModelRouter); ok {
		streamResp, err := sp.StreamComplete(ctx, req)
		if err == nil {
			text, usage, streamErr := StreamToCallback(ctx, streamResp, onChunk)

			c.emitSafe(ctx, events.RunEvent{
				ID:         mustGenerateStreamEventID(),
				Type:       events.EventStreamDone,
				Source:     "model",
				Status:     "succeeded",
				Message:    fmt.Sprintf("Streaming completed: %d tokens", usage.TotalTokens),
				StartedAt:  startedAt,
				FinishedAt: time.Now().UTC(),
				DurationMs: time.Since(startedAt).Milliseconds(),
			})

			return StreamResult{
				Text:  text,
				Usage: usage,
				Err:   streamErr,
			}, nil
		}
		// If streaming failed with a non-transient error, fall through to non-streaming
	}

	// Fallback to non-streaming
	resp, err := c.router.Complete(ctx, req)
	if err != nil {
		c.emitSafe(ctx, events.RunEvent{
			ID:         mustGenerateStreamEventID(),
			Type:       events.EventStreamDone,
			Source:     "model",
			Status:     "failed",
			Message:    "Completion failed",
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
			DurationMs: time.Since(startedAt).Milliseconds(),
			Error:      err.Error(),
		})
		return StreamResult{Err: err}, err
	}

	// Deliver as single chunk
	chunk := StreamChunk{
		Text:      resp.Text,
		Done:      true,
		Usage:     &resp.Usage,
		RequestID: resp.RequestID,
	}
	if onChunk != nil {
		onChunk(chunk)
	}

	c.emitSafe(ctx, events.RunEvent{
		ID:         mustGenerateStreamEventID(),
		Type:       events.EventStreamDone,
		Source:     "model",
		Status:     "succeeded",
		Message:    fmt.Sprintf("Non-streaming completed: %d tokens", resp.Usage.TotalTokens),
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
		DurationMs: time.Since(startedAt).Milliseconds(),
	})

	return StreamResult{
		Text:      resp.Text,
		Usage:     resp.Usage,
		RequestID: resp.RequestID,
	}, nil
}

func (c *StreamingClient) emitSafe(ctx context.Context, event events.RunEvent) {
	events.SafeEmit(c.emitter, ctx, event)
}

func mustGenerateStreamEventID() string {
	id, err := events.GenerateEventID()
	if err != nil {
		return "evt_stream_error"
	}
	return id
}
