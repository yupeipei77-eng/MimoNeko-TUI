package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/auth"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
)

type DoctorCommand struct{}

func (c *DoctorCommand) Name() string { return "doctor" }

func (c *DoctorCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	report, err := config.Doctor(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "doctor failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "MimoNeko Doctor Report for %s\n", config.ConfigDir(root))
	fmt.Fprintf(env.Stdout, "config_exists=%v\n", report.ConfigExists)
	fmt.Fprintf(env.Stdout, "system_prompt=%v\n", report.SystemPromptExists)
	fmt.Fprintf(env.Stdout, "coding_rules=%v\n", report.CodingRulesExists)
	fmt.Fprintf(env.Stdout, "tools_schema=%v\n", report.ToolsSchemaExists)
	fmt.Fprintf(env.Stdout, "models_configured=%v\n", report.ModelsConfigured)
	fmt.Fprintf(env.Stdout, "worktree_config=%v\n", report.WorktreeConfigExists)
	fmt.Fprintf(env.Stdout, "patch_config=%v\n", report.PatchConfigExists)
	fmt.Fprintf(env.Stdout, "events_config=%v\n", report.EventsConfigExists)
	fmt.Fprintf(env.Stdout, "review_config=%v\n", report.ReviewConfigExists)
	fmt.Fprintf(env.Stdout, "validation_config=%v\n", report.ValidationConfigExists)

	fmt.Fprintf(env.Stdout, "worktree_isolation=%v\n", report.WorktreeIsolation)
	fmt.Fprintf(env.Stdout, "patch_require_clean_main=%v\n", report.PatchRequireCleanMain)
	fmt.Fprintf(env.Stdout, "patch_max_diff_bytes=%d\n", report.PatchMaxDiffBytes)
	fmt.Fprintf(env.Stdout, "review_max_diff_bytes=%d\n", report.ReviewMaxDiffBytes)
	fmt.Fprintf(env.Stdout, "validation_max_output_bytes=%d\n", report.ValidationMaxOutputBytes)
	fmt.Fprintf(env.Stdout, "validation_timeout_seconds=%d\n", report.ValidationTimeoutSeconds)
	fmt.Fprintf(env.Stdout, "multiagent_max_iterations=%d\n", report.MultiAgentMaxIterations)
	fmt.Fprintf(env.Stdout, "multiagent_default_worktree=%v\n", report.MultiAgentDefaultWorktree)
	fmt.Fprintf(env.Stdout, "multiagent_default_dry_run=%v\n", report.MultiAgentDefaultDryRun)

	if report.DefaultAPIKeyEnv != "" {
		fmt.Fprintf(env.Stdout, "api_key_status=%s\n", apiKeyStatus(report.DefaultAPIKeyEnv))
	}
	printDoctorRuntimeChecks(root, env)

	for _, err := range report.Errors {
		fmt.Fprintf(env.Stderr, "error: %v\n", err)
	}

	if len(report.Errors) > 0 {
		for _, hint := range report.Hints {
			fmt.Fprintln(env.Stderr, hint)
		}
		return 1
	}
	return 0
}

func printDoctorRuntimeChecks(root string, env Env) {
	userConfigPath := auth.GetUserConfigPath()
	_, statErr := os.Stat(userConfigPath)
	userConfigExists := statErr == nil
	fmt.Fprintf(env.Stdout, "user_config_path=%s\n", userConfigPath)
	fmt.Fprintf(env.Stdout, "user_config_exists=%v\n", userConfigExists)

	userCfg, userErr := auth.LoadUserConfig()
	if userErr != nil {
		fmt.Fprintf(env.Stdout, "user_config_error=%s\n", sanitizeDoctorLine(userErr.Error()))
		userCfg = &auth.Config{}
	}

	cfg, cfgErr := config.Load(root)
	if cfgErr != nil {
		fmt.Fprintf(env.Stdout, "project_config_error=%s\n", sanitizeDoctorLine(cfgErr.Error()))
		cfg = &config.Root{}
	}

	provider := doctorDefaultProvider(cfg, userCfg)
	model := doctorDefaultModel(cfg, userCfg, provider)
	baseURL := doctorBaseURL(cfg, provider)
	if strings.TrimSpace(baseURL) == "" {
		baseURL = auth.GetBaseURL(provider)
	}
	key := auth.GetAPIKey(provider)
	apiKeyStatus := "missing"
	if strings.TrimSpace(key) != "" {
		apiKeyStatus = "configured (masked)"
	}

	fmt.Fprintf(env.Stdout, "provider_configured=%v\n", doctorProviderConfigured(cfg, userCfg))
	fmt.Fprintf(env.Stdout, "default_provider=%s\n", emptyDoctorValue(provider))
	fmt.Fprintf(env.Stdout, "api_key=%s\n", apiKeyStatus)
	fmt.Fprintf(env.Stdout, "base_url=%s\n", emptyDoctorValue(baseURL))
	fmt.Fprintf(env.Stdout, "default_model=%s\n", emptyDoctorValue(model))

	gitOK, gitDetail := doctorGitRepo(root)
	fmt.Fprintf(env.Stdout, "git_repo=%v\n", gitOK)
	if gitDetail != "" {
		fmt.Fprintf(env.Stdout, "git_repo_detail=%s\n", gitDetail)
	}

	testOK, testDetail := doctorGoTest(root)
	fmt.Fprintf(env.Stdout, "go_test_runnable=%v\n", testOK)
	if testDetail != "" {
		fmt.Fprintf(env.Stdout, "go_test_detail=%s\n", testDetail)
	}
}

func doctorProviderConfigured(cfg *config.Root, userCfg *auth.Config) bool {
	return cfg != nil && len(cfg.Models.Providers) > 0 ||
		userCfg != nil && len(userCfg.Auth.Providers) > 0
}

func doctorDefaultProvider(cfg *config.Root, userCfg *auth.Config) string {
	if userCfg != nil && strings.TrimSpace(userCfg.Auth.DefaultProvider) != "" {
		return strings.TrimSpace(userCfg.Auth.DefaultProvider)
	}
	if cfg != nil {
		if provider := findProviderForDefaultModel(cfg); provider != "" && provider != "unknown" {
			return provider
		}
		if len(cfg.Models.Providers) > 0 {
			return strings.TrimSpace(cfg.Models.Providers[0].Name)
		}
	}
	if userCfg != nil {
		for provider := range userCfg.Auth.Providers {
			if strings.TrimSpace(provider) != "" {
				return strings.TrimSpace(provider)
			}
		}
	}
	return ""
}

func doctorDefaultModel(cfg *config.Root, userCfg *auth.Config, provider string) string {
	if cfg != nil && strings.TrimSpace(cfg.Models.Routing.DefaultModel) != "" {
		return strings.TrimSpace(cfg.Models.Routing.DefaultModel)
	}
	if userCfg != nil && strings.TrimSpace(userCfg.Preferences.DefaultModel) != "" {
		return strings.TrimSpace(userCfg.Preferences.DefaultModel)
	}
	if strings.TrimSpace(provider) != "" {
		return auth.DefaultModel(provider)
	}
	return ""
}

func doctorBaseURL(cfg *config.Root, provider string) string {
	provider = strings.TrimSpace(provider)
	if cfg == nil || provider == "" {
		return ""
	}
	for _, p := range cfg.Models.Providers {
		if strings.EqualFold(strings.TrimSpace(p.Name), provider) {
			return strings.TrimSpace(p.BaseURL)
		}
	}
	return ""
}

func doctorGitRepo(root string) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, truncateDoctorDetail(string(out), err)
	}
	return strings.TrimSpace(string(out)) == "true", ""
}

func doctorGoTest(root string) (bool, string) {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		if os.IsNotExist(err) {
			return false, "go.mod not found"
		}
		return false, sanitizeDoctorLine(err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "./...")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, truncateDoctorDetail(string(out), err)
	}
	return true, "go test ./..."
}

func truncateDoctorDetail(output string, err error) string {
	detail := strings.TrimSpace(output)
	if err != nil {
		if detail != "" {
			detail += " | "
		}
		detail += err.Error()
	}
	return sanitizeDoctorLine(detail)
}

func sanitizeDoctorLine(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 300 {
		return value[:300] + "...(truncated)"
	}
	return value
}

func emptyDoctorValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "missing"
	}
	return value
}

func init() {
	commands.Register(&DoctorCommand{})
}
