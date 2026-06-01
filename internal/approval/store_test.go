package approval

import (
	"testing"
	"time"
)

func TestMemoryStoreListEmpty(t *testing.T) {
	store := NewMemoryStore()
	list := store.List()
	if len(list) != 0 {
		t.Errorf("List() returned %d items, want 0", len(list))
	}
}

func TestMemoryStoreAdd(t *testing.T) {
	store := NewMemoryStore()
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

	err := store.Add(req)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	list := store.List()
	if len(list) != 1 {
		t.Errorf("List() returned %d items, want 1", len(list))
	}
}

func TestMemoryStoreGet(t *testing.T) {
	store := NewMemoryStore()
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

	got, err := store.Get("apr_test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != "apr_test" {
		t.Errorf("Get() returned ID %q, want %q", got.ID, "apr_test")
	}
}

func TestMemoryStoreGetNotFound(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("Get() should return error for nonexistent request")
	}
}

func TestMemoryStoreUpdate(t *testing.T) {
	store := NewMemoryStore()
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

	req.Status = StatusApproved
	err := store.Update(req)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, _ := store.Get("apr_test")
	if got.Status != StatusApproved {
		t.Errorf("Status = %q, want %q", got.Status, StatusApproved)
	}
}

func TestMemoryStorePending(t *testing.T) {
	store := NewMemoryStore()

	// Add pending request
	store.Add(&ApprovalRequest{
		ID:        "apr_pending",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	})

	// Add approved request
	store.Add(&ApprovalRequest{
		ID:        "apr_approved",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusApproved,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	})

	pending := store.Pending()
	if len(pending) != 1 {
		t.Errorf("Pending() returned %d items, want 1", len(pending))
	}
	if pending[0].ID != "apr_pending" {
		t.Errorf("Pending() returned ID %q, want %q", pending[0].ID, "apr_pending")
	}
}

func TestMemoryStoreCount(t *testing.T) {
	store := NewMemoryStore()
	if store.Count() != 0 {
		t.Errorf("Count() = %d, want 0", store.Count())
	}

	store.Add(&ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	})

	if store.Count() != 1 {
		t.Errorf("Count() = %d, want 1", store.Count())
	}
}

func TestMemoryStoreClear(t *testing.T) {
	store := NewMemoryStore()
	store.Add(&ApprovalRequest{
		ID:        "apr_test",
		RunID:     "run-123",
		ToolName:  "file_write",
		Scope:     ScopeTool,
		Status:    StatusPending,
		Reason:    "test",
		RiskLevel: "high",
		CreatedAt: time.Now().UTC(),
	})

	store.Clear()
	if store.Count() != 0 {
		t.Errorf("Count() after Clear() = %d, want 0", store.Count())
	}
}

func TestMemoryStoreDuplicateAdd(t *testing.T) {
	store := NewMemoryStore()
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

func TestMemoryStoreUpdateNotFound(t *testing.T) {
	store := NewMemoryStore()
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

	err := store.Update(req)
	if err == nil {
		t.Error("Update() should return error for nonexistent request")
	}
}
