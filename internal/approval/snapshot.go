package approval

import (
	"fmt"
	"time"
)

// ResumeSnapshot contains all information needed to resume a tool execution
// after approval. This is a data model only - actual resume is not implemented.
type ResumeSnapshot struct {
	// ApprovalID links this snapshot to an ApprovalRequest.
	ApprovalID string `json:"approval_id"`

	// RunID identifies the run this snapshot belongs to.
	RunID string `json:"run_id"`

	// ToolName is the name of the tool to be executed.
	ToolName string `json:"tool_name"`

	// ToolArgs contains the raw tool arguments for future resume.
	// This is NOT displayed in CLI - only SanitizedPreview is shown.
	ToolArgs map[string]string `json:"tool_args"`

	// RiskLevel is the risk level of the operation.
	RiskLevel string `json:"risk_level"`

	// Reason explains why approval was required.
	Reason string `json:"reason"`

	// Path is the file path involved (if any).
	Path string `json:"path,omitempty"`

	// Command is the command to be executed (if any).
	Command string `json:"command,omitempty"`

	// CreatedAt is when the snapshot was created.
	CreatedAt time.Time `json:"created_at"`

	// SanitizedPreview is a human-readable, sanitized preview of the operation.
	// This is what CLI displays - it never contains raw secrets.
	SanitizedPreview string `json:"sanitized_preview"`
}

// Validate checks that the snapshot has all required fields.
func (s *ResumeSnapshot) Validate() error {
	if s.ApprovalID == "" {
		return fmt.Errorf("snapshot: approval_id is required")
	}
	if s.RunID == "" {
		return fmt.Errorf("snapshot: run_id is required")
	}
	if s.ToolName == "" {
		return fmt.Errorf("snapshot: tool_name is required")
	}
	if s.CreatedAt.IsZero() {
		return fmt.Errorf("snapshot: created_at is required")
	}
	return nil
}
