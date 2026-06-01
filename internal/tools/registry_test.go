package tools

import (
	"context"
	"testing"
	"time"
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

func TestRegistryMetadataRegistration(t *testing.T) {
	r := NewMemoryRegistry()
	metadata := ToolMetadata{
		Name:             "custom_read",
		Description:      "Read custom data",
		RiskLevel:        RiskLevelLow,
		Timeout:          5 * time.Second,
		RequiresApproval: false,
		AllowedPaths:     []string{"b", "a"},
	}

	if err := RegisterToolMetadata(r, metadata); err != nil {
		t.Fatalf("RegisterToolMetadata() error = %v", err)
	}

	got, ok := LookupToolMetadata(r, "custom_read")
	if !ok {
		t.Fatal("LookupToolMetadata(custom_read) not found")
	}
	if got.Name != metadata.Name || got.RiskLevel != RiskLevelLow || got.Timeout != 5*time.Second || got.RequiresApproval {
		t.Fatalf("metadata = %+v, want registered metadata", got)
	}
	if len(got.AllowedPaths) != 2 || got.AllowedPaths[0] != "a" || got.AllowedPaths[1] != "b" {
		t.Fatalf("AllowedPaths = %v, want sorted copy", got.AllowedPaths)
	}
}

func TestRegistryMetadataDuplicateRegistration(t *testing.T) {
	r := NewMemoryRegistry()
	metadata := ToolMetadata{Name: "custom_read", RiskLevel: RiskLevelLow}

	if err := RegisterToolMetadata(r, metadata); err != nil {
		t.Fatalf("RegisterToolMetadata() error = %v", err)
	}
	if err := RegisterToolMetadata(r, metadata); err == nil {
		t.Fatal("duplicate RegisterToolMetadata() should fail")
	}
}

func TestRegistryMetadataLookupFromRegisteredTool(t *testing.T) {
	r := NewMemoryRegistry()
	if err := r.Register(&FileWriteTool{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	metadata, ok := LookupToolMetadata(r, "file_write")
	if !ok {
		t.Fatal("LookupToolMetadata(file_write) not found")
	}
	if metadata.Name != "file_write" || metadata.RiskLevel != RiskLevelMedium || !metadata.RequiresApproval {
		t.Fatalf("metadata = %+v, want file_write medium approval metadata", metadata)
	}
}

func TestRegistryListMetadataSorted(t *testing.T) {
	r := NewMemoryRegistry()
	for _, metadata := range []ToolMetadata{
		{Name: "z_tool", RiskLevel: RiskLevelHigh},
		{Name: "a_tool", RiskLevel: RiskLevelLow},
	} {
		if err := RegisterToolMetadata(r, metadata); err != nil {
			t.Fatalf("RegisterToolMetadata(%s) error = %v", metadata.Name, err)
		}
	}

	list := ListToolMetadata(r)
	if len(list) != 2 {
		t.Fatalf("ListToolMetadata() count = %d, want 2", len(list))
	}
	if list[0].Name != "a_tool" || list[1].Name != "z_tool" {
		t.Fatalf("ListToolMetadata() order = %v, want sorted", []string{list[0].Name, list[1].Name})
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

func (t *emptyNameTool) Name() string                  { return "" }
func (t *emptyNameTool) Description() string           { return "" }
func (t *emptyNameTool) RiskLevel() string             { return "low" }
func (t *emptyNameTool) Concurrency() ConcurrencyClass { return ConcurrencyReadOnly }
func (t *emptyNameTool) Run(_ context.Context, _ ToolRequest) (ToolResponse, error) {
	return ToolResponse{}, nil
}
