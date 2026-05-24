package events

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// JSONLRunEventStore implements EventStore with append-only JSONL persistence.
//
// Default path: .reasonforge/events/run_events.jsonl
//
// Safety guarantees:
//   - Append-only: no update or delete operations.
//   - Directory permissions 0700, file permissions 0600.
//   - Event IDs use crypto/rand (enforced by GenerateEventID).
//   - Corrupted lines are skipped with a warning; they never cause a panic.
//   - SanitizeEvent() must be called before Append to ensure redaction.
type JSONLRunEventStore struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	writer   *bufio.Writer
	contents []RunEvent // in-memory cache for queries
	loaded   bool
}

// NewJSONLRunEventStore creates a new JSONLRunEventStore.
// It creates the directory (0700) and file (0600) if they don't exist.
func NewJSONLRunEventStore(path string) (*JSONLRunEventStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("events: create directory %s: %w", dir, err)
	}

	// Check if file exists
	_, statErr := os.Stat(path)

	// Open for append; create if missing
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("events: open file %s: %w", path, err)
	}

	store := &JSONLRunEventStore{
		path:   path,
		file:   f,
		writer: bufio.NewWriter(f),
	}

	// Load existing events if file existed
	if statErr == nil {
		if err := store.loadExisting(); err != nil {
			f.Close()
			return nil, fmt.Errorf("events: load existing events: %w", err)
		}
	}

	return store, nil
}

// loadExisting reads all existing events from the JSONL file into memory.
// Uses line-by-line reading so that corrupted lines can be safely skipped.
func (s *JSONLRunEventStore) loadExisting() error {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open for reading: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Allow lines up to 1MB
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	corruptedLines := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var evt RunEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			corruptedLines++
			log.Printf("events: skipping corrupted line in %s: %v", s.path, err)
			continue
		}
		s.contents = append(s.contents, evt)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("events: scanner error in %s: %v", s.path, err)
	}

	if corruptedLines > 0 {
		log.Printf("events: %d corrupted lines skipped in %s", corruptedLines, s.path)
	}

	s.loaded = true
	return nil
}

// Append persists an event to the JSONL store (append-only).
// SanitizeEvent() should be called before Append to ensure redaction.
// Events with an empty RunID are rejected to prevent polluting run
// aggregation with orphan events.
func (s *JSONLRunEventStore) Append(ctx context.Context, event RunEvent) error {
	if strings.TrimSpace(event.RunID) == "" {
		return fmt.Errorf("events: reject event with empty run_id (type=%s)", event.Type)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("events: marshal event: %w", err)
	}

	if _, err := s.writer.Write(data); err != nil {
		return fmt.Errorf("events: write event: %w", err)
	}
	if _, err := s.writer.Write([]byte("\n")); err != nil {
		return fmt.Errorf("events: write newline: %w", err)
	}

	if err := s.writer.Flush(); err != nil {
		return fmt.Errorf("events: flush: %w", err)
	}

	s.contents = append(s.contents, event)
	return nil
}

// Write implements EventSink. It delegates to Append.
func (s *JSONLRunEventStore) Write(ctx context.Context, event RunEvent) error {
	return s.Append(ctx, event)
}

// Close flushes and closes the underlying file.
func (s *JSONLRunEventStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.writer != nil {
		if err := s.writer.Flush(); err != nil {
			return fmt.Errorf("events: final flush: %w", err)
		}
	}
	if s.file != nil {
		if err := s.file.Close(); err != nil {
			return fmt.Errorf("events: close file: %w", err)
		}
		s.file = nil
		s.writer = nil
	}
	return nil
}

// ListRuns returns a summary of all runs, sorted by StartedAt descending.
func (s *JSONLRunEventStore) ListRuns(ctx context.Context) ([]RunSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Group events by RunID
	runEvents := make(map[string][]RunEvent)
	for _, evt := range s.contents {
		if strings.TrimSpace(evt.RunID) == "" {
			continue
		}
		runEvents[evt.RunID] = append(runEvents[evt.RunID], evt)
	}

	var summaries []RunSummary
	for runID, events := range runEvents {
		summary := buildRunSummary(runID, events)
		summaries = append(summaries, summary)
	}

	// Sort by StartedAt descending (most recent first)
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].StartedAt.After(summaries[j].StartedAt)
	})

	return summaries, nil
}

