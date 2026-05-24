package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/reasonforge/reasonforge/internal/contextengine"
	"github.com/reasonforge/reasonforge/internal/conversation"
	"github.com/reasonforge/reasonforge/internal/events"
	"github.com/reasonforge/reasonforge/internal/modelrouter"
	"github.com/reasonforge/reasonforge/internal/patch"
	"github.com/reasonforge/reasonforge/internal/scratchpad"
	"github.com/reasonforge/reasonforge/internal/task"
	"github.com/reasonforge/reasonforge/internal/tools"
	"github.com/reasonforge/reasonforge/internal/worktree"
)

// AgentState represents the lifecycle state of an agent run or step.
type AgentState string

const (
	AgentStatePending         AgentState = "pending"
	AgentStateRunning         AgentState = "running"
	AgentStateWaitingApproval AgentState = "waiting_approval"
	AgentStateSucceeded       AgentState = "succeeded"
	AgentStateFailed          AgentState = "failed"
	AgentStateCancelled       AgentState = "cancelled"
)

// IsTerminal returns true if the state is a terminal state.
func (s AgentState) IsTerminal() bool {
	switch s {
	case AgentStateSucceeded, AgentStateFailed, AgentStateCancelled:
		return true
	default:
		return false
	}
}

// ToolCall represents a parsed tool invocation from model output.
type ToolCall struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args"`
}

// AgentStep represents a single iteration within an agent run.
// Each step is either a model call (Type="model") or a tool execution (Type="tool").
type AgentStep struct {
	StepID        string              `json:"step_id"`
	Index         int                 `json:"index"`
	Type          string              `json:"type"` // "model" or "tool"
	State         AgentState          `json:"state"`
	ModelProvider string              `json:"model_provider,omitempty"`
	Model         string              `json:"model,omitempty"`
	ModelText     string              `json:"model_text,omitempty"`
	ToolCall      *ToolCall           `json:"tool_call,omitempty"`
	ToolResponse  *tools.ToolResponse `json:"tool_response,omitempty"`
	Error         string              `json:"error,omitempty"`
	StartedAt     time.Time           `json:"started_at"`
	FinishedAt    time.Time           `json:"finished_at,omitempty"`
}

// AgentRunRequest is the input to an AgentRuntime.Run call.
type AgentRunRequest struct {
	TaskID         string            `json:"task_id"`
	ConversationID string            `json:"conversation_id"`
	RepoRoot       string            `json:"repo_root"`
	Goal           string            `json:"goal"`
	Contract       task.TaskContract `json:"contract"`
	MaxSteps       int               `json:"max_steps"`
	DryRun         bool              `json:"dry_run"`
	Metadata       map[string]string `json:"metadata,omitempty"`

	// UseWorktree enables worktree isolation mode.
	// When true, the agent runs inside an isolated git worktree.
	UseWorktree bool `json:"use_worktree"`

	// WorktreeID optionally specifies an existing worktree to use.
	// If empty and UseWorktree is true, a new worktree is created.
	WorktreeID string `json:"worktree_id,omitempty"`
}

// AgentRunResult is the output of an AgentRuntime.Run call.
type AgentRunResult struct {
	RunID        string      `json:"run_id"`
	TaskID       string      `json:"task_id"`
	State        AgentState  `json:"state"`
	Steps        []AgentStep `json:"steps"`
	FinalMessage string      `json:"final_message,omitempty"`
	Error        string      `json:"error,omitempty"`
	StartedAt    time.Time   `json:"started_at"`
	FinishedAt   time.Time   `json:"finished_at,omitempty"`

	// WorktreeID is the ID of the worktree used (when UseWorktree=true).
	WorktreeID string `json:"worktree_id,omitempty"`

	// PatchPreview contains the diff preview from the worktree (when UseWorktree=true).
	PatchPreview *patch.PatchPreview `json:"patch_preview,omitempty"`
}

// Dependencies holds the external dependencies required by an AgentRuntime.
type Dependencies struct {
	ContextEngine   contextengine.ContextEngine
	ModelRouter     modelrouter.ModelRouter
	ToolRuntime     tools.ToolRuntime
	ToolRegistry    tools.ToolRegistry
	ConversationLog conversation.ConversationLog
	Scratchpad      scratchpad.Scratchpad
	CheckpointStore CheckpointStore

	// WorktreeMgr is optional. When provided with UseWorktree=true,
	// the agent runs inside an isolated git worktree.
	WorktreeMgr worktree.WorktreeManager

	// PatchMgr is optional. When provided with UseWorktree=true,
	// the agent result includes a PatchPreview.
	PatchMgr patch.PatchManager

	// EventEmitter is optional. When provided, the agent emits
	// structured events (run.started, tool.started, etc.) for
	// progress tracking. When nil, no events are emitted.
	EventEmitter events.EventEmitter
}

// AgentRuntime is the interface for running an agent loop.
type AgentRuntime interface {
	// Run executes the agent loop for the given request.
	// The agent builds context, calls the model, parses tool calls,
	// checks approval, executes tools, and loops until completion
	// or a termination condition is met.
	Run(ctx context.Context, req AgentRunRequest) (AgentRunResult, error)
}

// GenerateRunID creates a unique run identifier.
func GenerateRunID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return "run_" + hex.EncodeToString(b), nil
}

// GenerateStepID creates a unique step identifier.
func GenerateStepID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate step id: %w", err)
	}
	return "step_" + hex.EncodeToString(b), nil
}
