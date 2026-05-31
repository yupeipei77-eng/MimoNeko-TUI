package agent

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mimoneko/mimoneko/internal/contextengine"
	"github.com/mimoneko/mimoneko/internal/events"
	"github.com/mimoneko/mimoneko/internal/modelrouter"
	"github.com/mimoneko/mimoneko/internal/patch"
	"github.com/mimoneko/mimoneko/internal/scratchpad"
	"github.com/mimoneko/mimoneko/internal/tools"
	"github.com/mimoneko/mimoneko/internal/worktree"
)

// SingleAgentRuntime implements AgentRuntime with a single-agent loop.
//
// The loop follows this cycle:
//  1. Validate TaskContract
//  2. Build context via ContextEngine.Build
//  3. Call model via ModelRouter.Complete
//  4. Parse tool call from model output
//  5. Check contract allows the tool
//  6. Execute tool via ToolRuntime.Run
//  7. Convert ToolResponse -> ScratchpadItem and inject
//  8. Save checkpoint
//  9. Loop until max_steps, no tool call, or error
type SingleAgentRuntime struct {
	deps           Dependencies
	approvalPolicy ApprovalPolicy
	stdout         io.Writer
}

// NewSingleAgentRuntime creates a SingleAgentRuntime with the given dependencies.
func NewSingleAgentRuntime(deps Dependencies) *SingleAgentRuntime {
	return &SingleAgentRuntime{
		deps:           deps,
		approvalPolicy: DefaultApprovalPolicy(),
		stdout:         io.Discard,
	}
}

// SetApprovalPolicy configures the approval policy for the runtime.
func (rt *SingleAgentRuntime) SetApprovalPolicy(policy ApprovalPolicy) {
	rt.approvalPolicy = policy
}

// SetOutput sets the stdout writer for interactive approval prompts.
func (rt *SingleAgentRuntime) SetOutput(w io.Writer) {
	if w != nil {
		rt.stdout = w
	}
}

