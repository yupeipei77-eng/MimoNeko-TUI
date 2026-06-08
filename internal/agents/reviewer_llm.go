package agents

import (
	"context"
	"fmt"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/contextengine"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/modelrouter"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"
)

// ReviewerLLM 调用 LLM 审查 Patch Intent
type ReviewerLLM struct {
	router modelrouter.ModelRouter
}

// NewReviewerLLM 创建 ReviewerLLM
func NewReviewerLLM(router modelrouter.ModelRouter) *ReviewerLLM {
	return &ReviewerLLM{router: router}
}

// ReviewIntent 调用 LLM 审查 Patch Intent
func (r *ReviewerLLM) ReviewIntent(ctx context.Context, intent *CoderPatchIntent, bundle contextengine.Bundle) (*ReviewerIntentReview, error) {
	// 验证 intent
	if intent.ImplementationStatus != ImplementationStatusIntentOnly {
		return nil, fmt.Errorf("reviewer: intent implementation_status must be %q, got %q", ImplementationStatusIntentOnly, intent.ImplementationStatus)
	}
	if !intent.NoFileWrites {
		return nil, fmt.Errorf("reviewer: intent no_file_writes must be true")
	}

	// 脱敏输入
	sanitizedGoal := security.SanitizeText(intent.Goal)
	sanitizedPlanSummary := security.SanitizeText(intent.PlanSummary)

	// 构建 prompt
	prompt := buildReviewerPrompt(sanitizedGoal, sanitizedPlanSummary, intent.FilesToChange, intent.Changes)

	// 构建 completion request
	req := modelrouter.CompletionRequest{
		Bundle:          bundle,
		MaxOutputTokens: 4096,
		Temperature:     0.3,
	}

	// 更新当前输入为 reviewer prompt
	req.Bundle.CurrentInput = []byte(prompt)

	// 调用 LLM
	resp, err := r.router.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("reviewer: LLM call failed: %w", err)
	}

	// 解析响应
	review, err := ParseReviewerIntentReviewResponse(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("reviewer: %w", err)
	}

	// 验证安全性
	if err := ValidateReviewerReview(review); err != nil {
		return nil, fmt.Errorf("reviewer: validation failed: %w", err)
	}

	// 脱敏字段
	review.Goal = security.SanitizeText(review.Goal)
	review.Summary = security.SanitizeText(review.Summary)

	return review, nil
}

// buildReviewerPrompt 构建 Reviewer prompt
func buildReviewerPrompt(goal string, planSummary string, filesToChange []PatchIntentFile, changes []PatchIntentChange) string {
	filesText := ""
	for _, file := range filesToChange {
		filesText += fmt.Sprintf("- %s (%s, risk: %s)\n  Reason: %s\n", file.Path, file.ChangeType, file.RiskLevel, file.Reason)
	}

	changesText := ""
	for i, change := range changes {
		changesText += fmt.Sprintf("%d. File: %s\n   Description: %s\n   Effect: %s\n   Safety: %s\n", i+1, change.FilePath, change.Description, change.ExpectedEffect, change.SafetyNotes)
	}

	return fmt.Sprintf(`You are a code review agent. Your ONLY job is to review a patch intent.

STRICT CONSTRAINTS:
- You can ONLY review the patch intent
- You CANNOT write code
- You CANNOT generate real diffs
- You CANNOT apply patches
- You CANNOT run commands
- You CANNOT claim to have modified code
- You CANNOT claim to have run tests
- You MUST output structured JSON
- implementation_status MUST be "review_only"
- no_file_writes MUST be true
- no_patch_generated MUST be true

USER GOAL:
%s

PLAN SUMMARY:
%s

FILES TO CHANGE:
%s

CHANGES:
%s

OUTPUT FORMAT (JSON only):
{
  "goal": "the user's goal",
  "review_status": "approved|changes_requested|rejected|needs_clarification",
  "implementation_status": "review_only",
  "summary": "brief summary of the review",
  "approved": true/false,
  "issues": [
    {
      "id": "issue_1",
      "severity": "low|medium|high",
      "file_path": "file.go",
      "description": "what the issue is",
      "recommendation": "how to fix it"
    }
  ],
  "risks": ["potential risk 1"],
  "required_changes": ["change 1"],
  "validation_suggestions": ["run tests"],
  "no_file_writes": true,
  "no_patch_generated": true
}

IMPORTANT:
- implementation_status MUST be "review_only"
- no_file_writes MUST be true
- no_patch_generated MUST be true
- "approved" only means intent review passed, NOT that patch can be applied
- Do NOT include any real diffs
- Do NOT include any file modifications
- Do NOT include any tool calls
- Do NOT include any shell commands
- Output ONLY the JSON, no other text`, goal, planSummary, filesText, changesText)
}

// RunReviewerWithLLM 运行 Reviewer LLM
func RunReviewerWithLLM(ctx context.Context, intent *CoderPatchIntent, reviewerLLM *ReviewerLLM, bundle contextengine.Bundle) (*ReviewerIntentReview, error) {
	// 验证 intent
	if intent.ImplementationStatus != ImplementationStatusIntentOnly {
		return nil, fmt.Errorf("reviewer: intent implementation_status must be %q, got %q", ImplementationStatusIntentOnly, intent.ImplementationStatus)
	}

	// 审查 intent
	review, err := reviewerLLM.ReviewIntent(ctx, intent, bundle)
	if err != nil {
		return nil, err
	}

	return review, nil
}
