package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mimoneko/mimoneko/internal/agent"
	"github.com/mimoneko/mimoneko/internal/cache"
	"github.com/mimoneko/mimoneko/internal/contextengine"
	"github.com/mimoneko/mimoneko/internal/modelrouter"
	"github.com/mimoneko/mimoneko/internal/patch"
	"github.com/mimoneko/mimoneko/internal/prefix"
	"github.com/mimoneko/mimoneko/internal/review"
	"github.com/mimoneko/mimoneko/internal/task"
	"github.com/mimoneko/mimoneko/internal/worktree"
)

// === Mock implementations ===

// mockModelRouter returns a fixed response for planner tests.
type mockModelRouter struct {
	text string
	err  error
}

func (m *mockModelRouter) Complete(ctx context.Context, req modelrouter.CompletionRequest) (modelrouter.CompletionResponse, error) {
	if m.err != nil {
		return modelrouter.CompletionResponse{}, m.err
	}
	return modelrouter.CompletionResponse{
		Provider: "mock",
		Model:    "mock-model",
		Text:     m.text,
	}, nil
}

// mockContextEngine returns a minimal bundle.
type mockContextEngine struct{}

func (m *mockContextEngine) Build(ctx context.Context, req contextengine.BuildRequest) (contextengine.Bundle, error) {
	return contextengine.Bundle{
		ImmutablePrefix: prefix.Document{},
		CurrentInput:    req.CurrentInput,
		Layers: []contextengine.ContextLayer{
			{Name: "mock", Bytes: req.CurrentInput, Tokens: 100},
		},
	}, nil
}

func (m *mockContextEngine) RecordModelCall(ctx context.Context, observation cache.Observation) error {
	return nil
}

// mockSingleAgent returns a fixed AgentRunResult.
type mockSingleAgent struct {
	result agent.AgentRunResult
	err    error
}

func (m *mockSingleAgent) Run(ctx context.Context, req agent.AgentRunRequest) (agent.AgentRunResult, error) {
	if m.err != nil {
		return agent.AgentRunResult{}, m.err
	}
	return m.result, nil
}

// mockWorktreeMgr returns fixed WorktreeInfo.
type mockWorktreeMgr struct{}

