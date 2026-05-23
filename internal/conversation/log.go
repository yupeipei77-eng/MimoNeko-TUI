package conversation

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
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
		id, err := generateEventID()
		if err != nil {
			return fmt.Errorf("generate event id: %w", err)
		}
		event.ID = id
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	path := l.conversationPath(event.ConversationID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create conversation dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open conversation file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
		return fmt.Errorf("write event: %w", err)
	}

	return f.Sync()
}

// ReadStats contains statistics about a JSONL read operation.
type ReadStats struct {
	CorruptLineCount int
}

// ReadWithStats returns events matching the query along with read statistics.
func (l *JSONLConversationLog) ReadWithStats(ctx context.Context, query Query) ([]Event, ReadStats, error) {
	if err := ctx.Err(); err != nil {
		return nil, ReadStats{}, err
	}

	events, stats, err := l.readAllWithStats(query.ConversationID)
	if err != nil {
		return nil, ReadStats{}, err
	}

	return l.filterEvents(events, query), stats, nil
}

// Read returns events matching the query. Events are returned in append order.
// Archived events are excluded unless IncludeArchived is true.
func (l *JSONLConversationLog) Read(ctx context.Context, query Query) ([]Event, error) {
	events, _, err := l.ReadWithStats(ctx, query)
	return events, err
}

// Tail returns the last N events matching the query.
func (l *JSONLConversationLog) Tail(ctx context.Context, query Query) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	events, _, err := l.readAllWithStats(query.ConversationID)
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
	events, _, err := l.readAllWithStats(conversationID)
	return events, err
}

func (l *JSONLConversationLog) readAllWithStats(conversationID string) ([]Event, ReadStats, error) {
	path := l.conversationPath(conversationID)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ReadStats{}, nil
		}
		return nil, ReadStats{}, fmt.Errorf("open conversation file: %w", err)
	}
	defer f.Close()

	var events []Event
	var corruptCount int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			corruptCount++
			continue
		}
		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, ReadStats{}, err
	}

	return events, ReadStats{CorruptLineCount: corruptCount}, nil
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

// generateEventID creates a collision-resistant event ID using crypto/rand.
// Format: evt_<32 hex chars> (16 random bytes encoded as hex).
func generateEventID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return "evt_" + hex.EncodeToString(b), nil
}
