package modelrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mimoneko/mimoneko/internal/auth"
	"github.com/mimoneko/mimoneko/internal/pathutil"
)

// MimoProvider implements the Provider interface for the Mimo API.
//
// Mimo differs from OpenAI in these key ways:
//   - Auth uses the "api-key" header instead of "Authorization: Bearer".
//   - Supports max_completion_tokens (in addition to max_tokens).
//   - Non-streaming responses may return content in delta.content (not always message.content).
//   - Errors use {"code":"xxx","message":"yyy"} format instead of {"error":{"message":"..."}}.
type MimoProvider struct {
	name       string
	baseURL    string
	apiKeyEnv  string
	models     map[string]bool
	httpClient *http.Client
}

// NewMimoProvider creates a new Mimo provider from configuration.
// The httpClient defaults to a client with a 60-second timeout if nil.
func NewMimoProvider(name, baseURL, apiKeyEnv string, models []string, httpClient *http.Client) *MimoProvider {
	modelSet := make(map[string]bool, len(models))
	for _, m := range models {
		modelSet[m] = true
	}

	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	return &MimoProvider{
		name:       name,
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKeyEnv:  apiKeyEnv,
		models:     modelSet,
		httpClient: client,
	}
}

// Name returns the provider identifier.
func (p *MimoProvider) Name() string { return p.name }

// APIKeyStatus returns whether the API key is configured, without revealing the value.
func (p *MimoProvider) APIKeyStatus() string {
	if p.apiKeyEnv == "" {
		return "missing"
	}
	key := os.Getenv(p.apiKeyEnv)
	if strings.TrimSpace(key) == "" {
		return "missing"
	}
	return "configured"
}

// Supports returns whether this provider can serve the given model.
func (p *MimoProvider) Supports(model string) bool { return p.models[model] }

// resolveAPIKey reads the API key from the environment variable.
func (p *MimoProvider) resolveAPIKey() (string, error) {
	if p.apiKeyEnv == "" {
		return "", fmt.Errorf("provider %q: no api_key_env configured", p.name)
	}

	key := os.Getenv(p.apiKeyEnv)
	if strings.TrimSpace(key) == "" {
		key = auth.GetAPIKeyForEnv(p.apiKeyEnv)
	}
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("provider %q: API key not found in environment variable %s", p.name, p.apiKeyEnv)
	}
	if pathutil.APIKeyLooksPlaceholder(key) {
		return "", fmt.Errorf("provider %q: API key in environment variable %s appears to be a placeholder; set a real key", p.name, p.apiKeyEnv)
	}

	return key, nil
}

// ---------------------------------------------------------------------------
// Request / response types
// ---------------------------------------------------------------------------

type mimoRequest struct {
	Model               string    `json:"model"`
	Messages            []Message `json:"messages"`
	MaxCompletionTokens int       `json:"max_completion_tokens"`
	Temperature         float64   `json:"temperature"`
	Stream              bool      `json:"stream,omitempty"`
}

type mimoResponse struct {
	ID      string       `json:"id"`
	Model   string       `json:"model"`
	Choices []mimoChoice `json:"choices"`
	Usage   openAIUsage  `json:"usage"`
}

type mimoChoice struct {
	Message *mimoMessage `json:"message,omitempty"`
	Delta   *mimoDelta   `json:"delta,omitempty"`
}

type mimoMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type mimoDelta struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// mimoStreamChunk is one SSE chunk from Mimo's streaming API.
type mimoStreamChunk struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Choices []mimoStreamChoice `json:"choices"`
	Usage   *openAIUsage       `json:"usage,omitempty"`
}

type mimoStreamChoice struct {
	Delta        mimoDelta `json:"delta"`
	FinishReason string    `json:"finish_reason"`
}

// OpenAI error format (fallback).
type openAIErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
}

// ---------------------------------------------------------------------------
// Complete (non-streaming)
// ---------------------------------------------------------------------------

