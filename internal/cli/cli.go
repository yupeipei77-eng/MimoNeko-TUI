package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/reasonforge/reasonforge/internal/agent"
	"github.com/reasonforge/reasonforge/internal/cache"
	"github.com/reasonforge/reasonforge/internal/config"
	"github.com/reasonforge/reasonforge/internal/contextengine"
	"github.com/reasonforge/reasonforge/internal/conversation"
	"github.com/reasonforge/reasonforge/internal/events"
	"github.com/reasonforge/reasonforge/internal/modelrouter"
	"github.com/reasonforge/reasonforge/internal/multiagent"
	"github.com/reasonforge/reasonforge/internal/patch"
	"github.com/reasonforge/reasonforge/internal/prefix"
	"github.com/reasonforge/reasonforge/internal/review"
	"github.com/reasonforge/reasonforge/internal/scratchpad"
	"github.com/reasonforge/reasonforge/internal/task"
	"github.com/reasonforge/reasonforge/internal/tools"
	"github.com/reasonforge/reasonforge/internal/validation"
	"github.com/reasonforge/reasonforge/internal/version"
	"github.com/reasonforge/reasonforge/internal/worktree"
)

type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Getwd  func() (string, error)
}

func Run(args []string, env Env) int {
	if env.Stdout == nil {
		env.Stdout = io.Discard
	}
	if env.Stderr == nil {
		env.Stderr = io.Discard
	}
	if env.Getwd == nil {
		env.Getwd = func() (string, error) { return ".", nil }
	}

	if len(args) == 0 {
		printUsage(env.Stderr)
		return 2
	}

	switch args[0] {
	case "version":
		return runVersion(args[1:], env)
	case "init":
		return runInit(args[1:], env)
	case "doctor":
		return runDoctor(args[1:], env)
	case "cache-report":
		return runCacheReport(args[1:], env)
	case "models":
		return runModels(args[1:], env)
	case "tools":
		return runTools(args[1:], env)
	case "tool-run":
		return runToolRun(args[1:], env)
	case "run":
		return runAgent(args[1:], env)
	case "patch":
		return runPatch(args[1:], env)
	case "multi-run":
		return runMultiAgent(args[1:], env)
	case "runs":
		return runRuns(args[1:], env)
	case "run-status":
		return runRunStatus(args[1:], env)
	case "run-events":
		return runRunEvents(args[1:], env)
	case "help", "-h", "--help":
		if len(args) > 1 {
			fmt.Fprintf(env.Stderr, "%s accepts no arguments\n", args[0])
			return 2
		}
		printUsage(env.Stdout)
		return 0
	default:
		fmt.Fprintf(env.Stderr, "unknown command %q\n", args[0])
		printUsage(env.Stderr)
		return 2
	}
}

func runVersion(args []string, env Env) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}
	fmt.Fprintln(env.Stdout, version.String())
	return 0
}

func runInit(args []string, env Env) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
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

	written, err := config.Init(root)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	if len(written) == 0 {
		fmt.Fprintf(env.Stdout, "ReasonForge already initialized at %s\n", config.ConfigDir(root))
		return 0
	}

	fmt.Fprintf(env.Stdout, "Initialized ReasonForge at %s\n", config.ConfigDir(root))
	for _, path := range written {
		fmt.Fprintf(env.Stdout, "created %s\n", filepath.ToSlash(path))
	}
	return 0
}

func runDoctor(args []string, env Env) int {
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

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "doctor failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(env.Stdout, "ReasonForge doctor OK")
	fmt.Fprintf(env.Stdout, "config_dir=%s\n", filepath.ToSlash(cfg.Dir))
	fmt.Fprintf(env.Stdout, "default_model=%s\n", cfg.Models.Routing.DefaultModel)
	fmt.Fprintf(env.Stdout, "immutable_prefix_sources=%d\n", len(cfg.Prefix.ImmutableSources))
	fmt.Fprintf(env.Stdout, "prefix_canonicalization=enabled\n")
	fmt.Fprintf(env.Stdout, "prefix_hash_stable=true\n")
	fmt.Fprintf(env.Stdout, "context_budget_valid=true\n")
	fmt.Fprintf(env.Stdout, "cache_report=available\n")
	fmt.Fprintf(env.Stdout, "append_only_log=available\n")
	fmt.Fprintf(env.Stdout, "budget_warn_ratio=%.2f\n", cfg.Prefix.Budget.WarnRatio)
	fmt.Fprintf(env.Stdout, "budget_block_ratio=%.2f\n", cfg.Prefix.Budget.BlockRatio)
	fmt.Fprintf(env.Stdout, "cache_estimated_ttl=%s\n", cfg.Prefix.Cache.EstimatedTTL)
	fmt.Fprintf(env.Stdout, "event_id_collision_resistant=true\n")
	fmt.Fprintf(env.Stdout, "worktree_isolation=%v\n", cfg.Worktree.Enabled)
	fmt.Fprintf(env.Stdout, "worktree_max_active=%d\n", cfg.Worktree.MaxActive)
	fmt.Fprintf(env.Stdout, "patch_require_clean_main=%v\n", cfg.Patch.RequireCleanMain)
	fmt.Fprintf(env.Stdout, "patch_max_diff_bytes=%d\n", cfg.Patch.MaxDiffBytes)
	fmt.Fprintf(env.Stdout, "review_max_diff_bytes=%d\n", cfg.Review.MaxDiffBytes)
	fmt.Fprintf(env.Stdout, "review_high_risk_file_count=%d\n", cfg.Review.HighRiskFileCount)
	fmt.Fprintf(env.Stdout, "review_high_risk_line_count=%d\n", cfg.Review.HighRiskLineCount)
	fmt.Fprintf(env.Stdout, "review_strict_model_review=%v\n", cfg.Review.StrictModelReview)
	fmt.Fprintf(env.Stdout, "validation_max_output_bytes=%d\n", cfg.Validation.MaxOutputBytes)
	fmt.Fprintf(env.Stdout, "validation_timeout_seconds=%d\n", cfg.Validation.TimeoutSeconds)
	fmt.Fprintf(env.Stdout, "multiagent_max_iterations=%d\n", cfg.MultiAgent.MaxIterations)
	fmt.Fprintf(env.Stdout, "multiagent_max_allowed_iterations=%d\n", cfg.MultiAgent.MaxAllowedIterations)
	fmt.Fprintf(env.Stdout, "multiagent_default_worktree=%v\n", cfg.MultiAgent.DefaultWorktree)
	fmt.Fprintf(env.Stdout, "multiagent_default_dry_run=%v\n", cfg.MultiAgent.DefaultDryRun)
	fmt.Fprintf(env.Stdout, "events_enabled=%v\n", cfg.Events.Enabled)
	fmt.Fprintf(env.Stdout, "events_store_path=%s\n", cfg.Events.StorePath)
	fmt.Fprintf(env.Stdout, "events_max_message_bytes=%d\n", cfg.Events.MaxMessageBytes)
	fmt.Fprintf(env.Stdout, "events_emit_tool_events=%v\n", cfg.Events.EmitToolEvents)
	fmt.Fprintf(env.Stdout, "events_emit_patch_events=%v\n", cfg.Events.EmitPatchEvents)
	fmt.Fprintf(env.Stdout, "events_emit_validation_events=%v\n", cfg.Events.EmitValidationEvents)
	return 0
}