// Run executes the agent loop for the given request.
func (rt *SingleAgentRuntime) Run(ctx context.Context, req AgentRunRequest) (AgentRunResult, error) {
	// 1. Validate the contract
	if err := req.Contract.Validate(); err != nil {
		return AgentRunResult{}, fmt.Errorf("agent: invalid task contract: %w", err)
	}

	// Use contract's MaxSteps if request doesn't override
	maxSteps := req.MaxSteps
	if maxSteps <= 0 {
		maxSteps = req.Contract.MaxSteps
	}
	if maxSteps <= 0 {
		maxSteps = 5 // safety fallback
	}

	// 2. Handle worktree isolation
	var worktreeInfo *worktree.WorktreeInfo
	var originalRepoRoot string

	if req.UseWorktree {
		if rt.deps.WorktreeMgr == nil {
			return AgentRunResult{}, fmt.Errorf("agent: worktree manager not configured but UseWorktree=true")
		}

		if req.WorktreeID != "" {
			// Use existing worktree
			info, err := rt.deps.WorktreeMgr.Get(ctx, req.WorktreeID)
			if err != nil {
				return AgentRunResult{}, fmt.Errorf("agent: get worktree %q: %w", req.WorktreeID, err)
			}
			worktreeInfo = &info
		} else {
			// Create new worktree
			info, err := rt.deps.WorktreeMgr.Create(ctx, worktree.CreateWorktreeRequest{
				RepoRoot: req.RepoRoot,
				TaskID:   req.TaskID,
				BaseRef:  "HEAD",
				Metadata: map[string]string{"goal": req.Goal, "source": "agent_runtime"},
			})
			if err != nil {
				return AgentRunResult{}, fmt.Errorf("agent: create worktree: %w", err)
			}
			worktreeInfo = &info
		}

		// Redirect RepoRoot to the worktree path
		originalRepoRoot = req.RepoRoot
		req.RepoRoot = worktreeInfo.Path
	}

	// 3. Generate run ID and initialize result
	runID, err := GenerateRunID()
	if err != nil {
		return AgentRunResult{}, fmt.Errorf("agent: generate run id: %w", err)
	}

	result := AgentRunResult{
		RunID:     runID,
		TaskID:    req.TaskID,
		State:     AgentStateRunning,
		Steps:     []AgentStep{},
		StartedAt: time.Now().UTC(),
	}

	if worktreeInfo != nil {
		result.WorktreeID = worktreeInfo.ID
	}

	// 4. Save initial checkpoint
	if err := rt.saveCheckpoint(ctx, result, req.Contract.ID); err != nil {
		return AgentRunResult{}, fmt.Errorf("agent: initial checkpoint failed: %w", err)
	}

	// Emit run.started event
	events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
		ID:         mustGenerateEventID(),
		RunID:      runID,
		TaskID:     req.TaskID,
		WorktreeID: result.WorktreeID,
		Type:       events.EventRunStarted,
		Source:     "agent",
		Status:     "started",
		Message:    "Agent run started",
		StartedAt:  result.StartedAt,
		Metadata:   map[string]string{"goal": req.Goal, "dry_run": fmt.Sprintf("%v", req.DryRun)},
	})

	// Wrap ctx with RunContext so downstream components (ToolRuntime, PatchManager, etc.)
	// can emit events with correct RunID/TaskID/WorktreeID.
	rc := events.RunContext{
		RunID:      runID,
		TaskID:     req.TaskID,
		WorktreeID: result.WorktreeID,
	}
	ctx = events.WithRunContext(ctx, rc)

	// 4. Agent loop
	toolCallCount := 0
	currentInput := []byte(req.Goal)

	for stepIndex := 0; stepIndex < maxSteps; stepIndex++ {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			result.State = AgentStateCancelled
			result.Error = fmt.Sprintf("context cancelled: %v", err)
			break
		}

		// 4a. Build context
		modelStep, _, bundleErr := rt.buildContextAndCallModel(ctx, req, currentInput, stepIndex)

		if bundleErr != nil {
			modelStep.State = AgentStateFailed
			modelStep.Error = bundleErr.Error()
			modelStep.FinishedAt = time.Now().UTC()
			result.Steps = append(result.Steps, modelStep)
			result.State = AgentStateFailed
			result.Error = fmt.Sprintf("model call failed at step %d: %v", stepIndex, bundleErr)
			break
		}

		modelStep.State = AgentStateSucceeded
		modelStep.FinishedAt = time.Now().UTC()
		result.Steps = append(result.Steps, modelStep)

		// 4b. Parse tool call from model output
		toolCall, parseErr := ParseToolCall(modelStep.ModelText)
		if parseErr != nil {
			// Log parse error but don't fail the run
			modelStep.Error = fmt.Sprintf("tool_call parse warning: %v", parseErr)
		}

		if toolCall == nil {
			// No tool call -> model has finished, treat as success
			result.State = AgentStateSucceeded
			result.FinalMessage = modelStep.ModelText
			break
		}

		// 4c. Check contract allows this tool
		if !req.Contract.IsToolAllowed(toolCall.Name) {
			toolStep := AgentStep{
				StepID:     mustGenerateStepID(),
				Index:      stepIndex,
				Type:       "tool",
				State:      AgentStateFailed,
				ToolCall:   toolCall,
				Error:      fmt.Sprintf("tool %q denied by task contract", toolCall.Name),
				StartedAt:  time.Now().UTC(),
				FinishedAt: time.Now().UTC(),
			}
			result.Steps = append(result.Steps, toolStep)
			result.State = AgentStateFailed
			result.Error = fmt.Sprintf("tool %q denied by task contract at step %d", toolCall.Name, stepIndex)
			break
		}

		// Check MaxToolCalls
		toolCallCount++
		if req.Contract.MaxToolCalls > 0 && toolCallCount > req.Contract.MaxToolCalls {
			toolStep := AgentStep{
				StepID:     mustGenerateStepID(),
				Index:      stepIndex,
				Type:       "tool",
				State:      AgentStateFailed,
				ToolCall:   toolCall,
				Error:      fmt.Sprintf("max_tool_calls (%d) exceeded", req.Contract.MaxToolCalls),
				StartedAt:  time.Now().UTC(),
				FinishedAt: time.Now().UTC(),
			}
			result.Steps = append(result.Steps, toolStep)
			result.State = AgentStateFailed
			result.Error = fmt.Sprintf("max_tool_calls (%d) exceeded at step %d", req.Contract.MaxToolCalls, stepIndex)
			break
		}

		// Check tool risk with ApprovalPolicy
		tool, toolFound := rt.deps.ToolRegistry.Get(toolCall.Name)
		if !toolFound {
			toolStep := AgentStep{
				StepID:     mustGenerateStepID(),
				Index:      stepIndex,
				Type:       "tool",
				State:      AgentStateFailed,
				ToolCall:   toolCall,
				Error:      fmt.Sprintf("tool %q not found in registry", toolCall.Name),
				StartedAt:  time.Now().UTC(),
				FinishedAt: time.Now().UTC(),
			}
			result.Steps = append(result.Steps, toolStep)
			result.State = AgentStateFailed
			result.Error = fmt.Sprintf("tool %q not found at step %d", toolCall.Name, stepIndex)
			break
		}

		// Evaluate approval policy
		riskLevel := tool.RiskLevel()
		decision := rt.approvalPolicy.Check(toolCall.Name, riskLevel, func() bool {
			return req.Contract.IsToolAllowed(toolCall.Name)
		})

		shouldBreak := false
		switch decision {
		case ApprovalDenied:
			toolStep := AgentStep{
				StepID:     mustGenerateStepID(),
				Index:      stepIndex,
				Type:       "tool",
				State:      AgentStateFailed,
				ToolCall:   toolCall,
				Error:      fmt.Sprintf("tool %q denied by approval policy (risk=%s)", toolCall.Name, riskLevel),
				StartedAt:  time.Now().UTC(),
				FinishedAt: time.Now().UTC(),
			}
			result.Steps = append(result.Steps, toolStep)
			result.State = AgentStateFailed
			result.Error = fmt.Sprintf("tool %q denied by approval policy at step %d", toolCall.Name, stepIndex)
			shouldBreak = true

		case ApprovalRequiresApproval:
			// Try interactive approval if available
			if rt.approvalPolicy.RequestInteractiveApproval(rt.stdout, toolCall.Name, riskLevel, toolCall.Args) {
				// Approved interactively, fall through to execute
			} else {
				toolStep := AgentStep{
					StepID:     mustGenerateStepID(),
					Index:      stepIndex,
					Type:       "tool",
					State:      AgentStateWaitingApproval,
					ToolCall:   toolCall,
					Error:      fmt.Sprintf("tool %q (risk=%s) requires approval", toolCall.Name, riskLevel),
					StartedAt:  time.Now().UTC(),
					FinishedAt: time.Now().UTC(),
				}
				result.Steps = append(result.Steps, toolStep)
				result.State = AgentStateWaitingApproval
				result.Error = fmt.Sprintf("tool %q requires approval at step %d", toolCall.Name, stepIndex)
				shouldBreak = true
			}

		case ApprovalAutoApproved:
			// Proceed to execute
		}

		if shouldBreak {
			break
		}

		// 4d. Execute the tool
		// Note: tool.started/tool.finished events are now emitted by ToolRuntime.
		// The agent emits step-level events (step.started/step.finished) instead
		// to avoid duplicate tool events.
		events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
			ID:         mustGenerateEventID(),
			RunID:      runID,
			TaskID:     req.TaskID,
			WorktreeID: result.WorktreeID,
			StepID:     mustGenerateStepID(),
			ParentID:   runID,
			Type:       events.EventCoderStarted,
			Source:     "agent",
			Status:     "started",
			Message:    fmt.Sprintf("Agent step %d: tool %s", stepIndex, toolCall.Name),
			StartedAt:  time.Now().UTC(),
			Metadata:   map[string]string{"tool_name": toolCall.Name, "step_index": fmt.Sprintf("%d", stepIndex)},
		})

		toolStep := rt.executeTool(ctx, req, toolCall, stepIndex)
		result.Steps = append(result.Steps, toolStep)

		// Emit step finished event
		stepStatus := "succeeded"
		stepError := ""
		if toolStep.State == AgentStateFailed {
			stepStatus = "failed"
			stepError = toolStep.Error
		}
		events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
			ID:         mustGenerateEventID(),
			RunID:      runID,
			TaskID:     req.TaskID,
			WorktreeID: result.WorktreeID,
			StepID:     toolStep.StepID,
			ParentID:   runID,
			Type:       events.EventCoderFinished,
			Source:     "agent",
			Status:     stepStatus,
			Message:    fmt.Sprintf("Agent step %d completed: tool %s", stepIndex, toolCall.Name),
			StartedAt:  toolStep.StartedAt,
			FinishedAt: toolStep.FinishedAt,
			DurationMs: toolStep.FinishedAt.Sub(toolStep.StartedAt).Milliseconds(),
			Error:      stepError,
			Metadata:   map[string]string{"tool_name": toolCall.Name, "step_index": fmt.Sprintf("%d", stepIndex)},
		})

		if !toolStep.State.IsTerminal() || toolStep.State == AgentStateFailed {
			if toolStep.State == AgentStateFailed {
				result.State = AgentStateFailed
				result.Error = fmt.Sprintf("tool %q failed at step %d: %s", toolCall.Name, stepIndex, toolStep.Error)
				break
			}
		}

		// 4e. Convert ToolResponse -> ScratchpadItem and inject
		if toolStep.ToolResponse != nil && toolStep.ToolResponse.Success {
			rt.injectToolResponse(ctx, req.TaskID, *toolStep.ToolResponse)
		}

		// 4f. Save checkpoint
		if err := rt.saveCheckpoint(ctx, result, req.Contract.ID); err != nil {
			result.State = AgentStateFailed
			result.Error = fmt.Sprintf("checkpoint failed at step %d: %v", stepIndex, err)
			break
		}

		// 4g. Prepare next iteration's input from tool output
		if toolStep.ToolResponse != nil {
			currentInput = []byte(toolStep.ToolResponse.Stdout)
		}
	}

	// If loop exhausted max_steps without reaching a terminal or waiting state
	if !result.State.IsTerminal() && result.State != AgentStateWaitingApproval {
		result.State = AgentStateFailed
		result.Error = fmt.Sprintf("max_steps (%d) reached without completion", maxSteps)
	}

	result.FinishedAt = time.Now().UTC()

	// Save final checkpoint
	if cpErr := rt.saveCheckpoint(ctx, result, req.Contract.ID); cpErr != nil {
		result.State = AgentStateFailed
		result.Error = fmt.Sprintf("final checkpoint failed: %v", cpErr)
	}

	// 6. Handle worktree post-run: generate PatchPreview, update state
	if worktreeInfo != nil {
		// Update worktree state based on agent result
		switch result.State {
		case AgentStateSucceeded:
			// Keep worktree active for user to review and apply
		case AgentStateFailed:
			if rt.deps.WorktreeMgr != nil {
				_ = rt.deps.WorktreeMgr.UpdateState(ctx, worktreeInfo.ID, worktree.WorktreeStateFailed)
			}
			// Keep worktree for debugging (don't auto-delete)
		case AgentStateCancelled:
			// Keep worktree for debugging (don't auto-delete)
		}

		// Generate PatchPreview if PatchMgr is available
		if rt.deps.PatchMgr != nil {
			repoRootForPreview := originalRepoRoot
			if repoRootForPreview == "" {
				repoRootForPreview = req.Contract.RepoRoot
			}
			preview, previewErr := rt.deps.PatchMgr.Preview(ctx, patch.PatchPreviewRequest{
				RepoRoot:   repoRootForPreview,
				WorktreeID: worktreeInfo.ID,
				Contract:   req.Contract,
			})
			if previewErr == nil {
				result.PatchPreview = &preview
			}
			// Preview error is non-fatal; the worktree still exists for manual review
		}
	}

	// Emit terminal event based on final state
	terminalEventType := events.EventRunFailed
	terminalStatus := "failed"
	terminalMessage := "Agent run failed"
	switch result.State {
	case AgentStateSucceeded:
		terminalEventType = events.EventRunSucceeded
		terminalStatus = "succeeded"
		terminalMessage = "Agent run succeeded"
	case AgentStateCancelled:
		terminalEventType = events.EventRunCancelled
		terminalStatus = "cancelled"
		terminalMessage = "Agent run cancelled"
	}
	events.SafeEmit(rt.deps.EventEmitter, ctx, events.RunEvent{
		ID:         mustGenerateEventID(),
		RunID:      runID,
		TaskID:     req.TaskID,
		WorktreeID: result.WorktreeID,
		Type:       terminalEventType,
		Source:     "agent",
		Status:     terminalStatus,
		Message:    terminalMessage,
		StartedAt:  result.StartedAt,
		FinishedAt: result.FinishedAt,
		DurationMs: result.FinishedAt.Sub(result.StartedAt).Milliseconds(),
		Error:      result.Error,
		Metadata:   map[string]string{"steps": fmt.Sprintf("%d", len(result.Steps))},
	})

	return result, nil
}

