package approval

import (
	"context"
	"fmt"
	"time"
)

// ResumeResult represents the result of a resume execution.
type ResumeResult struct {
	// ApprovalID is the ID of the approval request.
	ApprovalID string `json:"approval_id"`

	// RunID is the run ID from the snapshot.
	RunID string `json:"run_id"`

	// ToolName is the tool that was executed.
	ToolName string `json:"tool_name"`

	// RiskLevel is the risk level of the operation.
	RiskLevel string `json:"risk_level"`

	// Status is the result status: started, completed, failed.
	Status string `json:"status"`

	// ErrorMessage is the error message if failed.
	ErrorMessage string `json:"error_message,omitempty"`

	// StartedAt is when the resume started.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the resume completed (or failed).
	CompletedAt time.Time `json:"completed_at,omitempty"`

	// DurationMs is the execution duration in milliseconds.
	DurationMs int64 `json:"duration_ms"`
}

// ToolExecutor is the interface for executing tools.
type ToolExecutor interface {
	ExecuteTool(ctx context.Context, toolName string, args map[string]string) (string, error)
}

// ResumeExecutor handles the execution of approved approval requests.
type ResumeExecutor struct {
	approvalStore *FileStore
	snapshotStore *SnapshotStore
	toolExecutor  ToolExecutor
}

// NewResumeExecutor creates a new ResumeExecutor.
func NewResumeExecutor(
	approvalStore *FileStore,
	snapshotStore *SnapshotStore,
	toolExecutor ToolExecutor,
) *ResumeExecutor {
	return &ResumeExecutor{
		approvalStore: approvalStore,
		snapshotStore: snapshotStore,
		toolExecutor:  toolExecutor,
	}
}

// Resume executes an approved approval request.
func (e *ResumeExecutor) Resume(ctx context.Context, approvalID string) (*ResumeResult, error) {
	// 1. Load approval
	req, err := e.approvalStore.Get(approvalID)
	if err != nil {
		return nil, fmt.Errorf("approval not found: %s", approvalID)
	}

	// 2. Check approval status
	if req.IsPending() {
		return nil, fmt.Errorf("approval still pending: %s", approvalID)
	}
	if req.Status == StatusRejected {
		return nil, fmt.Errorf("approval rejected: %s", approvalID)
	}
	if req.Status == StatusExpired {
		return nil, fmt.Errorf("approval expired: %s", approvalID)
	}
	if req.Status != StatusApproved {
		return nil, fmt.Errorf("approval not approved: %s (status: %s)", approvalID, req.Status)
	}

	// 3. Check if already resumed
	if req.IsResumed() {
		return nil, fmt.Errorf("approval already resumed: %s", approvalID)
	}

	// 4. Load snapshot
	snap, err := e.snapshotStore.Get(approvalID)
	if err != nil {
		return nil, fmt.Errorf("approval snapshot missing: %s", approvalID)
	}

	// 5. Verify snapshot matches approval
	if snap.ApprovalID != req.ID {
		return nil, fmt.Errorf("snapshot mismatch: snapshot approval_id=%s, expected=%s", snap.ApprovalID, req.ID)
	}

	// 6. Execute tool
	startedAt := time.Now().UTC()
	result := &ResumeResult{
		ApprovalID: approvalID,
		RunID:      snap.RunID,
		ToolName:   snap.ToolName,
		RiskLevel:  snap.RiskLevel,
		Status:     "started",
		StartedAt:  startedAt,
	}

	// Execute the tool
	output, err := e.toolExecutor.ExecuteTool(ctx, snap.ToolName, snap.ToolArgs)
	completedAt := time.Now().UTC()
	result.CompletedAt = completedAt
	result.DurationMs = completedAt.Sub(startedAt).Milliseconds()

	if err != nil {
		result.Status = "failed"
		result.ErrorMessage = err.Error()

		// Mark as resumed even on failure to prevent retries
		req.Resume()
		e.approvalStore.Update(req)

		return result, nil
	}

	// 7. Success
	result.Status = "completed"

	// Mark as resumed
	if err := req.Resume(); err != nil {
		return result, fmt.Errorf("mark as resumed failed: %w", err)
	}
	if err := e.approvalStore.Update(req); err != nil {
		return result, fmt.Errorf("update approval failed: %w", err)
	}

	_ = output // Output is available but not stored in result for now

	return result, nil
}
