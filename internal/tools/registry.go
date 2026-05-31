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
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewMemoryRegistry creates an empty in-memory ToolRegistry.
func NewMemoryRegistry() ToolRegistry {
	return &memoryRegistry{
		tools: make(map[string]Tool),
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
		infos = append(infos, ToolInfo{
			Name:        t.Name(),
			Description: t.Description(),
			Enabled:     true,
			RiskLevel:   t.RiskLevel(),
		})
	}
	slices.SortFunc(infos, func(a, b ToolInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return infos
}
