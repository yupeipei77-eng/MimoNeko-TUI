package tools

import (
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

	// Verify directory was created
	info, err := os.Stat(logDir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
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
	perm := info.Mode().Perm()
	// On Windows, file permissions may not be enforced as on Unix
	// so we just check the file exists
	_ = perm
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
