package modelprofile

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mimoneko/mimoneko/internal/auth"
	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/pathutil"
	"gopkg.in/yaml.v3"
)

const (
	providerTypeOpenAICompatible  = "openai-compatible"
	providerTypeMimo              = "mimo"
	defaultPurpose                = "coding"
	defaultMaxOutputTokens        = 4096
	maxResponseBodyBytes          = 1 << 20
	maxDisplayedResponseBytes     = 256
	maxDisplayedChatResponseBytes = 2048
)

// Preset describes a built-in OpenAI-compatible provider profile.
type Preset struct {
	Name            string
	Type            string
	BaseURL         string
	APIKeyEnv       string
	SuggestedModels []string
}

// SetupOptions configures a provider/model entry in models.yaml.
type SetupOptions struct {
	Preset              string
	Provider            string
	BaseURL             string
	APIKeyEnv           string
	Model               string
	Purpose             string
	MaxOutputTokens     int
	SupportsPrefixCache bool
	SetDefault          bool
}

// SetupResult describes the updated profile and any safe user hints.
type SetupResult struct {
	Provider string
	Model    string
	Hints    []string
}

// DiscoverOptions configures a /models request.
type DiscoverOptions struct {
	Provider  string
	BaseURL   string
	APIKeyEnv string
	Client    *http.Client
}

// TestOptions configures a tiny chat/completions smoke test.
type TestOptions struct {
	Provider  string
	Model     string
	BaseURL   string
	APIKeyEnv string
	Prompt    string
	Client    *http.Client
}

// TestResult is the safe result of a model smoke test.
type TestResult struct {
	Provider  string
	Model     string
	BaseURL   string
	APIKeyEnv string
	Status    string
	LatencyMs int64
	Response  string
	Error     string
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string
	Content string
}

// ChatOptions configures a lightweight MimoNeko chat completion.
type ChatOptions struct {
	Provider  string
	Model     string
	BaseURL   string
	APIKeyEnv string
	Prompt    string
	MaxTokens int
	Client    *http.Client
	Messages  []ChatMessage
}

// ChatResult is a sanitized text response from a configured model.
type ChatResult struct {
	Provider          string
	Model             string
	Response          string
	PromptTokens      int
	CachedTokens      int
	CachedTokensKnown bool
	CompletionTokens  int
	TotalTokens       int
}

// ChatStreamChunk represents a single streaming chunk from a chat completion.
type ChatStreamChunk struct {
	Text          string
	ReasoningText string
	Done          bool
}

// ChatStreamFunc sends chunks via callback and returns the final result.
type ChatStreamFunc func(ctx context.Context, onChunk func(ChatStreamChunk)) (ChatResult, error)

// RemoveOptions selects a provider or model to remove.
type RemoveOptions struct {
	Provider string
	Model    string
}

type chatAPIMessage struct {
	Content string `json:"content"`
}

type chatAPIDelta struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type chatAPIChoice struct {
	Message *chatAPIMessage `json:"message,omitempty"`
	Delta   *chatAPIDelta   `json:"delta,omitempty"`
}

type chatAPIUsage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	PromptTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details,omitempty"`
}

type chatAPIResponse struct {
	Model   string          `json:"model"`
	Choices []chatAPIChoice `json:"choices"`
	Usage   chatAPIUsage    `json:"usage"`
}

var presets = map[string]Preset{
	"mimo": {
		Name:      "mimo",
		Type:      providerTypeMimo,
		BaseURL:   "https://token-plan-cn.xiaomimimo.com/v1",
		APIKeyEnv: "MIMO_API_KEY",
		SuggestedModels: []string{
			"mimo-v2.5-pro",
			"mimo-v2.5",
			"mimo-v2-pro",
		},
	},
	"openai": {
		Name:      "openai",
		Type:      providerTypeOpenAICompatible,
		BaseURL:   "https://api.openai.com/v1",
		APIKeyEnv: "OPENAI_API_KEY",
	},
	"deepseek": {
		Name:      "deepseek",
		Type:      providerTypeOpenAICompatible,
		BaseURL:   "https://api.deepseek.com/v1",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	},
	"glm": {
		Name:      "glm",
		Type:      providerTypeOpenAICompatible,
		BaseURL:   "https://open.bigmodel.cn/api/paas/v4",
		APIKeyEnv: "GLM_API_KEY",
	},
	"custom-openai-compatible": {
		Name: "custom-openai-compatible",
		Type: providerTypeOpenAICompatible,
	},
}

