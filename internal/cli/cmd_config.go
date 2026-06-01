package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mimoneko/mimoneko/internal/auth"
	"github.com/mimoneko/mimoneko/internal/config"
)

type ConfigCommand struct{}

func (c *ConfigCommand) Name() string { return "config" }

func (c *ConfigCommand) Run(args []string, env Env) int {
	if len(args) == 0 {
		printConfigHelp(env)
		return 0
	}

	switch args[0] {
	case "show":
		return c.runShow(args[1:], env)
	case "set", "set-key":
		return cmdConfigSetKey(args[1:], env)
	case "get", "get-key":
		return cmdConfigGetKey(args[1:], env)
	case "list", "ls":
		return cmdConfigList(env)
	case "path":
		return cmdConfigPath(env)
	default:
		PrintError(env.Stderr, "Unknown config command", fmt.Sprintf("未知命令 %q。", args[0]), "运行: mimoneko config")
		printConfigHelp(env)
		return 1
	}
}

func (c *ConfigCommand) runShow(args []string, env Env) int {
	userConfig, err := auth.LoadUserConfig()
	if err != nil {
		PrintErrorDetails(env.Stderr, "Config show failed", "加载用户配置失败。", "检查用户配置文件权限。", err.Error())
		return 1
	}

	PrintHeader(env.Stdout, "Config Show")
	userRows := []KV{
		{Key: "Path", Value: auth.GetUserConfigPath()},
		{Key: "Default", Value: displayProvider(userConfig.Auth.DefaultProvider)},
		{Key: "Model", Value: userConfig.Preferences.DefaultModel},
		{Key: "Dry Run", Value: fmt.Sprintf("%v", userConfig.Preferences.DryRun)},
		{Key: "Worktree", Value: fmt.Sprintf("%v", userConfig.Preferences.Worktree)},
	}
	if userConfig.Auth.DefaultProvider != "" {
		if provider, ok := userConfig.Auth.Providers[userConfig.Auth.DefaultProvider]; ok {
			userRows = append(userRows,
				KV{Key: "Key", Value: MaskSecret(provider.APIKey)},
				KV{Key: "Base URL", Value: provider.BaseURL},
			)
		}
	}
	PrintKV(env.Stdout, "User Config:", userRows)
	fmt.Fprintln(env.Stdout)

	printProjectAuthSummary(env)
	fmt.Fprintln(env.Stdout)

	envVar := auth.APIKeyEnv(userConfig.Auth.DefaultProvider)
	if envVar == "" {
		envVar = "MIMO_API_KEY"
	}
	PrintKV(env.Stdout, "Environment:", []KV{
		{Key: "API Key Env", Value: envVar},
		{Key: "Value", Value: envKeyDisplay(envVar)},
	})

	return 0
}

func init() {
	commands.Register(&ConfigCommand{})
}

func printConfigHelp(env Env) {
	fmt.Fprintln(env.Stdout, "用法: mimoneko config <命令>")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "命令:")
	fmt.Fprintln(env.Stdout, "  show                 查看当前配置（脱敏）")
	fmt.Fprintln(env.Stdout, "  set-key <provider> <key>  设置 API Key")
	fmt.Fprintln(env.Stdout, "  get-key <provider>   获取 API Key")
	fmt.Fprintln(env.Stdout, "  list                 列出所有配置")
	fmt.Fprintln(env.Stdout, "  path                 显示配置文件路径")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "示例:")
	fmt.Fprintln(env.Stdout, "  mimoneko config show")
	fmt.Fprintln(env.Stdout, "  mimoneko config set-key mimo your-api-key")
	fmt.Fprintln(env.Stdout, "  mimoneko config get-key mimo")
}

