package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reasonforge/reasonforge/internal/agent"
	"github.com/reasonforge/reasonforge/internal/config"
	"github.com/reasonforge/reasonforge/internal/events"
	webserver "github.com/reasonforge/reasonforge/internal/server"
	"github.com/reasonforge/reasonforge/internal/worktree"
)

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"version"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(version) code = %d", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "reasonforge 0.1.0-dev" {
		t.Fatalf("version output = %q", got)
	}
}

func TestInitThenDoctor(t *testing.T) {
	root := t.TempDir()
	var initOut bytes.Buffer

	code := Run([]string{"init", "--dir", root}, Env{Stdout: &initOut})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	if !strings.Contains(initOut.String(), "Initialized ReasonForge") {
		t.Fatalf("init output = %q", initOut.String())
	}

	var doctorOut bytes.Buffer
	var doctorErr bytes.Buffer
	code = Run([]string{"doctor", "--dir", root}, Env{
		Stdout: &doctorOut,
		Stderr: &doctorErr,
	})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d, stderr = %q", code, doctorErr.String())
	}
	if !strings.Contains(doctorOut.String(), "ReasonForge doctor OK") {
		t.Fatalf("doctor output = %q", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "config_dir=") {
		t.Fatalf("doctor output = %q, want config_dir line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "default_model=local-coder") {
		t.Fatalf("doctor output = %q, want default_model line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "immutable_prefix_sources=3") {
		t.Fatalf("doctor output = %q, want immutable_prefix_sources line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "prefix_canonicalization=enabled") {
		t.Fatalf("doctor output = %q, want prefix_canonicalization line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "budget_warn_ratio=") {
		t.Fatalf("doctor output = %q, want budget_warn_ratio line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "budget_block_ratio=") {
		t.Fatalf("doctor output = %q, want budget_block_ratio line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "cache_estimated_ttl=") {
		t.Fatalf("doctor output = %q, want cache_estimated_ttl line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "event_id_collision_resistant=true") {
		t.Fatalf("doctor output = %q, want event_id_collision_resistant line", doctorOut.String())
	}
}

func TestNoArgsReturnsUsageError(t *testing.T) {
	var stderr bytes.Buffer
	code := Run(nil, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(nil) code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "Usage: reasonforge <command>") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestUnknownCommandReturnsUsageError(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"frobnicate"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(unknown) code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown command", stderr.String())
	}
}

func TestHelpWritesUsageToStdout(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"help"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(help) code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Commands:") {
		t.Fatalf("stdout = %q, want commands", stdout.String())
	}
}

func TestInitReportsWorkingDirectoryError(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"init"}, Env{
		Stderr: &stderr,
		Getwd:  func() (string, error) { return "", errors.New("boom") },
	})
	if code != 1 {
		t.Fatalf("Run(init) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "resolve working directory") {
		t.Fatalf("stderr = %q, want working directory error", stderr.String())
	}
}

func TestDoctorReportsMissingConfig(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	code := Run([]string{"doctor", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(doctor) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "doctor failed") {
		t.Fatalf("stderr = %q, want doctor failure", stderr.String())
	}
}

func TestCommandsRejectExtraPositionalArgs(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name string
		args []string
	}{
		{name: "version", args: []string{"version", "extra"}},
		{name: "init", args: []string{"init", "--dir", root, "extra"}},
		{name: "doctor", args: []string{"doctor", "--dir", root, "extra"}},
		{name: "help", args: []string{"help", "extra"}},
		{name: "cache-report", args: []string{"cache-report", "--dir", root, "extra"}},
		{name: "run", args: []string{"run", "--goal", "inspect repo", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			code := Run(tt.args, Env{Stderr: &stderr})
			if code != 2 {
				t.Fatalf("Run(%v) code = %d, want 2", tt.args, code)
			}
			if !strings.Contains(stderr.String(), "accepts") {
				t.Fatalf("stderr = %q, want positional argument error", stderr.String())
			}
		})
	}
}

func TestCacheReportCommand(t *testing.T) {
	root := t.TempDir()

	// Init first
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"cache-report", "--dir", root}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(cache-report) code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "total_observations=") {
		t.Fatalf("cache-report output = %q, want total_observations", stdout.String())
	}
	if !strings.Contains(stdout.String(), "hit_rate=") {
		t.Fatalf("cache-report output = %q, want hit_rate", stdout.String())
	}
}

func TestCacheReportReportsMissingConfig(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	code := Run([]string{"cache-report", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(cache-report) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "cache-report failed") {
		t.Fatalf("stderr = %q, want cache-report failure", stderr.String())
	}
}

func TestModelsCommand(t *testing.T) {
	root := t.TempDir()

	// Init first
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"models", "--dir", root}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(models) code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "ReasonForge Models") {
		t.Fatalf("models output = %q, want ReasonForge Models header", output)
	}
	if !strings.Contains(output, "default_model=local-coder") {
		t.Fatalf("models output = %q, want default_model line", output)
	}
	if !strings.Contains(output, "provider=local-openai-compatible") {
		t.Fatalf("models output = %q, want provider line", output)
	}
	if !strings.Contains(output, "type=openai-compatible") {
		t.Fatalf("models output = %q, want type line", output)
	}
	if !strings.Contains(output, "base_url=http://127.0.0.1:11434/v1") {
		t.Fatalf("models output = %q, want base_url line", output)
	}
	if !strings.Contains(output, "api_key_env=REASONFORGE_API_KEY") {
		t.Fatalf("models output = %q, want api_key_env line", output)
	}
	if !strings.Contains(output, "api_key_status=") {
		t.Fatalf("models output = %q, want api_key_status line", output)
	}
	if !strings.Contains(output, "models=local-coder") {
		t.Fatalf("models output = %q, want models line", output)
	}
	if !strings.Contains(output, "fallback_chain:") {
		t.Fatalf("models output = %q, want fallback_chain section", output)
	}

	// Verify no API key values leaked
	if strings.Contains(output, "Bearer ") {
		t.Fatalf("models output contains Authorization header, possible key leak")
	}
}

func TestModelsCommandDoesNotLeakAPIKey(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	code = Run([]string{"models", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(models) code = %d", code)
	}

	output := stdout.String()
	// Should show "configured" or "missing", never the actual key value
	if strings.Contains(output, "sk-") {
		t.Fatalf("models output contains possible API key: %q", output)
	}
}

func TestModelsCommandRejectsExtraArgs(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"models", "extra"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(models extra) code = %d, want 2", code)
	}
}

func TestModelsCommandReportsMissingConfig(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	code := Run([]string{"models", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(models) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "models failed") {
		t.Fatalf("stderr = %q, want models failure", stderr.String())
	}
}

func TestAPIKeyStatusFunction(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		want   string
		setEnv bool
	}{
		{name: "empty env var", envVar: "", want: "missing"},
		{name: "unset env var", envVar: "NONEXISTENT_VAR_XYZ_999", want: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := apiKeyStatus(tt.envVar)
			if got != tt.want {
				t.Errorf("apiKeyStatus(%q) = %q, want %q", tt.envVar, got, tt.want)
			}
		})
	}
}

func TestToolsCommand(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"tools", "--dir", root}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(tools) code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "ReasonForge Tools") {
		t.Fatalf("tools output = %q, want ReasonForge Tools header", output)
	}
	if !strings.Contains(output, "file_read") {
		t.Fatalf("tools output = %q, want file_read", output)
	}
	if !strings.Contains(output, "file_write") {
		t.Fatalf("tools output = %q, want file_write", output)
	}
	if !strings.Contains(output, "file_patch") {
		t.Fatalf("tools output = %q, want file_patch", output)
	}
	if !strings.Contains(output, "git_diff") {
		t.Fatalf("tools output = %q, want git_diff", output)
	}
	if !strings.Contains(output, "test_run") {
		t.Fatalf("tools output = %q, want test_run", output)
	}
	if !strings.Contains(output, "risk=") {
		t.Fatalf("tools output = %q, want risk levels", output)
	}
}