// PresetNames returns stable preset names for UI prompts.
func PresetNames() []string {
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetPreset returns a built-in provider preset.
func GetPreset(name string) (Preset, bool) {
	preset, ok := presets[strings.TrimSpace(name)]
	return preset, ok
}

// ModelsPath returns the models.yaml path for a MimoNeko root.
func ModelsPath(root string) string {
	return filepath.Join(config.ConfigDir(root), "models.yaml")
}

// Load reads only .mimoneko/models.yaml.
func Load(root string) (config.ModelsConfig, error) {
	var models config.ModelsConfig
	content, err := os.ReadFile(ModelsPath(root))
	if err != nil {
		return models, fmt.Errorf("read models.yaml: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&models); err != nil {
		return models, fmt.Errorf("parse models.yaml: %w", err)
	}
	return models, nil
}

func loadModelsForConnection(root string, explicitConnection bool) (config.ModelsConfig, error) {
	models, err := Load(root)
	if err == nil {
		return models, nil
	}
	if explicitConnection {
		return models, nil
	}
	if userModels, ok, userErr := auth.UserModelsConfig(); userErr != nil {
		return models, userErr
	} else if ok {
		return userModels, nil
	}
	return models, err
}

// Save writes only .mimoneko/models.yaml.
func Save(root string, models config.ModelsConfig) error {
	if err := validateModels(models); err != nil {
		return err
	}
	data, err := yaml.Marshal(models)
	if err != nil {
		return fmt.Errorf("marshal models.yaml: %w", err)
	}
	path := ModelsPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write models.yaml: %w", err)
	}
	return nil
}

// Setup adds or updates a provider and model profile.
func Setup(root string, opt SetupOptions) (SetupResult, error) {
	models, err := Load(root)
	if err != nil {
		return SetupResult{}, err
	}
	if err := applyPreset(&opt); err != nil {
		return SetupResult{}, err
	}
	normalizeSetupOptions(&opt)
	if err := validateSetupOptions(opt); err != nil {
		return SetupResult{}, err
	}

	providerIndex := findProviderIndex(models, opt.Provider)
	provider := config.ProviderConfig{
		Name:      opt.Provider,
		Type:      providerTypeForPreset(opt.Preset),
		BaseURL:   strings.TrimRight(opt.BaseURL, "/"),
		APIKeyEnv: opt.APIKeyEnv,
	}
	if providerIndex >= 0 {
		provider = models.Providers[providerIndex]
		provider.Type = providerTypeForPreset(opt.Preset)
		provider.BaseURL = strings.TrimRight(opt.BaseURL, "/")
		provider.APIKeyEnv = opt.APIKeyEnv
	}

	model := config.ModelConfig{
		Name:                opt.Model,
		Purpose:             opt.Purpose,
		MaxOutputTokens:     opt.MaxOutputTokens,
		SupportsPrefixCache: opt.SupportsPrefixCache,
	}
	modelIndex := findModelIndex(provider, opt.Model)
	if modelIndex >= 0 {
		existing := provider.Models[modelIndex]
		model.MaxContextTokens = existing.MaxContextTokens
		model.ReasoningLevel = existing.ReasoningLevel
		model.CapabilitySource = existing.CapabilitySource
		model.Pricing = existing.Pricing
		if !opt.SupportsPrefixCache {
			model.SupportsPrefixCache = existing.SupportsPrefixCache
		}
		if capability, ok := CapabilityFor(provider.Name, model.Name); ok {
			ApplyCapability(&model, capability, false)
		}
		provider.Models[modelIndex] = model
	} else {
		if capability, ok := CapabilityFor(provider.Name, model.Name); ok {
			ApplyCapability(&model, capability, false)
		}
		provider.Models = append(provider.Models, model)
	}

	if providerIndex >= 0 {
		models.Providers[providerIndex] = provider
	} else {
		models.Providers = append(models.Providers, provider)
	}

	if opt.SetDefault {
		setDefaultModel(&models, opt.Provider, opt.Model)
	}

	if err := Save(root, models); err != nil {
		return SetupResult{}, err
	}

	result := SetupResult{Provider: opt.Provider, Model: opt.Model}
	if APIKeyStatus(opt.APIKeyEnv) == "missing" {
		result.Hints = APIKeyHint(opt.APIKeyEnv)
	}
	return result, nil
}

// Use switches routing.default_model and the first fallback entry.
func Use(root, modelName string) (string, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return "", errors.New("model name is required")
	}
	models, err := Load(root)
	if err != nil {
		return "", err
	}
	providerName, ok := FindProviderForModel(models, modelName)
	if !ok {
		return "", fmt.Errorf("model %q is not configured; run model setup or model discover first", modelName)
	}
	setDefaultModel(&models, providerName, modelName)
	if err := Save(root, models); err != nil {
		return "", err
	}
	return providerName, nil
}

