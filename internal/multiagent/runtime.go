package multiagent

import (
	"context"
	"fmt"
	"time"

	"github.com/reasonforge/reasonforge/internal/agent"
	"github.com/reasonforge/reasonforge/internal/contextengine"
	"github.com/reasonforge/reasonforge/internal/events"
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

	// EventEmitter is optional. When provided, the multi-agent runtime
	// emits structured events (run.started, planner.started, coder.started,
	// reviewer.started, etc.) for progress tracking. When nil, no events
	// are emitted.
	EventEmitter events.EventEmitter
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

	// Emit run.started event
	events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
		ID:        mustGenerateMultiAgentEventID(),
		RunID:     runID,
		TaskID:    req.TaskID,
		Type:      events.EventRunStarted,
		Source:    "multiagent",
		Status:    "started",
		Message:   "Multi-agent run started",
		StartedAt: result.StartedAt,
		Metadata:  map[string]string{"goal": req.Goal, "max_iterations": fmt.Sprintf("%d", maxIter)},
	})

	// Build agents
	planner := NewPlannerAgent(rt.deps.ModelRouter, rt.deps.ContextEngine, req.Model)
	coder := NewCoderAgent(rt.deps.SingleAgent)
	reviewer := NewReviewerAgent(rt.deps.ReviewMgr, rt.deps.WorktreeMgr)

	// === Phase 1: Planner ===
	plannerStartedAt := time.Now().UTC()
	events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
		ID:        mustGenerateMultiAgentEventID(),
		RunID:     runID,
		TaskID:    req.TaskID,
		Type:      events.EventPlannerStarted,
		Source:    "multiagent",
		Status:    "started",
		Message:   "Planner started",
		StartedAt: plannerStartedAt,
	})

	planResult, err := planner.Plan(ctx, PlanRequest{
		Goal:     req.Goal,
		Contract: req.Contract,
		RepoRoot: req.RepoRoot,
	})
	plannerFinishedAt := time.Now().UTC()
	if err != nil {
		events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
			ID:         mustGenerateMultiAgentEventID(),
			RunID:      runID,
			TaskID:     req.TaskID,
			Type:       events.EventPlannerFinished,
			Source:     "multiagent",
			Status:     "failed",
			Message:    "Planner failed",
			StartedAt:  plannerStartedAt,
			FinishedAt: plannerFinishedAt,
			DurationMs: plannerFinishedAt.Sub(plannerStartedAt).Milliseconds(),
			Error:      err.Error(),
		})
		result.State = MultiAgentStateFailed
		result.Error = fmt.Sprintf("planner failed: %v", err)
		result.FinishedAt = plannerFinishedAt
		rt.saveCheckpointOrFail(ctx, &result, "planner_failed")
		return result, nil
	}

	events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
		ID:         mustGenerateMultiAgentEventID(),
		RunID:      runID,
		TaskID:     req.TaskID,
		Type:       events.EventPlannerFinished,
		Source:     "multiagent",
		Status:     "succeeded",
		Message:    "Plan generated",
		StartedAt:  plannerStartedAt,
		FinishedAt: plannerFinishedAt,
		DurationMs: plannerFinishedAt.Sub(plannerStartedAt).Milliseconds(),
		Metadata:   map[string]string{"plan_steps": fmt.Sprintf("%d", len(planResult.Plan.Steps)), "risk_level": planResult.Plan.RiskLevel},
	})

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
		coderStartedAt := time.Now().UTC()
		events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
			ID:         mustGenerateMultiAgentEventID(),
			RunID:      runID,
			TaskID:     req.TaskID,
			WorktreeID: worktreeID,
			Type:       events.EventCoderStarted,
			Source:     "multiagent",
			Status:     "started",
			Message:    fmt.Sprintf("Coder started (iteration %d)", iterIndex),
			StartedAt:  coderStartedAt,
			Metadata:   map[string]string{"iteration": fmt.Sprintf("%d", iterIndex)},
		})

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
		coderFinishedAt := time.Now().UTC()
		if err != nil {
			events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
				ID:         mustGenerateMultiAgentEventID(),
				RunID:      runID,
				TaskID:     req.TaskID,
				WorktreeID: worktreeID,
				Type:       events.EventCoderFinished,
				Source:     "multiagent",
				Status:     "failed",
				Message:    fmt.Sprintf("Coder failed (iteration %d)", iterIndex),
				StartedAt:  coderStartedAt,
				FinishedAt: coderFinishedAt,
				DurationMs: coderFinishedAt.Sub(coderStartedAt).Milliseconds(),
				Error:      err.Error(),
				Metadata:   map[string]string{"iteration": fmt.Sprintf("%d", iterIndex)},
			})
			result.State = MultiAgentStateFailed
			result.Error = fmt.Sprintf("coder failed at iteration %d: %v", iterIndex, err)
			result.FinishedAt = coderFinishedAt
			rt.saveCheckpointOrFail(ctx, &result, "coder_failed")
			return result, nil
		}

		worktreeID = coderResult.WorktreeID
		result.WorktreeID = worktreeID
		sharedCtx.WorktreeID = worktreeID
		sharedCtx.AddMessage(coderResult.Message)

		events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
			ID:         mustGenerateMultiAgentEventID(),
			RunID:      runID,
			TaskID:     req.TaskID,
			WorktreeID: worktreeID,
			Type:       events.EventCoderFinished,
			Source:     "multiagent",
			Status:     "succeeded",
			Message:    fmt.Sprintf("Coding finished (iteration %d)", iterIndex),
			StartedAt:  coderStartedAt,
			FinishedAt: coderFinishedAt,
			DurationMs: coderFinishedAt.Sub(coderStartedAt).Milliseconds(),
			Metadata:   map[string]string{"iteration": fmt.Sprintf("%d", iterIndex), "worktree_id": worktreeID},
		})

		if err := rt.saveCheckpoint(ctx, result, "coder_done"); err != nil {
			result.State = MultiAgentStateFailed
			result.Error = fmt.Sprintf("iteration %d coder checkpoint failed: %v", iterIndex, err)
			result.FinishedAt = time.Now().UTC()
			return result, nil
		}

		// === Reviewer ===
		reviewerStartedAt := time.Now().UTC()
		events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
			ID:         mustGenerateMultiAgentEventID(),
			RunID:      runID,
			TaskID:     req.TaskID,
			WorktreeID: worktreeID,
			Type:       events.EventReviewerStarted,
			Source:     "multiagent",
			Status:     "started",
			Message:    fmt.Sprintf("Reviewer started (iteration %d)", iterIndex),
			StartedAt:  reviewerStartedAt,
			Metadata:   map[string]string{"iteration": fmt.Sprintf("%d", iterIndex)},
		})

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
		reviewerFinishedAt := time.Now().UTC()
		if err != nil {
			events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
				ID:         mustGenerateMultiAgentEventID(),
				RunID:      runID,
				TaskID:     req.TaskID,
				WorktreeID: worktreeID,
				Type:       events.EventReviewerFinished,
				Source:     "multiagent",
				Status:     "failed",
				Message:    fmt.Sprintf("Reviewer failed (iteration %d)", iterIndex),
				StartedAt:  reviewerStartedAt,
				FinishedAt: reviewerFinishedAt,
				DurationMs: reviewerFinishedAt.Sub(reviewerStartedAt).Milliseconds(),
				Error:      err.Error(),
				Metadata:   map[string]string{"iteration": fmt.Sprintf("%d", iterIndex)},
			})
			result.State = MultiAgentStateFailed
			result.Error = fmt.Sprintf("reviewer failed at iteration %d: %v", iterIndex, err)
			result.FinishedAt = reviewerFinishedAt
			rt.saveCheckpointOrFail(ctx, &result, "reviewer_failed")
			return result, nil
		}

		events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
			ID:         mustGenerateMultiAgentEventID(),
			RunID:      runID,
			TaskID:     req.TaskID,
			WorktreeID: worktreeID,
			Type:       events.EventReviewerFinished,
			Source:     "multiagent",
			Status:     "succeeded",
			Message:    fmt.Sprintf("Review completed (iteration %d)", iterIndex),
			StartedAt:  reviewerStartedAt,
			FinishedAt: reviewerFinishedAt,
			DurationMs: reviewerFinishedAt.Sub(reviewerStartedAt).Milliseconds(),
			Metadata: map[string]string{
				"iteration":      fmt.Sprintf("%d", iterIndex),
				"recommendation": string(reviewResult.Recommendation),
				"risk_level":     reviewResult.Report.RiskScore.Level,
			},
		})

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
			rt.emitTerminalEvent(ctx, result, events.EventRunSucceeded, "succeeded", "Multi-agent run succeeded")
			rt.saveCheckpointOrFail(ctx, &result, "loop_end")
			return result, nil

		case review.RecommendationReject:
			result.State = MultiAgentStateRejected
			result.FinalRecommendation = review.RecommendationReject
			result.FinalMessage = reviewResult.Message.Content
			result.FinishedAt = time.Now().UTC()
			rt.emitTerminalEvent(ctx, result, events.EventRunFailed, "failed", "Multi-agent run rejected")
			rt.saveCheckpointOrFail(ctx, &result, "loop_end")
			return result, nil

		case review.RecommendationRequestChanges:
			// If we've exhausted iterations, stop with request_changes state
			if iterIndex >= maxIter-1 {
				result.State = MultiAgentStateRequestChanges
				result.FinalRecommendation = review.RecommendationRequestChanges
				result.FinalMessage = fmt.Sprintf("Max iterations (%d) reached with request_changes recommendation", maxIter)
				result.FinishedAt = time.Now().UTC()
				rt.emitTerminalEvent(ctx, result, events.EventRunFailed, "failed", result.FinalMessage)
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
	rt.emitTerminalEvent(ctx, result, events.EventRunFailed, "failed", result.Error)
	rt.saveCheckpointOrFail(ctx, &result, "loop_end")
	return result, nil
}

// emitTerminalEvent emits a terminal event (run.succeeded/failed/cancelled) for the multi-agent run.
func (rt *DefaultMultiAgentRuntime) emitTerminalEvent(ctx context.Context, result MultiAgentRunResult, eventType events.EventType, status string, message string) {
	events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
		ID:         mustGenerateMultiAgentEventID(),
		RunID:      result.RunID,
		TaskID:     result.TaskID,
		WorktreeID: result.WorktreeID,
		Type:       eventType,
		Source:     "multiagent",
		Status:     status,
		Message:    message,
		StartedAt:  result.StartedAt,
		FinishedAt: result.FinishedAt,
		DurationMs: result.FinishedAt.Sub(result.StartedAt).Milliseconds(),
		Error:      result.Error,
		Metadata:   map[string]string{"iterations": fmt.Sprintf("%d", len(result.Iterations))},
	})
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

// mustGenerateMultiAgentEventID generates a unique event ID for multi-agent events.
func mustGenerateMultiAgentEventID() string {
	id, err := events.GenerateEventID()
	if err != nil {
		return "evt_error"
	}
	return id
}