func cmdConfigSetKey(args []string, env Env) int {
	if len(args) < 2 {
		fmt.Fprintln(env.Stderr, "用法: mimoneko config set-key <provider> <key>")
		return 1
	}

	provider := args[0]
	apiKey := args[1]

	// Load user config
	userConfig, err := auth.LoadUserConfig()
	if err != nil {
		PrintErrorDetails(env.Stderr, "Config failed", "加载用户配置失败。", "检查用户配置文件权限。", err.Error())
		return 1
	}

	// Initialize providers map if needed
	if userConfig.Auth.Providers == nil {
		userConfig.Auth.Providers = make(map[string]auth.ProviderConfig)
	}

	// Get existing config or create new
	existing := userConfig.Auth.Providers[provider]
	existing.APIKey = apiKey

	// Set default base URL if not set
	if existing.BaseURL == "" {
		existing.BaseURL = auth.GetBaseURL(provider)
	}

	userConfig.Auth.Providers[provider] = existing

	// Set as default if it's the first provider
	if userConfig.Auth.DefaultProvider == "" {
		userConfig.Auth.DefaultProvider = provider
	}

	// Save config
	if err := auth.SaveUserConfig(userConfig); err != nil {
		PrintErrorDetails(env.Stderr, "Config failed", "保存用户配置失败。", "检查用户配置文件权限。", err.Error())
		return 1
	}

	PrintSuccess(env.Stdout, fmt.Sprintf("API Key 已保存到 %s", auth.GetUserConfigPath()))
	fmt.Fprintf(env.Stdout, "API Key  %s\n", MaskSecret(apiKey))
	return 0
}

func cmdConfigGetKey(args []string, env Env) int {
	if len(args) < 1 {
		fmt.Fprintln(env.Stderr, "用法: mimoneko config get-key <provider>")
		return 1
	}

	provider := args[0]

	// 1. Check environment variables
	envVar := getEnvVarName(provider)
	apiKey := os.Getenv(envVar)
	if apiKey != "" {
		fmt.Fprintf(env.Stdout, "API Key (%s): %s\n", envVar, auth.SanitizeAPIKey(apiKey))
		return 0
	}

	// 2. Check user config
	userConfig, err := auth.LoadUserConfig()
	if err != nil {
		PrintErrorDetails(env.Stderr, "Config failed", "加载用户配置失败。", "检查用户配置文件权限。", err.Error())
		return 1
	}

	if p, ok := userConfig.Auth.Providers[provider]; ok && p.APIKey != "" {
		fmt.Fprintf(env.Stdout, "API Key: %s\n", auth.SanitizeAPIKey(p.APIKey))
		return 0
	}

	fmt.Fprintf(env.Stdout, "未设置 API Key\n")
	fmt.Fprintf(env.Stdout, "运行 'mimoneko config set-key %s <your-key>' 进行设置\n", provider)
	return 1
}

func cmdConfigList(env Env) int {
	root, err := config.Load(".")
	if err != nil {
		PrintErrorDetails(env.Stderr, "Config failed", "加载项目配置失败。", "检查当前目录是否已初始化。", err.Error())
		return 1
	}

	fmt.Fprintln(env.Stdout, "配置:")
	fmt.Fprintf(env.Stdout, "  项目配置: %s\n", root.Dir)
	fmt.Fprintf(env.Stdout, "  用户配置: %s\n", auth.GetUserConfigPath())
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "API Keys:")

	providers := []string{"mimo", "openai", "deepseek", "glm"}
	for _, p := range providers {
		envVar := getEnvVarName(p)
		key := os.Getenv(envVar)
		source := "env"

		if key == "" {
			// Check user config
			userConfig, err := auth.LoadUserConfig()
			if err == nil {
				if pc, ok := userConfig.Auth.Providers[p]; ok && pc.APIKey != "" {
					key = pc.APIKey
					source = "config"
				}
			}
		}

		status := "未设置"
		if key != "" {
			status = auth.SanitizeAPIKey(key) + " (" + source + ")"
		}
		fmt.Fprintf(env.Stdout, "  %-12s %s\n", p, status)
	}

	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "模型配置:", filepath.Join(root.Dir, "models.yaml"))
	return 0
}

func cmdConfigPath(env Env) int {
	root, err := config.Load(".")
	if err != nil {
		PrintErrorDetails(env.Stderr, "Config failed", "加载项目配置失败。", "检查当前目录是否已初始化。", err.Error())
		return 1
	}
	fmt.Fprintln(env.Stdout, root.Dir)
	return 0
}

func getEnvVarName(provider string) string {
	switch strings.ToLower(provider) {
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	case "glm", "zhipu":
		return "GLM_API_KEY"
	default:
		return auth.APIKeyEnv(provider)
	}
}
