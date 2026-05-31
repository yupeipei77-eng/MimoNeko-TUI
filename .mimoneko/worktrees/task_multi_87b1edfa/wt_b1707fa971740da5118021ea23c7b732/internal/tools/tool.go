package tools

import "context"

// Tool is the interface every tool implementation must satisfy.
type Tool interface {
	// Name returns the unique tool identifier, e.g. "file_read".
	Name() string

	// Description returns a short human-readable description.
	Description() string

	// RiskLevel returns the tool's risk classification: "low" or "medium".
	RiskLevel() string

	// Run executes the tool. The ToolRuntime guarantees that:
	//   - RepoRoot is set and validated
	//   - Args are present (tool-specific validation is the tool's responsibility)
	//   - Timeout context is applied
	//   - DryRun flag is respected
	Run(ctx context.Context, req ToolRequest) (ToolResponse, error)
}