func resolveRoot(dir string, env Env) (string, error) {
	if strings.TrimSpace(dir) != "" {
		return filepath.Abs(dir)
	}

	root, err := env.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return filepath.Abs(root)
}

func runCacheReport(args []string, env Env) int {
	fs := flag.NewFlagSet("cache-report", flag.ContinueOnError)
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

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "cache-report failed: %v\n", err)
		return 1
	}

	registryPath := cfg.Prefix.Cache.RegistryPath
	if !filepath.IsAbs(registryPath) {
		registryPath = filepath.Join(root, registryPath)
	}

	registry, err := NewCacheRegistryForCLI(registryPath, cfg.Prefix.Cache)
	if err != nil {
		fmt.Fprintf(env.Stderr, "cache-report failed: %v\n", err)
		return 1
	}

	report, err := registry.Report()
	if err != nil {
		fmt.Fprintf(env.Stderr, "cache-report failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "total_observations=%d\n", report.GlobalSummary.TotalObservations)
	fmt.Fprintf(env.Stdout, "total_tokens=%d\n", report.GlobalSummary.TotalTokens)
	fmt.Fprintf(env.Stdout, "cached_tokens=%d\n", report.GlobalSummary.TotalCachedTokens)
	fmt.Fprintf(env.Stdout, "hit_rate=%.4f\n", report.GlobalSummary.OverallHitRate)
	fmt.Fprintf(env.Stdout, "estimated_saving_percent=%.2f\n", report.GlobalSummary.EstimatedSavingPercent)
	fmt.Fprintf(env.Stdout, "fingerprint_count=%d\n", len(report.ByFingerprint))

	for _, fp := range report.ByFingerprint {
		fmt.Fprintf(env.Stdout, "  fingerprint=%s hit_rate=%.4f reuse_count=%d uncached_tokens=%d\n",
			fp.PrefixHash, fp.HitRate, fp.ReuseCount, fp.UncachedTokens)
	}

	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: reasonforge <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  version      Print version information")
	fmt.Fprintln(w, "  init         Create local .reasonforge config files")
	fmt.Fprintln(w, "  doctor       Validate local ReasonForge configuration")
	fmt.Fprintln(w, "  cache-report Show prefix cache statistics")
	fmt.Fprintln(w, "  models       Show model provider configuration")
	fmt.Fprintln(w, "  tools        List available tools and their status")
	fmt.Fprintln(w, "  tool-run     Execute a tool with arguments")
	fmt.Fprintln(w, "  run          Run an agent task")
	fmt.Fprintln(w, "  multi-run    Run multi-agent task (Planner->Coder->Reviewer)")
	fmt.Fprintln(w, "  patch        Manage patches (list, preview, validate, review, apply, discard)")
	fmt.Fprintln(w, "  runs         List recent runs with state and progress")
	fmt.Fprintln(w, "  run-status   Show detailed status for a specific run")
	fmt.Fprintln(w, "  run-events   Show events for a specific run")
}

func runModels(args []string, env Env) int {
	fs := flag.NewFlagSet("models", flag.ContinueOnError)
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

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "models failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(env.Stdout, "ReasonForge Models")
	fmt.Fprintf(env.Stdout, "default_model=%s\n", cfg.Models.Routing.DefaultModel)
	fmt.Fprintln(env.Stdout)

	for _, provider := range cfg.Models.Providers {
		fmt.Fprintf(env.Stdout, "provider=%s\n", provider.Name)
		fmt.Fprintf(env.Stdout, "type=%s\n", provider.Type)
		fmt.Fprintf(env.Stdout, "base_url=%s\n", provider.BaseURL)
		fmt.Fprintf(env.Stdout, "api_key_env=%s\n", provider.APIKeyEnv)
		fmt.Fprintf(env.Stdout, "api_key_status=%s\n", apiKeyStatus(provider.APIKeyEnv))

		modelNames := make([]string, 0, len(provider.Models))
		for _, m := range provider.Models {
			modelNames = append(modelNames, m.Name)
		}
		fmt.Fprintf(env.Stdout, "models=%s\n", strings.Join(modelNames, ","))
		fmt.Fprintln(env.Stdout)
	}

	if len(cfg.Models.Routing.FallbackChain) > 0 {
		fmt.Fprintln(env.Stdout, "fallback_chain:")
		for i, entry := range cfg.Models.Routing.FallbackChain {
			fmt.Fprintf(env.Stdout, "%d. %s/%s\n", i+1, entry.Provider, entry.Model)
		}
	} else {
		fmt.Fprintln(env.Stdout, "fallback_chain:")
		fmt.Fprintf(env.Stdout, "1. %s/%s\n", findProviderForDefaultModel(cfg), cfg.Models.Routing.DefaultModel)
	}

	return 0
}

// apiKeyStatus checks whether an API key environment variable is configured.
// It returns "configured" if the env var has a non-empty value, "missing" otherwise.
// It never reveals the actual key value.
func apiKeyStatus(envVar string) string {
	if envVar == "" {
		return "missing"
	}
	val := os.Getenv(envVar)
	if strings.TrimSpace(val) == "" {
		return "missing"
	}
	return "configured"
}

// findProviderForDefaultModel returns the provider name for the default model.
func findProviderForDefaultModel(cfg *config.Root) string {
	for _, provider := range cfg.Models.Providers {
		for _, model := range provider.Models {
			if model.Name == cfg.Models.Routing.DefaultModel {
				return provider.Name
			}
		}
	}
	return "unknown"
}

func rejectExtraArgs(fs *flag.FlagSet, env Env) bool {
	if fs.NArg() == 0 {
		return false
	}

	fmt.Fprintf(env.Stderr, "%s accepts no positional arguments: %s\n", fs.Name(), strings.Join(fs.Args(), " "))
	return true
}

func runTools(args []string, env Env) int {
	fs := flag.NewFlagSet("tools", flag.ContinueOnError)
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

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "tools failed: %v\n", err)
		return 1
	}

	registry := tools.NewMemoryRegistry()
	testCmds := tools.TestCommandsFromConfig(cfg)

	if err := tools.RegisterBuiltinTools(registry, testCmds); err != nil {
		fmt.Fprintf(env.Stderr, "tools failed: %v\n", err)
		return 1
	}

	enabledMap := tools.EnabledToolsFromConfig(cfg)

	fmt.Fprintln(env.Stdout, "ReasonForge Tools")
	for _, info := range registry.List() {
		enabled := "true"
		if e, ok := enabledMap[info.Name]; ok && !e {
			enabled = "false"
		}
		fmt.Fprintf(env.Stdout, "%-12s enabled=%-5s risk=%s\n", info.Name, enabled, info.RiskLevel)
	}
	return 0
}

