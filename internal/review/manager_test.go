package review

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/reasonforge/reasonforge/internal/patch"
	"github.com/reasonforge/reasonforge/internal/task"
)

// mockPatchManager implements patch.PatchManager for testing.
type mockPatchManager struct {
	preview patch.PatchPreview
	err     error
}

func (m *mockPatchManager) Preview(ctx context.Context, req patch.PatchPreviewRequest) (patch.PatchPreview, error) {
	return m.preview, m.err
}

func (m *mockPatchManager) Apply(ctx context.Context, req patch.PatchApplyRequest) (patch.PatchApplyResult, error) {
	return patch.PatchApplyResult{}, nil
}

func (m *mockPatchManager) Discard(ctx context.Context, req patch.PatchDiscardRequest) error {
	return nil
}

// mockValidationRunner implements ValidationRunner for testing.
type mockValidationRunner struct {
	result ValidationResult
	err    error
}

func (m *mockValidationRunner) Validate(ctx context.Context, req ValidationRequest) (ValidationResult, error) {
	return m.result, m.err
}

// mockModelReviewerImpl implements ModelReviewer for testing.
type mockModelReviewerImpl struct {
	result ModelReviewResult
	err    error
}

func (m *mockModelReviewerImpl) Review(ctx context.Context, req ModelReviewRequest) (ModelReviewResult, error) {
	return m.result, m.err
}

func makeLowRiskPreview() patch.PatchPreview {
	return patch.PatchPreview{
		WorktreeID: "wt_test",
		FilesChanged: []patch.FileChange{
			{Path: "main.go", Status: "modified", Additions: 5, Deletions: 2},
		},
		Diff:        "small diff",
		Summary:     patch.DiffSummary{FilesChanged: 1, Additions: 5, Deletions: 2},
		RiskLevel:   "low",
		GeneratedAt: time.Now().UTC(),
	}
}

func makeViolationPreview() patch.PatchPreview {
	return patch.PatchPreview{
		WorktreeID: "wt_test",
		FilesChanged: []patch.FileChange{
			{Path: ".env", Status: "modified", Additions: 1, Deletions: 0},
		},
		Diff:    "[diff redacted due to policy violations]",
		Summary: patch.DiffSummary{FilesChanged: 1, Additions: 1},
		Violations: []patch.PatchViolation{
			{Path: ".env", Reason: "sensitive file"},
		},
		RiskLevel:   "high",
		GeneratedAt: time.Now().UTC(),
	}
}

func makeHighRiskPreview() patch.PatchPreview {
	// 8 files with moderate changes -> triggers medium file count + medium line count
	// 30 + 30 = 60 = high risk (but not critical)
	files := make([]patch.FileChange, 8)
	for i := range files {
		files[i] = patch.FileChange{Path: "file.go", Status: "modified", Additions: 15, Deletions: 5}
	}
	return patch.PatchPreview{
		WorktreeID:   "wt_test",
		FilesChanged: files,
		Diff:         "diff content",
		Summary: patch.DiffSummary{
			FilesChanged: 8,
			Additions:    120,
			Deletions:    40,
		},
		RiskLevel:   "medium",
		GeneratedAt: time.Now().UTC(),
	}
}

func TestReview_Violations_Reject(t *testing.T) {
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeViolationPreview()},
		nil,
		nil,
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:   "/tmp/repo",
		WorktreeID: "wt_test",
		Contract:   task.DefaultContract("/tmp/repo", "review"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Recommendation != RecommendationReject {
		t.Errorf("expected reject, got %s", report.Recommendation)
	}
}

func TestReview_HighRisk_RequestChanges(t *testing.T) {
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeHighRiskPreview()},
		nil,
		nil,
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:   "/tmp/repo",
		WorktreeID: "wt_test",
		Contract:   task.DefaultContract("/tmp/repo", "review"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Recommendation != RecommendationRequestChanges {
		t.Errorf("expected request_changes, got %s", report.Recommendation)
	}
}

