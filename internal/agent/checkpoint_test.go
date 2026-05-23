package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJSONLCheckpointStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoints.jsonl")

	store, err := NewJSONLCheckpointStore(path)
	if err != nil {
		t.Fatalf("NewJSONLCheckpointStore() error = %v", err)
	}

	cp := Checkpoint{
		RunID:     "run_test123",
		TaskID:    "task_abc",
		State:     AgentStateRunning,
		StepIndex: 2,
		Steps: []AgentStep{
			{
				StepID:    "step_001",
				Index:     0,
				Type:      "model",
				State:     AgentStateSucceeded,
				ModelText: "I'll read the README",
			},
		},
		ContractID: "tc_contract1",
		CreatedAt:  time.Now().UTC().Truncate(time.Millisecond),
	}

	ctx := context.Background()
	if err := store.Save(ctx, cp); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load(ctx, "run_test123")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.RunID != cp.RunID {
		t.Errorf("RunID = %q, want %q", loaded.RunID, cp.RunID)
	}
	if loaded.TaskID != cp.TaskID {
		t.Errorf("TaskID = %q, want %q", loaded.TaskID, cp.TaskID)
	}
	if loaded.State != cp.State {
		t.Errorf("State = %q, want %q", loaded.State, cp.State)
	}
	if loaded.StepIndex != cp.StepIndex {
		t.Errorf("StepIndex = %d, want %d", loaded.StepIndex, cp.StepIndex)
	}
	if loaded.ContractID != cp.ContractID {
		t.Errorf("ContractID = %q, want %q", loaded.ContractID, cp.ContractID)
	}
	if len(loaded.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(loaded.Steps))
	}
	if loaded.Steps[0].StepID != "step_001" {
		t.Errorf("Steps[0].StepID = %q, want %q", loaded.Steps[0].StepID, "step_001")
	}
}

func TestJSONLCheckpointStore_LoadNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoints.jsonl")

	store, err := NewJSONLCheckpointStore(path)
	if err != nil {
		t.Fatalf("NewJSONLCheckpointStore() error = %v", err)
	}

	_, err = store.Load(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Load() should return error for nonexistent run ID")
	}
}

func TestJSONLCheckpointStore_MultipleCheckpointsSameRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoints.jsonl")

	store, err := NewJSONLCheckpointStore(path)
	if err != nil {
		t.Fatalf("NewJSONLCheckpointStore() error = %v", err)
	}

	ctx := context.Background()

	// Save two checkpoints for the same run
	cp1 := Checkpoint{
		RunID:     "run_same",
		State:     AgentStateRunning,
		StepIndex: 1,
		CreatedAt: time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond),
	}
	cp2 := Checkpoint{
		RunID:     "run_same",
		State:     AgentStateSucceeded,
		StepIndex: 3,
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}

	if err := store.Save(ctx, cp1); err != nil {
		t.Fatalf("Save() cp1 error = %v", err)
	}
	if err := store.Save(ctx, cp2); err != nil {
		t.Fatalf("Save() cp2 error = %v", err)
	}

	// Load should return the latest checkpoint
	loaded, err := store.Load(ctx, "run_same")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.State != AgentStateSucceeded {
		t.Errorf("State = %q, want %q (latest checkpoint)", loaded.State, AgentStateSucceeded)
	}
	if loaded.StepIndex != 3 {
		t.Errorf("StepIndex = %d, want 3", loaded.StepIndex)
	}
}

func TestJSONLCheckpointStore_List(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoints.jsonl")

	store, err := NewJSONLCheckpointStore(path)
	if err != nil {
		t.Fatalf("NewJSONLCheckpointStore() error = %v", err)
	}

	ctx := context.Background()

	// Save checkpoints for different runs
	runs := []struct {
		runID string
		delay time.Duration
	}{
		{"run_first", 0},
		{"run_second", time.Second},
		{"run_third", 2 * time.Second},
	}

	for _, r := range runs {
		cp := Checkpoint{
			RunID:     r.runID,
			State:     AgentStateSucceeded,
			StepIndex: 1,
			CreatedAt: time.Now().UTC().Add(r.delay).Truncate(time.Millisecond),
		}
		if err := store.Save(ctx, cp); err != nil {
			t.Fatalf("Save() %s error = %v", r.runID, err)
		}
	}

	ids, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("List() returned %d IDs, want 3", len(ids))
	}

	// Should be ordered by creation time descending
	if ids[0] != "run_third" {
		t.Errorf("ids[0] = %q, want %q", ids[0], "run_third")
	}
}

func TestJSONLCheckpointStore_CancelledContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoints.jsonl")

	store, err := NewJSONLCheckpointStore(path)
	if err != nil {
		t.Fatalf("NewJSONLCheckpointStore() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cp := Checkpoint{
		RunID:     "run_cancel",
		State:     AgentStateRunning,
		CreatedAt: time.Now().UTC(),
	}

	if err := store.Save(ctx, cp); err == nil {
		t.Error("Save() with cancelled context should return error")
	}
}

func TestDefaultCheckpointPath(t *testing.T) {
	path := DefaultCheckpointPath("/my/repo")
	expected := filepath.Join("/my/repo", ".reasonforge", "logs", "checkpoints.jsonl")
	if path != expected {
		t.Errorf("DefaultCheckpointPath() = %q, want %q", path, expected)
	}
}

func TestJSONLCheckpointStore_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "checkpoints.jsonl")

	store, err := NewJSONLCheckpointStore(path)
	if err != nil {
		t.Fatalf("NewJSONLCheckpointStore() error = %v", err)
	}

	// Directory should exist
	if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
		t.Error("NewJSONLCheckpointStore() should create parent directory")
	}

	// File should exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("NewJSONLCheckpointStore() should create checkpoint file")
	}

	_ = store // use store
}
