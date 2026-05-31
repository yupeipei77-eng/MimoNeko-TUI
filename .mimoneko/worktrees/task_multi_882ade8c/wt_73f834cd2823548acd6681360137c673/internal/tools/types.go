package tools

// ToolRequest is the standard input for every tool execution.
// All fields are set by ToolRuntime before reaching a Tool.
type ToolRequest struct {
	// ToolName identifies which tool to invoke.
	ToolName string

	// RepoRoot is the workspace root directory; all file paths must stay within it.
	RepoRoot string

	// TaskID identifies the calling task for audit tracing.
	TaskID string

	// Args contains tool-specific key-value arguments.
	Args map[string]string

	// MaxOutputBytes caps the combined Stdout+Stderr output.
	// 0 means use the default from policy.
	MaxOutputBytes int

	// TimeoutSeconds caps execution duration.
	// 0 means use the default from policy.
	TimeoutSeconds int

	// DryRun indicates the tool should report what it would do without performing side effects.
	DryRun bool

	// Metadata carries optional caller-provided key-value pairs.
	Metadata map[string]string
}

// ToolResponse is the structured result of every tool execution.
type ToolResponse struct {
	// ToolName echoes the tool that was invoked.
	ToolName string

	// Success indicates whether the tool completed without error.
	Success bool

	// ExitCode mirrors a process exit code (0 = success, non-zero = failure).
	ExitCode int

	// Stdout holds the primary text output.
	Stdout string

	// Stderr holds error or diagnostic output.
	Stderr string

	// OutputBytes is the number of bytes in Stdout + Stderr before truncation accounting.
	OutputBytes int

	// Truncated is true when output exceeded MaxOutputBytes and was cut.
	Truncated bool

	// Artifacts lists files created or modified by the tool.
	Artifacts []ToolArtifact

	// AuditID links this response to its audit log entry.
	AuditID string

	// Error contains a human-readable error message when Success is false.
	Error string
}

// ToolArtifact describes a file produced or modified by a tool.
type ToolArtifact struct {
	// Kind is the artifact type, e.g. "file_created", "file_modified", "file_patched".
	Kind string

	// Path is the repo-relative path of the artifact.
	Path string

	// ContentHash is a SHA-256 hex digest of the file content after the tool ran.
	ContentHash string
}

// ToolInfo describes a registered tool for listing purposes.
type ToolInfo struct {
	Name        string
	Description string
	Enabled     bool
	RiskLevel   string
}