func runToolRun(args []string, env Env) int {
	fs := flag.NewFlagSet("tool-run", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	dryRun := fs.Bool("dry-run", false, "dry run mode (no side effects)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "tool-run requires a tool name")
		return 2
	}
	toolName := remaining[0]

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "tool-run failed: %v\n", err)
		return 1
	}

	guard := tools.SafetyGuardFromConfig(cfg)
	registry := tools.NewMemoryRegistry()
	testCmds := tools.TestCommandsFromConfig(cfg)

	if err := tools.RegisterBuiltinTools(registry, testCmds); err != nil {
		fmt.Fprintf(env.Stderr, "tool-run failed: %v\n", err)
		return 1
	}

	enabledMap := tools.EnabledToolsFromConfig(cfg)
	auditPath := tools.DefaultAuditLogPath(root)
	auditLog, err := tools.NewAuditLog(auditPath)
	if err != nil {
		fmt.Fprintf(env.Stderr, "tool-run failed: audit log: %v\n", err)
		return 1
	}
	defer auditLog.Close()

	runtime := tools.NewDefaultToolRuntime(registry, guard, auditLog, enabledMap)

	// Parse tool arguments from remaining flags
	toolArgs := make(map[string]string)
	for i := 1; i < len(remaining); i++ {
		if strings.HasPrefix(remaining[i], "--") {
			key := strings.TrimPrefix(remaining[i], "--")
			if i+1 < len(remaining) && !strings.HasPrefix(remaining[i+1], "--") {
				toolArgs[key] = remaining[i+1]
				i++
			} else {
				toolArgs[key] = "true"
			}
		}
	}

	req := tools.ToolRequest{
		ToolName: toolName,
		RepoRoot: root,
		Args:     toolArgs,
		DryRun:   *dryRun,
		Metadata: map[string]string{"source": "cli"},
	}

	resp, err := runtime.Run(context.Background(), req)
	if err != nil {
		fmt.Fprintf(env.Stderr, "tool-run failed: %v\n", err)
		return 1
	}

	if resp.Success {
		if resp.Stdout != "" {
			fmt.Fprint(env.Stdout, resp.Stdout)
		}
		if resp.Stderr != "" {
			fmt.Fprint(env.Stderr, resp.Stderr)
		}
		return resp.ExitCode
	}

	fmt.Fprintf(env.Stderr, "tool-run %s failed: %s\n", toolName, resp.Error)
	if resp.Stdout != "" {
		fmt.Fprint(env.Stdout, resp.Stdout)
	}
	if resp.Stderr != "" {
		fmt.Fprint(env.Stderr, resp.Stderr)
	}
	return resp.ExitCode
}

func runAgent(args []string, env Env) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	goal := fs.String("goal", "", "task goal (required)")
	maxSteps := fs.Int("max-steps", 0, "max agent loop steps (default: from contract)")
	dryRun := fs.Bool("dry-run", true, "dry run mode (no side effects)")
	autoApproveMedium := fs.Bool("auto-approve-medium", false, "auto-approve medium-risk tools without prompting")
	taskID := fs.String("task-id", "", "task ID (auto-generated if empty)")
	useWorktree := fs.Bool("worktree", false, "run in isolated git worktree")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}

	if strings.TrimSpace(*goal) == "" {
		fmt.Fprintln(env.Stderr, "run requires --goal")
		return 2
	}

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run failed: %v\n", err)
		return 1
	}

	contract := task.DefaultContract(root, *goal)
	contract.DryRun = *dryRun
	if *maxSteps > 0 {
		contract.MaxSteps = *maxSteps
	}

	tid := *taskID
	if tid == "" {
		tid = "task_cli_" + generateShortID()
	}

	deps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run failed: %v\n", err)
		return 1
	}
	defer cleanup()

	rt := agent.NewSingleAgentRuntime(deps)
	rt.SetOutput(env.Stdout)

	policy := agent.InteractiveApprovalPolicy(os.Stdin)
	policy.AutoApproveMediumRisk = *autoApproveMedium
	rt.SetApprovalPolicy(policy)

	req := agent.AgentRunRequest{
		TaskID:      tid,
		RepoRoot:    root,
		Goal:        *goal,
		Contract:    contract,
		MaxSteps:    *maxSteps,
		DryRun:      *dryRun,
		UseWorktree: *useWorktree,
	}

	fmt.Fprintf(env.Stdout, "ReasonForge Agent\n")
	fmt.Fprintf(env.Stdout, "run_id=pending goal=%q max_steps=%d dry_run=%v worktree=%v\n", *goal, contract.MaxSteps, contract.DryRun, *useWorktree)

	result, err := rt.Run(context.Background(), req)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "run_id=%s state=%s steps=%d\n", result.RunID, result.State, len(result.Steps))
	if result.WorktreeID != "" {
		fmt.Fprintf(env.Stdout, "worktree_id=%s\n", result.WorktreeID)
	}
	if result.FinalMessage != "" {
		fmt.Fprintf(env.Stdout, "message=%s\n", result.FinalMessage)
	}
	if result.Error != "" {
		fmt.Fprintf(env.Stdout, "error=%s\n", result.Error)
	}
	if result.PatchPreview != nil {
		fmt.Fprintf(env.Stdout, "patch_preview:\n")
		fmt.Fprintf(env.Stdout, "  files_changed=%d\n", result.PatchPreview.Summary.FilesChanged)
		fmt.Fprintf(env.Stdout, "  additions=%d deletions=%d\n", result.PatchPreview.Summary.Additions, result.PatchPreview.Summary.Deletions)
		fmt.Fprintf(env.Stdout, "  risk_level=%s\n", result.PatchPreview.RiskLevel)
		if len(result.PatchPreview.Violations) > 0 {
			fmt.Fprintf(env.Stdout, "  violations=%d\n", len(result.PatchPreview.Violations))
		}
		fmt.Fprintf(env.Stdout, "  review with: reasonforge patch preview %s\n", result.WorktreeID)
	}

	switch result.State {
	case agent.AgentStateSucceeded:
		return 0
	case agent.AgentStateFailed:
		return 1
	case agent.AgentStateCancelled:
		return 130
	case agent.AgentStateWaitingApproval:
		return 2
	default:
		return 1
	}
}

