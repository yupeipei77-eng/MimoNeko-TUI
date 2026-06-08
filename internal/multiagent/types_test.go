package multiagent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/patch"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/review"
)

func TestMultiAgentStateIsTerminal(t *testing.T) {
	terminalStates := []MultiAgentState{
		MultiAgentStateSucceeded,
		MultiAgentStateRequestChanges,
		MultiAgentStateRejected,
		MultiAgentStateFailed,
		MultiAgentStateCancelled,
	}
	for _, s := range terminalStates {
		if !s.IsTerminal() {
			t.Errorf("expected %q to be terminal", s)
		}
	}

	nonTerminalStates := []MultiAgentState{
		MultiAgentStatePending,
		MultiAgentStateRunning,
	}
	for _, s := range nonTerminalStates {
		if s.IsTerminal() {
			t.Errorf("expected %q to NOT be terminal", s)
		}
	}
}

func TestAgentRoleConstants(t *testing.T) {
	if AgentRolePlanner != "planner" {
		t.Errorf("expected AgentRolePlanner=%q, got %q", "planner", AgentRolePlanner)
	}
	if AgentRoleCoder != "coder" {
		t.Errorf("expected AgentRoleCoder=%q, got %q", "coder", AgentRoleCoder)
	}
	if AgentRoleReviewer != "reviewer" {
		t.Errorf("expected AgentRoleReviewer=%q, got %q", "reviewer", AgentRoleReviewer)
	}
}

func TestTaskPlanJSONParseSuccess(t *testing.T) {
	raw := `{
		"goal": "Fix README typo",
		"steps": [
			{
				"index": 0,
				"title": "Fix typo",
				"description": "Change 'teh' to 'the'",
				"target_paths": ["README.md"],
				"expected_outcome": "README has correct spelling"
			}
		],
		"risk_level": "low",
		"notes": "Simple text fix"
	}`

	var plan TaskPlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		t.Fatalf("failed to parse valid TaskPlan JSON: %v", err)
	}
	if plan.Goal != "Fix README typo" {
		t.Errorf("expected goal=%q, got %q", "Fix README typo", plan.Goal)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Title != "Fix typo" {
		t.Errorf("expected step title=%q, got %q", "Fix typo", plan.Steps[0].Title)
	}
	if plan.RiskLevel != "low" {
		t.Errorf("expected risk_level=%q, got %q", "low", plan.RiskLevel)
	}
}

func TestTaskPlanNonJSONParseFailure(t *testing.T) {
	raw := `This is not JSON at all`
	var plan TaskPlan
	if err := json.Unmarshal([]byte(raw), &plan); err == nil {
		t.Error("expected error parsing non-JSON input, got nil")
	}
}

func TestTaskPlanInvalidJSONStructure(t *testing.T) {
	raw := `{"goal": 123, "steps": "not an array"}`
	var plan TaskPlan
	if err := json.Unmarshal([]byte(raw), &plan); err == nil {
		t.Error("expected error parsing invalid JSON structure, got nil")
	}
}

func TestValidateMaxIterations(t *testing.T) {
	tests := []struct {
		input    int
		expected int
		hasError bool
	}{
		{0, DefaultMaxIterations, false},
		{-1, DefaultMaxIterations, false},
		{1, 1, false},
		{2, 2, false},
		{5, 5, false},
		{6, 0, true},
		{100, 0, true},
	}

	for _, tt := range tests {
		result, err := ValidateMaxIterations(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("ValidateMaxIterations(%d): expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("ValidateMaxIterations(%d): unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("ValidateMaxIterations(%d): expected %d, got %d", tt.input, tt.expected, result)
			}
		}
	}
}

func TestGenerateMultiAgentRunID(t *testing.T) {
	id, err := GenerateMultiAgentRunID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(id) == 0 {
		t.Error("expected non-empty run ID")
	}
	if id[:4] != "mar_" {
		t.Errorf("expected run ID prefix 'mar_', got %q", id[:4])
	}

	id2, _ := GenerateMultiAgentRunID()
	if id == id2 {
		t.Error("expected unique IDs, got identical")
	}
}

func TestAgentMessage(t *testing.T) {
	now := time.Now().UTC()
	msg := AgentMessage{
		Role:      AgentRolePlanner,
		Content:   "Plan generated",
		Metadata:  map[string]string{"key": "value"},
		CreatedAt: now,
	}
	if msg.Role != AgentRolePlanner {
		t.Errorf("expected role=%q, got %q", AgentRolePlanner, msg.Role)
	}
	if msg.Content != "Plan generated" {
		t.Errorf("expected content=%q, got %q", "Plan generated", msg.Content)
	}
}

