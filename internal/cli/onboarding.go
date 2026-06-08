package cli

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/auth"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/modelprofile"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/pathutil"
)

func runFirstTimeSetup(env Env) int {
	reader := bufio.NewReader(env.Stdin)
	ui := newCLIUI()

	ui.PrintHeader(env.Stdout, "Welcome to MimoNeko")
	fmt.Fprintln(env.Stdout, "MiMo-first AI Coding Agent")
	fmt.Fprintln(env.Stdout, "Fast. Safe. Cache-aware.")
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "No model configuration was found.")
	fmt.Fprintln(env.Stdout, "I will guide you through 3 setup steps:")
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "1. Select a model provider")
	fmt.Fprintln(env.Stdout, "2. Enter an API Key")
	fmt.Fprintln(env.Stdout, "3. Test the connection")
	fmt.Fprintln(env.Stdout)
	PrintKV(env.Stdout, "Recommended defaults:", []KV{
		{Key: "Provider", Value: "MiMo"},
		{Key: "Model", Value: auth.DefaultModel("mimo")},
		{Key: "Base URL", Value: auth.GetBaseURL("mimo")},
	})
	fmt.Fprintln(env.Stdout)

	PrintStep(env.Stdout, 1, 3, "Select Provider")
	provider, err := promptOnboardingProvider(reader, env)
	if err != nil {
		return handleOnboardingPromptError(env, err)
	}
	fmt.Fprintln(env.Stdout)

	PrintStep(env.Stdout, 2, 3, "Configure API Key")
	apiKey, err := promptOnboardingSecret(reader, env, "API Key")
	if err != nil {
		return handleOnboardingPromptError(env, err)
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" && provider == "local" {
		apiKey = "local"
	}
	if apiKey == "" {
		PrintError(env.Stderr, "Setup failed", "API Key is required.", "Run: mimoneko auth login")
		return 1
	}
	if pathutil.APIKeyLooksPlaceholder(apiKey) {
		PrintError(env.Stderr, "Setup failed", "API Key looks like a sample placeholder.", "Enter a real API Key.")
		return 1
	}

	baseURL, err := promptOnboardingInput(reader, env, "Base URL", auth.GetBaseURL(provider))
	if err != nil {
		return handleOnboardingPromptError(env, err)
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = auth.GetBaseURL(provider)
	}

	model, err := promptOnboardingModel(reader, env, provider)
	if err != nil {
		return handleOnboardingPromptError(env, err)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = auth.DefaultModel(provider)
	}
	fmt.Fprintf(env.Stdout, "Saved key: %s\n\n", MaskSecret(apiKey))

	userConfig, err := auth.LoadUserConfig()
	if err != nil {
		PrintErrorDetails(env.Stderr, "Setup failed", "Could not load user config.", "Check user directory permissions and try again.", modelprofile.SanitizeText(err.Error(), apiKey))
		return 1
	}
	if userConfig.Auth.Providers == nil {
		userConfig.Auth.Providers = make(map[string]auth.ProviderConfig)
	}
	userConfig.Auth.Providers[provider] = auth.ProviderConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}
	userConfig.Auth.DefaultProvider = provider
	userConfig.Preferences.DefaultModel = model

	if err := auth.SaveUserConfig(userConfig); err != nil {
		PrintErrorDetails(env.Stderr, "Setup failed", "Could not save user config.", "Check user directory permissions and try again.", modelprofile.SanitizeText(err.Error(), apiKey))
		return 1
	}
	if err := auth.ApplyUserConfigToEnv(); err != nil {
		PrintErrorDetails(env.Stderr, "Setup failed", "Could not load user config.", "Check the saved user config and try again.", modelprofile.SanitizeText(err.Error(), apiKey))
		return 1
	}

	PrintSuccess(env.Stdout, fmt.Sprintf("Configuration saved to %s", auth.GetUserConfigPath()))
	fmt.Fprintln(env.Stdout)
	PrintStep(env.Stdout, 3, 3, "Test Connection")

	result, err := modelprofile.Test(context.Background(), ".", modelprofile.TestOptions{
		Provider:  provider,
		Model:     model,
		BaseURL:   baseURL,
		APIKeyEnv: auth.APIKeyEnv(provider),
		Prompt:    "Reply with OK only.",
	})
	if err != nil {
		reason, suggestion, details := friendlyModelError(modelprofile.SanitizeText(err.Error(), apiKey))
		PrintErrorDetails(env.Stderr, "Connection failed", reason, suggestion, details)
		return 1
	}
	if result.Status != "ok" {
		reason, suggestion, details := friendlyModelError(modelprofile.SanitizeText(result.Error, apiKey))
		PrintErrorDetails(env.Stderr, "Connection failed", reason, suggestion, details)
		return 1
	}

	fmt.Fprintln(env.Stdout)
	PrintSuccess(env.Stdout, "Configuration Complete")
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "Provider: %s\n", displayOnboardingProvider(provider))
	fmt.Fprintf(env.Stdout, "Model: %s\n", model)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "Press Enter to continue")
	waitForOnboardingEnter(reader)
	return 0
}

func promptOnboardingLine(reader *bufio.Reader, env Env, prompt string, defaultValue string) string {
	if defaultValue == "" {
		fmt.Fprintf(env.Stdout, "%s: ", prompt)
	} else {
		fmt.Fprintf(env.Stdout, "%s [%s]: ", prompt, defaultValue)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

func normalizeOnboardingProvider(choice string) string {
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "", "1", "mimo", "mimo (recommended)", "mimo recommended":
		return "mimo"
	case "2", "openai", "openai-compatible", "openai compatible":
		return "openai"
	case "3", "local":
		return "local"
	default:
		return ""
	}
}