// runMultiAgent handles the "multi-run" command.
// It runs the Planner->Coder->Reviewer multi-agent loop.
// Default: worktree=true, dry-run=true, max_iterations=2.
// Does NOT auto-apply, auto-commit, or auto-push.
func runMultiAgent(args []string, env Env) int {
	fs := flag.NewFlagSet("multi-run", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	model := fs.String("model", "", "model name (default: from config)")
	maxIterations := fs.Int("max-iterations", 0, "max iterations (default: 2, max: 5)")
	dryRun := fs.Bool("dry-run", true, "dry run mode (no side effects)")
	useWorktree := fs.Bool("worktree", true, "run in isolated git worktree (default: true)")
	approveMedium := fs.Bool("approve-medium", false, "auto-approve medium-risk tools for coder agent")
	modelReview := fs.Bool("model-review", false, "use AI model review in reviewer agent")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Goal is the first positional argument
	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "multi-run requires a goal argument")
		fmt.Fprintln(env.Stderr, "Usage: reasonforge multi-run \"fix typo in README\"")
		return 2
	}
	goal := remaining[0]

	// multi-run requires worktree isolation; --worktree=false is not supported
	if !*useWorktree {
		fmt.Fprintln(env.Stderr, "multi-run requires worktree isolation; --worktree=false is not supported")
		return 2
	}

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "multi-run failed: %v\n", err)
		return 1
	}

	// Apply multiagent config defaults
	if *maxIterations == 0 {
		*maxIterations = cfg.MultiAgent.MaxIterations
	}
	if !*modelReview {
		*modelReview = cfg.MultiAgent.ReviewerUseModelReview
	}

	// Validate max iterations
	validatedMaxIter, err := multiagent.ValidateMaxIterations(*maxIterations)
	if err != nil {
		fmt.Fprintf(env.Stderr, "multi-run failed: %v\n", err)
		return 2
	}

	// Build task contract
	contract := task.DefaultContract(root, goal)
	contract.DryRun = *dryRun
	if *approveMedium {
		contract.RequireApprovalForRisk = []string{"high"}
	}

	taskID := "task_multi_" + generateShortID()

	// Build agent dependencies
	agentDeps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "multi-run failed: %v\n", err)
		return 1
	}
	defer cleanup()

	// Build multi-agent dependencies
	multiDeps, multiCleanup, err := buildMultiAgentDependencies(root, cfg, agentDeps)
	if err != nil {
		fmt.Fprintf(env.Stderr, "multi-run failed: %v\n", err)
		return 1
	}
	defer multiCleanup()

	rt := multiagent.NewDefaultMultiAgentRuntime(multiDeps)

	req := multiagent.MultiAgentRunRequest{
		TaskID:         taskID,
		RepoRoot:       root,
		Goal:           goal,
		Contract:       contract,
		MaxIterations:  validatedMaxIter,
		DryRun:         *dryRun,
		UseWorktree:    *useWorktree,
		UseModelReview: *modelReview,
		Model:          *model,
		Metadata:       map[string]string{"source": "cli"},
	}

	fmt.Fprintf(env.Stdout, "ReasonForge Multi-Agent\n")
	fmt.Fprintf(env.Stdout, "goal=%q max_iterations=%d dry_run=%v worktree=%v\n",
		goal, validatedMaxIter, *dryRun, *useWorktree)

	result, err := rt.Run(context.Background(), req)
	if err != nil {
		fmt.Fprintf(env.Stderr, "multi-run failed: %v\n", err)
		return 1
	}

	// Output results
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "run_id=%s\n", result.RunID)
	fmt.Fprintf(env.Stdout, "state=%s\n", result.State)
	if result.WorktreeID != "" {
		fmt.Fprintf(env.Stdout, "worktree_id=%s\n", result.WorktreeID)
	}

	// Plan summary
	if len(result.Plan.Steps) > 0 {
		fmt.Fprintf(env.Stdout, "plan_steps=%d risk_level=%s\n", len(result.Plan.Steps), result.Plan.RiskLevel)
		for _, step := range result.Plan.Steps {
			fmt.Fprintf(env.Stdout, "  step %d: %s\n", step.Index, step.Title)
		}
	}

	// Iteration results
	for _, iter := range result.Iterations {
		fmt.Fprintf(env.Stdout, "iteration %d: recommendation=%s\n", iter.Index, iter.Recommendation)
	}

	// Final recommendation
	fmt.Fprintf(env.Stdout, "final_recommendation=%s\n", result.FinalRecommendation)

	if result.Error != "" {
		fmt.Fprintf(env.Stdout, "error=%s\n", result.Error)
	}

	// If approved, suggest next steps (do NOT auto-apply)
	if result.State == multiagent.MultiAgentStateSucceeded && result.WorktreeID != "" {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "To apply changes, run:\n")
		fmt.Fprintf(env.Stdout, "  reasonforge patch apply %s\n", result.WorktreeID)
	}

	switch result.State {
	case multiagent.MultiAgentStateSucceeded:
		return 0
	case multiagent.MultiAgentStateRejected, multiagent.MultiAgentStateFailed:
		return 1
	case multiagent.MultiAgentStateCancelled:
		return 130
	case multiagent.MultiAgentStateRequestChanges:
		return 1
	default:
		return 1
	}
}

// buildMultiAgentDependencies constructs multi-agent dependencies from project config and existing agent deps.
func buildMultiAgentDependencies(root string, cfg *config.Root, agentDeps agent.Dependencies) (multiagent.Dependencies, func(), error) {
	// Build worktree manager
	wtMgr, wtCleanup, err := buildWorktreeManager(root)
	if err != nil {
		return multiagent.Dependencies{}, nil, fmt.Errorf("worktree manager: %w", err)
	}

	// Build patch manager
	patchMgr := patch.NewGitPatchManager(wtMgr, patchConfigFromConfig(cfg))

	// Build validation runner
	registry := tools.NewMemoryRegistry()
	testCmds := tools.TestCommandsFromConfig(cfg)
	if err := tools.RegisterBuiltinTools(registry, testCmds); err != nil {
		wtCleanup()
		return multiagent.Dependencies{}, nil, fmt.Errorf("tool registry: %w", err)
	}
	enabledMap := tools.EnabledToolsFromConfig(cfg)
	guard := tools.SafetyGuardFromConfig(cfg)
	auditPath := tools.DefaultAuditLogPath(root)
	auditLog, err := tools.NewAuditLog(auditPath)
	if err != nil {
		wtCleanup()
		return multiagent.Dependencies{}, nil, fmt.Errorf("audit log: %w", err)
	}
	toolRt := tools.NewDefaultToolRuntime(registry, guard, auditLog, enabledMap)

	valCfg := validationConfigFromConfig(cfg)
	valRunner := validation.NewValidationRunner(toolRt, valCfg)

	// Build model reviewer - always create so --model-review CLI flag can enable it
	var modelReviewerInst review.ModelReviewer
	modelReviewerInst = review.NewDefaultModelReviewer(agentDeps.ModelRouter)

	reviewCfg := reviewConfigFromConfig(cfg)
	reviewMgr := review.NewDefaultPatchReviewManager(patchMgr, valRunner, modelReviewerInst, reviewCfg)

	// Inject event emitter into sub-components if available
	if agentDeps.EventEmitter != nil {
		emitter := agentDeps.EventEmitter
		patchMgr.SetEventEmitter(emitter)
		valRunner.SetEventEmitter(emitter)
		reviewMgr.SetEventEmitter(emitter)
	}

	// Build checkpoint store
	cpPath := multiagent.DefaultMultiAgentCheckpointPath(root)
	cpStore, err := multiagent.NewJSONLMultiAgentCheckpointStore(cpPath)
	if err != nil {
		auditLog.Close()
		wtCleanup()
		return multiagent.Dependencies{}, nil, fmt.Errorf("multi-agent checkpoint store: %w", err)
	}

	// Build single agent runtime for the coder
	singleAgent := agent.NewSingleAgentRuntime(agentDeps)

	deps := multiagent.Dependencies{
		ContextEngine:   agentDeps.ContextEngine,
		ModelRouter:     agentDeps.ModelRouter,
		SingleAgent:     singleAgent,
		ReviewMgr:       reviewMgr,
		WorktreeMgr:     wtMgr,
		CheckpointStore: cpStore,
		EventEmitter:    agentDeps.EventEmitter,
	}

	cleanup := func() {
		auditLog.Close()
		wtCleanup()
	}

	return deps, cleanup, nil
}

