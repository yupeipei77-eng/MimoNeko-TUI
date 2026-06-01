package agents

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAgentRoleJSONSerialization(t *testing.T) {
	tests := []struct {
		name string
		role AgentRole
		want string
	}{
		{"planner", AgentRolePlanner, `"planner"`},
		{"coder", AgentRoleCoder, `"coder"`},
		{"reviewer", AgentRoleReviewer, `"reviewer"`},
		{"validator", AgentRoleValidator, `"validator"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.role)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(data) != tt.want {
				t.Errorf("json.Marshal() = %q, want %q", string(data), tt.want)
			}

			var got AgentRole
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if got != tt.role {
				t.Errorf("json.Unmarshal() = %q, want %q", got, tt.role)
			}
		})
	}
}

func TestAgentStatusJSONSerialization(t *testing.T) {
	tests := []struct {
		name   string
		status AgentStatus
		want   string
	}{
		{"pending", AgentStatusPending, `"pending"`},
		{"running", AgentStatusRunning, `"running"`},
		{"completed", AgentStatusCompleted, `"completed"`},
		{"failed", AgentStatusFailed, `"failed"`},
		{"skipped", AgentStatusSkipped, `"skipped"`},
		{"completed_stub", AgentStatusCompletedStub, `"completed_stub"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.status)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(data) != tt.want {
				t.Errorf("json.Marshal() = %q, want %q", string(data), tt.want)
			}

			var got AgentStatus
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if got != tt.status {
				t.Errorf("json.Unmarshal() = %q, want %q", got, tt.status)
			}
		})
	}
}

func TestAgentStepJSONSerialization(t *testing.T) {
	step := AgentStep{
		ID:            "step_test",
		RunID:         "run_test",
		Role:          AgentRolePlanner,
		Status:        AgentStatusCompleted,
		InputSummary:  "test input",
		OutputSummary: "test output",
		StartedAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		CompletedAt:   time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC),
	}

	data, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got AgentStep
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ID != step.ID {
		t.Errorf("ID = %q, want %q", got.ID, step.ID)
	}
	if got.Role != step.Role {
		t.Errorf("Role = %q, want %q", got.Role, step.Role)
	}
	if got.Status != step.Status {
		t.Errorf("Status = %q, want %q", got.Status, step.Status)
	}
}

func TestAgentWorkflowJSONSerialization(t *testing.T) {
	workflow, err := NewAgentWorkflow("test goal")
	if err != nil {
		t.Fatalf("NewAgentWorkflow() error = %v", err)
	}

	data, err := json.Marshal(workflow)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got AgentWorkflow
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ID != workflow.ID {
		t.Errorf("ID = %q, want %q", got.ID, workflow.ID)
	}
	if got.Goal != workflow.Goal {
		t.Errorf("Goal = %q, want %q", got.Goal, workflow.Goal)
	}
	if len(got.Steps) != len(workflow.Steps) {
		t.Errorf("Steps length = %d, want %d", len(got.Steps), len(workflow.Steps))
	}
}

func TestAgentStatusIsTerminal(t *testing.T) {
	tests := []struct {
		status AgentStatus
		want   bool
	}{
		{AgentStatusPending, false},
		{AgentStatusRunning, false},
		{AgentStatusCompleted, true},
		{AgentStatusFailed, true},
		{AgentStatusSkipped, true},
		{AgentStatusCompletedStub, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := tt.status.IsTerminal()
			if got != tt.want {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllAgentRoles(t *testing.T) {
	roles := AllAgentRoles()
	if len(roles) != 4 {
		t.Errorf("AllAgentRoles() returned %d roles, want 4", len(roles))
	}

	expected := []AgentRole{AgentRolePlanner, AgentRoleCoder, AgentRoleReviewer, AgentRoleValidator}
	for i, role := range roles {
		if role != expected[i] {
			t.Errorf("AllAgentRoles()[%d] = %q, want %q", i, role, expected[i])
		}
	}
}

func TestRoleDescription(t *testing.T) {
	roles := AllAgentRoles()
	for _, role := range roles {
		desc := RoleDescription(role)
		if desc == "" {
			t.Errorf("RoleDescription(%q) is empty", role)
		}
		if desc == "Unknown role" {
			t.Errorf("RoleDescription(%q) returned 'Unknown role'", role)
		}
	}
}

func TestNewAgentWorkflow(t *testing.T) {
	workflow, err := NewAgentWorkflow("test goal")
	if err != nil {
		t.Fatalf("NewAgentWorkflow() error = %v", err)
	}

	if workflow.ID == "" {
		t.Error("Workflow ID is empty")
	}
	if workflow.RunID == "" {
		t.Error("Workflow RunID is empty")
	}
	if workflow.Goal != "test goal" {
		t.Errorf("Workflow Goal = %q, want %q", workflow.Goal, "test goal")
	}
	if workflow.Status != AgentStatusPending {
		t.Errorf("Workflow Status = %q, want %q", workflow.Status, AgentStatusPending)
	}
	if len(workflow.Steps) != 4 {
		t.Errorf("Workflow Steps length = %d, want 4", len(workflow.Steps))
	}
}

func TestNewAgentStep(t *testing.T) {
	step := NewAgentStep("run_test", AgentRolePlanner)
	if step.ID == "" {
		t.Error("Step ID is empty")
	}
	if step.RunID != "run_test" {
		t.Errorf("Step RunID = %q, want %q", step.RunID, "run_test")
	}
	if step.Role != AgentRolePlanner {
		t.Errorf("Step Role = %q, want %q", step.Role, AgentRolePlanner)
	}
	if step.Status != AgentStatusPending {
		t.Errorf("Step Status = %q, want %q", step.Status, AgentStatusPending)
	}
}

func TestAgentWorkflowFindStep(t *testing.T) {
	workflow, _ := NewAgentWorkflow("test goal")

	// Find existing step
	step, ok := workflow.FindStep(AgentRolePlanner)
	if !ok {
		t.Error("FindStep(AgentRolePlanner) not found")
	}
	if step.Role != AgentRolePlanner {
		t.Errorf("Step Role = %q, want %q", step.Role, AgentRolePlanner)
	}

	// Find non-existing role
	_, ok = workflow.FindStep("nonexistent")
	if ok {
		t.Error("FindStep('nonexistent') should not be found")
	}
}

func TestAgentWorkflowIsComplete(t *testing.T) {
	workflow, _ := NewAgentWorkflow("test goal")

	// Not complete initially
	if workflow.IsComplete() {
		t.Error("Workflow should not be complete initially")
	}

	// Complete all steps
	for i := range workflow.Steps {
		workflow.Steps[i].Status = AgentStatusCompleted
	}

	if !workflow.IsComplete() {
		t.Error("Workflow should be complete after all steps completed")
	}
}
