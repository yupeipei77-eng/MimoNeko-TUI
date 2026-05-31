package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mimoneko/mimoneko/internal/agent"
	"github.com/mimoneko/mimoneko/internal/cache"
	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/contextengine"
	"github.com/mimoneko/mimoneko/internal/conversation"
	"github.com/mimoneko/mimoneko/internal/events"
	"github.com/mimoneko/mimoneko/internal/memory"
	"github.com/mimoneko/mimoneko/internal/modelrouter"
	"github.com/mimoneko/mimoneko/internal/multiagent"
	"github.com/mimoneko/mimoneko/internal/patch"
	"github.com/mimoneko/mimoneko/internal/prefix"
	"github.com/mimoneko/mimoneko/internal/review"
	"github.com/mimoneko/mimoneko/internal/scratchpad"
	"github.com/mimoneko/mimoneko/internal/tools"
	"github.com/mimoneko/mimoneko/internal/validation"
	"github.com/mimoneko/mimoneko/internal/worktree"
)

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
	conversationLog := conversation.NewJSONLConversationLog(filepath.Join(root, config.DirName(), "logs", "conversations"))
	scratch := scratchpad.NewVolatileScratchpad()
	budgetGuard, err := contextengine.NewBudgetGuard(contextengine.BudgetThresholds{
		WarnRatio:  cfg.Prefix.Budget.WarnRatio,
		BlockRatio: cfg.Prefix.Budget.BlockRatio,
	})
	if err != nil {
		return agent.Dependencies{}, nil, fmt.Errorf("budget guard: %w", err)
	}
	contextEngine := contextengine.NewDefaultContextEngine(prefixBuilder, conversationLog, scratch, cacheRegistry, budgetGuard, root, cfg.Prefix)
	contextEngine.SetMemoryStore(memory.NewJSONLStore(filepath.Join(root, config.DirName(), "memory", "records.jsonl")))

	providers := make(map[string]modelrouter.Provider, len(cfg.Models.Providers))
	for _, providerCfg := range cfg.Models.Providers {
		models := make([]string, 0, len(providerCfg.Models))
		for _, modelCfg := range providerCfg.Models {
			models = append(models, modelCfg.Name)
		}
		switch providerCfg.Type {
		case "mimo":
			providers[providerCfg.Name] = modelrouter.NewMimoProvider(
				providerCfg.Name,
				providerCfg.BaseURL,
				providerCfg.APIKeyEnv,
				models,
				nil,
			)
		default:
			providers[providerCfg.Name] = modelrouter.NewOpenAICompatibleProvider(
				providerCfg.Name,
				providerCfg.BaseURL,
				providerCfg.APIKeyEnv,
				models,
				nil,
			)
		}
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

	wtMgr, wtCleanup, err := buildWorktreeManagerFromConfig(root, cfg)
	if err != nil {
		auditLog.Close()
		return agent.Dependencies{}, nil, fmt.Errorf("worktree manager: %w", err)
	}
	patchMgr := patch.NewGitPatchManager(wtMgr, patchConfigFromConfig(cfg))

	checkpointPath := agent.DefaultCheckpointPath(root)
	checkpointStore, err := agent.NewJSONLCheckpointStore(checkpointPath)
	if err != nil {
		wtCleanup()
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
		WorktreeMgr:     wtMgr,
		PatchMgr:        patchMgr,
	}

	cleanup := func() {
		wtCleanup()
		auditLog.Close()
	}

	emitter, eventCleanup, err := buildEventEmitter(root, cfg)
	if err != nil {
		wtCleanup()
		auditLog.Close()
		return agent.Dependencies{}, nil, err
	}
	deps.EventEmitter = emitter
	toolRt.SetEventEmitter(emitter)
	patchMgr.SetEventEmitter(emitter)
	prevCleanup := cleanup
	cleanup = func() {
		eventCleanup()
		prevCleanup()
	}

	return deps, cleanup, nil
}

func buildMultiAgentDependencies(root string, cfg *config.Root, agentDeps agent.Dependencies) (multiagent.Dependencies, func(), error) {
	wtMgr := agentDeps.WorktreeMgr
	wtCleanup := func() {}
	var err error
	if wtMgr == nil {
		wtMgr, wtCleanup, err = buildWorktreeManagerFromConfig(root, cfg)
		if err != nil {
			return multiagent.Dependencies{}, nil, fmt.Errorf("worktree manager: %w", err)
		}
	}

	patchMgr := agentDeps.PatchMgr
	if patchMgr == nil {
		patchMgr = patch.NewGitPatchManager(wtMgr, patchConfigFromConfig(cfg))
	}

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

	var modelReviewerInst review.ModelReviewer
	modelReviewerInst = review.NewDefaultModelReviewer(agentDeps.ModelRouter)

	reviewCfg := reviewConfigFromConfig(cfg)
	reviewMgr := review.NewDefaultPatchReviewManager(patchMgr, valRunner, modelReviewerInst, reviewCfg)

	emitter := agentDeps.EventEmitter
	eventCleanup := func() {}
	if emitter == nil {
		emitter, eventCleanup, err = buildEventEmitter(root, cfg)
		if err != nil {
			auditLog.Close()
			wtCleanup()
			return multiagent.Dependencies{}, nil, err
		}
	}

	if emitter != nil {
		if setter, ok := patchMgr.(interface{ SetEventEmitter(events.EventEmitter) }); ok {
			setter.SetEventEmitter(emitter)
		}
		valRunner.SetEventEmitter(emitter)
		reviewMgr.SetEventEmitter(emitter)
		toolRt.SetEventEmitter(emitter)
	}

	cpPath := multiagent.DefaultMultiAgentCheckpointPath(root)
	cpStore, err := multiagent.NewJSONLMultiAgentCheckpointStore(cpPath)
	if err != nil {
		eventCleanup()
		auditLog.Close()
		wtCleanup()
		return multiagent.Dependencies{}, nil, fmt.Errorf("multi-agent checkpoint store: %w", err)
	}

	singleAgent := agent.NewSingleAgentRuntime(agentDeps)

	deps := multiagent.Dependencies{
		ContextEngine:   agentDeps.ContextEngine,
		ModelRouter:     agentDeps.ModelRouter,
		SingleAgent:     singleAgent,
		ReviewMgr:       reviewMgr,
		WorktreeMgr:     wtMgr,
		CheckpointStore: cpStore,
		EventEmitter:    emitter,
	}

	cleanup := func() {
		eventCleanup()
		auditLog.Close()
		wtCleanup()
	}

	return deps, cleanup, nil
}

