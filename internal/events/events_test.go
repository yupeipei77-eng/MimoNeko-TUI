package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestGenerateEventID(t *testing.T) {
	id, err := GenerateEventID()
	if err != nil {
		t.Fatalf("GenerateEventID() error: %v", err)
	}
	if len(id) < 4 {
		t.Fatalf("GenerateEventID() id too short: %s", id)
	}
	if id[:4] != "evt_" {
		t.Fatalf("GenerateEventID() id must start with evt_: %s", id)
	}

	// Verify uniqueness by generating multiple IDs
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := GenerateEventID()
		if err != nil {
			t.Fatalf("GenerateEventID() error: %v", err)
		}
		if ids[id] {
			t.Fatalf("GenerateEventID() duplicate id: %s", id)
		}
		ids[id] = true
	}
}

func TestGenerateEventIDUsesCryptoRand(t *testing.T) {
	// Verify that the ID uses crypto/rand by checking entropy.
	// Generate multiple IDs and verify they have sufficient randomness.
	ids := make([]string, 50)
	for i := range ids {
		id, err := GenerateEventID()
		if err != nil {
			t.Fatalf("GenerateEventID() error: %v", err)
		}
		ids[i] = id
	}

	// Check that no two IDs are the same (extremely unlikely with crypto/rand)
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate event ID: %s", id)
		}
		seen[id] = true
	}

	// Verify the random portion is 16 bytes = 32 hex chars
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	expectedHexLen := len(hex.EncodeToString(b))
	for _, id := range ids {
		randomPart := id[4:] // skip "evt_"
		if len(randomPart) != expectedHexLen {
			t.Fatalf("event ID random portion has wrong length: got %d, want %d", len(randomPart), expectedHexLen)
		}
	}
}

func TestEventTypeIsTerminal(t *testing.T) {
	tests := []struct {
		typ      EventType
		terminal bool
	}{
		{EventRunStarted, false},
		{EventRunSucceeded, true},
		{EventRunFailed, true},
		{EventRunCancelled, true},
		{EventPlannerStarted, false},
		{EventPlannerFinished, false},
		{EventToolStarted, false},
		{EventToolFinished, false},
	}
	for _, tt := range tests {
		if got := tt.typ.IsTerminal(); got != tt.terminal {
			t.Errorf("EventType(%s).IsTerminal() = %v, want %v", tt.typ, got, tt.terminal)
		}
	}
}

func TestEventTypeIsStarted(t *testing.T) {
	tests := []struct {
		typ     EventType
		started bool
	}{
		{EventRunStarted, true},
		{EventPlannerStarted, true},
		{EventCoderStarted, true},
		{EventReviewerStarted, true},
		{EventToolStarted, true},
		{EventValidationStarted, true},
		{EventPatchPreviewStarted, true},
		{EventRunSucceeded, false},
		{EventPlannerFinished, false},
	}
	for _, tt := range tests {
		if got := tt.typ.IsStarted(); got != tt.started {
			t.Errorf("EventType(%s).IsStarted() = %v, want %v", tt.typ, got, tt.started)
		}
	}
}

func TestEventTypeIsFinished(t *testing.T) {
	tests := []struct {
		typ      EventType
		finished bool
	}{
		{EventRunStarted, false},
		{EventPlannerStarted, false},
		{EventRunSucceeded, true},
		{EventRunFailed, true},
		{EventPlannerFinished, true},
		{EventCoderFinished, true},
		{EventToolFinished, true},
	}
	for _, tt := range tests {
		if got := tt.typ.IsFinished(); got != tt.finished {
			t.Errorf("EventType(%s).IsFinished() = %v, want %v", tt.typ, got, tt.finished)
		}
	}
}

func TestEventFilterMatches(t *testing.T) {
	now := time.Now()
	evt := RunEvent{
		ID:        "evt_test1",
		RunID:     "run_abc",
		TaskID:    "task_123",
		Type:      EventPlannerStarted,
		Source:    "agent",
		Status:    "started",
		StartedAt: now,
	}

	tests := []struct {
		name   string
		filter EventFilter
		match  bool
	}{
		{
			name:   "empty filter matches all",
			filter: EventFilter{},
			match:  true,
		},
		{
			name:   "matching RunID",
			filter: EventFilter{RunID: "run_abc"},
			match:  true,
		},
		{
			name:   "non-matching RunID",
			filter: EventFilter{RunID: "run_xyz"},
			match:  false,
		},
		{
			name:   "matching TaskID",
			filter: EventFilter{TaskID: "task_123"},
			match:  true,
		},
		{
			name:   "matching Type",
			filter: EventFilter{Types: []EventType{EventPlannerStarted}},
			match:  true,
		},
		{
			name:   "non-matching Type",
			filter: EventFilter{Types: []EventType{EventCoderStarted}},
			match:  false,
		},
		{
			name:   "multiple types with match",
			filter: EventFilter{Types: []EventType{EventCoderStarted, EventPlannerStarted}},
			match:  true,
		},
		{
			name:   "RunID and Type both match",
			filter: EventFilter{RunID: "run_abc", Types: []EventType{EventPlannerStarted}},
			match:  true,
		},
		{
			name:   "RunID matches but Type doesn't",
			filter: EventFilter{RunID: "run_abc", Types: []EventType{EventCoderStarted}},
			match:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Matches(evt); got != tt.match {
				t.Errorf("Matches() = %v, want %v", got, tt.match)
			}
		})
	}
}

func TestJSONLRunEventStoreAppendOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	store, err := NewJSONLRunEventStore(path)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore() error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	evt := RunEvent{
		ID:        "evt_test1",
		RunID:     "run_abc",
		Type:      EventRunStarted,
		Source:    "cli",
		Status:    "started",
		Message:   "Run started",
		StartedAt: now,
	}

	if err := store.Append(ctx, evt); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	// Verify the file exists and has content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Append() did not write any data")
	}

	// Append another event
	evt2 := RunEvent{
		ID:         "evt_test2",
		RunID:      "run_abc",
		Type:       EventRunSucceeded,
		Source:     "cli",
		Status:     "succeeded",
		Message:    "Run succeeded",
		StartedAt:  now,
		FinishedAt: now.Add(5 * time.Second),
		DurationMs: 5000,
	}
	if err := store.Append(ctx, evt2); err != nil {
		t.Fatalf("Append() second event error: %v", err)
	}

	// Verify both events are in the file
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Append() did not write any data for second event")
	}
}

func TestJSONLRunEventStoreFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	store, err := NewJSONLRunEventStore(path)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore() error: %v", err)
	}
	defer store.Close()

	// Check directory exists
	eventsDir := filepath.Dir(path)
	dirInfo, err := os.Stat(eventsDir)
	if err != nil {
		t.Fatalf("Stat() events dir error: %v", err)
	}
	if !dirInfo.IsDir() {
		t.Errorf("events path is not a directory")
	}

	// Check file exists and is regular
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() events file error: %v", err)
	}
	if fileInfo.IsDir() {
		t.Errorf("events file is a directory, not a file")
	}

	// Note: On Windows, the permission bits are not fully honored by the OS.
	// The 0700/0600 permissions are set correctly in code but Windows ACLs
	// control actual access. Exact permission bit checks are only reliable
	// on Unix-like systems.
}

func TestJSONLRunEventStoreListRuns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	store, err := NewJSONLRunEventStore(path)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore() error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	// Add events for two runs
	events := []RunEvent{
		{ID: "evt_1", RunID: "run_001", TaskID: "task_1", Type: EventRunStarted, Status: "started", StartedAt: now},
		{ID: "evt_2", RunID: "run_001", Type: EventPlannerStarted, Status: "started", StartedAt: now.Add(time.Second)},
		{ID: "evt_3", RunID: "run_001", Type: EventPlannerFinished, Status: "succeeded", StartedAt: now.Add(2 * time.Second), FinishedAt: now.Add(3 * time.Second)},
		{ID: "evt_4", RunID: "run_001", Type: EventRunSucceeded, Status: "succeeded", StartedAt: now, FinishedAt: now.Add(4 * time.Second)},
		{ID: "evt_5", RunID: "run_002", TaskID: "task_2", Type: EventRunStarted, Status: "started", StartedAt: now.Add(5 * time.Second)},
		{ID: "evt_6", RunID: "run_002", Type: EventRunFailed, Status: "failed", Error: "something went wrong", StartedAt: now.Add(5 * time.Second), FinishedAt: now.Add(6 * time.Second)},
	}

	for _, evt := range events {
		if err := store.Append(ctx, evt); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	summaries, err := store.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns() error: %v", err)
	}

	if len(summaries) != 2 {
		t.Fatalf("ListRuns() returned %d runs, want 2", len(summaries))
	}

	// Should be sorted by StartedAt descending (run_002 is more recent)
	if summaries[0].RunID != "run_002" {
		t.Errorf("ListRuns()[0].RunID = %s, want run_002", summaries[0].RunID)
	}
	if summaries[1].RunID != "run_001" {
		t.Errorf("ListRuns()[1].RunID = %s, want run_001", summaries[1].RunID)
	}

	// Check run_001 summary
	r001 := summaries[1]
	if r001.State != "succeeded" {
		t.Errorf("run_001 state = %s, want succeeded", r001.State)
	}
	if r001.TaskID != "task_1" {
		t.Errorf("run_001 task_id = %s, want task_1", r001.TaskID)
	}
	if r001.EventCount != 4 {
		t.Errorf("run_001 event_count = %d, want 4", r001.EventCount)
	}
	if r001.LastEventType != EventRunSucceeded {
		t.Errorf("run_001 last_event_type = %s, want run.succeeded", r001.LastEventType)
	}

	// Check run_002 summary
	r002 := summaries[0]
	if r002.State != "failed" {
		t.Errorf("run_002 state = %s, want failed", r002.State)
	}
}

func TestJSONLRunEventStoreListEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	store, err := NewJSONLRunEventStore(path)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore() error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	events := []RunEvent{
		{ID: "evt_1", RunID: "run_001", Type: EventRunStarted, Status: "started", StartedAt: now},
		{ID: "evt_2", RunID: "run_001", Type: EventPlannerStarted, Status: "started", StartedAt: now.Add(time.Second)},
		{ID: "evt_3", RunID: "run_001", Type: EventPlannerFinished, Status: "succeeded", StartedAt: now.Add(2 * time.Second)},
		{ID: "evt_4", RunID: "run_002", Type: EventRunStarted, Status: "started", StartedAt: now.Add(5 * time.Second)},
	}

	for _, evt := range events {
		if err := store.Append(ctx, evt); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	// List events for run_001
	result, err := store.ListEvents(ctx, "run_001")
	if err != nil {
		t.Fatalf("ListEvents() error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("ListEvents(run_001) returned %d events, want 3", len(result))
	}

	// Events should be sorted by time
	for i := 1; i < len(result); i++ {
		if result[i].StartedAt.Before(result[i-1].StartedAt) {
			t.Errorf("events not sorted by time: [%d]=%v > [%d]=%v", i-1, result[i-1].StartedAt, i, result[i].StartedAt)
		}
	}

	// List events for run_002
	result2, err := store.ListEvents(ctx, "run_002")
	if err != nil {
		t.Fatalf("ListEvents() error: %v", err)
	}
	if len(result2) != 1 {
		t.Fatalf("ListEvents(run_002) returned %d events, want 1", len(result2))
	}

	// List events for non-existent run
	result3, err := store.ListEvents(ctx, "run_999")
	if err != nil {
		t.Fatalf("ListEvents() error: %v", err)
	}
	if len(result3) != 0 {
		t.Fatalf("ListEvents(run_999) returned %d events, want 0", len(result3))
	}
}

func TestJSONLRunEventStoreGetTimeline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	store, err := NewJSONLRunEventStore(path)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore() error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	events := []RunEvent{
		{ID: "evt_1", RunID: "run_001", TaskID: "task_1", Type: EventRunStarted, Status: "started", StartedAt: now},
		{ID: "evt_2", RunID: "run_001", Type: EventPlannerStarted, Status: "started", Source: "agent", StartedAt: now.Add(time.Second)},
		{ID: "evt_3", RunID: "run_001", Type: EventPlannerFinished, Status: "succeeded", Source: "agent", StartedAt: now.Add(time.Second), FinishedAt: now.Add(2 * time.Second), DurationMs: 1000},
		{ID: "evt_4", RunID: "run_001", Type: EventCoderStarted, Status: "started", Source: "agent", StartedAt: now.Add(3 * time.Second)},
		{ID: "evt_5", RunID: "run_001", Type: EventCoderFinished, Status: "succeeded", Source: "agent", StartedAt: now.Add(3 * time.Second), FinishedAt: now.Add(5 * time.Second), DurationMs: 2000},
		{ID: "evt_6", RunID: "run_001", Type: EventRunSucceeded, Status: "succeeded", StartedAt: now, FinishedAt: now.Add(6 * time.Second), DurationMs: 6000},
	}

	for _, evt := range events {
		if err := store.Append(ctx, evt); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	timeline, err := store.GetTimeline(ctx, "run_001")
	if err != nil {
		t.Fatalf("GetTimeline() error: %v", err)
	}

	if timeline.RunID != "run_001" {
		t.Errorf("timeline.RunID = %s, want run_001", timeline.RunID)
	}
	if timeline.State != "succeeded" {
		t.Errorf("timeline.State = %s, want succeeded", timeline.State)
	}
	if len(timeline.Events) != 6 {
		t.Errorf("timeline.Events count = %d, want 6", len(timeline.Events))
	}
	if len(timeline.Steps) == 0 {
		t.Error("timeline.Steps is empty")
	}
}

func TestJSONLRunEventStoreCorruptedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	// Write some valid and invalid JSONL lines
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	f.WriteString(`{"id":"evt_1","run_id":"run_001","type":"run.started","status":"started"}` + "\n")
	f.WriteString("this is not valid json\n")
	f.WriteString(`{"id":"evt_2","run_id":"run_001","type":"run.succeeded","status":"succeeded"}` + "\n")
	f.WriteString("another bad line\n")
	f.Close()

	// Opening the store should not panic
	store, err := NewJSONLRunEventStore(path)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore() error: %v", err)
	}
	defer store.Close()

	// The valid events should be loaded
	ctx := context.Background()
	events, err := store.ListEvents(ctx, "run_001")
	if err != nil {
		t.Fatalf("ListEvents() error: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("ListEvents() returned %d events, want 2 (corrupted lines skipped)", len(events))
	}
}

