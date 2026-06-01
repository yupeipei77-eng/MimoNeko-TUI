package tools

import (
	"cmp"
	"fmt"
	"slices"
	"sync"
)

// ToolRegistry stores and retrieves Tool implementations by name.
type ToolRegistry interface {
	// Register adds a tool. Returns an error if a tool with the same name already exists.
	Register(tool Tool) error

	// Get returns a tool by name. The second return value is false if not found.
	Get(name string) (Tool, bool)

	// List returns metadata for all registered tools, sorted by name.
	List() []ToolInfo
}

// memoryRegistry is the default in-memory ToolRegistry implementation.
type memoryRegistry struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	metadata map[string]ToolMetadata
}

// NewMemoryRegistry creates an empty in-memory ToolRegistry.
func NewMemoryRegistry() ToolRegistry {
	return &memoryRegistry{
		tools:    make(map[string]Tool),
		metadata: make(map[string]ToolMetadata),
	}
}

func (r *memoryRegistry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tools: cannot register nil tool")
	}
	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tools: tool name must not be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tools: tool %q already registered", name)
	}
	r.tools[name] = tool
	if _, exists := r.metadata[name]; !exists {
		r.metadata[name] = metadataFromTool(tool)
	}
	return nil
}

func (r *memoryRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]
	return t, ok
}

func (r *memoryRegistry) List() []ToolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]ToolInfo, 0, len(r.tools))
	for _, t := range r.tools {
		metadata, ok := r.metadata[t.Name()]
		if !ok {
			metadata = metadataFromTool(t)
		}
		infos = append(infos, ToolInfo{
			Name:             t.Name(),
			Description:      t.Description(),
			Enabled:          true,
			RiskLevel:        string(metadata.RiskLevel),
			Timeout:          metadata.Timeout,
			RequiresApproval: metadata.RequiresApproval,
			AllowedPaths:     slices.Clone(metadata.AllowedPaths),
		})
	}
	slices.SortFunc(infos, func(a, b ToolInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return infos
}

func (r *memoryRegistry) RegisterMetadata(metadata ToolMetadata) error {
	metadata, err := normalizeMetadata(metadata)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.metadata[metadata.Name]; exists {
		return fmt.Errorf("tools: metadata for %q already registered", metadata.Name)
	}
	r.metadata[metadata.Name] = copyMetadata(metadata)
	return nil
}

func (r *memoryRegistry) Metadata(name string) (ToolMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata, ok := r.metadata[name]
	if !ok {
		return ToolMetadata{}, false
	}
	return copyMetadata(metadata), true
}

func (r *memoryRegistry) ListMetadata() []ToolMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata := make([]ToolMetadata, 0, len(r.metadata))
	for _, item := range r.metadata {
		metadata = append(metadata, copyMetadata(item))
	}
	slices.SortFunc(metadata, func(a, b ToolMetadata) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return metadata
}