// buildAgentDependencies constructs all agent dependencies from the project config.
// It returns the dependencies, a cleanup function, and any error.
func buildAgentDependencies(root string, cfg *config.Root) (agent.Dependencies, func(), error) {
	cacheRegistryPath := cfg.Prefix.Cache.RegistryPath
	if !filepath.IsAbs(cacheRegistryPath) {
		cacheRegistryPath = filepath.Join(root, cacheRegistryPath)
	}

	cacheRegistry, err := cache.NewJSONLCacheRegistry(cacheRegistryPath)
	if err != nil {
		return agent.Dependencies{}, nil, fmt.Errorf("cache registry: %w", err)
	}
	if strings.TrimSpace(cfg.Prefix.Cache.EstimatedTTL) != "" {
		ttl, err := time.ParseDuration(cfg.Prefix.Cache.EstimatedTTL)
		if err != nil {
			return agent.Dependencies{}, nil, fmt.Errorf("cache ttl: %w", err)
		}
		cacheRegistry.SetTTL(ttl)
	}

	prefixBuilder := prefix.NewImmutablePrefixBuilder(cfg.Prefix.ByteStable)
	conversationLog := conversation.NewJSONLConversationLog(filepath.Join(root, config.DirName, "logs", "conversations"))
	scratch := scratchpad.NewVolatileScratchpad()
	budgetGuard, err := contextengine.NewBudgetGuard(contextengine.BudgetThresholds{
		WarnRatio:  cfg.Prefix.Budget.WarnRatio,
		BlockRatio: cfg.Prefix.Budget.BlockRatio,
	})
	if err != nil {
		return agent.Dependencies{}, nil, fmt.Errorf("budget guard: %w", err)
	}
	contextEngine := contextengine.NewDefaultContextEngine(prefixBuilder, conversationLog, scratch, cacheRegistry, budgetGuard, root, cfg.Prefix)

	providers := make(map[string]modelrouter.Provider, len(cfg.Models.Providers))
	for _, providerCfg := range cfg.Models.Providers {
		models := make([]string, 0, len(providerCfg.Models))
		for _, modelCfg := range providerCfg.Models {
			models = append(models, modelCfg.Name)
		}
		providers[providerCfg.Name] = modelrouter.NewOpenAICompatibleProvider(
			providerCfg.Name,
			providerCfg.BaseURL,
			providerCfg.APIKeyEnv,
			models,
			nil,
		)
	}

	fallbackChain, err := modelrouter.BuildFallbackChainFromConfig(cfg)
	if err != nil {
		return agent.Dependencies{}, nil, fmt.Errorf("model routing: %w", err)
	}
	modelRouter := modelrouter.NewDefaultModelRouter(providers, fallbackChain, cfg.Models.Routing.DefaultModel, cacheRegistry)

	registry := tools.NewMemoryRegistry()
	testCmds := tools.TestCommandsFromConfig(cfg)
	if err := tools.RegisterBuiltinTools(registry, testCmds); err != nil {
		return agent.Dependencies{}, nil, err
	}

	enabledMap := tools.EnabledToolsFromConfig(cfg)
	guard := tools.SafetyGuardFromConfig(cfg)
	auditPath := tools.DefaultAuditLogPath(root)
	auditLog, err := tools.NewAuditLog(auditPath)
	if err != nil {
		return agent.Dependencies{}, nil, fmt.Errorf("audit log: %w", err)
	}

	toolRt := tools.NewDefaultToolRuntime(registry, guard, auditLog, enabledMap)

	checkpointPath := agent.DefaultCheckpointPath(root)
	checkpointStore, err := agent.NewJSONLCheckpointStore(checkpointPath)
	if err != nil {
		auditLog.Close()
		return agent.Dependencies{}, nil, fmt.Errorf("checkpoint store: %w", err)
	}

	deps := agent.Dependencies{
		ContextEngine:   contextEngine,
		ModelRouter:     modelRouter,
		ToolRuntime:     toolRt,
		ToolRegistry:    registry,
		ConversationLog: conversationLog,
		Scratchpad:      scratch,
		CheckpointStore: checkpointStore,
	}

	cleanup := func() { auditLog.Close() }

	// Set up event system if enabled
	if cfg.Events.Enabled {
		eventStorePath := cfg.Events.StorePath
		if !filepath.IsAbs(eventStorePath) {
			eventStorePath = filepath.Join(root, eventStorePath)
		}
		eventStore, eventErr := events.NewJSONLRunEventStore(eventStorePath)
		if eventErr == nil {
			bus := events.NewDefaultEventBus(eventStore)
			deps.EventEmitter = events.NewEventEmitter(bus)
			// Close event store on cleanup
			prevCleanup := cleanup
			cleanup = func() {
				eventStore.Close()
				prevCleanup()
			}
		}
		// EventStore initialization failure is non-fatal;
		// the runtime proceeds without event recording.
	}

	return deps, cleanup, nil
}

func generateShortID() string {
	b := make([]byte, 4)
	// Use crypto/rand for uniqueness but fall back to timestamp
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// runPatch handles the "patch" subcommand.
func runPatch(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "patch requires a subcommand: list, preview, validate, review, apply, discard")
		return 2
	}

	switch args[0] {
	case "list":
		return runPatchList(args[1:], env)
	case "preview":
		return runPatchPreview(args[1:], env)
	case "validate":
		return runPatchValidate(args[1:], env)
	case "review":
		return runPatchReview(args[1:], env)
	case "apply":
		return runPatchApply(args[1:], env)
	case "discard":
		return runPatchDiscard(args[1:], env)
	default:
		fmt.Fprintf(env.Stderr, "unknown patch subcommand %q (use: list, preview, validate, review, apply, discard)\n", args[0])
		return 2
	}
}

// runPatchList lists all worktrees managed by ReasonForge.
func runPatchList(args []string, env Env) int {
	fs := flag.NewFlagSet("patch list", flag.ContinueOnError)
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

	mgr, cleanup, err := buildWorktreeManager(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch list failed: %v\n", err)
		return 1
	}
	defer cleanup()

	worktrees, err := mgr.List(context.Background())
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch list failed: %v\n", err)
		return 1
	}

	if len(worktrees) == 0 {
		fmt.Fprintln(env.Stdout, "No worktrees found.")
		return 0
	}

	fmt.Fprintln(env.Stdout, "ReasonForge Worktrees")
	for _, wt := range worktrees {
		fmt.Fprintf(env.Stdout, "id=%s task_id=%s state=%s path=%s created_at=%s\n",
			wt.ID, wt.TaskID, wt.State, filepath.ToSlash(wt.Path), wt.CreatedAt.Format(time.RFC3339))
	}
	return 0
}

