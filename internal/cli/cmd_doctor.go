package cli

import (
	"flag"
	"fmt"

	"github.com/mimoneko/mimoneko/internal/config"
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

func init() {
	commands.Register(&DoctorCommand{})
}