func TestReview_LowRisk_Approve(t *testing.T) {
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeLowRiskPreview()},
		nil,
		nil,
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:   "/tmp/repo",
		WorktreeID: "wt_test",
		Contract:   task.DefaultContract("/tmp/repo", "review"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Recommendation != RecommendationApprove {
		t.Errorf("expected approve, got %s", report.Recommendation)
	}
}

func TestReview_ValidationFails_RequestChanges(t *testing.T) {
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeLowRiskPreview()},
		&mockValidationRunner{
			result: ValidationResult{
				Success: false,
				Commands: []CommandValidationResult{
					{CommandName: "go-test", Success: false, ExitCode: 1},
				},
				Summary: "1 command failed",
			},
		},
		nil,
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:     "/tmp/repo",
		WorktreeID:   "wt_test",
		WorktreePath: "/tmp/worktree",
		Contract:     task.DefaultContract("/tmp/repo", "review"),
		RunTests:     true,
		TestCommands: []string{"go-test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Recommendation != RecommendationRequestChanges {
		t.Errorf("expected request_changes, got %s", report.Recommendation)
	}
}

func TestReview_ValidationSucceeds_Approve(t *testing.T) {
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeLowRiskPreview()},
		&mockValidationRunner{
			result: ValidationResult{
				Success: true,
				Commands: []CommandValidationResult{
					{CommandName: "go-test", Success: true, ExitCode: 0},
				},
				Summary: "all tests passed",
			},
		},
		nil,
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:     "/tmp/repo",
		WorktreeID:   "wt_test",
		WorktreePath: "/tmp/worktree",
		Contract:     task.DefaultContract("/tmp/repo", "review"),
		RunTests:     true,
		TestCommands: []string{"go-test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Recommendation != RecommendationApprove {
		t.Errorf("expected approve, got %s", report.Recommendation)
	}
}

func TestReview_ModelReject_OverridesApprove(t *testing.T) {
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeLowRiskPreview()},
		nil,
		&mockModelReviewerImpl{
			result: ModelReviewResult{
				Provider:       "mock",
				Model:          "mock-model",
				Summary:        "risky changes",
				Recommendation: RecommendationReject,
			},
		},
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:       "/tmp/repo",
		WorktreeID:     "wt_test",
		Contract:       task.DefaultContract("/tmp/repo", "review"),
		UseModelReview: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Recommendation != RecommendationReject {
		t.Errorf("expected reject from model, got %s", report.Recommendation)
	}
}

func TestReview_ModelApprove_CannotOverrideCriticalSafety(t *testing.T) {
	// Even if the model says approve, a critical safety finding must result in reject
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeViolationPreview()},
		nil,
		&mockModelReviewerImpl{
			result: ModelReviewResult{
				Provider:       "mock",
				Model:          "mock-model",
				Summary:        "looks fine",
				Recommendation: RecommendationApprove,
			},
		},
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:       "/tmp/repo",
		WorktreeID:     "wt_test",
		Contract:       task.DefaultContract("/tmp/repo", "review"),
		UseModelReview: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Recommendation != RecommendationReject {
		t.Errorf("expected reject despite model approve, got %s", report.Recommendation)
	}
}

func TestReview_ModelRequestChanges(t *testing.T) {
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeLowRiskPreview()},
		nil,
		&mockModelReviewerImpl{
			result: ModelReviewResult{
				Provider:       "mock",
				Model:          "mock-model",
				Summary:        "needs changes",
				Recommendation: RecommendationRequestChanges,
			},
		},
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:       "/tmp/repo",
		WorktreeID:     "wt_test",
		Contract:       task.DefaultContract("/tmp/repo", "review"),
		UseModelReview: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Recommendation != RecommendationRequestChanges {
		t.Errorf("expected request_changes from model, got %s", report.Recommendation)
	}
}

