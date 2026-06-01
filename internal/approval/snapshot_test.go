package approval

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotStoreLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshots.json")

	store := NewSnapshotStore(path)
	err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if store.Count() != 0 {
		t.Errorf("Count() = %d, want 0", store.Count())
	}
}

func TestSnapshotStoreSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshots.json")

	store := NewSnapshotStore(path)
	snap := &ResumeSnapshot{
		ApprovalID:       "apr_test",
		RunID:            "run-123",
		ToolName:         "file_write",
		ToolArgs:         map[string]string{"path": ".env"},
		RiskLevel:        "high",
		Reason:           "high-risk tool requires approval",
		Path:             ".env",
		CreatedAt:        time.Now().UTC(),
		SanitizedPreview: "Write to .env (sensitive path)",
	}

	if err := store.Add(snap); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Reload
	store2 := NewSnapshotStore(path)
	if err := store2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if store2.Count() != 1 {
		t.Errorf("Count() = %d, want 1", store2.Count())
	}

	loaded, err := store2.Get("apr_test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if loaded.ToolName != "file_write" {
		t.Errorf("ToolName = %q, want %q", loaded.ToolName, "file_write")
	}
	if loaded.SanitizedPreview != "Write to .env (sensitive path)" {
		t.Errorf("SanitizedPreview = %q, want %q", loaded.SanitizedPreview, "Write to .env (sensitive path)")
	}
}

func TestSnapshotStoreDeterministicOrdering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshots.json")

	store := NewSnapshotStore(path)
	now := time.Now().UTC()

	store.Add(&ResumeSnapshot{
		ApprovalID: "apr_c",
		RunID:      "run-123",
		ToolName:   "file_write",
		CreatedAt:  now.Add(2 * time.Hour),
	})
	store.Add(&ResumeSnapshot{
		ApprovalID: "apr_a",
		RunID:      "run-123",
		ToolName:   "file_write",
		CreatedAt:  now,
	})
	store.Add(&ResumeSnapshot{
		ApprovalID: "apr_b",
		RunID:      "run-123",
		ToolName:   "file_write",
		CreatedAt:  now.Add(1 * time.Hour),
	})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	content := string(data)
	aIdx := indexOf(content, "apr_a")
	bIdx := indexOf(content, "apr_b")
	cIdx := indexOf(content, "apr_c")

	if aIdx >= bIdx || bIdx >= cIdx {
		t.Errorf("Snapshots not ordered correctly: a=%d, b=%d, c=%d", aIdx, bIdx, cIdx)
	}
}

func TestSnapshotStoreDuplicateAdd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshots.json")

	store := NewSnapshotStore(path)
	snap := &ResumeSnapshot{
		ApprovalID: "apr_test",
		RunID:      "run-123",
		ToolName:   "file_write",
		CreatedAt:  time.Now().UTC(),
	}

	store.Add(snap)
	err := store.Add(snap)
	if err == nil {
		t.Error("Add() should return error for duplicate approval_id")
	}
}

func TestSnapshotStoreUpsert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshots.json")

	store := NewSnapshotStore(path)
	snap := &ResumeSnapshot{
		ApprovalID: "apr_test",
		RunID:      "run-123",
		ToolName:   "file_write",
		CreatedAt:  time.Now().UTC(),
	}

	store.Add(snap)

	snap.ToolName = "file_read"
	err := store.Upsert(snap)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	loaded, _ := store.Get("apr_test")
	if loaded.ToolName != "file_read" {
		t.Errorf("ToolName = %q, want %q", loaded.ToolName, "file_read")
	}
}

func TestSnapshotStoreInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshots.json")

	os.WriteFile(path, []byte("invalid json"), 0600)

	store := NewSnapshotStore(path)
	err := store.Load()
	if err == nil {
		t.Error("Load() should return error for invalid JSON")
	}
}

func TestResumeSnapshotValidate(t *testing.T) {
	tests := []struct {
		name    string
		snap    ResumeSnapshot
		wantErr bool
	}{
		{
			name: "valid",
			snap: ResumeSnapshot{
				ApprovalID: "apr_test",
				RunID:      "run-123",
				ToolName:   "file_write",
				CreatedAt:  time.Now().UTC(),
			},
			wantErr: false,
		},
		{
			name: "missing approval_id",
			snap: ResumeSnapshot{
				RunID:     "run-123",
				ToolName:  "file_write",
				CreatedAt: time.Now().UTC(),
			},
			wantErr: true,
		},
		{
			name: "missing run_id",
			snap: ResumeSnapshot{
				ApprovalID: "apr_test",
				ToolName:   "file_write",
				CreatedAt:  time.Now().UTC(),
			},
			wantErr: true,
		},
		{
			name: "missing tool_name",
			snap: ResumeSnapshot{
				ApprovalID: "apr_test",
				RunID:      "run-123",
				CreatedAt:  time.Now().UTC(),
			},
			wantErr: true,
		},
		{
			name: "missing created_at",
			snap: ResumeSnapshot{
				ApprovalID: "apr_test",
				RunID:      "run-123",
				ToolName:   "file_write",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.snap.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSnapshotStoreGetNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshots.json")

	store := NewSnapshotStore(path)
	store.Load()

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("Get() should return error for nonexistent snapshot")
	}
}
