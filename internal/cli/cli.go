package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mimoneko/mimoneko/internal/auth"
)

func Run(args []string, env Env) int {
	if env.Stdout == nil {
		env.Stdout = io.Discard
	}
	if env.Stderr == nil {
		env.Stderr = io.Discard
	}
	if env.Stdin == nil {
		env.Stdin = os.Stdin
	}
	if env.Getwd == nil {
		env.Getwd = func() (string, error) { return ".", nil }
	}

	// Check if this is a command that needs API key
	needsAPIKey := len(args) == 0 || (len(args) > 0 && args[0] == "run")

	// If no args, show help and check config
	if len(args) == 0 {
		printUsage(env.Stdout)
		fmt.Fprintln(env.Stdout)

		// Check if API key is configured
		if !hasAPIKeyConfigured() {
			fmt.Fprintln(env.Stdout, "未检测到 API Key 配置")
			fmt.Fprintln(env.Stdout)
			if promptYesNo(env, "是否现在配置? [Y/n]") {
				return runFirstTimeSetup(env)
			}
			fmt.Fprintln(env.Stdout)
			fmt.Fprintln(env.Stdout, "提示: 运行 'mimoneko auth login' 进行配置")
		}
		return 0
	}

	// For 'run' command, check if API key is configured
	if needsAPIKey && args[0] == "run" {
		if !hasAPIKeyConfigured() {
			fmt.Fprintln(env.Stdout, "未检测到 API Key 配置")
			fmt.Fprintln(env.Stdout)
			if promptYesNo(env, "是否现在配置? [Y/n]") {
				result := runFirstTimeSetup(env)
				if result != 0 {
					return result
				}
				fmt.Fprintln(env.Stdout)
				fmt.Fprintln(env.Stdout, "配置完成，继续执行任务...")
				fmt.Fprintln(env.Stdout)
			} else {
				fmt.Fprintln(env.Stdout)
				fmt.Fprintln(env.Stdout, "提示: 运行 'mimoneko auth login' 进行配置")
				return 1
			}
		}
	}

	return commands.Dispatch(args, env)
}

// hasAPIKeyConfigured checks if any API key is configured
func hasAPIKeyConfigured() bool {
	// Check environment variables
	if os.Getenv("MIMO_API_KEY") != "" {
		return true
	}
	if os.Getenv("MIMONEKO_API_KEY") != "" {
		return true
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return true
	}

	// Check user config
	config, err := auth.LoadUserConfig()
	if err != nil {
		return false
	}

	return config.Auth.DefaultProvider != ""
}

// promptYesNo prompts the user for yes/no input
func promptYesNo(env Env, prompt string) bool {
	fmt.Fprintf(env.Stdout, "%s ", prompt)
	reader := bufio.NewReader(env.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

// runFirstTimeSetup runs the first-time setup wizard
func runFirstTimeSetup(env Env) int {
	fmt.Fprintln(env.Stdout, "╔══════════════════════════════════════════════════════════════╗")
	fmt.Fprintln(env.Stdout, "║                    MioNeko v0.1.0-beta                       ║")
	fmt.Fprintln(env.Stdout, "║            专为 MiMo 大模型而生的 Agent AI 编程工具           ║")
	fmt.Fprintln(env.Stdout, "╚══════════════════════════════════════════════════════════════╝")
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "首次配置向导")
	fmt.Fprintln(env.Stdout)

	// Select provider
	provider := auth.PromptSelect("选择 Provider", []string{"MiMo (推荐)", "OpenAI-compatible"})
	switch provider {
	case "MiMo (推荐)", "mimo":
		provider = "mimo"
	case "OpenAI-compatible", "openai":
		provider = "openai"
	}

	// Get API Key
	apiKey := auth.PromptPassword("输入 API Key")
	if apiKey == "" {
		fmt.Fprintln(env.Stderr, "\n✗ API Key 不能为空")
		return 1
	}

	// Get Base URL
	defaultBaseURL := auth.GetBaseURL(provider)
	baseURL := auth.PromptInput("Base URL", defaultBaseURL)

	// Get model
	defaultModel := "mimo-v2.5-pro"
	if provider == "openai" {
		defaultModel = "gpt-4"
	}
	model := auth.PromptInput("选择模型", defaultModel)

	// Save config
	config, err := auth.LoadUserConfig()
	if err != nil {
		fmt.Fprintf(env.Stderr, "\n✗ 加载配置失败: %v\n", err)
		return 1
	}

	if config.Auth.Providers == nil {
		config.Auth.Providers = make(map[string]auth.ProviderConfig)
	}

	config.Auth.Providers[provider] = auth.ProviderConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}
	config.Auth.DefaultProvider = provider
	config.Preferences.DefaultModel = model

	if err := auth.SaveUserConfig(config); err != nil {
		fmt.Fprintf(env.Stderr, "\n✗ 保存配置失败: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "\n✓ 配置已保存到 %s\n", auth.GetUserConfigPath())
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "下一步:")
	fmt.Fprintln(env.Stdout, "  mimoneko model test      # 测试模型连接")
	fmt.Fprintln(env.Stdout, "  mimoneko run \"你的任务\"  # 运行任务")

	return 0
}
