package scratchpad

import (
	"context"
	"time"
)

type ItemKind string

const (
	ItemKindRAGResult   ItemKind = "rag_result"
	ItemKindToolOutput  ItemKind = "tool_output"
	ItemKindReasoning   ItemKind = "reasoning"
	ItemKindRepoContext ItemKind = "repo_context"
)

type Item struct {
	ID        string
	TaskID    string
	Kind      ItemKind
	Content   []byte
	Metadata  map[string]string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Scope struct {
	TaskID string
	Kinds  []ItemKind
	Limit  int
}

type Snapshot struct {
	TaskID string
	Items  []Item
}

type Scratchpad interface {
	Put(ctx context.Context, item Item) error
	Snapshot(ctx context.Context, scope Scope) (Snapshot, error)
	Clear(ctx context.Context, scope Scope) error
}
