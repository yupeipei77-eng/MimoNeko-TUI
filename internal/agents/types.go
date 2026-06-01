// Package agents provides the Multi-Agent Workflow Skeleton for MimoNeko.
//
// This package implements a skeleton layer for multi-agent workflows.
// It does NOT:
//   - Call real LLMs
//   - Modify business files
//   - Apply patches
//   - Execute real code
//
// The skeleton is designed for CLI observability and event emission.
package agents

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// AgentRole identifies the role of an agent in the workflow.
type AgentRole string

const (
	AgentRolePlanner   AgentRole = "planner"
	AgentRoleCoder     AgentRole = "coder"
	AgentRoleReviewer  AgentRole = "reviewer"
	AgentRoleValidator AgentRole = "validator"
)

// AllAgentRoles returns all available agent roles.
func AllAgentRoles() []AgentRole {
	return []AgentRole{
		AgentRolePlanner,
		AgentRoleCoder,
		AgentRoleReviewer,
		AgentRoleValidator,
	}
}

// RoleDescription returns a human-readable description of the role.
func RoleDescription(role AgentRole) string {
	switch role {
	case AgentRolePlanner:
		return "Decomposes user goal into actionable plan steps"
	case AgentRoleCoder:
		return "Generates patch intent based on plan (skeleton: no real patch)"
	case AgentRoleReviewer:
		return "Reviews patch intent for quality and safety (skeleton: no real diff)"
	case AgentRoleValidator:
		return "Validates review output (skeleton: no real tests)"
	default:
		return "Unknown role"
	}
}

// AgentStatus represents the status of an agent step or workflow.
type AgentStatus string

const (
	AgentStatusPending       AgentStatus = "pending"
	AgentStatusRunning       AgentStatus = "running"
	AgentStatusCompleted     AgentStatus = "completed"
	AgentStatusFailed        AgentStatus = "failed"
	AgentStatusSkipped       AgentStatus = "skipped"
	AgentStatusCompletedStub AgentStatus = "completed_stub"
)

// IsTerminal returns true if the status is a terminal state.
func (s AgentStatus) IsTerminal() bool {
	switch s {
	case AgentStatusCompleted, AgentStatusFailed, AgentStatusSkipped, AgentStatusCompletedStub:
		return true
	default:
		return false
	}
}

// AgentStep represents a single step in a workflow.
type AgentStep struct {
	ID            string      `json:"id"`
	RunID         string      `json:"run_id"`
	Role          AgentRole   `json:"role"`
	Status        AgentStatus `json:"status"`
	InputSummary  string      `json:"input_summary"`
	OutputSummary string      `json:"output_summary"`
	StartedAt     time.Time   `json:"started_at,omitempty"`
	CompletedAt   time.Time   `json:"completed_at,omitempty"`
	ErrorMessage  string      `json:"error_message,omitempty"`
}

// NewAgentStep creates a new AgentStep.
func NewAgentStep(runID string, role AgentRole) AgentStep {
	return AgentStep{
		ID:     generateStepID(),
		RunID:  runID,
		Role:   role,
		Status: AgentStatusPending,
	}
}

// AgentWorkflow represents a complete multi-agent workflow.
type AgentWorkflow struct {
	ID          string      `json:"id"`
	RunID       string      `json:"run_id"`
	Goal        string      `json:"goal"`
	Steps       []AgentStep `json:"steps"`
	Status      AgentStatus `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	CompletedAt time.Time   `json:"completed_at,omitempty"`
}

// NewAgentWorkflow creates a new AgentWorkflow.
func NewAgentWorkflow(goal string) (*AgentWorkflow, error) {
	runID, err := generateRunID()
	if err != nil {
		return nil, err
	}

	workflow := &AgentWorkflow{
		ID:        generateWorkflowID(),
		RunID:     runID,
		Goal:      goal,
		Status:    AgentStatusPending,
		CreatedAt: time.Now().UTC(),
	}

	// Add steps for all roles
	for _, role := range AllAgentRoles() {
		workflow.Steps = append(workflow.Steps, NewAgentStep(runID, role))
	}

	return workflow, nil
}

// FindStep finds a step by role.
func (w *AgentWorkflow) FindStep(role AgentRole) (*AgentStep, bool) {
	for i := range w.Steps {
		if w.Steps[i].Role == role {
			return &w.Steps[i], true
		}
	}
	return nil, false
}

// IsComplete returns true if all steps are in terminal states.
func (w *AgentWorkflow) IsComplete() bool {
	for _, step := range w.Steps {
		if !step.Status.IsTerminal() {
			return false
		}
	}
	return true
}

// generateStepID creates a unique step identifier.
func generateStepID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("step_%d", time.Now().UnixNano())
	}
	return "step_" + hex.EncodeToString(b)
}

// generateWorkflowID creates a unique workflow identifier.
func generateWorkflowID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("wf_%d", time.Now().UnixNano())
	}
	return "wf_" + hex.EncodeToString(b)
}

// generateRunID creates a unique run identifier.
func generateRunID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("agents: generate run id: %w", err)
	}
	return "run_" + hex.EncodeToString(b), nil
}
