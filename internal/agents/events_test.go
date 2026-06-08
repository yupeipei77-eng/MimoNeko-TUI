package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/events"
)

// mockEventEmitter captures emitted events for testing.
type mockEventEmitter struct {
	events []events.RunEvent
}

func (m *mockEventEmitter) Emit(ctx context.Context, event events.RunEvent) error {
	m.events = append(m.events, event)
	return nil
}

func TestWorkflowEventEmitterWorkflowStarted(t *testing.T) {
	mock := &mockEventEmitter{}
	emitter := NewWorkflowEventEmitter(mock)

	workflow, _ := NewAgentWorkflow("test goal")
	emitter.EmitWorkflowStarted(context.Background(), workflow)

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}

	evt := mock.events[0]
	if evt.Type != events.EventWorkflowStarted {
		t.Errorf("event type = %q, want %q", evt.Type, events.EventWorkflowStarted)
	}
	if evt.Source != "agents" {
		t.Errorf("source = %q, want %q", evt.Source, "agents")
	}
	if evt.RunID != workflow.RunID {
		t.Errorf("run_id = %q, want %q", evt.RunID, workflow.RunID)
	}
	if evt.Metadata["workflow_id"] != workflow.ID {
		t.Errorf("workflow_id = %q, want %q", evt.Metadata["workflow_id"], workflow.ID)
	}
	if evt.Metadata["goal"] != "test goal" {
		t.Errorf("goal = %q, want %q", evt.Metadata["goal"], "test goal")
	}
}

func TestWorkflowEventEmitterStepEvents(t *testing.T) {
	mock := &mockEventEmitter{}
	emitter := NewWorkflowEventEmitter(mock)

	workflow, _ := NewAgentWorkflow("test goal")

	// Emit step started
	emitter.EmitStepStarted(context.Background(), workflow, AgentRolePlanner)

	// Emit step completed
	step, _ := workflow.FindStep(AgentRolePlanner)
	step.Status = AgentStatusCompleted
	step.OutputSummary = "planner output"
	emitter.EmitStepCompleted(context.Background(), workflow, *step)

	if len(mock.events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(mock.events))
	}

	// Check step started
	if mock.events[0].Type != events.EventStepStarted {
		t.Errorf("event[0] type = %q, want %q", mock.events[0].Type, events.EventStepStarted)
	}
	if mock.events[0].Metadata["role"] != "planner" {
		t.Errorf("event[0] role = %q, want %q", mock.events[0].Metadata["role"], "planner")
	}

	// Check step completed
	if mock.events[1].Type != events.EventStepCompleted {
		t.Errorf("event[1] type = %q, want %q", mock.events[1].Type, events.EventStepCompleted)
	}
	if mock.events[1].Metadata["status"] != "completed" {
		t.Errorf("event[1] status = %q, want %q", mock.events[1].Metadata["status"], "completed")
	}
}

func TestWorkflowEventEmitterStepFailed(t *testing.T) {
	mock := &mockEventEmitter{}
	emitter := NewWorkflowEventEmitter(mock)

	workflow, _ := NewAgentWorkflow("test goal")
	step, _ := workflow.FindStep(AgentRoleCoder)
	step.Status = AgentStatusFailed

	emitter.EmitStepFailed(context.Background(), workflow, *step, &testError{"test error"})

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}

	evt := mock.events[0]
	if evt.Type != events.EventStepFailed {
		t.Errorf("event type = %q, want %q", evt.Type, events.EventStepFailed)
	}
	if evt.Error != "test error" {
		t.Errorf("error = %q, want %q", evt.Error, "test error")
	}
}

func TestWorkflowEventEmitterWorkflowCompleted(t *testing.T) {
	mock := &mockEventEmitter{}
	emitter := NewWorkflowEventEmitter(mock)

	workflow, _ := NewAgentWorkflow("test goal")
	workflow.Status = AgentStatusCompleted
	emitter.EmitWorkflowCompleted(context.Background(), workflow)

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}

	evt := mock.events[0]
	if evt.Type != events.EventWorkflowCompleted {
		t.Errorf("event type = %q, want %q", evt.Type, events.EventWorkflowCompleted)
	}
	if evt.Metadata["status"] != "completed" {
		t.Errorf("status = %q, want %q", evt.Metadata["status"], "completed")
	}
}

