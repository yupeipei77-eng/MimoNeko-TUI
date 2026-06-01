package cli

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/mimoneko/mimoneko/internal/auth"
	"github.com/mimoneko/mimoneko/internal/modelprofile"
	"github.com/mimoneko/mimoneko/internal/pathutil"
)

func runFirstTimeSetup(env Env) int {
	reader := bufio.NewReader(env.Stdin)
	ui := newCLIUI()

	ui.PrintHeader(env.Stdout, "Welcome to MioNeko")
	fmt.Fprintln(env.Stdout, "MiMo-first AI Coding Agent")
	fmt.Fprintln(env.Stdout, "Fast. Safe. Cache-aware.")
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "检测到你还没有配置模型。")
	fmt.Fprintln(env.Stdout, "我会引导你完成 3 步配置：")
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "1. 选择模型服务")
	fmt.Fprintln(env.Stdout, "2. 输入 API Key")
	fmt.Fprintln(env.Stdout, "3. 测试连接")
	fmt.Fprintln(env.Stdout)
	PrintKV(env.Stdout, "默认推荐：", []KV{
		{Key: "Provider", Value: "MiMo"},
		{Key: "Model", Value: auth.DefaultModel("mimo")},
		{Key: "Base URL", Value: auth.GetBaseURL("mimo")},
	})
	fmt.Fprintln(env.Stdout)

	PrintStep(env.Stdout, 1, 3, "选择 Provider")
	fmt.Fprintln(env.Stdout, "1. MiMo (recommended)")
	fmt.Fprintln(env.Stdout, "2. OpenAI-compatible")
	fmt.Fprintln(env.Stdout, "3. Local")
	providerChoice := promptOnboardingLine(reader, env, "请选择 [1]", "1")
	provider := normalizeOnboardingProvider(providerChoice)
	if provider == "" {
		PrintError(env.Stderr, "Setup failed", "不支持的 Provider 选择。", "请选择 1、2 或 3。")
		return 1
	}
	fmt.Fprintln(env.Stdout)

	PrintStep(env.Stdout, 2, 3, "配置 API Key")
	apiKey := strings.TrimSpace(promptSecretLine(reader, env, "API Key"))
	if apiKey == "" && provider == "local" {
		apiKey = "local"
	}
	if apiKey == "" {
		PrintError(env.Stderr, "Setup failed", "API Key 不能为空。", "重新运行: mimoneko auth login")
		return 1
	}
	if pathutil.APIKeyLooksPlaceholder(apiKey) {
		PrintError(env.Stderr, "Setup failed", "API Key 看起来是示例占位值。", "请输入真实 API Key。")
		return 1
	}

	baseURL := promptOnboardingLine(reader, env, "Base URL", auth.GetBaseURL(provider))
	model := promptOnboardingLine(reader, env, "Model", auth.DefaultModel(provider))
	fmt.Fprintf(env.Stdout, "Saved key: %s\n\n", MaskSecret(apiKey))

	userConfig, err := auth.LoadUserConfig()
	if err != nil {
		PrintErrorDetails(env.Stderr, "Setup failed", "加载配置失败。", "检查用户目录权限后重试。", modelprofile.SanitizeText(err.Error(), apiKey))
		return 1
	}
	if userConfig.Auth.Providers == nil {
		userConfig.Auth.Providers = make(map[string]auth.ProviderConfig)
	}
	userConfig.Auth.Providers[provider] = auth.ProviderConfig{
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
	}
	userConfig.Auth.DefaultProvider = provider
	userConfig.Preferences.DefaultModel = model

	if err := auth.SaveUserConfig(userConfig); err != nil {
		PrintErrorDetails(env.Stderr, "Setup failed", "保存配置失败。", "检查用户目录权限后重试。", modelprofile.SanitizeText(err.Error(), apiKey))
		return 1
	}
	if err := auth.ApplyUserConfigToEnv(); err != nil {
		PrintErrorDetails(env.Stderr, "Setup failed", "加载用户配置失败。", "检查用户配置后重试。", modelprofile.SanitizeText(err.Error(), apiKey))
		return 1
	}

	PrintSuccess(env.Stdout, fmt.Sprintf("配置已保存到 %s", auth.GetUserConfigPath()))
	fmt.Fprintln(env.Stdout)
	PrintStep(env.Stdout, 3, 3, "测试连接")

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
	PrintSuccess(env.Stdout, "配置成功")
	fmt.Fprintln(env.Stdout, "你现在可以运行：")
	fmt.Fprintln(env.Stdout, "mimoneko \"修改 README\"")
	fmt.Fprintln(env.Stdout, "或：")
	fmt.Fprintln(env.Stdout, "mimoneko run \"修改 README\"")
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
	case "", "1", "mimo", "mimo (recommended)", "mimo (推荐)", "mimo 推荐":
		return "mimo"
	case "2", "openai", "openai-compatible", "openai compatible":
		return "openai"
	case "3", "local":
		return "local"
	default:
		return ""
	}
}
