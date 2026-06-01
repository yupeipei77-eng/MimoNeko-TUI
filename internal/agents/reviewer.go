package agents

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ReviewStatus 枚举
const (
	ReviewStatusApproved           = "approved"
	ReviewStatusChangesRequested   = "changes_requested"
	ReviewStatusRejected           = "rejected"
	ReviewStatusNeedsClarification = "needs_clarification"
)

// ImplementationStatusReviewOnly 是 Reviewer 的唯一允许状态
const ImplementationStatusReviewOnly = "review_only"

// DangerousReviewStatuses 是 Reviewer 必须拒绝的状态
var DangerousReviewStatuses = []string{
	"implemented",
	"applied",
	"done",
	"patched",
	"executed",
	"modified",
	"committed",
	"pushed",
	"tested",
}

// ReviewerIntentReview 表示 Reviewer 对 Patch Intent 的审查结果
type ReviewerIntentReview struct {
	Goal                  string        `json:"goal"`
	ReviewStatus          string        `json:"review_status"`
	ImplementationStatus  string        `json:"implementation_status"`
	Summary               string        `json:"summary"`
	Approved              bool          `json:"approved"`
	Issues                []ReviewIssue `json:"issues"`
	Risks                 []string      `json:"risks"`
	RequiredChanges       []string      `json:"required_changes"`
	ValidationSuggestions []string      `json:"validation_suggestions"`
	NoFileWrites          bool          `json:"no_file_writes"`
	NoPatchGenerated      bool          `json:"no_patch_generated"`
}

// ReviewIssue 表示一个审查问题
type ReviewIssue struct {
	ID             string `json:"id"`
	Severity       string `json:"severity"`
	FilePath       string `json:"file_path"`
	Description    string `json:"description"`
	Recommendation string `json:"recommendation"`
}