// buildContextAndCallModel performs one iteration of building context and calling the model.
func (rt *SingleAgentRuntime) buildContextAndCallModel(
	ctx context.Context,
	req AgentRunRequest,
	currentInput []byte,
	stepIndex int,
) (AgentStep, modelrouter.CompletionResponse, error) {
	stepID, _ := GenerateStepID()
	step := AgentStep{
		StepID:    stepID,
		Index:     stepIndex,
		Type:      "model",
		State:     AgentStateRunning,
		StartedAt: time.Now().UTC(),
	}

	// Build context bundle
	bundle, err := rt.deps.ContextEngine.Build(ctx, contextengine.BuildRequest{
		TaskID:         req.TaskID,
		ConversationID: req.ConversationID,
		RepoRoot:       req.RepoRoot,
		Budget: contextengine.TokenBudget{
			ImmutablePrefix: 100000,
			Conversation:    50000,
			Scratchpad:      30000,
			Output:          4096,
		},
		CurrentInput: currentInput,
	})
	if err != nil {
		return step, modelrouter.CompletionResponse{}, fmt.Errorf("context build: %w", err)
	}

	// Call model
	completionReq := modelrouter.CompletionRequest{
		Bundle:          bundle,
		MaxOutputTokens: 4096,
	}

	resp, err := rt.deps.ModelRouter.Complete(ctx, completionReq)
	if err != nil {
		return step, modelrouter.CompletionResponse{}, fmt.Errorf("model complete: %w", err)
	}

	step.ModelProvider = resp.Provider
	step.Model = resp.Model
	step.ModelText = resp.Text

	return step, resp, nil
}

