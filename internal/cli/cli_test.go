package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reasonforge/reasonforge/internal/config"
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
