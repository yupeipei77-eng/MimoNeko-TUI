package contextengine

import (
	"context"

	"github.com/mimoneko/mimoneko/internal/cache"
	"github.com/mimoneko/mimoneko/internal/conversation"
	"github.com/mimoneko/mimoneko/internal/memory"
	"github.com/mimoneko/mimoneko/internal/prefix"
	"github.com/mimoneko/mimoneko/internal/scratchpad"
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
	MemoryScope    string
	MemoryQuery    string
	MemoryLimit    int
}

type VolatileContext struct {
	ConversationTail []conversation.Event
	Scratchpad       scratchpad.Snapshot
	MemoryResults    []memory.SearchResult
}

type ContextReport struct {
	PrefixTokens       int
	ConversationTokens int
	MemoryTokens       int
	ScratchpadTokens   int
	CurrentInputTokens int
	TotalTokens        int
	BudgetStatus       BudgetStatus
}

// ContextLayer represents a single layer in the assembled context.
// Layers are ordered: ImmutablePrefix → ConversationLog → Scratchpad → CurrentInput.
type ContextLayer struct {
	Name   string
	Bytes  []byte
	Tokens int
}

type Bundle struct {
	ImmutablePrefix  prefix.Document
	Volatile         VolatileContext
	CurrentInput     []byte
	Layers           []ContextLayer
	CacheFingerprint prefix.Fingerprint
	Report           ContextReport
}

type ContextEngine interface {
	Build(ctx context.Context, req BuildRequest) (Bundle, error)
	RecordModelCall(ctx context.Context, observation cache.Observation) error
}