func buildEventEmitter(root string, cfg *config.Root) (events.EventEmitter, func(), error) {
	if !cfg.Events.Enabled {
		return &events.NoopEventEmitter{}, func() {}, nil
	}

	eventStorePath := cfg.Events.StorePath
	if !filepath.IsAbs(eventStorePath) {
		eventStorePath = filepath.Join(root, eventStorePath)
	}
	eventStore, err := events.NewJSONLRunEventStore(eventStorePath)
	if err != nil {
		return nil, nil, fmt.Errorf("event store: %w", err)
	}

	bus := events.NewDefaultEventBus(eventStore)
	cleanup := func() { _ = eventStore.Close() }
	return events.NewEventEmitter(bus), cleanup, nil
}

type patchCLIEventRun struct {
	ctx       context.Context
	emitter   events.EventEmitter
	startedAt time.Time
}

func beginPatchCLIEventRun(ctx context.Context, emitter events.EventEmitter, runIDPrefix, taskID, worktreeID, message string) patchCLIEventRun {
	runID := runIDPrefix + "_" + generateShortID()
	if taskID == "" {
		taskID = "task_" + runID
	}
	ctx = events.WithRunContext(ctx, events.RunContext{
		RunID:      runID,
		TaskID:     taskID,
		WorktreeID: worktreeID,
	})

	startedAt := time.Now().UTC()
	events.SafeEmit(emitter, ctx, events.RunEvent{
		ID:        mustGenerateCLIEventID(),
		Type:      events.EventRunStarted,
		Source:    "cli",
		Status:    "started",
		Message:   message,
		StartedAt: startedAt,
		Metadata:  map[string]string{"worktree_id": worktreeID},
	})

	return patchCLIEventRun{
		ctx:       ctx,
		emitter:   emitter,
		startedAt: startedAt,
	}
}

func (r patchCLIEventRun) finish(success bool, message string, runErr error) {
	eventType := events.EventRunSucceeded
	status := "succeeded"
	errMsg := ""
	if !success {
		eventType = events.EventRunFailed
		status = "failed"
		if runErr != nil {
			errMsg = runErr.Error()
		}
	}

	finishedAt := time.Now().UTC()
	events.SafeEmit(r.emitter, r.ctx, events.RunEvent{
		ID:         mustGenerateCLIEventID(),
		Type:       eventType,
		Source:     "cli",
		Status:     status,
		Message:    message,
		StartedAt:  r.startedAt,
		FinishedAt: finishedAt,
		DurationMs: finishedAt.Sub(r.startedAt).Milliseconds(),
		Error:      errMsg,
	})
}

func buildWorktreeManager(root string) (worktree.WorktreeManager, func(), error) {
	return buildWorktreeManagerFromConfig(root, nil)
}

func buildWorktreeManagerFromConfig(root string, cfg *config.Root) (worktree.WorktreeManager, func(), error) {
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		return nil, nil, fmt.Errorf("worktree registry: %w", err)
	}

	wtCfg := worktree.DefaultGitWorktreeManagerConfig()
	if cfg != nil {
		wtCfg.BranchPrefix = cfg.Worktree.BranchPrefix
		wtCfg.MaxActive = cfg.Worktree.MaxActive
		wtCfg.KeepFailed = cfg.Worktree.KeepFailed
		wtCfg.KeepCancelled = cfg.Worktree.KeepCancelled
	}
	cleanup := func() { _ = registry.Close() }
	return worktree.NewGitWorktreeManager(registry, wtCfg), cleanup, nil
}

func patchConfigFromConfig(cfg *config.Root) patch.GitPatchManagerConfig {
	return patch.GitPatchManagerConfig{
		MaxDiffBytes:     cfg.Patch.MaxDiffBytes,
		RequireCleanMain: cfg.Patch.RequireCleanMain,
		AllowBinary:      cfg.Patch.AllowBinary,
	}
}

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

func validationConfigFromConfig(cfg *config.Root) validation.ValidationConfig {
	return validation.ValidationConfig{
		DefaultTestCommands: cfg.Validation.DefaultTestCommands,
		MaxOutputBytes:      cfg.Validation.MaxOutputBytes,
		TimeoutSeconds:      cfg.Validation.TimeoutSeconds,
	}
}

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