func TestWorkflowEventEmitterWorkflowFailed(t *testing.T) {
	mock := &mockEventEmitter{}
	emitter := NewWorkflowEventEmitter(mock)

	workflow, _ := NewAgentWorkflow("test goal")
	workflow.Status = AgentStatusFailed
	emitter.EmitWorkflowFailed(context.Background(), workflow, &testError{"workflow error"})

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}

	evt := mock.events[0]
	if evt.Type != events.EventWorkflowFailed {
		t.Errorf("event type = %q, want %q", evt.Type, events.EventWorkflowFailed)
	}
	if evt.Error != "workflow error" {
		t.Errorf("error = %q, want %q", evt.Error, "workflow error")
	}
}

func TestWorkflowEventEmitterRedactsGoal(t *testing.T) {
	mock := &mockEventEmitter{}
	emitter := NewWorkflowEventEmitter(mock)

	workflow, _ := NewAgentWorkflow("using API_KEY=sk-abcdefghijklmnopqrstuvwxyz")
	emitter.EmitWorkflowStarted(context.Background(), workflow)

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}

	goal := mock.events[0].Metadata["goal"]
	if strings.Contains(goal, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("goal leaked API key: %q", goal)
	}
}

func TestWorkflowEventEmitterRedactsOutputSummary(t *testing.T) {
	mock := &mockEventEmitter{}
	emitter := NewWorkflowEventEmitter(mock)

	workflow, _ := NewAgentWorkflow("test goal")
	step, _ := workflow.FindStep(AgentRolePlanner)
	step.OutputSummary = "using API_KEY=sk-abcdefghijklmnopqrstuvwxyz"
	step.Status = AgentStatusCompleted

	emitter.EmitStepCompleted(context.Background(), workflow, *step)

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}

	outputSummary := mock.events[0].Metadata["output_summary"]
	if strings.Contains(outputSummary, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("output_summary leaked API key: %q", outputSummary)
	}
}

func TestWorkflowEventEmitterRedactsErrorMessage(t *testing.T) {
	mock := &mockEventEmitter{}
	emitter := NewWorkflowEventEmitter(mock)

	workflow, _ := NewAgentWorkflow("test goal")
	step, _ := workflow.FindStep(AgentRoleCoder)
	step.Status = AgentStatusFailed

	emitter.EmitStepFailed(context.Background(), workflow, *step, &testError{"error with API_KEY=sk-abcdefghijklmnopqrstuvwxyz"})

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}

	errorMsg := mock.events[0].Error
	if strings.Contains(errorMsg, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("error leaked API key: %q", errorMsg)
	}
}

func TestWorkflowEventEmitterNilEmitter(t *testing.T) {
	emitter := NewWorkflowEventEmitter(nil)

	workflow, _ := NewAgentWorkflow("test goal")

	// Should not panic
	emitter.EmitWorkflowStarted(context.Background(), workflow)
	emitter.EmitStepStarted(context.Background(), workflow, AgentRolePlanner)
	emitter.EmitWorkflowCompleted(context.Background(), workflow)
}

func TestWorkflowEventEmitterEventOrder(t *testing.T) {
	mock := &mockEventEmitter{}
	emitter := NewWorkflowEventEmitter(mock)

	workflow, _ := NewAgentWorkflow("test goal")

	// Emit events in order
	emitter.EmitWorkflowStarted(context.Background(), workflow)

	for _, role := range AllAgentRoles() {
		emitter.EmitStepStarted(context.Background(), workflow, role)
		step, _ := workflow.FindStep(role)
		step.Status = AgentStatusCompleted
		emitter.EmitStepCompleted(context.Background(), workflow, *step)
	}

	emitter.EmitWorkflowCompleted(context.Background(), workflow)

	// Expected: 1 workflow_started + 4 step_started + 4 step_completed + 1 workflow_completed = 10
	if len(mock.events) != 10 {
		t.Fatalf("expected 10 events, got %d", len(mock.events))
	}

	// Check order
	expectedTypes := []events.EventType{
		events.EventWorkflowStarted,
		events.EventStepStarted,
		events.EventStepCompleted,
		events.EventStepStarted,
		events.EventStepCompleted,
		events.EventStepStarted,
		events.EventStepCompleted,
		events.EventStepStarted,
		events.EventStepCompleted,
		events.EventWorkflowCompleted,
	}

	for i, expected := range expectedTypes {
		if mock.events[i].Type != expected {
			t.Errorf("event[%d] type = %q, want %q", i, mock.events[i].Type, expected)
		}
	}
}
