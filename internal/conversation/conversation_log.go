package conversation

import (
	"context"
	"encoding/json"
	"time"
)

type EventType string

const (
	EventUserMessage    EventType = "user_message"
	EventAssistantDelta EventType = "assistant_delta"
	EventModelCall      EventType = "model_call"
	EventToolCall       EventType = "tool_call"
	EventPatch          EventType = "patch"
	EventTestRun        EventType = "test_run"
	EventRollback       EventType = "rollback"
	EventTaskState      EventType = "task_state"
)

type Event struct {
	ID             string
	ConversationID string
	TaskID         string
	Type           EventType
	Payload        json.RawMessage
	CreatedAt      time.Time
}

type Query struct {
	ConversationID string
	TaskID         string
	AfterID        string
	Limit          int
}

type ConversationLog interface {
	Append(ctx context.Context, event Event) error
	Read(ctx context.Context, query Query) ([]Event, error)
	Tail(ctx context.Context, query Query) ([]Event, error)
}
