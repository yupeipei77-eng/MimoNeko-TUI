package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reasonforge/reasonforge/internal/tools"
)

// --- SanitizeCheckpoint tests ---

func TestSanitizeCheckpoint_ModelTextTruncated(t *testing.T) {
	longText := strings.Repeat("x", 600) // exceeds 512 bytes
	cp := Checkpoint{
		RunID: "run_sanitize",
		Steps: []AgentStep{
			{
				StepID:    "step_001",
				Type:      "model",
				ModelText: longText,
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	sanitized := SanitizeCheckpoint(cp)
	if len(sanitized.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(sanitized.Steps))
	}

	st := sanitized.Steps[0]
	// ModelText should be truncated to 512 bytes + truncation marker
	if len(st.ModelText) > maxModelTextBytes+len(truncationMarker) {
		t.Errorf("ModelText len = %d, want at most %d", len(st.ModelText), maxModelTextBytes+len(truncationMarker))
	}
	if !strings.HasSuffix(st.ModelText, truncationMarker) {
		t.Errorf("ModelText should end with truncation marker, got suffix %q", st.ModelText[maxModelTextBytes-10:])
	}
	// Original should be unchanged
	if len(cp.Steps[0].ModelText) != 600 {
		t.Errorf("original ModelText should not be modified, len = %d", len(cp.Steps[0].ModelText))
	}
}

func TestSanitizeCheckpoint_ModelTextUnderLimit(t *testing.T) {
	shortText := "Hello, I am reading the file."
	cp := Checkpoint{
		RunID: "run_sanitize",
		Steps: []AgentStep{
			{
				StepID:    "step_001",
				Type:      "model",
				ModelText: shortText,
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	sanitized := SanitizeCheckpoint(cp)
	if sanitized.Steps[0].ModelText != shortText {
		t.Errorf("ModelText under limit should be preserved, got %q", sanitized.Steps[0].ModelText)
	}
}

func TestSanitizeCheckpoint_ToolResponseStdoutTruncated(t *testing.T) {
	longOutput := strings.Repeat("a", 1200) // exceeds 1024 bytes
	cp := Checkpoint{
		RunID: "run_sanitize",
		Steps: []AgentStep{
			{
				StepID: "step_002",
				Type:   "tool",
				ToolResponse: &tools.ToolResponse{
					ToolName: "file_read",
					Success:  true,
					Stdout:   longOutput,
				},
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	sanitized := SanitizeCheckpoint(cp)
	st := sanitized.Steps[0]
	if st.ToolResponse == nil {
		t.Fatal("ToolResponse should not be nil")
	}
	if len(st.ToolResponse.Stdout) > maxToolOutputBytes+len(truncationMarker) {
		t.Errorf("Stdout len = %d, want at most %d", len(st.ToolResponse.Stdout), maxToolOutputBytes+len(truncationMarker))
	}
	if !strings.HasSuffix(st.ToolResponse.Stdout, truncationMarker) {
		t.Error("Stdout should end with truncation marker")
	}
}

func TestSanitizeCheckpoint_ToolResponseStderrTruncated(t *testing.T) {
	longErr := strings.Repeat("e", 1200)
	cp := Checkpoint{
		RunID: "run_sanitize",
		Steps: []AgentStep{
			{
				StepID: "step_003",
				Type:   "tool",
				ToolResponse: &tools.ToolResponse{
					ToolName: "shell_exec",
					Success:  false,
					Stderr:   longErr,
				},
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	sanitized := SanitizeCheckpoint(cp)
	st := sanitized.Steps[0]
	if len(st.ToolResponse.Stderr) > maxToolOutputBytes+len(truncationMarker) {
		t.Errorf("Stderr len = %d, want at most %d", len(st.ToolResponse.Stderr), maxToolOutputBytes+len(truncationMarker))
	}
	if !strings.HasSuffix(st.ToolResponse.Stderr, truncationMarker) {
		t.Error("Stderr should end with truncation marker")
	}
}

func TestSanitizeCheckpoint_ToolCallArgsRedacted(t *testing.T) {
	cp := Checkpoint{
		RunID: "run_sanitize",
		Steps: []AgentStep{
			{
				StepID: "step_004",
				Type:   "tool",
				ToolCall: &ToolCall{
					Name: "file_write",
					Args: map[string]string{
						"path":    "/repo/main.go",
						"content": "package main\nfunc main() {}\n",
					},
				},
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	sanitized := SanitizeCheckpoint(cp)
	args := sanitized.Steps[0].ToolCall.Args

	// path should be preserved (whitelisted)
	if args["path"] != "/repo/main.go" {
		t.Errorf("path should be preserved, got %q", args["path"])
	}
	// content should be redacted
	if args["content"] != "<redacted>" {
		t.Errorf("content should be redacted, got %q", args["content"])
	}
}

func TestSanitizeCheckpoint_NoFileWriteContent(t *testing.T) {
	cp := Checkpoint{
		RunID: "run_sanitize",
		Steps: []AgentStep{
			{
				StepID: "step_005",
				Type:   "tool",
				ToolCall: &ToolCall{
					Name: "file_write",
					Args: map[string]string{
						"path":    "out.go",
						"content": "secret code here",
					},
				},
			},
			{
				StepID: "step_006",
				Type:   "tool",
				ToolCall: &ToolCall{
					Name: "file_patch",
					Args: map[string]string{
						"path": "src.go",
						"old":  "old code",
						"new":  "new code",
					},
				},
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	sanitized := SanitizeCheckpoint(cp)

	// file_write: content should be redacted
	writeArgs := sanitized.Steps[0].ToolCall.Args
	if writeArgs["content"] != "<redacted>" {
		t.Errorf("file_write content should be redacted, got %q", writeArgs["content"])
	}
	if writeArgs["path"] != "out.go" {
		t.Errorf("file_write path should be preserved, got %q", writeArgs["path"])
	}

	// file_patch: old and new should be redacted
	patchArgs := sanitized.Steps[1].ToolCall.Args
	if patchArgs["old"] != "<redacted>" {
		t.Errorf("file_patch old should be redacted, got %q", patchArgs["old"])
	}
	if patchArgs["new"] != "<redacted>" {
		t.Errorf("file_patch new should be redacted, got %q", patchArgs["new"])
	}
	if patchArgs["path"] != "src.go" {
		t.Errorf("file_patch path should be preserved, got %q", patchArgs["path"])
	}
}

func TestSanitizeCheckpoint_CompleteFileReadStdoutTruncated(t *testing.T) {
	// A file_read that returns complete file content should be truncated
	completeFile := strings.Repeat("line of code\n", 200) // well over 1024 bytes
	cp := Checkpoint{
		RunID: "run_sanitize",
		Steps: []AgentStep{
			{
				StepID: "step_007",
				Type:   "tool",
				ToolResponse: &tools.ToolResponse{
					ToolName: "file_read",
					Success:  true,
					Stdout:   completeFile,
				},
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	sanitized := SanitizeCheckpoint(cp)
	if !strings.HasSuffix(sanitized.Steps[0].ToolResponse.Stdout, truncationMarker) {
		t.Error("file_read stdout should be truncated")
	}
}

func TestSanitizeCheckpoint_PathRetained(t *testing.T) {
	cp := Checkpoint{
		RunID: "run_sanitize",
		Steps: []AgentStep{
			{
				StepID: "step_008",
				Type:   "tool",
				ToolCall: &ToolCall{
					Name: "file_read",
					Args: map[string]string{
						"path":      "/repo/README.md",
						"max_bytes": "4096",
					},
				},
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	sanitized := SanitizeCheckpoint(cp)
	args := sanitized.Steps[0].ToolCall.Args

	// Both path and max_bytes are whitelisted
	if args["path"] != "/repo/README.md" {
		t.Errorf("path should be preserved, got %q", args["path"])
	}
	if args["max_bytes"] != "4096" {
		t.Errorf("max_bytes should be preserved, got %q", args["max_bytes"])
	}
}

func TestSanitizeCheckpoint_NilFieldsHandled(t *testing.T) {
	cp := Checkpoint{
		RunID: "run_sanitize",
		Steps: []AgentStep{
			{
				StepID:    "step_009",
				Type:      "model",
				ModelText: "simple text",
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	sanitized := SanitizeCheckpoint(cp)
	if sanitized.Steps[0].ModelText != "simple text" {
		t.Errorf("ModelText should be preserved, got %q", sanitized.Steps[0].ModelText)
	}
	if sanitized.Steps[0].ToolCall != nil {
		t.Error("ToolCall should be nil")
	}
	if sanitized.Steps[0].ToolResponse != nil {
		t.Error("ToolResponse should be nil")
	}
}

func TestSanitizeCheckpoint_NilArgsHandled(t *testing.T) {
	cp := Checkpoint{
		RunID: "run_sanitize",
		Steps: []AgentStep{
			{
				StepID: "step_010",
				Type:   "tool",
				ToolCall: &ToolCall{
					Name: "file_read",
					Args: nil,
				},
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	sanitized := SanitizeCheckpoint(cp)
	if sanitized.Steps[0].ToolCall.Args != nil {
		t.Errorf("nil Args should remain nil, got %v", sanitized.Steps[0].ToolCall.Args)
	}
}

func TestSanitizeCheckpoint_CommandNamePreserved(t *testing.T) {
	cp := Checkpoint{
		RunID: "run_sanitize",
		Steps: []AgentStep{
			{
				StepID: "step_011",
				Type:   "tool",
				ToolCall: &ToolCall{
					Name: "shell_exec",
					Args: map[string]string{
						"command_name": "go test",
						"path":         "/repo",
						"env_vars":     "SECRET=leaked",
					},
				},
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	sanitized := SanitizeCheckpoint(cp)
	args := sanitized.Steps[0].ToolCall.Args

	if args["command_name"] != "go test" {
		t.Errorf("command_name should be preserved, got %q", args["command_name"])
	}
	if args["path"] != "/repo" {
		t.Errorf("path should be preserved, got %q", args["path"])
	}
	if args["env_vars"] != "<redacted>" {
		t.Errorf("env_vars should be redacted, got %q", args["env_vars"])
	}
}

// --- Original checkpoint store tests ---

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