// ListEvents returns all events for a given run, ordered by time.
func (s *JSONLRunEventStore) ListEvents(ctx context.Context, runID string) ([]RunEvent, error) {
	if strings.TrimSpace(runID) == "" {
		return nil, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var result []RunEvent
	for _, evt := range s.contents {
		if evt.RunID == runID {
			result = append(result, evt)
		}
	}

	// Sort by StartedAt
	sort.Slice(result, func(i, j int) bool {
		// Use StartedAt; if zero, fall back to FinishedAt
		ti := result[i].StartedAt
		if ti.IsZero() {
			ti = result[i].FinishedAt
		}
		tj := result[j].StartedAt
		if tj.IsZero() {
			tj = result[j].FinishedAt
		}
		return ti.Before(tj)
	})

	return result, nil
}

// GetTimeline reconstructs the timeline for a given run.
func (s *JSONLRunEventStore) GetTimeline(ctx context.Context, runID string) (RunTimeline, error) {
	events, err := s.ListEvents(ctx, runID)
	if err != nil {
		return RunTimeline{}, err
	}

	if len(events) == 0 {
		return RunTimeline{}, nil
	}

	return BuildTimeline(events), nil
}

// buildRunSummary creates a RunSummary from a list of events for a single run.
// The LastEventType/LastMessage are derived from the last event in append order,
// which represents the natural chronological order of events.
func buildRunSummary(runID string, events []RunEvent) RunSummary {
	if len(events) == 0 {
		return RunSummary{RunID: runID}
	}

	// Last event in append order = most recent event
	lastEvent := events[len(events)-1]

	// Find earliest timestamp for StartedAt
	var startedAt time.Time
	for _, evt := range events {
		t := evt.StartedAt
		if t.IsZero() {
			t = evt.FinishedAt
		}
		if !t.IsZero() && (startedAt.IsZero() || t.Before(startedAt)) {
			startedAt = t
		}
	}

	// Determine state from terminal events
	state := "running"
	var finishedAt time.Time

	for _, evt := range events {
		if evt.Type.IsTerminal() {
			switch evt.Type {
			case EventRunSucceeded:
				state = "succeeded"
			case EventRunFailed:
				state = "failed"
			case EventRunCancelled:
				state = "cancelled"
			}
			finishedAt = evt.FinishedAt
		}
	}

	return RunSummary{
		RunID:         runID,
		TaskID:        events[0].TaskID,
		State:         state,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		EventCount:    len(events),
		LastEventType: lastEvent.Type,
		LastMessage:   lastEvent.Message,
	}
}

// DefaultEventStorePath returns the default path for the event store.
func DefaultEventStorePath(repoRoot string) string {
	return filepath.Join(repoRoot, ".reasonforge", "events", "run_events.jsonl")
}

// CorruptedLineInfo tracks corrupted lines encountered during load.
type CorruptedLineInfo struct {
	Line  int
	Error string
}

// LoadEventsFromFile reads and parses events from a JSONL file,
// returning events and info about any corrupted lines.
// This is useful for diagnostics and testing.
func LoadEventsFromFile(path string) ([]RunEvent, []CorruptedLineInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var events []RunEvent
	var corrupted []CorruptedLineInfo
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var evt RunEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			corrupted = append(corrupted, CorruptedLineInfo{
				Line:  lineNum,
				Error: err.Error(),
			})
			continue
		}
		events = append(events, evt)
	}

	if err := scanner.Err(); err != nil {
		corrupted = append(corrupted, CorruptedLineInfo{
			Line:  lineNum + 1,
			Error: fmt.Sprintf("scanner error: %v", err),
		})
	}

	return events, corrupted, nil
}

// Ensure JSONLRunEventStore implements EventStore and EventSink.
var _ EventStore = (*JSONLRunEventStore)(nil)
var _ EventSink = (*JSONLRunEventStore)(nil)

// NoopEventStore is an EventStore that discards all events.
// Useful for testing or when events are disabled.
type NoopEventStore struct{}

