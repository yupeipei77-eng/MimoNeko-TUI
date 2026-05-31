package repoindex

import (
	"testing"
	"time"
)

func TestIndexRequest(t *testing.T) {
	req := IndexRequest{
		RepoRoot: "/path/to/repo",
		Ref:      "main",
		Paths:    []string{"src/", "cmd/"},
	}

	if req.RepoRoot != "/path/to/repo" {
		t.Errorf("IndexRequest.RepoRoot = %q, want %q", req.RepoRoot, "/path/to/repo")
	}
	if req.Ref != "main" {
		t.Errorf("IndexRequest.Ref = %q, want %q", req.Ref, "main")
	}
	if len(req.Paths) != 2 {
		t.Errorf("len(IndexRequest.Paths) = %d, want 2", len(req.Paths))
	}
}

func TestSnapshot(t *testing.T) {
	now := time.Now()
	snap := Snapshot{
		RepoRoot:     "/path/to/repo",
		Ref:          "main",
		IndexedFiles: 42,
		IndexID:      "idx-123",
		CreatedAt:    now,
	}

	if snap.RepoRoot != "/path/to/repo" {
		t.Errorf("Snapshot.RepoRoot = %q, want %q", snap.RepoRoot, "/path/to/repo")
	}
	if snap.Ref != "main" {
		t.Errorf("Snapshot.Ref = %q, want %q", snap.Ref, "main")
	}
	if snap.IndexedFiles != 42 {
		t.Errorf("Snapshot.IndexedFiles = %d, want %d", snap.IndexedFiles, 42)
	}
	if snap.IndexID != "idx-123" {
		t.Errorf("Snapshot.IndexID = %q, want %q", snap.IndexID, "idx-123")
	}
	if !snap.CreatedAt.Equal(now) {
		t.Errorf("Snapshot.CreatedAt = %v, want %v", snap.CreatedAt, now)
	}
}

func TestQuery(t *testing.T) {
	q := Query{
		RepoRoot: "/path/to/repo",
		Text:     "function main",
		Paths:    []string{"cmd/"},
		Limit:    10,
	}

	if q.RepoRoot != "/path/to/repo" {
		t.Errorf("Query.RepoRoot = %q, want %q", q.RepoRoot, "/path/to/repo")
	}
	if q.Text != "function main" {
		t.Errorf("Query.Text = %q, want %q", q.Text, "function main")
	}
	if q.Limit != 10 {
		t.Errorf("Query.Limit = %d, want %d", q.Limit, 10)
	}
}

func TestMatch(t *testing.T) {
	m := Match{
		Path:      "cmd/main.go",
		StartLine: 10,
		EndLine:   20,
		Score:     0.95,
		Snippet:   "func main() {",
	}

	if m.Path != "cmd/main.go" {
		t.Errorf("Match.Path = %q, want %q", m.Path, "cmd/main.go")
	}
	if m.StartLine != 10 {
		t.Errorf("Match.StartLine = %d, want %d", m.StartLine, 10)
	}
	if m.EndLine != 20 {
		t.Errorf("Match.EndLine = %d, want %d", m.EndLine, 20)
	}
	if m.Score != 0.95 {
		t.Errorf("Match.Score = %f, want %f", m.Score, 0.95)
	}
	if m.Snippet != "func main() {" {
		t.Errorf("Match.Snippet = %q, want %q", m.Snippet, "func main() {")
	}
}

func TestQueryDefaultLimit(t *testing.T) {
	q := Query{
		Text: "test",
	}

	// Limit should default to 0 (unset) - caller should handle defaults
	if q.Limit != 0 {
		t.Errorf("Query.Limit = %d, want 0 (default)", q.Limit)
	}
}
