package review

import (
	"context"
	"fmt"
	"time"

	"github.com/nekonomimo/nekonomimo/internal/events"
	"github.com/nekonomimo/nekonomimo/internal/patch"
	"github.com/nekonomimo/nekonomimo/internal/task"
)

// ValidationRunner executes test commands through ToolRuntime.
type ValidationRunner interface {
	Validate(ctx context.Context, req ValidationRequest) (ValidationResult, error)
}

// ValidationRequest is the input to ValidationRunner.Validate.
type ValidationRequest struct {
	// RepoRoot is the workspace root (should be a worktree path, not main path).
	RepoRoot string

	// TaskID identifies the task for audit tracing.
	TaskID string

	// TestCommands lists the test command names to execute.
	TestCommands []string

	// MaxOutputBytes caps the combined output size per command.
	MaxOutputBytes int

	// TimeoutSeconds caps the total validation duration.
	TimeoutSeconds int
}

// ModelReviewer performs optional AI model review.
type ModelReviewer interface {
	Review(ctx context.Context, req ModelReviewRequest) (ModelReviewResult, error)
}

// ModelReviewRequest is the input to ModelReviewer.Review.
type ModelReviewRequest struct {
	// RepoRoot is the main repository root.
	RepoRoot string

	// WorktreeID identifies the worktree being reviewed.
	WorktreeID string

	// Preview contains the patch preview data.
	Preview patch.PatchPreview

	// RuleFindings contains findings from the rule-based review.
	RuleFindings []ReviewFinding

	// Model specifies which model to use (empty = default).
	Model string
}

// ReviewConfig configures the review pipeline behavior.
type ReviewConfig struct {
	// MaxDiffBytes caps the diff output size.
	MaxDiffBytes int

	// HighRiskFileCount is the file count threshold for high risk.
	HighRiskFileCount int

	// MediumRiskFileCount is the file count threshold for medium risk.
	MediumRiskFileCount int

	// HighRiskLineCount is the line count threshold for high risk.
	HighRiskLineCount int

	// MediumRiskLineCount is the line count threshold for medium risk.
	MediumRiskLineCount int

	// RequireTestsForCodeChanges produces a warning when source code is modified
	// without corresponding test changes.
	RequireTestsForCodeChanges bool

	// AllowBinary controls whether binary files are allowed.
	AllowBinary bool

	// StrictModelReview causes the entire review to fail if the model review fails.
	StrictModelReview bool
}

// DefaultReviewConfig returns safe defaults.
func DefaultReviewConfig() ReviewConfig {
	return ReviewConfig{
		MaxDiffBytes:               131072,
		HighRiskFileCount:          20,
		MediumRiskFileCount:        5,
		HighRiskLineCount:          500,
		MediumRiskLineCount:        100,
		RequireTestsForCodeChanges: false,
		AllowBinary:                false,
		StrictModelReview:          false,
	}
}

// DefaultPatchReviewManager implements PatchReviewManager.
type DefaultPatchReviewManager struct {
	patchMgr         patch.PatchManager
	ruleReviewer     *RuleBasedReviewer
	riskScorer       *RiskScorer
	validationRunner ValidationRunner
	modelReviewer    ModelReviewer
	cfg              ReviewConfig
	eventEmitter     events.EventEmitter
}

// NewDefaultPatchReviewManager creates a new DefaultPatchReviewManager.
func NewDefaultPatchReviewManager(
	patchMgr patch.PatchManager,
	validationRunner ValidationRunner,
	modelReviewer ModelReviewer,
	cfg ReviewConfig,
) *DefaultPatchReviewManager {
	ruleCfg := RuleBasedReviewerConfig{
		MaxDiffBytes:               cfg.MaxDiffBytes,
		HighRiskFileCount:          cfg.HighRiskFileCount,
		MediumRiskFileCount:        cfg.MediumRiskFileCount,
		HighRiskLineCount:          cfg.HighRiskLineCount,
		MediumRiskLineCount:        cfg.MediumRiskLineCount,
		AllowBinary:                cfg.AllowBinary,
		RequireTestsForCodeChanges: cfg.RequireTestsForCodeChanges,
	}
	riskCfg := RiskScorerConfig{
		HighRiskFileCount:   cfg.HighRiskFileCount,
		MediumRiskFileCount: cfg.MediumRiskFileCount,
		HighRiskLineCount:   cfg.HighRiskLineCount,
		MediumRiskLineCount: cfg.MediumRiskLineCount,
	}

	return &DefaultPatchReviewManager{
		patchMgr:         patchMgr,
		ruleReviewer:     NewRuleBasedReviewer(ruleCfg),
		riskScorer:       NewRiskScorer(riskCfg),
		validationRunner: validationRunner,
		modelReviewer:    modelReviewer,
		cfg:              cfg,
		eventEmitter:     &events.NoopEventEmitter{},
	}
}

