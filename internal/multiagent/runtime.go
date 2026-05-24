package multiagent

import (
	"context"
	"fmt"
	"time"

	"github.com/reasonforge/reasonforge/internal/agent"
	"github.com/reasonforge/reasonforge/internal/contextengine"
	"github.com/reasonforge/reasonforge/internal/modelrouter"
	"github.com/reasonforge/reasonforge/internal/review"
	"github.com/reasonforge/reasonforge/internal/worktree"
)

// Dependencies holds the external dependencies for the Multi-Agent Runtime.
type Dependencies struct {
	ContextEngine   contextengine.ContextEngine
	ModelRouter     modelrouter.ModelRouter
	SingleAgent     agent.AgentRuntime
	ReviewMgr       review.PatchReviewManager
	WorktreeMgr     worktree.WorktreeManager
	CheckpointStore MultiAgentCheckpointStore
}

// DefaultMultiAgentRuntime implements MultiAgentRuntime with a
// Planner -> Coder -> Reviewer iteration loop.
type DefaultMultiAgentRuntime struct {
	deps Dependencies
}

// NewDefaultMultiAgentRuntime creates a new DefaultMultiAgentRuntime.
func NewDefaultMultiAgentRuntime(deps Dependencies) *DefaultMultiAgentRuntime {
	return &DefaultMultiAgentRuntime{deps: deps}
}

// Run executes the multi-agent loop: Plan -> Code -> Review -> iterate.
func (rt *DefaultMultiAgentRuntime) Run(ctx context.Context, req MultiAgentRunRequest) (MultiAgentRunResult, error) {
	// Validate max iterations
	maxIter, err := ValidateMaxIterations(req.MaxIterations)
	if err != nil {
		return MultiAgentRunResult{}, err
	}

	// Validate the contract
	if err := req.Contract.Validate(); err != nil {
		return MultiAgentRunResult{}, fmt.Errorf("multiagent: invalid task contract: %w", err)
	}

	// Generate run ID
	runID, err := GenerateMultiAgentRunID()
	if err != nil {
		return MultiAgentRunResult{}, fmt.Errorf("multiagent: generate run id: %w", err)
	}

	result := MultiAgentRunResult{
		RunID:      runID,
		TaskID:     req.TaskID,
		State:      MultiAgentStatePending,
		Iterations: []MultiAgentIteration{},
		StartedAt:  time.Now().UTC(),
	}

	// Create shared task context
	sharedCtx := NewSharedTaskContext(req.Goal)

	// Save initial checkpoint
	if err := rt.saveCheckpoint(ctx, result, "init"); err != nil {
		result.State = MultiAgentStateFailed
		result.Error = fmt.Sprintf("initial checkpoint failed: %v", err)
		result.FinishedAt = time.Now().UTC()
		return result, nil
	}

	result.State = MultiAgentStateRunning

	// Build agents
	planner := NewPlannerAgent(rt.deps.ModelRouter, rt.deps.ContextEngine, req.Model)
	coder := NewCoderAgent(rt.deps.SingleAgent)
	reviewer := NewReviewerAgent(rt.deps.ReviewMgr, rt.deps.WorktreeMgr)

	// === Phase 1: Planner ===
	planResult, err := planner.Plan(ctx, PlanRequest{
		Goal:     req.Goal,
		Contract: req.Contract,
		RepoRoot: req.RepoRoot,
	})
	if err != nil {
		result.State = MultiAgentStateFailed
		result.Error = fmt.Sprintf("planner failed: %v", err)
		result.FinishedAt = time.Now().UTC()
		rt.saveCheckpointOrFail(ctx, &result, "planner_failed")
		return result, nil
	}

	sharedCtx.Plan = planResult.Plan
	sharedCtx.AddMessage(planResult.Message)
	result.Plan = planResult.Plan

	if err := rt.saveCheckpoint(ctx, result, "planner_done"); err != nil {
		result.State = MultiAgentStateFailed
		result.Error = fmt.Sprintf("planner checkpoint failed: %v", err)
		result.FinishedAt = time.Now().UTC()
		return result, nil
	}

	// === Phase 2: Iteration Loop (Coder -> Reviewer) ===
	var worktreeID string

	for iterIndex := 0; iterIndex < maxIter; iterIndex++ {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			result.State = MultiAgentStateCancelled
			result.Error = fmt.Sprintf("context cancelled at iteration %d: %v", iterIndex, err)
			result.FinishedAt = time.Now().UTC()
			rt.saveCheckpointOrFail(ctx, &result, "cancelled")
			return result, nil
		}

		iterStartedAt := time.Now().UTC()

		// Save iteration start checkpoint
		if err := rt.saveCheckpoint(ctx, result, "loop_start"); err != nil {
			result.State = MultiAgentStateFailed
			result.Error = fmt.Sprintf("iteration %d start checkpoint failed: %v", iterIndex, err)
			result.FinishedAt = time.Now().UTC()
			return result, nil
		}

		// Build reviewer feedback for the coder
		feedback := sharedCtx.LastReviewerMessage()

		// === Coder ===
		coderResult, err := coder.Code(ctx, CodeRequest{
			Goal:       req.Goal,
			Plan:       planResult.Plan,
			Contract:   req.Contract,
			RepoRoot:   req.RepoRoot,
			TaskID:     req.TaskID,
			WorktreeID: worktreeID,
			DryRun:     req.DryRun,
			Feedback:   feedback,
		})
		if err != nil {
			result.State = MultiAgentStateFailed
			result.Error = fmt.Sprintf("coder failed at iteration %d: %v", iterIndex, err)
			result.FinishedAt = time.Now().UTC()
			rt.saveCheckpointOrFail(ctx, &result, "coder_failed")
			return result, nil
		}

		worktreeID = coderResult.WorktreeID
		result.WorktreeID = worktreeID
		sharedCtx.WorktreeID = worktreeID
		sharedCtx.AddMessage(coderResult.Message)

		if err := rt.saveCheckpoint(ctx, result, "coder_done"); err != nil {
			result.State = MultiAgentStateFailed
			result.Error = fmt.Sprintf("iteration %d coder checkpoint failed: %v", iterIndex, err)
			result.FinishedAt = time.Now().UTC()
			return result, nil
		}

		// === Reviewer ===
		reviewResult, err := reviewer.Review(ctx, ReviewRequest{
			CoderResult:    coderResult.Result,
			Contract:       req.Contract,
			RepoRoot:       req.RepoRoot,
			WorktreeID:     worktreeID,
			TaskID:         req.TaskID,
			RunTests:       true,
			TestCommands:   req.TestCommands,
			UseModelReview: req.UseModelReview,
			Model:          req.Model,
		})
		if err != nil {
			result.State = MultiAgentStateFailed
			result.Error = fmt.Sprintf("reviewer failed at iteration %d: %v", iterIndex, err)
			result.FinishedAt = time.Now().UTC()
			rt.saveCheckpointOrFail(ctx, &result, "reviewer_failed")
			return result, nil
		}

		sharedCtx.AddReviewReport(reviewResult.Report)
		sharedCtx.AddMessage(reviewResult.Message)

		// Record iteration
		iteration := MultiAgentIteration{
			Index:           iterIndex,
			PlannerMessage:  planResult.Message,
			CoderResult:     coderResult.Result,
			ReviewReport:    reviewResult.Report,
			ReviewerMessage: reviewResult.Message,
			Recommendation:  reviewResult.Recommendation,
			StartedAt:       iterStartedAt,
			FinishedAt:      time.Now().UTC(),
		}
		result.Iterations = append(result.Iterations, iteration)

		// === Decision ===
		switch reviewResult.Recommendation {
		case review.RecommendationApprove:
			result.State = MultiAgentStateSucceeded
			result.FinalRecommendation = review.RecommendationApprove
			result.FinalMessage = reviewResult.Message.Content
			result.FinishedAt = time.Now().UTC()
			rt.saveCheckpointOrFail(ctx, &result, "loop_end")
			return result, nil

		case review.RecommendationReject:
			result.State = MultiAgentStateRejected
			result.FinalRecommendation = review.RecommendationReject
			result.FinalMessage = reviewResult.Message.Content
			result.FinishedAt = time.Now().UTC()
			rt.saveCheckpointOrFail(ctx, &result, "loop_end")
			return result, nil

		case review.RecommendationRequestChanges:
			// If we've exhausted iterations, stop with request_changes state
			if iterIndex >= maxIter-1 {
				result.State = MultiAgentStateRequestChanges
				result.FinalRecommendation = review.RecommendationRequestChanges
				result.FinalMessage = fmt.Sprintf("Max iterations (%d) reached with request_changes recommendation", maxIter)
				result.FinishedAt = time.Now().UTC()
				rt.saveCheckpointOrFail(ctx, &result, "loop_end")
				return result, nil
			}
			// Otherwise, continue to next iteration with feedback
		}
	}

	// Should not reach here, but safety fallback
	result.State = MultiAgentStateFailed
	result.Error = fmt.Sprintf("iteration loop exited unexpectedly after %d iterations", maxIter)
	result.FinishedAt = time.Now().UTC()
	rt.saveCheckpointOrFail(ctx, &result, "loop_end")
	return result, nil
}

