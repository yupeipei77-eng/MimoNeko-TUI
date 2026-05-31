package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/events"
)

func testRootConfig(root string, enabled bool) *config.Root {
	return &config.Root{
		Events: config.EventsConfig{
			Enabled:   enabled,
			StorePath: ".mimoneko/events/run_events.jsonl",
		},
	}
}

func writeTestEvents(t *testing.T, root string, evts ...events.RunEvent) {
	t.Helper()
	path := filepath.Join(root, ".mimoneko", "events", "run_events.jsonl")
	store, err := events.NewJSONLRunEventStore(path)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore() error = %v", err)
	}
	defer store.Close()

	for _, evt := range evts {
		if err := store.Append(context.Background(), evt); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}
}

func newTestServer(root string, enabled bool) *LocalServer {
	return NewLocalServer(root, testRootConfig(root, enabled), Options{})
}

func TestServeDefaultHostLocalhost(t *testing.T) {
	root := t.TempDir()
	srv := NewLocalServer(root, testRootConfig(root, true), Options{})
	if srv.Address() != "127.0.0.1:8765" {
		t.Fatalf("Address() = %q, want 127.0.0.1:8765", srv.Address())
	}
}

func TestHealthzEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	newTestServer(t.TempDir(), true).Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != `{"ok":true}` {
		t.Fatalf("body = %q, want ok JSON", rr.Body.String())
	}
}