// SetEventEmitter sets the optional event emitter for review events.
func (m *DefaultPatchReviewManager) SetEventEmitter(emitter events.EventEmitter) {
	if emitter != nil {
		m.eventEmitter = emitter
	}
}

// Review executes the full review pipeline.
func (m *DefaultPatchReviewManager) Review(ctx context.Context, req PatchReviewRequest) (PatchReviewReport, error) {
	reviewStartedAt := time.Now().UTC()
	events.SafeEmit(m.eventEmitter, ctx, events.RunEvent{
		ID:        mustGenerateReviewEventID(),
		Type:      events.EventReviewerStarted,
		Source:    "review",
		Status:    "started",
		Message:   "Patch review started",
		StartedAt: reviewStartedAt,
		Metadata:  map[string]string{"worktree_id": req.WorktreeID},
	})

	// 1. Generate PatchPreview via PatchManager
	preview, err := m.patchMgr.Preview(ctx, patch.PatchPreviewRequest{
		RepoRoot:   req.RepoRoot,
		WorktreeID: req.WorktreeID,
		Contract:   req.Contract,
	})
	if err != nil {
		events.SafeEmit(m.eventEmitter, ctx, events.RunEvent{
			ID:         mustGenerateReviewEventID(),
			Type:       events.EventReviewerFinished,
			Source:     "review",
			Status:     "failed",
			Message:    "Patch review failed: preview error",
			StartedAt:  reviewStartedAt,
			FinishedAt: time.Now().UTC(),
			DurationMs: time.Since(reviewStartedAt).Milliseconds(),
			Error:      err.Error(),
		})
		return PatchReviewReport{}, fmt.Errorf("review: preview: %w", err)
	}

	// 2. Convert PatchPreview to PreviewData for rule-based review
	previewData := patchPreviewToPreviewData(preview)

	// 3. Rule-based review
	findings := m.ruleReviewer.Review(previewData)

	// 4. Risk scoring
	riskScore := m.riskScorer.Score(previewData, findings)

	// 5. Optional test validation
	var validation *ValidationResult
	if req.RunTests && len(req.TestCommands) > 0 {
		// WorktreePath is required for test validation.
		// Tests must execute in the isolated worktree, not the main workspace.
		if req.WorktreePath == "" {
			return PatchReviewReport{}, fmt.Errorf("review: WorktreePath is required when RunTests=true with TestCommands")
		}
		if m.validationRunner != nil {
			valReq := ValidationRequest{
				RepoRoot:       req.WorktreePath, // Use worktree path, NOT main repo root
				TaskID:         req.Contract.ID,
				TestCommands:   req.TestCommands,
				MaxOutputBytes: 65536,
				TimeoutSeconds: 120,
			}
			valResult, err := m.validationRunner.Validate(ctx, valReq)
			if err != nil {
				findings = append(findings, ReviewFinding{
					Severity: SeverityError,
					Category: CategoryTest,
					Message:  fmt.Sprintf("validation runner failed: %v", err),
				})
			} else {
				validation = &valResult
				// Add findings for failed commands
				for _, cmd := range valResult.Commands {
					if !cmd.Success {
						findings = append(findings, ReviewFinding{
							Severity: SeverityError,
							Category: CategoryTest,
							Path:     cmd.CommandName,
							Message:  fmt.Sprintf("test command %q failed (exit code %d)", cmd.CommandName, cmd.ExitCode),
						})
					}
				}
			}
		}
	}

	// 6. Optional model review
	var modelReview *ModelReviewResult
	if req.UseModelReview && m.modelReviewer != nil {
		modelResult, err := m.modelReviewer.Review(ctx, ModelReviewRequest{
			RepoRoot:     req.RepoRoot,
			WorktreeID:   req.WorktreeID,
			Preview:      preview,
			RuleFindings: findings,
			Model:        req.Model,
		})
		if err != nil {
			if m.cfg.StrictModelReview {
				return PatchReviewReport{}, fmt.Errorf("review: model review failed: %w", err)
			}
			findings = append(findings, ReviewFinding{
				Severity: SeverityWarning,
				Category: CategoryModel,
				Message:  fmt.Sprintf("model review failed: %v", err),
			})
		} else {
			modelReview = &modelResult
		}
	}

	// 7. Compute deterministic recommendation
	recommendation := computeRecommendation(preview, riskScore, findings, validation, modelReview)

	reviewFinishedAt := time.Now().UTC()
	reviewStatus := "succeeded"
	if recommendation == RecommendationReject {
		reviewStatus = "failed"
	}
	events.SafeEmit(m.eventEmitter, ctx, events.RunEvent{
		ID:         mustGenerateReviewEventID(),
		Type:       events.EventReviewerFinished,
		Source:     "review",
		Status:     reviewStatus,
		Message:    fmt.Sprintf("Patch review completed: %s", recommendation),
		StartedAt:  reviewStartedAt,
		FinishedAt: reviewFinishedAt,
		DurationMs: reviewFinishedAt.Sub(reviewStartedAt).Milliseconds(),
		Metadata: map[string]string{
			"recommendation": string(recommendation),
			"risk_level":     riskScore.Level,
			"worktree_id":    req.WorktreeID,
		},
	})

	return PatchReviewReport{
		WorktreeID:     req.WorktreeID,
		Preview:        preview,
		RiskScore:      riskScore,
		Findings:       findings,
		Validation:     validation,
		ModelReview:    modelReview,
		Recommendation: recommendation,
		CreatedAt:      time.Now().UTC(),
	}, nil
}

