package validation

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/reasonforge/reasonforge/internal/review"
	"github.com/reasonforge/reasonforge/internal/tools"
)

// mockToolRuntime implements tools.ToolRuntime for testing.
type mockToolRuntime struct {
	responses map[string]tools.ToolResponse
	err       error
	callCount int
	lastReq   *tools.ToolRequest
}

func (m *mockToolRuntime) Run(ctx context.Context, req tools.ToolRequest) (tools.ToolResponse, error) {
	m.callCount++
	m.lastReq = &req

	if m.err != nil {
		return tools.ToolResponse{}, m.err
	}

	resp, ok := m.responses[req.Args["command_name"]]
	if !ok {
		return tools.ToolResponse{
			ToolName: "test_run",
			Success:  false,
			ExitCode: 1,
			Error:    "command not configured",
		}, nil
	}
	return resp, nil
}

func TestValidationRunner_Success(t *testing.T) {
	rt := &mockToolRuntime{
		responses: map[string]tools.ToolResponse{
			"go-test": {
				ToolName: "test_run",
				Success:  true,
				ExitCode: 0,
				Stdout:   "ok  pkg1\nok  pkg2\n",
			},
		},
	}

	runner := NewValidationRunner(rt, DefaultValidationConfig())

	result, err := runner.Validate(context.Background(), review.ValidationRequest{
		RepoRoot:       "/tmp/worktree",
		TaskID:         "task_001",
		TestCommands:   []string{"go-test"},
		MaxOutputBytes: 65536,
		TimeoutSeconds: 120,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected success")
	}
	if len(result.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(result.Commands))
	}
	if result.Commands[0].CommandName != "go-test" {
		t.Errorf("expected go-test, got %s", result.Commands[0].CommandName)
	}
	if !result.Commands[0].Success {
		t.Error("expected command success")
	}
}

func TestValidationRunner_CommandFails(t *testing.T) {
	rt := &mockToolRuntime{
		responses: map[string]tools.ToolResponse{
			"go-test": {
				ToolName: "test_run",
				Success:  false,
				ExitCode: 1,
				Stderr:   "FAIL  pkg1\n",
				Error:    "command \"go-test\" exited with code 1",
			},
		},
	}

	runner := NewValidationRunner(rt, DefaultValidationConfig())

	result, err := runner.Validate(context.Background(), review.ValidationRequest{
		RepoRoot:       "/tmp/worktree",
		TaskID:         "task_001",
		TestCommands:   []string{"go-test"},
		MaxOutputBytes: 65536,
		TimeoutSeconds: 120,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Error("expected failure")
	}
	if result.Commands[0].Success {
		t.Error("expected command failure")
	}
	if result.Commands[0].ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.Commands[0].ExitCode)
	}
}

