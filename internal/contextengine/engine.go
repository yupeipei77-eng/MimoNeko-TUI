package contextengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// effectiveRoot returns the repo root from the request if set, otherwise the engine default.
func (e *DefaultContextEngine) effectiveRoot(req BuildRequest) string {
	if strings.TrimSpace(req.RepoRoot) != "" {
		return req.RepoRoot
	}
	return e.repoRoot
}

// Build assembles a context bundle with the correct layer ordering.
func (e *DefaultContextEngine) Build(ctx context.Context, req BuildRequest) (Bundle, error) {
	if err := ctx.Err(); err != nil {
		return Bundle{}, err
	}

	root := e.effectiveRoot(req)

	// 1. Build Immutable Prefix
	systemPrompt, err := e.readSource(root, "system_prompt")
	if err != nil {
		return Bundle{}, fmt.Errorf("read system_prompt: %w", err)
	}
	codingRules, err := e.readSource(root, "coding_rules")
	if err != nil {
		return Bundle{}, fmt.Errorf("read coding_rules: %w", err)
	}
	toolSchemas, err := e.readToolSchemas(root)
	if err != nil {
		return Bundle{}, fmt.Errorf("read tool schemas: %w", err)
	}

	prefixReq := prefix.BuildRequest{
		Version:      e.prefixCfg.Version,
		SystemPrompt: systemPrompt,
		CodingRules:  codingRules,
		ToolSchemas:  toolSchemas,
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

	// 6. Build layers (ordered: Prefix → Conversation → Scratchpad → CurrentInput)
	var layers []ContextLayer
	layers = append(layers, ContextLayer{Name: "immutable_prefix", Bytes: prefixDoc.Bytes, Tokens: prefixDoc.TokenEstimate})

	convBytes := estimateEventBytes(convTail)
	layers = append(layers, ContextLayer{Name: "conversation_log", Bytes: convBytes, Tokens: convTokens})

	scratchBytes := estimateItemBytes(snap.Items)
	layers = append(layers, ContextLayer{Name: "scratchpad", Bytes: scratchBytes, Tokens: scratchTokens})

	layers = append(layers, ContextLayer{Name: "current_input", Bytes: req.CurrentInput, Tokens: currentInputTokens})

	// 7. Build report
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
		CurrentInput:     req.CurrentInput,
		Layers:           layers,
		CacheFingerprint: prefix.Fingerprint{SHA256: prefixDoc.SHA256, Version: prefixDoc.Version},
		Report:           report,
	}, nil
}

// RecordModelCall delegates to the cache registry.
func (e *DefaultContextEngine) RecordModelCall(ctx context.Context, observation cache.Observation) error {
	return e.cacheRegistry.Record(ctx, observation)
}

// safePath joins root and rel, then verifies the result stays within root.
// Returns an error if the resolved path escapes the root directory.
// It also rejects absolute paths in rel (both Unix / and Windows drive letters).
func safePath(root, rel string) (string, error) {
	// Reject absolute paths in rel
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is absolute, must be relative", rel)
	}
	// Reject paths starting with / even on Windows where filepath.IsAbs may not catch it
	if len(rel) > 0 && rel[0] == '/' {
		return "", fmt.Errorf("path %q starts with /, must be relative", rel)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	joined := filepath.Join(absRoot, rel)
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if !strings.HasPrefix(absJoined, absRoot+string(os.PathSeparator)) && absJoined != absRoot {
		return "", fmt.Errorf("path %q escapes root %q", rel, absRoot)
	}
	return absJoined, nil
}

// readSource reads a static file source by name from the prefix config.
// It reads from root + source.Path and enforces path safety.
// If required=true and the file does not exist, an error is returned.
func (e *DefaultContextEngine) readSource(root, name string) ([]byte, error) {
	for _, src := range e.prefixCfg.ImmutableSources {
		if src.Name == name && src.Kind == "static_file" {
			resolved, err := safePath(root, src.Path)
			if err != nil {
				return nil, fmt.Errorf("source %q: %w", name, err)
			}
			data, err := os.ReadFile(resolved)
			if err != nil {
				if os.IsNotExist(err) && !src.Required {
					return nil, nil
				}
				return nil, fmt.Errorf("read source %q from %q: %w", name, resolved, err)
			}
			return data, nil
		}
	}
	return nil, nil
}

// readToolSchemas extracts tool schema entries from the prefix config.
// For generated_schema sources, it reads the schema file from disk.
func (e *DefaultContextEngine) readToolSchemas(root string) ([]prefix.ToolSchema, error) {
	var schemas []prefix.ToolSchema
	for _, src := range e.prefixCfg.ImmutableSources {
		if src.Kind == "generated_schema" {
			resolved, err := safePath(root, src.Path)
			if err != nil {
				return nil, fmt.Errorf("schema %q: %w", src.Name, err)
			}
			data, err := os.ReadFile(resolved)
			if err != nil {
				if os.IsNotExist(err) && !src.Required {
					schemas = append(schemas, prefix.ToolSchema{Name: src.Name, Bytes: nil})
					continue
				}
				return nil, fmt.Errorf("read schema %q from %q: %w", src.Name, resolved, err)
			}
			schemas = append(schemas, prefix.ToolSchema{Name: src.Name, Bytes: data})
		}
	}
	return schemas, nil
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

func estimateEventBytes(events []conversation.Event) []byte {
	var buf []byte
	for _, e := range events {
		buf = append(buf, e.Payload...)
		buf = append(buf, '\n')
	}
	return buf
}

func estimateItemBytes(items []scratchpad.Item) []byte {
	var buf []byte
	for _, i := range items {
		buf = append(buf, i.Content...)
		buf = append(buf, '\n')
	}
	return buf
}
