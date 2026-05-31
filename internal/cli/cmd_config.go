package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	case "set", "set-key":
		return cmdConfigSetKey(args[1:], env)
	case "get", "get-key":
		return cmdConfigGetKey(args[1:], env)
	case "list", "ls":
		return cmdConfigList(env)
	case "path":
		return cmdConfigPath(env)
	default:
		fmt.Fprintf(env.Stderr, "Unknown config command: %s\n", args[0])
		printConfigHelp(env)
		return 1
	}
}

func init() {
	commands.Register(&ConfigCommand{})
}

func printConfigHelp(env Env) {
	fmt.Fprintln(env.Stdout, "Usage: mimoneko config <command>")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "Commands:")
	fmt.Fprintln(env.Stdout, "  set-key <provider> <key>   Set API key for a provider")
	fmt.Fprintln(env.Stdout, "  get-key <provider>         Get API key for a provider")
	fmt.Fprintln(env.Stdout, "  list                       List all configuration")
	fmt.Fprintln(env.Stdout, "  path                       Show config file path")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "Examples:")
	fmt.Fprintln(env.Stdout, "  mimoneko config set-key mimo your-api-key")
	fmt.Fprintln(env.Stdout, "  mimoneko config set-key openai sk-xxx")
	fmt.Fprintln(env.Stdout, "  mimoneko config get-key mimo")
}

func cmdConfigSetKey(args []string, env Env) int {
	if len(args) < 2 {
		fmt.Fprintln(env.Stderr, "Usage: mimoneko config set-key <provider> <key>")
		return 1
	}

	provider := args[0]
	apiKey := args[1]

	// 确定环境变量名
	envVar := getEnvVarName(provider)

	// 写入到 .env 文件
	envFile := findEnvFile()
	if err := writeEnvFile(envFile, envVar, apiKey); err != nil {
		fmt.Fprintf(env.Stderr, "Error writing to %s: %v\n", envFile, err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "✓ API key for '%s' has been saved to %s\n", provider, envFile)
	fmt.Fprintf(env.Stdout, "  Environment variable: %s\n", envVar)
	fmt.Fprintln(env.Stdout, "\nPlease restart your terminal or run:")
	fmt.Fprintf(env.Stdout, "  set %s=%s\n", envVar, apiKey)
	return 0
}

func cmdConfigGetKey(args []string, env Env) int {
	if len(args) < 1 {
		fmt.Fprintln(env.Stderr, "Usage: mimoneko config get-key <provider>")
		return 1
	}

	provider := args[0]
	envVar := getEnvVarName(provider)
	apiKey := os.Getenv(envVar)

	if apiKey == "" {
		// 尝试从 .env 文件读取
		envFile := findEnvFile()
		apiKey = readFromEnvFile(envFile, envVar)
	}

	if apiKey == "" {
		fmt.Fprintf(env.Stdout, "No API key set for '%s'\n", provider)
		fmt.Fprintf(env.Stdout, "Set it with: mimoneko config set-key %s <your-key>\n", provider)
		return 1
	}

	// 只显示前几位和后几位
	masked := maskApiKey(apiKey)
	fmt.Fprintf(env.Stdout, "API key for '%s': %s\n", provider, masked)
	return 0
}

func cmdConfigList(env Env) int {
	root, err := config.Load(".")
	if err != nil {
		fmt.Fprintf(env.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Fprintln(env.Stdout, "Configuration:")
	fmt.Fprintf(env.Stdout, "  Config directory: %s\n", root.Dir)
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "API Keys:")

	providers := []string{"mimo", "openai", "deepseek", "glm"}
	for _, p := range providers {
		envVar := getEnvVarName(p)
		key := os.Getenv(envVar)
		if key == "" {
			key = readFromEnvFile(findEnvFile(), envVar)
		}
		status := "not set"
		if key != "" {
			status = maskApiKey(key)
		}
		fmt.Fprintf(env.Stdout, "  %-12s %s\n", p, status)
	}

	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "Models config:", filepath.Join(root.Dir, "models.yaml"))
	return 0
}

func cmdConfigPath(env Env) int {
	root, err := config.Load(".")
	if err != nil {
		fmt.Fprintf(env.Stderr, "Error: %v\n", err)
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

func maskApiKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func findEnvFile() string {
	// 优先使用当前目录的 .env
	if _, err := os.Stat(".env"); err == nil {
		return ".env"
	}

	// 默认在当前目录创建
	return ".env"
}

func writeEnvFile(path, name, value string) error {
	// 读取现有内容
	content := ""
	if data, err := os.ReadFile(path); err == nil {
		content = string(data)
	}

	// 检查是否已存在
	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, name+"=") {
			lines[i] = name + "=" + value
			found = true
			break
		}
	}

	if !found {
		// 添加到文件末尾
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += name + "=" + value + "\n"
		return os.WriteFile(path, []byte(content), 0644)
	}

	// 写回文件
	newContent := strings.Join(lines, "\n")
	return os.WriteFile(path, []byte(newContent), 0644)
}

func readFromEnvFile(path, name string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, name+"=") {
			return strings.TrimPrefix(line, name+"=")
		}
	}
	return ""
}
