package patch

import (
	"context"
	"time"

	"github.com/reasonforge/reasonforge/internal/task"
)

// FileChange describes a single file modification in a patch.
type FileChange struct {
	// Path is the repo-relative path of the changed file.
	Path string `json:"path"`

	// Status is the git status code: "added", "modified", "deleted", "renamed".
	Status string `json:"status"`

	// Additions is the number of lines added.
	Additions int `json:"additions"`

	// Deletions is the number of lines deleted.
	Deletions int `json:"deletions"`
}

// DiffSummary is an aggregate summary of changes in a patch.
type DiffSummary struct {
	// FilesChanged is the total number of files changed.
	FilesChanged int `json:"files_changed"`

	// Additions is the total lines added.
	Additions int `json:"additions"`

	// Deletions is the total lines deleted.
	Deletions int `json:"deletions"`

	// HasBinary indicates whether any binary files are included.
	HasBinary bool `json:"has_binary"`
}

// PatchViolation describes a file that violates the TaskContract constraints.
type PatchViolation struct {
	// Path is the repo-relative path of the violating file.
	Path string `json:"path"`

	// Reason explains why the path is a violation.
	Reason string `json:"reason"`
}

// PatchPreview is the result of previewing a worktree's changes.
type PatchPreview struct {
	// WorktreeID identifies the worktree this preview is for.
	WorktreeID string `json:"worktree_id"`

	// FilesChanged lists individual file changes.
	FilesChanged []FileChange `json:"files_changed"`

	// Diff is the raw unified diff output.
	Diff string `json:"diff"`

	// Summary is the aggregate diff summary.
	Summary DiffSummary `json:"summary"`

	// RiskLevel is the overall risk assessment: "low", "medium", "high".
	RiskLevel string `json:"risk_level"`

	// Violations lists any TaskContract violations found.
	Violations []PatchViolation `json:"violations,omitempty"`

	// GeneratedAt is when the preview was generated.
	GeneratedAt time.Time `json:"generated_at"`
}

// PatchPreviewRequest is the input to PatchManager.Preview.
type PatchPreviewRequest struct {
	// RepoRoot is the main repository root.
	RepoRoot string

	// WorktreeID identifies the worktree to preview.
	WorktreeID string

	// Contract defines the execution boundary for violation checking.
	Contract task.TaskContract
}

// PatchApplyRequest is the input to PatchManager.Apply.
type PatchApplyRequest struct {
	// RepoRoot is the main repository root.
	RepoRoot string

	// WorktreeID identifies the worktree whose changes to apply.
	WorktreeID string

	// Contract defines the execution boundary for violation checking.
	Contract task.TaskContract

	// DryRun indicates whether to only output the diff without modifying the main workspace.
	DryRun bool

	// MaxDiffBytes caps the diff output size.
	MaxDiffBytes int
}

// PatchApplyResult is the result of applying a patch.
type PatchApplyResult struct {
	// WorktreeID identifies the worktree that was applied.
	WorktreeID string `json:"worktree_id"`

	// Applied indicates whether the patch was actually applied.
	Applied bool `json:"applied"`

	// FilesChanged lists individual file changes that were applied.
	FilesChanged []FileChange `json:"files_changed"`

	// Summary is the aggregate summary of applied changes.
	Summary DiffSummary `json:"summary"`

	// StateUpdateError holds the error message if the worktree state update
	// failed after a successful patch apply. The patch itself is not rolled
	// back, but callers should be aware of this inconsistency.
	StateUpdateError string `json:"state_update_error,omitempty"`
}

// PatchDiscardRequest is the input to PatchManager.Discard.
type PatchDiscardRequest struct {
	// RepoRoot is the main repository root.
	RepoRoot string

	// WorktreeID identifies the worktree to discard.
	WorktreeID string
}

// PatchManager handles patch preview, apply, and discard operations.
//
// It generates diffs between a worktree and the main workspace,
// checks them against TaskContract constraints, and applies or
// discards them as requested.
type PatchManager interface {
	// Preview generates a diff preview for the worktree's changes
	// and checks them against the TaskContract.
	Preview(ctx context.Context, req PatchPreviewRequest) (PatchPreview, error)

	// Apply applies the worktree's changes to the main workspace.
	// It refuses if there are violations or if the main workspace is dirty.
	Apply(ctx context.Context, req PatchApplyRequest) (PatchApplyResult, error)

	// Discard removes the worktree and marks it as discarded.
	// It never deletes files from the main workspace.
	Discard(ctx context.Context, req PatchDiscardRequest) error
}