func TestValidationRunner_NoTestCommands(t *testing.T) {
	rt := &mockToolRuntime{}
	runner := NewValidationRunner(rt, DefaultValidationConfig())

	result, err := runner.Validate(context.Background(), review.ValidationRequest{
		RepoRoot:     "/tmp/worktree",
		TaskID:       "task_001",
		TestCommands: []string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Error("expected failure when no test commands configured")
	}
	if result.Summary != "no test commands configured" {
		t.Errorf("unexpected summary: %s", result.Summary)
	}
}

func TestValidationRunner_UsesWorktreePath(t *testing.T) {
	rt := &mockToolRuntime{
		responses: map[string]tools.ToolResponse{
			"go-test": {Success: true, ExitCode: 0},
		},
	}
	runner := NewValidationRunner(rt, DefaultValidationConfig())

	worktreePath := "/tmp/repo/.reasonforge/worktrees/task_001/wt_abc"

	_, err := runner.Validate(context.Background(), review.ValidationRequest{
		RepoRoot:       worktreePath,
		TaskID:         "task_001",
		TestCommands:   []string{"go-test"},
		MaxOutputBytes: 65536,
		TimeoutSeconds: 120,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rt.lastReq == nil {
		t.Fatal("expected tool runtime call")
	}
	if rt.lastReq.RepoRoot != worktreePath {
		t.Errorf("expected worktree path %s, got %s", worktreePath, rt.lastReq.RepoRoot)
	}
}

func TestValidationRunner_UsesTestRunTool(t *testing.T) {
	rt := &mockToolRuntime{
		responses: map[string]tools.ToolResponse{
			"go-test": {Success: true, ExitCode: 0},
		},
	}
	runner := NewValidationRunner(rt, DefaultValidationConfig())

	_, err := runner.Validate(context.Background(), review.ValidationRequest{
		RepoRoot:     "/tmp/worktree",
		TaskID:       "task_001",
		TestCommands: []string{"go-test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rt.lastReq.ToolName != "test_run" {
		t.Errorf("expected tool name test_run, got %s", rt.lastReq.ToolName)
	}
}

func TestValidationRunner_ToolRuntimeError(t *testing.T) {
	rt := &mockToolRuntime{
		err: fmt.Errorf("tool not found"),
	}
	runner := NewValidationRunner(rt, DefaultValidationConfig())

	result, err := runner.Validate(context.Background(), review.ValidationRequest{
		RepoRoot:     "/tmp/worktree",
		TaskID:       "task_001",
		TestCommands: []string{"go-test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Error("expected failure when tool runtime errors")
	}
}

func TestValidationRunner_MultipleCommands(t *testing.T) {
	rt := &mockToolRuntime{
		responses: map[string]tools.ToolResponse{
			"go-test": {Success: true, ExitCode: 0, Stdout: "ok"},
			"go-vet":  {Success: true, ExitCode: 0, Stdout: "ok"},
			"go-lint": {Success: false, ExitCode: 1, Stderr: "issues found"},
		},
	}
	runner := NewValidationRunner(rt, DefaultValidationConfig())

	result, err := runner.Validate(context.Background(), review.ValidationRequest{
		RepoRoot:     "/tmp/worktree",
		TaskID:       "task_001",
		TestCommands: []string{"go-test", "go-vet", "go-lint"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Error("expected failure when one command fails")
	}
	if len(result.Commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(result.Commands))
	}
	if !result.Commands[0].Success || !result.Commands[1].Success {
		t.Error("expected first two commands to succeed")
	}
	if result.Commands[2].Success {
		t.Error("expected third command to fail")
	}
}

func TestValidationRunner_SanitizesAPIKeys(t *testing.T) {
	rt := &mockToolRuntime{
		responses: map[string]tools.ToolResponse{
			"go-test": {
				Success:  true,
				ExitCode: 0,
				Stdout:   "API_KEY=my-secret-key\nok",
				Stderr:   "TOKEN=abc123\n",
			},
		},
	}
	runner := NewValidationRunner(rt, DefaultValidationConfig())

	result, err := runner.Validate(context.Background(), review.ValidationRequest{
		RepoRoot:     "/tmp/worktree",
		TaskID:       "task_001",
		TestCommands: []string{"go-test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(result.Commands[0].Stdout, "my-secret-key") {
		t.Error("stdout should not contain API key values")
	}
	if strings.Contains(result.Commands[0].Stderr, "abc123") {
		t.Error("stderr should not contain token values")
	}
}

func TestValidationRunner_DefaultConfig(t *testing.T) {
	cfg := DefaultValidationConfig()
	if cfg.MaxOutputBytes != 65536 {
		t.Errorf("expected MaxOutputBytes 65536, got %d", cfg.MaxOutputBytes)
	}
	if cfg.TimeoutSeconds != 120 {
		t.Errorf("expected TimeoutSeconds 120, got %d", cfg.TimeoutSeconds)
	}
	if len(cfg.DefaultTestCommands) == 0 {
		t.Error("expected non-empty default test commands")
	}
}

func TestValidationRunner_MaxOutputBytes(t *testing.T) {
	rt := &mockToolRuntime{
		responses: map[string]tools.ToolResponse{
			"go-test": {Success: true, ExitCode: 0, Stdout: "ok"},
		},
	}
	runner := NewValidationRunner(rt, DefaultValidationConfig())

	_, err := runner.Validate(context.Background(), review.ValidationRequest{
		RepoRoot:       "/tmp/worktree",
		TaskID:         "task_001",
		TestCommands:   []string{"go-test"},
		MaxOutputBytes: 32768,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rt.lastReq.MaxOutputBytes != 32768 {
		t.Errorf("expected MaxOutputBytes 32768, got %d", rt.lastReq.MaxOutputBytes)
	}
}

func TestValidationRunner_Timeout(t *testing.T) {
	rt := &mockToolRuntime{
		responses: map[string]tools.ToolResponse{
			"go-test": {Success: true, ExitCode: 0, Stdout: "ok"},
		},
	}
	runner := NewValidationRunner(rt, DefaultValidationConfig())

	_, err := runner.Validate(context.Background(), review.ValidationRequest{
		RepoRoot:       "/tmp/worktree",
		TaskID:         "task_001",
		TestCommands:   []string{"go-test"},
		TimeoutSeconds: 60,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rt.lastReq.TimeoutSeconds != 60 {
		t.Errorf("expected TimeoutSeconds 60, got %d", rt.lastReq.TimeoutSeconds)
	}
}

func TestSanitizeOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // should NOT contain this
	}{
		{
			name:     "API_KEY redacted",
			input:    "API_KEY=sk-12345abcdef\nok",
			contains: "sk-12345abcdef",
		},
		{
			name:     "SECRET redacted",
			input:    "DB_SECRET=mydbpass\nok",
			contains: "mydbpass",
		},
		{
			name:     "TOKEN redacted",
			input:    "AUTH_TOKEN=token123\nok",
			contains: "token123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeOutput(tt.input)
			if strings.Contains(result, tt.contains) {
				t.Errorf("output should not contain %q, got: %s", tt.contains, result)
			}
		})
	}
}
