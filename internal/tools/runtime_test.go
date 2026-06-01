package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mimoneko/mimoneko/internal/events"
)

func TestRuntimeUnknownTool(t *testing.T) {
	rt := newTestRuntime(t, t.TempDir())
	_, err := rt.Run(context.Background(), ToolRequest{
		ToolName: "nonexistent",
		RepoRoot: t.TempDir(),
	})
	if err == nil {
		t.Fatal("Run(unknown tool) should fail")
	}
}

func TestRuntimeDisabledTool(t *testing.T) {
	root := t.TempDir()
	registry := NewMemoryRegistry()
	_ = registry.Register(&FileReadTool{})

	guard := NewSafetyGuard(ToolPolicy{
		DenyWritePaths: DefaultDenyWritePaths(),
		DenyReadPaths:  DefaultDenyReadPaths(),
	})

	auditPath := filepath.Join(root, ".mimoneko", "logs", "tools.jsonl")
	audit, err := NewAuditLog(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	defer audit.Close()

	rt := NewDefaultToolRuntime(registry, guard, audit, map[string]bool{
		"file_read": false, // disabled
	})

	_, err = rt.Run(context.Background(), ToolRequest{
		ToolName: "file_read",
		RepoRoot: root,
		Args:     map[string]string{"path": "test.txt"},
	})
	if err == nil {
		t.Fatal("Run(disabled tool) should fail")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("error = %q, want 'disabled'", err.Error())
	}
}

func TestRuntimeMissingRepoRoot(t *testing.T) {
	root := t.TempDir()
	registry := NewMemoryRegistry()
	_ = registry.Register(&FileReadTool{})

	guard := NewSafetyGuard(ToolPolicy{})
	auditPath := filepath.Join(root, ".mimoneko", "logs", "tools.jsonl")
	audit, _ := NewAuditLog(auditPath)
	defer audit.Close()

	rt := NewDefaultToolRuntime(registry, guard, audit, map[string]bool{"file_read": true})

	_, err := rt.Run(context.Background(), ToolRequest{
		ToolName: "file_read",
		RepoRoot: "",
		Args:     map[string]string{"path": "test.txt"},
	})
	if err == nil {
		t.Fatal("Run(missing repo_root) should fail")
	}
}

func TestRuntimeMetadataRegistrationAndLookup(t *testing.T) {
	rt := newTestRuntime(t, t.TempDir())
	metadata := ToolMetadata{
		Name:             "runtime_observer",
		Description:      "Observe runtime metadata",
		RiskLevel:        RiskLevelLow,
		RequiresApproval: false,
	}

	if err := rt.RegisterMetadata(metadata); err != nil {
		t.Fatalf("RegisterMetadata() error = %v", err)
	}
	got, ok := rt.Metadata("runtime_observer")
	if !ok {
		t.Fatal("Metadata(runtime_observer) not found")
	}
	if got.Name != metadata.Name || got.RiskLevel != RiskLevelLow {
		t.Fatalf("metadata = %+v, want runtime_observer metadata", got)
	}
	if len(rt.ListMetadata()) == 0 {
		t.Fatal("ListMetadata() should include registered metadata")
	}
}

func TestRuntimeFileReadSuccess(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	rt := newTestRuntime(t, root)
	resp, err := rt.Run(context.Background(), ToolRequest{
		ToolName: "file_read",
		RepoRoot: root,
		Args:     map[string]string{"path": "hello.txt"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}
	if resp.Stdout != "world" {
		t.Fatalf("Stdout = %q, want 'world'", resp.Stdout)
	}
	if resp.AuditID == "" {
		t.Fatal("AuditID should be set")
	}
}

func TestRuntimeWritesAuditLog(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	auditPath := DefaultAuditLogPath(root)
	audit, err := NewAuditLog(auditPath)
	if err != nil {
		t.Fatal(err)
	}

	registry := NewMemoryRegistry()
	_ = registry.Register(&FileReadTool{})
	guard := NewSafetyGuard(ToolPolicy{
		DenyWritePaths: DefaultDenyWritePaths(),
		DenyReadPaths:  DefaultDenyReadPaths(),
	})

	rt := NewDefaultToolRuntime(registry, guard, audit, map[string]bool{"file_read": true})

	_, err = rt.Run(context.Background(), ToolRequest{
		ToolName: "file_read",
		RepoRoot: root,
		Args:     map[string]string{"path": "test.txt"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Close audit and check file
	audit.Close()

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Should have 2 entries: pre-execution and post-execution
	if len(lines) < 2 {
		t.Fatalf("audit log lines = %d, want >= 2", len(lines))
	}
}

func TestRuntimeOutputTruncation(t *testing.T) {
	root := t.TempDir()
	largeContent := strings.Repeat("x", 100000)
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(largeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	registry := NewMemoryRegistry()
	_ = registry.Register(&FileReadTool{})
	guard := NewSafetyGuard(ToolPolicy{
		MaxOutputBytes: 100,
		DenyReadPaths:  DefaultDenyReadPaths(),
	})
	auditPath := DefaultAuditLogPath(root)
	audit, _ := NewAuditLog(auditPath)
	defer audit.Close()

	rt := NewDefaultToolRuntime(registry, guard, audit, map[string]bool{"file_read": true})

	resp, err := rt.Run(context.Background(), ToolRequest{
		ToolName: "file_read",
		RepoRoot: root,
		Args:     map[string]string{"path": "big.txt"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !resp.Truncated {
		t.Fatal("response should be truncated")
	}
}

func TestRuntimeTimeout(t *testing.T) {
	root := t.TempDir()
	registry := NewMemoryRegistry()
	_ = registry.Register(&slowTool{})
	guard := NewSafetyGuard(ToolPolicy{
		DefaultTimeoutSeconds: 1,
	})
	auditPath := DefaultAuditLogPath(root)
	audit, _ := NewAuditLog(auditPath)
	defer audit.Close()

	rt := NewDefaultToolRuntime(registry, guard, audit, map[string]bool{"slow_tool": true})

	start := time.Now()
	resp, err := rt.Run(context.Background(), ToolRequest{
		ToolName: "slow_tool",
		RepoRoot: root,
	})
	elapsed := time.Since(start)

	// The tool should timeout
	if err == nil && resp.Success {
		if elapsed < 800*time.Millisecond {
			t.Fatal("slow tool should have timed out")
		}
	}
}

// slowTool is a test tool that sleeps for a long time.
type slowTool struct{}

func (t *slowTool) Name() string                  { return "slow_tool" }
func (t *slowTool) Description() string           { return "A slow test tool" }
func (t *slowTool) RiskLevel() string             { return "low" }
func (t *slowTool) Concurrency() ConcurrencyClass { return ConcurrencyReadOnly }
func (t *slowTool) Run(ctx context.Context, _ ToolRequest) (ToolResponse, error) {
	select {
	case <-time.After(30 * time.Second):
		return ToolResponse{Success: true}, nil
	case <-ctx.Done():
		return ToolResponse{Success: false, Error: "cancelled"}, ctx.Err()
	}
}

func newTestRuntime(t *testing.T, root string) *DefaultToolRuntime {
	t.Helper()
	registry := NewMemoryRegistry()
	testCmds := map[string]TestCommandDef{}
	if err := RegisterBuiltinTools(registry, testCmds); err != nil {
		t.Fatalf("RegisterBuiltinTools() error = %v", err)
	}

	guard := NewSafetyGuard(ToolPolicy{
		DenyWritePaths: DefaultDenyWritePaths(),
		DenyReadPaths:  DefaultDenyReadPaths(),
	})

	auditPath := DefaultAuditLogPath(root)
	audit, err := NewAuditLog(auditPath)
	if err != nil {
		t.Fatalf("NewAuditLog() error = %v", err)
	}
	t.Cleanup(func() { audit.Close() })

	enabled := map[string]bool{
		"file_read":  true,
		"file_write": true,
		"file_patch": true,
		"git_diff":   true,
		"test_run":   true,
	}

	return NewDefaultToolRuntime(registry, guard, audit, enabled)
}

// captureEmitter captures emitted events for test assertions.
type captureEmitter struct {
	events []events.RunEvent
	err    error // if set, Emit returns this error
}

func (c *captureEmitter) Emit(ctx context.Context, event events.RunEvent) error {
	c.events = append(c.events, event)
	return c.err
}

// failingEmitter always returns an error on Emit.
type failingEmitter struct {
	captured []events.RunEvent
}

func (f *failingEmitter) Emit(ctx context.Context, event events.RunEvent) error {
	f.captured = append(f.captured, event)
	return errors.New("emitter failure")
}

func newTestToolRuntimeWithEmitter(t *testing.T, emitter events.EventEmitter) *DefaultToolRuntime {
	t.Helper()
	registry := NewMemoryRegistry()
	testCmds := map[string]TestCommandDef{}
	if err := RegisterBuiltinTools(registry, testCmds); err != nil {
		t.Fatalf("RegisterBuiltinTools() error = %v", err)
	}

	root := t.TempDir()
	guard := NewSafetyGuard(ToolPolicy{
		DenyWritePaths: DefaultDenyWritePaths(),
		DenyReadPaths:  DefaultDenyReadPaths(),
	})

	auditPath := DefaultAuditLogPath(root)
	audit, err := NewAuditLog(auditPath)
	if err != nil {
		t.Fatalf("NewAuditLog() error = %v", err)
	}
	t.Cleanup(func() { audit.Close() })

	enabled := map[string]bool{
		"file_read":  true,
		"file_write": true,
		"file_patch": true,
		"git_diff":   true,
		"test_run":   true,
	}

	rt := NewDefaultToolRuntime(registry, guard, audit, enabled)
	if emitter != nil {
		rt.SetEventEmitter(emitter)
	}
	return rt
}

func TestToolRuntimeEmitsToolStartedFinished(t *testing.T) {
	emitter := &captureEmitter{}
	rt := newTestToolRuntimeWithEmitter(t, emitter)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	rc := events.RunContext{RunID: "run_tool_test", TaskID: "task_001"}
	ctx = events.WithRunContext(ctx, rc)

	resp, err := rt.Run(ctx, ToolRequest{
		ToolName: "file_read",
		RepoRoot: root,
		Args:     map[string]string{"path": "test.txt"},
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("Run() resp.Success = false, want true")
	}

	// Should have emitted tool.started and tool.finished
	if len(emitter.events) != 2 {
		t.Fatalf("expected 2 events (started + finished), got %d", len(emitter.events))
	}

	started := emitter.events[0]
	if started.Type != events.EventToolStarted {
		t.Errorf("first event type = %s, want tool.started", started.Type)
	}
	if started.Status != "started" {
		t.Errorf("first event status = %s, want started", started.Status)
	}
	if started.Source != "tool" {
		t.Errorf("first event source = %s, want tool", started.Source)
	}
	if started.RunID != "run_tool_test" {
		t.Errorf("first event RunID = %q, want run_tool_test", started.RunID)
	}
	if started.TaskID != "task_001" {
		t.Errorf("first event TaskID = %q, want task_001", started.TaskID)
	}

	finished := emitter.events[1]
	if finished.Type != events.EventToolFinished {
		t.Errorf("second event type = %s, want tool.finished", finished.Type)
	}
	if finished.Status != "succeeded" {
		t.Errorf("second event status = %s, want succeeded", finished.Status)
	}
	if finished.DurationMs < 0 {
		t.Errorf("second event DurationMs = %d, want >= 0", finished.DurationMs)
	}
	if finished.RunID != "run_tool_test" {
		t.Errorf("second event RunID = %q, want run_tool_test", finished.RunID)
	}
	if finished.TaskID != "task_001" {
		t.Errorf("second event TaskID = %q, want task_001", finished.TaskID)
	}
}

func TestToolRuntimeEventEmitterNilNoop(t *testing.T) {
	// Create ToolRuntime without setting an emitter (defaults to NoopEventEmitter)
	root := t.TempDir()
	registry := NewMemoryRegistry()
	testCmds := map[string]TestCommandDef{}
	if err := RegisterBuiltinTools(registry, testCmds); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	guard := NewSafetyGuard(ToolPolicy{})
	auditPath := DefaultAuditLogPath(root)
	auditLog, _ := NewAuditLog(auditPath)
	defer auditLog.Close()

	rt := NewDefaultToolRuntime(registry, guard, auditLog, map[string]bool{"file_read": true})
	// Don't call SetEventEmitter - should use NoopEventEmitter

	ctx := context.Background()
	rc := events.RunContext{RunID: "run_nil_test"}
	ctx = events.WithRunContext(ctx, rc)

	// Should not panic and should return a valid response
	resp, err := rt.Run(ctx, ToolRequest{
		ToolName: "file_read",
		RepoRoot: root,
		Args:     map[string]string{"path": "test.txt"},
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !resp.Success {
		t.Errorf("resp.Success = false, want true")
	}
}

func TestToolRuntimeEmitFailureDoesNotFailTool(t *testing.T) {
	emitter := &failingEmitter{}
	rt := newTestToolRuntimeWithEmitter(t, emitter)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	rc := events.RunContext{RunID: "run_emitfail_test"}
	ctx = events.WithRunContext(ctx, rc)

	// Even though the emitter returns an error, the tool should still succeed
	resp, err := rt.Run(ctx, ToolRequest{
		ToolName: "file_read",
		RepoRoot: root,
		Args:     map[string]string{"path": "test.txt"},
	})
	if err != nil {
		t.Fatalf("Run() error: %v (emit failure should not propagate)", err)
	}
	if !resp.Success {
		t.Errorf("resp.Success = false, want true (emit failure should not affect tool)")
	}

	// The emitter should still have received the events
	if len(emitter.captured) != 2 {
		t.Errorf("expected 2 events captured despite emit error, got %d", len(emitter.captured))
	}
}

func TestToolRuntimeEventsIncludeRunContext(t *testing.T) {
	emitter := &captureEmitter{}
	rt := newTestToolRuntimeWithEmitter(t, emitter)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	rc := events.RunContext{
		RunID:      "run_ctx_test",
		TaskID:     "task_ctx",
		WorktreeID: "wt_ctx",
	}
	ctx = events.WithRunContext(ctx, rc)

	_, err := rt.Run(ctx, ToolRequest{
		ToolName: "file_read",
		RepoRoot: root,
		TaskID:   "task_from_request",
		Args:     map[string]string{"path": "test.txt", "command_name": "go-test"},
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(emitter.events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(emitter.events))
	}

	for i, evt := range emitter.events {
		if evt.RunID != "run_ctx_test" {
			t.Errorf("event[%d].RunID = %q, want run_ctx_test", i, evt.RunID)
		}
		if evt.TaskID != "task_from_request" {
			t.Errorf("event[%d].TaskID = %q, want task_from_request", i, evt.TaskID)
		}
		if evt.WorktreeID != "wt_ctx" {
			t.Errorf("event[%d].WorktreeID = %q, want wt_ctx", i, evt.WorktreeID)
		}
	}

	// Verify tool_name metadata
	started := emitter.events[0]
	if started.Metadata["tool_name"] != "file_read" {
		t.Errorf("started metadata tool_name = %q, want file_read", started.Metadata["tool_name"])
	}
	// Verify command_name metadata from Args
	if started.Metadata["command_name"] != "go-test" {
		t.Errorf("started metadata command_name = %q, want go-test", started.Metadata["command_name"])
	}
}
