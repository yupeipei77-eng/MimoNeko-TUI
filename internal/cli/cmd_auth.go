package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/mimoneko/mimoneko/internal/auth"
	"github.com/mimoneko/mimoneko/internal/modelprofile"
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
	return runFirstTimeSetup(env)
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

	if err := auth.ApplyUserConfigToEnv(); err != nil {
		fmt.Fprintln(env.Stdout, "✗ 连接失败")
		fmt.Fprintf(env.Stderr, "  %v\n", err)
		return 1
	}
	result, err := modelprofile.Test(context.Background(), ".", modelprofile.TestOptions{
		Provider:  provider,
		Model:     config.Preferences.DefaultModel,
		BaseURL:   providerConfig.BaseURL,
		APIKeyEnv: auth.APIKeyEnv(provider),
		Prompt:    "Reply with OK only.",
	})
	if err == nil && result.Status == "ok" {
		fmt.Fprintln(env.Stdout, "✓ 已连接")
	} else {
		fmt.Fprintln(env.Stdout, "✗ 连接失败")
		if err != nil {
			fmt.Fprintf(env.Stderr, "  %s\n", modelprofile.SanitizeText(err.Error()))
		} else {
			fmt.Fprintf(env.Stderr, "  %s\n", modelprofile.SanitizeText(result.Error))
		}
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