// Remove deletes a model or provider without touching environment variables.
func Remove(root string, opt RemoveOptions) error {
	opt.Provider = strings.TrimSpace(opt.Provider)
	opt.Model = strings.TrimSpace(opt.Model)
	if (opt.Provider == "") == (opt.Model == "") {
		return errors.New("exactly one of --provider or --model is required")
	}

	models, err := Load(root)
	if err != nil {
		return err
	}

	if opt.Provider != "" {
		idx := findProviderIndex(models, opt.Provider)
		if idx < 0 {
			return fmt.Errorf("provider %q is not configured", opt.Provider)
		}
		for _, model := range models.Providers[idx].Models {
			if model.Name == models.Routing.DefaultModel {
				return fmt.Errorf("cannot remove provider %q because it contains current default_model %q", opt.Provider, model.Name)
			}
		}
		models.Providers = append(models.Providers[:idx], models.Providers[idx+1:]...)
		models.Routing.FallbackChain = filterFallback(models.Routing.FallbackChain, opt.Provider, "")
		return Save(root, models)
	}

	for i, provider := range models.Providers {
		modelIndex := findModelIndex(provider, opt.Model)
		if modelIndex < 0 {
			continue
		}
		if opt.Model == models.Routing.DefaultModel {
			return fmt.Errorf("cannot remove current default_model %q; switch default model first", opt.Model)
		}
		if len(provider.Models) == 1 {
			return fmt.Errorf("cannot remove model %q because provider %q would have no models", opt.Model, provider.Name)
		}
		provider.Models = append(provider.Models[:modelIndex], provider.Models[modelIndex+1:]...)
		models.Providers[i] = provider
		models.Routing.FallbackChain = filterFallback(models.Routing.FallbackChain, provider.Name, opt.Model)
		return Save(root, models)
	}
	return fmt.Errorf("model %q is not configured", opt.Model)
}

// Discover queries an OpenAI-compatible /models endpoint.
func Discover(ctx context.Context, root string, opt DiscoverOptions) ([]string, error) {
	models, err := loadModelsForConnection(root, strings.TrimSpace(opt.BaseURL) != "")
	if err != nil {
		return nil, err
	}
	baseURL, apiKeyEnv, err := resolveProviderConnection(models, opt.Provider, "", opt.BaseURL, opt.APIKeyEnv)
	if err != nil {
		return nil, err
	}
	providerName, _, providerErr := resolveProviderAndModel(models, opt.Provider, "")
	if providerErr != nil {
		providerName = strings.TrimSpace(opt.Provider)
	}
	providerType := resolveProviderType(models, providerName)
	apiKey, err := resolveAPIKey(apiKeyEnv)
	if err != nil {
		return nil, err
	}

	client := opt.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	setProviderAuthHeader(req, providerType, apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("provider did not expose /models (status %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse /models response: %w", err)
	}
	names := make([]string, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		if strings.TrimSpace(item.ID) != "" {
			names = append(names, item.ID)
		}
	}
	return names, nil
}