func TestSharedTaskContextDoesNotSaveSensitiveDiff(t *testing.T) {
	ctx := NewSharedTaskContext("test goal")

	// Report with violations - diff should be redacted
	report := review.PatchReviewReport{
		WorktreeID: "wt_123",
		Preview: patch.PatchPreview{
			WorktreeID: "wt_123",
			Diff:       "sensitive diff content here",
			Violations: []patch.PatchViolation{
				{Path: ".env", Reason: "denied path"},
			},
		},
		Recommendation: review.RecommendationReject,
		CreatedAt:      time.Now().UTC(),
	}

	ctx.AddReviewReport(report)

	if len(ctx.ReviewHistory) != 1 {
		t.Fatalf("expected 1 review in history, got %d", len(ctx.ReviewHistory))
	}

	// Diff should be redacted because of violations
	stored := ctx.ReviewHistory[0]
	if stored.Preview.Diff != "[diff redacted: policy violations present]" {
		t.Errorf("expected redacted diff with violations, got %q", stored.Preview.Diff)
	}
}

func TestSharedTaskContextDoesNotSaveAPIKeyPatterns(t *testing.T) {
	ctx := NewSharedTaskContext("test goal")

	// Report without violations but with API key in diff
	report := review.PatchReviewReport{
		WorktreeID: "wt_456",
		Preview: patch.PatchPreview{
			WorktreeID: "wt_456",
			Diff:       "api_key=sk-abc123secret",
		},
		Recommendation: review.RecommendationApprove,
		CreatedAt:      time.Now().UTC(),
	}

	ctx.AddReviewReport(report)

	stored := ctx.ReviewHistory[0]
	if stored.Preview.Diff != "[diff redacted: potential secret detected]" {
		t.Errorf("expected redacted diff with API key, got %q", stored.Preview.Diff)
	}
}

func TestSharedTaskContextPreservesCleanDiff(t *testing.T) {
	ctx := NewSharedTaskContext("test goal")

	// Report without violations and no API keys
	report := review.PatchReviewReport{
		WorktreeID: "wt_789",
		Preview: patch.PatchPreview{
			WorktreeID: "wt_789",
			Diff:       "+added line\n-removed line",
		},
		Recommendation: review.RecommendationApprove,
		CreatedAt:      time.Now().UTC(),
	}

	ctx.AddReviewReport(report)

	stored := ctx.ReviewHistory[0]
	if stored.Preview.Diff != "+added line\n-removed line" {
		t.Errorf("expected clean diff to be preserved, got %q", stored.Preview.Diff)
	}
}

func TestSharedTaskContextMessages(t *testing.T) {
	ctx := NewSharedTaskContext("test goal")

	ctx.AddMessage(AgentMessage{Role: AgentRolePlanner, Content: "plan done", CreatedAt: time.Now().UTC()})
	ctx.AddMessage(AgentMessage{Role: AgentRoleReviewer, Content: "needs work", CreatedAt: time.Now().UTC()})
	ctx.AddMessage(AgentMessage{Role: AgentRoleCoder, Content: "coding", CreatedAt: time.Now().UTC()})

	if len(ctx.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(ctx.Messages))
	}

	last := ctx.LastReviewerMessage()
	if last != "needs work" {
		t.Errorf("expected last reviewer message=%q, got %q", "needs work", last)
	}
}

func TestSharedTaskContextLastReview(t *testing.T) {
	ctx := NewSharedTaskContext("test goal")

	if last := ctx.LastReview(); last != nil {
		t.Error("expected nil LastReview with no reviews")
	}

	report := review.PatchReviewReport{
		WorktreeID:     "wt_1",
		Recommendation: review.RecommendationApprove,
		CreatedAt:      time.Now().UTC(),
	}
	ctx.AddReviewReport(report)

	if last := ctx.LastReview(); last == nil {
		t.Error("expected non-nil LastReview after adding review")
	} else if last.WorktreeID != "wt_1" {
		t.Errorf("expected worktree_id=%q, got %q", "wt_1", last.WorktreeID)
	}
}