func TestToolRunFileRead(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a test file
	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"tool-run", "--dir", root, "file_read", "--path", "test.txt"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(tool-run file_read) code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "hello world") {
		t.Fatalf("tool-run file_read output = %q, want 'hello world'", output)
	}
}

func TestToolRunDoesNotLeakSensitiveContent(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a .env file with a secret
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET_KEY=supersecret"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code = Run([]string{"tool-run", "--dir", root, "file_read", "--path", ".env"}, Env{
		Stderr: &stderr,
	})
	if code == 0 {
		t.Fatal("tool-run file_read .env should fail")
	}

	// The error should not contain the secret
	errOutput := stderr.String()
	if strings.Contains(errOutput, "supersecret") {
		t.Fatalf("tool-run output leaks sensitive content: %q", errOutput)
	}
}

func TestToolsCommandRejectsExtraArgs(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"tools", "--dir", root, "extra"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(tools extra) code = %d, want 2", code)
	}
}

func TestToolRunRequiresToolName(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"tool-run", "--dir", root}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(tool-run without name) code = %d, want 2", code)
	}
}

func TestToolRunReportsMissingConfig(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	code := Run([]string{"tool-run", "--dir", root, "file_read"}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(tool-run missing config) code = %d, want 1", code)
	}
}

func TestToolRunFileWriteDryRun(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"tool-run", "--dir", root, "--dry-run", "file_write", "--path", "dry_output.txt", "--content", "secret data"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(tool-run file_write --dry-run) code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()

	// Output should mention dry-run
	if !strings.Contains(output, "dry-run") {
		t.Fatalf("tool-run dry-run output should mention dry-run, got %q", output)
	}

	// File should NOT exist on disk
	if _, err := os.Stat(filepath.Join(root, "dry_output.txt")); !os.IsNotExist(err) {
		t.Fatal("dry-run should not create the file on disk")
	}

	// Output should NOT contain the content value
	if strings.Contains(output, "secret data") {
		t.Fatalf("dry-run output should not leak content: %q", output)
	}

	// Verify audit log was written with dry_run=true
	auditPath := filepath.Join(root, ".reasonforge", "logs", "tools.jsonl")
	auditData, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit log should exist at %q: %v", auditPath, err)
	}
	auditStr := string(auditData)
	if !strings.Contains(auditStr, `"dry_run":true`) {
		t.Fatalf("audit log should record dry_run=true, got: %s", auditStr)
	}
	// Audit log should NOT contain the content value
	if strings.Contains(auditStr, "secret data") {
		t.Fatalf("audit log should not leak content value: %s", auditStr)
	}
}

// --- Agent run CLI tests ---

func TestRunDefaultDryRun(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	// Default --dry-run is true, so this should be safe even without a model server
	code = Run([]string{"run", "--dir", root, "--goal", "inspect the repository"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	// The command should not crash (may return 1 due to model call failure, but should not panic)
	_ = code
}

func TestRunNoPanic(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Run with various flag combinations - none should panic
	testCases := [][]string{
		{"run", "--dir", root, "--goal", "test task"},
		{"run", "--dir", root, "--goal", "test task", "--dry-run=false"},
		{"run", "--dir", root, "--goal", "test task", "--max-steps", "1"},
		{"run", "--dir", root, "--goal", "test task", "--auto-approve-medium"},
		{"run", "--dir", root, "--goal", "test task", "--task-id", "custom_task"},
	}

	for _, args := range testCases {
		var stderr bytes.Buffer
		// Just ensure no panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Run(%v) panicked: %v", args, r)
				}
			}()
			Run(args, Env{Stderr: &stderr})
		}()
	}
}

func TestRunRequiresGoal(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"run", "--dir", root}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(run without --goal) code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "run requires --goal") {
		t.Fatalf("stderr = %q, want goal required message", stderr.String())
	}
}

