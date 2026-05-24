package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reasonforge/reasonforge/internal/cache"
	"github.com/reasonforge/reasonforge/internal/contextengine"
	"github.com/reasonforge/reasonforge/internal/modelrouter"
	"github.com/reasonforge/reasonforge/internal/scratchpad"
	"github.com/reasonforge/reasonforge/internal/task"
	"github.com/reasonforge/reasonforge/internal/tools"
)

// mockContextEngine implements contextengine.ContextEngine for testing.
type mockContextEngine struct {
	bundle contextengine.Bundle
	err    error
}

func (m *mockContextEngine) Build(ctx context.Context, req contextengine.BuildRequest) (contextengine.Bundle, error) {
	return m.bundle, m.err
}
func (m *mockContextEngine) RecordModelCall(ctx context.Context, obs cache.Observation) error {
	return nil
}

// mockModelRouter implements modelrouter.ModelRouter for testing.
type mockModelRouter struct {
	responses []modelrouter.CompletionResponse
	calls     int
	err       error
}

func (m *mockModelRouter) Complete(ctx context.Context, req modelrouter.CompletionRequest) (modelrouter.CompletionResponse, error) {
	if m.err != nil {
		return modelrouter.CompletionResponse{}, m.err
	}
	if m.calls < len(m.responses) {
		resp := m.responses[m.calls]
		m.calls++
		return resp, nil
	}
	// Default: return empty text (no tool call -> finish)
	return modelrouter.CompletionResponse{
		Provider: "mock",
		Model:    "mock-model",
		Text:     "Task completed.",
	}, nil
}

// mockToolRuntime implements tools.ToolRuntime for testing.
type mockToolRuntime struct {
	responses map[string]tools.ToolResponse
	calls     []tools.ToolRequest
	err       error
}

func (m *mockToolRuntime) Run(ctx context.Context, req tools.ToolRequest) (tools.ToolResponse, error) {
	m.calls = append(m.calls, req)
	if m.err != nil {
		return tools.ToolResponse{}, m.err
	}
	if resp, ok := m.responses[req.ToolName]; ok {
		return resp, nil
	}
	return tools.ToolResponse{
		ToolName: req.ToolName,
		Success:  true,
		ExitCode: 0,
		Stdout:   fmt.Sprintf("mock output for %s", req.ToolName),
	}, nil
}

// mockToolRegistry implements tools.ToolRegistry for testing.
type mockToolRegistry struct {
	tools map[string]tools.Tool
}

func (m *mockToolRegistry) Register(tool tools.Tool) error { return nil }
func (m *mockToolRegistry) Get(name string) (tools.Tool, bool) {
	t, ok := m.tools[name]
	return t, ok
}
func (m *mockToolRegistry) List() []tools.ToolInfo {
	var infos []tools.ToolInfo
	for _, t := range m.tools {
		infos = append(infos, tools.ToolInfo{
			Name:      t.Name(),
			Enabled:   true,
			RiskLevel: t.RiskLevel(),
		})
	}
	return infos
}

// mockTool implements tools.Tool for testing.
type mockTool struct {
	name      string
	riskLevel string
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return "mock tool" }
func (m *mockTool) RiskLevel() string   { return m.riskLevel }
func (m *mockTool) Run(ctx context.Context, req tools.ToolRequest) (tools.ToolResponse, error) {
	return tools.ToolResponse{ToolName: m.name, Success: true, Stdout: "mock"}, nil
}

// mockCheckpointStore implements CheckpointStore for testing.
type mockCheckpointStore struct {
	checkpoints []Checkpoint
}

func (m *mockCheckpointStore) Save(ctx context.Context, cp Checkpoint) error {
	m.checkpoints = append(m.checkpoints, cp)
	return nil
}
func (m *mockCheckpointStore) Load(ctx context.Context, runID string) (Checkpoint, error) {
	for i := len(m.checkpoints) - 1; i >= 0; i-- {
		if m.checkpoints[i].RunID == runID {
			return m.checkpoints[i], nil
		}
	}
	return Checkpoint{}, fmt.Errorf("not found")
}
func (m *mockCheckpointStore) List(ctx context.Context) ([]string, error) {
	seen := make(map[string]bool)
	var ids []string
	for _, cp := range m.checkpoints {
		if !seen[cp.RunID] {
			seen[cp.RunID] = true
			ids = append(ids, cp.RunID)
		}
	}
	return ids, nil
}

