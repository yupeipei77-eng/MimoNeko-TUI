package agents

import (
	"fmt"
	"strings"
	"time"
)

// WorkflowManager manages the lifecycle of an AgentWorkflow.
type WorkflowManager struct {
	workflow *AgentWorkflow
}

// NewWorkflowManager creates a new WorkflowManager.
func NewWorkflowManager(goal string) (*WorkflowManager, error) {
	workflow, err := NewAgentWorkflow(goal)
	if err != nil {
		return nil, err
	}
	return &WorkflowManager{workflow: workflow}, nil
}

// Workflow returns the underlying workflow.
func (m *WorkflowManager) Workflow() *AgentWorkflow {
	return m.workflow
}

// Start starts the workflow.
func (m *WorkflowManager) Start() error {
	if m.workflow.Status != AgentStatusPending {
		return fmt.Errorf("agents: workflow already started (status: %s)", m.workflow.Status)
	}
	m.workflow.Status = AgentStatusRunning
	return nil
}

// StartStep starts a step by role.
func (m *WorkflowManager) StartStep(role AgentRole) (*AgentStep, error) {
	if m.workflow.Status != AgentStatusRunning {
		return nil, fmt.Errorf("agents: workflow not running (status: %s)", m.workflow.Status)
	}

	step, ok := m.workflow.FindStep(role)
	if !ok {
		return nil, fmt.Errorf("agents: step not found for role %s", role)
	}

	if step.Status != AgentStatusPending {
		return nil, fmt.Errorf("agents: step %s already started (status: %s)", role, step.Status)
	}

	step.Status = AgentStatusRunning
	step.StartedAt = time.Now().UTC()
	return step, nil
}

// CompleteStep completes a step with output.
func (m *WorkflowManager) CompleteStep(role AgentRole, output string) (*AgentStep, error) {
	step, ok := m.workflow.FindStep(role)
	if !ok {
		return nil, fmt.Errorf("agents: step not found for role %s", role)
	}

	if step.Status != AgentStatusRunning {
		return nil, fmt.Errorf("agents: step %s not running (status: %s)", role, step.Status)
	}

	step.Status = AgentStatusCompleted
	step.OutputSummary = output
	step.CompletedAt = time.Now().UTC()
	return step, nil
}

// CompleteStepStub completes a step with stub output (skeleton phase).
func (m *WorkflowManager) CompleteStepStub(role AgentRole, output string) (*AgentStep, error) {
	step, ok := m.workflow.FindStep(role)
	if !ok {
		return nil, fmt.Errorf("agents: step not found for role %s", role)
	}

	if step.Status != AgentStatusRunning {
		return nil, fmt.Errorf("agents: step %s not running (status: %s)", role, step.Status)
	}

	step.Status = AgentStatusCompletedStub
	step.OutputSummary = "stub: " + output
	step.CompletedAt = time.Now().UTC()
	return step, nil
}

// SkipStep skips a step with reason.
func (m *WorkflowManager) SkipStep(role AgentRole, reason string) (*AgentStep, error) {
	step, ok := m.workflow.FindStep(role)
	if !ok {
		return nil, fmt.Errorf("agents: step not found for role %s", role)
	}

	if step.Status != AgentStatusPending {
		return nil, fmt.Errorf("agents: step %s already started (status: %s)", role, step.Status)
	}

	step.Status = AgentStatusSkipped
	step.OutputSummary = "skipped: " + reason
	step.CompletedAt = time.Now().UTC()
	return step, nil
}

// FailStep fails a step with error.
func (m *WorkflowManager) FailStep(role AgentRole, err error) (*AgentStep, error) {
	step, ok := m.workflow.FindStep(role)
	if !ok {
		return nil, fmt.Errorf("agents: step not found for role %s", role)
	}

	if step.Status != AgentStatusRunning {
		return nil, fmt.Errorf("agents: step %s not running (status: %s)", role, step.Status)
	}

	step.Status = AgentStatusFailed
	step.ErrorMessage = err.Error()
	step.CompletedAt = time.Now().UTC()
	return step, nil
}

// Complete completes the workflow.
func (m *WorkflowManager) Complete() error {
	if !m.workflow.IsComplete() {
		return fmt.Errorf("agents: workflow not complete (steps still pending/running)")
	}
	m.workflow.Status = AgentStatusCompleted
	m.workflow.CompletedAt = time.Now().UTC()
	return nil
}

// Fail fails the workflow.
func (m *WorkflowManager) Fail(err error) {
	m.workflow.Status = AgentStatusFailed
	m.workflow.CompletedAt = time.Now().UTC()
}

// Summary returns a human-readable summary of the workflow.
func (m *WorkflowManager) Summary() string {
	return FormatWorkflowSummary(m.workflow)
}

// FormatWorkflowSummary formats a workflow summary.
func FormatWorkflowSummary(w *AgentWorkflow) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "Workflow:\n")
	fmt.Fprintf(&buf, "  ID: %s\n", w.ID)
	fmt.Fprintf(&buf, "  Goal: %s\n", w.Goal)
	fmt.Fprintf(&buf, "  Status: %s\n", w.Status)
	fmt.Fprintf(&buf, "\n")

	fmt.Fprintf(&buf, "Steps:\n")
	for i, step := range w.Steps {
		fmt.Fprintf(&buf, "  %d. %-12s %s\n", i+1, step.Role, step.Status)
		if step.OutputSummary != "" {
			fmt.Fprintf(&buf, "     Output: %s\n", truncateString(step.OutputSummary, 80))
		}
		if step.ErrorMessage != "" {
			fmt.Fprintf(&buf, "     Error: %s\n", truncateString(step.ErrorMessage, 80))
		}
	}

	return buf.String()
}

// truncateString truncates a string to maxLen.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
