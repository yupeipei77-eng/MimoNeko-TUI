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
	"github.com/reasonforge/reasonforge/internal/modelrouter"
	"github.com/reasonforge/reasonforge/internal/patch"
	"github.com/reasonforge/reasonforge/internal/prefix"
	"github.com/reasonforge/reasonforge/internal/scratchpad"
	"github.com/reasonforge/reasonforge/internal/task"
	"github.com/reasonforge/reasonforge/internal/tools"
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
	fmt.Fprintln(w, "  patch        Manage patches (list, preview, apply, discard)")
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
		fmt.Fprintln(env.Stderr, "patch requires a subcommand: list, preview, apply, discard")
		return 2
	}

	switch args[0] {
	case "list":
		return runPatchList(args[1:], env)
	case "preview":
		return runPatchPreview(args[1:], env)
	case "apply":
		return runPatchApply(args[1:], env)
	case "discard":
		return runPatchDiscard(args[1:], env)
	default:
		fmt.Fprintf(env.Stderr, "unknown patch subcommand %q (use: list, preview, apply, discard)\n", args[0])
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
