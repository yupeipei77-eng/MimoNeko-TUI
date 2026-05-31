package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestJSONLStoreSearchRanksScopedMatches(t *testing.T) {
	store := NewJSONLStore(filepath.Join(t.TempDir(), "memory.jsonl"))
	now := time.Now().UTC()
	if err := store.Put(context.Background(), Record{
		ID:        "m1",
		Scope:     "repo",
		Text:      "raw mode command palette supports arrow key navigation",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Put() error: %v", err)
	}
	if err := store.Put(context.Background(), Record{
		ID:        "m2",
		Scope:     "other",
		Text:      "unrelated billing note",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	results, err := store.Search(context.Background(), SearchQuery{Scope: "repo", Text: "palette arrow", Limit: 3})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 || results[0].Record.ID != "m1" {
		t.Fatalf("results = %+v, want scoped palette memory", results)
	}
	if results[0].Score <= 0 {
		t.Fatalf("score = %f, want positive", results[0].Score)
	}
}

func TestJSONLStoreGetReturnsLatestRecordVersion(t *testing.T) {
	store := NewJSONLStore(filepath.Join(t.TempDir(), "memory.jsonl"))
	oldTime := time.Now().Add(-time.Hour).UTC()
	newTime := time.Now().UTC()
	_ = store.Put(context.Background(), Record{ID: "m1", Text: "old", CreatedAt: oldTime, UpdatedAt: oldTime})
	_ = store.Put(context.Background(), Record{ID: "m1", Text: "new", CreatedAt: oldTime, UpdatedAt: newTime})

	record, ok, err := store.Get(context.Background(), "m1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if !ok || record.Text != "new" {
		t.Fatalf("record=%+v ok=%v, want latest version", record, ok)
	}
}
