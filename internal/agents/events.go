package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/events"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"
)

// WorkflowEventEmitter wraps an EventEmitter for agent workflow events.
// It handles redaction and stable metadata ordering.
type WorkflowEventEmitter struct {
	emitter events.EventEmitter
}

// NewWorkflowEventEmitter creates a new WorkflowEventEmitter.
// If emitter is nil, all emit calls are no-ops.
func NewWorkflowEventEmitter(emitter events.EventEmitter) *WorkflowEventEmitter {
	if emitter == nil {
		emitter = &events.NoopEventEmitter{}
	}
	return &WorkflowEventEmitter{emitter: emitter}
}

// EmitWorkflowStarted emits a workflow started event.
func (e *WorkflowEventEmitter) EmitWorkflowStarted(ctx context.Context, workflow *AgentWorkflow) {
	goal := security.SanitizeText(workflow.Goal)
	events.SafeEmit(e.emitter, ctx, events.RunEvent{
		ID:        mustGenerateEventID(),
		RunID:     workflow.RunID,
		Timestamp: time.Now().UTC(),
		Type:      events.EventWorkflowStarted,
		Source:    "agents",
		Status:    "started",
		Message:   fmt.Sprintf("Workflow started: %s", goal),
		Metadata: map[string]string{
			"workflow_id": workflow.ID,
			"goal":        goal,
			"status":      string(workflow.Status),
		},
	})
}

// EmitStepStarted emits a step started event.
func (e *WorkflowEventEmitter) EmitStepStarted(ctx context.Context, workflow *AgentWorkflow, role AgentRole) {
	goal := security.SanitizeText(workflow.Goal)
	events.SafeEmit(e.emitter, ctx, events.RunEvent{
		ID:        mustGenerateEventID(),
		RunID:     workflow.RunID,
		Timestamp: time.Now().UTC(),
		Type:      events.EventStepStarted,
		Source:    "agents",
		Status:    "started",
		Message:   fmt.Sprintf("Step started: %s", role),
		Metadata: map[string]string{
			"workflow_id": workflow.ID,
			"role":        string(role),
			"goal":        goal,
		},
	})
}

// EmitStepCompleted emits a step completed event.
func (e *WorkflowEventEmitter) EmitStepCompleted(ctx context.Context, workflow *AgentWorkflow, step AgentStep) {
	inputSummary := security.SanitizeText(step.InputSummary)
	outputSummary := security.SanitizeText(step.OutputSummary)
	events.SafeEmit(e.emitter, ctx, events.RunEvent{
		ID:        mustGenerateEventID(),
		RunID:     workflow.RunID,
		Timestamp: time.Now().UTC(),
		Type:      events.EventStepCompleted,
		Source:    "agents",
		Status:    string(step.Status),
		Message:   fmt.Sprintf("Step completed: %s", step.Role),
		Metadata: map[string]string{
			"workflow_id":    workflow.ID,
			"role":           string(step.Role),
			"status":         string(step.Status),
			"input_summary":  inputSummary,
			"output_summary": outputSummary,
		},
	})
}

// EmitStepFailed emits a step failed event.
func (e *WorkflowEventEmitter) EmitStepFailed(ctx context.Context, workflow *AgentWorkflow, step AgentStep, err error) {
	errorMessage := security.SanitizeText(err.Error())
	events.SafeEmit(e.emitter, ctx, events.RunEvent{
		ID:        mustGenerateEventID(),
		RunID:     workflow.RunID,
		Timestamp: time.Now().UTC(),
		Type:      events.EventStepFailed,
		Source:    "agents",
		Status:    "failed",
		Message:   fmt.Sprintf("Step failed: %s", step.Role),
		Error:     errorMessage,
		Metadata: map[string]string{
			"workflow_id":   workflow.ID,
			"role":          string(step.Role),
			"error_message": errorMessage,
		},
	})
}

// EmitWorkflowCompleted emits a workflow completed event.
func (e *WorkflowEventEmitter) EmitWorkflowCompleted(ctx context.Context, workflow *AgentWorkflow) {
	goal := security.SanitizeText(workflow.Goal)
	events.SafeEmit(e.emitter, ctx, events.RunEvent{
		ID:        mustGenerateEventID(),
		RunID:     workflow.RunID,
		Timestamp: time.Now().UTC(),
		Type:      events.EventWorkflowCompleted,
		Source:    "agents",
		Status:    string(workflow.Status),
		Message:   fmt.Sprintf("Workflow completed: %s", goal),
		Metadata: map[string]string{
			"workflow_id": workflow.ID,
			"goal":        goal,
			"status":      string(workflow.Status),
		},
	})
}

// EmitWorkflowFailed emits a workflow failed event.
func (e *WorkflowEventEmitter) EmitWorkflowFailed(ctx context.Context, workflow *AgentWorkflow, err error) {
	goal := security.SanitizeText(workflow.Goal)
	errorMessage := security.SanitizeText(err.Error())
	events.SafeEmit(e.emitter, ctx, events.RunEvent{
		ID:        mustGenerateEventID(),
		RunID:     workflow.RunID,
		Timestamp: time.Now().UTC(),
		Type:      events.EventWorkflowFailed,
		Source:    "agents",
		Status:    "failed",
		Message:   fmt.Sprintf("Workflow failed: %s", goal),
		Error:     errorMessage,
		Metadata: map[string]string{
			"workflow_id":   workflow.ID,
			"goal":          goal,
			"error_message": errorMessage,
		},
	})
}

// mustGenerateEventID generates an event ID or returns a fallback.
func mustGenerateEventID() string {
	id, err := events.GenerateEventID()
	if err != nil {
		return "evt_error"
	}
	return id
}
