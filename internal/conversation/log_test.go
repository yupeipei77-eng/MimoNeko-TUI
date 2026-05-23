package conversation

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestAppendAndRead(t *testing.T) {
	log := NewJSONLConversationLog(t.TempDir())

	events := []Event{
		{ConversationID: "c1", TaskID: "t1", Type: EventUserMessage, Payload: json.RawMessage(`"hello"`)},
		{ConversationID: "c1", TaskID: "t1", Type: EventAssistantDelta, Payload: json.RawMessage(`"hi"`)},
		{ConversationID: "c1", TaskID: "t1", Type: EventToolCall, Payload: json.RawMessage(`"tool"`)},
	}

	for _, e := range events {
		if err := log.Append(context.Background(), e); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	result, err := log.Read(context.Background(), Query{ConversationID: "c1"})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Read() returned %d events, want 3", len(result))
	}

	// Verify order
	if result[0].Type != EventUserMessage || result[1].Type != EventAssistantDelta || result[2].Type != EventToolCall {
		t.Errorf("Events out of order: %v", result)
	}
}

func TestReadFiltersByConversationID(t *testing.T) {
	log := NewJSONLConversationLog(t.TempDir())

	_ = log.Append(context.Background(), Event{ConversationID: "c1", Type: EventUserMessage, Payload: json.RawMessage(`"a"`)})
	_ = log.Append(context.Background(), Event{ConversationID: "c2", Type: EventUserMessage, Payload: json.RawMessage(`"b"`)})
	_ = log.Append(context.Background(), Event{ConversationID: "c1", Type: EventAssistantDelta, Payload: json.RawMessage(`"c"`)})

	result, _ := log.Read(context.Background(), Query{ConversationID: "c1"})
	if len(result) != 2 {
		t.Errorf("Read(c1) returned %d events, want 2", len(result))
	}
}

func TestReadAfterID(t *testing.T) {
	log := NewJSONLConversationLog(t.TempDir())

	_ = log.Append(context.Background(), Event{ID: "e1", ConversationID: "c1", Type: EventUserMessage, Payload: json.RawMessage(`"a"`)})
	_ = log.Append(context.Background(), Event{ID: "e2", ConversationID: "c1", Type: EventAssistantDelta, Payload: json.RawMessage(`"b"`)})
	_ = log.Append(context.Background(), Event{ID: "e3", ConversationID: "c1", Type: EventToolCall, Payload: json.RawMessage(`"c"`)})

	result, _ := log.Read(context.Background(), Query{ConversationID: "c1", AfterID: "e1"})
	if len(result) != 2 {
		t.Fatalf("Read(after e1) returned %d events, want 2", len(result))
	}
	if result[0].ID != "e2" {
		t.Errorf("First event after e1 = %q, want e2", result[0].ID)
	}
}

func TestTailReturnsLastN(t *testing.T) {
	log := NewJSONLConversationLog(t.TempDir())

	for i := 0; i < 10; i++ {
		_ = log.Append(context.Background(), Event{ConversationID: "c1", Type: EventUserMessage, Payload: json.RawMessage(`"x"`)})
	}

	result, _ := log.Tail(context.Background(), Query{ConversationID: "c1", Limit: 3})
	if len(result) != 3 {
		t.Fatalf("Tail(limit=3) returned %d events, want 3", len(result))
	}
}

func TestArchivedExcludedByDefault(t *testing.T) {
	log := NewJSONLConversationLog(t.TempDir())

	_ = log.Append(context.Background(), Event{ID: "e1", ConversationID: "c1", Type: EventUserMessage, Payload: json.RawMessage(`"a"`)})
	_ = log.Append(context.Background(), Event{ID: "e2", ConversationID: "c1", Type: EventUserMessage, Payload: json.RawMessage(`"b"`), Archived: true})

	result, _ := log.Read(context.Background(), Query{ConversationID: "c1"})
	if len(result) != 1 {
		t.Fatalf("Read() returned %d events, want 1 (archived excluded)", len(result))
	}
	if result[0].ID != "e1" {
		t.Errorf("Non-archived event ID = %q, want e1", result[0].ID)
	}

	// IncludeArchived should return both
	resultAll, _ := log.Read(context.Background(), Query{ConversationID: "c1", IncludeArchived: true})
	if len(resultAll) != 2 {
		t.Fatalf("Read(IncludeArchived) returned %d events, want 2", len(resultAll))
	}
}

