package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mimoneko/mimoneko/internal/auth"
	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/modelprofile"
	"github.com/mimoneko/mimoneko/internal/pathutil"
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
		PrintError(env.Stderr, "Unknown auth command", fmt.Sprintf("未知命令 %q。", args[0]), "运行: mimoneko auth")
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
		PrintErrorDetails(env.Stderr, "Auth status failed", "加载用户配置失败。", "检查用户配置文件权限。", err.Error())
		return 1
	}

	if config.Auth.DefaultProvider == "" {
		PrintHeader(env.Stdout, "Auth Status")
		PrintWarning(env.Stdout, "未配置")
		fmt.Fprintln(env.Stdout, "运行: mimoneko auth login")
		return 0
	}

	provider := config.Auth.DefaultProvider
	providerConfig, ok := config.Auth.Providers[provider]
	if !ok {
		PrintError(env.Stdout, "Auth status failed", fmt.Sprintf("Provider %q 未配置。", provider), "运行: mimoneko auth login")
		return 1
	}

	ui := newCLIUI()
	ui.PrintHeader(env.Stdout, "Auth Status")
	PrintKV(env.Stdout, "User Config:", []KV{
		{Key: "Path", Value: auth.GetUserConfigPath()},
		{Key: "Provider", Value: displayProvider(provider)},
		{Key: "Base URL", Value: providerConfig.BaseURL},
		{Key: "Key", Value: MaskSecret(providerConfig.APIKey)},
		{Key: "Model", Value: config.Preferences.DefaultModel},
	})
	fmt.Fprintln(env.Stdout)
	printProjectAuthSummary(env)
	fmt.Fprintln(env.Stdout)

	if err := auth.ApplyUserConfigToEnv(); err != nil {
		PrintErrorDetails(env.Stderr, "Connection failed", "无法加载用户配置到环境变量。", "运行: mimoneko auth login", err.Error())
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
		fmt.Fprintf(env.Stdout, "Status  %s Ready\n", ui.Icon("success"))
	} else {
		fmt.Fprintf(env.Stdout, "Status  %s Connection failed\n", ui.Icon("error"))
		if err != nil {
			reason, suggestion, details := friendlyModelError(modelprofile.SanitizeText(err.Error()))
			PrintErrorDetails(env.Stderr, "Connection failed", reason, suggestion, details)
		} else {
			reason, suggestion, details := friendlyModelError(result.Error)
			PrintErrorDetails(env.Stderr, "Connection failed", reason, suggestion, details)
		}
		return 1
	}

	return 0
}

func (c *AuthCommand) runLogout(args []string, env Env) int {
	config, err := auth.LoadUserConfig()
	if err != nil {
		PrintErrorDetails(env.Stderr, "Logout failed", "加载用户配置失败。", "检查用户配置文件权限。", err.Error())
		return 1
	}

	if config.Auth.DefaultProvider == "" {
		PrintWarning(env.Stdout, "未配置")
		return 0
	}

	config.Auth.Providers = nil
	config.Auth.DefaultProvider = ""

	if err := auth.SaveUserConfig(config); err != nil {
		PrintErrorDetails(env.Stderr, "Logout failed", "保存用户配置失败。", "检查用户配置文件权限。", err.Error())
		return 1
	}

	PrintSuccess(env.Stdout, "已登出")
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

func printProjectAuthSummary(env Env) {
	root, err := resolveRoot("", env)
	if err != nil {
		PrintKV(env.Stdout, "Project Config:", []KV{
			{Key: "Path", Value: "(unknown)"},
			{Key: "Secret", Value: "Not stored"},
		})
		return
	}
	modelsPath := filepath.Join(config.ConfigDir(root), "models.yaml")
	rows := []KV{
		{Key: "Path", Value: modelsPath},
		{Key: "Secret", Value: "Not stored"},
	}
	cfg, err := config.Load(root)
	if err == nil {
		provider := findProjectDefaultProvider(cfg)
		if provider.Name != "" {
			rows = append(rows,
				KV{Key: "Provider", Value: displayProvider(provider.Name)},
				KV{Key: "Key", Value: provider.APIKeyEnv},
			)
		}
	} else {
		rows = append(rows, KV{Key: "Status", Value: "Missing"})
	}
	PrintKV(env.Stdout, "Project Config:", rows)
}

func findProjectDefaultProvider(cfg *config.Root) config.ProviderConfig {
	defaultModel := strings.TrimSpace(cfg.Models.Routing.DefaultModel)
	if defaultModel == "" && len(cfg.Models.Providers) > 0 {
		return cfg.Models.Providers[0]
	}
	for _, provider := range cfg.Models.Providers {
		for _, model := range provider.Models {
			if model.Name == defaultModel {
				return provider
			}
		}
	}
	return config.ProviderConfig{}
}

func envKeyDisplay(envVar string) string {
	key := strings.TrimSpace(os.Getenv(envVar))
	if key == "" {
		return "missing"
	}
	if pathutil.APIKeyLooksPlaceholder(key) {
		return "missing"
	}
	return MaskSecret(key)
}

func init() {
	commands.Register(&AuthCommand{})
}