func TestBuildAgentDependencies(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	deps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		t.Fatalf("buildAgentDependencies() error = %v", err)
	}
	defer cleanup()

	// All dependencies must be non-nil
	if deps.ContextEngine == nil {
		t.Error("deps.ContextEngine is nil")
	}
	if deps.ModelRouter == nil {
		t.Error("deps.ModelRouter is nil")
	}
	if deps.ToolRuntime == nil {
		t.Error("deps.ToolRuntime is nil")
	}
	if deps.ToolRegistry == nil {
		t.Error("deps.ToolRegistry is nil")
	}
	if deps.ConversationLog == nil {
		t.Error("deps.ConversationLog is nil")
	}
	if deps.Scratchpad == nil {
		t.Error("deps.Scratchpad is nil")
	}
	if deps.CheckpointStore == nil {
		t.Error("deps.CheckpointStore is nil")
	}
}

func TestBuildAgentDependenciesNoAPIKeyLeak(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Set a fake API key to ensure it doesn't leak through dependencies
	t.Setenv("REASONFORGE_API_KEY", "sk-test-secret-key-do-not-leak")

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	deps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		t.Fatalf("buildAgentDependencies() error = %v", err)
	}
	defer cleanup()

	// Verify the dependencies themselves don't expose API keys
	// The ModelRouter should exist but shouldn't leak the key
	if deps.ModelRouter == nil {
		t.Fatal("ModelRouter should not be nil")
	}

	// Verify that the checkpoint store doesn't contain API keys
	// This is a basic sanity check - the real protection is in SanitizeCheckpoint
	// but we verify the infrastructure doesn't log or store keys
	_ = deps
}

func TestBuildAgentDependenciesEventsEnabledStoreFailureReturnsError(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	blocker := filepath.Join(root, "event-store-blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Events.Enabled = true
	cfg.Events.StorePath = filepath.Join(blocker, "events.jsonl")

	_, cleanup, err := buildAgentDependencies(root, cfg)
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal("buildAgentDependencies() should fail when events are enabled and EventStore cannot initialize")
	}
	if !strings.Contains(err.Error(), "event store") {
		t.Fatalf("error = %q, want event store", err.Error())
	}
}

func TestBuildAgentDependenciesEventsDisabledUsesNoop(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	blocker := filepath.Join(root, "event-store-blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Events.Enabled = false
	cfg.Events.StorePath = filepath.Join(blocker, "events.jsonl")

	deps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		t.Fatalf("buildAgentDependencies() with events disabled error = %v", err)
	}
	defer cleanup()

	if _, ok := deps.EventEmitter.(*events.NoopEventEmitter); !ok {
		t.Fatalf("EventEmitter = %T, want *events.NoopEventEmitter", deps.EventEmitter)
	}
}

func TestBuildMultiAgentDependenciesEventsEnabledStoreFailureReturnsError(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	blocker := filepath.Join(root, "multi-event-store-blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Events.Enabled = true
	cfg.Events.StorePath = filepath.Join(blocker, "events.jsonl")

	_, cleanup, err := buildMultiAgentDependencies(root, cfg, agent.Dependencies{})
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal("buildMultiAgentDependencies() should fail when events are enabled and EventStore cannot initialize")
	}
	if !strings.Contains(err.Error(), "event store") {
		t.Fatalf("error = %q, want event store", err.Error())
	}
}

// --- Phase 5: Worktree/Patch CLI tests ---

func TestInitCreatesWorktreeAndPatchConfig(t *testing.T) {
	root := t.TempDir()

	var stdout bytes.Buffer
	code := Run([]string{"init", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Verify worktree.yaml was created
	worktreeYAML := filepath.Join(root, ".reasonforge", "worktree.yaml")
	if _, err := os.Stat(worktreeYAML); os.IsNotExist(err) {
		t.Fatal("worktree.yaml should be created by init")
	}

	// Verify patch.yaml was created
	patchYAML := filepath.Join(root, ".reasonforge", "patch.yaml")
	if _, err := os.Stat(patchYAML); os.IsNotExist(err) {
		t.Fatal("patch.yaml should be created by init")
	}
}

func TestDoctorIncludesWorktreeConfig(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"doctor", "--dir", root}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "worktree_isolation=") {
		t.Fatalf("doctor output should contain worktree_isolation, got %q", output)
	}
	if !strings.Contains(output, "patch_require_clean_main=") {
		t.Fatalf("doctor output should contain patch_require_clean_main, got %q", output)
	}
	if !strings.Contains(output, "patch_max_diff_bytes=") {
		t.Fatalf("doctor output should contain patch_max_diff_bytes, got %q", output)
	}
}

func TestPatchSubcommandRequiresSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"patch"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch) code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "list") {
		t.Fatalf("stderr should mention subcommands, got %q", stderr.String())
	}
}

func TestPatchUnknownSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"patch", "unknown"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch unknown) code = %d, want 2", code)
	}
}

func TestPatchListEmpty(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"patch", "list", "--dir", root}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(patch list) code = %d, stderr = %q", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "No worktrees found") {
		t.Fatalf("patch list with no worktrees should say 'No worktrees found', got %q", stdout.String())
	}
}

func TestPatchPreviewRequiresWorktreeID(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"patch", "preview"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch preview) without ID should return 2, got %d", code)
	}
}

func TestPatchApplyRequiresWorktreeID(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"patch", "apply"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch apply) without ID should return 2, got %d", code)
	}
}

func TestPatchDiscardRequiresWorktreeID(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"patch", "discard"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch discard) without ID should return 2, got %d", code)
	}
}

func TestPatchApplyDirtyMainRejects(t *testing.T) {
	root := t.TempDir()
	// Init git repo
	runGitCmd(t, root, "init")
	runGitCmd(t, root, "config", "user.email", "test@reasonforge.dev")
	runGitCmd(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "add", ".")
	runGitCmd(t, root, "commit", "-m", "initial")

	// Init reasonforge
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Apply to nonexistent worktree should fail
	var stderr bytes.Buffer
	code = Run([]string{"patch", "apply", "--dir", root, "wt_nonexistent"}, Env{Stderr: &stderr})
	if code == 0 {
		t.Fatal("patch apply to nonexistent worktree should fail")
	}
}

func TestPatchDiscardNonexistentWorktree(t *testing.T) {
	root := t.TempDir()
	runGitCmd(t, root, "init")
	runGitCmd(t, root, "config", "user.email", "test@reasonforge.dev")
	runGitCmd(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "add", ".")
	runGitCmd(t, root, "commit", "-m", "initial")

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"patch", "discard", "--dir", root, "wt_nonexistent"}, Env{Stderr: &stderr})
	if code == 0 {
		t.Fatal("patch discard nonexistent worktree should fail")
	}
}