func TestReview_ModelFailure_WarningFinding(t *testing.T) {
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeLowRiskPreview()},
		nil,
		&mockModelReviewerImpl{
			err: fmt.Errorf("model unavailable"),
		},
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:       "/tmp/repo",
		WorktreeID:     "wt_test",
		Contract:       task.DefaultContract("/tmp/repo", "review"),
		UseModelReview: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, f := range report.Findings {
		if f.Category == CategoryModel && f.Severity == SeverityWarning {
			found = true
		}
	}
	if !found {
		t.Error("expected warning finding for model failure")
	}

	// Should still approve since no other issues
	if report.Recommendation != RecommendationApprove {
		t.Errorf("expected approve, got %s", report.Recommendation)
	}
}

func TestReview_ModelFailure_StrictMode(t *testing.T) {
	cfg := DefaultReviewConfig()
	cfg.StrictModelReview = true
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeLowRiskPreview()},
		nil,
		&mockModelReviewerImpl{
			err: fmt.Errorf("model unavailable"),
		},
		cfg,
	)

	_, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:       "/tmp/repo",
		WorktreeID:     "wt_test",
		Contract:       task.DefaultContract("/tmp/repo", "review"),
		UseModelReview: true,
	})
	if err == nil {
		t.Error("expected error in strict mode when model fails")
	}
}

func TestReview_NoModelReview_WhenNotRequested(t *testing.T) {
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeLowRiskPreview()},
		nil,
		&mockModelReviewerImpl{
			result: ModelReviewResult{
				Provider:       "mock",
				Model:          "mock-model",
				Summary:        "looks fine",
				Recommendation: RecommendationApprove,
			},
		},
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:       "/tmp/repo",
		WorktreeID:     "wt_test",
		Contract:       task.DefaultContract("/tmp/repo", "review"),
		UseModelReview: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.ModelReview != nil {
		t.Error("model review should not be populated when not requested")
	}
}

func TestReview_CriticalRiskLevel_Reject(t *testing.T) {
	// Create a preview that would cause critical risk score
	files := make([]patch.FileChange, 30)
	for i := range files {
		files[i] = patch.FileChange{Path: "file.go", Status: "modified", Additions: 50, Deletions: 50}
	}
	preview := patch.PatchPreview{
		WorktreeID:   "wt_test",
		FilesChanged: files,
		Summary: patch.DiffSummary{
			FilesChanged: 30,
			Additions:    1500,
			Deletions:    1500,
			HasBinary:    true,
		},
		RiskLevel:   "high",
		GeneratedAt: time.Now().UTC(),
	}

	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: preview},
		nil,
		nil,
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:   "/tmp/repo",
		WorktreeID: "wt_test",
		Contract:   task.DefaultContract("/tmp/repo", "review"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Critical risk from violations or critical findings should reject
	if report.RiskScore.Level != "critical" && report.Recommendation == RecommendationApprove {
		t.Error("should not approve when risk is critical")
	}
}

func TestReview_RunTestsFalse_NoValidation(t *testing.T) {
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeLowRiskPreview()},
		&mockValidationRunner{
			result: ValidationResult{Success: false},
		},
		nil,
		DefaultReviewConfig(),
	)

	report, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:     "/tmp/repo",
		WorktreeID:   "wt_test",
		Contract:     task.DefaultContract("/tmp/repo", "review"),
		RunTests:     false,
		TestCommands: []string{"go-test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Validation != nil {
		t.Error("validation should not run when RunTests is false")
	}
}