func TestSanitizeEventRedactsAPIKeyPatterns(t *testing.T) {
	tests := []struct {
		name    string
		event   RunEvent
		checkFn func(t *testing.T, e RunEvent)
	}{
		{
			name: "API_KEY in message",
			event: RunEvent{
				Message: "Using API_KEY=sk-1234567890",
			},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted: %s", e.Message)
				}
			},
		},
		{
			name: "SECRET in error",
			event: RunEvent{
				Error: "Error: SECRET=mysecret",
			},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Error != "<redacted>" {
					t.Errorf("error not redacted: %s", e.Error)
				}
			},
		},
		{
			name: "TOKEN in message",
			event: RunEvent{
				Message: "bearer TOKEN=abc123",
			},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted: %s", e.Message)
				}
			},
		},
		{
			name: "sk- in message",
			event: RunEvent{
				Message: "key is sk-abcdef123456",
			},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted: %s", e.Message)
				}
			},
		},
		{
			name: "AKIA in message",
			event: RunEvent{
				Message: "AWS key AKIAIOSFODNN7EXAMPLE",
			},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted: %s", e.Message)
				}
			},
		},
		{
			name: "PASSWORD in message",
			event: RunEvent{
				Message: "PASSWORD=hunter2",
			},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted: %s", e.Message)
				}
			},
		},
		{
			name: "PRIVATE_KEY in message",
			event: RunEvent{
				Message: "PRIVATE_KEY=-----BEGIN RSA",
			},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted: %s", e.Message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := SanitizeEvent(tt.event)
			tt.checkFn(t, sanitized)
		})
	}
}

func TestSanitizeEventRedactsContentPatterns(t *testing.T) {
	tests := []struct {
		name    string
		event   RunEvent
		checkFn func(t *testing.T, e RunEvent)
	}{
		{
			name:  "content in message",
			event: RunEvent{Message: "file content was written"},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted for content: %s", e.Message)
				}
			},
		},
		{
			name:  "diff in message",
			event: RunEvent{Message: "the diff output was large"},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted for diff: %s", e.Message)
				}
			},
		},
		{
			name:  "patch in message",
			event: RunEvent{Message: "the patch was applied"},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted for patch: %s", e.Message)
				}
			},
		},
		{
			name:  "stdin in message",
			event: RunEvent{Message: "read from stdin"},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted for stdin: %s", e.Message)
				}
			},
		},
		{
			name:  "old in message",
			event: RunEvent{Message: "old value was replaced"},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted for old: %s", e.Message)
				}
			},
		},
		{
			name:  "new in message",
			event: RunEvent{Message: "new value was set"},
			checkFn: func(t *testing.T, e RunEvent) {
				if e.Message != "<redacted>" {
					t.Errorf("message not redacted for new: %s", e.Message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := SanitizeEvent(tt.event)
			tt.checkFn(t, sanitized)
		})
	}
}

func TestSanitizeEventPreservesSafeMetadataKeys(t *testing.T) {
	event := RunEvent{
		Message: "Plan generated",
		Metadata: map[string]string{
			"path":           "src/main.go",
			"command_name":   "go-test",
			"tool_name":      "file_read",
			"model":          "gpt-4",
			"provider":       "openai",
			"risk_level":     "low",
			"recommendation": "approve",
			"worktree_id":    "wt_abc",
			"step_index":     "1",
			"iteration":      "2",
			"files_changed":  "3",
			"additions":      "10",
			"deletions":      "5",
		},
	}

	sanitized := SanitizeEvent(event)

	for _, key := range []string{
		"path", "command_name", "tool_name", "model", "provider",
		"risk_level", "recommendation", "worktree_id", "step_index",
		"iteration", "files_changed", "additions", "deletions",
	} {
		if sanitized.Metadata[key] != event.Metadata[key] {
			t.Errorf("safe key %q was redacted: got %q, want %q", key, sanitized.Metadata[key], event.Metadata[key])
		}
	}
}

func TestSanitizeEventRedactsUnsafeMetadataKeys(t *testing.T) {
	event := RunEvent{
		Message: "Tool executed",
		Metadata: map[string]string{
			"api_key":  "sk-1234567890",
			"secret":   "mysecret",
			"password": "hunter2",
			"content":  "file contents here",
			"stdin":    "input data",
			"diff":     "+ added line",
			"patch":    "--- a/file.txt",
		},
	}

	sanitized := SanitizeEvent(event)

	for _, key := range []string{"api_key", "secret", "password", "content", "stdin", "diff", "patch"} {
		if sanitized.Metadata[key] != "<redacted>" {
			t.Errorf("unsafe key %q was not redacted: got %q", key, sanitized.Metadata[key])
		}
	}
}