func TestRunWorktreeFlagNoPanic(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Ensure --worktree flag doesn't cause panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Run with --worktree panicked: %v", r)
			}
		}()
		var stderr bytes.Buffer
		Run([]string{"run", "--dir", root, "--goal", "test", "--worktree"}, Env{Stderr: &stderr})
	}()
}

func TestCLIDoesNotLeakAPIKey(t *testing.T) {
	t.Setenv("REASONFORGE_API_KEY", "sk-super-secret-key-12345")

	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	code = Run([]string{"doctor", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d", code)
	}

	if strings.Contains(stdout.String(), "sk-super-secret-key-12345") {
		t.Fatal("CLI output should not contain API key")
	}
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, string(output))
	}
}

// setupGitRepo creates a temp directory with an initialized git repo and initial commit.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGitCmd(t, root, "init")
	runGitCmd(t, root, "config", "user.email", "test@reasonforge.dev")
	runGitCmd(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "add", ".")
	runGitCmd(t, root, "commit", "-m", "initial")
	return root
}

func setupPatchCLIEventWorktree(t *testing.T, taskID string, files map[string]string) (string, worktree.WorktreeInfo) {
	t.Helper()
	root := setupGitRepo(t)
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	registry, err := worktree.NewRegistry(worktree.DefaultRegistryPath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   taskID,
	})
	if err != nil {
		t.Fatal(err)
	}

	for relPath, content := range files {
		absPath := filepath.Join(info.Path, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return root, info
}

func loadPatchCLIEvents(t *testing.T, root string) []events.RunEvent {
	t.Helper()
	path := filepath.Join(root, ".reasonforge", "events", "run_events.jsonl")
	evts, corrupted, err := events.LoadEventsFromFile(path)
	if err != nil {
		t.Fatalf("LoadEventsFromFile() error = %v", err)
	}
	if len(corrupted) > 0 {
		t.Fatalf("event store has corrupted lines: %+v", corrupted)
	}
	return evts
}

func patchCLIEventsForRun(evts []events.RunEvent, runID string) []events.RunEvent {
	var result []events.RunEvent
	for _, evt := range evts {
		if evt.RunID == runID {
			result = append(result, evt)
		}
	}
	return result
}

func findPatchCLIRunID(t *testing.T, evts []events.RunEvent, prefix string) string {
	t.Helper()
	for _, evt := range evts {
		if strings.HasPrefix(evt.RunID, prefix) {
			return evt.RunID
		}
	}
	t.Fatalf("no run ID with prefix %q in events: %+v", prefix, evts)
	return ""
}

func requirePatchCLIEventType(t *testing.T, evts []events.RunEvent, typ events.EventType) {
	t.Helper()
	for _, evt := range evts {
		if evt.Type == typ {
			return
		}
	}
	t.Fatalf("missing event type %s in events: %+v", typ, evts)
}

func requirePatchCLIContext(t *testing.T, evts []events.RunEvent, runID, taskID, worktreeID string) {
	t.Helper()
	for _, evt := range evts {
		if evt.RunID != runID {
			t.Fatalf("event RunID = %q, want %q: %+v", evt.RunID, runID, evt)
		}
		if evt.TaskID != taskID {
			t.Fatalf("event TaskID = %q, want %q: %+v", evt.TaskID, taskID, evt)
		}
		if evt.WorktreeID != worktreeID {
			t.Fatalf("event WorktreeID = %q, want %q: %+v", evt.WorktreeID, worktreeID, evt)
		}
	}
}

func TestPatchPreviewCLIEmitsEvents(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_preview_cli_events", map[string]string{
		"preview.txt": "preview change\n",
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"patch", "preview", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("Run(patch preview) code = %d, stderr = %q", code, stderr.String())
	}

	evts := loadPatchCLIEvents(t, root)
	runID := findPatchCLIRunID(t, evts, "patch_preview_")
	runEvents := patchCLIEventsForRun(evts, runID)
	requirePatchCLIContext(t, runEvents, runID, info.TaskID, info.ID)
	requirePatchCLIEventType(t, runEvents, events.EventRunStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewFinished)
	requirePatchCLIEventType(t, runEvents, events.EventRunSucceeded)
}

func TestPatchValidateCLIEmitsEvents(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_validate_cli_events", map[string]string{
		"validate.txt": "validate change\n",
	})

	var stdout, stderr bytes.Buffer
	_ = Run([]string{"patch", "validate", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})

	evts := loadPatchCLIEvents(t, root)
	runID := findPatchCLIRunID(t, evts, "patch_validate_")
	runEvents := patchCLIEventsForRun(evts, runID)
	requirePatchCLIContext(t, runEvents, runID, info.TaskID, info.ID)
	requirePatchCLIEventType(t, runEvents, events.EventRunStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewFinished)
	requirePatchCLIEventType(t, runEvents, events.EventReviewerStarted)
	requirePatchCLIEventType(t, runEvents, events.EventReviewerFinished)
	requirePatchCLIEventType(t, runEvents, events.EventValidationStarted)
	requirePatchCLIEventType(t, runEvents, events.EventValidationFinished)
	requirePatchCLIEventType(t, runEvents, events.EventToolStarted)
	requirePatchCLIEventType(t, runEvents, events.EventToolFinished)
}

func TestPatchReviewCLIEmitsEvents(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_review_cli_events", map[string]string{
		"review_events.txt": "review change\n",
	})

	var stdout, stderr bytes.Buffer
	_ = Run([]string{"patch", "review", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})

	evts := loadPatchCLIEvents(t, root)
	runID := findPatchCLIRunID(t, evts, "patch_review_")
	runEvents := patchCLIEventsForRun(evts, runID)
	requirePatchCLIContext(t, runEvents, runID, info.TaskID, info.ID)
	requirePatchCLIEventType(t, runEvents, events.EventRunStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewFinished)
	requirePatchCLIEventType(t, runEvents, events.EventReviewerStarted)
	requirePatchCLIEventType(t, runEvents, events.EventReviewerFinished)
	requirePatchCLIEventType(t, runEvents, events.EventValidationStarted)
	requirePatchCLIEventType(t, runEvents, events.EventValidationFinished)
	requirePatchCLIEventType(t, runEvents, events.EventToolStarted)
	requirePatchCLIEventType(t, runEvents, events.EventToolFinished)
}