// Test sends a tiny chat/completions request and returns a sanitized result.
func Test(ctx context.Context, root string, opt TestOptions) (TestResult, error) {
	models, err := loadModelsForConnection(root, strings.TrimSpace(opt.BaseURL) != "")
	if err != nil {
		return TestResult{}, err
	}
	baseURL, apiKeyEnv, err := resolveProviderConnection(models, opt.Provider, opt.Model, opt.BaseURL, opt.APIKeyEnv)
	if err != nil {
		return TestResult{}, err
	}
	providerName, modelName, err := resolveProviderAndModel(models, opt.Provider, opt.Model)
	if err != nil {
		if strings.TrimSpace(opt.BaseURL) == "" {
			return TestResult{}, err
		}
		providerName = strings.TrimSpace(opt.Provider)
		modelName = strings.TrimSpace(opt.Model)
	}
	if modelName == "" {
		return TestResult{}, errors.New("model is required")
	}
	apiKey, err := resolveAPIKey(apiKeyEnv)
	if err != nil {
		return TestResult{}, err
	}

	client := opt.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	prompt := strings.TrimSpace(opt.Prompt)
	if prompt == "" {
		prompt = "Reply with OK only."
	}

	providerType := resolveProviderType(models, providerName)
	body, err := marshalChatRequest(providerType, modelName, []map[string]string{
		{"role": "user", "content": prompt},
	}, 16, 0, false)
	if err != nil {
		return TestResult{}, fmt.Errorf("marshal request: %w", err)
	}

	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return TestResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setProviderAuthHeader(req, providerType, apiKey)

	resp, err := client.Do(req)
	latency := time.Since(started).Milliseconds()
	result := TestResult{Provider: providerName, Model: modelName, BaseURL: baseURL, APIKeyEnv: apiKeyEnv, LatencyMs: latency}
	if err != nil {
		result.Status = "failed"
		result.Error = SanitizeText(err.Error(), apiKey)
		return result, nil
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return TestResult{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		result.Status = "failed"
		result.Error = fmt.Sprintf("API returned status %d", resp.StatusCode)
		return result, nil
	}

	var parsed chatAPIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		result.Status = "failed"
		result.Error = "parse response failed"
		return result, nil
	}
	if parsed.Model != "" {
		result.Model = parsed.Model
	}
	result.Response = limitDisplay(SanitizeText(firstChoiceContent(parsed.Choices), apiKey))
	result.Status = "ok"
	return result, nil
}

