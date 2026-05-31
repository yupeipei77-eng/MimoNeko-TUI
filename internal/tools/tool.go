package tools

import "context"

// ConcurrencyClass indicates whether a tool can run concurrently with others.
type ConcurrencyClass string

const (
	// ConcurrencyReadOnly indicates the tool only reads data and is safe to run concurrently.
	ConcurrencyReadOnly ConcurrencyClass = "read_only"

	// ConcurrencyWrite indicates the tool modifies data and must run serially.
	ConcurrencyWrite ConcurrencyClass = "write"

	// ConcurrencyDestructive indicates the tool can cause destructive changes.
	ConcurrencyDestructive ConcurrencyClass = "destructive"
)

// Tool is the interface every tool implementation must satisfy.
type Tool interface {
	// Name returns the unique tool identifier, e.g. "file_read".
	Name() string

	// Description returns a short human-readable description.
	Description() string

	// RiskLevel returns the tool's risk classification: "low" or "medium".
	RiskLevel() string

	// Concurrency returns the tool's concurrency classification.
	// Default implementations can return ConcurrencyReadOnly for safe tools.
	Concurrency() ConcurrencyClass

	// Run executes the tool. The ToolRuntime guarantees that:
	//   - RepoRoot is set and validated
	//   - Args are present (tool-specific validation is the tool's responsibility)
	//   - Timeout context is applied
	//   - DryRun flag is respected
	Run(ctx context.Context, req ToolRequest) (ToolResponse, error)
}
