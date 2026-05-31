package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mimoneko/mimoneko/internal/auth"
)

type AuthCommand struct{}

func (c *AuthCommand) Name() string { return "auth" }

func (c *AuthCommand) Run(args []string, env Env) int {
	if len(args) == 0 {
		printAuthHelp(env)
		return 0
	}

	switch args[0] {
	case "login":
		return c.runLogin(args[1:], env)
	case "status":
		return c.runStatus(args[1:], env)
	case "logout":
		return c.runLogout(args[1:], env)
	default:
		fmt.Fprintf(env.Stderr, "未知命令 '%s'\n\n", args[0])
		printAuthHelp(env)
		return 1
	}
}

func printAuthHelp(env Env) {
	fmt.Fprintln(env.Stdout, "用法: mimoneko auth <命令>")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "命令:")
	fmt.Fprintln(env.Stdout, "  login    交互式登录配置")
	fmt.Fprintln(env.Stdout, "  status   查看当前配置状态")
	fmt.Fprintln(env.Stdout, "  logout   清除配置")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "示例:")
	fmt.Fprintln(env.Stdout, "  mimoneko auth login")
	fmt.Fprintln(env.Stdout, "  mimoneko auth status")
}

func (c *AuthCommand) runLogin(args []string, env Env) int {
	fmt.Fprintln(env.Stdout, "╔══════════════════════════════════════════════════════════════╗")
	fmt.Fprintln(env.Stdout, "║                    MioNeko v0.1.0-beta                       ║")
	fmt.Fprintln(env.Stdout, "║            专为 MiMo 大模型而生的 Agent AI 编程工具           ║")
	fmt.Fprintln(env.Stdout, "╚══════════════════════════════════════════════════════════════╝")
	fmt.Fprintln(env.Stdout)

	// Select provider
	provider := auth.PromptSelect("选择 Provider", []string{"MiMo (推荐)", "OpenAI-compatible", "Local"})
	switch provider {
	case "MiMo (推荐)", "mimo":
		provider = "mimo"
	case "OpenAI-compatible", "openai":
		provider = "openai"
	case "Local", "local":
		provider = "local"
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

	// Test connection
	fmt.Fprint(env.Stdout, "✓ 正在测试连接...")

	client := &http.Client{Timeout: 30 * time.Second}
	started := time.Now()

	req, err := http.NewRequestWithContext(context.Background(), "GET", strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		fmt.Fprintf(env.Stdout, " 失败\n\n")
		fmt.Fprintf(env.Stderr, "✗ 连接失败: %v\n", err)
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	latency := time.Since(started).Milliseconds()

	if err != nil {
		fmt.Fprintf(env.Stdout, " 失败\n\n")
		fmt.Fprintf(env.Stderr, "✗ 连接失败: %v\n", err)
		fmt.Fprintln(env.Stderr, "  请检查网络连接")
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Fprintf(env.Stdout, " 成功 (延迟: %dms)\n", latency)
	} else {
		fmt.Fprintf(env.Stdout, " 失败\n\n")
		printHTTPError(env.Stderr, resp.StatusCode)
		return 1
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "下一步:")
	fmt.Fprintln(env.Stdout, "  mimoneko model test      # 测试模型连接")
	fmt.Fprintln(env.Stdout, "  mimoneko run \"你的任务\"  # 运行任务")

	return 0
}

func (c *AuthCommand) runStatus(args []string, env Env) int {
	config, err := auth.LoadUserConfig()
	if err != nil {
		fmt.Fprintf(env.Stderr, "✗ 加载配置失败: %v\n", err)
		return 1
	}

	if config.Auth.DefaultProvider == "" {
		fmt.Fprintln(env.Stdout, "未配置")
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, "运行 'mimoneko auth login' 进行配置")
		return 0
	}

	provider := config.Auth.DefaultProvider
	providerConfig, ok := config.Auth.Providers[provider]
	if !ok {
		fmt.Fprintf(env.Stdout, "✗ Provider %q 未配置\n", provider)
		return 1
	}

	fmt.Fprintln(env.Stdout, "当前配置:")
	fmt.Fprintf(env.Stdout, "  Provider: %s\n", provider)
	fmt.Fprintf(env.Stdout, "  Base URL: %s\n", providerConfig.BaseURL)
	fmt.Fprintf(env.Stdout, "  API Key: %s\n", auth.SanitizeAPIKey(providerConfig.APIKey))
	fmt.Fprintf(env.Stdout, "  Model: %s\n", config.Preferences.DefaultModel)

	// Test connection
	fmt.Fprint(env.Stdout, "  Status: ")

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), "GET", strings.TrimRight(providerConfig.BaseURL, "/")+"/models", nil)
	if err != nil {
		fmt.Fprintln(env.Stdout, "✗ 连接失败")
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+providerConfig.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(env.Stdout, "✗ 连接失败")
		fmt.Fprintln(env.Stderr, "  请检查网络连接")
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Fprintln(env.Stdout, "✓ 已连接")
	} else {
		fmt.Fprintln(env.Stdout, "✗ 连接失败")
		printHTTPError(env.Stderr, resp.StatusCode)
		return 1
	}

	return 0
}

func (c *AuthCommand) runLogout(args []string, env Env) int {
	config, err := auth.LoadUserConfig()
	if err != nil {
		fmt.Fprintf(env.Stderr, "✗ 加载配置失败: %v\n", err)
		return 1
	}

	if config.Auth.DefaultProvider == "" {
		fmt.Fprintln(env.Stdout, "未配置")
		return 0
	}

	config.Auth.Providers = nil
	config.Auth.DefaultProvider = ""

	if err := auth.SaveUserConfig(config); err != nil {
		fmt.Fprintf(env.Stderr, "✗ 保存配置失败: %v\n", err)
		return 1
	}

	fmt.Fprintln(env.Stdout, "✓ 已登出")
	return 0
}

// printHTTPError prints a user-friendly error message for HTTP status codes
func printHTTPError(w io.Writer, statusCode int) {
	switch statusCode {
	case 401:
		fmt.Fprintln(w, "✗ API Key 无效")
		fmt.Fprintln(w, "  请执行: mimoneko auth login")
	case 403:
		fmt.Fprintln(w, "✗ 访问被拒绝")
		fmt.Fprintln(w, "  请检查 API Key 权限")
	case 404:
		fmt.Fprintln(w, "✗ Base URL 配置错误")
		fmt.Fprintln(w, "  请检查 Base URL 是否正确")
	case 429:
		fmt.Fprintln(w, "✗ 额度不足或请求过快")
		fmt.Fprintln(w, "  请稍后重试或检查账户额度")
	default:
		fmt.Fprintf(w, "✗ 连接失败 (状态码: %d)\n", statusCode)
	}
}

func init() {
	commands.Register(&AuthCommand{})
}
