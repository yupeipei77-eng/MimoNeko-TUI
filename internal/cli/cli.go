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
	"github.com/reasonforge/reasonforge/internal/prefix"
	"github.com/reasonforge/reasonforge/internal/scratchpad"
	"github.com/reasonforge/reasonforge/internal/task"
	"github.com/reasonforge/reasonforge/internal/tools"
	"github.com/reasonforge/reasonforge/internal/version"
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

	cacheRegistryPath := cfg.Prefix.Cache.RegistryPath
	if !filepath.IsAbs(cacheRegistryPath) {
		cacheRegistryPath = filepath.Join(root, cacheRegistryPath)
	}

	cacheRegistry, err := cache.NewJSONLCacheRegistry(cacheRegistryPath)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run failed: cache registry: %v\n", err)
		return 1
	}
	if strings.TrimSpace(cfg.Prefix.Cache.EstimatedTTL) != "" {
		ttl, err := time.ParseDuration(cfg.Prefix.Cache.EstimatedTTL)
		if err != nil {
			fmt.Fprintf(env.Stderr, "run failed: cache ttl: %v\n", err)
			return 1
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
		fmt.Fprintf(env.Stderr, "run failed: budget guard: %v\n", err)
		return 1
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
		fmt.Fprintf(env.Stderr, "run failed: model routing: %v\n", err)
		return 1
	}
	modelRouter := modelrouter.NewDefaultModelRouter(providers, fallbackChain, cfg.Models.Routing.DefaultModel, cacheRegistry)

	registry := tools.NewMemoryRegistry()
	testCmds := tools.TestCommandsFromConfig(cfg)
	if err := tools.RegisterBuiltinTools(registry, testCmds); err != nil {
		fmt.Fprintf(env.Stderr, "run failed: %v\n", err)
		return 1
	}

	enabledMap := tools.EnabledToolsFromConfig(cfg)
	guard := tools.SafetyGuardFromConfig(cfg)
	auditPath := tools.DefaultAuditLogPath(root)
	auditLog, err := tools.NewAuditLog(auditPath)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run failed: audit log: %v\n", err)
		return 1
	}
	defer auditLog.Close()

	toolRt := tools.NewDefaultToolRuntime(registry, guard, auditLog, enabledMap)

	checkpointPath := agent.DefaultCheckpointPath(root)
	checkpointStore, err := agent.NewJSONLCheckpointStore(checkpointPath)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run failed: checkpoint store: %v\n", err)
		return 1
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

	rt := agent.NewSingleAgentRuntime(deps)
	rt.SetOutput(env.Stdout)

	policy := agent.InteractiveApprovalPolicy(os.Stdin)
	policy.AutoApproveMediumRisk = *autoApproveMedium
	rt.SetApprovalPolicy(policy)

	req := agent.AgentRunRequest{
		TaskID:   tid,
		RepoRoot: root,
		Goal:     *goal,
		Contract: contract,
		MaxSteps: *maxSteps,
		DryRun:   *dryRun,
	}

	fmt.Fprintf(env.Stdout, "ReasonForge Agent\n")
	fmt.Fprintf(env.Stdout, "run_id=pending goal=%q max_steps=%d dry_run=%v\n", *goal, contract.MaxSteps, contract.DryRun)

	result, err := rt.Run(context.Background(), req)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "run_id=%s state=%s steps=%d\n", result.RunID, result.State, len(result.Steps))
	if result.FinalMessage != "" {
		fmt.Fprintf(env.Stdout, "message=%s\n", result.FinalMessage)
	}
	if result.Error != "" {
		fmt.Fprintf(env.Stdout, "error=%s\n", result.Error)
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

func generateShortID() string {
	b := make([]byte, 4)
	// Use crypto/rand for uniqueness but fall back to timestamp
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
