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
		fmt.Fprintf(env.Stderr, "未知命令 '%s'\n\n", args[0])
		printConfigHelp(env)
		return 1
	}
}

func (c *ConfigCommand) runShow(args []string, env Env) int {
	config, err := auth.LoadUserConfig()
	if err != nil {
		fmt.Fprintf(env.Stderr, "✗ 加载配置失败: %v\n", err)
		return 1
	}

	fmt.Fprintln(env.Stdout, "当前配置:")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "认证:")
	fmt.Fprintln(env.Stdout, "  Providers:")

	if len(config.Auth.Providers) == 0 {
		fmt.Fprintln(env.Stdout, "    (未配置)")
	} else {
		for name, provider := range config.Auth.Providers {
			fmt.Fprintf(env.Stdout, "    %s:\n", name)
			fmt.Fprintf(env.Stdout, "      API Key: %s\n", auth.SanitizeAPIKey(provider.APIKey))
			fmt.Fprintf(env.Stdout, "      Base URL: %s\n", provider.BaseURL)
		}
	}

	fmt.Fprintf(env.Stdout, "  默认 Provider: %s\n", config.Auth.DefaultProvider)
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "偏好:")
	fmt.Fprintf(env.Stdout, "  默认模型: %s\n", config.Preferences.DefaultModel)
	fmt.Fprintf(env.Stdout, "  试运行: %v\n", config.Preferences.DryRun)
	fmt.Fprintf(env.Stdout, "  工作树: %v\n", config.Preferences.Worktree)

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
		fmt.Fprintf(env.Stderr, "✗ 加载配置失败: %v\n", err)
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
		fmt.Fprintf(env.Stderr, "✗ 保存配置失败: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "✓ API Key 已保存到 %s\n", auth.GetUserConfigPath())
	fmt.Fprintf(env.Stdout, "  API Key: %s\n", auth.SanitizeAPIKey(apiKey))
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
		fmt.Fprintf(env.Stderr, "✗ 加载配置失败: %v\n", err)
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
		fmt.Fprintf(env.Stderr, "✗ 加载配置失败: %v\n", err)
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
		fmt.Fprintf(env.Stderr, "✗ 加载配置失败: %v\n", err)
		return 1
	}
	fmt.Fprintln(env.Stdout, root.Dir)
	return 0
}

func getEnvVarName(provider string) string {
	switch strings.ToLower(provider) {
	case "mimo":
		return "MIMO_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	case "glm", "zhipu":
		return "GLM_API_KEY"
	default:
		return strings.ToUpper(provider) + "_API_KEY"
	}
}
