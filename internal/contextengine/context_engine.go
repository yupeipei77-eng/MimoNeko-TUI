package contextengine

import (
	"context"

	"github.com/reasonforge/reasonforge/internal/cache"
	"github.com/reasonforge/reasonforge/internal/conversation"
	"github.com/reasonforge/reasonforge/internal/prefix"
	"github.com/reasonforge/reasonforge/internal/scratchpad"
)

type TokenBudget struct {
	ImmutablePrefix int
	Conversation    int
	Scratchpad      int
	Output          int
}

type BuildRequest struct {
	TaskID         string
	ConversationID string
	RepoRoot       string
	Budget         TokenBudget
	CurrentInput   []byte
}

type VolatileContext struct {
	ConversationTail []conversation.Event
	Scratchpad       scratchpad.Snapshot
}

type ContextReport struct {
	PrefixTokens       int
	ConversationTokens int
	ScratchpadTokens   int
	CurrentInputTokens int
	TotalTokens        int
	BudgetStatus       BudgetStatus
}

type Bundle struct {
	ImmutablePrefix  prefix.Document
	Volatile         VolatileContext
	CacheFingerprint prefix.Fingerprint
	Report           ContextReport
}

type ContextEngine interface {
	Build(ctx context.Context, req BuildRequest) (Bundle, error)
	RecordModelCall(ctx context.Context, observation cache.Observation) error
}
