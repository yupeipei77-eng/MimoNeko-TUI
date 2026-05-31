package events

import (
	"time"
)

// RunTimeline is a reconstructed view of a run's execution timeline.
type RunTimeline struct {
	// RunID is the unique run identifier.
	RunID string `json:"run_id"`

	// TaskID identifies the task (optional).
	TaskID string `json:"task_id,omitempty"`

	// State is the current state: running, succeeded, failed, cancelled.
	State string `json:"state"`

	// Events is the raw list of events for the run.
	Events []RunEvent `json:"events"`

	// Steps is the hierarchical view of execution phases.
	Steps []TimelineStep `json:"steps"`

	// StartedAt is when the run started.
	StartedAt time.Time `json:"started_at"`

	// FinishedAt is when the run finished (zero if still running).
	FinishedAt time.Time `json:"finished_at,omitempty"`

	// DurationMs is the total duration in milliseconds.
	DurationMs int64 `json:"duration_ms,omitempty"`
}

// TimelineStep represents a single phase in the execution timeline,
// potentially with nested children (e.g., tool events under a coder step).
type TimelineStep struct {
	// ID is the event ID for this step.
	ID string `json:"id"`

	// Type is the event type.
	Type EventType `json:"type"`

	// Source is the component that emitted the event.
	Source string `json:"source"`

	// Status is the step status.
	Status string `json:"status"`

	// Message is a human-readable description.
	Message string `json:"message,omitempty"`

	// StartedAt is when the step started.
	StartedAt time.Time `json:"started_at,omitempty"`

	// FinishedAt is when the step finished.
	FinishedAt time.Time `json:"finished_at,omitempty"`

	// DurationMs is the step duration in milliseconds.
	DurationMs int64 `json:"duration_ms,omitempty"`

	// Children are nested sub-steps (e.g., tool calls under a coder step).
	Children []TimelineStep `json:"children,omitempty"`
}

// ProgressState represents the current progress of a run.
type ProgressState struct {
	// RunID is the unique run identifier.
	RunID string `json:"run_id"`

	// State is the current run state.
	State string `json:"state"`

	// CurrentPhase is the current execution phase (e.g., "planner", "coder", "reviewer").
	CurrentPhase string `json:"current_phase"`

	// Percent is the estimated completion percentage (0-100).
	Percent int `json:"percent"`

	// CompletedSteps is the number of completed steps.
	CompletedSteps int `json:"completed_steps"`

	// TotalSteps is the estimated total number of steps.
	TotalSteps int `json:"total_steps"`

	// LastEvent is the most recent event.
	LastEvent RunEvent `json:"last_event"`
}

// phasePercent maps event types to heuristic progress percentages.
var phasePercent = map[EventType]int{
	EventRunStarted:           5,
	EventPlannerStarted:       10,
	EventPlannerFinished:      20,
	EventCoderStarted:         25,
	EventCoderFinished:        50,
	EventReviewerStarted:      55,
	EventReviewerFinished:     80,
	EventValidationStarted:    82,
	EventValidationFinished:   90,
	EventPatchPreviewStarted:  91,
	EventPatchPreviewFinished: 95,
	EventRunSucceeded:         100,
	EventRunFailed:            100,
	EventRunCancelled:         100,
}

// ComputeProgressState derives the current progress state from a list of events.
func ComputeProgressState(timeline RunTimeline) ProgressState {
	state := "running"
	percent := 0
	currentPhase := ""
	completedSteps := 0

	if len(timeline.Events) == 0 {
		return ProgressState{
			RunID:          timeline.RunID,
			State:          "unknown",
			CurrentPhase:   "",
			Percent:        0,
			CompletedSteps: 0,
			TotalSteps:     0,
		}
	}

	lastEvent := timeline.Events[len(timeline.Events)-1]

	// Walk events to compute progress
	for _, evt := range timeline.Events {
		if p, ok := phasePercent[evt.Type]; ok {
			if p > percent {
				percent = p
			}
		}

		// Count finished steps
		if evt.Type.IsFinished() && evt.Type != EventRunSucceeded &&
			evt.Type != EventRunFailed && evt.Type != EventRunCancelled {
			completedSteps++
		}

		// Determine current phase from started or finished events
		if evt.Type.IsStarted() {
			currentPhase = phaseFromEventType(evt.Type)
		} else if evt.Type.IsFinished() && !evt.Type.IsTerminal() {
			currentPhase = phaseFromEventType(evt.Type)
		}
	}

	// Determine overall run state
	switch lastEvent.Type {
	case EventRunSucceeded:
		state = "succeeded"
		currentPhase = ""
	case EventRunFailed:
		state = "failed"
		currentPhase = ""
	case EventRunCancelled:
		state = "cancelled"
		currentPhase = ""
	}

	// Estimate total steps from timeline steps
	totalSteps := len(timeline.Steps)
	if totalSteps == 0 {
		// Estimate from events: count started events
		startedCount := 0
		for _, evt := range timeline.Events {
			if evt.Type.IsStarted() && evt.Type != EventRunStarted {
				startedCount++
			}
		}
		totalSteps = startedCount
	}

	return ProgressState{
		RunID:          timeline.RunID,
		State:          state,
		CurrentPhase:   currentPhase,
		Percent:        percent,
		CompletedSteps: completedSteps,
		TotalSteps:     totalSteps,
		LastEvent:      lastEvent,
	}
}

