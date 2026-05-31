package worktree

import (
	"context"
	"time"
)

// WorktreeState represents the lifecycle state of a worktree.
type WorktreeState string

const (
	WorktreeStateActive    WorktreeState = "active"
	WorktreeStateApplied   WorktreeState = "applied"
	WorktreeStateDiscarded WorktreeState = "discarded"
	WorktreeStateFailed    WorktreeState = "failed"
)

// IsTerminal returns true if the state is a terminal state.
func (s WorktreeState) IsTerminal() bool {
	switch s {
	case WorktreeStateApplied, WorktreeStateDiscarded, WorktreeStateFailed:
		return true
	default:
		return false
	}
}

// CreateWorktreeRequest is the input to WorktreeManager.Create.
type CreateWorktreeRequest struct {
	// RepoRoot is the root of the git repository.
	RepoRoot string

	// TaskID identifies the task this worktree belongs to.
	TaskID string

	// BaseRef is the git ref to base the worktree on (e.g. "HEAD", "main").
	// Empty means HEAD.
	BaseRef string

	// Metadata carries optional key-value pairs.
	Metadata map[string]string
}

// WorktreeInfo holds metadata about a created worktree.
type WorktreeInfo struct {
	// ID is the unique identifier for this worktree (crypto/rand).
	ID string `json:"id"`

	// TaskID is the task this worktree belongs to.
	TaskID string `json:"task_id"`

	// RepoRoot is the original repository root.
	RepoRoot string `json:"repo_root"`

	// Path is the filesystem path of the worktree.
	Path string `json:"path"`

	// Branch is the name of the branch created for this worktree.
	Branch string `json:"branch"`

	// BaseRef is the git ref the worktree was based on.
	BaseRef string `json:"base_ref"`

	// CreatedAt is when the worktree was created.
	CreatedAt time.Time `json:"created_at"`

	// State is the current lifecycle state.
	State WorktreeState `json:"state"`

	// Metadata carries optional key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// WorktreeManager manages isolated git worktrees for agent task execution.
//
// All worktrees are created under .mimoneko/worktrees/<task_id>/<worktree_id>.
// The manager maintains a JSONL registry of all worktrees it has created.
// It will never delete worktrees it did not create.
type WorktreeManager interface {
	// Create creates a new isolated git worktree for the given task.
	Create(ctx context.Context, req CreateWorktreeRequest) (WorktreeInfo, error)

	// Remove removes a worktree and its branch.
	// It only removes worktrees that are in the registry.
	Remove(ctx context.Context, id string) error

	// Get returns information about a specific worktree.
	Get(ctx context.Context, id string) (WorktreeInfo, error)

	// List returns all worktrees managed by this manager.
	List(ctx context.Context) ([]WorktreeInfo, error)

	// UpdateState updates the state of a worktree in the registry.
	UpdateState(ctx context.Context, id string, state WorktreeState) error
}