func testDeps() Dependencies {
	return Dependencies{
		ContextEngine: &mockContextEngine{
			bundle: contextengine.Bundle{
				Report: contextengine.ContextReport{},
			},
		},
		ModelRouter: &mockModelRouter{
			responses: []modelrouter.CompletionResponse{},
		},
		ToolRuntime: &mockToolRuntime{
			responses: make(map[string]tools.ToolResponse),
		},
		ToolRegistry: &mockToolRegistry{
			tools: map[string]tools.Tool{
				"file_read":  &mockTool{name: "file_read", riskLevel: "low"},
				"git_diff":   &mockTool{name: "git_diff", riskLevel: "low"},
				"test_run":   &mockTool{name: "test_run", riskLevel: "medium"},
				"file_write": &mockTool{name: "file_write", riskLevel: "medium"},
			},
		},
		Scratchpad:      scratchpad.NewVolatileScratchpad(),
		CheckpointStore: &mockCheckpointStore{},
	}
}

func TestSingleAgentRuntime_SimpleCompletion(t *testing.T) {
	deps := testDeps()
	deps.ModelRouter = &mockModelRouter{
		responses: []modelrouter.CompletionResponse{
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     "I have completed the task successfully.",
			},
		},
	}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:           "tc_test",
		Goal:         "read the README",
		RepoRoot:     "/repo",
		MaxSteps:     5,
		MaxToolCalls: 10,
	}

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_001",
		RepoRoot: "/repo",
		Goal:     "read the README",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateSucceeded {
		t.Errorf("State = %q, want %q", result.State, AgentStateSucceeded)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(result.Steps))
	}
	if result.Steps[0].Type != "model" {
		t.Errorf("Step[0].Type = %q, want %q", result.Steps[0].Type, "model")
	}
}

func TestSingleAgentRuntime_ToolCallLoop(t *testing.T) {
	deps := testDeps()
	deps.ModelRouter = &mockModelRouter{
		responses: []modelrouter.CompletionResponse{
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     `Let me read the file. {"tool_call": {"name": "file_read", "args": {"path": "README.md"}}}`,
			},
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     "I've read the file. The task is done.",
			},
		},
	}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:           "tc_test",
		Goal:         "read the README",
		RepoRoot:     "/repo",
		MaxSteps:     5,
		MaxToolCalls: 10,
	}

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_002",
		RepoRoot: "/repo",
		Goal:     "read the README",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateSucceeded {
		t.Errorf("State = %q, want %q", result.State, AgentStateSucceeded)
	}

	// Should have 3 steps: model (with tool_call) + tool + model (completion)
	if len(result.Steps) < 2 {
		t.Fatalf("len(Steps) = %d, want at least 2", len(result.Steps))
	}

	// First step is model call with tool_call
	if result.Steps[0].Type != "model" {
		t.Errorf("Step[0].Type = %q, want %q", result.Steps[0].Type, "model")
	}

	// Find the tool step
	var toolStepFound bool
	for _, step := range result.Steps {
		if step.Type == "tool" && step.ToolCall != nil && step.ToolCall.Name == "file_read" {
			toolStepFound = true
			if step.State != AgentStateSucceeded {
				t.Errorf("tool step State = %q, want %q", step.State, AgentStateSucceeded)
			}
		}
	}
	if !toolStepFound {
		t.Error("expected to find a tool step for file_read")
	}
}

func TestSingleAgentRuntime_DeniedTool(t *testing.T) {
	deps := testDeps()
	deps.ModelRouter = &mockModelRouter{
		responses: []modelrouter.CompletionResponse{
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     `I need to write a file. {"tool_call": {"name": "file_write", "args": {"path": "out.txt", "content": "hello"}}}`,
			},
		},
	}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:           "tc_test",
		Goal:         "write a file",
		RepoRoot:     "/repo",
		MaxSteps:     5,
		MaxToolCalls: 10,
		DeniedTools:  []string{"file_write"},
	}

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_003",
		RepoRoot: "/repo",
		Goal:     "write a file",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateFailed {
		t.Errorf("State = %q, want %q", result.State, AgentStateFailed)
	}
}