// runPatchPreview shows the diff preview for a worktree.
func runPatchPreview(args []string, env Env) int {
	fs := flag.NewFlagSet("patch preview", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "patch preview requires a worktree ID")
		return 2
	}
	wtID := remaining[0]

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch preview failed: %v\n", err)
		return 1
	}

	wtMgr, cleanup, err := buildWorktreeManager(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch preview failed: %v\n", err)
		return 1
	}
	defer cleanup()

	patchMgr := patch.NewGitPatchManager(wtMgr, patchConfigFromConfig(cfg))

	contract := task.DefaultContract(root, "patch preview")

	preview, err := patchMgr.Preview(context.Background(), patch.PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: wtID,
		Contract:   contract,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch preview failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "worktree_id=%s\n", preview.WorktreeID)
	fmt.Fprintf(env.Stdout, "files_changed=%d\n", preview.Summary.FilesChanged)
	fmt.Fprintf(env.Stdout, "additions=%d deletions=%d\n", preview.Summary.Additions, preview.Summary.Deletions)
	fmt.Fprintf(env.Stdout, "has_binary=%v\n", preview.Summary.HasBinary)
	fmt.Fprintf(env.Stdout, "risk_level=%s\n", preview.RiskLevel)

	if len(preview.Violations) > 0 {
		fmt.Fprintf(env.Stdout, "violations=%d\n", len(preview.Violations))
		for _, v := range preview.Violations {
			fmt.Fprintf(env.Stdout, "  violation: path=%s reason=%s\n", v.Path, v.Reason)
		}
	}

	for _, f := range preview.FilesChanged {
		fmt.Fprintf(env.Stdout, "  file: path=%s status=%s additions=%d deletions=%d\n", f.Path, f.Status, f.Additions, f.Deletions)
	}

	if preview.Diff != "" && len(preview.Violations) == 0 {
		fmt.Fprintln(env.Stdout, "--- diff ---")
		// Truncate diff output to respect max_output_bytes
		maxBytes := cfg.Patch.MaxDiffBytes
		if maxBytes <= 0 {
			maxBytes = 131072
		}
		diff := preview.Diff
		if len(diff) > maxBytes {
			diff = diff[:maxBytes] + "\n... (truncated)"
		}
		fmt.Fprint(env.Stdout, diff)
	}

	return 0
}

// runPatchApply applies a worktree's changes to the main workspace.
func runPatchApply(args []string, env Env) int {
	fs := flag.NewFlagSet("patch apply", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	dryRun := fs.Bool("dry-run", false, "dry run: show what would be applied without modifying files")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "patch apply requires a worktree ID")
		return 2
	}
	wtID := remaining[0]

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch apply failed: %v\n", err)
		return 1
	}

	wtMgr, cleanup, err := buildWorktreeManager(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch apply failed: %v\n", err)
		return 1
	}
	defer cleanup()

	patchMgr := patch.NewGitPatchManager(wtMgr, patchConfigFromConfig(cfg))

	contract := task.DefaultContract(root, "patch apply")

	result, err := patchMgr.Apply(context.Background(), patch.PatchApplyRequest{
		RepoRoot:     root,
		WorktreeID:   wtID,
		Contract:     contract,
		DryRun:       *dryRun,
		MaxDiffBytes: cfg.Patch.MaxDiffBytes,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch apply failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "worktree_id=%s applied=%v\n", result.WorktreeID, result.Applied)
	fmt.Fprintf(env.Stdout, "files_changed=%d additions=%d deletions=%d\n",
		result.Summary.FilesChanged, result.Summary.Additions, result.Summary.Deletions)

	if !result.Applied && !*dryRun {
		fmt.Fprintln(env.Stdout, "No changes were applied.")
	}

	return 0
}

// runPatchDiscard discards a worktree and its changes.
func runPatchDiscard(args []string, env Env) int {
	fs := flag.NewFlagSet("patch discard", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "patch discard requires a worktree ID")
		return 2
	}
	wtID := remaining[0]

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch discard failed: %v\n", err)
		return 1
	}

	wtMgr, cleanup, err := buildWorktreeManager(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch discard failed: %v\n", err)
		return 1
	}
	defer cleanup()

	patchMgr := patch.NewGitPatchManager(wtMgr, patchConfigFromConfig(cfg))

	if err := patchMgr.Discard(context.Background(), patch.PatchDiscardRequest{
		RepoRoot:   root,
		WorktreeID: wtID,
	}); err != nil {
		fmt.Fprintf(env.Stderr, "patch discard failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "worktree_id=%s state=discarded\n", wtID)
	return 0
}

// runPatchValidate executes patch validation: preview + rule review + test validation.
// Does NOT call model review. Does NOT auto-apply.
func runPatchValidate(args []string, env Env) int {
	fs := flag.NewFlagSet("patch validate", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Collect --test-command flags from remaining args
	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "patch validate requires a worktree ID")
		return 2
	}
	wtID := remaining[0]

	// Parse additional --test-command flags from remaining positional args
	var testCommands []string
	for i := 1; i < len(remaining); i++ {
		if remaining[i] == "--test-command" && i+1 < len(remaining) {
			testCommands = append(testCommands, remaining[i+1])
			i++
		}
	}

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}

	wtMgr, cleanup, err := buildWorktreeManager(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}
	defer cleanup()

	// Get worktree path for validation
	wtInfo, err := wtMgr.Get(context.Background(), wtID)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}

	patchMgr := patch.NewGitPatchManager(wtMgr, patchConfigFromConfig(cfg))

	// Build validation runner
	registry := tools.NewMemoryRegistry()
	testCmds := tools.TestCommandsFromConfig(cfg)
	if err := tools.RegisterBuiltinTools(registry, testCmds); err != nil {
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}
	enabledMap := tools.EnabledToolsFromConfig(cfg)
	guard := tools.SafetyGuardFromConfig(cfg)
	auditPath := tools.DefaultAuditLogPath(root)
	auditLog, err := tools.NewAuditLog(auditPath)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}
	defer auditLog.Close()
	toolRt := tools.NewDefaultToolRuntime(registry, guard, auditLog, enabledMap)

	valCfg := validationConfigFromConfig(cfg)
	valRunner := validation.NewValidationRunner(toolRt, valCfg)

	reviewCfg := reviewConfigFromConfig(cfg)
	reviewMgr := review.NewDefaultPatchReviewManager(patchMgr, valRunner, nil, reviewCfg)

	// Use default test commands if none specified
	if len(testCommands) == 0 {
		testCommands = cfg.Validation.DefaultTestCommands
	}

	contract := task.DefaultContract(root, "patch validate")

	report, err := reviewMgr.Review(context.Background(), review.PatchReviewRequest{
		RepoRoot:       root,
		WorktreeID:     wtID,
		WorktreePath:   wtInfo.Path, // Validation runs in worktree, not main workspace
		Contract:       contract,
		RunTests:       true,
		TestCommands:   testCommands,
		UseModelReview: false,
		MaxDiffBytes:   cfg.Review.MaxDiffBytes,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}

	printReviewReport(env, report, cfg)

	if report.Recommendation == review.RecommendationReject {
		return 1
	}
	if report.Recommendation == review.RecommendationRequestChanges {
		return 1
	}
	return 0
}

