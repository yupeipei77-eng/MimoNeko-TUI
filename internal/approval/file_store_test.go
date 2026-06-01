package approval

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStoreLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	store := NewFileStore(path)
	err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if store.Count() != 0 {
		t.Errorf("Count() = %d, want 0", store.Count())
	}
}

func TestFileStoreSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	store := NewFileStore(path)
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test approval",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	if err := store.Add(req); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Create new store and load
	store2 := NewFileStore(path)
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

	if loaded.ID != "apr_test" {
		t.Errorf("ID = %q, want %q", loaded.ID, "apr_test")
	}
	if loaded.ToolName != "file_write" {
		t.Errorf("ToolName = %q, want %q", loaded.ToolName, "file_write")
	}
	if loaded.Status != StatusPending {
		t.Errorf("Status = %q, want %q", loaded.Status, StatusPending)
	}
}

func TestFileStoreDeterministicOrdering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	store := NewFileStore(path)

	// Add requests in reverse order
	now := time.Now().UTC()
	store.Add(&ApprovalRequest{
		ID:        "apr_c",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: now.Add(2 * time.Hour),
	})
	store.Add(&ApprovalRequest{
		ID:        "apr_a",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: now,
	})
	store.Add(&ApprovalRequest{
		ID:        "apr_b",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: now.Add(1 * time.Hour),
	})

	// Read file directly
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Verify ordering (by created_at)
	// The JSON should contain IDs in order: apr_a, apr_b, apr_c
	content := string(data)
	aIdx := indexOf(content, "apr_a")
	bIdx := indexOf(content, "apr_b")
	cIdx := indexOf(content, "apr_c")

	if aIdx >= bIdx || bIdx >= cIdx {
		t.Errorf("Requests not ordered correctly: a=%d, b=%d, c=%d", aIdx, bIdx, cIdx)
	}
}

func TestFileStoreApprovePersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	store := NewFileStore(path)
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	store.Add(req)
	req.Approve("user-1")
	store.Update(req)

	// Reload and verify
	store2 := NewFileStore(path)
	store2.Load()

	loaded, _ := store2.Get("apr_test")
	if loaded.Status != StatusApproved {
		t.Errorf("Status = %q, want %q", loaded.Status, StatusApproved)
	}
	if loaded.DecidedBy != "user-1" {
		t.Errorf("DecidedBy = %q, want %q", loaded.DecidedBy, "user-1")
	}
}

func TestFileStoreRejectPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	store := NewFileStore(path)
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	store.Add(req)
	req.Reject("user-1")
	store.Update(req)

	// Reload and verify
	store2 := NewFileStore(path)
	store2.Load()

	loaded, _ := store2.Get("apr_test")
	if loaded.Status != StatusRejected {
		t.Errorf("Status = %q, want %q", loaded.Status, StatusRejected)
	}
}

func TestFileStoreUpdateNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	store := NewFileStore(path)
	store.Load()

	req := &ApprovalRequest{
		ID:        "apr_nonexistent",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	err := store.Update(req)
	if err == nil {
		t.Error("Update() should return error for nonexistent request")
	}
}

func TestFileStoreDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	store := NewFileStore(path)
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	store.Add(req)
	store.Delete("apr_test")

	if store.Count() != 0 {
		t.Errorf("Count() = %d, want 0", store.Count())
	}

	// Reload and verify
	store2 := NewFileStore(path)
	store2.Load()

	if store2.Count() != 0 {
		t.Errorf("Count() after reload = %d, want 0", store2.Count())
	}
}

func TestFileStoreDeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	store := NewFileStore(path)
	store.Load()

	err := store.Delete("apr_nonexistent")
	if err == nil {
		t.Error("Delete() should return error for nonexistent request")
	}
}

func TestFileStoreInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	// Write invalid JSON
	os.WriteFile(path, []byte("invalid json"), 0600)

	store := NewFileStore(path)
	err := store.Load()
	if err == nil {
		t.Error("Load() should return error for invalid JSON")
	}
}

func TestFileStoreEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	// Write empty file
	os.WriteFile(path, []byte(""), 0600)

	store := NewFileStore(path)
	err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if store.Count() != 0 {
		t.Errorf("Count() = %d, want 0", store.Count())
	}
}

func TestFileStoreDuplicateAdd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	store := NewFileStore(path)
	req := &ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	}

	store.Add(req)
	err := store.Add(req)
	if err == nil {
		t.Error("Add() should return error for duplicate ID")
	}
}

func TestFileStorePending(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.json")

	store := NewFileStore(path)
	now := time.Now().UTC()

	store.Add(&ApprovalRequest{
		ID:        "apr_pending",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: now,
	})
	store.Add(&ApprovalRequest{
		ID:        "apr_approved",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusApproved,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: now.Add(1 * time.Hour),
	})

	pending := store.Pending()
	if len(pending) != 1 {
		t.Errorf("Pending() returned %d items, want 1", len(pending))
	}
	if pending[0].ID != "apr_pending" {
		t.Errorf("Pending() returned ID %q, want %q", pending[0].ID, "apr_pending")
	}
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
