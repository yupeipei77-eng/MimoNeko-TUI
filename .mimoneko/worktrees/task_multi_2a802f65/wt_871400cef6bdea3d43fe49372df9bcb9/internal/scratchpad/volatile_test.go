package scratchpad

import (
	"context"
	"strings"
	"testing"
	"time"
)

func makeItem(id, taskID string, kind ItemKind, content string, priority int) Item {
	return Item{
		ID:        id,
		TaskID:    taskID,
		Kind:      kind,
		Content:   []byte(content),
		Priority:  priority,
		CreatedAt: time.Now(),
	}
}

func TestPutAndSnapshot(t *testing.T) {
	sp := NewVolatileScratchpad()

	item := makeItem("i1", "t1", ItemKindToolOutput, "hello world", 0)
	if err := sp.Put(context.Background(), item); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	snap, err := sp.Snapshot(context.Background(), Scope{TaskID: "t1"})
	if err != nil {
		t.Fatalf("Snapshot() error: %v", err)
	}
	if len(snap.Items) != 1 {
		t.Fatalf("Snapshot() returned %d items, want 1", len(snap.Items))
	}
	if snap.Items[0].ID != "i1" {
		t.Errorf("Snapshot() item ID = %q, want %q", snap.Items[0].ID, "i1")
	}
}

func TestSnapshotFiltersByKinds(t *testing.T) {
	sp := NewVolatileScratchpad()
	_ = sp.Put(context.Background(), makeItem("i1", "t1", ItemKindToolOutput, "tool", 0))
	_ = sp.Put(context.Background(), makeItem("i2", "t1", ItemKindRAGResult, "rag", 0))

	snap, _ := sp.Snapshot(context.Background(), Scope{TaskID: "t1", Kinds: []ItemKind{ItemKindToolOutput}})
	if len(snap.Items) != 1 {
		t.Fatalf("Snapshot() returned %d items, want 1", len(snap.Items))
	}
	if snap.Items[0].Kind != ItemKindToolOutput {
		t.Errorf("Snapshot() kind = %q, want %q", snap.Items[0].Kind, ItemKindToolOutput)
	}
}

func TestSnapshotRespectsLimit(t *testing.T) {
	sp := NewVolatileScratchpad()
	for i := 0; i < 5; i++ {
		_ = sp.Put(context.Background(), makeItem("i", "t1", ItemKindReasoning, "data", 0))
	}

	snap, _ := sp.Snapshot(context.Background(), Scope{TaskID: "t1", Limit: 2})
	if len(snap.Items) != 2 {
		t.Errorf("Snapshot() returned %d items, want 2", len(snap.Items))
	}
}

func TestSnapshotRespectsTokenBudget(t *testing.T) {
	sp := NewVolatileScratchpad()
	// Each item is 100 bytes = 25 tokens
	_ = sp.Put(context.Background(), makeItem("i1", "t1", ItemKindReasoning, strings.Repeat("a", 100), 0))
	_ = sp.Put(context.Background(), makeItem("i2", "t1", ItemKindReasoning, strings.Repeat("b", 100), 0))
	_ = sp.Put(context.Background(), makeItem("i3", "t1", ItemKindReasoning, strings.Repeat("c", 100), 0))

	// Budget of 50 tokens should fit 2 items (50 tokens)
	snap, _ := sp.Snapshot(context.Background(), Scope{TaskID: "t1", TokenBudget: 50})
	if len(snap.Items) != 2 {
		t.Errorf("Snapshot() returned %d items, want 2 (token budget)", len(snap.Items))
	}
}

func TestPriorityOrdering(t *testing.T) {
	sp := NewVolatileScratchpad()
	_ = sp.Put(context.Background(), makeItem("low", "t1", ItemKindReasoning, "low", 1))
	_ = sp.Put(context.Background(), makeItem("high", "t1", ItemKindReasoning, "high", 10))
	_ = sp.Put(context.Background(), makeItem("mid", "t1", ItemKindReasoning, "mid", 5))

	snap, _ := sp.Snapshot(context.Background(), Scope{TaskID: "t1"})
	if len(snap.Items) != 3 {
		t.Fatalf("Snapshot() returned %d items, want 3", len(snap.Items))
	}
	if snap.Items[0].ID != "high" {
		t.Errorf("first item priority = %d, want highest", snap.Items[0].Priority)
	}
	if snap.Items[1].ID != "mid" {
		t.Errorf("second item ID = %q, want mid", snap.Items[1].ID)
	}
	if snap.Items[2].ID != "low" {
		t.Errorf("third item ID = %q, want low", snap.Items[2].ID)
	}
}

func TestExpiredItemsExcluded(t *testing.T) {
	sp := NewVolatileScratchpad()
	item := makeItem("i1", "t1", ItemKindReasoning, "data", 0)
	item.ExpiresAt = time.Now().Add(-1 * time.Hour) // already expired
	_ = sp.Put(context.Background(), item)

	snap, _ := sp.Snapshot(context.Background(), Scope{TaskID: "t1"})
	if len(snap.Items) != 0 {
		t.Errorf("Snapshot() returned %d items, want 0 (expired)", len(snap.Items))
	}
}

func TestClearAllKinds(t *testing.T) {
	sp := NewVolatileScratchpad()
	_ = sp.Put(context.Background(), makeItem("i1", "t1", ItemKindReasoning, "data", 0))
	_ = sp.Put(context.Background(), makeItem("i2", "t1", ItemKindToolOutput, "tool", 0))

	_ = sp.Clear(context.Background(), Scope{TaskID: "t1"})

	snap, _ := sp.Snapshot(context.Background(), Scope{TaskID: "t1"})
	if len(snap.Items) != 0 {
		t.Errorf("Snapshot() after Clear() returned %d items, want 0", len(snap.Items))
	}
}

func TestClearSpecificKinds(t *testing.T) {
	sp := NewVolatileScratchpad()
	_ = sp.Put(context.Background(), makeItem("i1", "t1", ItemKindReasoning, "data", 0))
	_ = sp.Put(context.Background(), makeItem("i2", "t1", ItemKindToolOutput, "tool", 0))

	_ = sp.Clear(context.Background(), Scope{TaskID: "t1", Kinds: []ItemKind{ItemKindReasoning}})

	snap, _ := sp.Snapshot(context.Background(), Scope{TaskID: "t1"})
	if len(snap.Items) != 1 {
		t.Fatalf("Snapshot() after Clear() returned %d items, want 1", len(snap.Items))
	}
	if snap.Items[0].Kind != ItemKindToolOutput {
		t.Errorf("remaining kind = %q, want %q", snap.Items[0].Kind, ItemKindToolOutput)
	}
}