// phaseFromEventType extracts the phase name from a started or finished event type.
func phaseFromEventType(t EventType) string {
	switch t {
	case EventRunStarted, EventRunSucceeded, EventRunFailed, EventRunCancelled:
		return "run"
	case EventPlannerStarted, EventPlannerFinished:
		return "planner"
	case EventCoderStarted, EventCoderFinished:
		return "coder"
	case EventReviewerStarted, EventReviewerFinished:
		return "reviewer"
	case EventToolStarted, EventToolFinished:
		return "tool"
	case EventValidationStarted, EventValidationFinished:
		return "validation"
	case EventPatchPreviewStarted, EventPatchPreviewFinished:
		return "patch_preview"
	default:
		return ""
	}
}

// BuildTimeline constructs a RunTimeline from a list of events.
func BuildTimeline(events []RunEvent) RunTimeline {
	if len(events) == 0 {
		return RunTimeline{}
	}

	runID := events[0].RunID
	taskID := events[0].TaskID
	state := "running"
	var startedAt, finishedAt time.Time
	var durationMs int64

	// Find run start and end times
	for _, evt := range events {
		if evt.Type == EventRunStarted && (startedAt.IsZero() || evt.StartedAt.Before(startedAt)) {
			startedAt = evt.StartedAt
			if startedAt.IsZero() {
				startedAt = evt.StartedAt
			}
		}
		if evt.Type.IsTerminal() {
			state = runStateFromEvent(evt.Type)
			finishedAt = evt.FinishedAt
			durationMs = evt.DurationMs
		}
	}

	// If no explicit started time found, use first event
	if startedAt.IsZero() {
		startedAt = events[0].StartedAt
		if startedAt.IsZero() {
			startedAt = events[0].StartedAt
		}
	}

	// Build hierarchical steps
	steps := buildTimelineSteps(events)

	// Calculate duration if not set
	if durationMs == 0 && !startedAt.IsZero() && !finishedAt.IsZero() {
		durationMs = finishedAt.Sub(startedAt).Milliseconds()
	}

	return RunTimeline{
		RunID:      runID,
		TaskID:     taskID,
		State:      state,
		Events:     events,
		Steps:      steps,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		DurationMs: durationMs,
	}
}

// buildTimelineSteps organizes events into a hierarchical structure.
// Tool events become children of their parent phase events.
func buildTimelineSteps(events []RunEvent) []TimelineStep {
	var steps []TimelineStep
	currentParentIdx := -1 // index into steps for the current parent phase

	for _, evt := range events {
		// Skip run-level events (they are captured in the timeline itself)
		if evt.Type == EventRunStarted || evt.Type.IsTerminal() {
			continue
		}

		step := TimelineStep{
			ID:         evt.ID,
			Type:       evt.Type,
			Source:     evt.Source,
			Status:     evt.Status,
			Message:    evt.Message,
			StartedAt:  evt.StartedAt,
			FinishedAt: evt.FinishedAt,
			DurationMs: evt.DurationMs,
		}

		// Tool events are children of the current parent (coder/reviewer)
		if evt.Type == EventToolStarted || evt.Type == EventToolFinished {
			if currentParentIdx >= 0 && currentParentIdx < len(steps) {
				parent := &steps[currentParentIdx]
				// Check if this is a "finished" matching the current tool
				if evt.Type == EventToolFinished && len(parent.Children) > 0 {
					lastChild := &parent.Children[len(parent.Children)-1]
					if lastChild.Type == EventToolStarted && lastChild.Status == "started" {
						// Update the last tool started step with finish info
						lastChild.Status = evt.Status
						lastChild.FinishedAt = evt.FinishedAt
						lastChild.DurationMs = evt.DurationMs
						if evt.Error != "" {
							lastChild.Message = evt.Error
						}
						continue
					}
				}
				parent.Children = append(parent.Children, step)
				continue
			}
			// No parent, add as top-level
			steps = append(steps, step)
			continue
		}

		// Finished events update their corresponding started step
		if evt.Type.IsFinished() {
			// Find the matching started step
			for i := len(steps) - 1; i >= 0; i-- {
				if steps[i].Type == matchingStartedType(evt.Type) && steps[i].Status == "started" {
					steps[i].Status = evt.Status
					steps[i].FinishedAt = evt.FinishedAt
					steps[i].DurationMs = evt.DurationMs
					if evt.Message != "" {
						steps[i].Message = evt.Message
					}
					currentParentIdx = -1
					break
				}
			}
			continue
		}

		// Started events create new top-level steps
		if evt.Type.IsStarted() {
			steps = append(steps, step)
			// Track as potential parent for nested tool events
			currentParentIdx = len(steps) - 1
		}
	}

	return steps
}

// matchingStartedType returns the started event type for a finished event type.
func matchingStartedType(finished EventType) EventType {
	switch finished {
	case EventPlannerFinished:
		return EventPlannerStarted
	case EventCoderFinished:
		return EventCoderStarted
	case EventReviewerFinished:
		return EventReviewerStarted
	case EventValidationFinished:
		return EventValidationStarted
	case EventPatchPreviewFinished:
		return EventPatchPreviewStarted
	default:
		return ""
	}
}

// runStateFromEvent converts a terminal event type to a run state string.
func runStateFromEvent(t EventType) string {
	switch t {
	case EventRunSucceeded:
		return "succeeded"
	case EventRunFailed:
		return "failed"
	case EventRunCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}
