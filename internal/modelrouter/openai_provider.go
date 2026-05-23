package modelrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OpenAICompatibleProvider implements the Provider interface for OpenAI-compatible APIs.
// It uses net/http for requests and never logs or exposes API keys.
type OpenAICompatibleProvider struct {
	name       string
	baseURL    string
	apiKeyEnv  string
	models     map[string]bool // supported model names
	httpClient *http.Client
}

// NewOpenAICompatibleProvider creates a new provider from configuration.
// The httpClient defaults to a client with a 60-second timeout if nil.
func NewOpenAICompatibleProvider(name, baseURL, apiKeyEnv string, models []string, httpClient *http.Client) *OpenAICompatibleProvider {
	modelSet := make(map[string]bool, len(models))
	for _, m := range models {
		modelSet[m] = true
	}

	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	return &OpenAICompatibleProvider{
		name:       name,
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKeyEnv:  apiKeyEnv,
		models:     modelSet,
		httpClient: client,
	}
}

// Name returns the provider identifier.
func (p *OpenAICompatibleProvider) Name() string {
	return p.name
}

// Supports returns whether this provider can serve the given model.
func (p *OpenAICompatibleProvider) Supports(model string) bool {
	return p.models[model]
}

// Complete sends a completion request to the OpenAI-compatible API.
//
// It reads the API key from the environment variable specified in api_key_env.
// If the key is missing, it returns an error that does NOT contain the key value.
// The request body is built from the Bundle via BundleToMessages, but the
// Authorization header is never logged or included in error messages.
func (p *OpenAICompatibleProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	apiKey, err := p.resolveAPIKey()
	if err != nil {
		return CompletionResponse{}, err
	}

	// Convert Bundle to messages
	messages := BundleToMessages(req.Bundle)

	// Build request body
	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	temp := req.Temperature
	if temp <= 0 {
		temp = 0.0
	}

	requestBody := openAIRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temp,
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
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

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
		// Sanitize error: never include the full response body as it might contain keys
		return CompletionResponse{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("parse response: %w", err)
	}

	usage := parseUsage(apiResp.Usage, req.Bundle.Report.TotalTokens)

	return CompletionResponse{
		Provider:  p.name,
		Model:     apiResp.Model,
		Text:      extractText(apiResp),
		Usage:     usage,
		RequestID: apiResp.ID,
	}, nil
}

// resolveAPIKey reads the API key from the environment variable.
// Returns a clear error if missing, without leaking the key value.
func (p *OpenAICompatibleProvider) resolveAPIKey() (string, error) {
	if p.apiKeyEnv == "" {
		return "", fmt.Errorf("provider %q: no api_key_env configured", p.name)
	}

	key := os.Getenv(p.apiKeyEnv)
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("provider %q: API key not found in environment variable %s", p.name, p.apiKeyEnv)
	}

	return key, nil
}

// APIKeyStatus returns whether the API key is configured, without revealing the value.
func (p *OpenAICompatibleProvider) APIKeyStatus() string {
	if p.apiKeyEnv == "" {
		return "missing"
	}
	key := os.Getenv(p.apiKeyEnv)
	if strings.TrimSpace(key) == "" {
		return "missing"
	}
	return "configured"
}

// APIKeyEnvName returns the name of the environment variable for the API key.
func (p *OpenAICompatibleProvider) APIKeyEnvName() string {
	return p.apiKeyEnv
}

// BaseURL returns the provider's base URL.
func (p *OpenAICompatibleProvider) BaseURL() string {
	return p.baseURL
}

// openAIRequest is the request body for the OpenAI Chat Completion API.
type openAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

// openAIResponse is the response from the OpenAI Chat Completion API.
type openAIResponse struct {
	ID      string           `json:"id"`
	Model   string           `json:"model"`
	Choices []openAIChoice   `json:"choices"`
	Usage   openAIUsage      `json:"usage"`
}

// openAIChoice is a single choice in the response.
type openAIChoice struct {
	Message openAIChoiceMessage `json:"message"`
}

// openAIChoiceMessage is the message within a choice.
type openAIChoiceMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIUsage is the usage information from the API response.
type openAIUsage struct {
	PromptTokens     int                     `json:"prompt_tokens"`
	CompletionTokens int                     `json:"completion_tokens"`
	TotalTokens      int                     `json:"total_tokens"`
	PromptTokensDetails *promptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

// promptTokensDetails contains cached token information.
type promptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// parseUsage converts OpenAI API usage to our Usage type.
// If the API returns cached_tokens, Estimated=false.
// If cached_tokens is missing, Estimated=true.
// If the API returns no usage at all, uses bundleReportTotal for estimation.
func parseUsage(apiUsage openAIUsage, bundleReportTotal int) Usage {
	usage := Usage{
		InputTokens:  apiUsage.PromptTokens,
		OutputTokens: apiUsage.CompletionTokens,
		TotalTokens:  apiUsage.TotalTokens,
	}

	if apiUsage.PromptTokensDetails != nil {
		usage.CachedTokens = apiUsage.PromptTokensDetails.CachedTokens
		usage.Estimated = false
	} else {
		usage.CachedTokens = 0
		usage.Estimated = true
	}

	// If API returns zero usage, estimate from bundle report
	if usage.InputTokens == 0 && usage.TotalTokens == 0 && bundleReportTotal > 0 {
		usage.InputTokens = bundleReportTotal
		usage.TotalTokens = bundleReportTotal
		usage.Estimated = true
	}

	return usage
}

// extractText gets the content from the first choice in the response.
func extractText(resp openAIResponse) string {
	if len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Message.Content
}
