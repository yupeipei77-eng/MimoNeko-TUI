package multiagent

import (
	"context"
	"fmt"
	"time"

	"github.com/mimoneko/mimoneko/internal/agent"
	"github.com/mimoneko/mimoneko/internal/review"
	"github.com/mimoneko/mimoneko/internal/task"
	"github.com/mimoneko/mimoneko/internal/worktree"
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

// ReviewCategory classifies the nature of review findings.
type ReviewCategory string

const (
	ReviewCategoryCodeIssue   ReviewCategory = "code_issue"
	ReviewCategoryPlanIssue   ReviewCategory = "plan_issue"
	ReviewCategorySafetyIssue ReviewCategory = "safety_issue"
)

// ReviewResult is the output of ReviewerAgent.Review.
type ReviewResult struct {
	Report         review.PatchReviewReport
	Message        AgentMessage
	Recommendation review.ReviewRecommendation
	Category       ReviewCategory
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

	// Classify the review findings
	category := classifyReviewFindings(report)

	msg := AgentMessage{
		Role:      AgentRoleReviewer,
		Content:   messageContent,
		Metadata:  map[string]string{"recommendation": string(recommendation), "risk_level": report.RiskScore.Level, "category": string(category)},
		CreatedAt: time.Now().UTC(),
	}

	return ReviewResult{
		Report:         report,
		Message:        msg,
		Recommendation: recommendation,
		Category:       category,
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

// classifyReviewFindings determines the primary review category based on findings.
func classifyReviewFindings(report review.PatchReviewReport) ReviewCategory {
	// Safety violations take priority
	if len(report.Preview.Violations) > 0 {
		return ReviewCategorySafetyIssue
	}

	// Check for critical security findings
	for _, f := range report.Findings {
		if f.Severity == review.SeverityCritical && f.Category == review.CategorySecurity {
			return ReviewCategorySafetyIssue
		}
	}

	// Check for plan-level issues (very large changes suggest plan problems)
	if report.Preview.Summary.FilesChanged > 10 || report.Preview.Summary.Additions+report.Preview.Summary.Deletions > 500 {
		return ReviewCategoryPlanIssue
	}

	// Check if validation failed (code issue)
	if report.Validation != nil && !report.Validation.Success {
		return ReviewCategoryCodeIssue
	}

	// Default to code issue
	return ReviewCategoryCodeIssue
}