func TestComputeRecommendation_Deterministic(t *testing.T) {
	// Test all recommendation rules
	tests := []struct {
		name       string
		preview    patch.PatchPreview
		risk       RiskScore
		findings   []ReviewFinding
		validation *ValidationResult
		model      *ModelReviewResult
		want       ReviewRecommendation
	}{
		{
			name:     "critical finding rejects",
			findings: []ReviewFinding{{Severity: SeverityCritical}},
			risk:     RiskScore{Level: "low"},
			want:     RecommendationReject,
		},
		{
			name:    "violations reject",
			preview: patch.PatchPreview{Violations: []patch.PatchViolation{{Path: ".env"}}},
			risk:    RiskScore{Level: "low"},
			want:    RecommendationReject,
		},
		{
			name:       "validation fail request_changes",
			preview:    patch.PatchPreview{},
			risk:       RiskScore{Level: "low"},
			validation: &ValidationResult{Success: false},
			want:       RecommendationRequestChanges,
		},
		{
			name: "critical risk reject",
			risk: RiskScore{Level: "critical"},
			want: RecommendationReject,
		},
		{
			name: "high risk request_changes",
			risk: RiskScore{Level: "high"},
			want: RecommendationRequestChanges,
		},
		{
			name:  "model reject overrides approve",
			risk:  RiskScore{Level: "low"},
			model: &ModelReviewResult{Recommendation: RecommendationReject},
			want:  RecommendationReject,
		},
		{
			name:  "model request_changes",
			risk:  RiskScore{Level: "low"},
			model: &ModelReviewResult{Recommendation: RecommendationRequestChanges},
			want:  RecommendationRequestChanges,
		},
		{
			name: "low risk approve",
			risk: RiskScore{Level: "low"},
			want: RecommendationApprove,
		},
		{
			name:  "model approve cannot override high risk",
			risk:  RiskScore{Level: "high"},
			model: &ModelReviewResult{Recommendation: RecommendationApprove},
			want:  RecommendationRequestChanges,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeRecommendation(tt.preview, tt.risk, tt.findings, tt.validation, tt.model)
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// recordingValidationRunner captures the RepoRoot from ValidationRequest.
type recordingValidationRunner struct {
	capturedRepoRoot string
	result           ValidationResult
	err              error
}

func (r *recordingValidationRunner) Validate(ctx context.Context, req ValidationRequest) (ValidationResult, error) {
	r.capturedRepoRoot = req.RepoRoot
	return r.result, r.err
}

func TestPatchReviewManagerValidationUsesWorktreePath(t *testing.T) {
	runner := &recordingValidationRunner{
		result: ValidationResult{
			Success: true,
			Commands: []CommandValidationResult{
				{CommandName: "go-test", Success: true, ExitCode: 0},
			},
			Summary: "all tests passed",
		},
	}

	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeLowRiskPreview()},
		runner,
		nil,
		DefaultReviewConfig(),
	)

	_, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:     "/tmp/main-repo",
		WorktreeID:   "wt_test",
		WorktreePath: "/tmp/worktree-path",
		Contract:     task.DefaultContract("/tmp/main-repo", "review"),
		RunTests:     true,
		TestCommands: []string{"go-test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runner.capturedRepoRoot != "/tmp/worktree-path" {
		t.Errorf("validation RepoRoot = %q, want %q", runner.capturedRepoRoot, "/tmp/worktree-path")
	}
}

func TestPatchReviewManagerRunTestsRequiresWorktreePath(t *testing.T) {
	mgr := NewDefaultPatchReviewManager(
		&mockPatchManager{preview: makeLowRiskPreview()},
		&mockValidationRunner{result: ValidationResult{Success: true}},
		nil,
		DefaultReviewConfig(),
	)

	_, err := mgr.Review(context.Background(), PatchReviewRequest{
		RepoRoot:     "/tmp/main-repo",
		WorktreeID:   "wt_test",
		WorktreePath: "", // Empty - must error
		Contract:     task.DefaultContract("/tmp/main-repo", "review"),
		RunTests:     true,
		TestCommands: []string{"go-test"},
	})
	if err == nil {
		t.Fatal("expected error when RunTests=true and WorktreePath is empty")
	}
	if !strings.Contains(err.Error(), "WorktreePath") {
		t.Errorf("error should mention WorktreePath, got: %v", err)
	}
}
