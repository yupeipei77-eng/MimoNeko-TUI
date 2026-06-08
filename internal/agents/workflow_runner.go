package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/contextengine"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/modelrouter"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"
)

// WorkflowRunner 运行端到端 dry-run 工作流
type WorkflowRunner struct {
	plannerLLM   *PlannerLLM
	coderLLM     *CoderLLM
	reviewerLLM  *ReviewerLLM
	validatorLLM *ValidatorLLM
}

// NewWorkflowRunner 创建 WorkflowRunner
func NewWorkflowRunner(router modelrouter.ModelRouter) *WorkflowRunner {
	return &WorkflowRunner{
		plannerLLM:   NewPlannerLLM(router),
		coderLLM:     NewCoderLLM(router),
		reviewerLLM:  NewReviewerLLM(router),
		validatorLLM: NewValidatorLLM(router),
	}
}

// RunDryRun 运行端到端 dry-run
func (r *WorkflowRunner) RunDryRun(ctx context.Context, goal string, bundle contextengine.Bundle, provider string, model string) (*AgentDryRunReport, error) {
	// 创建 report
	workflowID := generateWorkflowID()
	runID, _ := generateRunID()

	report := &AgentDryRunReport{
		Goal:             security.SanitizeText(goal),
		WorkflowID:       workflowID,
		RunID:            runID,
		Provider:         provider,
		Model:            model,
		Status:           WorkflowStatusRunning,
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
	}

	// 验证安全约束
	if err := report.Validate(); err != nil {
		report.Status = WorkflowStatusFailed
		report.ErrorMessage = err.Error()
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}

	// Step 1: Planner
	plan, err := r.plannerLLM.GeneratePlan(ctx, goal, bundle)
	if err != nil {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRolePlanner)
		report.ErrorMessage = fmt.Sprintf("planner failed: %v", err)
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}
	report.PlannerPlan = plan

	// 验证 planner 安全约束
	if plan.ImplementationStatus != ImplementationStatusPlanOnly {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRolePlanner)
		report.ErrorMessage = fmt.Sprintf("planner implementation_status must be %q, got %q", ImplementationStatusPlanOnly, plan.ImplementationStatus)
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}

	// Step 2: Coder
	intent, err := r.coderLLM.GeneratePatchIntent(ctx, plan, bundle)
	if err != nil {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleCoder)
		report.ErrorMessage = fmt.Sprintf("coder failed: %v", err)
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}
	report.CoderIntent = intent

	// 验证 coder 安全约束
	if intent.ImplementationStatus != ImplementationStatusIntentOnly {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleCoder)
		report.ErrorMessage = fmt.Sprintf("coder implementation_status must be %q, got %q", ImplementationStatusIntentOnly, intent.ImplementationStatus)
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}
	if !intent.NoFileWrites {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleCoder)
		report.ErrorMessage = "coder no_file_writes must be true"
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}

	// Step 3: Reviewer
	review, err := r.reviewerLLM.ReviewIntent(ctx, intent, bundle)
	if err != nil {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleReviewer)
		report.ErrorMessage = fmt.Sprintf("reviewer failed: %v", err)
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}
	report.ReviewerReview = review

	// 验证 reviewer 安全约束
	if review.ImplementationStatus != ImplementationStatusReviewOnly {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleReviewer)
		report.ErrorMessage = fmt.Sprintf("reviewer implementation_status must be %q, got %q", ImplementationStatusReviewOnly, review.ImplementationStatus)
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}
	if !review.NoFileWrites {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleReviewer)
		report.ErrorMessage = "reviewer no_file_writes must be true"
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}
	if !review.NoPatchGenerated {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleReviewer)
		report.ErrorMessage = "reviewer no_patch_generated must be true"
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}

	// Step 4: Validator
	suggestions, err := r.validatorLLM.GenerateSuggestions(ctx, intent, review, bundle)
	if err != nil {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleValidator)
		report.ErrorMessage = fmt.Sprintf("validator failed: %v", err)
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}
	report.ValidatorSuggestions = suggestions

	// 验证 validator 安全约束
	if suggestions.ImplementationStatus != ImplementationStatusSuggestionsOnly {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleValidator)
		report.ErrorMessage = fmt.Sprintf("validator implementation_status must be %q, got %q", ImplementationStatusSuggestionsOnly, suggestions.ImplementationStatus)
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}
	if !suggestions.NoFileWrites {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleValidator)
		report.ErrorMessage = "validator no_file_writes must be true"
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}
	if !suggestions.NoTestsExecuted {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleValidator)
		report.ErrorMessage = "validator no_tests_executed must be true"
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}
	if !suggestions.NoToolsExecuted {
		report.Status = WorkflowStatusFailed
		report.FailedAtRole = string(AgentRoleValidator)
		report.ErrorMessage = "validator no_tools_executed must be true"
		report.CompletedAt = time.Now().UTC()
		return report, nil
	}

	// 完成
	report.Status = WorkflowStatusCompleted
	report.CompletedAt = time.Now().UTC()

	return report, nil
}

// RunSkeletonDryRun 运行 skeleton dry-run（不调用 LLM）
func (r *WorkflowRunner) RunSkeletonDryRun(goal string) (*AgentDryRunReport, error) {
	workflowID := generateWorkflowID()
	runID, _ := generateRunID()

	report := &AgentDryRunReport{
		Goal:             security.SanitizeText(goal),
		WorkflowID:       workflowID,
		RunID:            runID,
		Provider:         "skeleton",
		Model:            "skeleton",
		Status:           WorkflowStatusCompleted,
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
		CompletedAt:      time.Now().UTC(),
	}

	// Planner skeleton
	plan := &AgentPlan{
		Goal:                 security.SanitizeText(goal),
		Summary:              "Skeleton plan (no LLM)",
		Steps:                []PlanStep{{ID: "step_1", Title: "Analyze", Description: "Analyze goal", RiskLevel: "low"}},
		ImplementationStatus: ImplementationStatusPlanOnly,
	}
	report.PlannerPlan = plan

	// Coder skeleton
	intent := &CoderPatchIntent{
		Goal:                 security.SanitizeText(goal),
		PlanSummary:          plan.Summary,
		ImplementationStatus: ImplementationStatusIntentOnly,
		FilesToChange:        []PatchIntentFile{{Path: "placeholder.go", ChangeType: "edit", Reason: "skeleton", RiskLevel: "low"}},
		Changes:              []PatchIntentChange{{ID: "c1", FilePath: "placeholder.go", Description: "skeleton change"}},
		NoFileWrites:         true,
	}
	report.CoderIntent = intent

	// Reviewer skeleton
	review := &ReviewerIntentReview{
		Goal:                 security.SanitizeText(goal),
		ReviewStatus:         ReviewStatusApproved,
		ImplementationStatus: ImplementationStatusReviewOnly,
		Summary:              "Skeleton review (no LLM)",
		Approved:             true,
		NoFileWrites:         true,
		NoPatchGenerated:     true,
	}
	report.ReviewerReview = review

	// Validator skeleton
	suggestions := &ValidatorSuggestions{
		Goal:                 security.SanitizeText(goal),
		ValidationStatus:     "pending",
		ImplementationStatus: ImplementationStatusSuggestionsOnly,
		Summary:              "Skeleton suggestions (no LLM)",
		Checks:               []ValidationCheck{{ID: "c1", Category: "test", Description: "run tests", Priority: "high"}},
		RecommendedCommands:  []string{"go test ./..."},
		NoFileWrites:         true,
		NoTestsExecuted:      true,
		NoToolsExecuted:      true,
	}
	report.ValidatorSuggestions = suggestions

	return report, nil
}