// executeTool runs a tool through ToolRuntime.
func (rt *SingleAgentRuntime) executeTool(
	ctx context.Context,
	req AgentRunRequest,
	toolCall *ToolCall,
	stepIndex int,
) AgentStep {
	stepID, _ := GenerateStepID()
	step := AgentStep{
		StepID:    stepID,
		Index:     stepIndex,
		Type:      "tool",
		State:     AgentStateRunning,
		ToolCall:  toolCall,
		StartedAt: time.Now().UTC(),
	}

	toolReq := tools.ToolRequest{
		ToolName: toolCall.Name,
		RepoRoot: req.RepoRoot,
		TaskID:   req.TaskID,
		Args:     toolCall.Args,
		DryRun:   req.DryRun || req.Contract.DryRun,
		Metadata: map[string]string{"source": "agent_runtime", "run_id": stepID},
	}

	resp, err := rt.deps.ToolRuntime.Run(ctx, toolReq)
	if err != nil {
		step.State = AgentStateFailed
		step.Error = err.Error()
		step.FinishedAt = time.Now().UTC()
		return step
	}

	step.ToolResponse = &resp
	if resp.Success {
		step.State = AgentStateSucceeded
	} else {
		step.State = AgentStateFailed
		step.Error = resp.Error
	}
	step.FinishedAt = time.Now().UTC()
	return step
}

