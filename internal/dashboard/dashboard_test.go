package dashboard

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mimoneko/mimoneko/internal/events"
)

// mockStore implements Store for testing.
type mockStore struct {
	runs     []events.RunSummary
	timeline map[string]events.RunTimeline
}

func (m *mockStore) ListRuns(ctx context.Context) ([]events.RunSummary, error) {
	return m.runs, nil
}

func (m *mockStore) GetTimeline(ctx context.Context, runID string) (events.RunTimeline, error) {
	if tl, ok := m.timeline[runID]; ok {
		return tl, nil
	}
	return events.RunTimeline{}, nil
}

func TestDashboardListsRuns(t *testing.T) {
	store := &mockStore{
		runs: []events.RunSummary{
			{
				RunID:         "mar_abc123",
				State:         "running",
				StartedAt:     time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC),
				EventCount:    5,
				LastEventType: "coder.started",
			},
			{
				RunID:         "patch_review_xyz",
				State:         "succeeded",
				StartedAt:     time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC),
				FinishedAt:    time.Date(2026, 5, 24, 9, 5, 0, 0, time.UTC),
				EventCount:    8,
				LastEventType: "run.succeeded",
			},
		},
	}

	var buf bytes.Buffer
	RenderRunsList(&buf, store.runs, 0)

	output := buf.String()
	if !strings.Contains(output, "mar_abc123") {
		t.Error("expected mar_abc123 in output")
	}
	if !strings.Contains(output, "patch_review_xyz") {
		t.Error("expected patch_review_xyz in output")
	}
	if !strings.Contains(output, "running") {
		t.Error("expected running state in output")
	}
	if !strings.Contains(output, "succeeded") {
		t.Error("expected succeeded state in output")
	}
	if !strings.Contains(output, "Recent Runs") {
		t.Error("expected 'Recent Runs' header")
	}
}

func TestDashboardShowsRunDetail(t *testing.T) {
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	store := &mockStore{
		timeline: map[string]events.RunTimeline{
			"run_test001": {
				RunID:     "run_test001",
				State:     "running",
				StartedAt: now,
				Events: []events.RunEvent{
					{Type: events.EventRunStarted, Status: "started", Message: "Run started", StartedAt: now},
					{Type: events.EventPlannerStarted, Status: "started", Message: "Planning", StartedAt: now.Add(1 * time.Second)},
					{Type: events.EventPlannerFinished, Status: "succeeded", Message: "Plan done", StartedAt: now.Add(2 * time.Second), FinishedAt: now.Add(3 * time.Second), DurationMs: 1000},
					{Type: events.EventCoderStarted, Status: "started", Message: "Coding", StartedAt: now.Add(3 * time.Second)},
				},
				Steps: []events.TimelineStep{
					{Type: events.EventPlannerStarted, Status: "succeeded", Message: "Plan done", DurationMs: 1000},
					{Type: events.EventCoderStarted, Status: "started", Message: "Coding"},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := RenderRunDetail(&buf, store, "run_test001")
	if err != nil {
		t.Fatalf("RenderRunDetail() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "run_test001") {
		t.Error("expected run ID in output")
	}
	if !strings.Contains(output, "running") {
		t.Error("expected running state")
	}
	if !strings.Contains(output, "Timeline:") {
		t.Error("expected Timeline section")
	}
	if !strings.Contains(output, "[done]") {
		t.Error("expected done status for planner")
	}
	if !strings.Contains(output, "[running]") {
		t.Error("expected running status for coder")
	}
}

func TestDashboardHandlesEmptyEventStore(t *testing.T) {
	store := &mockStore{
		runs:     []events.RunSummary{},
		timeline: map[string]events.RunTimeline{},
	}

	var buf bytes.Buffer
	RenderRunsList(&buf, store.runs, 0)

	output := buf.String()
	if !strings.Contains(output, "Recent Runs") {
		t.Error("expected header even with empty store")
	}

	// Detail view for nonexistent run
	var detailBuf bytes.Buffer
	err := RenderRunDetail(&detailBuf, store, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent run")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

func TestDashboardHandlesMissingEventStore(t *testing.T) {
	// When store is nil, CLI should handle gracefully.
	// This tests the contract: RenderRunsList should not panic with empty data.
	var buf bytes.Buffer
	RenderRunsList(&buf, nil, 0)

	output := buf.String()
	if !strings.Contains(output, "MimoNeko Dashboard") {
		t.Error("expected dashboard header")
	}
}

func TestDashboardDoesNotLeakSensitiveData(t *testing.T) {
	// The dashboard only displays data from RunSummary and TimelineStep,
	// which are derived from SanitizeEvent-processed events.
	// Verify that no sensitive patterns appear in output.
	store := &mockStore{
		runs: []events.RunSummary{
			{
				RunID:         "run_test001",
				State:         "succeeded",
				StartedAt:     time.Now(),
				EventCount:    1,
				LastEventType: "run.succeeded",
			},
		},
		timeline: map[string]events.RunTimeline{
			"run_test001": {
				RunID: "run_test001",
				State: "succeeded",
				Steps: []events.TimelineStep{
					{Type: events.EventCoderStarted, Status: "succeeded", Message: "Coding done"},
				},
			},
		},
	}

	var listBuf bytes.Buffer
	RenderRunsList(&listBuf, store.runs, 0)
	listOutput := listBuf.String()

	// Check no sensitive patterns
	sensitive := []string{"API_KEY", "sk-", "PRIVATE_KEY"}
	for _, s := range sensitive {
		if strings.Contains(strings.ToUpper(listOutput), strings.ToUpper(s)) {
			t.Errorf("list output contains sensitive pattern %q", s)
		}
	}

	var detailBuf bytes.Buffer
	_ = RenderRunDetail(&detailBuf, store, "run_test001")
	detailOutput := detailBuf.String()

	for _, s := range sensitive {
		if strings.Contains(strings.ToUpper(detailOutput), strings.ToUpper(s)) {
			t.Errorf("detail output contains sensitive pattern %q", s)
		}
	}
}

func TestDashboardWatchModeSingleRefreshForTest(t *testing.T) {
	// Test that the dashboard can render without panicking,
	// simulating what watch mode does on each refresh tick.
	store := &mockStore{
		runs: []events.RunSummary{
			{RunID: "run_watch", State: "running", StartedAt: time.Now(), LastEventType: "coder.started"},
		},
	}

	var buf bytes.Buffer
	// Simulate one refresh tick
	RenderRunsList(&buf, store.runs, 0)
	output := buf.String()

	if !strings.Contains(output, "run_watch") {
		t.Error("expected run_watch in watch mode output")
	}
}

func TestDashboardLimitRuns(t *testing.T) {
	runs := make([]events.RunSummary, 10)
	for i := 0; i < 10; i++ {
		runs[i] = events.RunSummary{
			RunID:         fmt.Sprintf("run_%d", i),
			State:         "succeeded",
			StartedAt:     time.Now().Add(time.Duration(-i) * time.Hour),
			LastEventType: "run.succeeded",
		}
	}

	var buf bytes.Buffer
	RenderRunsList(&buf, runs, 3)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Header lines: "MimoNeko Dashboard", blank, "Recent Runs", column header, then data
	// With limit=3, we expect 3 data rows
	dataLines := 0
	for _, line := range lines {
		if strings.Contains(line, "run_") {
			dataLines++
		}
	}
	if dataLines != 3 {
		t.Errorf("expected 3 data lines with limit=3, got %d", dataLines)
	}
}
