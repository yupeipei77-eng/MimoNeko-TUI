package cli

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mimoneko/mimoneko/internal/cache"
	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/modelrouter"
	"github.com/mimoneko/mimoneko/internal/patch"
	"github.com/mimoneko/mimoneko/internal/review"
	"github.com/mimoneko/mimoneko/internal/security"
	"github.com/mimoneko/mimoneko/internal/task"
	"github.com/mimoneko/mimoneko/internal/tools"
	"github.com/mimoneko/mimoneko/internal/validation"
)

type PatchCommand struct{}

func (c *PatchCommand) Name() string { return "patch" }

func (c *PatchCommand) Run(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "patch requires a subcommand: list, preview, validate, review, apply, discard")
		return 2
	}

	switch args[0] {
	case "list":
		return c.runList(args[1:], env)
	case "preview":
		return c.runPreview(args[1:], env)
	case "validate":
		return c.runValidate(args[1:], env)
	case "review":
		return c.runReview(args[1:], env)
	case "apply":
		return c.runApply(args[1:], env)
	case "discard":
		return c.runDiscard(args[1:], env)
	default:
		fmt.Fprintf(env.Stderr, "unknown patch subcommand %q (use: list, preview, validate, review, apply, discard)\n", args[0])
		return 2
	}
}

func (c *PatchCommand) runList(args []string, env Env) int {
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
		PrintErrorDetails(env.Stderr, "Patch list failed", "Unable to load the worktree manager.", "Run: mimoneko doctor", err.Error())
		return 1
	}
	defer cleanup()

	worktrees, err := mgr.List(context.Background())
	if err != nil {
		PrintErrorDetails(env.Stderr, "Patch list failed", "Unable to read patch/worktree state.", "Check .mimoneko/worktrees, then run: mimoneko doctor", err.Error())
		return 1
	}

	PrintHeader(env.Stdout, "Patch List")
	if len(worktrees) == 0 {
		PrintInfo(env.Stdout, "No worktrees found.")
		return 0
	}

	for _, wt := range worktrees {
		PrintKV(env.Stdout, "", []KV{
			{Key: "ID", Value: wt.ID},
			{Key: "Task", Value: wt.TaskID},
			{Key: "State", Value: string(wt.State)},
			{Key: "Path", Value: filepath.ToSlash(wt.Path)},
			{Key: "Created", Value: wt.CreatedAt.Format(time.RFC3339)},
		})
		fmt.Fprintln(env.Stdout)
	}
	return 0
}

func (c *PatchCommand) runPreview(args []string, env Env) int {
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

	emitter, eventCleanup, err := buildEventEmitter(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch preview failed: %v\n", err)
		return 1
	}
	defer eventCleanup()

	wtMgr, cleanup, err := buildWorktreeManager(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch preview failed: %v\n", err)
		return 1
	}
	defer cleanup()

	patchMgr := patch.NewGitPatchManager(wtMgr, patchConfigFromConfig(cfg))

	wtInfo, err := wtMgr.Get(context.Background(), wtID)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch preview failed: %v\n", err)
		return 1
	}

	eventRun := beginPatchCLIEventRun(context.Background(), emitter, "patch_preview", wtInfo.TaskID, wtID, "Preview command started")
	patchMgr.SetEventEmitter(eventRun.emitter)

	contract := task.DefaultContract(root, "patch preview")
	if wtInfo.TaskID != "" {
		contract.ID = wtInfo.TaskID
	}

	preview, err := patchMgr.Preview(eventRun.ctx, patch.PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: wtID,
		Contract:   contract,
	})
	if err != nil {
		eventRun.finish(false, "Preview command failed", err)
		fmt.Fprintf(env.Stderr, "patch preview failed: %v\n", err)
		return 1
	}
	eventRun.finish(true, "Preview command succeeded", nil)

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

