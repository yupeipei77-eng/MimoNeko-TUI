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

	fmt.Fprintln(env.Stdout, "欢迎使用 MioNeko")
	fmt.Fprintln(env.Stdout, "检测到你还没有配置模型")
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "请选择：")
	fmt.Fprintln(env.Stdout, "1. MiMo")
	fmt.Fprintln(env.Stdout, "2. OpenAI-compatible")
	fmt.Fprintln(env.Stdout, "3. Local")
	fmt.Fprintln(env.Stdout)

	providerChoice := promptOnboardingLine(reader, env, "请选择 [1]", "1")
	provider := normalizeOnboardingProvider(providerChoice)
	if provider == "" {
		fmt.Fprintln(env.Stderr, "不支持的 Provider 选择")
		return 1
	}

	apiKey := strings.TrimSpace(promptOnboardingLine(reader, env, "API Key", ""))
	if apiKey == "" && provider == "local" {
		apiKey = "local"
	}
	if apiKey == "" {
		fmt.Fprintln(env.Stderr, "API Key 不能为空")
		return 1
	}
	if pathutil.APIKeyLooksPlaceholder(apiKey) {
		fmt.Fprintln(env.Stderr, "API Key 看起来是示例占位值，请输入真实 API Key")
		return 1
	}

	baseURL := promptOnboardingLine(reader, env, "Base URL", auth.GetBaseURL(provider))
	model := promptOnboardingLine(reader, env, "Model", auth.DefaultModel(provider))

	userConfig, err := auth.LoadUserConfig()
	if err != nil {
		fmt.Fprintf(env.Stderr, "加载配置失败: %v\n", err)
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
		fmt.Fprintf(env.Stderr, "保存配置失败: %v\n", err)
		return 1
	}
	if err := auth.ApplyUserConfigToEnv(); err != nil {
		fmt.Fprintf(env.Stderr, "加载用户配置失败: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "\n配置已保存到 %s\n", auth.GetUserConfigPath())
	fmt.Fprintln(env.Stdout, "正在执行: mimoneko model test")

	result, err := modelprofile.Test(context.Background(), ".", modelprofile.TestOptions{
		Provider:  provider,
		Model:     model,
		BaseURL:   baseURL,
		APIKeyEnv: auth.APIKeyEnv(provider),
		Prompt:    "Reply with OK only.",
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "model test failed: %s\n", modelprofile.SanitizeText(err.Error()))
		return 1
	}
	if result.Status != "ok" {
		fmt.Fprintf(env.Stderr, "model test failed: %s\n", modelprofile.SanitizeText(result.Error))
		return 1
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "配置成功")
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
