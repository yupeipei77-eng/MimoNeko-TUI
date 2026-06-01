package events

import (
	"context"
	"strings"
)

// EventEmitter is the interface that runtime components use to emit events.
// When nil, event emission is disabled and the runtime behaves as if
// the event system does not exist.
//
// All runtime components that emit events must accept this as an optional
// dependency. Emit failures are logged but must not crash the runtime.
type EventEmitter interface {
	// Emit records a structured event.
	// Implementations must be safe for concurrent use.
	// Errors from Emit must not propagate to callers; they are for
	// internal logging/metrics only.
	Emit(ctx context.Context, event RunEvent) error
}

// NoopEventEmitter is an EventEmitter that discards all events.
// Used as a safe default when event emission is disabled.
type NoopEventEmitter struct{}

// Emit discards the event and returns nil.
func (n *NoopEventEmitter) Emit(ctx context.Context, event RunEvent) error { return nil }

// Ensure NoopEventEmitter implements EventEmitter.
var _ EventEmitter = (*NoopEventEmitter)(nil)

// SafeEmit calls emitter.Emit if the emitter is non-nil, logging any error.
// If emitter is nil, the call is a no-op. This is the recommended way to
// emit events from runtime components.
// Events with an empty RunID are skipped to prevent
// polluting the EventStore with orphan events.
func SafeEmit(emitter EventEmitter, ctx context.Context, event RunEvent) {
	if emitter == nil {
		return
	}
	// Enrich from context if fields are empty
	EnrichFromCtx(ctx, &event)
	// Skip events with empty RunID (would be rejected by EventStore anyway).
	if strings.TrimSpace(event.RunID) == "" {
		return
	}
	// Sanitize before emission
	event = SanitizeEvent(event)
	_ = emitter.Emit(ctx, event)
}

// EnrichFromCtx populates RunID, TaskID, and WorktreeID on the event
// from RunContext stored in ctx, but only if the event fields are empty.
// This allows callers to explicitly set IDs while still falling back
// to context propagation.
func EnrichFromCtx(ctx context.Context, event *RunEvent) {
	rc := RunContextFrom(ctx)
	if event.RunID == "" {
		event.RunID = rc.RunID
	}
	if event.TaskID == "" {
		event.TaskID = rc.TaskID
	}
	if event.WorktreeID == "" {
		event.WorktreeID = rc.WorktreeID
	}
}

// NewEventEmitter creates an EventEmitter from an EventBus and optional EventStore.
// If bus is nil, returns a NoopEventEmitter.
func NewEventEmitter(bus *DefaultEventBus) EventEmitter {
	if bus == nil {
		return &NoopEventEmitter{}
	}
	return bus
}

// NewEventEmitterFromStore creates an EventEmitter that writes to an EventStore.
// If store is nil, returns a NoopEventEmitter.
func NewEventEmitterFromStore(store EventStore) EventEmitter {
	if store == nil {
		return &NoopEventEmitter{}
	}
	return &storeEventEmitter{store: store}
}

// storeEventEmitter implements EventEmitter by writing to an EventStore.
type storeEventEmitter struct {
	store EventStore
}

// Emit writes the event to the store.
func (e *storeEventEmitter) Emit(ctx context.Context, event RunEvent) error {
	return e.store.Append(ctx, event)
}
