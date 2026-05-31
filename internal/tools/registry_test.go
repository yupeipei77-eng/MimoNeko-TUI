package tools

import (
	"context"
	"testing"
)

func TestRegistryRegisterAndLookup(t *testing.T) {
	r := NewMemoryRegistry()
	tool := &FileReadTool{}

	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, ok := r.Get("file_read")
	if !ok {
		t.Fatal("Get(file_read) not found")
	}
	if got.Name() != "file_read" {
		t.Fatalf("Get() name = %q, want file_read", got.Name())
	}
}

func TestRegistryDuplicateRegistration(t *testing.T) {
	r := NewMemoryRegistry()
	tool := &FileReadTool{}

	if err := r.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := r.Register(tool)
	if err == nil {
		t.Fatal("duplicate Register() should fail")
	}
}

func TestRegistryListSorted(t *testing.T) {
	r := NewMemoryRegistry()
	_ = r.Register(&FileReadTool{})
	_ = r.Register(&GitDiffTool{})
	_ = r.Register(&FileWriteTool{})

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("List() count = %d, want 3", len(list))
	}

	// Should be sorted by name
	names := make([]string, len(list))
	for i, info := range list {
		names[i] = info.Name
	}
	if names[0] != "file_read" || names[1] != "file_write" || names[2] != "git_diff" {
		t.Fatalf("List() order = %v, want sorted", names)
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewMemoryRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("Get(nonexistent) should return false")
	}
}

func TestRegistryRegisterNil(t *testing.T) {
	r := NewMemoryRegistry()
	err := r.Register(nil)
	if err == nil {
		t.Fatal("Register(nil) should fail")
	}
}

func TestRegistryRegisterEmptyName(t *testing.T) {
	r := NewMemoryRegistry()
	// Create a tool with empty name using a custom implementation
	err := r.Register(&emptyNameTool{})
	if err == nil {
		t.Fatal("Register(empty name) should fail")
	}
}

type emptyNameTool struct{}

func (t *emptyNameTool) Name() string        { return "" }
func (t *emptyNameTool) Description() string  { return "" }
func (t *emptyNameTool) RiskLevel() string    { return "low" }
func (t *emptyNameTool) Concurrency() ConcurrencyClass { return ConcurrencyReadOnly }
func (t *emptyNameTool) Run(_ context.Context, _ ToolRequest) (ToolResponse, error) {
	return ToolResponse{}, nil
}
