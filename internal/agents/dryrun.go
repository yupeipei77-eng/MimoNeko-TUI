package agents

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// WorkflowStatus 枚举
const (
	WorkflowStatusPending   = "pending"
	WorkflowStatusRunning   = "running"
	WorkflowStatusCompleted = "completed"
	WorkflowStatusFailed    = "failed"
)

// AgentDryRunReport 表示端到端 dry-run 的完整报告
type AgentDryRunReport struct {
	Goal                 string                `json:"goal"`
	WorkflowID           string                `json:"workflow_id"`
	RunID                string                `json:"run_id"`
	Provider             string                `json:"provider"`
	Model                string                `json:"model"`
	PlannerPlan          *AgentPlan            `json:"planner_plan,omitempty"`
	CoderIntent          *CoderPatchIntent     `json:"coder_intent,omitempty"`
	ReviewerReview       *ReviewerIntentReview `json:"reviewer_review,omitempty"`
	ValidatorSuggestions *ValidatorSuggestions `json:"validator_suggestions,omitempty"`
	Status               string                `json:"status"`
	FailedAtRole         string                `json:"failed_at_role,omitempty"`
	ErrorMessage         string                `json:"error_message,omitempty"`
	NoFileWrites         bool                  `json:"no_file_writes"`
	NoPatchGenerated     bool                  `json:"no_patch_generated"`
	NoToolsExecuted      bool                  `json:"no_tools_executed"`
	NoTestsExecuted      bool                  `json:"no_tests_executed"`
	CreatedAt            time.Time             `json:"created_at"`
	CompletedAt          time.Time             `json:"completed_at,omitempty"`
}

// Validate 检查 report 是否满足安全约束
func (r *AgentDryRunReport) Validate() error {
	if !r.NoFileWrites {
		return fmt.Errorf("dryrun: no_file_writes must be true")
	}
	if !r.NoPatchGenerated {
		return fmt.Errorf("dryrun: no_patch_generated must be true")
	}
	if !r.NoToolsExecuted {
		return fmt.Errorf("dryrun: no_tools_executed must be true")
	}
	if !r.NoTestsExecuted {
		return fmt.Errorf("dryrun: no_tests_executed must be true")
	}
	return nil
}

// FormatDryRunReport 格式化 AgentDryRunReport 用于显示
func FormatDryRunReport(report *AgentDryRunReport) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "╔══════════════════════════════════════════════════════════════╗\n")
	fmt.Fprintf(&buf, "║              MioNeko Agent Dry Run Report                   ║\n")
	fmt.Fprintf(&buf, "╚══════════════════════════════════════════════════════════════╝\n")
	fmt.Fprintf(&buf, "\n")

	fmt.Fprintf(&buf, "Goal: %s\n", report.Goal)
	fmt.Fprintf(&buf, "Workflow ID: %s\n", report.WorkflowID)
	fmt.Fprintf(&buf, "Run ID: %s\n", report.RunID)
	fmt.Fprintf(&buf, "Provider: %s\n", report.Provider)
	fmt.Fprintf(&buf, "Model: %s\n", report.Model)
	fmt.Fprintf(&buf, "Status: %s\n", report.Status)
	fmt.Fprintf(&buf, "\n")

	// Planner
	fmt.Fprintf(&buf, "━━━ Planner ━━━\n")
	if report.PlannerPlan != nil {
		fmt.Fprintf(&buf, "  Status: %s\n", report.PlannerPlan.ImplementationStatus)
		fmt.Fprintf(&buf, "  Summary: %s\n", report.PlannerPlan.Summary)
		fmt.Fprintf(&buf, "  Steps: %d\n", len(report.PlannerPlan.Steps))
	} else {
		fmt.Fprintf(&buf, "  (not executed)\n")
	}
	fmt.Fprintf(&buf, "\n")

	// Coder
	fmt.Fprintf(&buf, "━━━ Coder ━━━\n")
	if report.CoderIntent != nil {
		fmt.Fprintf(&buf, "  Status: %s\n", report.CoderIntent.ImplementationStatus)
		fmt.Fprintf(&buf, "  Files: %d\n", len(report.CoderIntent.FilesToChange))
		fmt.Fprintf(&buf, "  Changes: %d\n", len(report.CoderIntent.Changes))
	} else {
		fmt.Fprintf(&buf, "  (not executed)\n")
	}
	fmt.Fprintf(&buf, "\n")

	// Reviewer
	fmt.Fprintf(&buf, "━━━ Reviewer ━━━\n")
	if report.ReviewerReview != nil {
		fmt.Fprintf(&buf, "  Status: %s\n", report.ReviewerReview.ImplementationStatus)
		fmt.Fprintf(&buf, "  Review: %s\n", report.ReviewerReview.ReviewStatus)
		fmt.Fprintf(&buf, "  Approved: %v\n", report.ReviewerReview.Approved)
		fmt.Fprintf(&buf, "  Issues: %d\n", len(report.ReviewerReview.Issues))
	} else {
		fmt.Fprintf(&buf, "  (not executed)\n")
	}
	fmt.Fprintf(&buf, "\n")

	// Validator
	fmt.Fprintf(&buf, "━━━ Validator ━━━\n")
	if report.ValidatorSuggestions != nil {
		fmt.Fprintf(&buf, "  Status: %s\n", report.ValidatorSuggestions.ImplementationStatus)
		fmt.Fprintf(&buf, "  Checks: %d\n", len(report.ValidatorSuggestions.Checks))
		fmt.Fprintf(&buf, "  Commands: %d\n", len(report.ValidatorSuggestions.RecommendedCommands))
	} else {
		fmt.Fprintf(&buf, "  (not executed)\n")
	}
	fmt.Fprintf(&buf, "\n")

	// Error
	if report.ErrorMessage != "" {
		fmt.Fprintf(&buf, "━━━ Error ━━━\n")
		fmt.Fprintf(&buf, "  Failed at: %s\n", report.FailedAtRole)
		fmt.Fprintf(&buf, "  Error: %s\n", report.ErrorMessage)
		fmt.Fprintf(&buf, "\n")
	}

	// Safety
	fmt.Fprintf(&buf, "━━━ Safety ━━━\n")
	fmt.Fprintf(&buf, "  No files were modified: %v\n", report.NoFileWrites)
	fmt.Fprintf(&buf, "  No patch was generated: %v\n", report.NoPatchGenerated)
	fmt.Fprintf(&buf, "  No tools were executed: %v\n", report.NoToolsExecuted)
	fmt.Fprintf(&buf, "  No tests were executed: %v\n", report.NoTestsExecuted)
	fmt.Fprintf(&buf, "\n")

	fmt.Fprintf(&buf, "This was an end-to-end dry run.\n")

	return buf.String()
}

// FormatDryRunReportJSON 格式化 AgentDryRunReport 为 JSON
func FormatDryRunReportJSON(report *AgentDryRunReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("dryrun: marshal JSON: %w", err)
	}
	return string(data), nil
}