func (c *PatchCommand) runApply(args []string, env Env) int {
	fs := flag.NewFlagSet("patch apply", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	dryRun := fs.Bool("dry-run", false, "dry run: show what would be applied without modifying files")
	approve := fs.Bool("approve", false, "confirm patch application after preview")
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
		RepoRoot:       root,
		WorktreeID:     wtID,
		Contract:       contract,
		DryRun:         *dryRun,
		MaxDiffBytes:   cfg.Patch.MaxDiffBytes,
		PermissionMode: security.GetPermissionMode(),
		Approved:       *approve,
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

func (c *PatchCommand) runDiscard(args []string, env Env) int {
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

func (c *PatchCommand) runValidate(args []string, env Env) int {
	fs := flag.NewFlagSet("patch validate", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "patch validate requires a worktree ID")
		return 2
	}
	wtID := remaining[0]

	var testCommands []string
	explicitTestCommands := false
	for i := 1; i < len(remaining); i++ {
		if remaining[i] == "--test-command" && i+1 < len(remaining) {
			testCommands = append(testCommands, remaining[i+1])
			explicitTestCommands = true
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

	emitter, eventCleanup, err := buildEventEmitter(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}
	defer eventCleanup()

	wtMgr, cleanup, err := buildWorktreeManager(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}
	defer cleanup()

	wtInfo, err := wtMgr.Get(context.Background(), wtID)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}

	eventRun := beginPatchCLIEventRun(context.Background(), emitter, "patch_validate", wtInfo.TaskID, wtID, "Validate command started")

	patchMgr := patch.NewGitPatchManager(wtMgr, patchConfigFromConfig(cfg))
	patchMgr.SetEventEmitter(eventRun.emitter)

	registry := tools.NewMemoryRegistry()
	testCmds := tools.TestCommandsFromConfig(cfg)
	if err := tools.RegisterBuiltinTools(registry, testCmds); err != nil {
		eventRun.finish(false, "Validate command failed", err)
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}
	enabledMap := tools.EnabledToolsFromConfig(cfg)
	guard := tools.SafetyGuardFromConfig(cfg)
	auditPath := tools.DefaultAuditLogPath(root)
	auditLog, err := tools.NewAuditLog(auditPath)
	if err != nil {
		eventRun.finish(false, "Validate command failed", err)
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}
	defer auditLog.Close()
	toolRt := tools.NewDefaultToolRuntime(registry, guard, auditLog, enabledMap)
	toolRt.SetEventEmitter(eventRun.emitter)

	valCfg := validationConfigFromConfig(cfg)
	valRunner := validation.NewValidationRunner(toolRt, valCfg)
	valRunner.SetEventEmitter(eventRun.emitter)

	reviewCfg := reviewConfigFromConfig(cfg)
	reviewMgr := review.NewDefaultPatchReviewManager(patchMgr, valRunner, nil, reviewCfg)
	reviewMgr.SetEventEmitter(eventRun.emitter)

	if len(testCommands) == 0 {
		testCommands = cfg.Validation.DefaultTestCommands
	}

	contract := task.DefaultContract(root, "patch validate")
	if wtInfo.TaskID != "" {
		contract.ID = wtInfo.TaskID
	}

	report, err := reviewMgr.Review(eventRun.ctx, review.PatchReviewRequest{
		RepoRoot:       root,
		WorktreeID:     wtID,
		WorktreePath:   wtInfo.Path,
		Contract:       contract,
		RunTests:       true,
		TestCommands:   testCommands,
		ForceTests:     explicitTestCommands,
		UseModelReview: false,
		MaxDiffBytes:   cfg.Review.MaxDiffBytes,
	})
	if err != nil {
		eventRun.finish(false, "Validate command failed", err)
		fmt.Fprintf(env.Stderr, "patch validate failed: %v\n", err)
		return 1
	}

	printReviewReport(env, report, cfg)

	if report.Recommendation == review.RecommendationReject {
		eventRun.finish(false, "Validate command rejected", fmt.Errorf("recommendation=%s", report.Recommendation))
		return 1
	}
	if report.Recommendation == review.RecommendationRequestChanges {
		eventRun.finish(false, "Validate command requested changes", fmt.Errorf("recommendation=%s", report.Recommendation))
		return 1
	}
	eventRun.finish(true, "Validate command succeeded", nil)
	return 0
}

func (c *PatchCommand) runReview(args []string, env Env) int {
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

	var testCommands []string
	explicitTestCommands := false
	for i := 1; i < len(remaining); i++ {
		if remaining[i] == "--test-command" && i+1 < len(remaining) {
			testCommands = append(testCommands, remaining[i+1])
			explicitTestCommands = true
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

	emitter, eventCleanup, err := buildEventEmitter(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}
	defer eventCleanup()

	wtMgr, cleanup, err := buildWorktreeManager(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}
	defer cleanup()

	wtInfo, err := wtMgr.Get(context.Background(), wtID)
	if err != nil {
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}

	eventRun := beginPatchCLIEventRun(context.Background(), emitter, "patch_review", wtInfo.TaskID, wtID, "Review command started")

	patchMgr := patch.NewGitPatchManager(wtMgr, patchConfigFromConfig(cfg))
	patchMgr.SetEventEmitter(eventRun.emitter)

	registry := tools.NewMemoryRegistry()
	testCmds := tools.TestCommandsFromConfig(cfg)
	if err := tools.RegisterBuiltinTools(registry, testCmds); err != nil {
		eventRun.finish(false, "Review command failed", err)
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}
	enabledMap := tools.EnabledToolsFromConfig(cfg)
	guard := tools.SafetyGuardFromConfig(cfg)
	auditPath := tools.DefaultAuditLogPath(root)
	auditLog, err := tools.NewAuditLog(auditPath)
	if err != nil {
		eventRun.finish(false, "Review command failed", err)
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}
	defer auditLog.Close()
	toolRt := tools.NewDefaultToolRuntime(registry, guard, auditLog, enabledMap)
	toolRt.SetEventEmitter(eventRun.emitter)

	valCfg := validationConfigFromConfig(cfg)
	valRunner := validation.NewValidationRunner(toolRt, valCfg)
	valRunner.SetEventEmitter(eventRun.emitter)

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
	reviewMgr.SetEventEmitter(eventRun.emitter)

	if len(testCommands) == 0 {
		testCommands = cfg.Validation.DefaultTestCommands
	}

	runTests := !*noTests

	contract := task.DefaultContract(root, "patch review")
	if wtInfo.TaskID != "" {
		contract.ID = wtInfo.TaskID
	}

	report, err := reviewMgr.Review(eventRun.ctx, review.PatchReviewRequest{
		RepoRoot:       root,
		WorktreeID:     wtID,
		WorktreePath:   wtInfo.Path,
		Contract:       contract,
		RunTests:       runTests,
		TestCommands:   testCommands,
		ForceTests:     explicitTestCommands,
		UseModelReview: *modelReview,
		Model:          *model,
		MaxDiffBytes:   cfg.Review.MaxDiffBytes,
	})
	if err != nil {
		eventRun.finish(false, "Review command failed", err)
		fmt.Fprintf(env.Stderr, "patch review failed: %v\n", err)
		return 1
	}

	printReviewReport(env, report, cfg)

	if report.Recommendation == review.RecommendationReject {
		eventRun.finish(false, "Review command rejected", fmt.Errorf("recommendation=%s", report.Recommendation))
		return 1
	}
	if report.Recommendation == review.RecommendationRequestChanges {
		eventRun.finish(false, "Review command requested changes", fmt.Errorf("recommendation=%s", report.Recommendation))
		return 1
	}
	eventRun.finish(true, "Review command succeeded", nil)
	return 0
}

func init() {
	commands.Register(&PatchCommand{})
}
