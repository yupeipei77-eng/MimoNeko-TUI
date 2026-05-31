package scratchpad

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/nekonomimo/nekonomimo/internal/prefix"
)

const defaultSoftTokenLimit = 100000

// VolatileScratchpad is an in-memory implementation of the Scratchpad interface.
// Items live only for the process lifetime and are not persisted to disk.
type VolatileScratchpad struct {
	mu    sync.Mutex
	items map[string][]Item
}

// NewVolatileScratchpad creates a new empty volatile scratchpad.
func NewVolatileScratchpad() *VolatileScratchpad {
	return &VolatileScratchpad{
		items: make(map[string][]Item),
	}
}

// Put adds an item to the scratchpad and evicts low-priority items if needed.
func (v *VolatileScratchpad) Put(ctx context.Context, item Item) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	v.items[item.TaskID] = append(v.items[item.TaskID], item)
	v.evict(item.TaskID)
	return nil
}

// Snapshot returns items matching the scope, ordered by priority (highest first).
// Items exceeding the scope's token budget or count limit are excluded.
// Expired items are not returned but are not removed from storage.
func (v *VolatileScratchpad) Snapshot(ctx context.Context, scope Scope) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	items := v.items[scope.TaskID]

	// Filter by kind and expiration
	var filtered []Item
	for _, item := range items {
		if item.ExpiresAt.After(now) || item.ExpiresAt.IsZero() {
			if len(scope.Kinds) == 0 || containsKind(scope.Kinds, item.Kind) {
				filtered = append(filtered, item)
			}
		}
	}

	// Sort by priority descending, then by CreatedAt ascending for ties
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Priority != filtered[j].Priority {
			return filtered[i].Priority > filtered[j].Priority
		}
		return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
	})

	// Apply limit and token budget
	selected := applyBudget(filtered, scope.Limit, scope.TokenBudget)

	return Snapshot{TaskID: scope.TaskID, Items: selected}, nil
}

// Clear removes items from the scratchpad. If scope.Kinds is empty,
// all items for the task are removed. Otherwise, only matching kinds are removed.
func (v *VolatileScratchpad) Clear(ctx context.Context, scope Scope) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if len(scope.Kinds) == 0 {
		delete(v.items, scope.TaskID)
		return nil
	}

	items := v.items[scope.TaskID]
	var remaining []Item
	for _, item := range items {
		if !containsKind(scope.Kinds, item.Kind) {
			remaining = append(remaining, item)
		}
	}
	if len(remaining) == 0 {
		delete(v.items, scope.TaskID)
	} else {
		v.items[scope.TaskID] = remaining
	}
	return nil
}

// evict removes the lowest-priority, oldest items when total tokens exceed the soft limit.
func (v *VolatileScratchpad) evict(taskID string) {
	items := v.items[taskID]
	totalTokens := totalItemTokens(items)
	if totalTokens <= defaultSoftTokenLimit {
		return
	}

	// Sort by priority ascending (lowest first), then by CreatedAt ascending (oldest first)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	// Evict from the front (lowest priority, oldest)
	for len(items) > 0 && totalTokens > defaultSoftTokenLimit {
		evicted := items[0]
		totalTokens -= prefix.EstimateTokens(evicted.Content)
		items = items[1:]
	}

	v.items[taskID] = items
}

// applyBudget selects items up to the count limit and token budget.
func applyBudget(items []Item, countLimit, tokenBudget int) []Item {
	var selected []Item
	totalTokens := 0

	for _, item := range items {
		itemTokens := prefix.EstimateTokens(item.Content)

		if tokenBudget > 0 && totalTokens+itemTokens > tokenBudget {
			continue
		}

		totalTokens += itemTokens
		selected = append(selected, item)

		if countLimit > 0 && len(selected) >= countLimit {
			break
		}
	}
	return selected
}

func containsKind(kinds []ItemKind, kind ItemKind) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

func totalItemTokens(items []Item) int {
	total := 0
	for _, item := range items {
		total += prefix.EstimateTokens(item.Content)
	}
	return total
}
