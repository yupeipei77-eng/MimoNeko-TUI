package modelprofile

import (
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

	"github.com/reasonforge/reasonforge/internal/config"
	"gopkg.in/yaml.v3"
)

const (
	providerTypeOpenAICompatible = "openai-compatible"
	defaultPurpose               = "coding"
	defaultMaxOutputTokens       = 4096
	maxResponseBodyBytes         = 1 << 20
	maxDisplayedResponseBytes    = 256
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
	Status    string
	LatencyMs int64
	Response  string
	Error     string
}

// RemoveOptions selects a provider or model to remove.
type RemoveOptions struct {
	Provider string
	Model    string
}

var presets = map[string]Preset{
	"mimo": {
		Name:      "mimo",
		Type:      providerTypeOpenAICompatible,
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

// ModelsPath returns the models.yaml path for a ReasonForge root.
func ModelsPath(root string) string {
	return filepath.Join(config.ConfigDir(root), "models.yaml")
}

// Load reads only .reasonforge/models.yaml.
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

// Save writes only .reasonforge/models.yaml.
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
		Type:      providerTypeOpenAICompatible,
		BaseURL:   strings.TrimRight(opt.BaseURL, "/"),
		APIKeyEnv: opt.APIKeyEnv,
	}
	if providerIndex >= 0 {
		provider = models.Providers[providerIndex]
		provider.Type = providerTypeOpenAICompatible
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
		provider.Models[modelIndex] = model
	} else {
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
	models, err := Load(root)
	if err != nil && strings.TrimSpace(opt.BaseURL) == "" {
		return nil, err
	}
	baseURL, apiKeyEnv, err := resolveProviderConnection(models, opt.Provider, "", opt.BaseURL, opt.APIKeyEnv)
	if err != nil {
		return nil, err
	}
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
	req.Header.Set("Authorization", "Bearer "+apiKey)

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
	models, err := Load(root)
	if err != nil && strings.TrimSpace(opt.BaseURL) == "" {
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

	body, err := json.Marshal(map[string]any{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  16,
		"temperature": 0,
	})
	if err != nil {
		return TestResult{}, fmt.Errorf("marshal request: %w", err)
	}

	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return TestResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	latency := time.Since(started).Milliseconds()
	result := TestResult{Provider: providerName, Model: modelName, LatencyMs: latency}
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

	var parsed struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		result.Status = "failed"
		result.Error = "parse response failed"
		return result, nil
	}
	if parsed.Model != "" {
		result.Model = parsed.Model
	}
	if len(parsed.Choices) > 0 {
		result.Response = limitDisplay(SanitizeText(parsed.Choices[0].Message.Content, apiKey))
	}
	result.Status = "ok"
	return result, nil
}

// APIKeyStatus checks whether an API key environment variable is configured.
func APIKeyStatus(envVar string) string {
	if strings.TrimSpace(envVar) == "" {
		return "missing"
	}
	if strings.TrimSpace(os.Getenv(envVar)) == "" {
		return "missing"
	}
	return "configured"
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
		if provider.Type != providerTypeOpenAICompatible {
			return fmt.Errorf("models.yaml provider %q must be openai-compatible", provider.Name)
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
		return "", fmt.Errorf("API key not found in environment variable %s", envVar)
	}
	return key, nil
}

func limitDisplay(s string) string {
	if len(s) <= maxDisplayedResponseBytes {
		return s
	}
	return s[:maxDisplayedResponseBytes] + "...<truncated>"
}

func isSecretBoundary(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t' || b == '"' || b == '\'' || b == ',' || b == ';'
}

func redactTokenLike(text, token string) string {
	upper := strings.ToUpper(text)
	idx := strings.Index(upper, token)
	if idx < 0 {
		return text
	}
	end := idx + len(token)
	for end < len(text) && !isSecretBoundary(text[end]) {
		end++
	}
	return text[:idx] + token + "<redacted>" + text[end:]
}
