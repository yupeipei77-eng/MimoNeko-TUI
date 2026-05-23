package contextengine

import (
	"context"
	"fmt"

	"github.com/reasonforge/reasonforge/internal/cache"
	"github.com/reasonforge/reasonforge/internal/config"
	"github.com/reasonforge/reasonforge/internal/conversation"
	"github.com/reasonforge/reasonforge/internal/prefix"
	"github.com/reasonforge/reasonforge/internal/scratchpad"
)

// DefaultContextEngine assembles context bundles in the order:
// Immutable Prefix → Conversation Log → Scratchpad → Current User Input.
type DefaultContextEngine struct {
	prefixBuilder   prefix.PrefixBuilder
	conversationLog conversation.ConversationLog
	scratchpad      scratchpad.Scratchpad
	cacheRegistry   cache.CacheRegistry
	budgetGuard     *BudgetGuard
	repoRoot        string
	prefixCfg       config.PrefixConfig
}

// NewDefaultContextEngine creates a new engine with all dependencies.
func NewDefaultContextEngine(
	pb prefix.PrefixBuilder,
	cl conversation.ConversationLog,
	sp scratchpad.Scratchpad,
	cr cache.CacheRegistry,
	bg *BudgetGuard,
	repoRoot string,
	prefixCfg config.PrefixConfig,
) *DefaultContextEngine {
	return &DefaultContextEngine{
		prefixBuilder:   pb,
		conversationLog: cl,
		scratchpad:      sp,
		cacheRegistry:   cr,
		budgetGuard:     bg,
		repoRoot:        repoRoot,
		prefixCfg:       prefixCfg,
	}
}

// Build assembles a context bundle with the correct layer ordering.
func (e *DefaultContextEngine) Build(ctx context.Context, req BuildRequest) (Bundle, error) {
	if err := ctx.Err(); err != nil {
		return Bundle{}, err
	}

	// 1. Build Immutable Prefix
	prefixReq := prefix.BuildRequest{
		Version:      e.prefixCfg.Version,
		SystemPrompt: e.readSource("system_prompt"),
		CodingRules:  e.readSource("coding_rules"),
		ToolSchemas:  e.readToolSchemas(),
		Sources:      e.buildSources(),
	}
	prefixDoc, err := e.prefixBuilder.Build(ctx, prefixReq)
	if err != nil {
		return Bundle{}, fmt.Errorf("build immutable prefix: %w", err)
	}

	// 2. Get Conversation Tail
	var convTail []conversation.Event
	if req.ConversationID != "" {
		events, err := e.conversationLog.Tail(ctx, conversation.Query{
			ConversationID: req.ConversationID,
			Limit:          50,
		})
		if err != nil {
			return Bundle{}, fmt.Errorf("read conversation tail: %w", err)
		}
		convTail = events
	}

	// 3. Get Scratchpad Snapshot
	var snap scratchpad.Snapshot
	if req.TaskID != "" {
		snap, err = e.scratchpad.Snapshot(ctx, scratchpad.Scope{
			TaskID:     req.TaskID,
			TokenBudget: req.Budget.Scratchpad,
		})
		if err != nil {
			return Bundle{}, fmt.Errorf("snapshot scratchpad: %w", err)
		}
	}

	// 4. Compute token estimates
	convTokens := estimateEventTokens(convTail)
	scratchTokens := estimateItemTokens(snap.Items)
	currentInputTokens := prefix.EstimateTokens(req.CurrentInput)
	totalTokens := prefixDoc.TokenEstimate + convTokens + scratchTokens + currentInputTokens
	totalBudget := req.Budget.ImmutablePrefix + req.Budget.Conversation + req.Budget.Scratchpad

	// 5. Check budget
	var budgetStatus BudgetStatus
	if e.budgetGuard != nil && totalBudget > 0 {
		budgetStatus = e.budgetGuard.Check(totalTokens, totalBudget)
	}

	// 6. Build report
	report := ContextReport{
		PrefixTokens:       prefixDoc.TokenEstimate,
		ConversationTokens: convTokens,
		ScratchpadTokens:   scratchTokens,
		CurrentInputTokens: currentInputTokens,
		TotalTokens:        totalTokens,
		BudgetStatus:       budgetStatus,
	}

	return Bundle{
		ImmutablePrefix:  prefixDoc,
		Volatile: VolatileContext{
			ConversationTail: convTail,
			Scratchpad:       snap,
		},
		CacheFingerprint: prefix.Fingerprint{SHA256: prefixDoc.SHA256, Version: prefixDoc.Version},
		Report:           report,
	}, nil
}

// RecordModelCall delegates to the cache registry.
func (e *DefaultContextEngine) RecordModelCall(ctx context.Context, observation cache.Observation) error {
	return e.cacheRegistry.Record(ctx, observation)
}

// readSource reads a static file source by name from the prefix config.
func (e *DefaultContextEngine) readSource(name string) []byte {
	for _, src := range e.prefixCfg.ImmutableSources {
		if src.Name == name && src.Kind == "static_file" {
			// For Phase 1, return a placeholder. Real file reading
			// requires the repo root to be properly configured.
			return nil
		}
	}
	return nil
}

// readToolSchemas extracts tool schema entries from the prefix config.
func (e *DefaultContextEngine) readToolSchemas() []prefix.ToolSchema {
	var schemas []prefix.ToolSchema
	for _, src := range e.prefixCfg.ImmutableSources {
		if src.Kind == "generated_schema" {
			schemas = append(schemas, prefix.ToolSchema{
				Name:  src.Name,
				Bytes: nil, // populated by caller in production
			})
		}
	}
	return schemas
}

// buildSources converts config sources to prefix Sources.
func (e *DefaultContextEngine) buildSources() []prefix.Source {
	sources := make([]prefix.Source, 0, len(e.prefixCfg.ImmutableSources))
	for _, src := range e.prefixCfg.ImmutableSources {
		sources = append(sources, prefix.Source{
			Name:     src.Name,
			Kind:     prefix.SourceKind(src.Kind),
			Path:     src.Path,
			Required: src.Required,
		})
	}
	return sources
}

func estimateEventTokens(events []conversation.Event) int {
	total := 0
	for _, e := range events {
		total += prefix.EstimateTokens(e.Payload)
	}
	return total
}

func estimateItemTokens(items []scratchpad.Item) int {
	total := 0
	for _, i := range items {
		total += prefix.EstimateTokens(i.Content)
	}
	return total
}
