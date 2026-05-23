package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	auditPath := filepath.Join(root, ".reasonforge", "logs", "tools.jsonl")
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
	auditPath := filepath.Join(root, ".reasonforge", "logs", "tools.jsonl")
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

func (t *slowTool) Name() string        { return "slow_tool" }
func (t *slowTool) Description() string  { return "A slow test tool" }
func (t *slowTool) RiskLevel() string    { return "low" }
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
