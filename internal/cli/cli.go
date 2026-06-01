package cli

import (
	"io"
	"os"
	"strings"

	"github.com/mimoneko/mimoneko/internal/auth"
	"github.com/mimoneko/mimoneko/internal/pathutil"
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

	if len(args) == 0 {
		if !hasSavedUserModelConfig() {
			return runFirstTimeSetup(env)
		}
		printUsage(env.Stdout)
		return 0
	}

	if shouldTreatAsGoal(args[0]) {
		args = []string{"run", "--goal", strings.Join(args, " ")}
	}

	if args[0] == "run" || args[0] == "multi-run" || args[0] == "neko" || args[0] == "model" {
		if err := auth.ApplyUserConfigToEnv(); err != nil {
			PrintErrorDetails(env.Stderr, "Configuration failed", "加载用户配置失败。", "运行: mimoneko auth login", err.Error())
			return 1
		}
	}

	if (args[0] == "run" || args[0] == "multi-run") && !hasAnyModelAuthConfigured() {
		result := runFirstTimeSetup(env)
		if result != 0 {
			return result
		}
		if err := auth.ApplyUserConfigToEnv(); err != nil {
			PrintErrorDetails(env.Stderr, "Configuration failed", "加载用户配置失败。", "运行: mimoneko auth login", err.Error())
			return 1
		}
	}

	return commands.Dispatch(args, env)
}

func shouldTreatAsGoal(firstArg string) bool {
	firstArg = strings.TrimSpace(firstArg)
	if firstArg == "" {
		return false
	}
	switch firstArg {
	case "help", "-h", "--help":
		return false
	}
	return !commands.Has(firstArg)
}

func hasSavedUserModelConfig() bool {
	userConfig, err := auth.LoadUserConfig()
	if err != nil {
		return false
	}
	provider := strings.TrimSpace(userConfig.Auth.DefaultProvider)
	if provider == "" {
		return false
	}
	providerConfig, ok := userConfig.Auth.Providers[provider]
	return ok && strings.TrimSpace(providerConfig.APIKey) != ""
}

func hasAnyModelAuthConfigured() bool {
	if hasSavedUserModelConfig() {
		return true
	}
	for _, envVar := range []string{"MIMO_API_KEY", "MIMONEKO_API_KEY", "MimoNeko_API_KEY", "OPENAI_API_KEY", "MIMONEKO_LOCAL_API_KEY"} {
		value := strings.TrimSpace(os.Getenv(envVar))
		if value != "" && !pathutil.APIKeyLooksPlaceholder(value) {
			return true
		}
	}
	return false
}