func TestSingleAgentRuntime_MaxSteps(t *testing.T) {
	deps := testDeps()
	// Model always returns a tool call, never completing
	deps.ModelRouter = &mockModelRouter{
		responses: []modelrouter.CompletionResponse{},
	}
	// Override the default Complete to always return a tool call
	deps.ModelRouter = &infiniteToolCallRouter{}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:           "tc_test",
		Goal:         "infinite loop",
		RepoRoot:     "/repo",
		MaxSteps:     3,
		MaxToolCalls: 100,
	}

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_004",
		RepoRoot: "/repo",
		Goal:     "infinite loop",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateFailed {
		t.Errorf("State = %q, want %q", result.State, AgentStateFailed)
	}
	// Should have been stopped by max_steps
	if len(result.Steps) > 6 { // 3 iterations * 2 steps (model + tool)
		t.Errorf("too many steps: %d", len(result.Steps))
	}
}

// infiniteToolCallRouter always returns a tool call.
type infiniteToolCallRouter struct{}

func (r *infiniteToolCallRouter) Complete(ctx context.Context, req modelrouter.CompletionRequest) (modelrouter.CompletionResponse, error) {
	return modelrouter.CompletionResponse{
		Provider: "mock",
		Model:    "mock-model",
		Text:     `{"tool_call": {"name": "file_read", "args": {"path": "test.go"}}}`,
	}, nil
}

func TestSingleAgentRuntime_MaxToolCalls(t *testing.T) {
	deps := testDeps()
	deps.ModelRouter = &infiniteToolCallRouter{}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:           "tc_test",
		Goal:         "many tools",
		RepoRoot:     "/repo",
		MaxSteps:     100,
		MaxToolCalls: 2,
	}

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_005",
		RepoRoot: "/repo",
		Goal:     "many tools",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateFailed {
		t.Errorf("State = %q, want %q", result.State, AgentStateFailed)
	}
}

func TestSingleAgentRuntime_MediumRiskRequiresApproval(t *testing.T) {
	deps := testDeps()
	deps.ModelRouter = &mockModelRouter{
		responses: []modelrouter.CompletionResponse{
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     `I need to run tests. {"tool_call": {"name": "test_run", "args": {"command_name": "go-test"}}}`,
			},
		},
	}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:                     "tc_test",
		Goal:                   "run tests",
		RepoRoot:               "/repo",
		MaxSteps:               5,
		MaxToolCalls:           10,
		AllowedTools:           []string{"test_run"},
		RequireApprovalForRisk: []string{"medium"},
	}

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_006",
		RepoRoot: "/repo",
		Goal:     "run tests",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateWaitingApproval {
		t.Errorf("State = %q, want %q", result.State, AgentStateWaitingApproval)
	}
}

func TestSingleAgentRuntime_ContextEngineError(t *testing.T) {
	deps := testDeps()
	deps.ContextEngine = &mockContextEngine{
		err: fmt.Errorf("context engine failed"),
	}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:       "tc_test",
		Goal:     "test error",
		RepoRoot: "/repo",
		MaxSteps: 5,
	}

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_007",
		RepoRoot: "/repo",
		Goal:     "test error",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateFailed {
		t.Errorf("State = %q, want %q", result.State, AgentStateFailed)
	}
}

func TestSingleAgentRuntime_CancelledContext(t *testing.T) {
	deps := testDeps()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:       "tc_test",
		Goal:     "test cancel",
		RepoRoot: "/repo",
		MaxSteps: 5,
	}

	result, err := rt.Run(ctx, AgentRunRequest{
		TaskID:   "task_008",
		RepoRoot: "/repo",
		Goal:     "test cancel",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateCancelled {
		t.Errorf("State = %q, want %q", result.State, AgentStateCancelled)
	}
}

func TestSingleAgentRuntime_InvalidContract(t *testing.T) {
	deps := testDeps()
	rt := NewSingleAgentRuntime(deps)

	contract := task.TaskContract{
		// Goal is empty — invalid
		RepoRoot: "/repo",
		MaxSteps: 5,
	}

	_, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_009",
		RepoRoot: "/repo",
		Goal:     "test",
		Contract: contract,
	})
	if err == nil {
		t.Error("Run() with invalid contract should return error")
	}
}

