package agent

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/reasonforge/reasonforge/internal/contextengine"
	"github.com/reasonforge/reasonforge/internal/modelrouter"
	"github.com/reasonforge/reasonforge/internal/scratchpad"
	"github.com/reasonforge/reasonforge/internal/tools"
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

	// 2. Generate run ID and initialize result
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

	// 3. Save initial checkpoint
	rt.saveCheckpoint(ctx, result, req.Contract.ID)

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
		toolStep := rt.executeTool(ctx, req, toolCall, stepIndex)
		result.Steps = append(result.Steps, toolStep)

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
		rt.saveCheckpoint(ctx, result, req.Contract.ID)

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
	rt.saveCheckpoint(ctx, result, req.Contract.ID)

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

// saveCheckpoint persists the current run state.
func (rt *SingleAgentRuntime) saveCheckpoint(ctx context.Context, result AgentRunResult, contractID string) {
	if rt.deps.CheckpointStore == nil {
		return
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

	_ = rt.deps.CheckpointStore.Save(ctx, cp) // best-effort
}

func mustGenerateStepID() string {
	id, err := GenerateStepID()
	if err != nil {
		return "step_error"
	}
	return id
}