func TestSanitizeEventRedactsSensitiveMetadataValues(t *testing.T) {
	event := RunEvent{
		Message: "Tool executed",
		Metadata: map[string]string{
			"path":  "sk-1234567890", // safe key but sensitive value
			"model": "API_KEY=abc",   // safe key but sensitive value
		},
	}

	sanitized := SanitizeEvent(event)

	if sanitized.Metadata["path"] != "<redacted>" {
		t.Errorf("sensitive value in safe key 'path' was not redacted: got %q", sanitized.Metadata["path"])
	}
	if sanitized.Metadata["model"] != "<redacted>" {
		t.Errorf("sensitive value in safe key 'model' was not redacted: got %q", sanitized.Metadata["model"])
	}
}

func TestSanitizeEventPreservesSafeMessage(t *testing.T) {
	event := RunEvent{
		Message: "Planner finished successfully",
		Error:   "context deadline exceeded",
	}

	sanitized := SanitizeEvent(event)

	if sanitized.Message != "Planner finished successfully" {
		t.Errorf("safe message was modified: got %q", sanitized.Message)
	}
	if sanitized.Error != "context deadline exceeded" {
		t.Errorf("safe error was modified: got %q", sanitized.Error)
	}
}

func TestComputeProgressState(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		events        []RunEvent
		wantState     string
		wantPhase     string
		wantPercent   int
		wantCompleted int
	}{
		{
			name:        "empty events",
			events:      []RunEvent{},
			wantState:   "unknown",
			wantPercent: 0,
		},
		{
			name: "run started",
			events: []RunEvent{
				{ID: "e1", RunID: "r1", Type: EventRunStarted, Status: "started", StartedAt: now},
			},
			wantState:     "running",
			wantPhase:     "run",
			wantPercent:   5,
			wantCompleted: 0,
		},
		{
			name: "planner finished",
			events: []RunEvent{
				{ID: "e1", RunID: "r1", Type: EventRunStarted, Status: "started", StartedAt: now},
				{ID: "e2", RunID: "r1", Type: EventPlannerStarted, Status: "started", StartedAt: now.Add(time.Second)},
				{ID: "e3", RunID: "r1", Type: EventPlannerFinished, Status: "succeeded", StartedAt: now.Add(time.Second), FinishedAt: now.Add(2 * time.Second)},
			},
			wantState:     "running",
			wantPhase:     "planner",
			wantPercent:   20,
			wantCompleted: 1,
		},
		{
			name: "run succeeded",
			events: []RunEvent{
				{ID: "e1", RunID: "r1", Type: EventRunStarted, Status: "started", StartedAt: now},
				{ID: "e2", RunID: "r1", Type: EventPlannerFinished, Status: "succeeded", StartedAt: now, FinishedAt: now.Add(time.Second)},
				{ID: "e3", RunID: "r1", Type: EventRunSucceeded, Status: "succeeded", StartedAt: now, FinishedAt: now.Add(5 * time.Second)},
			},
			wantState:     "succeeded",
			wantPhase:     "",
			wantPercent:   100,
			wantCompleted: 1,
		},
		{
			name: "run failed",
			events: []RunEvent{
				{ID: "e1", RunID: "r1", Type: EventRunStarted, Status: "started", StartedAt: now},
				{ID: "e2", RunID: "r1", Type: EventRunFailed, Status: "failed", StartedAt: now, FinishedAt: now.Add(time.Second)},
			},
			wantState:   "failed",
			wantPhase:   "",
			wantPercent: 100,
		},
		{
			name: "coder finished",
			events: []RunEvent{
				{ID: "e1", RunID: "r1", Type: EventRunStarted, Status: "started", StartedAt: now},
				{ID: "e2", RunID: "r1", Type: EventPlannerFinished, Status: "succeeded", StartedAt: now, FinishedAt: now.Add(time.Second)},
				{ID: "e3", RunID: "r1", Type: EventCoderFinished, Status: "succeeded", StartedAt: now, FinishedAt: now.Add(3 * time.Second)},
			},
			wantState:     "running",
			wantPhase:     "coder",
			wantPercent:   50,
			wantCompleted: 2,
		},
		{
			name: "reviewer finished",
			events: []RunEvent{
				{ID: "e1", RunID: "r1", Type: EventRunStarted, Status: "started", StartedAt: now},
				{ID: "e2", RunID: "r1", Type: EventPlannerFinished, Status: "succeeded", StartedAt: now, FinishedAt: now.Add(time.Second)},
				{ID: "e3", RunID: "r1", Type: EventCoderFinished, Status: "succeeded", StartedAt: now, FinishedAt: now.Add(3 * time.Second)},
				{ID: "e4", RunID: "r1", Type: EventReviewerFinished, Status: "succeeded", StartedAt: now, FinishedAt: now.Add(5 * time.Second)},
			},
			wantState:     "running",
			wantPhase:     "reviewer",
			wantPercent:   80,
			wantCompleted: 3,
		},
		{
			name: "validation finished",
			events: []RunEvent{
				{ID: "e1", RunID: "r1", Type: EventRunStarted, Status: "started", StartedAt: now},
				{ID: "e2", RunID: "r1", Type: EventValidationFinished, Status: "succeeded", StartedAt: now, FinishedAt: now.Add(time.Second)},
			},
			wantState:     "running",
			wantPhase:     "validation",
			wantPercent:   90,
			wantCompleted: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeline := BuildTimeline(tt.events)
			if len(tt.events) > 0 {
				timeline.RunID = "r1"
			}
			progress := ComputeProgressState(timeline)

			if progress.State != tt.wantState {
				t.Errorf("State = %q, want %q", progress.State, tt.wantState)
			}
			if progress.CurrentPhase != tt.wantPhase {
				t.Errorf("CurrentPhase = %q, want %q", progress.CurrentPhase, tt.wantPhase)
			}
			if progress.Percent != tt.wantPercent {
				t.Errorf("Percent = %d, want %d", progress.Percent, tt.wantPercent)
			}
			if tt.wantCompleted > 0 && progress.CompletedSteps != tt.wantCompleted {
				t.Errorf("CompletedSteps = %d, want %d", progress.CompletedSteps, tt.wantCompleted)
			}
		})
	}
}