// saveCheckpoint persists the current multi-agent run state.
func (rt *DefaultMultiAgentRuntime) saveCheckpoint(ctx context.Context, result MultiAgentRunResult, phase string) error {
	if rt.deps.CheckpointStore == nil {
		return fmt.Errorf("multiagent: checkpoint store not configured")
	}

	iteration := len(result.Iterations)

	cp := MultiAgentCheckpoint{
		RunID:      result.RunID,
		TaskID:     result.TaskID,
		State:      result.State,
		Iteration:  iteration,
		Phase:      phase,
		WorktreeID: result.WorktreeID,
		Error:      result.Error,
		CreatedAt:  time.Now().UTC(),
	}

	if len(result.Iterations) > 0 {
		lastIter := result.Iterations[len(result.Iterations)-1]
		cp.Recommendation = string(lastIter.Recommendation)
	}

	// Include plan in checkpoint for planner_done and later phases
	if phase == "planner_done" || phase == "loop_start" || phase == "loop_end" {
		planCopy := result.Plan
		cp.Plan = &planCopy
	}

	if err := rt.deps.CheckpointStore.Save(ctx, cp); err != nil {
		return fmt.Errorf("multiagent: checkpoint save failed: %w", err)
	}

	return nil
}

// saveCheckpointOrFail persists a checkpoint and overrides the result state to
// failed if the save fails. This ensures no checkpoint error is silently ignored.
func (rt *DefaultMultiAgentRuntime) saveCheckpointOrFail(ctx context.Context, result *MultiAgentRunResult, phase string) {
	if err := rt.saveCheckpoint(ctx, *result, phase); err != nil {
		result.State = MultiAgentStateFailed
		result.Error = fmt.Sprintf("checkpoint failed (%s): %v", phase, err)
		result.FinishedAt = time.Now().UTC()
	}
}