func TestMultiAgentCheckpointStore(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "checkpoints", "multi_agent_runs.jsonl")

	store, err := NewJSONLMultiAgentCheckpointStore(path)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	// Verify directory permissions (on Windows, permissions are not enforced the same way)
	dirInfo, err := os.Stat(filepath.Join(tmpDir, "checkpoints"))
	if err != nil {
		t.Fatalf("failed to stat checkpoint dir: %v", err)
	}
	if dirInfo.Mode().Perm()&0o700 != 0o700 {
		t.Errorf("expected dir perm to include 0700, got %o", dirInfo.Mode().Perm())
	}

	// Verify file permissions
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat checkpoint file: %v", err)
	}
	if fileInfo.Mode().Perm()&0o600 != 0o600 {
		t.Errorf("expected file perm to include 0600, got %o", fileInfo.Mode().Perm())
	}

	// Save a checkpoint
	cp := MultiAgentCheckpoint{
		RunID:     "mar_test123",
		TaskID:    "task_1",
		State:     MultiAgentStateRunning,
		Iteration: 0,
		Phase:     "planner",
		CreatedAt: time.Now().UTC(),
	}
	if err := store.Save(ctx, cp); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	// Load the checkpoint
	loaded, err := store.Load(ctx, "mar_test123")
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}
	if loaded.RunID != "mar_test123" {
		t.Errorf("expected run_id=%q, got %q", "mar_test123", loaded.RunID)
	}
	if loaded.Phase != "planner" {
		t.Errorf("expected phase=%q, got %q", "planner", loaded.Phase)
	}

	// Save another checkpoint for the same run
	cp2 := MultiAgentCheckpoint{
		RunID:     "mar_test123",
		TaskID:    "task_1",
		State:     MultiAgentStateSucceeded,
		Iteration: 1,
		Phase:     "loop_end",
		CreatedAt: time.Now().UTC().Add(time.Second),
	}
	if err := store.Save(ctx, cp2); err != nil {
		t.Fatalf("failed to save second checkpoint: %v", err)
	}

	// Load should return the latest
	loaded2, err := store.Load(ctx, "mar_test123")
	if err != nil {
		t.Fatalf("failed to load latest checkpoint: %v", err)
	}
	if loaded2.Phase != "loop_end" {
		t.Errorf("expected latest phase=%q, got %q", "loop_end", loaded2.Phase)
	}

	// List
	ids, err := store.List(ctx)
	if err != nil {
		t.Fatalf("failed to list checkpoints: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("expected 1 unique run ID, got %d", len(ids))
	}

	// Load nonexistent
	_, err = store.Load(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error loading nonexistent run ID")
	}
}

func TestCheckpointRedactsSensitiveDiff(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "multi_agent_runs.jsonl")

	store, err := NewJSONLMultiAgentCheckpointStore(path)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	// Save a checkpoint with API key in error message
	cp := MultiAgentCheckpoint{
		RunID:     "mar_sensitive",
		TaskID:    "task_1",
		State:     MultiAgentStateFailed,
		Iteration: 0,
		Phase:     "coder",
		Error:     "failed with API_KEY=sk-abc123",
		Plan: &TaskPlan{
			Goal:  "Fix bug with token=secret123",
			Notes: "Contains PASSWORD=hunter2",
		},
		CreatedAt: time.Now().UTC(),
	}
	if err := store.Save(ctx, cp); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	// Read the raw file to verify sanitization
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read checkpoint file: %v", err)
	}

	raw := string(data)
	// Should not contain the actual API key/secret values
	if containsStr(raw, "sk-abc123") {
		t.Error("checkpoint file contains API key pattern")
	}
	if containsStr(raw, "secret123") {
		t.Error("checkpoint file contains secret in plan goal")
	}
	if containsStr(raw, "hunter2") {
		t.Error("checkpoint file contains password in plan notes")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestCheckpointSaveFailurePropagates(t *testing.T) {
	// Test that a cancelled context causes Save to fail,
	// simulating a checkpoint write failure that should cause run failure.
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "multi_agent_runs.jsonl")

	store, err := NewJSONLMultiAgentCheckpointStore(path)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Cancelled context should cause Save to fail
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cp := MultiAgentCheckpoint{
		RunID:     "mar_cancel",
		State:     MultiAgentStateRunning,
		CreatedAt: time.Now().UTC(),
	}
	if err := store.Save(ctx, cp); err == nil {
		t.Error("expected error saving checkpoint with cancelled context")
	}
}

func TestDefaultMultiAgentCheckpointPath(t *testing.T) {
	path := DefaultMultiAgentCheckpointPath("/my/repo")
	expected := filepath.Join("/my/repo", ".mimoneko", "checkpoints", "multi_agent_runs.jsonl")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}
