// Package approval provides the Approval Request data model for MioNeko.
//
// This package implements the data model only. It does NOT:
//   - Implement interactive approval workflows
//   - Persist requests to disk
//   - Integrate with EventStore
//   - Modify ToolRuntime execution behavior
//
// The model is designed for future CLI, persistence, and resume-execution features.
package approval

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// ApprovalStatus represents the current state of an approval request.
type ApprovalStatus string

const (
	StatusPending  ApprovalStatus = "pending"
	StatusApproved ApprovalStatus = "approved"
	StatusRejected ApprovalStatus = "rejected"
	StatusExpired  ApprovalStatus = "expired"
)

// ApprovalScope defines what type of operation requires approval.
type ApprovalScope string

const (
	ScopeTool    ApprovalScope = "tool"
	ScopePath    ApprovalScope = "path"
	ScopePatch   ApprovalScope = "patch"
	ScopeCommand ApprovalScope = "command"
)

// ApprovalRequest represents a request for approval before executing a sensitive operation.
type ApprovalRequest struct {
	// ID is the unique request identifier.
	ID string `json:"id"`

	// RunID identifies the run this request belongs to.
	RunID string `json:"run_id"`

	// ToolName is the name of the tool requesting approval.
	ToolName string `json:"tool_name"`

	// Scope defines what type of operation requires approval.
	Scope ApprovalScope `json:"scope"`

	// Status is the current approval status.
	Status ApprovalStatus `json:"status"`

	// Reason explains why approval is required.
	Reason string `json:"reason"`

	// RiskLevel is the risk level of the operation (low/medium/high/critical).
	RiskLevel string `json:"risk_level"`

	// Path is the file path involved (for path-scoped approvals).
	Path string `json:"path,omitempty"`

	// Command is the command being executed (for command-scoped approvals).
	Command string `json:"command,omitempty"`

	// PatchID identifies the patch (for patch-scoped approvals).
	PatchID string `json:"patch_id,omitempty"`

	// CreatedAt is when the request was created.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the request expires (zero means no expiration).
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// DecidedAt is when the request was approved/rejected (zero if pending).
	DecidedAt time.Time `json:"decided_at,omitempty"`

	// DecidedBy identifies who made the decision (empty if pending).
	DecidedBy string `json:"decided_by,omitempty"`

	// ResumedAt is when the approved request was resumed for execution (zero if not resumed).
	ResumedAt time.Time `json:"resumed_at,omitempty"`
}

// NewRequest creates a new ApprovalRequest with the given parameters.
// The ID is auto-generated, Status is set to pending, and CreatedAt is set to now.
func NewRequest(
	runID string,
	toolName string,
	scope ApprovalScope,
	reason string,
	riskLevel string,
	path string,
	command string,
	patchID string,
) (*ApprovalRequest, error) {
	id, err := generateRequestID()
	if err != nil {
		return nil, fmt.Errorf("approval: generate request id: %w", err)
	}

	req := &ApprovalRequest{
		ID:        id,
		RunID:     runID,
		ToolName:  toolName,
		Scope:     scope,
		Status:    StatusPending,
		Reason:    reason,
		RiskLevel: riskLevel,
		Path:      path,
		Command:   command,
		PatchID:   patchID,
		CreatedAt: time.Now().UTC(),
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	return req, nil
}

// Approve marks the request as approved.
// Returns an error if the request cannot be approved (not pending, expired, or rejected).
func (r *ApprovalRequest) Approve(decidedBy string) error {
	if r.Status == StatusExpired {
		return fmt.Errorf("approval: cannot approve expired request %s", r.ID)
	}
	if r.Status == StatusRejected {
		return fmt.Errorf("approval: cannot approve rejected request %s", r.ID)
	}
	if r.Status == StatusApproved {
		return fmt.Errorf("approval: request %s is already approved", r.ID)
	}
	if r.Status != StatusPending {
		return fmt.Errorf("approval: cannot approve request %s with status %s", r.ID, r.Status)
	}

	r.Status = StatusApproved
	r.DecidedAt = time.Now().UTC()
	r.DecidedBy = decidedBy
	return nil
}

// Reject marks the request as rejected.
// Returns an error if the request cannot be rejected (not pending, expired, or approved).
func (r *ApprovalRequest) Reject(decidedBy string) error {
	if r.Status == StatusExpired {
		return fmt.Errorf("approval: cannot reject expired request %s", r.ID)
	}
	if r.Status == StatusApproved {
		return fmt.Errorf("approval: cannot reject approved request %s", r.ID)
	}
	if r.Status == StatusRejected {
		return fmt.Errorf("approval: request %s is already rejected", r.ID)
	}
	if r.Status != StatusPending {
		return fmt.Errorf("approval: cannot reject request %s with status %s", r.ID, r.Status)
	}

	r.Status = StatusRejected
	r.DecidedAt = time.Now().UTC()
	r.DecidedBy = decidedBy
	return nil
}

// Expire marks the request as expired.
// Returns an error if the request is not pending.
func (r *ApprovalRequest) Expire() error {
	if r.Status != StatusPending {
		return fmt.Errorf("approval: cannot expire request %s with status %s", r.ID, r.Status)
	}

	r.Status = StatusExpired
	r.DecidedAt = time.Now().UTC()
	return nil
}

// IsPending returns true if the request is still pending.
func (r *ApprovalRequest) IsPending() bool {
	return r.Status == StatusPending
}

// IsResumed returns true if the request has been resumed.
func (r *ApprovalRequest) IsResumed() bool {
	return !r.ResumedAt.IsZero()
}

// Resume marks the request as resumed.
// Returns an error if the request cannot be resumed.
func (r *ApprovalRequest) Resume() error {
	if r.Status != StatusApproved {
		return fmt.Errorf("approval: cannot resume request %s with status %s", r.ID, r.Status)
	}
	if r.IsResumed() {
		return fmt.Errorf("approval: request %s already resumed", r.ID)
	}

	r.ResumedAt = time.Now().UTC()
	return nil
}

// IsExpired returns true if the request has expired based on the current time.
// A request is expired if:
//   - Status is already StatusExpired, OR
//   - ExpiresAt is set and the current time is after ExpiresAt
func (r *ApprovalRequest) IsExpired(now time.Time) bool {
	if r.Status == StatusExpired {
		return true
	}
	if !r.ExpiresAt.IsZero() && now.After(r.ExpiresAt) {
		return true
	}
	return false
}

// Validate checks that the request has all required fields.
func (r *ApprovalRequest) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("approval: id is required")
	}
	if r.RunID == "" {
		return fmt.Errorf("approval: run_id is required")
	}
	if r.ToolName == "" {
		return fmt.Errorf("approval: tool_name is required")
	}
	if r.Scope == "" {
		return fmt.Errorf("approval: scope is required")
	}
	if r.Status == "" {
		return fmt.Errorf("approval: status is required")
	}
	if r.Reason == "" {
		return fmt.Errorf("approval: reason is required")
	}
	if r.RiskLevel == "" {
		return fmt.Errorf("approval: risk_level is required")
	}

	// Validate scope
	switch r.Scope {
	case ScopeTool, ScopePath, ScopePatch, ScopeCommand:
		// valid
	default:
		return fmt.Errorf("approval: invalid scope %q", r.Scope)
	}

	// Validate status
	switch r.Status {
	case StatusPending, StatusApproved, StatusRejected, StatusExpired:
		// valid
	default:
		return fmt.Errorf("approval: invalid status %q", r.Status)
	}

	return nil
}

// generateRequestID creates a unique request identifier using crypto/rand.
func generateRequestID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "apr_" + hex.EncodeToString(b), nil
}
