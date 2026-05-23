package conversation

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// JSONLConversationLog is an append-only conversation log backed by JSONL files.
// Each conversation is stored in a separate file under baseDir.
// Events are immutable once written: there is no update or delete operation.
type JSONLConversationLog struct {
	mu      sync.Mutex
	baseDir string
}

// NewJSONLConversationLog creates a new log that stores files under baseDir.
// The directory is created on first write if it does not exist.
func NewJSONLConversationLog(baseDir string) *JSONLConversationLog {
	return &JSONLConversationLog{baseDir: baseDir}
}

// Append adds an event to the conversation log. If the event has no ID,
// one is generated. If CreatedAt is zero, it is set to the current UTC time.
func (l *JSONLConversationLog) Append(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if event.ID == "" {
		event.ID = fmt.Sprintf("evt_%d_%d", time.Now().UnixMilli(), time.Now().Nanosecond())
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	path := l.conversationPath(event.ConversationID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create conversation dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open conversation file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
		return fmt.Errorf("write event: %w", err)
	}

	return f.Sync()
}

// Read returns events matching the query. Events are returned in append order.
// Archived events are excluded unless IncludeArchived is true.
func (l *JSONLConversationLog) Read(ctx context.Context, query Query) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	events, err := l.readAll(query.ConversationID)
	if err != nil {
		return nil, err
	}

	return l.filterEvents(events, query), nil
}

// Tail returns the last N events matching the query.
func (l *JSONLConversationLog) Tail(ctx context.Context, query Query) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	events, err := l.readAll(query.ConversationID)
	if err != nil {
		return nil, err
	}

	filtered := l.filterEvents(events, query)

	if query.Limit > 0 && len(filtered) > query.Limit {
		filtered = filtered[len(filtered)-query.Limit:]
	}

	return filtered, nil
}

func (l *JSONLConversationLog) conversationPath(conversationID string) string {
	return filepath.Join(l.baseDir, conversationID+".jsonl")
}

func (l *JSONLConversationLog) readAll(conversationID string) ([]Event, error) {
	path := l.conversationPath(conversationID)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open conversation file: %w", err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue // skip malformed lines
		}
		events = append(events, event)
	}

	return events, scanner.Err()
}

func (l *JSONLConversationLog) filterEvents(events []Event, query Query) []Event {
	afterIDFound := query.AfterID == ""
	var result []Event

	for _, event := range events {
		if !afterIDFound {
			if event.ID == query.AfterID {
				afterIDFound = true
			}
			continue
		}

		if query.TaskID != "" && event.TaskID != query.TaskID {
			continue
		}
		if !query.IncludeArchived && event.Archived {
			continue
		}

		result = append(result, event)

		if query.Limit > 0 && len(result) >= query.Limit {
			break
		}
	}

	return result
}