func TestAPIRunsReturnsRuns(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	writeTestEvents(t, root,
		events.RunEvent{ID: "evt_1", RunID: "run_api_1", Type: events.EventRunStarted, Source: "test", Status: "started", StartedAt: now},
		events.RunEvent{ID: "evt_2", RunID: "run_api_1", Type: events.EventReviewerStarted, Source: "review", Status: "started", StartedAt: now.Add(time.Second)},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var resp runsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(resp.Runs) != 1 {
		t.Fatalf("runs len = %d, want 1", len(resp.Runs))
	}
	if resp.Runs[0].RunID != "run_api_1" {
		t.Fatalf("run_id = %q, want run_api_1", resp.Runs[0].RunID)
	}
	if resp.Runs[0].CurrentPhase != "reviewer" {
		t.Fatalf("current_phase = %q, want reviewer", resp.Runs[0].CurrentPhase)
	}
}

func TestAPIRunDetailReturnsTimeline(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	writeTestEvents(t, root,
		events.RunEvent{ID: "evt_1", RunID: "run_detail", Type: events.EventRunStarted, Source: "test", Status: "started", StartedAt: now},
		events.RunEvent{ID: "evt_2", RunID: "run_detail", Type: events.EventPatchPreviewStarted, Source: "patch", Status: "started", StartedAt: now.Add(time.Second)},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run_detail", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var resp runDetailResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if resp.RunID != "run_detail" {
		t.Fatalf("RunID = %q, want run_detail", resp.RunID)
	}
	if resp.Timeline.RunID != "run_detail" {
		t.Fatalf("Timeline.RunID = %q, want run_detail", resp.Timeline.RunID)
	}
	if resp.Progress.CurrentPhase != "patch_preview" {
		t.Fatalf("CurrentPhase = %q, want patch_preview", resp.Progress.CurrentPhase)
	}
}

func TestAPIRunEventsReturnsEvents(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	writeTestEvents(t, root,
		events.RunEvent{ID: "evt_1", RunID: "run_events", Type: events.EventRunStarted, Source: "test", Status: "started", StartedAt: now},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run_events/events", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var resp runEventsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(resp.Events) != 1 {
		t.Fatalf("events len = %d, want 1", len(resp.Events))
	}
	if resp.Events[0].Type != events.EventRunStarted {
		t.Fatalf("event type = %s, want run.started", resp.Events[0].Type)
	}
}

func TestAPIRunNotFound(t *testing.T) {
	root := t.TempDir()
	writeTestEvents(t, root, events.RunEvent{
		ID: "evt_1", RunID: "run_exists", Type: events.EventRunStarted, Source: "test", Status: "started", StartedAt: time.Now().UTC(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/runs/missing", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestAPIRunsHandlesEmptyStore(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".mimoneko", "events", "run_events.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "No runs yet") {
		t.Fatalf("body = %q, want No runs yet", rr.Body.String())
	}
}

func TestAPIDoesNotLeakSensitiveData(t *testing.T) {
	root := t.TempDir()
	writeTestEvents(t, root, events.RunEvent{
		ID:        "evt_secret",
		RunID:     "run_secret",
		Type:      events.EventRunStarted,
		Source:    "test",
		Status:    "started",
		Message:   "API_KEY=sk-secret-value",
		StartedAt: time.Now().UTC(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run_secret/events", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	for _, secret := range []string{"sk-secret-value", "API_KEY="} {
		if strings.Contains(body, secret) {
			t.Fatalf("API leaked %q in body: %s", secret, body)
		}
	}
}

func TestEventsDisabledShowsFriendlyMessage(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	newTestServer(t.TempDir(), false).Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Events system is disabled") {
		t.Fatalf("body = %q, want disabled message", rr.Body.String())
	}
}

func TestMissingEventStoreShowsNoRuns(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	newTestServer(t.TempDir(), true).Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "No runs yet") {
		t.Fatalf("body = %q, want No runs yet", rr.Body.String())
	}
}

func TestDashboardHomePageRenders(t *testing.T) {
	root := t.TempDir()
	writeTestEvents(t, root, events.RunEvent{
		ID: "evt_1", RunID: "run_home", Type: events.EventRunStarted, Source: "test", Status: "started", StartedAt: time.Now().UTC(),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "MimoNeko Dashboard") {
		t.Fatalf("home page missing title: %s", body)
	}
	if !strings.Contains(body, "Recent Runs") {
		t.Fatalf("home page missing Recent Runs: %s", body)
	}
	if !strings.Contains(body, "run_home") {
		t.Fatalf("home page missing run ID: %s", body)
	}
}

func TestDashboardRunDetailPageRenders(t *testing.T) {
	root := t.TempDir()
	writeTestEvents(t, root,
		events.RunEvent{ID: "evt_1", RunID: "run_page", Type: events.EventRunStarted, Source: "test", Status: "started", StartedAt: time.Now().UTC()},
		events.RunEvent{ID: "evt_2", RunID: "run_page", Type: events.EventToolStarted, Source: "tool", Status: "started", Message: "Tool started", StartedAt: time.Now().UTC()},
	)

	req := httptest.NewRequest(http.MethodGet, "/runs/run_page", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "run_page") {
		t.Fatalf("detail page missing run ID: %s", body)
	}
	if !strings.Contains(body, "Timeline") {
		t.Fatalf("detail page missing Timeline: %s", body)
	}
	if !strings.Contains(body, "Events") {
		t.Fatalf("detail page missing Events: %s", body)
	}
}

func TestDashboardRunDetailShowsState(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	writeTestEvents(t, root,
		events.RunEvent{ID: "evt_1", RunID: "run_state", Type: events.EventRunStarted, Source: "test", Status: "started", StartedAt: now},
		events.RunEvent{ID: "evt_2", RunID: "run_state", Type: events.EventRunSucceeded, Source: "test", Status: "succeeded", StartedAt: now, FinishedAt: now.Add(time.Second)},
	)

	req := httptest.NewRequest(http.MethodGet, "/runs/run_state", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `<strong id="state">succeeded</strong>`) {
		t.Fatalf("detail page did not show succeeded state: %s", body)
	}
}

func TestDashboardTerminalRunShowsCompletedPhase(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	writeTestEvents(t, root,
		events.RunEvent{ID: "evt_1", RunID: "run_done", Type: events.EventRunStarted, Source: "test", Status: "started", StartedAt: now},
		events.RunEvent{ID: "evt_2", RunID: "run_done", Type: events.EventReviewerStarted, Source: "review", Status: "started", StartedAt: now.Add(time.Second)},
		events.RunEvent{ID: "evt_3", RunID: "run_done", Type: events.EventRunSucceeded, Source: "test", Status: "succeeded", StartedAt: now, FinishedAt: now.Add(2 * time.Second)},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run_done", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	var resp runDetailResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if resp.Progress.CurrentPhase != "completed" {
		t.Fatalf("CurrentPhase = %q, want completed", resp.Progress.CurrentPhase)
	}
}

func TestDashboardFailedRunShowsFailedPhase(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	writeTestEvents(t, root,
		events.RunEvent{ID: "evt_1", RunID: "run_failed", Type: events.EventRunStarted, Source: "test", Status: "started", StartedAt: now},
		events.RunEvent{ID: "evt_2", RunID: "run_failed", Type: events.EventCoderStarted, Source: "agent", Status: "started", StartedAt: now.Add(time.Second)},
		events.RunEvent{ID: "evt_3", RunID: "run_failed", Type: events.EventRunFailed, Source: "test", Status: "failed", StartedAt: now, FinishedAt: now.Add(2 * time.Second)},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run_failed", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	var resp runDetailResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if resp.Progress.CurrentPhase != "failed" {
		t.Fatalf("CurrentPhase = %q, want failed", resp.Progress.CurrentPhase)
	}
}

func TestDashboardStateCardNotEmpty(t *testing.T) {
	root := t.TempDir()
	writeTestEvents(t, root, events.RunEvent{
		ID: "evt_1", RunID: "run_nonempty_state", Type: events.EventRunStarted, Source: "test", Status: "started", StartedAt: time.Now().UTC(),
	})

	req := httptest.NewRequest(http.MethodGet, "/runs/run_nonempty_state", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, `<strong id="state"></strong>`) {
		t.Fatalf("state card is empty: %s", body)
	}
	if !strings.Contains(body, `<strong id="state">running</strong>`) {
		t.Fatalf("state card did not show running: %s", body)
	}
}

func TestDashboardPageDoesNotLeakSensitiveData(t *testing.T) {
	root := t.TempDir()
	writeTestEvents(t, root, events.RunEvent{
		ID:        "evt_secret",
		RunID:     "run_page_secret",
		Type:      events.EventRunStarted,
		Source:    "test",
		Status:    "started",
		Message:   "PRIVATE_KEY=sk-page-secret",
		StartedAt: time.Now().UTC(),
	})

	req := httptest.NewRequest(http.MethodGet, "/runs/run_page_secret", nil)
	rr := httptest.NewRecorder()
	newTestServer(root, true).Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	for _, secret := range []string{"sk-page-secret", "PRIVATE_KEY="} {
		if strings.Contains(body, secret) {
			t.Fatalf("page leaked %q in body: %s", secret, body)
		}
	}
}
