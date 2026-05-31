package repoindex

import (
	"context"
	"time"
)

type IndexRequest struct {
	RepoRoot string
	Ref      string
	Paths    []string
}

type Snapshot struct {
	RepoRoot     string
	Ref          string
	IndexedFiles int
	IndexID      string
	CreatedAt    time.Time
}

type Query struct {
	RepoRoot string
	Text     string
	Paths    []string
	Limit    int
}

type Match struct {
	Path      string
	StartLine int
	EndLine   int
	Score     float64
	Snippet   string
}

type RepoIndexer interface {
	Index(ctx context.Context, req IndexRequest) (Snapshot, error)
	Query(ctx context.Context, query Query) ([]Match, error)
}