// Chat sends a lightweight chat request for the terminal console.
func Chat(ctx context.Context, root string, opt ChatOptions) (ChatResult, error) {
	models, err := loadModelsForConnection(root, strings.TrimSpace(opt.BaseURL) != "")
	if err != nil {
		return ChatResult{}, err
	}
	baseURL, apiKeyEnv, err := resolveProviderConnection(models, opt.Provider, opt.Model, opt.BaseURL, opt.APIKeyEnv)
	if err != nil {
		return ChatResult{}, err
	}
	providerName, modelName, err := resolveProviderAndModel(models, opt.Provider, opt.Model)
	if err != nil {
		if strings.TrimSpace(opt.BaseURL) == "" {
			return ChatResult{}, err
		}
		providerName = strings.TrimSpace(opt.Provider)
		modelName = strings.TrimSpace(opt.Model)
	}
	if modelName == "" {
		return ChatResult{}, errors.New("model is required")
	}
	prompt := strings.TrimSpace(opt.Prompt)
	if prompt == "" {
		return ChatResult{}, errors.New("prompt is required")
	}
	apiKey, err := resolveAPIKey(apiKeyEnv)
	if err != nil {
		return ChatResult{}, err
	}

	client := opt.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	maxTokens := opt.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 512
	}

	messages := []map[string]string{
		{
			"role":    "system",
			"content": "You are MimoNeko, a concise local AI coding workspace assistant. Reply as plain text. Do not reveal hidden reasoning or secrets.",
		},
	}

	for _, msg := range opt.Messages {
		messages = append(messages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	messages = append(messages, map[string]string{"role": "user", "content": prompt})

	providerType := resolveProviderType(models, providerName)
	body, err := marshalChatRequest(providerType, modelName, messages, maxTokens, 0.2, false)
	if err != nil {
		return ChatResult{}, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setProviderAuthHeader(req, providerType, apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return ChatResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return ChatResult{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return ChatResult{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	var parsed chatAPIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return ChatResult{}, fmt.Errorf("parse response failed")
	}
	if parsed.Model != "" {
		modelName = parsed.Model
	}
	response := limitDisplayBytes(SanitizeText(firstChoiceContent(parsed.Choices), apiKey), maxDisplayedChatResponseBytes)
	if strings.TrimSpace(response) == "" {
		response = "(empty response)"
	}
	return ChatResult{
		Provider:          providerName,
		Model:             modelName,
		Response:          response,
		PromptTokens:      parsed.Usage.PromptTokens,
		CachedTokens:      cachedTokensFromDetails(parsed.Usage.PromptTokensDetails),
		CachedTokensKnown: parsed.Usage.PromptTokensDetails != nil,
		CompletionTokens:  parsed.Usage.CompletionTokens,
		TotalTokens:       parsed.Usage.TotalTokens,
	}, nil
}

func cachedTokensFromDetails(details *struct {
	CachedTokens int `json:"cached_tokens"`
}) int {
	if details == nil {
		return 0
	}
	return details.CachedTokens
}

// ChatStream creates a streaming chat function that sends chunks via callback.
func ChatStream(root string, opt ChatOptions) (ChatStreamFunc, error) {
	models, err := loadModelsForConnection(root, strings.TrimSpace(opt.BaseURL) != "")
	if err != nil {
		return nil, err
	}
	baseURL, apiKeyEnv, err := resolveProviderConnection(models, opt.Provider, opt.Model, opt.BaseURL, opt.APIKeyEnv)
	if err != nil {
		return nil, err
	}
	providerName, modelName, err := resolveProviderAndModel(models, opt.Provider, opt.Model)
	if err != nil {
		if strings.TrimSpace(opt.BaseURL) == "" {
			return nil, err
		}
		providerName = strings.TrimSpace(opt.Provider)
		modelName = strings.TrimSpace(opt.Model)
	}
	if modelName == "" {
		return nil, errors.New("model is required")
	}
	prompt := strings.TrimSpace(opt.Prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	apiKey, err := resolveAPIKey(apiKeyEnv)
	if err != nil {
		return nil, err
	}
	providerType := resolveProviderType(models, providerName)

	maxTokens := opt.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	return func(ctx context.Context, onChunk func(ChatStreamChunk)) (ChatResult, error) {
		client := opt.Client
		if client == nil {
			client = &http.Client{Timeout: 120 * time.Second}
		}

		messages := []map[string]string{
			{
				"role":    "system",
				"content": "You are MimoNeko, a concise local AI coding workspace assistant. Reply as plain text. Do not reveal hidden reasoning or secrets.",
			},
		}

		for _, msg := range opt.Messages {
			messages = append(messages, map[string]string{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}

		messages = append(messages, map[string]string{"role": "user", "content": prompt})

		body, err := marshalChatRequest(providerType, modelName, messages, maxTokens, 0, true)
		if err != nil {
			return ChatResult{}, fmt.Errorf("marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return ChatResult{}, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		setProviderAuthHeader(req, providerType, apiKey)

		resp, err := client.Do(req)
		if err != nil {
			return ChatResult{}, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ChatResult{}, fmt.Errorf("API returned status %d", resp.StatusCode)
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var fullText strings.Builder
		var usage ChatResult
		usage.Provider = providerName
		usage.Model = modelName

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
				onChunk(ChatStreamChunk{Done: true})
				break
			}

			var chunk struct {
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens        int `json:"prompt_tokens"`
					CompletionTokens    int `json:"completion_tokens"`
					TotalTokens         int `json:"total_tokens"`
					PromptTokensDetails *struct {
						CachedTokens int `json:"cached_tokens"`
					} `json:"prompt_tokens_details,omitempty"`
				} `json:"usage,omitempty"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if chunk.Model != "" {
				usage.Model = chunk.Model
			}

			if len(chunk.Choices) > 0 {
				text := chunk.Choices[0].Delta.Content
				reasoning := chunk.Choices[0].Delta.ReasoningContent
				if reasoning != "" {
					fullText.WriteString(reasoning)
				}
				if text != "" {
					fullText.WriteString(text)
				}
				if text != "" || reasoning != "" {
					onChunk(ChatStreamChunk{Text: text, ReasoningText: reasoning})
				}
				if chunk.Choices[0].FinishReason != "" {
					if chunk.Usage != nil {
						usage.PromptTokens = chunk.Usage.PromptTokens
						if chunk.Usage.PromptTokensDetails != nil {
							usage.CachedTokens = chunk.Usage.PromptTokensDetails.CachedTokens
							usage.CachedTokensKnown = true
						}
						usage.CompletionTokens = chunk.Usage.CompletionTokens
						usage.TotalTokens = chunk.Usage.TotalTokens
					}
					onChunk(ChatStreamChunk{Done: true})
					break
				}
			}

			if chunk.Usage != nil && len(chunk.Choices) == 0 {
				usage.PromptTokens = chunk.Usage.PromptTokens
				if chunk.Usage.PromptTokensDetails != nil {
					usage.CachedTokens = chunk.Usage.PromptTokensDetails.CachedTokens
					usage.CachedTokensKnown = true
				}
				usage.CompletionTokens = chunk.Usage.CompletionTokens
				usage.TotalTokens = chunk.Usage.TotalTokens
				onChunk(ChatStreamChunk{Done: true})
				break
			}
		}

		response := fullText.String()
		if strings.TrimSpace(response) == "" {
			response = "(empty response)"
		}
		usage.Response = SanitizeText(response, apiKey)
		return usage, nil
	}, nil
}

// APIKeyStatus checks whether an API key environment variable is configured.
func APIKeyStatus(envVar string) string {
	return pathutil.APIKeyStatus(envVar)
}

// APIKeyHint returns safe shell hints without any secret value.
func APIKeyHint(envVar string) []string {
	if strings.TrimSpace(envVar) == "" {
		return nil
	}
	return []string{
		fmt.Sprintf("api key environment variable %s is missing", envVar),
		fmt.Sprintf("Windows: setx %s \"your-key\"", envVar),
		fmt.Sprintf("macOS/Linux: export %s=\"your-key\"", envVar),
	}
}

// FindProviderForModel returns the first configured provider supporting modelName.
func FindProviderForModel(models config.ModelsConfig, modelName string) (string, bool) {
	for _, provider := range models.Providers {
		if findModelIndex(provider, modelName) >= 0 {
			return provider.Name, true
		}
	}
	return "", false
}

// SanitizeText redacts common secret patterns and exact known secret values.
func SanitizeText(text string, secrets ...string) string {
	safe := text
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret != "" {
			safe = strings.ReplaceAll(safe, secret, "<redacted>")
		}
	}
	patterns := []string{"Bearer ", "sk-", "AKIA"}
	for _, pattern := range patterns {
		if idx := strings.Index(safe, pattern); idx >= 0 {
			end := idx + len(pattern)
			for end < len(safe) && !isSecretBoundary(safe[end]) {
				end++
			}
			safe = safe[:idx] + pattern + "<redacted>" + safe[end:]
		}
	}
	for _, token := range []string{"API_KEY", "SECRET", "TOKEN", "PASSWORD", "PRIVATE_KEY"} {
		if strings.Contains(strings.ToUpper(safe), token) {
			safe = redactTokenLike(safe, token)
		}
	}
	return safe
}

func providerTypeForPreset(presetName string) string {
	preset, ok := GetPreset(presetName)
	if ok && preset.Type == providerTypeMimo {
		return providerTypeMimo
	}
	return providerTypeOpenAICompatible
}

func applyPreset(opt *SetupOptions) error {
	if strings.TrimSpace(opt.Preset) == "" {
		return nil
	}
	preset, ok := GetPreset(opt.Preset)
	if !ok {
		return fmt.Errorf("unknown provider preset %q", opt.Preset)
	}
	if strings.TrimSpace(opt.Provider) == "" {
		opt.Provider = preset.Name
	}
	if strings.TrimSpace(opt.BaseURL) == "" {
		opt.BaseURL = preset.BaseURL
	}
	if strings.TrimSpace(opt.APIKeyEnv) == "" {
		opt.APIKeyEnv = preset.APIKeyEnv
	}
	if strings.TrimSpace(opt.Model) == "" && len(preset.SuggestedModels) > 0 {
		opt.Model = preset.SuggestedModels[0]
	}
	return nil
}

func normalizeSetupOptions(opt *SetupOptions) {
	opt.Provider = strings.TrimSpace(opt.Provider)
	opt.BaseURL = strings.TrimRight(strings.TrimSpace(opt.BaseURL), "/")
	opt.APIKeyEnv = strings.TrimSpace(opt.APIKeyEnv)
	opt.Model = strings.TrimSpace(opt.Model)
	opt.Purpose = strings.TrimSpace(opt.Purpose)
	if opt.Purpose == "" {
		opt.Purpose = defaultPurpose
	}
	if opt.MaxOutputTokens <= 0 {
		opt.MaxOutputTokens = defaultMaxOutputTokens
	}
}

func validateSetupOptions(opt SetupOptions) error {
	if opt.Provider == "" {
		return errors.New("provider name is required")
	}
	if opt.BaseURL == "" {
		return errors.New("base_url is required")
	}
	if opt.APIKeyEnv == "" {
		return errors.New("api_key_env is required")
	}
	if opt.Model == "" {
		return errors.New("model name is required")
	}
	return nil
}

func validateModels(models config.ModelsConfig) error {
	if len(models.Providers) == 0 {
		return errors.New("models.yaml must define at least one provider")
	}
	if strings.TrimSpace(models.Routing.DefaultModel) == "" {
		return errors.New("models.yaml routing.default_model is required")
	}
	if _, ok := FindProviderForModel(models, models.Routing.DefaultModel); !ok {
		return fmt.Errorf("models.yaml default model %q is not defined by any provider", models.Routing.DefaultModel)
	}
	for _, provider := range models.Providers {
		if strings.TrimSpace(provider.Name) == "" {
			return errors.New("models.yaml provider name is required")
		}
		if provider.Type != providerTypeOpenAICompatible && provider.Type != providerTypeMimo {
			return fmt.Errorf("models.yaml provider %q type must be 'openai-compatible' or 'mimo', got %q", provider.Name, provider.Type)
		}
		if strings.TrimSpace(provider.BaseURL) == "" {
			return fmt.Errorf("models.yaml provider %q base_url is required", provider.Name)
		}
		if len(provider.Models) == 0 {
			return fmt.Errorf("models.yaml provider %q must define at least one model", provider.Name)
		}
	}
	return nil
}

func findProviderIndex(models config.ModelsConfig, name string) int {
	for i, provider := range models.Providers {
		if provider.Name == name {
			return i
		}
	}
	return -1
}

func findModelIndex(provider config.ProviderConfig, modelName string) int {
	for i, model := range provider.Models {
		if model.Name == modelName {
			return i
		}
	}
	return -1
}

func setDefaultModel(models *config.ModelsConfig, providerName, modelName string) {
	models.Routing.DefaultModel = modelName
	entry := config.FallbackEntry{Provider: providerName, Model: modelName}
	if len(models.Routing.FallbackChain) == 0 {
		models.Routing.FallbackChain = []config.FallbackEntry{entry}
		return
	}
	models.Routing.FallbackChain[0] = entry
}

func filterFallback(chain []config.FallbackEntry, providerName, modelName string) []config.FallbackEntry {
	filtered := chain[:0]
	for _, entry := range chain {
		if providerName != "" && entry.Provider == providerName {
			continue
		}
		if modelName != "" && entry.Model == modelName {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func resolveProviderConnection(models config.ModelsConfig, providerName, modelName, baseURL, apiKeyEnv string) (string, string, error) {
	baseURL = strings.TrimSpace(baseURL)
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	if baseURL != "" && apiKeyEnv != "" {
		return strings.TrimRight(baseURL, "/"), apiKeyEnv, nil
	}
	providerName, _, err := resolveProviderAndModel(models, providerName, modelName)
	if err != nil {
		return "", "", err
	}
	idx := findProviderIndex(models, providerName)
	if idx < 0 {
		return "", "", fmt.Errorf("provider %q is not configured", providerName)
	}
	provider := models.Providers[idx]
	if baseURL == "" {
		baseURL = provider.BaseURL
	}
	if apiKeyEnv == "" {
		apiKeyEnv = provider.APIKeyEnv
	}
	if baseURL == "" {
		return "", "", errors.New("base_url is required")
	}
	if apiKeyEnv == "" {
		return "", "", errors.New("api_key_env is required")
	}
	return strings.TrimRight(baseURL, "/"), apiKeyEnv, nil
}

func resolveProviderAndModel(models config.ModelsConfig, providerName, modelName string) (string, string, error) {
	providerName = strings.TrimSpace(providerName)
	modelName = strings.TrimSpace(modelName)
	if providerName != "" {
		idx := findProviderIndex(models, providerName)
		if idx < 0 {
			return "", "", fmt.Errorf("provider %q is not configured", providerName)
		}
		provider := models.Providers[idx]
		if modelName == "" {
			if models.Routing.DefaultModel != "" && findModelIndex(provider, models.Routing.DefaultModel) >= 0 {
				modelName = models.Routing.DefaultModel
			} else if len(provider.Models) > 0 {
				modelName = provider.Models[0].Name
			}
		}
		if findModelIndex(provider, modelName) < 0 {
			return "", "", fmt.Errorf("model %q is not configured for provider %q", modelName, providerName)
		}
		return providerName, modelName, nil
	}
	if modelName != "" {
		foundProvider, ok := FindProviderForModel(models, modelName)
		if !ok {
			return "", "", fmt.Errorf("model %q is not configured; run model setup or model discover first", modelName)
		}
		return foundProvider, modelName, nil
	}
	if models.Routing.DefaultModel == "" {
		return "", "", errors.New("default_model is not configured")
	}
	foundProvider, ok := FindProviderForModel(models, models.Routing.DefaultModel)
	if !ok {
		return "", "", fmt.Errorf("default_model %q is not configured by any provider", models.Routing.DefaultModel)
	}
	return foundProvider, models.Routing.DefaultModel, nil
}

func resolveAPIKey(envVar string) (string, error) {
	if strings.TrimSpace(envVar) == "" {
		return "", errors.New("api_key_env is required")
	}
	key := strings.TrimSpace(os.Getenv(envVar))
	if key == "" {
		key = strings.TrimSpace(auth.GetAPIKeyForEnv(envVar))
	}
	if key == "" {
		return "", fmt.Errorf("API key not found in environment variable %s", envVar)
	}
	if pathutil.APIKeyLooksPlaceholder(key) {
		return "", fmt.Errorf("API key in environment variable %s appears to be a placeholder; set a real key", envVar)
	}
	return key, nil
}

func resolveProviderType(models config.ModelsConfig, providerName string) string {
	providerName = strings.TrimSpace(providerName)
	if idx := findProviderIndex(models, providerName); idx >= 0 {
		providerType := strings.TrimSpace(models.Providers[idx].Type)
		if providerType != "" {
			return providerType
		}
	}
	if providerName == providerTypeMimo {
		return providerTypeMimo
	}
	return providerTypeOpenAICompatible
}

func marshalChatRequest(providerType, modelName string, messages []map[string]string, maxTokens int, temperature float64, stream bool) ([]byte, error) {
	body := map[string]any{
		"model":       modelName,
		"messages":    messages,
		"temperature": temperature,
	}
	if stream {
		body["stream"] = true
	}
	if providerType == providerTypeMimo {
		body["max_completion_tokens"] = maxTokens
	} else {
		body["max_tokens"] = maxTokens
	}
	return json.Marshal(body)
}

func setProviderAuthHeader(req *http.Request, providerType, apiKey string) {
	if providerType == providerTypeMimo {
		req.Header.Set("api-key", apiKey)
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
}

func firstChoiceContent(choices []chatAPIChoice) string {
	if len(choices) == 0 {
		return ""
	}
	choice := choices[0]
	if choice.Message != nil && choice.Message.Content != "" {
		return choice.Message.Content
	}
	if choice.Delta != nil && choice.Delta.Content != "" {
		return choice.Delta.Content
	}
	return ""
}

func limitDisplay(s string) string {
	return limitDisplayBytes(s, maxDisplayedResponseBytes)
}

func limitDisplayBytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "...<truncated>"
}

func isSecretBoundary(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t' || b == '"' || b == '\'' || b == ',' || b == ';'
}

func redactTokenLike(text, token string) string {
	var out strings.Builder
	upper := strings.ToUpper(text)
	searchFrom := 0
	for {
		idx := strings.Index(upper[searchFrom:], token)
		if idx < 0 {
			out.WriteString(text[searchFrom:])
			return out.String()
		}
		idx += searchFrom
		valueStart := idx + len(token)
		for valueStart < len(text) && (text[valueStart] == ' ' || text[valueStart] == '\t') {
			valueStart++
		}
		if valueStart >= len(text) || (text[valueStart] != '=' && text[valueStart] != ':') {
			out.WriteString(text[searchFrom : idx+len(token)])
			searchFrom = idx + len(token)
			continue
		}
		valueStart++
		for valueStart < len(text) && (text[valueStart] == ' ' || text[valueStart] == '\t' || text[valueStart] == '"' || text[valueStart] == '\'') {
			valueStart++
		}
		end := valueStart
		for end < len(text) && !isSecretBoundary(text[end]) {
			end++
		}
		out.WriteString(text[searchFrom : idx+len(token)])
		out.WriteString("=<redacted>")
		searchFrom = end
	}
}
