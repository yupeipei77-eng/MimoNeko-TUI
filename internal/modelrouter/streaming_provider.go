package modelrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// streamRequest is the OpenAI streaming request body.
type streamRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
}

// streamChunk is a single SSE chunk from the OpenAI streaming API.
type streamChunk struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []streamChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage,omitempty"`
}

type streamChoice struct {
	Delta        streamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

type streamDelta struct {
	Content string `json:"content"`
}

// StreamComplete implements StreamingProvider for OpenAI-compatible APIs.
func (p *OpenAICompatibleProvider) StreamComplete(ctx context.Context, req CompletionRequest) (StreamResponse, error) {
	apiKey, err := p.resolveAPIKey()
	if err != nil {
		return StreamResponse{}, err
	}

	messages := BundleToMessages(req.Bundle)

	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	temp := req.Temperature
	if temp <= 0 {
		temp = 0.0
	}

	requestBody := streamRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temp,
		Stream:      true,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return StreamResponse{}, fmt.Errorf("marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return StreamResponse{}, fmt.Errorf("create stream request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return StreamResponse{}, fmt.Errorf("stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return StreamResponse{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	chunks := make(chan StreamChunk, 64)
	errCh := make(chan error, 1)

	go p.readSSEStream(ctx, resp.Body, chunks, errCh)

	return StreamResponse{
		Chunks: chunks,
		ErrCh:  errCh,
	}, nil
}

// SupportsStreaming returns true. OpenAI-compatible providers support streaming.
func (p *OpenAICompatibleProvider) SupportsStreaming() bool {
	return true
}

// readSSEStream reads an SSE stream and sends chunks to the channel.
func (p *OpenAICompatibleProvider) readSSEStream(ctx context.Context, body io.ReadCloser, chunks chan<- StreamChunk, errCh chan<- error) {
	defer close(chunks)
	defer close(errCh)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var requestID string

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and SSE comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Parse SSE data line
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for stream end marker
		if data == "[DONE]" {
			chunks <- StreamChunk{
				Done:      true,
				RequestID: requestID,
			}
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Skip malformed chunks but don't fail the stream
			continue
		}

		if chunk.ID != "" {
			requestID = chunk.ID
		}

		// Extract text delta from choices
		if len(chunk.Choices) > 0 {
			text := chunk.Choices[0].Delta.Content
			if text != "" {
				select {
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				case chunks <- StreamChunk{
					Text:      text,
					RequestID: requestID,
				}:
				}
			}

			// Check for finish reason with usage
			if chunk.Choices[0].FinishReason != "" {
				var usage *Usage
				if chunk.Usage != nil {
					u := parseUsage(*chunk.Usage, 0)
					usage = &u
				}
				chunks <- StreamChunk{
					Done:      true,
					Usage:     usage,
					RequestID: requestID,
				}
				return
			}
		}

		// Handle usage-only chunks (final chunk sometimes has only usage)
		if chunk.Usage != nil && len(chunk.Choices) == 0 {
			u := parseUsage(*chunk.Usage, 0)
			chunks <- StreamChunk{
				Done:      true,
				Usage:     &u,
				RequestID: requestID,
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		// Don't report context cancellation as an error
		if ctx.Err() == nil {
			errCh <- fmt.Errorf("stream read error: %w", err)
		}
	}
}

// CreateStreamChannel creates a buffered channel pair for streaming.
func CreateStreamChannel() (chan StreamChunk, chan error) {
	return make(chan StreamChunk, 64), make(chan error, 1)
}

// StreamToCallback drains a StreamResponse and calls a callback for each chunk.
// Returns the accumulated text, usage, and any error.
// This is useful for CLI streaming output.
func StreamToCallback(ctx context.Context, resp StreamResponse, onChunk func(chunk StreamChunk)) (string, Usage, error) {
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
					if onChunk != nil {
						onChunk(chunk)
					}
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
			if onChunk != nil {
				onChunk(chunk)
			}
			if chunk.Done && chunk.Usage != nil {
				usage = *chunk.Usage
			}
		}
	}
}

// MockStreamProvider creates a mock streaming provider for testing.
type MockStreamProvider struct {
	name   string
	chunks []StreamChunk
	err    error
	models map[string]bool
}

// NewMockStreamProvider creates a new mock streaming provider.
func NewMockStreamProvider(name string, textChunks []string) *MockStreamProvider {
	chunks := make([]StreamChunk, 0, len(textChunks)+1)
	for _, t := range textChunks {
		chunks = append(chunks, StreamChunk{Text: t})
	}
	chunks = append(chunks, StreamChunk{
		Done: true,
		Usage: &Usage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
			Estimated:    true,
		},
	})
	return &MockStreamProvider{
		name:   name,
		chunks: chunks,
	}
}

func (m *MockStreamProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	var text string
	for _, c := range m.chunks {
		text += c.Text
	}
	return CompletionResponse{
		Provider: m.name,
		Model:    req.Model,
		Text:     text,
		Usage:    Usage{Estimated: true},
	}, m.err
}

func (m *MockStreamProvider) Name() string { return m.name }

func (m *MockStreamProvider) Supports(model string) bool {
	if m.models == nil {
		return true
	}
	return m.models[model]
}

func (m *MockStreamProvider) StreamComplete(ctx context.Context, req CompletionRequest) (StreamResponse, error) {
	if m.err != nil {
		return StreamResponse{}, m.err
	}

	chunks := make(chan StreamChunk, len(m.chunks))
	errCh := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errCh)
		for _, chunk := range m.chunks {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case chunks <- chunk:
				// Small delay to simulate streaming
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	return StreamResponse{Chunks: chunks, ErrCh: errCh}, nil
}

func (m *MockStreamProvider) SupportsStreaming() bool { return true }