// runPatchReview executes patch review: preview + rule review + optional model review + optional test validation.
// Does NOT auto-apply.
func runPatchReview(args []string, env Env) int {
	fs := flag.NewFlagSet("patch review", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	modelReview := fs.Bool("model-review", false, "use AI model review")
	model := fs.String("model", "", "model name for review (default: from config)")
	noTests := fs.Bool("no-tests", false, "skip test validation")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "patch review requires a worktree ID")
		return 2
	}
	wtID := remaining[0]

	// Parse --test-command flags
	var testCommands []string
	for i := 1; i < len(remaining); i++ {
		if remaining[i] == "--test-command" && i+1 < len(remaining) {
			testCommands = append(testCommands, remaining[i+1])
			i++
		}
	}

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}

	wtMgr, cleanup, err := buildWorktreeManager(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}
	defer cleanup()

	// Get worktree path for validation
	wtInfo, err := wtMgr.Get(context.Background(), wtID)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}

	patchMgr := patch.NewGitPatchManager(wtMgr, patchConfigFromConfig(cfg))

	// Build validation runner
	registry := tools.NewMemoryRegistry()
	testCmds := tools.TestCommandsFromConfig(cfg)
	if err := tools.RegisterBuiltinTools(registry, testCmds); err != nil {
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}
	enabledMap := tools.EnabledToolsFromConfig(cfg)
	guard := tools.SafetyGuardFromConfig(cfg)
	auditPath := tools.DefaultAuditLogPath(root)
	auditLog, err := tools.NewAuditLog(auditPath)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}
	defer auditLog.Close()
	toolRt := tools.NewDefaultToolRuntime(registry, guard, auditLog, enabledMap)

	valCfg := validationConfigFromConfig(cfg)
	valRunner := validation.NewValidationRunner(toolRt, valCfg)

	// Build model reviewer if requested
	var modelReviewerInst review.ModelReviewer
	if *modelReview {
		providers := make(map[string]modelrouter.Provider, len(cfg.Models.Providers))
		for _, providerCfg := range cfg.Models.Providers {
			models := make([]string, 0, len(providerCfg.Models))
			for _, modelCfg := range providerCfg.Models {
				models = append(models, modelCfg.Name)
			}
			providers[providerCfg.Name] = modelrouter.NewOpenAICompatibleProvider(
				providerCfg.Name,
				providerCfg.BaseURL,
				providerCfg.APIKeyEnv,
				models,
				nil,
			)
		}
		fallbackChain, _ := modelrouter.BuildFallbackChainFromConfig(cfg)
		cacheRegistryPath := cfg.Prefix.Cache.RegistryPath
		if !filepath.IsAbs(cacheRegistryPath) {
			cacheRegistryPath = filepath.Join(root, cacheRegistryPath)
		}
		cacheRegistry, _ := cache.NewJSONLCacheRegistry(cacheRegistryPath)
		modelRouter := modelrouter.NewDefaultModelRouter(providers, fallbackChain, cfg.Models.Routing.DefaultModel, cacheRegistry)
		modelReviewerInst = review.NewDefaultModelReviewer(modelRouter)
	}

	reviewCfg := reviewConfigFromConfig(cfg)
	reviewMgr := review.NewDefaultPatchReviewManager(patchMgr, valRunner, modelReviewerInst, reviewCfg)

	// Use default test commands if none specified
	if len(testCommands) == 0 {
		testCommands = cfg.Validation.DefaultTestCommands
	}

	runTests := !*noTests

	contract := task.DefaultContract(root, "patch review")

	report, err := reviewMgr.Review(context.Background(), review.PatchReviewRequest{
		RepoRoot:       root,
		WorktreeID:     wtID,
		WorktreePath:   wtInfo.Path,
		Contract:       contract,
		RunTests:       runTests,
		TestCommands:   testCommands,
		UseModelReview: *modelReview,
		Model:          *model,
		MaxDiffBytes:   cfg.Review.MaxDiffBytes,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}

	printReviewReport(env, report, cfg)

	if report.Recommendation == review.RecommendationReject {
		return 1
	}
	if report.Recommendation == review.RecommendationRequestChanges {
		return 1
	}
	return 0
}

// printReviewReport outputs a PatchReviewReport in text format.
func printReviewReport(env Env, report review.PatchReviewReport, cfg *config.Root) {
	fmt.Fprintf(env.Stdout, "=== Patch Review Report ===\n")
	fmt.Fprintf(env.Stdout, "worktree_id=%s\n", report.WorktreeID)
	fmt.Fprintf(env.Stdout, "recommendation=%s\n", report.Recommendation)
	fmt.Fprintf(env.Stdout, "risk_level=%s risk_score=%d\n", report.RiskScore.Level, report.RiskScore.Score)

	if len(report.RiskScore.Reasons) > 0 {
		fmt.Fprintf(env.Stdout, "risk_reasons:\n")
		for _, r := range report.RiskScore.Reasons {
			fmt.Fprintf(env.Stdout, "  - %s\n", r)
		}
	}

	fmt.Fprintf(env.Stdout, "files_changed=%d additions=%d deletions=%d\n",
		report.Preview.Summary.FilesChanged,
		report.Preview.Summary.Additions,
		report.Preview.Summary.Deletions)
	fmt.Fprintf(env.Stdout, "has_binary=%v\n", report.Preview.Summary.HasBinary)

	if len(report.Preview.Violations) > 0 {
		fmt.Fprintf(env.Stdout, "violations=%d\n", len(report.Preview.Violations))
		for _, v := range report.Preview.Violations {
			fmt.Fprintf(env.Stdout, "  violation: path=%s reason=%s\n", v.Path, v.Reason)
		}
		// Do not print sensitive diff when violations exist
	}

	if len(report.Findings) > 0 {
		fmt.Fprintf(env.Stdout, "findings=%d\n", len(report.Findings))
		for _, f := range report.Findings {
			fmt.Fprintf(env.Stdout, "  finding: severity=%s category=%s path=%s message=%s\n",
				f.Severity, f.Category, f.Path, f.Message)
		}
	}

	if report.Validation != nil {
		fmt.Fprintf(env.Stdout, "validation_success=%v\n", report.Validation.Success)
		fmt.Fprintf(env.Stdout, "validation_summary=%s\n", report.Validation.Summary)
		for _, cmd := range report.Validation.Commands {
			fmt.Fprintf(env.Stdout, "  command: name=%s success=%v exit_code=%d duration_ms=%d\n",
				cmd.CommandName, cmd.Success, cmd.ExitCode, cmd.DurationMs)
		}
	}

	if report.ModelReview != nil {
		fmt.Fprintf(env.Stdout, "model_review:\n")
		fmt.Fprintf(env.Stdout, "  provider=%s model=%s\n", report.ModelReview.Provider, report.ModelReview.Model)
		fmt.Fprintf(env.Stdout, "  summary=%s\n", report.ModelReview.Summary)
		fmt.Fprintf(env.Stdout, "  recommendation=%s\n", report.ModelReview.Recommendation)
		for _, f := range report.ModelReview.Findings {
			fmt.Fprintf(env.Stdout, "  finding: severity=%s category=%s message=%s\n",
				f.Severity, f.Category, f.Message)
		}
	}

	// Only print diff when no violations (don't leak sensitive diff)
	if report.Preview.Diff != "" && len(report.Preview.Violations) == 0 {
		fmt.Fprintln(env.Stdout, "--- diff ---")
		maxBytes := cfg.Patch.MaxDiffBytes
		if maxBytes <= 0 {
			maxBytes = 131072
		}
		diff := report.Preview.Diff
		if len(diff) > maxBytes {
			diff = diff[:maxBytes] + "\n... (truncated)"
		}
		fmt.Fprint(env.Stdout, diff)
	}
}

// reviewConfigFromConfig creates a ReviewConfig from the project config.
func reviewConfigFromConfig(cfg *config.Root) review.ReviewConfig {
	return review.ReviewConfig{
		MaxDiffBytes:               cfg.Review.MaxDiffBytes,
		HighRiskFileCount:          cfg.Review.HighRiskFileCount,
		MediumRiskFileCount:        cfg.Review.MediumRiskFileCount,
		HighRiskLineCount:          cfg.Review.HighRiskLineCount,
		MediumRiskLineCount:        cfg.Review.MediumRiskLineCount,
		RequireTestsForCodeChanges: cfg.Review.RequireTestsForCodeChanges,
		AllowBinary:                cfg.Patch.AllowBinary,
		StrictModelReview:          cfg.Review.StrictModelReview,
	}
}

