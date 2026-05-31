package tools

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ToolAuditEvent represents a single audit log entry for a tool execution.
type ToolAuditEvent struct {
	ID         string            `json:"id"`
	Timestamp  time.Time         `json:"timestamp"`
	ToolName   string            `json:"tool_name"`
	TaskID     string            `json:"task_id"`
	RepoRoot   string            `json:"repo_root"`
	ArgsRedacted map[string]string `json:"args_redacted"`
	Success    bool              `json:"success"`
	ExitCode   int               `json:"exit_code"`
	OutputBytes int              `json:"output_bytes"`
	Error      string            `json:"error,omitempty"`
	DurationMs int64             `json:"duration_ms"`
	RiskLevel  string            `json:"risk_level"`
	DryRun     bool              `json:"dry_run"`
}

// AuditLog writes tool execution events as JSONL.
type AuditLog struct {
	mu   sync.Mutex
	path string
	file *os.File
}

// NewAuditLog creates or opens the audit log at the given path.
// It ensures the parent directory exists with mode 0700 and the file with mode 0600.
func NewAuditLog(path string) (*AuditLog, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("tools: create audit dir %q: %w", dir, err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("tools: open audit log %q: %w", path, err)
	}

	return &AuditLog{path: path, file: f}, nil
}

// Record writes a ToolAuditEvent as a single JSONL line.
func (l *AuditLog) Record(event ToolAuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("tools: marshal audit event: %w", err)
	}
	data = append(data, '\n')

	if _, err := l.file.Write(data); err != nil {
		return fmt.Errorf("tools: write audit event: %w", err)
	}
	return nil
}

// Close closes the audit log file.
func (l *AuditLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// Path returns the file path of the audit log.
func (l *AuditLog) Path() string {
	return l.path
}

// generateAuditID creates a cryptographically random 16-byte hex string.
func generateAuditID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("tools: generate audit id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// redactArgs returns a copy of args with sensitive values replaced.
// Only safe metadata keys (path, command_name, max_bytes, create_dirs) are
// preserved in cleartext. All other keys are redacted because they may
// contain file content, patch text, or other sensitive material.
func redactArgs(args map[string]string) map[string]string {
	if args == nil {
		return nil
	}
	result := make(map[string]string, len(args))
	for k, v := range args {
		if isSafeAuditKey(k) {
			result[k] = v
		} else {
			result[k] = "<redacted>"
		}
	}
	return result
}

// isSafeAuditKey returns true if the argument key is safe to log in cleartext.
func isSafeAuditKey(key string) bool {
	switch key {
	case "path", "command_name", "max_bytes", "create_dirs":
		return true
	default:
		return false
	}
}

// DefaultAuditLogPath returns the default audit log path under repoRoot.
func DefaultAuditLogPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".nekonomimo", "logs", "tools.jsonl")
}