// injectToolResponse converts a ToolResponse to a ScratchpadItem and writes it.
func (rt *SingleAgentRuntime) injectToolResponse(ctx context.Context, taskID string, resp tools.ToolResponse) {
	if rt.deps.Scratchpad == nil {
		return
	}

	item := scratchpad.Item{
		TaskID:   taskID,
		Kind:     scratchpad.ItemKindToolOutput,
		Content:  []byte(resp.Stdout),
		Priority: 5, // default priority for tool output
		Metadata: map[string]string{
			"tool_name": resp.ToolName,
			"audit_id":  resp.AuditID,
		},
	}

	_ = rt.deps.Scratchpad.Put(ctx, item) // best-effort; don't fail the run
}

// saveCheckpoint persists the current run state after sanitization.
// It returns an error if the checkpoint store is unavailable or the write fails.
func (rt *SingleAgentRuntime) saveCheckpoint(ctx context.Context, result AgentRunResult, contractID string) error {
	if rt.deps.CheckpointStore == nil {
		return fmt.Errorf("agent: checkpoint store not configured")
	}

	cp := Checkpoint{
		RunID:      result.RunID,
		TaskID:     result.TaskID,
		State:      result.State,
		StepIndex:  len(result.Steps),
		Steps:      result.Steps,
		ContractID: contractID,
		CreatedAt:  time.Now().UTC(),
	}

	// Sanitize before persisting to prevent sensitive data leakage
	cp = SanitizeCheckpoint(cp)

	if err := rt.deps.CheckpointStore.Save(ctx, cp); err != nil {
		return fmt.Errorf("agent: checkpoint save failed: %w", err)
	}
	return nil
}

func mustGenerateStepID() string {
	id, err := GenerateStepID()
	if err != nil {
		return "step_error"
	}
	return id
}

func mustGenerateEventID() string {
	id, err := events.GenerateEventID()
	if err != nil {
		return "evt_error"
	}
	return id
}
