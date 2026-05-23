package conversation

import (
	"context"
	"encoding/json"
	"os"
	"strings"
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
