// Package server implements the local-only ReasonForge Web Dashboard.
//
// The server is read-only. It reads sanitized RunEvent data from EventStore and
// exposes a small HTTP API plus HTML pages for local browser inspection.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/reasonforge/reasonforge/internal/config"
	"github.com/reasonforge/reasonforge/internal/events"
)

const (
	DefaultHost         = "127.0.0.1"
	DefaultPort         = 8765
	DefaultPollInterval = 2 * time.Second
)

// Options configures the local Web Dashboard server.
type Options struct {
	Host         string
	Port         int
	PollInterval time.Duration
}

// LocalServer serves the local Web Dashboard and read-only API.
type LocalServer struct {
	root    string
	cfg     *config.Root
	options Options
	handler http.Handler
}

// RunItem is the JSON shape returned by /api/runs.
type RunItem struct {
	RunID        string `json:"run_id"`
	State        string `json:"state"`
	Percent      int    `json:"percent"`
	CurrentPhase string `json:"current_phase"`
	LastEvent    string `json:"last_event"`
	StartedAt    string `json:"started_at,omitempty"`
	FinishedAt   string `json:"finished_at,omitempty"`
}

type runsResponse struct {
	Runs          []RunItem `json:"runs"`
	Message       string    `json:"message,omitempty"`
	EventsEnabled bool      `json:"events_enabled"`
}

type runDetailResponse struct {
	RunID    string               `json:"run_id"`
	State    string               `json:"state"`
	Progress events.ProgressState `json:"progress"`
	Timeline events.RunTimeline   `json:"timeline"`
}

type runEventsResponse struct {
	RunID  string            `json:"run_id"`
	Events []events.RunEvent `json:"events"`
}

// NewLocalServer creates a local dashboard server.
func NewLocalServer(root string, cfg *config.Root, opts Options) *LocalServer {
	if opts.Host == "" {
		opts.Host = DefaultHost
	}
	if opts.Port == 0 {
		opts.Port = DefaultPort
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = DefaultPollInterval
	}

	s := &LocalServer{
		root:    root,
		cfg:     cfg,
		options: opts,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/api/runs", s.handleAPIRuns)
	mux.HandleFunc("/api/runs/", s.handleAPIRunPath)
	mux.HandleFunc("/runs/", s.handleRunPage)
	mux.HandleFunc("/", s.handleHomePage)
	s.handler = mux
	return s
}

// Handler returns the HTTP handler. It is useful for tests.
func (s *LocalServer) Handler() http.Handler {
	return s.handler
}

// Address returns the host:port listen address.
func (s *LocalServer) Address() string {
	return fmt.Sprintf("%s:%d", s.options.Host, s.options.Port)
}

// URL returns the local HTTP URL for the server.
func (s *LocalServer) URL() string {
	return fmt.Sprintf("http://%s", s.Address())
}

// PollInterval returns the configured polling interval.
func (s *LocalServer) PollInterval() time.Duration {
	return s.options.PollInterval
}

// ListenAndServe starts the HTTP server.
func (s *LocalServer) ListenAndServe() error {
	return http.ListenAndServe(s.Address(), s.handler)
}

func (s *LocalServer) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *LocalServer) handleAPIRuns(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/runs" {
		s.handleAPIRunPath(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp, status := s.loadRunsResponse(r.Context())
	writeJSON(w, status, resp)
}

func (s *LocalServer) handleAPIRunPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		return
	}

	runID := parts[0]
	if len(parts) == 1 {
		resp, status := s.loadRunDetailResponse(r.Context(), runID)
		writeJSON(w, status, resp)
		return
	}
	if len(parts) == 2 && parts[1] == "events" {
		resp, status := s.loadRunEventsResponse(r.Context(), runID)
		writeJSON(w, status, resp)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
}

func (s *LocalServer) handleHomePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp, _ := s.loadRunsResponse(r.Context())
	data := homePageData{
		Runs:           resp.Runs,
		Message:        resp.Message,
		EventsEnabled:  resp.EventsEnabled,
		PollIntervalMS: int(s.options.PollInterval / time.Millisecond),
	}
	writeHTML(w, http.StatusOK, homeTemplate, data)
}

func (s *LocalServer) handleRunPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	runID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/runs/"), "/")
	if runID == "" || strings.Contains(runID, "/") {
		http.NotFound(w, r)
		return
	}

	resp, status := s.loadRunDetailResponse(r.Context(), runID)
	if status == http.StatusNotFound {
		writeHTML(w, http.StatusNotFound, messageTemplate, messagePageData{
			Title:   "Run Not Found",
			Message: "Run not found.",
		})
		return
	}
	if status != http.StatusOK {
		writeHTML(w, status, messageTemplate, messagePageData{
			Title:   "Dashboard Unavailable",
			Message: responseMessage(resp),
		})
		return
	}

	data := runPageData{
		Detail:         resp.(runDetailResponse),
		WorktreeID:     firstWorktreeID(resp.(runDetailResponse).Timeline.Events),
		PollIntervalMS: int(s.options.PollInterval / time.Millisecond),
	}
	writeHTML(w, http.StatusOK, runTemplate, data)
}

