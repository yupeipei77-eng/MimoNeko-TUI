// Package events implements the structured event system for MimoNeko.
//
// The event system records run lifecycle events (run.started, planner.started,
// coder.finished, etc.) so that subsequent phases (Dashboard / TUI / Desktop)
// can display task execution progress.
//
// This package only provides the backend data foundation. It does NOT implement
// Web UI, TUI, or Desktop interfaces.
//
// Safety guarantees:
//   - Events never contain API keys, full diffs, file content, or patch content.
//   - SanitizeEvent() enforces redaction before persistence.
//   - EventStore uses append-only JSONL; no update or delete is allowed.
//   - Event IDs use crypto/rand for collision resistance.
//   - Event emission failure does not crash the runtime (except initialization).
package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// EventType identifies the kind of event that occurred.
type EventType string

const (
	// Run lifecycle events
	EventRunStarted   EventType = "run.started"
	EventRunSucceeded EventType = "run.succeeded"
	EventRunFailed    EventType = "run.failed"
	EventRunCancelled EventType = "run.cancelled"

	// Planner events
	EventPlannerStarted  EventType = "planner.started"
	EventPlannerFinished EventType = "planner.finished"

	// Coder events
	EventCoderStarted  EventType = "coder.started"
	EventCoderFinished EventType = "coder.finished"

	// Reviewer events
	EventReviewerStarted  EventType = "reviewer.started"
	EventReviewerFinished EventType = "reviewer.finished"

	// Tool events
	EventToolStarted   EventType = "tool.started"
	EventToolFinished  EventType = "tool.finished"
	EventToolCalled    EventType = "tool.called"
	EventToolCompleted EventType = "tool.completed"
	EventToolFailed    EventType = "tool.failed"

	// Validation events
	EventValidationStarted  EventType = "validation.started"
	EventValidationFinished EventType = "validation.finished"

	// Patch preview events
	EventPatchPreviewStarted  EventType = "patch.preview.started"
	EventPatchPreviewFinished EventType = "patch.preview.finished"

	// Streaming events
	EventStreamDelta EventType = "stream.delta"
	EventStreamDone  EventType = "stream.done"

	// Sandbox detection events (detection only, no enforcement)
	EventPathViolationCandidate EventType = "path.violation_candidate"

	// Security enforcement events
	EventSecurityWarning      EventType = "security.warning"
	EventPathBlocked          EventType = "path.blocked"
	EventToolDenied           EventType = "tool.denied"
	EventToolApprovalRequired EventType = "tool.approval_required"

	// Agent workflow events
	EventWorkflowStarted   EventType = "agent.workflow_started"
	EventStepStarted       EventType = "agent.step_started"
	EventStepCompleted     EventType = "agent.step_completed"
	EventStepFailed        EventType = "agent.step_failed"
	EventWorkflowCompleted EventType = "agent.workflow_completed"
	EventWorkflowFailed    EventType = "agent.workflow_failed"
)

// IsTerminal returns true if the event type represents a terminal state
// for a run lifecycle.
func (t EventType) IsTerminal() bool {
	switch t {
	case EventRunSucceeded, EventRunFailed, EventRunCancelled:
		return true
	default:
		return false
	}
}

// IsStarted returns true if the event type represents the start of a phase.
func (t EventType) IsStarted() bool {
	switch t {
	case EventRunStarted, EventPlannerStarted, EventCoderStarted,
		EventReviewerStarted, EventToolStarted, EventValidationStarted,
		EventPatchPreviewStarted:
		return true
	default:
		return false
	}
}

// IsFinished returns true if the event type represents the completion of a phase.
func (t EventType) IsFinished() bool {
	switch t {
	case EventRunSucceeded, EventRunFailed, EventRunCancelled,
		EventPlannerFinished, EventCoderFinished, EventReviewerFinished,
		EventToolFinished, EventValidationFinished, EventPatchPreviewFinished:
		return true
	default:
		return false
	}
}

