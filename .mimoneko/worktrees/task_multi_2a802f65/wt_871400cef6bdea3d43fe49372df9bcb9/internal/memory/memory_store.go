package memory

import (
	"context"
	"time"
)

type Record struct {
	ID        string
	Scope     string
	Text      string
	Metadata  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SearchQuery struct {
	Scope string
	Text  string
	Limit int
}

type SearchResult struct {
	Record Record
	Score  float64
}

type MemoryStore interface {
	Put(ctx context.Context, record Record) error
	Get(ctx context.Context, id string) (Record, bool, error)
	Search(ctx context.Context, query SearchQuery) ([]SearchResult, error)
}