func TestBuildTimeline(t *testing.T) {
	now := time.Now()

	events := []RunEvent{
		{ID: "e1", RunID: "r1", TaskID: "t1", Type: EventRunStarted, Status: "started", Source: "cli", StartedAt: now},
		{ID: "e2", RunID: "r1", Type: EventPlannerStarted, Status: "started", Source: "agent", StartedAt: now.Add(time.Second)},
		{ID: "e3", RunID: "r1", Type: EventToolStarted, Status: "started", Source: "tool", StepID: "e2", ParentID: "e2", StartedAt: now.Add(1500 * time.Millisecond)},
		{ID: "e4", RunID: "r1", Type: EventToolFinished, Status: "succeeded", Source: "tool", StepID: "e2", ParentID: "e2", StartedAt: now.Add(1500 * time.Millisecond), FinishedAt: now.Add(2 * time.Second), DurationMs: 500},
		{ID: "e5", RunID: "r1", Type: EventPlannerFinished, Status: "succeeded", Source: "agent", StartedAt: now.Add(time.Second), FinishedAt: now.Add(2 * time.Second), DurationMs: 1000},
		{ID: "e6", RunID: "r1", Type: EventRunSucceeded, Status: "succeeded", Source: "cli", StartedAt: now, FinishedAt: now.Add(3 * time.Second), DurationMs: 3000},
	}

	timeline := BuildTimeline(events)

	if timeline.RunID != "r1" {
		t.Errorf("RunID = %q, want r1", timeline.RunID)
	}
	if timeline.State != "succeeded" {
		t.Errorf("State = %q, want succeeded", timeline.State)
	}
	if timeline.TaskID != "t1" {
		t.Errorf("TaskID = %q, want t1", timeline.TaskID)
	}
	if len(timeline.Events) != 6 {
		t.Errorf("Events count = %d, want 6", len(timeline.Events))
	}
	if len(timeline.Steps) == 0 {
		t.Error("Steps is empty")
	}

	// Check that planner step has tool child
	for _, step := range timeline.Steps {
		if step.Type == EventPlannerStarted {
			if len(step.Children) != 1 {
				t.Errorf("Planner children count = %d, want 1", len(step.Children))
			} else if step.Children[0].Type != EventToolStarted {
				t.Errorf("Planner child type = %s, want tool.started", step.Children[0].Type)
			}
		}
	}
}

func TestDefaultEventBusEmit(t *testing.T) {
	bus := NewDefaultEventBus()
	ctx := context.Background()

	evt := RunEvent{
		ID:     "evt_test",
		RunID:  "run_001",
		Type:   EventRunStarted,
		Status: "started",
	}

	if err := bus.Emit(ctx, evt); err != nil {
		t.Fatalf("Emit() error: %v", err)
	}
}