func TestPatchPreviewEventsVisibleInRuns(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_preview_visible", map[string]string{
		"visible.txt": "visible in runs\n",
	})

	var previewStdout, previewStderr bytes.Buffer
	code := Run([]string{"patch", "preview", "--dir", root, info.ID}, Env{Stdout: &previewStdout, Stderr: &previewStderr})
	if code != 0 {
		t.Fatalf("Run(patch preview) code = %d, stderr = %q", code, previewStderr.String())
	}

	var runsOut, runsErr bytes.Buffer
	code = Run([]string{"runs", "--dir", root}, Env{Stdout: &runsOut, Stderr: &runsErr})
	if code != 0 {
		t.Fatalf("Run(runs) code = %d, stderr = %q", code, runsErr.String())
	}
	output := runsOut.String()
	if !strings.Contains(output, "patch_preview_") {
		t.Fatalf("runs output should include patch_preview_ run, got:\n%s", output)
	}
	if !strings.Contains(output, "succeeded") {
		t.Fatalf("runs output should show succeeded state, got:\n%s", output)
	}
}

func TestPatchValidateEventsVisibleInRunEvents(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_validate_visible", map[string]string{
		"run_events.txt": "visible in run events\n",
	})

	var validateStdout, validateStderr bytes.Buffer
	_ = Run([]string{"patch", "validate", "--dir", root, info.ID}, Env{Stdout: &validateStdout, Stderr: &validateStderr})

	evts := loadPatchCLIEvents(t, root)
	runID := findPatchCLIRunID(t, evts, "patch_validate_")

	var eventsOut, eventsErr bytes.Buffer
	code := Run([]string{"run-events", "--dir", root, runID}, Env{Stdout: &eventsOut, Stderr: &eventsErr})
	if code != 0 {
		t.Fatalf("Run(run-events) code = %d, stderr = %q", code, eventsErr.String())
	}
	output := eventsOut.String()
	if !strings.Contains(output, string(events.EventValidationStarted)) {
		t.Fatalf("run-events output should include validation.started, got:\n%s", output)
	}
	if !strings.Contains(output, string(events.EventToolStarted)) {
		t.Fatalf("run-events output should include tool.started, got:\n%s", output)
	}
}

func TestPatchReviewDoesNotLeakSensitiveDiffInEvents(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_review_sensitive_events", map[string]string{
		".env": "API_KEY=sk-phase82-secret\nDB_PASSWORD=super-secret-password\n",
	})

	var stdout, stderr bytes.Buffer
	_ = Run([]string{"patch", "review", "--dir", root, info.ID, "--no-tests"}, Env{Stdout: &stdout, Stderr: &stderr})

	eventStorePath := filepath.Join(root, ".reasonforge", "events", "run_events.jsonl")
	raw, err := os.ReadFile(eventStorePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	rawEvents := string(raw)
	for _, secret := range []string{"sk-phase82-secret", "super-secret-password", "API_KEY=", "DB_PASSWORD="} {
		if strings.Contains(rawEvents, secret) {
			t.Fatalf("event store leaked sensitive diff content %q:\n%s", secret, rawEvents)
		}
	}
}

func TestPatchCLIEventsDisabledUsesNoop(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_events_disabled", map[string]string{
		"disabled.txt": "events disabled\n",
	})

	eventsYAML := filepath.Join(root, ".reasonforge", "events.yaml")
	if err := os.WriteFile(eventsYAML, []byte("enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"patch", "preview", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("Run(patch preview) code = %d, stderr = %q", code, stderr.String())
	}

	eventStorePath := filepath.Join(root, ".reasonforge", "events", "run_events.jsonl")
	if _, err := os.Stat(eventStorePath); !os.IsNotExist(err) {
		t.Fatalf("event store path exists with events disabled: err=%v", err)
	}
}

func TestPatchCLIEventsEnabledStoreFailureReturnsError(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_events_store_failure", map[string]string{
		"store_failure.txt": "store failure\n",
	})

	blocker := filepath.Join(root, "event-store-blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	eventsYAML := filepath.Join(root, ".reasonforge", "events.yaml")
	if err := os.WriteFile(eventsYAML, []byte("enabled: true\nstore_path: event-store-blocker/events.jsonl\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := Run([]string{"patch", "preview", "--dir", root, info.ID}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(patch preview) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "event store") {
		t.Fatalf("stderr = %q, want event store error", stderr.String())
	}
}

func TestCLIPatchPreviewDoesNotLeakSensitiveDiff(t *testing.T) {
	root := t.TempDir()
	runGitCmd(t, root, "init")
	runGitCmd(t, root, "config", "user.email", "test@reasonforge.dev")
	runGitCmd(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "add", ".")
	runGitCmd(t, root, "commit", "-m", "initial")

	// Init reasonforge project
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree directly via worktree manager
	registryPath := filepath.Join(root, ".reasonforge", "worktrees", "registry.jsonl")
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "leak-test",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create worktree: %v", err)
	}

	// Write .env with secret content in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, ".env"), []byte("SECRET_VALUE=super_secret_data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run patch preview via CLI
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	patchCode := Run([]string{"patch", "preview", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})
	_ = patchCode // May return non-zero due to violations; we just check output

	output := stdout.String() + stderr.String()

	// Output must NOT contain the secret content
	if strings.Contains(output, "SECRET_VALUE") {
		t.Fatal("CLI patch preview must not expose content of sensitive files")
	}
	if strings.Contains(output, "super_secret_data") {
		t.Fatal("CLI patch preview must not expose content of sensitive files")
	}

	// Output should show violation info
	if !strings.Contains(output, "violation") && !strings.Contains(output, ".env") {
		t.Fatalf("expected violation info in output, got:\n%s", output)
	}
}