// Complete sends a non-streaming completion request to the Mimo API.
//
// It reads the API key from the environment variable specified in api_key_env.
// The "api-key" header is used instead of "Authorization: Bearer".
func (p *MimoProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	apiKey, err := p.resolveAPIKey()
	if err != nil {
		return CompletionResponse{}, err
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

	requestBody := mimoRequest{
		Model:               req.Model,
		Messages:            messages,
		MaxCompletionTokens: maxTokens,
		Temperature:         temp,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("marshal request body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return CompletionResponse{}, p.parseError(respBody, resp.StatusCode)
	}

	var apiResp mimoResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("parse response: %w", err)
	}

	usage := parseMimoUsage(apiResp.Usage, req.Bundle.Report.TotalTokens)

	return CompletionResponse{
		Provider:  p.name,
		Model:     apiResp.Model,
		Text:      extractMimoText(apiResp),
		Usage:     usage,
		RequestID: apiResp.ID,
	}, nil
}

// extractMimoText extracts the content from a Mimo response.
// Mimo may return content in either message.content (OpenAI-compatible)
// or delta.content (Mimo-specific, even for non-streaming).
func extractMimoText(resp mimoResponse) string {
	if len(resp.Choices) == 0 {
		return ""
	}
	choice := resp.Choices[0]
	if choice.Message != nil && choice.Message.Content != "" {
		return choice.Message.Content
	}
	if choice.Delta != nil && choice.Delta.Content != "" {
		return choice.Delta.Content
	}
	return ""
}

// parseError tries Mimo error format {"code":"...","message":"..."}
// first, then falls back to OpenAI {"error":{"message":"..."}}.
func (p *MimoProvider) parseError(body []byte, statusCode int) error {
	var errBody struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &errBody); err == nil && errBody.Message != "" {
		return fmt.Errorf("API returned status %d: %s (code: %s)", statusCode, errBody.Message, errBody.Code)
	}

	var openAIErr struct {
		Error openAIErrorBody `json:"error"`
	}
	if err := json.Unmarshal(body, &openAIErr); err == nil && openAIErr.Error.Message != "" {
		return fmt.Errorf("API returned status %d: %s", statusCode, openAIErr.Error.Message)
	}

	return fmt.Errorf("API returned status %d", statusCode)
}

// ---------------------------------------------------------------------------
// Streaming
// ---------------------------------------------------------------------------

// StreamComplete sends a streaming completion request to the Mimo API.
func (p *MimoProvider) StreamComplete(ctx context.Context, req CompletionRequest) (StreamResponse, error) {
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

	requestBody := mimoRequest{
		Model:               req.Model,
		Messages:            messages,
		MaxCompletionTokens: maxTokens,
		Temperature:         temp,
		Stream:              true,
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
	httpReq.Header.Set("api-key", apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return StreamResponse{}, fmt.Errorf("stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return StreamResponse{}, p.parseError(respBody, resp.StatusCode)
	}

	chunks := make(chan StreamChunk, 64)
	errCh := make(chan error, 1)

	go p.readMimoSSEStream(ctx, resp.Body, chunks, errCh)

	return StreamResponse{
		Chunks: chunks,
		ErrCh:  errCh,
	}, nil
}

// SupportsStreaming returns true. Mimo supports streaming.
func (p *MimoProvider) SupportsStreaming() bool { return true }

// APIKeyEnvName returns the name of the environment variable for the API key.
func (p *MimoProvider) APIKeyEnvName() string {
	return p.apiKeyEnv
}

// BaseURL returns the provider's base URL.
func (p *MimoProvider) BaseURL() string {
	return p.baseURL
}

// readMimoSSEStream reads an SSE stream from Mimo and sends chunks to the channel.
// Mimo streaming is OpenAI-SSE compatible with these additions:
//   - Delta may include reasoning_content.
//   - Stream ends with [DONE] or finish_reason.
func (p *MimoProvider) readMimoSSEStream(ctx context.Context, body io.ReadCloser, chunks chan<- StreamChunk, errCh chan<- error) {
	defer close(chunks)
	defer close(errCh)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var requestID string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			chunks <- StreamChunk{
				Done:      true,
				RequestID: requestID,
			}
			return
		}

		var chunk mimoStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.ID != "" {
			requestID = chunk.ID
		}

		if len(chunk.Choices) > 0 {
			reasoning := chunk.Choices[0].Delta.ReasoningContent
			text := chunk.Choices[0].Delta.Content

			if reasoning != "" || text != "" {
				select {
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				case chunks <- StreamChunk{
					Text:          text,
					ReasoningText: reasoning,
					RequestID:     requestID,
				}:
				}
			}

			if chunk.Choices[0].FinishReason != "" {
				var usage *Usage
				if chunk.Usage != nil {
					u := parseMimoUsage(*chunk.Usage, 0)
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

		if chunk.Usage != nil && len(chunk.Choices) == 0 {
			u := parseMimoUsage(*chunk.Usage, 0)
			chunks <- StreamChunk{
				Done:      true,
				Usage:     &u,
				RequestID: requestID,
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() == nil {
			errCh <- fmt.Errorf("stream read error: %w", err)
		}
	}
}
