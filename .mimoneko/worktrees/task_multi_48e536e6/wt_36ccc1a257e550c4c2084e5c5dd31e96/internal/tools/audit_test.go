package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditLogWritesJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	audit, err := NewAuditLog(path)
	if err != nil {
		t.Fatalf("NewAuditLog() error = %v", err)
	}

	event := ToolAuditEvent{
		ID:          "test-123",
		ToolName:    "file_read",
		Success:     true,
		ExitCode:    0,
		DurationMs:  42,
		RiskLevel:   "low",
	}
	if err := audit.Record(event); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := audit.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var parsed ToolAuditEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if parsed.ToolName != "file_read" {
		t.Fatalf("tool_name = %q, want file_read", parsed.ToolName)
	}
}

func TestAuditLogDirectoryPermissions(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	path := filepath.Join(logDir, "audit.jsonl")

	audit, err := NewAuditLog(path)
	if err != nil {
		t.Fatalf("NewAuditLog() error = %v", err)
	}
	audit.Close()

	info, err := os.Stat(logDir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}

	// On Unix-like systems, verify 0700 permissions
	if !isWindows() {
		perm := info.Mode().Perm()
		if perm != 0o700 {
			t.Fatalf("directory permissions = %04o, want 0700", perm)
		}
	}
}

func TestAuditLogFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	audit, err := NewAuditLog(path)
	if err != nil {
		t.Fatalf("NewAuditLog() error = %v", err)
	}
	audit.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	// On Unix-like systems, verify 0600 permissions
	if !isWindows() {
		perm := info.Mode().Perm()
		if perm != 0o600 {
			t.Fatalf("file permissions = %04o, want 0600", perm)
		}
	}
}

// isWindows returns true when running on Windows.
func isWindows() bool {
	return os.PathSeparator == '\\'
}

func TestRedactArgs(t *testing.T) {
	args := map[string]string{
		"path":    "main.go",
		"content": "secret data",
	}
	redacted := redactArgs(args)
	if redacted["content"] != "<redacted>" {
		t.Fatalf("content should be redacted, got %q", redacted["content"])
	}
	if redacted["path"] != "main.go" {
		t.Fatalf("path should not be redacted, got %q", redacted["path"])
	}
}

func TestRedactArgsContentFields(t *testing.T) {
	args := map[string]string{
		"path":         "code.go",
		"command_name": "go-test",
		"max_bytes":    "1024",
		"create_dirs":  "true",
		"content":      "file content here",
		"old":          "old text",
		"new":          "new text",
		"patch":        "diff content",
		"diff":         "unified diff",
		"stdin":        "input data",
		"unknown_key":  "some value",
	}
	redacted := redactArgs(args)

	// Safe keys should be preserved
	safeKeys := []string{"path", "command_name", "max_bytes", "create_dirs"}
	for _, key := range safeKeys {
		if redacted[key] != args[key] {
			t.Fatalf("key %q should be preserved, got %q want %q", key, redacted[key], args[key])
		}
	}

	// Sensitive keys should be redacted
	sensitiveKeys := []string{"content", "old", "new", "patch", "diff", "stdin", "unknown_key"}
	for _, key := range sensitiveKeys {
		if redacted[key] != "<redacted>" {
			t.Fatalf("key %q should be redacted, got %q", key, redacted[key])
		}
	}
}

func TestRedactArgsFilePatchInAuditLog(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte("foo bar baz"), 0o644); err != nil {
		t.Fatal(err)
	}

	auditPath := filepath.Join(root, "audit.jsonl")
	audit, err := NewAuditLog(auditPath)
	if err != nil {
		t.Fatal(err)
	}

	registry := NewMemoryRegistry()
	_ = registry.Register(&FilePatchTool{})
	guard := NewSafetyGuard(ToolPolicy{
		DenyWritePaths: DefaultDenyWritePaths(),
		DenyReadPaths:  DefaultDenyReadPaths(),
	})

	rt := NewDefaultToolRuntime(registry, guard, audit, map[string]bool{"file_patch": true})

	_, err = rt.Run(context.Background(), ToolRequest{
		ToolName: "file_patch",
		RepoRoot: root,
		Args:     map[string]string{"path": "code.go", "old": "bar", "new": "BAR"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	audit.Close()

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}

	// Verify old and new are redacted in audit log
	raw := string(data)
	if strings.Contains(raw, `"old":"bar"`) {
		t.Fatal("audit log contains unredacted 'old' value")
	}
	if strings.Contains(raw, `"new":"BAR"`) {
		t.Fatal("audit log contains unredacted 'new' value")
	}
	// Verify path is still present
	if !strings.Contains(raw, `"path":"code.go"`) {
		t.Fatal("audit log should preserve path in cleartext")
	}
}

func TestRedactArgsFileWriteContentInAuditLog(t *testing.T) {
	root := t.TempDir()

	auditPath := filepath.Join(root, "audit.jsonl")
	audit, err := NewAuditLog(auditPath)
	if err != nil {
		t.Fatal(err)
	}

	registry := NewMemoryRegistry()
	_ = registry.Register(&FileWriteTool{})
	guard := NewSafetyGuard(ToolPolicy{
		DenyWritePaths: DefaultDenyWritePaths(),
		DenyReadPaths:  DefaultDenyReadPaths(),
	})

	rt := NewDefaultToolRuntime(registry, guard, audit, map[string]bool{"file_write": true})

	_, err = rt.Run(context.Background(), ToolRequest{
		ToolName: "file_write",
		RepoRoot: root,
		Args:     map[string]string{"path": "output.txt", "content": "secret payload"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	audit.Close()

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}

	raw := string(data)
	if strings.Contains(raw, `"content":"secret payload"`) {
		t.Fatal("audit log contains unredacted content value")
	}
	if !strings.Contains(raw, `"path":"output.txt"`) {
		t.Fatal("audit log should preserve path in cleartext")
	}
}

func TestRedactArgsNil(t *testing.T) {
	redacted := redactArgs(nil)
	if redacted != nil {
		t.Fatal("redactArgs(nil) should return nil")
	}
}

func TestGenerateAuditID(t *testing.T) {
	id1, err := generateAuditID()
	if err != nil {
		t.Fatalf("generateAuditID() error = %v", err)
	}
	id2, err := generateAuditID()
	if err != nil {
		t.Fatalf("generateAuditID() error = %v", err)
	}
	if id1 == id2 {
		t.Fatal("audit IDs should be unique")
	}
	if len(id1) != 32 { // 16 bytes hex
		t.Fatalf("audit ID length = %d, want 32", len(id1))
	}
}

func TestAuditLogMultipleRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	audit, err := NewAuditLog(path)
	if err != nil {
		t.Fatalf("NewAuditLog() error = %v", err)
	}

	for i := 0; i < 5; i++ {
		event := ToolAuditEvent{
			ID:       strings.Repeat("a", 32),
			ToolName: "file_read",
			Success:  true,
		}
		if err := audit.Record(event); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
	}
	audit.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 {
		t.Fatalf("line count = %d, want 5", len(lines))
	}
}