func (s *LocalServer) loadRunsResponse(ctx context.Context) (runsResponse, int) {
	if !s.cfg.Events.Enabled {
		return runsResponse{
			Runs:          []RunItem{},
			Message:       "Events system is disabled. Enable it in .reasonforge/events.yaml.",
			EventsEnabled: false,
		}, http.StatusOK
	}

	store, cleanup, missing, err := s.openStore()
	if err != nil {
		return runsResponse{Runs: []RunItem{}, Message: "Could not read event store.", EventsEnabled: true}, http.StatusInternalServerError
	}
	if missing {
		return runsResponse{Runs: []RunItem{}, Message: "No runs yet.", EventsEnabled: true}, http.StatusOK
	}
	defer cleanup()

	summaries, err := store.ListRuns(ctx)
	if err != nil {
		return runsResponse{Runs: []RunItem{}, Message: "Could not list runs.", EventsEnabled: true}, http.StatusInternalServerError
	}
	items := make([]RunItem, 0, len(summaries))
	for _, summary := range summaries {
		item := RunItem{
			RunID:      summary.RunID,
			State:      summary.State,
			LastEvent:  string(summary.LastEventType),
			StartedAt:  formatTime(summary.StartedAt),
			FinishedAt: formatTime(summary.FinishedAt),
		}
		timeline, err := sanitizedTimeline(ctx, store, summary.RunID)
		if err == nil && timeline.RunID != "" {
			progress := events.ComputeProgressState(timeline)
			item.Percent = progress.Percent
			item.CurrentPhase = progress.CurrentPhase
			item.State = progress.State
		}
		if item.Percent == 0 && summary.State != "running" {
			item.Percent = 100
		}
		items = append(items, item)
	}

	message := ""
	if len(items) == 0 {
		message = "No runs yet."
	}
	return runsResponse{Runs: items, Message: message, EventsEnabled: true}, http.StatusOK
}

func (s *LocalServer) loadRunDetailResponse(ctx context.Context, runID string) (any, int) {
	if !s.cfg.Events.Enabled {
		return map[string]string{"error": "events system is disabled"}, http.StatusServiceUnavailable
	}

	store, cleanup, missing, err := s.openStore()
	if err != nil {
		return map[string]string{"error": "could not read event store"}, http.StatusInternalServerError
	}
	if missing {
		return map[string]string{"error": "run not found"}, http.StatusNotFound
	}
	defer cleanup()

	timeline, err := sanitizedTimeline(ctx, store, runID)
	if err != nil {
		return map[string]string{"error": "could not read run"}, http.StatusInternalServerError
	}
	if timeline.RunID == "" {
		return map[string]string{"error": "run not found"}, http.StatusNotFound
	}

	progress := events.ComputeProgressState(timeline)
	return runDetailResponse{
		RunID:    timeline.RunID,
		State:    progress.State,
		Progress: progress,
		Timeline: timeline,
	}, http.StatusOK
}

func (s *LocalServer) loadRunEventsResponse(ctx context.Context, runID string) (any, int) {
	if !s.cfg.Events.Enabled {
		return map[string]string{"error": "events system is disabled"}, http.StatusServiceUnavailable
	}

	store, cleanup, missing, err := s.openStore()
	if err != nil {
		return map[string]string{"error": "could not read event store"}, http.StatusInternalServerError
	}
	if missing {
		return map[string]string{"error": "run not found"}, http.StatusNotFound
	}
	defer cleanup()

	evts, err := store.ListEvents(ctx, runID)
	if err != nil {
		return map[string]string{"error": "could not read events"}, http.StatusInternalServerError
	}
	if len(evts) == 0 {
		return map[string]string{"error": "run not found"}, http.StatusNotFound
	}
	return runEventsResponse{RunID: runID, Events: sanitizeEvents(evts)}, http.StatusOK
}

func (s *LocalServer) openStore() (*events.JSONLRunEventStore, func(), bool, error) {
	path := s.eventStorePath()
	store, err := events.OpenJSONLRunEventStoreReadOnly(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, func() {}, true, nil
		}
		return nil, nil, false, err
	}
	return store, func() { _ = store.Close() }, false, nil
}

func (s *LocalServer) eventStorePath() string {
	path := s.cfg.Events.StorePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.root, path)
	}
	return path
}

func sanitizedTimeline(ctx context.Context, store *events.JSONLRunEventStore, runID string) (events.RunTimeline, error) {
	evts, err := store.ListEvents(ctx, runID)
	if err != nil {
		return events.RunTimeline{}, err
	}
	if len(evts) == 0 {
		return events.RunTimeline{}, nil
	}
	return events.BuildTimeline(sanitizeEvents(evts)), nil
}

func sanitizeEvents(evts []events.RunEvent) []events.RunEvent {
	out := make([]events.RunEvent, 0, len(evts))
	for _, evt := range evts {
		out = append(out, events.SanitizeEvent(evt))
	}
	return out
}

func firstWorktreeID(evts []events.RunEvent) string {
	for i := len(evts) - 1; i >= 0; i-- {
		if evts[i].WorktreeID != "" {
			return evts[i].WorktreeID
		}
	}
	return ""
}

func responseMessage(resp any) string {
	if m, ok := resp.(map[string]string); ok && m["error"] != "" {
		return m["error"]
	}
	return "Dashboard unavailable."
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeHTML(w http.ResponseWriter, status int, tmpl *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = tmpl.Execute(w, data)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
