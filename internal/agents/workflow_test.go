package agents

import (
	"testing"
)

func TestWorkflowManagerStart(t *testing.T) {
	mgr, err := NewWorkflowManager("test goal")
	if err != nil {
		t.Fatalf("NewWorkflowManager() error = %v", err)
	}

	// Start workflow
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if mgr.Workflow().Status != AgentStatusRunning {
		t.Errorf("Status = %q, want %q", mgr.Workflow().Status, AgentStatusRunning)
	}

	// Cannot start again
	if err := mgr.Start(); err == nil {
		t.Error("Start() should return error for already started workflow")
	}
}

func TestWorkflowManagerStartStep(t *testing.T) {
	mgr, _ := NewWorkflowManager("test goal")
	mgr.Start()

	// Start planner step
	step, err := mgr.StartStep(AgentRolePlanner)
	if err != nil {
		t.Fatalf("StartStep() error = %v", err)
	}
	if step.Status != AgentStatusRunning {
		t.Errorf("Step Status = %q, want %q", step.Status, AgentStatusRunning)
	}
	if step.StartedAt.IsZero() {
		t.Error("Step StartedAt should not be zero")
	}

	// Cannot start again
	_, err = mgr.StartStep(AgentRolePlanner)
	if err == nil {
		t.Error("StartStep() should return error for already running step")
	}
}

func TestWorkflowManagerCompleteStep(t *testing.T) {
	mgr, _ := NewWorkflowManager("test goal")
	mgr.Start()
	mgr.StartStep(AgentRolePlanner)

	// Complete planner step
	step, err := mgr.CompleteStep(AgentRolePlanner, "planner output")
	if err != nil {
		t.Fatalf("CompleteStep() error = %v", err)
	}
	if step.Status != AgentStatusCompleted {
		t.Errorf("Step Status = %q, want %q", step.Status, AgentStatusCompleted)
	}
	if step.OutputSummary != "planner output" {
		t.Errorf("Step OutputSummary = %q, want %q", step.OutputSummary, "planner output")
	}
	if step.CompletedAt.IsZero() {
		t.Error("Step CompletedAt should not be zero")
	}
}

func TestWorkflowManagerCompleteStepStub(t *testing.T) {
	mgr, _ := NewWorkflowManager("test goal")
	mgr.Start()
	mgr.StartStep(AgentRoleCoder)

	// Complete coder step with stub
	step, err := mgr.CompleteStepStub(AgentRoleCoder, "coder output")
	if err != nil {
		t.Fatalf("CompleteStepStub() error = %v", err)
	}
	if step.Status != AgentStatusCompletedStub {
		t.Errorf("Step Status = %q, want %q", step.Status, AgentStatusCompletedStub)
	}
	if step.OutputSummary != "stub: coder output" {
		t.Errorf("Step OutputSummary = %q, want %q", step.OutputSummary, "stub: coder output")
	}
}

func TestWorkflowManagerSkipStep(t *testing.T) {
	mgr, _ := NewWorkflowManager("test goal")
	mgr.Start()

	// Skip reviewer step
	step, err := mgr.SkipStep(AgentRoleReviewer, "not needed")
	if err != nil {
		t.Fatalf("SkipStep() error = %v", err)
	}
	if step.Status != AgentStatusSkipped {
		t.Errorf("Step Status = %q, want %q", step.Status, AgentStatusSkipped)
	}
	if step.OutputSummary != "skipped: not needed" {
		t.Errorf("Step OutputSummary = %q, want %q", step.OutputSummary, "skipped: not needed")
	}
}

func TestWorkflowManagerFailStep(t *testing.T) {
	mgr, _ := NewWorkflowManager("test goal")
	mgr.Start()
	mgr.StartStep(AgentRoleValidator)

	// Fail validator step
	step, err := mgr.FailStep(AgentRoleValidator, &testError{"test error"})
	if err != nil {
		t.Fatalf("FailStep() error = %v", err)
	}
	if step.Status != AgentStatusFailed {
		t.Errorf("Step Status = %q, want %q", step.Status, AgentStatusFailed)
	}
	if step.ErrorMessage != "test error" {
		t.Errorf("Step ErrorMessage = %q, want %q", step.ErrorMessage, "test error")
	}
}

func TestWorkflowManagerComplete(t *testing.T) {
	mgr, _ := NewWorkflowManager("test goal")
	mgr.Start()

	// Cannot complete with pending steps
	if err := mgr.Complete(); err == nil {
		t.Error("Complete() should return error with pending steps")
	}

	// Complete all steps
	mgr.StartStep(AgentRolePlanner)
	mgr.CompleteStep(AgentRolePlanner, "output")
	mgr.StartStep(AgentRoleCoder)
	mgr.CompleteStepStub(AgentRoleCoder, "output")
	mgr.StartStep(AgentRoleReviewer)
	mgr.CompleteStepStub(AgentRoleReviewer, "output")
	mgr.StartStep(AgentRoleValidator)
	mgr.CompleteStepStub(AgentRoleValidator, "output")

	// Now can complete
	if err := mgr.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if mgr.Workflow().Status != AgentStatusCompleted {
		t.Errorf("Status = %q, want %q", mgr.Workflow().Status, AgentStatusCompleted)
	}
}

func TestWorkflowManagerFail(t *testing.T) {
	mgr, _ := NewWorkflowManager("test goal")
	mgr.Start()

	mgr.Fail(&testError{"workflow error"})
	if mgr.Workflow().Status != AgentStatusFailed {
		t.Errorf("Status = %q, want %q", mgr.Workflow().Status, AgentStatusFailed)
	}
}

func TestRunWorkflowSkeleton(t *testing.T) {
	workflow, err := RunWorkflowSkeleton("test goal")
	if err != nil {
		t.Fatalf("RunWorkflowSkeleton() error = %v", err)
	}

	if workflow.Status != AgentStatusCompleted {
		t.Errorf("Status = %q, want %q", workflow.Status, AgentStatusCompleted)
	}
	if len(workflow.Steps) != 4 {
		t.Errorf("Steps length = %d, want 4", len(workflow.Steps))
	}

	// Planner should be completed
	planner, _ := workflow.FindStep(AgentRolePlanner)
	if planner.Status != AgentStatusCompleted {
		t.Errorf("Planner Status = %q, want %q", planner.Status, AgentStatusCompleted)
	}

	// Others should be completed_stub
	for _, role := range []AgentRole{AgentRoleCoder, AgentRoleReviewer, AgentRoleValidator} {
		step, _ := workflow.FindStep(role)
		if step.Status != AgentStatusCompletedStub {
			t.Errorf("%s Status = %q, want %q", role, step.Status, AgentStatusCompletedStub)
		}
	}
}

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