// Validate 检查 review 是否有所有必需字段且安全
func (r *ReviewerIntentReview) Validate() error {
	if r.Goal == "" {
		return fmt.Errorf("reviewer: goal is required")
	}
	if r.ReviewStatus == "" {
		return fmt.Errorf("reviewer: review_status is required")
	}
	if r.Summary == "" {
		return fmt.Errorf("reviewer: summary is required")
	}
	if r.ImplementationStatus != ImplementationStatusReviewOnly {
		return fmt.Errorf("reviewer: implementation_status must be %q, got %q", ImplementationStatusReviewOnly, r.ImplementationStatus)
	}
	if !r.NoFileWrites {
		return fmt.Errorf("reviewer: no_file_writes must be true")
	}
	if !r.NoPatchGenerated {
		return fmt.Errorf("reviewer: no_patch_generated must be true")
	}

	// Validate review_status
	validStatuses := []string{ReviewStatusApproved, ReviewStatusChangesRequested, ReviewStatusRejected, ReviewStatusNeedsClarification}
	valid := false
	for _, s := range validStatuses {
		if r.ReviewStatus == s {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("reviewer: review_status must be one of %v, got %q", validStatuses, r.ReviewStatus)
	}

	return nil
}

// ParseReviewerIntentReviewResponse 解析 LLM 响应为 ReviewerIntentReview
func ParseReviewerIntentReviewResponse(text string) (*ReviewerIntentReview, error) {
	// 尝试从 markdown code block 提取 JSON
	jsonStr := extractJSON(text)

	// 解析 JSON
	var review ReviewerIntentReview
	if err := json.Unmarshal([]byte(jsonStr), &review); err != nil {
		return nil, fmt.Errorf("reviewer: invalid JSON: %w", err)
	}

	// 强制安全值
	review.ImplementationStatus = ImplementationStatusReviewOnly
	review.NoFileWrites = true
	review.NoPatchGenerated = true

	// 验证
	if err := review.Validate(); err != nil {
		return nil, err
	}

	return &review, nil
}

// ValidateReviewerReview 验证 ReviewerIntentReview 是否安全
func ValidateReviewerReview(review *ReviewerIntentReview) error {
	// 检查 implementation_status
	for _, dangerous := range DangerousReviewStatuses {
		if strings.EqualFold(review.ImplementationStatus, dangerous) {
			return fmt.Errorf("reviewer: dangerous implementation_status %q detected, must be %q", review.ImplementationStatus, ImplementationStatusReviewOnly)
		}
	}

	// 检查 no_file_writes
	if !review.NoFileWrites {
		return fmt.Errorf("reviewer: no_file_writes must be true")
	}

	// 检查 no_patch_generated
	if !review.NoPatchGenerated {
		return fmt.Errorf("reviewer: no_patch_generated must be true")
	}

	// 检查 issues 中的危险内容
	for _, issue := range review.Issues {
		if err := validateReviewIssue(issue); err != nil {
			return err
		}
	}

	return nil
}

// validateReviewIssue 检查 review issue 不包含危险模式
func validateReviewIssue(issue ReviewIssue) error {
	content := strings.ToLower(issue.Description + " " + issue.Recommendation)
	for _, pattern := range DangerousContentPatterns {
		if strings.Contains(content, strings.ToLower(pattern)) {
			return fmt.Errorf("reviewer: dangerous content pattern %q detected in issue %q", pattern, issue.ID)
		}
	}
	return nil
}

// FormatReviewerReview 格式化 ReviewerIntentReview 用于显示
func FormatReviewerReview(review *ReviewerIntentReview) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "Reviewer Intent Review:\n")
	fmt.Fprintf(&buf, "  Goal: %s\n", review.Goal)
	fmt.Fprintf(&buf, "  Status: %s\n", review.ImplementationStatus)
	fmt.Fprintf(&buf, "  Review: %s\n", review.ReviewStatus)
	fmt.Fprintf(&buf, "  Approved: %v\n", review.Approved)
	fmt.Fprintf(&buf, "\n")

	fmt.Fprintf(&buf, "Summary:\n")
	fmt.Fprintf(&buf, "  %s\n", review.Summary)

	if len(review.Issues) > 0 {
		fmt.Fprintf(&buf, "\nIssues:\n")
		for i, issue := range review.Issues {
			fmt.Fprintf(&buf, "  %d. severity: %s\n", i+1, issue.Severity)
			fmt.Fprintf(&buf, "     file: %s\n", issue.FilePath)
			fmt.Fprintf(&buf, "     description: %s\n", issue.Description)
			fmt.Fprintf(&buf, "     recommendation: %s\n", issue.Recommendation)
		}
	}

	if len(review.Risks) > 0 {
		fmt.Fprintf(&buf, "\nRisks:\n")
		for _, risk := range review.Risks {
			fmt.Fprintf(&buf, "  - %s\n", risk)
		}
	}

	if len(review.RequiredChanges) > 0 {
		fmt.Fprintf(&buf, "\nRequired Changes:\n")
		for _, change := range review.RequiredChanges {
			fmt.Fprintf(&buf, "  - %s\n", change)
		}
	}

	if len(review.ValidationSuggestions) > 0 {
		fmt.Fprintf(&buf, "\nValidation Suggestions:\n")
		for _, suggestion := range review.ValidationSuggestions {
			fmt.Fprintf(&buf, "  - %s\n", suggestion)
		}
	}

	fmt.Fprintf(&buf, "\nNo files were modified.\n")
	fmt.Fprintf(&buf, "No patch was generated.\n")
	fmt.Fprintf(&buf, "No tools were executed.\n")
	fmt.Fprintf(&buf, "This is an intent review only.\n")

	return buf.String()
}

// FormatReviewerReviewJSON 格式化 ReviewerIntentReview 为 JSON
func FormatReviewerReviewJSON(review *ReviewerIntentReview) (string, error) {
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return "", fmt.Errorf("reviewer: marshal JSON: %w", err)
	}
	return string(data), nil
}
