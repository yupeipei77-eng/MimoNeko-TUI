// Package multiagent implements the Multi-Agent Runtime for MimoNeko.
//
// The multi-agent runtime orchestrates Planner -> Coder -> Reviewer agents
// in an iteration loop. It does NOT reimplement tools, patches, or reviews;
// instead it composes existing Phase 4-6 components:
//
//   - PlannerAgent uses ModelRouter to generate a TaskPlan
//   - CoderAgent delegates to SingleAgentRuntime for code modifications
//   - ReviewerAgent delegates to PatchReviewManager for patch review
//   - IterationLoop drives the cycle until approve/reject/max_iterations
//
// Safety guarantees:
//   - CoderAgent must use UseWorktree=true; never writes to main workspace
//   - ReviewerAgent cannot override deterministic reject rules
//   - No auto-apply, no auto-commit, no auto-push
//   - MaxIterations capped at 5
//   - Sensitive data (API keys, diffs with violations) redacted from SharedTaskContext and checkpoints
package multiagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/agent"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/review"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/task"
)

// MultiAgentState represents the lifecycle state of a multi-agent run.
type MultiAgentState string

const (
	MultiAgentStatePending        MultiAgentState = "pending"
	MultiAgentStateRunning        MultiAgentState = "running"
	MultiAgentStateSucceeded      MultiAgentState = "succeeded"
	MultiAgentStateRequestChanges MultiAgentState = "request_changes"
	MultiAgentStateRejected       MultiAgentState = "rejected"
	MultiAgentStateFailed         MultiAgentState = "failed"
	MultiAgentStateCancelled      MultiAgentState = "cancelled"
)

// IsTerminal returns true if the state is a terminal state.
func (s MultiAgentState) IsTerminal() bool {
	switch s {
	case MultiAgentStateSucceeded, MultiAgentStateRequestChanges,
		MultiAgentStateRejected, MultiAgentStateFailed, MultiAgentStateCancelled:
		return true
	default:
		return false
	}
}

// AgentRole identifies the role of an agent in the multi-agent pipeline.
type AgentRole string

const (
	AgentRolePlanner  AgentRole = "planner"
	AgentRoleCoder    AgentRole = "coder"
	AgentRoleReviewer AgentRole = "reviewer"
)

// AgentMessage carries a structured message from one agent to others.
type AgentMessage struct {
	Role      AgentRole         `json:"role"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// PlanStep describes a single step in a TaskPlan.
type PlanStep struct {
	Index           int      `json:"index"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	TargetPaths     []string `json:"target_paths,omitempty"`
	ExpectedOutcome string   `json:"expected_outcome,omitempty"`
}

// TaskPlan is the output of the PlannerAgent.
// It decomposes a user Goal into a sequence of PlanSteps.
type TaskPlan struct {
	Goal      string     `json:"goal"`
	Steps     []PlanStep `json:"steps"`
	RiskLevel string     `json:"risk_level"`
	Notes     string     `json:"notes,omitempty"`
}

// MultiAgentIteration captures one full Planner->Coder->Reviewer cycle.
type MultiAgentIteration struct {
	Index           int                         `json:"index"`
	PlannerMessage  AgentMessage                `json:"planner_message"`
	CoderResult     agent.AgentRunResult        `json:"coder_result"`
	ReviewReport    review.PatchReviewReport    `json:"review_report"`
	ReviewerMessage AgentMessage                `json:"reviewer_message"`
	Recommendation  review.ReviewRecommendation `json:"recommendation"`
	StartedAt       time.Time                   `json:"started_at"`
	FinishedAt      time.Time                   `json:"finished_at"`
}

// MultiAgentRunRequest is the input to MultiAgentRuntime.Run.
type MultiAgentRunRequest struct {
	TaskID         string            `json:"task_id"`
	RepoRoot       string            `json:"repo_root"`
	Goal           string            `json:"goal"`
	Contract       task.TaskContract `json:"contract"`
	MaxIterations  int               `json:"max_iterations"`
	DryRun         bool              `json:"dry_run"`
	UseWorktree    bool              `json:"use_worktree"`
	UseModelReview bool              `json:"use_model_review"`
	Model          string            `json:"model,omitempty"`
	TestCommands   []string          `json:"test_commands,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// MultiAgentRunResult is the output of MultiAgentRuntime.Run.
type MultiAgentRunResult struct {
	RunID               string                      `json:"run_id"`
	TaskID              string                      `json:"task_id"`
	State               MultiAgentState             `json:"state"`
	Plan                TaskPlan                    `json:"plan"`
	Iterations          []MultiAgentIteration       `json:"iterations"`
	FinalRecommendation review.ReviewRecommendation `json:"final_recommendation"`
	FinalMessage        string                      `json:"final_message,omitempty"`
	WorktreeID          string                      `json:"worktree_id,omitempty"`
	Error               string                      `json:"error,omitempty"`
	StartedAt           time.Time                   `json:"started_at"`
	FinishedAt          time.Time                   `json:"finished_at,omitempty"`
}

// MultiAgentRuntime is the interface for the multi-agent orchestration runtime.
type MultiAgentRuntime interface {
	// Run executes the multi-agent loop: Plan -> Code -> Review -> iterate.
	Run(ctx context.Context, req MultiAgentRunRequest) (MultiAgentRunResult, error)
}

// MaxAllowedIterations is the hard upper bound on iteration count.
const MaxAllowedIterations = 5

// DefaultMaxIterations is the default number of iterations.
const DefaultMaxIterations = 2

// ValidateMaxIterations ensures the iteration count is within bounds.
func ValidateMaxIterations(n int) (int, error) {
	if n <= 0 {
		return DefaultMaxIterations, nil
	}
	if n > MaxAllowedIterations {
		return 0, fmt.Errorf("multiagent: max_iterations %d exceeds allowed maximum %d", n, MaxAllowedIterations)
	}
	return n, nil
}

// GenerateMultiAgentRunID creates a unique run identifier.
func GenerateMultiAgentRunID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("multiagent: generate run id: %w", err)
	}
	return "mar_" + hex.EncodeToString(b), nil
}
