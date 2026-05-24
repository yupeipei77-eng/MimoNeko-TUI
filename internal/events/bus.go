package events

import (
	"context"
	"log"
	"sync"
)

// DefaultEventBus implements EventBus with in-memory fan-out to subscribers.
//
// Emit is non-blocking: if a subscriber's channel is full, the event is dropped
// for that subscriber. This ensures event emission never blocks the runtime
// for too long.
type DefaultEventBus struct {
	mu          sync.RWMutex
	subscribers []subscriber
	sinks       []EventSink
}

type subscriber struct {
	filter EventFilter
	ch     chan RunEvent
}

// NewDefaultEventBus creates a new DefaultEventBus with optional sinks.
func NewDefaultEventBus(sinks ...EventSink) *DefaultEventBus {
	return &DefaultEventBus{
		sinks: sinks,
	}
}

// Emit publishes an event to all matching subscribers and writes it to all sinks.
//
// Sink write failures are logged but do not cause Emit to return an error,
// because event emission must not crash the runtime. Subscriber delivery
// is best-effort (non-blocking).
func (b *DefaultEventBus) Emit(ctx context.Context, event RunEvent) error {
	// Write to all sinks first
	for _, sink := range b.sinks {
		if err := sink.Write(ctx, event); err != nil {
			log.Printf("events: sink write failed: %v", err)
		}
	}

	// Fan out to subscribers (non-blocking)
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		if sub.filter.Matches(event) {
			select {
			case sub.ch <- event:
			default:
				// Drop event if subscriber is too slow
				log.Printf("events: subscriber channel full, dropping event %s", event.ID)
			}
		}
	}

	return nil
}

// Subscribe registers a subscriber that receives events matching the filter.
// The returned channel has a buffer of 256 events. Callers must drain the
// channel to avoid event drops.
func (b *DefaultEventBus) Subscribe(ctx context.Context, filter EventFilter) (<-chan RunEvent, error) {
	ch := make(chan RunEvent, 256)

	b.mu.Lock()
	defer b.mu.Unlock()

	sub := subscriber{
		filter: filter,
		ch:     ch,
	}
	b.subscribers = append(b.subscribers, sub)

	// Cleanup when context is done
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, s := range b.subscribers {
			if s.ch == ch {
				b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
				close(ch)
				break
			}
		}
	}()

	return ch, nil
}