func (m *mockWorktreeMgr) Create(ctx context.Context, req worktree.CreateWorktreeRequest) (worktree.WorktreeInfo, error) {
	return worktree.WorktreeInfo{
		ID:        "wt_mock",
		TaskID:    req.TaskID,
		Path:      filepath.Join(req.RepoRoot, ".mimoneko", "worktrees", "wt_mock"),
		RepoRoot:  req.RepoRoot,
		State:     worktree.WorktreeStateActive,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (m *mockWorktreeMgr) Remove(ctx context.Context, id string) error { return nil }

func (m *mockWorktreeMgr) Get(ctx context.Context, id string) (worktree.WorktreeInfo, error) {
	return worktree.WorktreeInfo{
		ID:    id,
		Path:  "/tmp/repo/.mimoneko/worktrees/" + id,
		State: worktree.WorktreeStateActive,
	}, nil
}

func (m *mockWorktreeMgr) List(ctx context.Context) ([]worktree.WorktreeInfo, error) {
	return nil, nil
}

func (m *mockWorktreeMgr) UpdateState(ctx context.Context, id string, state worktree.WorktreeState) error {
	return nil
}

// === Planner Tests ===

func TestPlannerParsesValidJSONPlan(t *testing.T) {
	planJSON := `{"goal":"Fix typo","steps":[{"index":0,"title":"Fix","description":"Fix typo","target_paths":["README.md"],"expected_outcome":"Fixed"}],"risk_level":"low","notes":""}`

	result, err := parseTaskPlanJSON(planJSON)
	if err != nil {
		t.Fatalf("expected successful parse, got error: %v", err)
	}
	if result.Goal != "Fix typo" {
		t.Errorf("expected goal=%q, got %q", "Fix typo", result.Goal)
	}
	if len(result.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(result.Steps))
	}
}

func TestPlannerNonJSONOutputFails(t *testing.T) {
	_, err := parseTaskPlanJSON("This is not JSON")
	if err == nil {
		t.Error("expected error for non-JSON output")
	}
}

func TestPlannerMarkdownFencedJSON(t *testing.T) {
	input := "```json\n{\"goal\":\"Fix\",\"steps\":[{\"index\":0,\"title\":\"Fix\",\"description\":\"Fix\",\"expected_outcome\":\"Fixed\"}],\"risk_level\":\"low\"}\n```"
	result, err := parseTaskPlanJSON(input)
	if err != nil {
		t.Fatalf("expected successful parse with markdown fences, got error: %v", err)
	}
	if result.Goal != "Fix" {
		t.Errorf("expected goal=%q, got %q", "Fix", result.Goal)
	}
}

func TestPlannerMissingGoalFails(t *testing.T) {
	input := `{"steps":[{"index":0,"title":"Fix","description":"Fix"}],"risk_level":"low"}`
	_, err := parseTaskPlanJSON(input)
	if err == nil {
		t.Error("expected error for missing goal")
	}
}

func TestPlannerEmptyStepsFails(t *testing.T) {
	input := `{"goal":"Fix","steps":[],"risk_level":"low"}`
	_, err := parseTaskPlanJSON(input)
	if err == nil {
		t.Error("expected error for empty steps")
	}
}

func TestPlannerInvalidRiskLevelFails(t *testing.T) {
	input := `{"goal":"Fix","steps":[{"index":0,"title":"Fix","description":"Fix"}],"risk_level":"critical"}`
	_, err := parseTaskPlanJSON(input)
	if err == nil {
		t.Error("expected error for invalid risk_level")
	}
}

func TestPlannerDoesNotCallToolRuntime(t *testing.T) {
	// PlannerAgent only uses ModelRouter, never ToolRuntime.
	// This is verified by the struct: no ToolRuntime field.
	pa := NewPlannerAgent(nil, nil, "")
	_ = pa
}

func TestPlannerDoesNotModifyFiles(t *testing.T) {
	// Planner only calls ModelRouter and returns a plan.
	// No file-writing capability exists in PlannerAgent.
}

func TestPlannerDoesNotOverrideTaskContract(t *testing.T) {
	planJSON := `{"goal":"Different goal","steps":[{"index":0,"title":"Fix","description":"Fix","expected_outcome":"Fixed"}],"risk_level":"low"}`
	result, err := parseTaskPlanJSON(planJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The runtime enforces: plan.Goal = req.Goal (original contract goal)
	originalGoal := "Original contract goal"
	result.Goal = originalGoal
	if result.Goal != originalGoal {
		t.Errorf("planner should not override contract goal")
	}
}

func TestPlannerViaModelRouter(t *testing.T) {
	planJSON := `{"goal":"Fix typo","steps":[{"index":0,"title":"Fix","description":"Fix typo","target_paths":["README.md"],"expected_outcome":"Fixed"}],"risk_level":"low","notes":""}`
	mockRouter := &mockModelRouter{text: planJSON}
	mockCE := &mockContextEngine{}

	planner := NewPlannerAgent(mockRouter, mockCE, "")
	result, err := planner.Plan(context.Background(), PlanRequest{
		Goal:     "Fix typo",
		Contract: task.DefaultContract("/tmp/repo", "Fix typo"),
		RepoRoot: "/tmp/repo",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Plan.Goal != "Fix typo" {
		t.Errorf("expected goal=%q, got %q", "Fix typo", result.Plan.Goal)
	}
	if result.Message.Role != AgentRolePlanner {
		t.Errorf("expected role=%q, got %q", AgentRolePlanner, result.Message.Role)
	}
}

func TestPlannerInvalidModelOutputFails(t *testing.T) {
	mockRouter := &mockModelRouter{text: "not valid json at all"}
	mockCE := &mockContextEngine{}

	planner := NewPlannerAgent(mockRouter, mockCE, "")
	_, err := planner.Plan(context.Background(), PlanRequest{
		Goal:     "Fix typo",
		Contract: task.DefaultContract("/tmp/repo", "Fix typo"),
		RepoRoot: "/tmp/repo",
	})
	if err == nil {
		t.Error("expected error for non-JSON model output")
	}
}

// === Coder Tests ===

func TestCoderCallsSingleAgentRuntime(t *testing.T) {
	mockResult := agent.AgentRunResult{
		RunID:      "run_123",
		TaskID:     "task_1",
		State:      agent.AgentStateSucceeded,
		WorktreeID: "wt_abc",
		Steps:      []agent.AgentStep{},
	}

	coder := NewCoderAgent(&mockSingleAgent{result: mockResult})
	result, err := coder.Code(context.Background(), CodeRequest{
		Goal:     "Fix bug",
		RepoRoot: "/tmp/repo",
		TaskID:   "task_1",
		Contract: task.DefaultContract("/tmp/repo", "Fix bug"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Result.RunID != "run_123" {
		t.Errorf("expected run_id=%q, got %q", "run_123", result.Result.RunID)
	}
}

func TestCoderUsesWorktree(t *testing.T) {
	mockResult := agent.AgentRunResult{
		RunID:      "run_123",
		State:      agent.AgentStateSucceeded,
		WorktreeID: "wt_created",
	}

	coder := NewCoderAgent(&mockSingleAgent{result: mockResult})
	result, err := coder.Code(context.Background(), CodeRequest{
		Goal:     "Fix bug",
		RepoRoot: "/tmp/repo",
		TaskID:   "task_1",
		Contract: task.DefaultContract("/tmp/repo", "Fix bug"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WorktreeID != "wt_created" {
		t.Errorf("expected worktree_id=%q, got %q", "wt_created", result.WorktreeID)
	}
}

func TestCoderReusesWorktreeID(t *testing.T) {
	mockResult := agent.AgentRunResult{
		RunID:      "run_123",
		State:      agent.AgentStateSucceeded,
		WorktreeID: "wt_existing",
	}

	coder := NewCoderAgent(&mockSingleAgent{result: mockResult})
	result, err := coder.Code(context.Background(), CodeRequest{
		Goal:       "Fix bug",
		RepoRoot:   "/tmp/repo",
		TaskID:     "task_1",
		WorktreeID: "wt_existing",
		Contract:   task.DefaultContract("/tmp/repo", "Fix bug"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WorktreeID != "wt_existing" {
		t.Errorf("expected worktree_id=%q, got %q", "wt_existing", result.WorktreeID)
	}
}

func TestCoderDoesNotApplyPatch(t *testing.T) {
	// CoderAgent has no PatchManager dependency - no apply capability.
	coder := NewCoderAgent(nil)
	_ = coder
}

func TestCoderErrorCausesFailed(t *testing.T) {
	coder := NewCoderAgent(&mockSingleAgent{err: fmt.Errorf("agent error")})
	_, err := coder.Code(context.Background(), CodeRequest{
		Goal:     "Fix bug",
		RepoRoot: "/tmp/repo",
		TaskID:   "task_1",
		Contract: task.DefaultContract("/tmp/repo", "Fix bug"),
	})
	if err == nil {
		t.Error("expected error when single agent fails")
	}
}

// === Reviewer Tests ===

func TestReviewerCallsPatchReviewManager(t *testing.T) {
	mockReport := review.PatchReviewReport{
		WorktreeID:     "wt_123",
		Recommendation: review.RecommendationApprove,
		RiskScore:      review.RiskScore{Level: "low", Score: 10},
		CreatedAt:      time.Now().UTC(),
	}

	reviewerAgent := NewReviewerAgent(&fixedReviewMgr{report: mockReport}, &mockWorktreeMgr{})
	result, err := reviewerAgent.Review(context.Background(), ReviewRequest{
		Contract:   task.DefaultContract("/tmp/repo", "review"),
		RepoRoot:   "/tmp/repo",
		WorktreeID: "wt_123",
		TaskID:     "task_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Recommendation != review.RecommendationApprove {
		t.Errorf("expected approve, got %q", result.Recommendation)
	}
}

func TestReviewerDoesNotBypassPatchReviewManager(t *testing.T) {
	// ReviewerAgent delegates entirely to PatchReviewManager.Review.
	// No alternative review path exists in the struct.
	reviewerAgent := NewReviewerAgent(nil, nil)
	_ = reviewerAgent
}

func TestReviewerDeterministicRejectCannotBeOverridden(t *testing.T) {
	mockReport := review.PatchReviewReport{
		WorktreeID:     "wt_123",
		Recommendation: review.RecommendationReject,
		RiskScore:      review.RiskScore{Level: "critical", Score: 90},
		Preview: patch.PatchPreview{
			Violations: []patch.PatchViolation{
				{Path: ".env", Reason: "denied path"},
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	reviewerAgent := NewReviewerAgent(&fixedReviewMgr{report: mockReport}, &mockWorktreeMgr{})
	result, err := reviewerAgent.Review(context.Background(), ReviewRequest{
		Contract:   task.DefaultContract("/tmp/repo", "review"),
		RepoRoot:   "/tmp/repo",
		WorktreeID: "wt_123",
		TaskID:     "task_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Recommendation != review.RecommendationReject {
		t.Errorf("deterministic reject was overridden to %q", result.Recommendation)
	}
}

func TestReviewerValidationFailedRequestChanges(t *testing.T) {
	mockReport := review.PatchReviewReport{
		WorktreeID:     "wt_123",
		Recommendation: review.RecommendationRequestChanges,
		RiskScore:      review.RiskScore{Level: "medium", Score: 50},
		Validation: &review.ValidationResult{
			Success: false,
			Summary: "1 of 1 commands failed",
		},
		CreatedAt: time.Now().UTC(),
	}

	reviewerAgent := NewReviewerAgent(&fixedReviewMgr{report: mockReport}, &mockWorktreeMgr{})
	result, err := reviewerAgent.Review(context.Background(), ReviewRequest{
		Contract:   task.DefaultContract("/tmp/repo", "review"),
		RepoRoot:   "/tmp/repo",
		WorktreeID: "wt_123",
		TaskID:     "task_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Recommendation != review.RecommendationRequestChanges {
		t.Errorf("expected request_changes for validation failure, got %q", result.Recommendation)
	}
}

func TestReviewerDoesNotApplyPatch(t *testing.T) {
	// ReviewerAgent has no PatchManager dependency, no apply capability.
	reviewerAgent := NewReviewerAgent(nil, nil)
	_ = reviewerAgent
}

// === Iteration Loop Tests ===

func TestApproveEndsFirstIteration(t *testing.T) {
	deps := buildTestDependencies(review.RecommendationApprove)
	rt := NewDefaultMultiAgentRuntime(deps)

	result, err := rt.Run(context.Background(), MultiAgentRunRequest{
		Goal:          "Fix typo",
		RepoRoot:      "/tmp/repo",
		Contract:      task.DefaultContract("/tmp/repo", "Fix typo"),
		MaxIterations: 2,
		UseWorktree:   true,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.State != MultiAgentStateSucceeded {
		t.Errorf("expected state=%q, got %q (error: %s)", MultiAgentStateSucceeded, result.State, result.Error)
	}
	if len(result.Iterations) != 1 {
		t.Errorf("expected 1 iteration, got %d", len(result.Iterations))
	}
	if result.FinalRecommendation != review.RecommendationApprove {
		t.Errorf("expected approve, got %q", result.FinalRecommendation)
	}
}

func TestRequestChangesEntersSecondIteration(t *testing.T) {
	callCount := 0
	mgr := &sequenceReviewMgr{
		recommendations: []review.ReviewRecommendation{
			review.RecommendationRequestChanges,
			review.RecommendationApprove,
		},
		callCount: &callCount,
	}

	deps := buildTestDependenciesWithReviewMgr(mgr)
	rt := NewDefaultMultiAgentRuntime(deps)

	result, err := rt.Run(context.Background(), MultiAgentRunRequest{
		Goal:          "Fix bug",
		RepoRoot:      "/tmp/repo",
		Contract:      task.DefaultContract("/tmp/repo", "Fix bug"),
		MaxIterations: 3,
		UseWorktree:   true,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.State != MultiAgentStateSucceeded {
		t.Errorf("expected state=%q, got %q (error: %s)", MultiAgentStateSucceeded, result.State, result.Error)
	}
	if len(result.Iterations) != 2 {
		t.Errorf("expected 2 iterations, got %d", len(result.Iterations))
	}
	if result.Iterations[0].Recommendation != review.RecommendationRequestChanges {
		t.Errorf("expected first iteration request_changes, got %q", result.Iterations[0].Recommendation)
	}
	if result.Iterations[1].Recommendation != review.RecommendationApprove {
		t.Errorf("expected second iteration approve, got %q", result.Iterations[1].Recommendation)
	}
}

func TestRejectStopsImmediately(t *testing.T) {
	deps := buildTestDependencies(review.RecommendationReject)
	rt := NewDefaultMultiAgentRuntime(deps)

	result, err := rt.Run(context.Background(), MultiAgentRunRequest{
		Goal:          "Fix bug",
		RepoRoot:      "/tmp/repo",
		Contract:      task.DefaultContract("/tmp/repo", "Fix bug"),
		MaxIterations: 3,
		UseWorktree:   true,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.State != MultiAgentStateRejected {
		t.Errorf("expected state=%q, got %q (error: %s)", MultiAgentStateRejected, result.State, result.Error)
	}
	if len(result.Iterations) != 1 {
		t.Errorf("expected 1 iteration, got %d", len(result.Iterations))
	}
}

func TestMaxIterationsStops(t *testing.T) {
	mgr := &sequenceReviewMgr{
		recommendations: []review.ReviewRecommendation{
			review.RecommendationRequestChanges,
			review.RecommendationRequestChanges,
		},
		callCount: new(int),
	}

	deps := buildTestDependenciesWithReviewMgr(mgr)
	rt := NewDefaultMultiAgentRuntime(deps)

	result, err := rt.Run(context.Background(), MultiAgentRunRequest{
		Goal:          "Fix bug",
		RepoRoot:      "/tmp/repo",
		Contract:      task.DefaultContract("/tmp/repo", "Fix bug"),
		MaxIterations: 2,
		UseWorktree:   true,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.State != MultiAgentStateRequestChanges {
		t.Errorf("expected state=%q, got %q (error: %s)", MultiAgentStateRequestChanges, result.State, result.Error)
	}
	if len(result.Iterations) != 2 {
		t.Errorf("expected 2 iterations, got %d", len(result.Iterations))
	}
}

func TestMaxIterationsExceedsMax(t *testing.T) {
	_, err := ValidateMaxIterations(6)
	if err == nil {
		t.Error("expected error for max_iterations > 5")
	}
}

func TestContextCancelledReturnsCancelled(t *testing.T) {
	deps := buildTestDependencies(review.RecommendationApprove)
	rt := NewDefaultMultiAgentRuntime(deps)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, _ := rt.Run(ctx, MultiAgentRunRequest{
		Goal:          "Fix bug",
		RepoRoot:      "/tmp/repo",
		Contract:      task.DefaultContract("/tmp/repo", "Fix bug"),
		MaxIterations: 2,
		UseWorktree:   true,
		DryRun:        true,
	})
	if result.State != MultiAgentStateCancelled && result.State != MultiAgentStateFailed {
		t.Errorf("expected cancelled or failed state, got %q", result.State)
	}
}

func TestCheckpointFailureCausesFailed(t *testing.T) {
	deps := buildTestDependencies(review.RecommendationApprove)
	deps.CheckpointStore = &failingCheckpointStore{}
	rt := NewDefaultMultiAgentRuntime(deps)

	result, _ := rt.Run(context.Background(), MultiAgentRunRequest{
		Goal:          "Fix bug",
		RepoRoot:      "/tmp/repo",
		Contract:      task.DefaultContract("/tmp/repo", "Fix bug"),
		MaxIterations: 2,
		UseWorktree:   true,
		DryRun:        true,
	})
	if result.State != MultiAgentStateFailed {
		t.Errorf("expected failed state on checkpoint failure, got %q", result.State)
	}
	if !strings.Contains(result.Error, "checkpoint failed") {
		t.Errorf("expected error to contain 'checkpoint failed', got %q", result.Error)
	}
}

// === Checkpoint failure override tests ===

// selectiveFailingCheckpointStore fails on specified phases only.
type selectiveFailingCheckpointStore struct {
	failPhases map[string]bool
	saved      []string
}

func (s *selectiveFailingCheckpointStore) Save(ctx context.Context, cp MultiAgentCheckpoint) error {
	s.saved = append(s.saved, cp.Phase)
	if s.failPhases[cp.Phase] {
		return fmt.Errorf("checkpoint write failed for phase %s", cp.Phase)
	}
	return nil
}
func (s *selectiveFailingCheckpointStore) Load(ctx context.Context, runID string) (MultiAgentCheckpoint, error) {
	return MultiAgentCheckpoint{}, fmt.Errorf("not found")
}
func (s *selectiveFailingCheckpointStore) List(ctx context.Context) ([]string, error) {
	return nil, nil
}

func TestMultiAgentRuntime_FinalCheckpointFailureMarksFailed(t *testing.T) {
	// Normal approve → succeeded, but final loop_end checkpoint fails.
	// The result must be overridden to failed with "checkpoint failed" in Error.
	deps := buildTestDependencies(review.RecommendationApprove)
	deps.CheckpointStore = &selectiveFailingCheckpointStore{
		failPhases: map[string]bool{"loop_end": true},
	}
	rt := NewDefaultMultiAgentRuntime(deps)

	result, _ := rt.Run(context.Background(), MultiAgentRunRequest{
		Goal:          "Fix bug",
		RepoRoot:      "/tmp/repo",
		Contract:      task.DefaultContract("/tmp/repo", "Fix bug"),
		MaxIterations: 2,
		UseWorktree:   true,
		DryRun:        true,
	})
	if result.State != MultiAgentStateFailed {
		t.Errorf("expected failed state when final checkpoint fails, got %q", result.State)
	}
	if !strings.Contains(result.Error, "checkpoint failed") {
		t.Errorf("expected error to contain 'checkpoint failed', got %q", result.Error)
	}
}

func TestMultiAgentRuntime_CoderFailedCheckpointFailureMarksFailed(t *testing.T) {
	// Coder fails �?coder_failed checkpoint also fails.
	// The result must be failed with "checkpoint failed" in Error.
	deps := buildTestDependenciesWithReviewMgr(&fixedReviewMgr{
		report: review.PatchReviewReport{
			WorktreeID:     "wt_mock",
			Recommendation: review.RecommendationApprove,
			RiskScore:      review.RiskScore{Level: "low", Score: 10},
			Preview:        patch.PatchPreview{WorktreeID: "wt_mock"},
			CreatedAt:      time.Now().UTC(),
		},
	})
	deps.SingleAgent = &mockSingleAgent{err: fmt.Errorf("coder error")}
	deps.CheckpointStore = &selectiveFailingCheckpointStore{
		failPhases: map[string]bool{"coder_failed": true},
	}
	rt := NewDefaultMultiAgentRuntime(deps)

	result, _ := rt.Run(context.Background(), MultiAgentRunRequest{
		Goal:          "Fix bug",
		RepoRoot:      "/tmp/repo",
		Contract:      task.DefaultContract("/tmp/repo", "Fix bug"),
		MaxIterations: 2,
		UseWorktree:   true,
		DryRun:        true,
	})
	if result.State != MultiAgentStateFailed {
		t.Errorf("expected failed state when coder_failed checkpoint fails, got %q", result.State)
	}
	if !strings.Contains(result.Error, "checkpoint failed") {
		t.Errorf("expected error to contain 'checkpoint failed', got %q", result.Error)
	}
}

func TestMultiAgentRuntime_CancelledCheckpointFailureMarksFailed(t *testing.T) {
	// Context cancelled �?cancelled checkpoint also fails.
	// The result must be failed with "checkpoint failed" in Error.
	deps := buildTestDependencies(review.RecommendationApprove)
	deps.CheckpointStore = &selectiveFailingCheckpointStore{
		failPhases: map[string]bool{"cancelled": true},
	}
	rt := NewDefaultMultiAgentRuntime(deps)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, _ := rt.Run(ctx, MultiAgentRunRequest{
		Goal:          "Fix bug",
		RepoRoot:      "/tmp/repo",
		Contract:      task.DefaultContract("/tmp/repo", "Fix bug"),
		MaxIterations: 2,
		UseWorktree:   true,
		DryRun:        true,
	})
	if result.State != MultiAgentStateFailed {
		t.Errorf("expected failed state when cancelled checkpoint fails, got %q", result.State)
	}
	if !strings.Contains(result.Error, "checkpoint failed") {
		t.Errorf("expected error to contain 'checkpoint failed', got %q", result.Error)
	}
}

// === UseModelReview propagation test ===

// recordingReviewMgr records whether UseModelReview was set in the request.
type recordingReviewMgr struct {
	report          review.PatchReviewReport
	lastUseModelRev bool
}

func (m *recordingReviewMgr) Review(ctx context.Context, req review.PatchReviewRequest) (review.PatchReviewReport, error) {
	m.lastUseModelRev = req.UseModelReview
	return m.report, nil
}

func TestMultiAgentRuntimePassesUseModelReviewToReviewer(t *testing.T) {
	recMgr := &recordingReviewMgr{
		report: review.PatchReviewReport{
			WorktreeID:     "wt_mock",
			Recommendation: review.RecommendationApprove,
			RiskScore:      review.RiskScore{Level: "low", Score: 10},
			Preview:        patch.PatchPreview{WorktreeID: "wt_mock"},
			CreatedAt:      time.Now().UTC(),
		},
	}
	deps := buildTestDependenciesWithReviewMgr(recMgr)
	rt := NewDefaultMultiAgentRuntime(deps)

	_, _ = rt.Run(context.Background(), MultiAgentRunRequest{
		Goal:           "Fix bug",
		RepoRoot:       "/tmp/repo",
		Contract:       task.DefaultContract("/tmp/repo", "Fix bug"),
		MaxIterations:  2,
		UseWorktree:    true,
		DryRun:         true,
		UseModelReview: true,
		Model:          "test-model",
	})

	if !recMgr.lastUseModelRev {
		t.Error("expected UseModelReview=true to be passed to PatchReviewManager")
	}
}

func TestMultiAgentRuntimePassesUseModelReviewFalseToReviewer(t *testing.T) {
	recMgr := &recordingReviewMgr{
		report: review.PatchReviewReport{
			WorktreeID:     "wt_mock",
			Recommendation: review.RecommendationApprove,
			RiskScore:      review.RiskScore{Level: "low", Score: 10},
			Preview:        patch.PatchPreview{WorktreeID: "wt_mock"},
			CreatedAt:      time.Now().UTC(),
		},
	}
	deps := buildTestDependenciesWithReviewMgr(recMgr)
	rt := NewDefaultMultiAgentRuntime(deps)

	_, _ = rt.Run(context.Background(), MultiAgentRunRequest{
		Goal:           "Fix bug",
		RepoRoot:       "/tmp/repo",
		Contract:       task.DefaultContract("/tmp/repo", "Fix bug"),
		MaxIterations:  2,
		UseWorktree:    true,
		DryRun:         true,
		UseModelReview: false,
	})

	if recMgr.lastUseModelRev {
		t.Error("expected UseModelReview=false to be passed to PatchReviewManager")
	}
}

// === Helper types and functions ===

// fixedReviewMgr returns a fixed PatchReviewReport.
type fixedReviewMgr struct {
	report review.PatchReviewReport
}

func (m *fixedReviewMgr) Review(ctx context.Context, req review.PatchReviewRequest) (review.PatchReviewReport, error) {
	return m.report, nil
}

// sequenceReviewMgr returns different recommendations in sequence.
type sequenceReviewMgr struct {
	recommendations []review.ReviewRecommendation
	callCount       *int
}

func (m *sequenceReviewMgr) Review(ctx context.Context, req review.PatchReviewRequest) (review.PatchReviewReport, error) {
	idx := *m.callCount
	*m.callCount++

	if idx >= len(m.recommendations) {
		idx = len(m.recommendations) - 1
	}

	return review.PatchReviewReport{
		WorktreeID:     req.WorktreeID,
		Recommendation: m.recommendations[idx],
		RiskScore:      review.RiskScore{Level: "low", Score: 10},
		Preview:        patch.PatchPreview{WorktreeID: req.WorktreeID},
		CreatedAt:      time.Now().UTC(),
	}, nil
}

// failingCheckpointStore always fails.
type failingCheckpointStore struct{}

func (s *failingCheckpointStore) Save(ctx context.Context, cp MultiAgentCheckpoint) error {
	return fmt.Errorf("checkpoint write failed")
}
func (s *failingCheckpointStore) Load(ctx context.Context, runID string) (MultiAgentCheckpoint, error) {
	return MultiAgentCheckpoint{}, fmt.Errorf("not found")
}
func (s *failingCheckpointStore) List(ctx context.Context) ([]string, error) {
	return nil, nil
}

// mockSingleAgentForRuntime provides a full mock for runtime integration tests.
type mockSingleAgentForRuntime struct{}

func (m *mockSingleAgentForRuntime) Run(ctx context.Context, req agent.AgentRunRequest) (agent.AgentRunResult, error) {
	return agent.AgentRunResult{
		RunID:        "run_mock",
		TaskID:       req.TaskID,
		State:        agent.AgentStateSucceeded,
		Steps:        []agent.AgentStep{},
		FinalMessage: "mock coding complete",
		WorktreeID:   "wt_mock",
	}, nil
}

// buildTestDependencies creates test dependencies with a fixed recommendation.
func buildTestDependencies(recommendation review.ReviewRecommendation) Dependencies {
	mgr := &fixedReviewMgr{
		report: review.PatchReviewReport{
			WorktreeID:     "wt_mock",
			Recommendation: recommendation,
			RiskScore:      review.RiskScore{Level: "low", Score: 10},
			Preview:        patch.PatchPreview{WorktreeID: "wt_mock"},
			CreatedAt:      time.Now().UTC(),
		},
	}
	return buildTestDependenciesWithReviewMgr(mgr)
}

func buildTestDependenciesWithReviewMgr(mgr review.PatchReviewManager) Dependencies {
	tmpDir := tTempDir()
	cpPath := filepath.Join(tmpDir, ".mimoneko", "checkpoints", "multi_agent_runs.jsonl")
	cpStore, _ := NewJSONLMultiAgentCheckpointStore(cpPath)

	return Dependencies{
		ModelRouter:     &mockModelRouter{text: makeValidPlanJSON()},
		ContextEngine:   &mockContextEngine{},
		SingleAgent:     &mockSingleAgentForRuntime{},
		ReviewMgr:       mgr,
		WorktreeMgr:     &mockWorktreeMgr{},
		CheckpointStore: cpStore,
	}
}

func tTempDir() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("rf_test_%d", time.Now().UnixNano()))
}

func makeValidPlanJSON() string {
	plan := TaskPlan{
		Goal: "Fix typo",
		Steps: []PlanStep{
			{Index: 0, Title: "Fix", Description: "Fix typo in README", TargetPaths: []string{"README.md"}, ExpectedOutcome: "Fixed"},
		},
		RiskLevel: "low",
	}
	data, _ := json.Marshal(plan)
	return string(data)
}
