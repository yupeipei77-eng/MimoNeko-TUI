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
		// Route to neko command (TUI/chat)
		return commands.Dispatch([]string{"neko"}, env)
	}

	if shouldTreatAsGoal(args) {
		args = []string{"run", "--goal", strings.Join(args, " ")}
	}

	if args[0] == "run" || args[0] == "multi-run" || args[0] == "neko" || args[0] == "model" {
		if err := auth.ApplyUserConfigToEnv(); err != nil {
			PrintErrorDetails(env.Stderr, "Configuration failed", "Unable to load user model configuration.", "Run: mimoneko auth login", err.Error())
			return 1
		}
	}

	if (args[0] == "run" || args[0] == "multi-run") && !hasAnyModelAuthConfigured() {
		result := runFirstTimeSetup(env)
		if result != 0 {
			return result
		}
		if err := auth.ApplyUserConfigToEnv(); err != nil {
			PrintErrorDetails(env.Stderr, "Configuration failed", "Unable to load user model configuration.", "Run: mimoneko auth login", err.Error())
			return 1
		}
	}

	return commands.Dispatch(args, env)
}

func shouldTreatAsGoal(args []string) bool {
	if len(args) == 0 {
		return false
	}
	firstArg := args[0]
	firstArg = strings.TrimSpace(firstArg)
	if firstArg == "" {
		return false
	}
	switch firstArg {
	case "help", "-h", "--help":
		return false
	}
	if commands.Has(firstArg) {
		return false
	}
	// If it's not a known command, treat it as a goal
	// This handles multi-word natural language goals.
	return true
}

func printReadyLanding(w io.Writer) {
	ui := newCLIUI()
	ui.PrintHeader(w, "MimoNeko Ready")
	_, _ = io.WriteString(w, "Your model configuration is ready.\n\n")
	_, _ = io.WriteString(w, "Quick start:\n")
	_, _ = io.WriteString(w, "  mimoneko                          # Enter interactive chat\n")
	_, _ = io.WriteString(w, "  mimoneko \"your goal\"               # Run a task directly\n")
	_, _ = io.WriteString(w, "  mimoneko run \"your goal\"           # Run a task explicitly\n")
	_, _ = io.WriteString(w, "  mimoneko agents run --goal \"...\" --dry-run  # Agent dry-run\n")
	_, _ = io.WriteString(w, "  mimoneko --help                   # Show all commands\n")
	_, _ = io.WriteString(w, "\n")
	_, _ = io.WriteString(w, "Developer mode:\n")
	_, _ = io.WriteString(w, "  go run ./cmd/mimoneko\n")
	_, _ = io.WriteString(w, "  go run ./cmd/mimoneko \"your goal\"\n")
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
