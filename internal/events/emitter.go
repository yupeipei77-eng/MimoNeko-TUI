package events

import "context"

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
func SafeEmit(emitter EventEmitter, ctx context.Context, event RunEvent) {
	if emitter == nil {
		return
	}
	// Sanitize before emission
	event = SanitizeEvent(event)
	_ = emitter.Emit(ctx, event)
}

// NewEventEmitter creates an EventEmitter from an EventBus and optional EventStore.
// If bus is nil, returns a NoopEventEmitter.
func NewEventEmitter(bus *DefaultEventBus) EventEmitter {
	if bus == nil {
		return &NoopEventEmitter{}
	}
	return bus
}
