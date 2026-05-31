// Package dashboard implements the local TUI dashboard for NekoMIMO.
//
// The dashboard provides a terminal-based view of run progress by reading
// from the EventStore. It reuses EventStore, RunTimeline, and ProgressState
// from the events package; it does not re-implement event parsing.
//
// This package does NOT implement Web UI, TUI frameworks, Desktop, or
// real-time servers. It is a lightweight CLI dashboard using only Go
// standard library for output.
//
// Security:
//   - Only displays data from SanitizeEvent-processed events.
//   - Never prints API keys, sensitive diffs, file content, or patch content.
//   - Does not read source files or trigger tool execution.
//   - Does not auto-apply, auto-commit, or auto-push.
package dashboard

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nekonomimo/nekonomimo/internal/events"
)

// Store is the subset of EventStore that the dashboard needs.
type Store interface {
	ListRuns(ctx context.Context) ([]events.RunSummary, error)
	GetTimeline(ctx context.Context, runID string) (events.RunTimeline, error)
}

// RenderRunsList renders a summary table of recent runs to w.
// It does not print sensitive data; all content comes from RunSummary
// which is derived from SanitizeEvent-processed events.
func RenderRunsList(w io.Writer, runs []events.RunSummary, limit int) {
	fmt.Fprintln(w, "NekoMIMO Dashboard")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Recent Runs")
	fmt.Fprintf(w, "%-36s %-12s %-10s %-14s %s\n", "RUN ID", "STATE", "PROGRESS", "PHASE", "LAST EVENT")

	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}

	for _, r := range runs {
		phase := "-"
		percent := 0

		// We need the timeline to compute progress accurately,
		// but for the list view we derive a quick estimate from state.
		switch r.State {
		case "succeeded":
			percent = 100
			phase = "completed"
		case "failed":
			percent = 100
			phase = "failed"
		case "cancelled":
			percent = 100
			phase = "cancelled"
		default:
			phase = guessPhaseFromEventType(string(r.LastEventType))
		}

		started := formatTime(r.StartedAt)
		lastEvent := string(r.LastEventType)
		if len(lastEvent) > 30 {
			lastEvent = lastEvent[:27] + "..."
		}

		fmt.Fprintf(w, "%-36s %-12s %-9s%% %-14s %s\n",
			r.RunID, r.State, fmt.Sprintf("%d", percent), phase, started+" "+lastEvent)
	}
}

// RenderRunsListWithProgress renders a summary table with accurate progress
// by reading timelines from the store. This is heavier than RenderRunsList
// but provides accurate progress percentages.
func RenderRunsListWithProgress(w io.Writer, store Store, runs []events.RunSummary, limit int) {
	fmt.Fprintln(w, "NekoMIMO Dashboard")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Recent Runs")
	fmt.Fprintf(w, "%-36s %-12s %-10s %-14s %s\n", "RUN ID", "STATE", "PROGRESS", "PHASE", "LAST EVENT")

	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}

	ctx := context.Background()
	for _, r := range runs {
		phase := "-"
		percent := 0

		if r.State == "running" {
			timeline, err := store.GetTimeline(ctx, r.RunID)
			if err == nil && timeline.RunID != "" {
				progress := events.ComputeProgressState(timeline)
				percent = progress.Percent
				if progress.CurrentPhase != "" {
					phase = progress.CurrentPhase
				}
			}
		} else {
			switch r.State {
			case "succeeded":
				percent = 100
				phase = "completed"
			case "failed":
				percent = 100
				phase = "failed"
			case "cancelled":
				percent = 100
				phase = "cancelled"
			}
		}

		started := formatTime(r.StartedAt)
		lastEvent := string(r.LastEventType)
		if len(lastEvent) > 30 {
			lastEvent = lastEvent[:27] + "..."
		}

		fmt.Fprintf(w, "%-36s %-12s %-9s%% %-14s %s\n",
			r.RunID, r.State, fmt.Sprintf("%d", percent), phase, started+" "+lastEvent)
	}
}

// RenderRunDetail renders a detailed view of a single run to w.
// It shows progress state and timeline steps.
// All data comes from already-sanitized events.
func RenderRunDetail(w io.Writer, store Store, runID string) error {
	ctx := context.Background()

	timeline, err := store.GetTimeline(ctx, runID)
	if err != nil {
		return fmt.Errorf("dashboard: get timeline: %w", err)
	}
	if timeline.RunID == "" {
		return fmt.Errorf("dashboard: run %q not found", runID)
	}

	progress := events.ComputeProgressState(timeline)

	fmt.Fprintln(w, "NekoMIMO Run Detail")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Run:          %s\n", progress.RunID)
	fmt.Fprintf(w, "State:        %s\n", progress.State)
	fmt.Fprintf(w, "Progress:     %d%%\n", progress.Percent)
	if progress.CurrentPhase != "" {
		fmt.Fprintf(w, "Current Phase: %s\n", progress.CurrentPhase)
	}
	fmt.Fprintf(w, "Steps:        %d/%d completed\n", progress.CompletedSteps, progress.TotalSteps)
	if !timeline.StartedAt.IsZero() {
		fmt.Fprintf(w, "Started:      %s\n", timeline.StartedAt.Format(time.RFC3339))
	}
	if !timeline.FinishedAt.IsZero() {
		fmt.Fprintf(w, "Finished:     %s\n", timeline.FinishedAt.Format(time.RFC3339))
	}
	if timeline.DurationMs > 0 {
		fmt.Fprintf(w, "Duration:     %dms\n", timeline.DurationMs)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Timeline:")

	if len(timeline.Steps) == 0 {
		fmt.Fprintln(w, "  (no steps)")
		return nil
	}

	for _, step := range timeline.Steps {
		status := formatStepStatus(step.Status)
		msg := step.Message
		if len(msg) > 50 {
			msg = msg[:47] + "..."
		}
		duration := ""
		if step.DurationMs > 0 {
			duration = fmt.Sprintf(" (%dms)", step.DurationMs)
		}
		fmt.Fprintf(w, "  %s %-32s %s%s\n", status, step.Type, msg, duration)

		// Render children (tool calls)
		for _, child := range step.Children {
			childStatus := formatStepStatus(child.Status)
			childMsg := child.Message
			if len(childMsg) > 40 {
				childMsg = childMsg[:37] + "..."
			}
			childDuration := ""
			if child.DurationMs > 0 {
				childDuration = fmt.Sprintf(" (%dms)", child.DurationMs)
			}
			fmt.Fprintf(w, "    %s %-28s %s%s\n", childStatus, child.Type, childMsg, childDuration)
		}
	}

	return nil
}

// formatStepStatus returns a display string for a step status.
func formatStepStatus(status string) string {
	switch status {
	case "started", "running":
		return "[running] "
	case "succeeded":
		return "[done]    "
	case "failed":
		return "[failed]  "
	case "cancelled":
		return "[cancel]  "
	default:
		return "[?]       "
	}
}

// guessPhaseFromEventType extracts a phase hint from an event type string.
func guessPhaseFromEventType(eventType string) string {
	switch {
	case strings.HasPrefix(eventType, "planner."):
		return "planner"
	case strings.HasPrefix(eventType, "coder."):
		return "coder"
	case strings.HasPrefix(eventType, "reviewer."):
		return "reviewer"
	case strings.HasPrefix(eventType, "tool."):
		return "tool"
	case strings.HasPrefix(eventType, "validation."):
		return "validation"
	case strings.HasPrefix(eventType, "patch."):
		return "patch"
	case strings.HasPrefix(eventType, "run."):
		return "run"
	default:
		return "-"
	}
}

// formatTime formats a time value for display.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}