func TestAppendCreatesFile(t *testing.T) {
	dir := t.TempDir()
	log := NewJSONLConversationLog(dir)

	_ = log.Append(context.Background(), Event{ConversationID: "new_conv", Type: EventUserMessage, Payload: json.RawMessage(`"test"`)})

	// File should exist
	path := dir + "/new_conv.jsonl"
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Conversation file not created: %v", err)
	}
}

func TestEventIDGeneration(t *testing.T) {
	log := NewJSONLConversationLog(t.TempDir())

	// No ID provided — one should be generated
	_ = log.Append(context.Background(), Event{ConversationID: "c1", Type: EventUserMessage, Payload: json.RawMessage(`"a"`)})

	result, _ := log.Read(context.Background(), Query{ConversationID: "c1"})
	if len(result) != 1 {
		t.Fatalf("Read() returned %d events, want 1", len(result))
	}
	if !strings.HasPrefix(result[0].ID, "evt_") {
		t.Errorf("Generated ID = %q, want evt_ prefix", result[0].ID)
	}

	// crypto/rand IDs should be 32 hex chars after evt_ prefix
	idSuffix := strings.TrimPrefix(result[0].ID, "evt_")
	if len(idSuffix) != 32 {
		t.Errorf("Generated ID suffix length = %d, want 32 (16 bytes hex)", len(idSuffix))
	}

	// ID provided — should be preserved
	_ = log.Append(context.Background(), Event{ID: "custom_id", ConversationID: "c2", Type: EventUserMessage, Payload: json.RawMessage(`"b"`)})

	result2, _ := log.Read(context.Background(), Query{ConversationID: "c2"})
	if len(result2) != 1 {
		t.Fatalf("Read() returned %d events, want 1", len(result2))
	}
	if result2[0].ID != "custom_id" {
		t.Errorf("Preserved ID = %q, want custom_id", result2[0].ID)
	}
}

// Test 5: Event IDs are unique under concurrent generation
func TestEventIDConcurrentUniqueness(t *testing.T) {
	log := NewJSONLConversationLog(t.TempDir())

	var wg sync.WaitGroup
	const n = 100
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = log.Append(context.Background(), Event{
				ConversationID: "c1",
				Type:           EventUserMessage,
				Payload:        json.RawMessage(`"concurrent"`),
			})
		}()
	}
	wg.Wait()

	result, _ := log.Read(context.Background(), Query{ConversationID: "c1", IncludeArchived: true})
	if len(result) != n {
		t.Fatalf("Read() returned %d events, want %d", len(result), n)
	}

	// Check uniqueness
	ids := make(map[string]bool)
	for _, e := range result {
		if ids[e.ID] {
			t.Errorf("Duplicate event ID: %q", e.ID)
		}
		ids[e.ID] = true
	}
}

// Test 6: JSONL file permissions are 0600
func TestConversationFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support Unix permission bits")
	}
	dir := t.TempDir()
	log := NewJSONLConversationLog(dir)

	_ = log.Append(context.Background(), Event{ConversationID: "c1", Type: EventUserMessage, Payload: json.RawMessage(`"a"`)})

	path := dir + "/c1.jsonl"
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("File permissions = %o, want 0600", perm)
	}
}

// Test 7: Malformed JSONL lines are counted in ReadStats
func TestMalformedJSONLCounted(t *testing.T) {
	dir := t.TempDir()
	log := NewJSONLConversationLog(dir)

	// Write a valid event
	_ = log.Append(context.Background(), Event{ConversationID: "c1", Type: EventUserMessage, Payload: json.RawMessage(`"valid"`)})

	// Manually append a malformed line
	path := dir + "/c1.jsonl"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("OpenFile() error: %v", err)
	}
	f.WriteString("this is not valid json\n")
	f.WriteString("also not json\n")
	f.Close()

	_, stats, err := log.ReadWithStats(context.Background(), Query{ConversationID: "c1"})
	if err != nil {
		t.Fatalf("ReadWithStats() error: %v", err)
	}

	if stats.CorruptLineCount != 2 {
		t.Errorf("CorruptLineCount = %d, want 2", stats.CorruptLineCount)
	}
}
