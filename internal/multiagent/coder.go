package multiagent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mimoneko/mimoneko/internal/agent"
	"github.com/mimoneko/mimoneko/internal/task"
)

// CoderAgent adapts SingleAgentRuntime for the multi-agent pipeline.
//
// Rules:
//   - Must delegate to SingleAgentRuntime.Run (not re-implement tool logic)
//   - Must use UseWorktree=true
//   - Must reuse existing WorktreeID if available
//   - Must not apply patch
//   - Must not commit/push
//   - Must not directly call ToolRuntime
//   - Must not write to main workspace
type CoderAgent struct {
	singleAgent agent.AgentRuntime
}

// NewCoderAgent creates a new CoderAgent wrapping the given SingleAgentRuntime.
func NewCoderAgent(singleAgent agent.AgentRuntime) *CoderAgent {
	return &CoderAgent{
		singleAgent: singleAgent,
	}
}

// CodeRequest is the input to CoderAgent.Code.
type CodeRequest struct {
	Goal       string
	Plan       TaskPlan
	Contract   task.TaskContract
	RepoRoot   string
	TaskID     string
	WorktreeID string // empty on first iteration; reused on subsequent iterations
	DryRun     bool
	Feedback   string // reviewer feedback from previous iteration
}

// CodeResult is the output of CoderAgent.Code.
type CodeResult struct {
	Result     agent.AgentRunResult
	Message    AgentMessage
	WorktreeID string // the worktree ID used/created
}

// Code executes coding by delegating to SingleAgentRuntime.
func (c *CoderAgent) Code(ctx context.Context, req CodeRequest) (CodeResult, error) {
	// Build the coder goal from the plan and feedback
	goal := buildCoderGoal(req)

	agentReq := agent.AgentRunRequest{
		TaskID:      req.TaskID,
		RepoRoot:    req.RepoRoot,
		Goal:        goal,
		Contract:    req.Contract,
		DryRun:      req.DryRun,
		UseWorktree: true, // Always use worktree
		WorktreeID:  req.WorktreeID,
	}

	result, err := c.singleAgent.Run(ctx, agentReq)
	if err != nil {
		return CodeResult{}, fmt.Errorf("coder: single agent run failed: %w", err)
	}

	// Extract worktree ID from result
	worktreeID := result.WorktreeID
	if worktreeID == "" {
		worktreeID = req.WorktreeID
	}

	msg := AgentMessage{
		Role:      AgentRoleCoder,
		Content:   fmt.Sprintf("Coding completed: state=%s, steps=%d", result.State, len(result.Steps)),
		Metadata:  map[string]string{"worktree_id": worktreeID, "state": string(result.State)},
		CreatedAt: time.Now().UTC(),
	}

	return CodeResult{
		Result:     result,
		Message:    msg,
		WorktreeID: worktreeID,
	}, nil
}

// buildCoderGoal constructs the goal string for the coder agent.
// It includes the original goal, plan steps, and reviewer feedback.
func buildCoderGoal(req CodeRequest) string {
	var sb strings.Builder

	sb.WriteString("Goal: ")
	sb.WriteString(req.Goal)
	sb.WriteString("\n\n")

	if len(req.Plan.Steps) > 0 {
		sb.WriteString("Plan:\n")
		for _, step := range req.Plan.Steps {
			sb.WriteString(fmt.Sprintf("%d. %s: %s\n", step.Index, step.Title, step.Description))
			if step.ExpectedOutcome != "" {
				sb.WriteString(fmt.Sprintf("   Expected: %s\n", step.ExpectedOutcome))
			}
		}
		sb.WriteString("\n")
	}

	if req.Feedback != "" {
		sb.WriteString("Reviewer feedback from previous iteration:\n")
		sb.WriteString(req.Feedback)
		sb.WriteString("\n\n")
		sb.WriteString("Please address the reviewer's feedback.\n")
	}

	return sb.String()
}