// Append discards the event. Returns an error if the event has an empty
// RunID, to enforce consistency.
func (n *NoopEventStore) Append(ctx context.Context, event RunEvent) error {
	if strings.TrimSpace(event.RunID) == "" {
		return fmt.Errorf("events: reject event with empty run_id (type=%s)", event.Type)
	}
	return nil
}

// ListRuns returns an empty list.
func (n *NoopEventStore) ListRuns(ctx context.Context) ([]RunSummary, error) { return nil, nil }

// ListEvents returns an empty list.
func (n *NoopEventStore) ListEvents(ctx context.Context, runID string) ([]RunEvent, error) {
	return nil, nil
}

// GetTimeline returns an empty timeline.
func (n *NoopEventStore) GetTimeline(ctx context.Context, runID string) (RunTimeline, error) {
	return RunTimeline{}, nil
}

// Write discards the event after applying the same validation as Append.
func (n *NoopEventStore) Write(ctx context.Context, event RunEvent) error {
	return n.Append(ctx, event)
}

// Ensure NoopEventStore implements EventStore and EventSink.
var _ EventStore = (*NoopEventStore)(nil)
var _ EventSink = (*NoopEventStore)(nil)

// apiKeyPatterns are substrings that indicate potential API key leakage.
var apiKeyPatterns = []string{
	"API_KEY",
	"SECRET",
	"TOKEN",
	"PASSWORD",
	"PRIVATE_KEY",
	"sk-",
	"sk_live_",
	"pk_live_",
	"AKIA",
}

// safeMetadataKeys is the whitelist of metadata keys that are safe to persist.
var safeMetadataKeys = map[string]bool{
	"path":           true,
	"command_name":   true,
	"tool_name":      true,
	"model":          true,
	"provider":       true,
	"risk_level":     true,
	"recommendation": true,
	"worktree_id":    true,
	"step_index":     true,
	"iteration":      true,
	"files_changed":  true,
	"additions":      true,
	"deletions":      true,
}

// redactPatterns are substrings that trigger full-line redaction in message/error fields.
var redactPatterns = []string{
	"API_KEY",
	"SECRET",
	"TOKEN",
	"PASSWORD",
	"PRIVATE_KEY",
	"sk-",
	"AKIA",
	"content",
	"old",
	"new",
	"patch",
	"diff",
	"stdin",
}

// SanitizeEvent redacts sensitive data from a RunEvent before persistence.
//
// Rules:
//  1. Metadata keys not in the safe whitelist are replaced with "<redacted>".
//  2. Message/Error fields containing API key patterns are fully redacted.
//  3. Content, old, new, patch, diff, stdin patterns in Message/Error are redacted.
//  4. Events with violations in patch diff are stripped of diff content.
//  5. Only summaries are recorded, not full file content.
func SanitizeEvent(event RunEvent) RunEvent {
	// Sanitize metadata
	if len(event.Metadata) > 0 {
		sanitized := make(map[string]string, len(event.Metadata))
		for k, v := range event.Metadata {
			if safeMetadataKeys[k] {
				// Check value for sensitive patterns
				if containsSensitivePattern(v) {
					sanitized[k] = "<redacted>"
				} else {
					sanitized[k] = v
				}
			} else {
				sanitized[k] = "<redacted>"
			}
		}
		event.Metadata = sanitized
	}

	// Sanitize message
	event.Message = sanitizeField(event.Message)

	// Sanitize error
	event.Error = sanitizeField(event.Error)

	return event
}

// sanitizeField checks a string field for sensitive patterns and redacts
// individual lines that contain them.
func sanitizeField(s string) string {
	if s == "" {
		return s
	}

	lines := strings.Split(s, "\n")
	modified := false

	for i, line := range lines {
		if containsSensitivePattern(line) {
			lines[i] = "<redacted>"
			modified = true
		}
	}

	if modified {
		return strings.Join(lines, "\n")
	}
	return s
}

// containsSensitivePattern checks if a string contains any sensitive pattern.
func containsSensitivePattern(s string) bool {
	upper := strings.ToUpper(s)
	for _, pattern := range apiKeyPatterns {
		if strings.Contains(upper, strings.ToUpper(pattern)) {
			return true
		}
	}
	for _, pattern := range redactPatterns {
		if strings.Contains(upper, strings.ToUpper(pattern)) {
			return true
		}
	}
	return false
}