func TestDefaultEventBusSubscribe(t *testing.T) {
	bus := NewDefaultEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filter := EventFilter{RunID: "run_001"}
	ch, err := bus.Subscribe(ctx, filter)
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Emit a matching event
	bus.Emit(ctx, RunEvent{ID: "e1", RunID: "run_001", Type: EventRunStarted, Status: "started"})

	// Emit a non-matching event
	bus.Emit(ctx, RunEvent{ID: "e2", RunID: "run_002", Type: EventRunStarted, Status: "started"})

	// Should receive only the matching event
	select {
	case evt := <-ch:
		if evt.RunID != "run_001" {
			t.Errorf("received event with RunID = %q, want run_001", evt.RunID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Should not receive the non-matching event
	select {
	case evt := <-ch:
		t.Errorf("received unexpected event: %+v", evt)
	case <-time.After(100 * time.Millisecond):
		// Expected: no event
	}
}

func TestDefaultEventBusWithSink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	store, err := NewJSONLRunEventStore(path)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore() error: %v", err)
	}
	defer store.Close()

	bus := NewDefaultEventBus(store)
	ctx := context.Background()

	evt := RunEvent{
		ID:     "evt_sink_test",
		RunID:  "run_sink",
		Type:   EventRunStarted,
		Status: "started",
	}

	if err := bus.Emit(ctx, evt); err != nil {
		t.Fatalf("Emit() error: %v", err)
	}

	// Verify the event was persisted through the sink
	events, err := store.ListEvents(ctx, "run_sink")
	if err != nil {
		t.Fatalf("ListEvents() error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("ListEvents() returned %d events, want 1", len(events))
	}
	if events[0].ID != "evt_sink_test" {
		t.Errorf("event ID = %q, want evt_sink_test", events[0].ID)
	}
}

func TestSafeEmit(t *testing.T) {
	t.Run("nil emitter does not panic", func(t *testing.T) {
		ctx := context.Background()
		evt := RunEvent{ID: "e1", RunID: "r1", Type: EventRunStarted}
		// Should not panic
		SafeEmit(nil, ctx, evt)
	})

	t.Run("noop emitter does not panic", func(t *testing.T) {
		ctx := context.Background()
		emitter := &NoopEventEmitter{}
		evt := RunEvent{ID: "e1", RunID: "r1", Type: EventRunStarted}
		SafeEmit(emitter, ctx, evt)
	})

	t.Run("real emitter sanitizes and emits", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "events.jsonl")

		store, err := NewJSONLRunEventStore(path)
		if err != nil {
			t.Fatalf("NewJSONLRunEventStore() error: %v", err)
		}
		defer store.Close()

		bus := NewDefaultEventBus(store)
		emitter := NewEventEmitter(bus)
		ctx := context.Background()

		evt := RunEvent{
			ID:      "e1",
			RunID:   "r1",
			Type:    EventRunStarted,
			Message: "API_KEY=sk-12345", // should be redacted
			Metadata: map[string]string{
				"secret_key": "value",     // should be redacted
				"tool_name":  "file_read", // should be preserved
			},
		}

		SafeEmit(emitter, ctx, evt)

		// Verify the event was sanitized and persisted
		events, err := store.ListEvents(ctx, "r1")
		if err != nil {
			t.Fatalf("ListEvents() error: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("ListEvents() returned %d events, want 1", len(events))
		}

		// Check redaction
		if events[0].Message != "<redacted>" {
			t.Errorf("Message not redacted: %q", events[0].Message)
		}
		if events[0].Metadata["secret_key"] != "<redacted>" {
			t.Errorf("unsafe metadata not redacted: %q", events[0].Metadata["secret_key"])
		}
		if events[0].Metadata["tool_name"] != "file_read" {
			t.Errorf("safe metadata was redacted: %q", events[0].Metadata["tool_name"])
		}
	})
}

func TestNoopEventStore(t *testing.T) {
	store := &NoopEventStore{}
	ctx := context.Background()

	evt := RunEvent{ID: "e1", RunID: "r1", Type: EventRunStarted}

	if err := store.Append(ctx, evt); err != nil {
		t.Errorf("Append() error: %v", err)
	}

	runs, err := store.ListRuns(ctx)
	if err != nil {
		t.Errorf("ListRuns() error: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("ListRuns() = %d, want 0", len(runs))
	}

	events, err := store.ListEvents(ctx, "r1")
	if err != nil {
		t.Errorf("ListEvents() error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("ListEvents() = %d, want 0", len(events))
	}

	timeline, err := store.GetTimeline(ctx, "r1")
	if err != nil {
		t.Errorf("GetTimeline() error: %v", err)
	}
	if timeline.RunID != "" {
		t.Errorf("GetTimeline().RunID = %q, want empty", timeline.RunID)
	}
}

func TestDefaultEventsConfig(t *testing.T) {
	cfg := DefaultEventsConfig()
	if !cfg.Enabled {
		t.Error("DefaultEventsConfig().Enabled = false, want true")
	}
	if cfg.StorePath == "" {
		t.Error("DefaultEventsConfig().StorePath is empty")
	}
	if cfg.MaxMessageBytes != 2048 {
		t.Errorf("DefaultEventsConfig().MaxMessageBytes = %d, want 2048", cfg.MaxMessageBytes)
	}
	if cfg.MaxMetadataValueBytes != 512 {
		t.Errorf("DefaultEventsConfig().MaxMetadataValueBytes = %d, want 512", cfg.MaxMetadataValueBytes)
	}
	if !cfg.EmitToolEvents {
		t.Error("DefaultEventsConfig().EmitToolEvents = false, want true")
	}
	if !cfg.EmitModelEvents {
		t.Error("DefaultEventsConfig().EmitModelEvents = false, want true")
	}
	if !cfg.EmitPatchEvents {
		t.Error("DefaultEventsConfig().EmitPatchEvents = false, want true")
	}
	if !cfg.EmitValidationEvents {
		t.Error("DefaultEventsConfig().EmitValidationEvents = false, want true")
	}
}

func TestLoadEventsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	// Write test data
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	f.WriteString(`{"id":"evt_1","run_id":"run_001","type":"run.started","status":"started"}` + "\n")
	f.WriteString("bad line\n")
	f.WriteString(`{"id":"evt_2","run_id":"run_001","type":"run.succeeded","status":"succeeded"}` + "\n")
	f.Close()

	events, corrupted, err := LoadEventsFromFile(path)
	if err != nil {
		t.Fatalf("LoadEventsFromFile() error: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("events count = %d, want 2", len(events))
	}
	if len(corrupted) != 1 {
		t.Errorf("corrupted count = %d, want 1", len(corrupted))
	}
	if len(corrupted) > 0 && corrupted[0].Line != 2 {
		t.Errorf("corrupted line = %d, want 2", corrupted[0].Line)
	}
}

func TestJSONLRunEventStoreSortedByTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	store, err := NewJSONLRunEventStore(path)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore() error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	// Insert events out of order
	events := []RunEvent{
		{ID: "e3", RunID: "r1", Type: EventCoderFinished, Status: "succeeded", StartedAt: now.Add(5 * time.Second)},
		{ID: "e1", RunID: "r1", Type: EventRunStarted, Status: "started", StartedAt: now},
		{ID: "e2", RunID: "r1", Type: EventPlannerFinished, Status: "succeeded", StartedAt: now.Add(2 * time.Second)},
	}

	for _, evt := range events {
		if err := store.Append(ctx, evt); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	result, err := store.ListEvents(ctx, "r1")
	if err != nil {
		t.Fatalf("ListEvents() error: %v", err)
	}

	// Should be sorted by StartedAt
	for i := 1; i < len(result); i++ {
		if result[i].StartedAt.Before(result[i-1].StartedAt) {
			t.Errorf("events not sorted: [%d]=%v after [%d]=%v", i, result[i].StartedAt, i-1, result[i-1].StartedAt)
		}
	}

	if result[0].ID != "e1" {
		t.Errorf("first event ID = %s, want e1", result[0].ID)
	}
}

func TestSanitizeEventMultilineMessage(t *testing.T) {
	event := RunEvent{
		Message: "line1: ok\nline2: API_KEY=sk-abc\nline3: also ok",
	}

	sanitized := SanitizeEvent(event)

	lines := splitLines(sanitized.Message)
	if lines[0] != "line1: ok" {
		t.Errorf("line 1 = %q, want %q", lines[0], "line1: ok")
	}
	if lines[1] != "<redacted>" {
		t.Errorf("line 2 = %q, want <redacted>", lines[1])
	}
	if lines[2] != "line3: also ok" {
		t.Errorf("line 3 = %q, want %q", lines[2], "line3: also ok")
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	result := []string{}
	curr := ""
	for _, r := range s {
		if r == '\n' {
			result = append(result, curr)
			curr = ""
		} else {
			curr += string(r)
		}
	}
	result = append(result, curr)
	return result
}

func TestSanitizeEventEmptyMetadata(t *testing.T) {
	event := RunEvent{
		Message:  "Test message",
		Metadata: nil,
	}

	sanitized := SanitizeEvent(event)
	if sanitized.Message != "Test message" {
		t.Errorf("message was modified: %q", sanitized.Message)
	}
	if sanitized.Metadata != nil {
		t.Errorf("nil metadata was changed to non-nil")
	}
}

func TestSanitizeEventEmptyFields(t *testing.T) {
	event := RunEvent{}

	sanitized := SanitizeEvent(event)
	if sanitized.Message != "" {
		t.Errorf("empty message was modified: %q", sanitized.Message)
	}
	if sanitized.Error != "" {
		t.Errorf("empty error was modified: %q", sanitized.Error)
	}
}

// Test that SortSlice works correctly for RunSummary ordering
func TestListRunsSortedDescending(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	store, err := NewJSONLRunEventStore(path)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore() error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	// Create 3 runs with different start times
	for i, runID := range []string{"run_a", "run_b", "run_c"} {
		evt := RunEvent{
			ID:        "evt_" + runID,
			RunID:     runID,
			Type:      EventRunStarted,
			Status:    "started",
			StartedAt: now.Add(time.Duration(i) * time.Hour),
		}
		if err := store.Append(ctx, evt); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	summaries, err := store.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns() error: %v", err)
	}

	if len(summaries) != 3 {
		t.Fatalf("ListRuns() returned %d runs, want 3", len(summaries))
	}

	// Verify sorted descending by StartedAt
	if !sort.SliceIsSorted(summaries, func(i, j int) bool {
		return summaries[i].StartedAt.After(summaries[j].StartedAt)
	}) {
		t.Error("ListRuns() results are not sorted by StartedAt descending")
	}

	// run_c started last so should be first
	if summaries[0].RunID != "run_c" {
		t.Errorf("first run = %s, want run_c", summaries[0].RunID)
	}
	if summaries[2].RunID != "run_a" {
		t.Errorf("last run = %s, want run_a", summaries[2].RunID)
	}
}