func TestSingleAgentRuntime_CheckpointsSaved(t *testing.T) {
	store := &mockCheckpointStore{}
	deps := testDeps()
	deps.CheckpointStore = store
	deps.ModelRouter = &mockModelRouter{
		responses: []modelrouter.CompletionResponse{
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     `{"tool_call": {"name": "file_read", "args": {"path": "a.go"}}}`,
			},
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     "Done.",
			},
		},
	}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:           "tc_test",
		Goal:         "test checkpoints",
		RepoRoot:     "/repo",
		MaxSteps:     5,
		MaxToolCalls: 10,
	}

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_010",
		RepoRoot: "/repo",
		Goal:     "test checkpoints",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateSucceeded {
		t.Errorf("State = %q, want %q", result.State, AgentStateSucceeded)
	}

	// Check that at least 2 checkpoints were saved (initial + at least one during loop)
	if len(store.checkpoints) < 2 {
		t.Errorf("expected at least 2 checkpoints, got %d", len(store.checkpoints))
	}
}

func TestSingleAgentRuntime_ToolResponseToScratchpad(t *testing.T) {
	sp := scratchpad.NewVolatileScratchpad()
	deps := testDeps()
	deps.Scratchpad = sp
	deps.ModelRouter = &mockModelRouter{
		responses: []modelrouter.CompletionResponse{
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     `{"tool_call": {"name": "file_read", "args": {"path": "main.go"}}}`,
			},
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     "Done reading.",
			},
		},
	}
	deps.ToolRuntime = &mockToolRuntime{
		responses: map[string]tools.ToolResponse{
			"file_read": {
				ToolName: "file_read",
				Success:  true,
				Stdout:   "package main\nfunc main() {}\n",
			},
		},
	}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:           "tc_test",
		Goal:         "read main.go",
		RepoRoot:     "/repo",
		MaxSteps:     5,
		MaxToolCalls: 10,
	}

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_011",
		RepoRoot: "/repo",
		Goal:     "read main.go",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateSucceeded {
		t.Errorf("State = %q, want %q", result.State, AgentStateSucceeded)
	}

	// Verify scratchpad has tool output
	snap, err := sp.Snapshot(context.Background(), scratchpad.Scope{
		TaskID: "task_011",
	})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snap.Items) == 0 {
		t.Error("scratchpad should contain tool output items")
	}

	found := false
	for _, item := range snap.Items {
		if item.Kind == scratchpad.ItemKindToolOutput {
			found = true
			if string(item.Content) != "package main\nfunc main() {}\n" {
				t.Errorf("scratchpad content = %q, want tool output", string(item.Content))
			}
		}
	}
	if !found {
		t.Error("scratchpad should contain a tool_output item")
	}
}

func TestSingleAgentRuntime_DryRun(t *testing.T) {
	toolRt := &mockToolRuntime{
		responses: map[string]tools.ToolResponse{
			"file_read": {
				ToolName: "file_read",
				Success:  true,
				Stdout:   "[dry-run] would read file",
			},
		},
	}

	deps := testDeps()
	deps.ToolRuntime = toolRt
	deps.ModelRouter = &mockModelRouter{
		responses: []modelrouter.CompletionResponse{
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     `{"tool_call": {"name": "file_read", "args": {"path": "README.md"}}}`,
			},
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     "Dry run complete.",
			},
		},
	}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:       "tc_test",
		Goal:     "dry run test",
		RepoRoot: "/repo",
		MaxSteps: 5,
		DryRun:   true,
	}

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_012",
		RepoRoot: "/repo",
		Goal:     "dry run test",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateSucceeded {
		t.Errorf("State = %q, want %q", result.State, AgentStateSucceeded)
	}

	// Verify DryRun was propagated to tool
	if len(toolRt.calls) == 0 {
		t.Fatal("expected at least one tool call")
	}
	if !toolRt.calls[0].DryRun {
		t.Error("DryRun should be propagated to tool request")
	}
}

