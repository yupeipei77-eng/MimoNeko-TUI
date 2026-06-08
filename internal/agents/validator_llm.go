package agents

import (
	"context"
	"fmt"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/contextengine"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/modelrouter"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"
)

// ValidatorLLM 调用 LLM 生成验证建议
type ValidatorLLM struct {
	router modelrouter.ModelRouter
}

// NewValidatorLLM 创建 ValidatorLLM
func NewValidatorLLM(router modelrouter.ModelRouter) *ValidatorLLM {
	return &ValidatorLLM{router: router}
}

// GenerateSuggestions 调用 LLM 生成验证建议
func (v *ValidatorLLM) GenerateSuggestions(ctx context.Context, intent *CoderPatchIntent, review *ReviewerIntentReview, bundle contextengine.Bundle) (*ValidatorSuggestions, error) {
	// 验证 intent
	if intent.ImplementationStatus != ImplementationStatusIntentOnly {
		return nil, fmt.Errorf("validator: intent implementation_status must be %q, got %q", ImplementationStatusIntentOnly, intent.ImplementationStatus)
	}

	// 验证 review
	if review.ImplementationStatus != ImplementationStatusReviewOnly {
		return nil, fmt.Errorf("validator: review implementation_status must be %q, got %q", ImplementationStatusReviewOnly, review.ImplementationStatus)
	}

	// 脱敏输入
	sanitizedGoal := security.SanitizeText(intent.Goal)
	sanitizedPlanSummary := security.SanitizeText(intent.PlanSummary)
	sanitizedReviewSummary := security.SanitizeText(review.Summary)

	// 构建 prompt
	prompt := buildValidatorPrompt(sanitizedGoal, sanitizedPlanSummary, sanitizedReviewSummary, intent.FilesToChange, intent.Changes, review.Issues)

	// 构建 completion request
	req := modelrouter.CompletionRequest{
		Bundle:          bundle,
		MaxOutputTokens: 4096,
		Temperature:     0.3,
	}

	// 更新当前输入为 validator prompt
	req.Bundle.CurrentInput = []byte(prompt)

	// 调用 LLM
	resp, err := v.router.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("validator: LLM call failed: %w", err)
	}

	// 解析响应
	suggestions, err := ParseValidatorSuggestionsResponse(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("validator: %w", err)
	}

	// 验证安全性
	if err := ValidateValidatorSuggestions(suggestions); err != nil {
		return nil, fmt.Errorf("validator: validation failed: %w", err)
	}

	// 脱敏字段
	suggestions.Goal = security.SanitizeText(suggestions.Goal)
	suggestions.Summary = security.SanitizeText(suggestions.Summary)

	return suggestions, nil
}

// buildValidatorPrompt 构建 Validator prompt
func buildValidatorPrompt(goal string, planSummary string, reviewSummary string, filesToChange []PatchIntentFile, changes []PatchIntentChange, issues []ReviewIssue) string {
	filesText := ""
	for _, file := range filesToChange {
		filesText += fmt.Sprintf("- %s (%s, risk: %s)\n", file.Path, file.ChangeType, file.RiskLevel)
	}

	changesText := ""
	for i, change := range changes {
		changesText += fmt.Sprintf("%d. File: %s\n   Description: %s\n", i+1, change.FilePath, change.Description)
	}

	issuesText := ""
	for i, issue := range issues {
		issuesText += fmt.Sprintf("%d. [%s] %s\n   File: %s\n   Recommendation: %s\n", i+1, issue.Severity, issue.Description, issue.FilePath, issue.Recommendation)
	}

	return fmt.Sprintf(`You are a validation agent. Your ONLY job is to generate validation suggestions.

STRICT CONSTRAINTS:
- You can ONLY generate validation suggestions
- You CANNOT run tests
- You CANNOT execute commands
- You CANNOT write files
- You CANNOT apply patches
- You CANNOT claim to have tested or verified anything
- You MUST output structured JSON
- implementation_status MUST be "suggestions_only"
- no_file_writes MUST be true
- no_tests_executed MUST be true
- no_tools_executed MUST be true

USER GOAL:
%s

PLAN SUMMARY:
%s

REVIEW SUMMARY:
%s

FILES TO CHANGE:
%s

CHANGES:
%s

REVIEW ISSUES:
%s

OUTPUT FORMAT (JSON only):
{
  "goal": "the user's goal",
  "validation_status": "pending",
  "implementation_status": "suggestions_only",
  "summary": "brief summary of validation suggestions",
  "checks": [
    {
      "id": "check_1",
      "category": "unit_test|integration_test|manual_check|code_review",
      "description": "what to check",
      "expected_signal": "what to expect",
      "priority": "low|medium|high",
      "related_files": ["file.go"]
    }
  ],
  "risks": ["potential risk 1"],
  "recommended_commands": ["go test ./..."],
  "manual_checks": ["check 1"],
  "no_file_writes": true,
  "no_tests_executed": true,
  "no_tools_executed": true
}

IMPORTANT:
- implementation_status MUST be "suggestions_only"
- no_file_writes MUST be true
- no_tests_executed MUST be true
- no_tools_executed MUST be true
- "recommended_commands" are suggestions only, NOT executed
- Do NOT include "test passed" or "tests passed"
- Do NOT include "executed" or "command executed"
- Do NOT include any real test results
- Output ONLY the JSON, no other text`, goal, planSummary, reviewSummary, filesText, changesText, issuesText)
}

// RunValidatorWithLLM 运行 Validator LLM
func RunValidatorWithLLM(ctx context.Context, intent *CoderPatchIntent, review *ReviewerIntentReview, validatorLLM *ValidatorLLM, bundle contextengine.Bundle) (*ValidatorSuggestions, error) {
	// 验证 intent
	if intent.ImplementationStatus != ImplementationStatusIntentOnly {
		return nil, fmt.Errorf("validator: intent implementation_status must be %q, got %q", ImplementationStatusIntentOnly, intent.ImplementationStatus)
	}

	// 验证 review
	if review.ImplementationStatus != ImplementationStatusReviewOnly {
		return nil, fmt.Errorf("validator: review implementation_status must be %q, got %q", ImplementationStatusReviewOnly, review.ImplementationStatus)
	}

	// 生成建议
	suggestions, err := validatorLLM.GenerateSuggestions(ctx, intent, review, bundle)
	if err != nil {
		return nil, err
	}

	return suggestions, nil
}
