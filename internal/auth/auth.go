package auth

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/pathutil"
	"gopkg.in/yaml.v3"
)

// Config represents the user-level configuration
type Config struct {
	Auth        AuthConfig        `yaml:"auth"`
	Preferences PreferencesConfig `yaml:"preferences"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Providers       map[string]ProviderConfig `yaml:"providers"`
	DefaultProvider string                    `yaml:"default_provider"`
}

// ProviderConfig represents a provider's configuration
type ProviderConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

// PreferencesConfig represents user preferences
type PreferencesConfig struct {
	DefaultModel string `yaml:"default_model"`
	DryRun       bool   `yaml:"dry_run"`
	Worktree     bool   `yaml:"worktree"`
}

// GetUserConfigDir returns the user-level config directory
func GetUserConfigDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("USERPROFILE"), ".mimoneko")
	}
	return filepath.Join(os.Getenv("HOME"), ".mimoneko")
}

// GetUserConfigPath returns the path to the user config file
func GetUserConfigPath() string {
	return filepath.Join(GetUserConfigDir(), "config.yaml")
}

// LoadUserConfig loads the user-level configuration
func LoadUserConfig() (*Config, error) {
	path := GetUserConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &config, nil
}

// SaveUserConfig saves the user-level configuration
func SaveUserConfig(config *Config) error {
	dir := GetUserConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	path := GetUserConfigPath()
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// GetAPIKey returns the API key for the given provider
func GetAPIKey(provider string) string {
	// 1. Check environment variables
	if key := os.Getenv("MIMO_API_KEY"); key != "" && provider == "mimo" && !pathutil.APIKeyLooksPlaceholder(key) {
		return key
	}
	if key := os.Getenv("MIMONEKO_API_KEY"); key != "" && !pathutil.APIKeyLooksPlaceholder(key) {
		return key
	}
	if key := os.Getenv("MimoNeko_API_KEY"); key != "" && !pathutil.APIKeyLooksPlaceholder(key) {
		return key
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" && provider == "openai" && !pathutil.APIKeyLooksPlaceholder(key) {
		return key
	}

	// 2. Check user config
	config, err := LoadUserConfig()
	if err != nil {
		return ""
	}

	if p, ok := config.Auth.Providers[provider]; ok {
		return p.APIKey
	}

	return ""
}

// GetAPIKeyForEnv returns the user-configured API key for a provider whose
// standard environment variable is envVar.
func GetAPIKeyForEnv(envVar string) string {
	envVar = strings.TrimSpace(envVar)
	if envVar == "" {
		return ""
	}
	userConfig, err := LoadUserConfig()
	if err != nil {
		return ""
	}
	for provider, providerConfig := range userConfig.Auth.Providers {
		if APIKeyEnv(provider) == envVar && strings.TrimSpace(providerConfig.APIKey) != "" {
			return providerConfig.APIKey
		}
	}
	return ""
}

// GetBaseURL returns the base URL for the given provider
func GetBaseURL(provider string) string {
	// 1. Check environment variables
	if url := os.Getenv("MIMO_BASE_URL"); url != "" && provider == "mimo" {
		return url
	}

	// 2. Check user config
	config, err := LoadUserConfig()
	if err != nil {
		return getDefaultBaseURL(provider)
	}

	if p, ok := config.Auth.Providers[provider]; ok && p.BaseURL != "" {
		return p.BaseURL
	}

	return getDefaultBaseURL(provider)
}

// getDefaultBaseURL returns the default base URL for a provider
func getDefaultBaseURL(provider string) string {
	switch provider {
	case "mimo":
		return "https://token-plan-cn.xiaomimimo.com/v1"
	case "openai":
		return "https://api.openai.com/v1"
	case "local":
		return "http://127.0.0.1:11434/v1"
	default:
		return ""
	}
}

// APIKeyEnv returns the conventional process environment variable for a provider.
func APIKeyEnv(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "mimo":
		return "MIMO_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	case "local":
		return "MIMONEKO_LOCAL_API_KEY"
	default:
		name := strings.ToUpper(strings.TrimSpace(provider))
		if name == "" {
			return ""
		}
		name = strings.NewReplacer("-", "_", ".", "_").Replace(name)
		return name + "_API_KEY"
	}
}

// ProviderType returns the project model provider type for a user-facing provider.
func ProviderType(provider string) string {
	if strings.ToLower(strings.TrimSpace(provider)) == "mimo" {
		return "mimo"
	}
	return "openai-compatible"
}

// DefaultModel returns the default model for a provider.
func DefaultModel(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "mimo":
		return "mimo-v2.5-pro"
	case "openai":
		return "gpt-4o-mini"
	case "local":
		return "local-coder"
	default:
		return "mimo-v2.5-pro"
	}
}

// UserModelsConfig converts the user-level auth config into a model profile.
func UserModelsConfig() (config.ModelsConfig, bool, error) {
	userConfig, err := LoadUserConfig()
	if err != nil {
		return config.ModelsConfig{}, false, err
	}
	provider := strings.TrimSpace(userConfig.Auth.DefaultProvider)
	if provider == "" {
		return config.ModelsConfig{}, false, nil
	}
	providerConfig, ok := userConfig.Auth.Providers[provider]
	if !ok {
		return config.ModelsConfig{}, false, nil
	}
	model := strings.TrimSpace(userConfig.Preferences.DefaultModel)
	if model == "" {
		model = DefaultModel(provider)
	}
	baseURL := strings.TrimSpace(providerConfig.BaseURL)
	if baseURL == "" {
		baseURL = GetBaseURL(provider)
	}
	models := config.ModelsConfig{
		Providers: []config.ProviderConfig{
			{
				Name:      provider,
				Type:      ProviderType(provider),
				BaseURL:   strings.TrimRight(baseURL, "/"),
				APIKeyEnv: APIKeyEnv(provider),
				Models: []config.ModelConfig{
					{
						Name:                model,
						Purpose:             "coding",
						MaxOutputTokens:     4096,
						SupportsPrefixCache: false,
					},
				},
			},
		},
		Routing: config.RoutingConfig{
			DefaultModel: model,
			FallbackChain: []config.FallbackEntry{
				{Provider: provider, Model: model},
			},
		},
	}
	return models, true, nil
}

// ApplyUserConfigToEnv makes saved user-level keys available to provider code
// that resolves keys through env var names. It updates only the current process.
func ApplyUserConfigToEnv() error {
	userConfig, err := LoadUserConfig()
	if err != nil {
		return err
	}
	for provider, providerConfig := range userConfig.Auth.Providers {
		envVar := APIKeyEnv(provider)
		key := strings.TrimSpace(providerConfig.APIKey)
		if envVar == "" || key == "" {
			continue
		}
		current := strings.TrimSpace(os.Getenv(envVar))
		if current == "" || pathutil.APIKeyLooksPlaceholder(current) {
			if err := os.Setenv(envVar, key); err != nil {
				return err
			}
		}
	}
	return nil
}

// SanitizeAPIKey returns a sanitized version of the API key
func SanitizeAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// PromptInput prompts the user for input
func PromptInput(prompt string, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" && defaultValue != "" {
		return defaultValue
	}
	return input
}

// PromptPassword prompts the user for a password (no echo)
func PromptPassword(prompt string) string {
	fmt.Printf("%s: ", prompt)
	// TODO: Implement password input without echo
	// For now, use regular input
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// PromptSelect prompts the user to select from options
func PromptSelect(prompt string, options []string) string {
	fmt.Printf("%s:\n", prompt)
	for i, option := range options {
		fmt.Printf("  %d) %s\n", i+1, option)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		for i, option := range options {
			if input == option || input == fmt.Sprintf("%d", i+1) {
				return option
			}
		}
		fmt.Println("Invalid selection. Please try again.")
	}
}