func TestDefaultContractIntegration(t *testing.T) {
	deps := testDeps()
	deps.ModelRouter = &mockModelRouter{
		responses: []modelrouter.CompletionResponse{
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     "I can only read files. The task is done.",
			},
		},
	}

	rt := NewSingleAgentRuntime(deps)
	contract := task.DefaultContract(filepath.FromSlash("/repo"), "read the README")

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_default",
		RepoRoot: filepath.FromSlash("/repo"),
		Goal:     "read the README",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != AgentStateSucceeded {
		t.Errorf("State = %q, want %q", result.State, AgentStateSucceeded)
	}
}

// failCheckpointStore always returns an error on Save.
type failCheckpointStore struct {
	saveErr error
	saves   int
}

func (f *failCheckpointStore) Save(ctx context.Context, cp Checkpoint) error {
	f.saves++
	return f.saveErr
}
func (f *failCheckpointStore) Load(ctx context.Context, runID string) (Checkpoint, error) {
	return Checkpoint{}, fmt.Errorf("not found")
}
func (f *failCheckpointStore) List(ctx context.Context) ([]string, error) {
	return nil, nil
}

func TestSingleAgentRuntime_InitialCheckpointFailure(t *testing.T) {
	deps := testDeps()
	deps.CheckpointStore = &failCheckpointStore{
		saveErr: fmt.Errorf("disk full"),
	}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:       "tc_cp_fail",
		Goal:     "test checkpoint failure",
		RepoRoot: "/repo",
		MaxSteps: 5,
	}

	_, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_cp_init",
		RepoRoot: "/repo",
		Goal:     "test checkpoint failure",
		Contract: contract,
	})
	if err == nil {
		t.Error("Run() should return error when initial checkpoint fails")
	}
	if !strings.Contains(err.Error(), "initial checkpoint failed") {
		t.Errorf("error should mention initial checkpoint, got: %v", err)
	}
}

func TestSingleAgentRuntime_MidLoopCheckpointFailure(t *testing.T) {
	deps := testDeps()
	// Store that fails after the first save (initial checkpoint succeeds, mid-loop fails)
	store := &failAfterFirstStore{}
	deps.CheckpointStore = store
	deps.ModelRouter = &mockModelRouter{
		responses: []modelrouter.CompletionResponse{
			{
				Provider: "mock",
				Model:    "mock-model",
				Text:     `{"tool_call": {"name": "file_read", "args": {"path": "a.go"}}}`,
			},
		},
	}

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:           "tc_cp_mid",
		Goal:         "test mid-loop checkpoint failure",
		RepoRoot:     "/repo",
		MaxSteps:     5,
		MaxToolCalls: 10,
	}

	result, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_cp_mid",
		RepoRoot: "/repo",
		Goal:     "test mid-loop checkpoint failure",
		Contract: contract,
	})
	if err != nil {
		t.Fatalf("Run() should not return error, got: %v", err)
	}
	if result.State != AgentStateFailed {
		t.Errorf("State = %q, want %q", result.State, AgentStateFailed)
	}
	if !strings.Contains(result.Error, "checkpoint failed") {
		t.Errorf("error should mention checkpoint failure, got: %s", result.Error)
	}
}

// failAfterFirstStore succeeds on first Save, fails on subsequent ones.
type failAfterFirstStore struct {
	saves int
}

func (f *failAfterFirstStore) Save(ctx context.Context, cp Checkpoint) error {
	f.saves++
	if f.saves > 1 {
		return fmt.Errorf("disk error after first save")
	}
	return nil
}
func (f *failAfterFirstStore) Load(ctx context.Context, runID string) (Checkpoint, error) {
	return Checkpoint{}, fmt.Errorf("not found")
}
func (f *failAfterFirstStore) List(ctx context.Context) ([]string, error) {
	return nil, nil
}

func TestSingleAgentRuntime_CheckpointStoreNil(t *testing.T) {
	deps := testDeps()
	deps.CheckpointStore = nil

	rt := NewSingleAgentRuntime(deps)
	contract := task.TaskContract{
		ID:       "tc_cp_nil",
		Goal:     "test nil checkpoint",
		RepoRoot: "/repo",
		MaxSteps: 5,
	}

	_, err := rt.Run(context.Background(), AgentRunRequest{
		TaskID:   "task_cp_nil",
		RepoRoot: "/repo",
		Goal:     "test nil checkpoint",
		Contract: contract,
	})
	if err == nil {
		t.Error("Run() should return error when checkpoint store is nil")
	}
}
