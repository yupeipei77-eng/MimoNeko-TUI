package multiagent

import (
	"context"
	"fmt"
	"time"

	"github.com/nekonomimo/nekonomimo/internal/agent"
	"github.com/nekonomimo/nekonomimo/internal/review"
	"github.com/nekonomimo/nekonomimo/internal/task"
	"github.com/nekonomimo/nekonomimo/internal/worktree"
)

// ReviewerAgent adapts PatchReviewManager for the multi-agent pipeline.
//
// Rules:
//   - Must delegate to PatchReviewManager.Review (not re-implement review logic)
//   - Must not bypass PatchReviewManager
//   - Deterministic reject cannot be overridden by model
//   - When validation fails, recommendation must be at least request_changes
//   - Must not apply patch
//   - Must not read files directly
//   - Must not execute test commands directly
type ReviewerAgent struct {
	reviewMgr   review.PatchReviewManager
	worktreeMgr worktree.WorktreeManager
	modelRouter modelReviewer // optional: for generating natural language summary
}

// modelReviewer is an optional interface for generating reviewer summaries via model.
// If nil, the reviewer uses only PatchReviewManager output.
type modelReviewer interface {
	GenerateSummary(ctx context.Context, report review.PatchReviewReport) (string, error)
}

// NewReviewerAgent creates a new ReviewerAgent.
func NewReviewerAgent(
	reviewMgr review.PatchReviewManager,
	worktreeMgr worktree.WorktreeManager,
) *ReviewerAgent {
	return &ReviewerAgent{
		reviewMgr:   reviewMgr,
		worktreeMgr: worktreeMgr,
	}
}

// SetModelReviewer sets an optional model reviewer for generating natural language summaries.
// The model reviewer cannot override PatchReviewManager's recommendation.
func (r *ReviewerAgent) SetModelReviewer(mr modelReviewer) {
	r.modelRouter = mr
}

// ReviewRequest is the input to ReviewerAgent.Review.
type ReviewRequest struct {
	CoderResult    agent.AgentRunResult
	Contract       task.TaskContract
	RepoRoot       string
	WorktreeID     string
	TaskID         string
	RunTests       bool
	TestCommands   []string
	UseModelReview bool
	Model          string
}

// ReviewResult is the output of ReviewerAgent.Review.
type ReviewResult struct {
	Report         review.PatchReviewReport
	Message        AgentMessage
	Recommendation review.ReviewRecommendation
}

// Review delegates to PatchReviewManager and optionally generates a summary.
func (r *ReviewerAgent) Review(ctx context.Context, req ReviewRequest) (ReviewResult, error) {
	// Get worktree path for test validation
	var worktreePath string
	if r.worktreeMgr != nil && req.WorktreeID != "" {
		wtInfo, err := r.worktreeMgr.Get(ctx, req.WorktreeID)
		if err != nil {
			return ReviewResult{}, fmt.Errorf("reviewer: get worktree %q: %w", req.WorktreeID, err)
		}
		worktreePath = wtInfo.Path
	}

	// Call PatchReviewManager.Review
	reviewReq := review.PatchReviewRequest{
		RepoRoot:       req.RepoRoot,
		WorktreeID:     req.WorktreeID,
		WorktreePath:   worktreePath,
		Contract:       req.Contract,
		RunTests:       req.RunTests,
		TestCommands:   req.TestCommands,
		UseModelReview: req.UseModelReview,
		Model:          req.Model,
	}

	report, err := r.reviewMgr.Review(ctx, reviewReq)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("reviewer: review failed: %w", err)
	}

	// The recommendation from PatchReviewManager is deterministic and must not be overridden.
	// This is the core safety rule: deterministic reject cannot be overridden by the model.
	recommendation := report.Recommendation

	// Optionally generate a natural language summary via model.
	// This does NOT override the PatchReviewManager recommendation.
	var summary string
	if r.modelRouter != nil {
		if modelSummary, err := r.modelRouter.GenerateSummary(ctx, report); err == nil {
			summary = modelSummary
		}
		// Model summary failure is non-fatal
	}

	// Build the reviewer message
	messageContent := buildReviewerMessage(report, recommendation, summary)

	msg := AgentMessage{
		Role:      AgentRoleReviewer,
		Content:   messageContent,
		Metadata:  map[string]string{"recommendation": string(recommendation), "risk_level": report.RiskScore.Level},
		CreatedAt: time.Now().UTC(),
	}

	return ReviewResult{
		Report:         report,
		Message:        msg,
		Recommendation: recommendation,
	}, nil
}

// buildReviewerMessage creates a human-readable summary of the review.
func buildReviewerMessage(report review.PatchReviewReport, recommendation review.ReviewRecommendation, modelSummary string) string {
	msg := fmt.Sprintf("Review complete: recommendation=%s, risk_level=%s",
		recommendation, report.RiskScore.Level)

	if len(report.Findings) > 0 {
		msg += fmt.Sprintf(", findings=%d", len(report.Findings))
	}

	if report.Validation != nil {
		msg += fmt.Sprintf(", validation_success=%v", report.Validation.Success)
	}

	if len(report.Preview.Violations) > 0 {
		msg += fmt.Sprintf(", violations=%d", len(report.Preview.Violations))
	}

	if modelSummary != "" {
		msg += "\n" + modelSummary
	}

	return msg
}