// computeRecommendation applies deterministic rules to determine the final recommendation.
// Safety rules always take priority over model suggestions.
func computeRecommendation(
	preview patch.PatchPreview,
	riskScore RiskScore,
	findings []ReviewFinding,
	validation *ValidationResult,
	modelReview *ModelReviewResult,
) ReviewRecommendation {
	// Rule 1: Has critical finding => reject
	for _, f := range findings {
		if f.Severity == SeverityCritical {
			return RecommendationReject
		}
	}

	// Rule 2: Has PatchPreview.Violations => reject
	if len(preview.Violations) > 0 {
		return RecommendationReject
	}

	// Rule 3: Validation failed => request_changes
	if validation != nil && !validation.Success {
		return RecommendationRequestChanges
	}

	// Rule 4: RiskScore critical => reject
	if riskScore.Level == "critical" {
		return RecommendationReject
	}

	// Rule 5: RiskScore high => request_changes
	if riskScore.Level == "high" {
		return RecommendationRequestChanges
	}

	// Rule 6: Model recommends reject => reject
	if modelReview != nil && modelReview.Recommendation == RecommendationReject {
		return RecommendationReject
	}

	// Rule 7: Model recommends request_changes => request_changes
	if modelReview != nil && modelReview.Recommendation == RecommendationRequestChanges {
		return RecommendationRequestChanges
	}

	// Rule 8: Otherwise approve
	return RecommendationApprove
}

// patchPreviewToPreviewData converts a PatchPreview to PreviewData for rule-based review.
func patchPreviewToPreviewData(p patch.PatchPreview) PreviewData {
	files := make([]FileChangeInfo, len(p.FilesChanged))
	for i, f := range p.FilesChanged {
		files[i] = FileChangeInfo{
			Path:      f.Path,
			Status:    f.Status,
			Additions: f.Additions,
			Deletions: f.Deletions,
		}
	}

	violations := make([]ViolationInfo, len(p.Violations))
	for i, v := range p.Violations {
		violations[i] = ViolationInfo{
			Path:   v.Path,
			Reason: v.Reason,
		}
	}

	// Diff is redacted if violations exist or if it was truncated
	diffRedacted := len(p.Violations) > 0 || p.Diff == "[diff redacted due to policy violations]"

	return PreviewData{
		WorktreeID:   p.WorktreeID,
		FilesChanged: files,
		Diff:         p.Diff,
		DiffRedacted: diffRedacted,
		Summary: SummaryInfo{
			FilesChanged: p.Summary.FilesChanged,
			Additions:    p.Summary.Additions,
			Deletions:    p.Summary.Deletions,
			HasBinary:    p.Summary.HasBinary,
		},
		Violations: violations,
	}
}

// PatchPreviewFromTaskContract creates a default TaskContract for review operations.
func PatchPreviewFromTaskContract(repoRoot string) task.TaskContract {
	return task.DefaultContract(repoRoot, "patch review")
}

// mustGenerateReviewEventID generates a unique event ID for review events.
func mustGenerateReviewEventID() string {
	id, err := events.GenerateEventID()
	if err != nil {
		return "evt_error"
	}
	return id
}