// validationConfigFromConfig creates a ValidationConfig from the project config.
func validationConfigFromConfig(cfg *config.Root) validation.ValidationConfig {
	return validation.ValidationConfig{
		DefaultTestCommands: cfg.Validation.DefaultTestCommands,
		MaxOutputBytes:      cfg.Validation.MaxOutputBytes,
		TimeoutSeconds:      cfg.Validation.TimeoutSeconds,
	}
}

// buildWorktreeManager creates a WorktreeManager for the given root.
func buildWorktreeManager(root string) (worktree.WorktreeManager, func(), error) {
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		return nil, nil, fmt.Errorf("worktree registry: %w", err)
	}

	cfg := worktree.DefaultGitWorktreeManagerConfig()
	cleanup := func() { registry.Close() }
	return worktree.NewGitWorktreeManager(registry, cfg), cleanup, nil
}

// patchConfigFromConfig creates a GitPatchManagerConfig from the project config.
func patchConfigFromConfig(cfg *config.Root) patch.GitPatchManagerConfig {
	return patch.GitPatchManagerConfig{
		MaxDiffBytes:     cfg.Patch.MaxDiffBytes,
		RequireCleanMain: cfg.Patch.RequireCleanMain,
		AllowBinary:      cfg.Patch.AllowBinary,
	}
}

// openEventStoreForRead opens the JSONLRunEventStore for read-only queries.
// Returns the store, a cleanup function, and any error.
func openEventStoreForRead(root string, cfg *config.Root) (*events.JSONLRunEventStore, func(), error) {
	eventStorePath := cfg.Events.StorePath
	if !filepath.IsAbs(eventStorePath) {
		eventStorePath = filepath.Join(root, eventStorePath)
	}
	store, err := events.NewJSONLRunEventStore(eventStorePath)
	if err != nil {
		return nil, nil, fmt.Errorf("event store: %w", err)
	}
	cleanup := func() { store.Close() }
	return store, cleanup, nil
}

// runRuns handles the "runs" command - lists recent runs.
func runRuns(args []string, env Env) int {
	fs := flag.NewFlagSet("runs", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	limit := fs.Int("limit", 20, "max number of runs to show")
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

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "runs failed: %v\n", err)
		return 1
	}

	if !cfg.Events.Enabled {
		fmt.Fprintln(env.Stderr, "events system is disabled; enable in .reasonforge/events.yaml")
		return 1
	}

	store, cleanup, err := openEventStoreForRead(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "runs failed: %v\n", err)
		return 1
	}
	defer cleanup()

	summaries, err := store.ListRuns(context.Background())
	if err != nil {
		fmt.Fprintf(env.Stderr, "runs failed: %v\n", err)
		return 1
	}

	if len(summaries) == 0 {
		fmt.Fprintln(env.Stdout, "No runs found.")
		return 0
	}

	// Apply limit
	if *limit > 0 && len(summaries) > *limit {
		summaries = summaries[:*limit]
	}

	fmt.Fprintln(env.Stdout, "ReasonForge Runs")
	fmt.Fprintf(env.Stdout, "%-36s %-12s %-20s %s\n", "RUN ID", "STATE", "STARTED", "LAST EVENT")
	for _, s := range summaries {
		started := "-"
		if !s.StartedAt.IsZero() {
			started = s.StartedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(env.Stdout, "%-36s %-12s %-20s %s\n", s.RunID, s.State, started, s.LastEventType)
	}

	return 0
}

// runRunStatus handles the "run-status" command - shows detailed status for a run.
func runRunStatus(args []string, env Env) int {
	fs := flag.NewFlagSet("run-status", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "run-status requires a run ID")
		return 2
	}
	runID := remaining[0]

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-status failed: %v\n", err)
		return 1
	}

	if !cfg.Events.Enabled {
		fmt.Fprintln(env.Stderr, "events system is disabled; enable in .reasonforge/events.yaml")
		return 1
	}

	store, cleanup, err := openEventStoreForRead(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-status failed: %v\n", err)
		return 1
	}
	defer cleanup()

	timeline, err := store.GetTimeline(context.Background(), runID)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-status failed: %v\n", err)
		return 1
	}

	if timeline.RunID == "" {
		fmt.Fprintf(env.Stderr, "run %q not found\n", runID)
		return 1
	}

	progress := events.ComputeProgressState(timeline)

	fmt.Fprintf(env.Stdout, "run_id=%s\n", progress.RunID)
	fmt.Fprintf(env.Stdout, "state=%s\n", progress.State)
	if progress.CurrentPhase != "" {
		fmt.Fprintf(env.Stdout, "current_phase=%s\n", progress.CurrentPhase)
	}
	fmt.Fprintf(env.Stdout, "percent=%d\n", progress.Percent)
	fmt.Fprintf(env.Stdout, "completed_steps=%d\n", progress.CompletedSteps)
	fmt.Fprintf(env.Stdout, "total_steps=%d\n", progress.TotalSteps)
	fmt.Fprintf(env.Stdout, "last_event=%s\n", progress.LastEvent.Type)
	if progress.LastEvent.WorktreeID != "" {
		fmt.Fprintf(env.Stdout, "worktree_id=%s\n", progress.LastEvent.WorktreeID)
	}
	if !timeline.StartedAt.IsZero() {
		fmt.Fprintf(env.Stdout, "started_at=%s\n", timeline.StartedAt.Format(time.RFC3339))
	}
	if !timeline.FinishedAt.IsZero() {
		fmt.Fprintf(env.Stdout, "finished_at=%s\n", timeline.FinishedAt.Format(time.RFC3339))
	}
	if timeline.DurationMs > 0 {
		fmt.Fprintf(env.Stdout, "duration_ms=%d\n", timeline.DurationMs)
	}

	return 0
}

// runRunEvents handles the "run-events" command - shows events for a run.
func runRunEvents(args []string, env Env) int {
	fs := flag.NewFlagSet("run-events", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "run-events requires a run ID")
		return 2
	}
	runID := remaining[0]

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-events failed: %v\n", err)
		return 1
	}

	if !cfg.Events.Enabled {
		fmt.Fprintln(env.Stderr, "events system is disabled; enable in .reasonforge/events.yaml")
		return 1
	}

	store, cleanup, err := openEventStoreForRead(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-events failed: %v\n", err)
		return 1
	}
	defer cleanup()

	evts, err := store.ListEvents(context.Background(), runID)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-events failed: %v\n", err)
		return 1
	}

	if len(evts) == 0 {
		fmt.Fprintf(env.Stderr, "no events found for run %q\n", runID)
		return 1
	}

	fmt.Fprintln(env.Stdout, "ReasonForge Run Events")
	fmt.Fprintf(env.Stdout, "%-20s %-24s %-12s %s\n", "TIME", "TYPE", "STATUS", "MESSAGE")
	for _, evt := range evts {
		ts := "-"
		if !evt.StartedAt.IsZero() {
			ts = evt.StartedAt.Format("15:04:05.000")
		} else if !evt.FinishedAt.IsZero() {
			ts = evt.FinishedAt.Format("15:04:05.000")
		}
		msg := evt.Message
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		fmt.Fprintf(env.Stdout, "%-20s %-24s %-12s %s\n", ts, evt.Type, evt.Status, msg)
	}

	return 0
}