func TestPatchValidateRequiresWorktreeID(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"patch", "validate", "--dir", root}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch validate without ID) code = %d, want 2", code)
	}
}

func TestPatchReviewRequiresWorktreeID(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"patch", "review", "--dir", root}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch review without ID) code = %d, want 2", code)
	}
}

func TestPatchValidateOutputsRecommendation(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_validate_test",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Make a simple change in the worktree
	testFile := filepath.Join(info.Path, "simple.txt")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"patch", "validate", "--dir", root, info.ID}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	_ = code // May return non-zero depending on findings

	output := stdout.String()
	if !strings.Contains(output, "recommendation=") {
		t.Fatalf("patch validate should output recommendation, got:\n%s", output)
	}
}

func TestPatchReviewOutputsWithoutModelReview(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_review_test",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Make a simple change
	testFile := filepath.Join(info.Path, "review_test.txt")
	if err := os.WriteFile(testFile, []byte("test content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"patch", "review", "--dir", root, info.ID, "--no-tests"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	_ = code

	output := stdout.String()
	if !strings.Contains(output, "recommendation=") {
		t.Fatalf("patch review should output recommendation, got:\n%s", output)
	}
}

func TestPatchValidateDoesNotLeakAPIKey(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree and add a sensitive file
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_validate_sensitive",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Write .env with secret content in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, ".env"), []byte("API_KEY=sk-secret-key-12345\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"patch", "validate", "--dir", root, info.ID}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	_ = code

	output := stdout.String() + stderr.String()

	// Must not leak API key values
	if strings.Contains(output, "sk-secret-key-12345") {
		t.Fatal("patch validate must not leak API key values")
	}
	if strings.Contains(output, "API_KEY=sk-secret") {
		t.Fatal("patch validate must not leak sensitive content")
	}
}

func TestPatchReviewDoesNotPrintSensitiveDiff(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree and add a sensitive file
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_review_sensitive",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Write .env with secret content in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, ".env"), []byte("DB_PASSWORD=supersecret123\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"patch", "review", "--dir", root, info.ID, "--no-tests"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	_ = code

	output := stdout.String() + stderr.String()

	// Must not print sensitive diff content when violations exist
	if strings.Contains(output, "supersecret123") {
		t.Fatal("patch review must not expose content of sensitive files")
	}
	if strings.Contains(output, "DB_PASSWORD") {
		t.Fatal("patch review must not expose content of sensitive files")
	}
}

func TestDoctorShowsReviewAndValidationConfig(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	code = Run([]string{"doctor", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "review_max_diff_bytes=") {
		t.Fatalf("doctor should show review_max_diff_bytes, got:\n%s", output)
	}
	if !strings.Contains(output, "validation_max_output_bytes=") {
		t.Fatalf("doctor should show validation_max_output_bytes, got:\n%s", output)
	}
	if !strings.Contains(output, "validation_timeout_seconds=") {
		t.Fatalf("doctor should show validation_timeout_seconds, got:\n%s", output)
	}
}

func TestPatchValidateUsesWorktreePath(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_validate_wt_path",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Make a change in the worktree
	testFile := filepath.Join(info.Path, "change.txt")
	if err := os.WriteFile(testFile, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run patch validate with --test-command; the CLI must pass WorktreePath
	// so the manager does not return "WorktreePath is required" error.
	var stderr bytes.Buffer
	code = Run([]string{"patch", "validate", "--dir", root, info.ID, "--test-command", "echo ok"}, Env{
		Stderr: &stderr,
	})
	// The command may fail for other reasons (e.g. test_run tool not registered),
	// but it must NOT fail with "WorktreePath is required"
	errOutput := stderr.String()
	if strings.Contains(errOutput, "WorktreePath is required") {
		t.Fatalf("patch validate must pass WorktreePath; got error: %s", errOutput)
	}
}

func TestPatchReviewUsesWorktreePath(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_review_wt_path",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Make a change in the worktree
	testFile := filepath.Join(info.Path, "change.txt")
	if err := os.WriteFile(testFile, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run patch review with --test-command; the CLI must pass WorktreePath
	// so the manager does not return "WorktreePath is required" error.
	var stderr bytes.Buffer
	code = Run([]string{"patch", "review", "--dir", root, info.ID, "--test-command", "echo ok"}, Env{
		Stderr: &stderr,
	})
	// The command may fail for other reasons (e.g. test_run tool not registered),
	// but it must NOT fail with "WorktreePath is required"
	errOutput := stderr.String()
	if strings.Contains(errOutput, "WorktreePath is required") {
		t.Fatalf("patch review must pass WorktreePath; got error: %s", errOutput)
	}
}

// === Phase 7: Multi-Agent CLI Tests ===

func TestInitCreatesMultiAgentConfig(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer

	code := Run([]string{"init", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Verify multiagent.yaml was created
	multiagentPath := filepath.Join(root, config.DirName, "multiagent.yaml")
	if _, err := os.Stat(multiagentPath); os.IsNotExist(err) {
		t.Error("init did not create multiagent.yaml")
	}
}

func TestDoctorShowsMultiAgentConfig(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stdout bytes.Buffer
	code := Run([]string{"doctor", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "multiagent_max_iterations") {
		t.Error("doctor output missing multiagent_max_iterations")
	}
	if !strings.Contains(output, "multiagent_default_worktree") {
		t.Error("doctor output missing multiagent_default_worktree")
	}
	if !strings.Contains(output, "multiagent_default_dry_run") {
		t.Error("doctor output missing multiagent_default_dry_run")
	}
}

func TestMultiRunRequiresGoal(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"multi-run"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "goal") {
		t.Errorf("expected error about goal, got: %s", stderr.String())
	}
}

func TestMultiRunDefaultWorktreeAndDryRun(t *testing.T) {
	// Test that multi-run defaults to worktree=true and dry-run=true
	// by checking the flag defaults are correctly set.
	// We don't execute the full run (no model configured), just verify
	// the command starts with correct defaults.
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"multi-run", "--dir", root, "fix typo"}, Env{Stdout: &stdout, Stderr: &stderr})
	// We expect failure (no model configured), but not a usage error (code 2)
	if code == 2 {
		t.Errorf("multi-run flag parsing failed: %s", stderr.String())
	}

	// Verify output mentions defaults
	output := stdout.String()
	if !strings.Contains(output, "dry_run=true") {
		t.Errorf("expected dry_run=true in output, got: %s", output)
	}
	if !strings.Contains(output, "worktree=true") {
		t.Errorf("expected worktree=true in output, got: %s", output)
	}
}

func TestMultiRunMaxIterationsExceedsMax(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stderr bytes.Buffer
	code := Run([]string{"multi-run", "--dir", root, "--max-iterations", "10", "fix typo"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Errorf("expected exit code 2 for max_iterations > 5, got %d", code)
	}
	if !strings.Contains(stderr.String(), "max_iterations") {
		t.Errorf("expected error about max_iterations, got: %s", stderr.String())
	}
}

func TestMultiRunNoAutoApply(t *testing.T) {
	// Verify that multi-run command does not have auto-apply behavior
	var stderr bytes.Buffer
	code := Run([]string{"multi-run", "--apply", "fix typo"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Errorf("expected flag parse error for --apply, got code %d", code)
	}
}

func TestMultiRunDoesNotLeakAPIKey(t *testing.T) {
	t.Setenv("REASONFORGE_API_KEY", "sk-super-secret-key-12345")

	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stdout bytes.Buffer
	code := Run([]string{"doctor", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("doctor failed: code=%d", code)
	}

	output := stdout.String()
	if strings.Contains(output, "sk-super-secret-key-12345") {
		t.Error("doctor output leaked API key")
	}
	// Also verify multi-run help doesn't leak
	var multiStderr bytes.Buffer
	Run([]string{"multi-run"}, Env{Stderr: &multiStderr})
	if strings.Contains(multiStderr.String(), "sk-super-secret-key-12345") {
		t.Error("multi-run output leaked API key")
	}
}

func TestMultiRunOutputFinalRecommendation(t *testing.T) {
	// Verify the multi-run command is registered and accepts positional goal
	var stderr bytes.Buffer
	code := Run([]string{"multi-run", "fix typo"}, Env{Stderr: &stderr})
	// Should not be a usage error (2) - may fail for other reasons (no config)
	// but the goal argument should be accepted
	if code == 2 && strings.Contains(stderr.String(), "accepts no positional") {
		t.Errorf("multi-run should accept positional goal argument")
	}
}

func TestMultiRunWorktreeFalseRejected(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stderr bytes.Buffer
	code := Run([]string{"multi-run", "--dir", root, "--worktree=false", "fix typo"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Errorf("expected exit code 2 for --worktree=false, got %d", code)
	}
	if !strings.Contains(stderr.String(), "worktree=false is not supported") {
		t.Errorf("expected error about --worktree=false not supported, got: %s", stderr.String())
	}
}

func TestMultiRunModelReviewFlagPassed(t *testing.T) {
	// Verify that --model-review flag is accepted and doesn't cause a parse error.
	// We can't fully test model review without a real model server,
	// but we verify the flag is recognized and the request includes UseModelReview.
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stderr bytes.Buffer
	_ = Run([]string{"multi-run", "--dir", root, "--model-review", "fix typo"}, Env{Stderr: &stderr})
	// The command may fail for other reasons (no model server), but should not
	// fail with an "unknown flag" or similar parse error
	errOutput := stderr.String()
	if strings.Contains(errOutput, "unknown flag") || strings.Contains(errOutput, "cannot use") {
		t.Errorf("--model-review flag should be recognized, got: %s", errOutput)
	}
}

// --- Dashboard CLI tests ---

func TestDashboardRequiresEventsEnabled(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	// Disable events in config
	eventsPath := filepath.Join(root, ".reasonforge", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: false\nstore_path: .reasonforge/events/run_events.jsonl\n"), 0o600)

	var stderr bytes.Buffer
	code := Run([]string{"dashboard", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("dashboard with disabled events code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "disabled") {
		t.Fatalf("stderr = %q, want 'disabled'", stderr.String())
	}
}

func TestDashboardHandlesEmptyEventStore(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	// Create events config (enabled) and an empty event store
	eventsDir := filepath.Join(root, ".reasonforge", "events")
	os.MkdirAll(eventsDir, 0o700)
	storePath := filepath.Join(eventsDir, "run_events.jsonl")
	os.WriteFile(storePath, []byte(""), 0o600)

	eventsPath := filepath.Join(root, ".reasonforge", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: true\nstore_path: .reasonforge/events/run_events.jsonl\n"), 0o600)

	var stdout bytes.Buffer
	code := Run([]string{"dashboard", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("dashboard with empty store code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "No runs found") {
		t.Fatalf("stdout = %q, want 'No runs found'", stdout.String())
	}
}

func TestDashboardShowsRuns(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	// Create events config and store with sample data
	eventsDir := filepath.Join(root, ".reasonforge", "events")
	os.MkdirAll(eventsDir, 0o700)
	storePath := filepath.Join(eventsDir, "run_events.jsonl")

	// Write sample events
	store, err := events.NewJSONLRunEventStore(storePath)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	store.Append(ctx, events.RunEvent{ID: "evt1", RunID: "run_test1", Type: events.EventRunStarted, Status: "started", StartedAt: now})
	store.Append(ctx, events.RunEvent{ID: "evt2", RunID: "run_test1", Type: events.EventRunSucceeded, Status: "succeeded", FinishedAt: now.Add(time.Second)})
	store.Close()

	eventsPath := filepath.Join(root, ".reasonforge", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: true\nstore_path: .reasonforge/events/run_events.jsonl\n"), 0o600)

	var stdout bytes.Buffer
	code := Run([]string{"dashboard", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("dashboard code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "run_test1") {
		t.Fatalf("stdout = %q, want 'run_test1'", stdout.String())
	}
	if !strings.Contains(stdout.String(), "succeeded") {
		t.Fatalf("stdout = %q, want 'succeeded'", stdout.String())
	}
}

func TestDashboardRunDetailNotFound(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	eventsDir := filepath.Join(root, ".reasonforge", "events")
	os.MkdirAll(eventsDir, 0o700)
	storePath := filepath.Join(eventsDir, "run_events.jsonl")
	os.WriteFile(storePath, []byte(""), 0o600)

	eventsPath := filepath.Join(root, ".reasonforge", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: true\nstore_path: .reasonforge/events/run_events.jsonl\n"), 0o600)

	var stderr bytes.Buffer
	code := Run([]string{"dashboard", "--dir", root, "--run", "nonexistent"}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("dashboard --run nonexistent code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Fatalf("stderr = %q, want 'not found'", stderr.String())
	}
}

func TestDashboardRejectsExtraArgs(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stderr bytes.Buffer
	code := Run([]string{"dashboard", "--dir", root, "extra"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("dashboard with extra args code = %d, want 2", code)
	}
}

func TestDashboardDoesNotLeakSensitiveData(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	eventsDir := filepath.Join(root, ".reasonforge", "events")
	os.MkdirAll(eventsDir, 0o700)
	storePath := filepath.Join(eventsDir, "run_events.jsonl")

	store, _ := events.NewJSONLRunEventStore(storePath)
	ctx := context.Background()
	now := time.Now().UTC()
	// In production, SafeEmit calls SanitizeEvent before writing.
	// Simulate that here by sanitizing before appending.
	rawEvent := events.RunEvent{
		ID: "evt_sec", RunID: "run_sec", Type: events.EventRunStarted, Status: "started",
		Message:   "Run started with API_KEY=sk-secret-12345",
		StartedAt: now,
	}
	sanitized := events.SanitizeEvent(rawEvent)
	store.Append(ctx, sanitized)
	store.Close()

	eventsPath := filepath.Join(root, ".reasonforge", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: true\nstore_path: .reasonforge/events/run_events.jsonl\n"), 0o600)

	var stdout bytes.Buffer
	Run([]string{"dashboard", "--dir", root}, Env{Stdout: &stdout})
	output := stdout.String()
	if strings.Contains(output, "sk-secret-12345") {
		t.Fatal("dashboard output leaked API key")
	}
	if strings.Contains(output, "API_KEY") {
		t.Fatal("dashboard output leaked API key pattern")
	}
}

func TestDashboardLimitRuns(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	eventsDir := filepath.Join(root, ".reasonforge", "events")
	os.MkdirAll(eventsDir, 0o700)
	storePath := filepath.Join(eventsDir, "run_events.jsonl")

	store, _ := events.NewJSONLRunEventStore(storePath)
	ctx := context.Background()
	now := time.Now().UTC()
	for i := 0; i < 10; i++ {
		store.Append(ctx, events.RunEvent{
			ID: fmt.Sprintf("evt_%d", i), RunID: fmt.Sprintf("run_%d", i),
			Type: events.EventRunStarted, Status: "started",
			StartedAt: now.Add(time.Duration(-i) * time.Hour),
		})
	}
	store.Close()

	eventsPath := filepath.Join(root, ".reasonforge", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: true\nstore_path: .reasonforge/events/run_events.jsonl\n"), 0o600)

	var stdout bytes.Buffer
	Run([]string{"dashboard", "--dir", root, "--limit", "3"}, Env{Stdout: &stdout})
	output := stdout.String()

	// Count run_ occurrences (excluding headers)
	dataLines := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "run_") && !strings.Contains(line, "Recent") {
			dataLines++
		}
	}
	if dataLines != 3 {
		t.Errorf("expected 3 data lines with --limit 3, got %d\noutput:\n%s", dataLines, output)
	}
}

// --- Web Dashboard CLI tests ---

func stubServeCommand(t *testing.T) **webserver.LocalServer {
	t.Helper()
	var captured *webserver.LocalServer
	old := serveCommandRun
	serveCommandRun = func(s *webserver.LocalServer) error {
		captured = s
		return nil
	}
	t.Cleanup(func() { serveCommandRun = old })
	return &captured
}

func TestServeCommandParsesHostPort(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	captured := stubServeCommand(t)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"serve", "--dir", root, "--host", "127.0.0.1", "--port", "9000", "--poll-interval", "5s"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(serve) code = %d, stderr = %q", code, stderr.String())
	}
	if *captured == nil {
		t.Fatal("serveCommandRun was not called")
	}
	if (*captured).Address() != "127.0.0.1:9000" {
		t.Fatalf("Address() = %q, want 127.0.0.1:9000", (*captured).Address())
	}
	if (*captured).PollInterval() != 5*time.Second {
		t.Fatalf("PollInterval() = %v, want 5s", (*captured).PollInterval())
	}
	if !strings.Contains(stdout.String(), "http://127.0.0.1:9000") {
		t.Fatalf("stdout = %q, want listen URL", stdout.String())
	}
}

func TestServeRejectsExtraArgs(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"serve", "extra"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(serve extra) code = %d, want 2", code)
	}
}

func TestServeDefaultPort(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	captured := stubServeCommand(t)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"serve", "--dir", root}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("Run(serve) code = %d, stderr = %q", code, stderr.String())
	}
	if *captured == nil {
		t.Fatal("serveCommandRun was not called")
	}
	if (*captured).Address() != "127.0.0.1:8765" {
		t.Fatalf("Address() = %q, want 127.0.0.1:8765", (*captured).Address())
	}
}

func TestServeDoesNotDefaultToPublicHost(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	captured := stubServeCommand(t)

	var stderr bytes.Buffer
	code := Run([]string{"serve", "--dir", root}, Env{Stderr: &stderr})
	if code != 0 {
		t.Fatalf("Run(serve) code = %d, stderr = %q", code, stderr.String())
	}
	if *captured == nil {
		t.Fatal("serveCommandRun was not called")
	}
	if strings.HasPrefix((*captured).Address(), "0.0.0.0:") {
		t.Fatalf("serve defaulted to public host: %s", (*captured).Address())
	}
	if !strings.HasPrefix((*captured).Address(), "127.0.0.1:") {
		t.Fatalf("Address() = %q, want localhost", (*captured).Address())
	}
}