// RunEvent is a structured record of something that happened during a run.
type RunEvent struct {
	// ID is the unique event identifier (crypto/rand).
	ID string `json:"id"`

	// RunID identifies the run this event belongs to.
	RunID string `json:"run_id"`

	// TaskID identifies the task (optional).
	TaskID string `json:"task_id,omitempty"`

	// WorktreeID identifies the worktree (optional).
	WorktreeID string `json:"worktree_id,omitempty"`

	// Timestamp is the primary occurrence time for structured audit events.
	Timestamp time.Time `json:"timestamp,omitempty"`

	// ToolName identifies the tool for structured tool audit events.
	ToolName string `json:"tool_name,omitempty"`

	// RiskLevel records the tool metadata risk level for audit review.
	RiskLevel string `json:"risk_level,omitempty"`

	// RequiresApproval records the tool metadata approval flag. Pointer form
	// allows false to be emitted for low-risk tools while non-tool events omit it.
	RequiresApproval *bool `json:"requires_approval,omitempty"`

	// ResultStatus mirrors Status for tool audit events in a stable field.
	ResultStatus string `json:"result_status,omitempty"`

	// ErrorMessage mirrors Error for tool audit events in a stable field.
	ErrorMessage string `json:"error_message,omitempty"`

	// StepID identifies the step within a run (optional).
	StepID string `json:"step_id,omitempty"`

	// ParentID references a parent event for nesting (optional).
	ParentID string `json:"parent_id,omitempty"`

	// Type is the event type.
	Type EventType `json:"type"`

	// Source is the component that emitted the event:
	// agent, tool, review, validation, patch, cli.
	Source string `json:"source"`

	// Status is the event status: started, running, succeeded, failed, cancelled.
	Status string `json:"status"`

	// Message is a human-readable description (sanitized).
	Message string `json:"message,omitempty"`

	// Metadata contains additional key-value pairs (sanitized).
	Metadata map[string]string `json:"metadata,omitempty"`

	// StartedAt is when the associated phase started (optional).
	StartedAt time.Time `json:"started_at,omitempty"`

	// FinishedAt is when the associated phase finished (optional).
	FinishedAt time.Time `json:"finished_at,omitempty"`

	// DurationMs is the duration in milliseconds (optional).
	DurationMs int64 `json:"duration_ms,omitempty"`

	// Error contains error information (sanitized).
	Error string `json:"error,omitempty"`
}

// GenerateEventID creates a unique event identifier using crypto/rand.
func GenerateEventID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("events: generate event id: %w", err)
	}
	return "evt_" + hex.EncodeToString(b), nil
}

// EventFilter defines criteria for subscribing to events.
type EventFilter struct {
	// RunID filters events by run ID.
	RunID string

	// TaskID filters events by task ID.
	TaskID string

	// Types filters events by event types.
	Types []EventType
}

// Matches returns true if the event matches the filter criteria.
func (f EventFilter) Matches(event RunEvent) bool {
	if f.RunID != "" && event.RunID != f.RunID {
		return false
	}
	if f.TaskID != "" && event.TaskID != f.TaskID {
		return false
	}
	if len(f.Types) > 0 {
		found := false
		for _, t := range f.Types {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// EventBus is the interface for emitting and subscribing to events.
type EventBus interface {
	// Emit publishes an event to all subscribers.
	Emit(ctx context.Context, event RunEvent) error

	// Subscribe registers a subscriber that receives events matching the filter.
	// Returns a channel of events and an error.
	Subscribe(ctx context.Context, filter EventFilter) (<-chan RunEvent, error)
}

// EventSink is the interface for writing events to a persistence layer.
type EventSink interface {
	// Write persists an event.
	Write(ctx context.Context, event RunEvent) error
}

// EventStore is the interface for querying stored events.
type EventStore interface {
	// Append persists an event (append-only, no update or delete).
	Append(ctx context.Context, event RunEvent) error

	// ListRuns returns a summary of all runs.
	ListRuns(ctx context.Context) ([]RunSummary, error)

	// ListEvents returns all events for a given run, ordered by time.
	ListEvents(ctx context.Context, runID string) ([]RunEvent, error)

	// GetTimeline reconstructs the timeline for a given run.
	GetTimeline(ctx context.Context, runID string) (RunTimeline, error)
}

// RunSummary is an aggregated view of a single run.
type RunSummary struct {
	// RunID is the unique run identifier.
	RunID string `json:"run_id"`

	// TaskID identifies the task (optional).
	TaskID string `json:"task_id,omitempty"`

	// State is the current state of the run: running, succeeded, failed, cancelled.
	State string `json:"state"`

	// StartedAt is when the run started.
	StartedAt time.Time `json:"started_at"`

	// FinishedAt is when the run finished (zero if still running).
	FinishedAt time.Time `json:"finished_at,omitempty"`

	// EventCount is the total number of events for this run.
	EventCount int `json:"event_count"`

	// LastEventType is the type of the most recent event.
	LastEventType EventType `json:"last_event_type"`

	// LastMessage is the message from the most recent event.
	LastMessage string `json:"last_message,omitempty"`
}

// RunContext carries run identification through the call stack via context.Context.
// All runtime components that emit events must read RunContext from ctx to populate
// RunID, TaskID, and WorktreeID on their events.
type RunContext struct {
	RunID      string
	TaskID     string
	WorktreeID string
}

type runContextKey struct{}

// WithRunContext returns a new context carrying the given RunContext.
func WithRunContext(ctx context.Context, rc RunContext) context.Context {
	return context.WithValue(ctx, runContextKey{}, rc)
}

// RunContextFrom extracts RunContext from ctx. Returns a zero-value RunContext
// if no RunContext is present.
func RunContextFrom(ctx context.Context) RunContext {
	if ctx == nil {
		return RunContext{}
	}
	rc, ok := ctx.Value(runContextKey{}).(RunContext)
	if !ok {
		return RunContext{}
	}
	return rc
}
