// Package review implements the Patch Review pipeline for MimoNeko.
//
// The review pipeline operates on a PatchPreview and produces a PatchReviewReport
// through a deterministic sequence:
//
//	PatchPreview -> RuleBasedReview -> RiskScoring -> TestValidation -> Optional ModelReview -> Recommendation
//
// Safety guarantees:
//   - Review must be based on PatchManager.Preview; it never reads sensitive files.
//   - Review never bypasses PatchManager violation rules.
//   - When violations exist, recommendation must be reject.
//   - When test validation fails, recommendation must be at least request_changes.
//   - When risk is high/critical, recommendation cannot be approve.
//   - Deterministic safety rules always override model suggestions.
package review

import (
	"context"
	"time"

	"github.com/mimoneko/mimoneko/internal/patch"
	"github.com/mimoneko/mimoneko/internal/task"
)

// ReviewRecommendation is the final recommendation of a patch review.
type ReviewRecommendation string

const (
	// RecommendationApprove indicates the patch is safe to apply.
	RecommendationApprove ReviewRecommendation = "approve"
	// RecommendationRequestChanges indicates the patch needs modifications before applying.
	RecommendationRequestChanges ReviewRecommendation = "request_changes"
	// RecommendationReject indicates the patch must not be applied.
	RecommendationReject ReviewRecommendation = "reject"
)

// ReviewFindingSeverity defines the severity levels for review findings.
type ReviewFindingSeverity string

const (
	SeverityInfo     ReviewFindingSeverity = "info"
	SeverityWarning  ReviewFindingSeverity = "warning"
	SeverityError    ReviewFindingSeverity = "error"
	SeverityCritical ReviewFindingSeverity = "critical"
)

// ReviewFindingCategory defines the categories for review findings.
type ReviewFindingCategory string

const (
	CategorySecurity ReviewFindingCategory = "security"
	CategoryTest     ReviewFindingCategory = "test"
	CategoryStyle    ReviewFindingCategory = "style"
	CategoryContract ReviewFindingCategory = "contract"
	CategoryRisk     ReviewFindingCategory = "risk"
	CategoryModel    ReviewFindingCategory = "model"
)

// ReviewFinding describes a single issue discovered during review.
type ReviewFinding struct {
	// Severity is the finding severity: info, warning, error, critical.
	Severity ReviewFindingSeverity `json:"severity"`

	// Category is the finding category: security, test, style, contract, risk, model.
	Category ReviewFindingCategory `json:"category"`

	// Path is the repo-relative file path, if applicable.
	Path string `json:"path,omitempty"`

	// Message describes the finding.
	Message string `json:"message"`
}

// RiskScore represents the risk assessment of a patch.
type RiskScore struct {
	// Level is the risk level: low, medium, high, critical.
	Level string `json:"level"`

	// Score is a numeric risk score from 0 (safest) to 100 (riskiest).
	Score int `json:"score"`

	// Reasons lists the factors that contributed to the risk score.
	Reasons []string `json:"reasons"`
}

// ModelReviewResult holds the result of an optional AI model review.
type ModelReviewResult struct {
	// Provider is the model provider name.
	Provider string `json:"provider"`

	// Model is the model name used.
	Model string `json:"model"`

	// Summary is the model's review summary.
	Summary string `json:"summary"`

	// Findings are issues identified by the model.
	Findings []ReviewFinding `json:"findings,omitempty"`

	// Recommendation is the model's suggested recommendation.
	Recommendation ReviewRecommendation `json:"recommendation"`
}

// PatchReviewRequest is the input to PatchReviewManager.Review.
type PatchReviewRequest struct {
	// RepoRoot is the main repository root, used for PatchManager.Preview and metadata.
	RepoRoot string

	// WorktreeID identifies the worktree to review.
	WorktreeID string

	// WorktreePath is the filesystem path of the isolated worktree.
	// When RunTests is true and TestCommands is non-empty, this must be set
	// so that ValidationRunner executes test_run in the worktree (not the main workspace).
	// If RunTests=true and WorktreePath is empty, Review() returns an error.
	WorktreePath string

	// Contract defines the execution boundary for violation checking.
	Contract task.TaskContract

	// RunTests indicates whether to run test validation.
	RunTests bool

	// TestCommands lists the test command names to run (from tools.yaml config).
	TestCommands []string

	// ForceTests runs TestCommands even when the patch has no file changes.
	// CLI callers set this when the user explicitly passed --test-command.
	ForceTests bool

	// UseModelReview indicates whether to request AI model review.
	UseModelReview bool

	// Model specifies which model to use for review (empty = default).
	Model string

	// MaxDiffBytes caps the diff output size.
	MaxDiffBytes int

	// Metadata carries optional key-value pairs.
	Metadata map[string]string
}

// PatchReviewReport is the output of a patch review.
type PatchReviewReport struct {
	// WorktreeID identifies the reviewed worktree.
	WorktreeID string `json:"worktree_id"`

	// Preview is the patch preview used for the review.
	Preview patch.PatchPreview `json:"preview"`

	// RiskScore is the computed risk assessment.
	RiskScore RiskScore `json:"risk_score"`

	// Findings lists all issues discovered during review.
	Findings []ReviewFinding `json:"findings"`

	// Validation holds the test validation result, if tests were run.
	Validation *ValidationResult `json:"validation,omitempty"`

	// ValidationSkipped is true when default validation was intentionally skipped.
	ValidationSkipped bool `json:"validation_skipped,omitempty"`

	// ValidationSkipReason describes why validation was skipped.
	ValidationSkipReason string `json:"validation_skip_reason,omitempty"`

	// ModelReview holds the AI model review result, if requested.
	ModelReview *ModelReviewResult `json:"model_review,omitempty"`

	// Recommendation is the final deterministic recommendation.
	Recommendation ReviewRecommendation `json:"recommendation"`

	// CreatedAt is when the report was generated.
	CreatedAt time.Time `json:"created_at"`
}

// ValidationResult holds the result of test validation.
// This type is defined in the review package to avoid circular imports,
// but the actual validation logic lives in the validation package.
type ValidationResult struct {
	// Success indicates whether all test commands passed.
	Success bool `json:"success"`

	// Commands lists individual command results.
	Commands []CommandValidationResult `json:"commands,omitempty"`

	// Summary is a human-readable summary of the validation.
	Summary string `json:"summary"`
}

// CommandValidationResult holds the result of a single test command.
type CommandValidationResult struct {
	// CommandName is the name of the test command that was run.
	CommandName string `json:"command_name"`

	// Success indicates whether the command exited with code 0.
	Success bool `json:"success"`

	// ExitCode is the process exit code.
	ExitCode int `json:"exit_code"`

	// Stdout holds the command's standard output (truncated to MaxOutputBytes).
	Stdout string `json:"stdout,omitempty"`

	// Stderr holds the command's standard error (truncated to MaxOutputBytes).
	Stderr string `json:"stderr,omitempty"`

	// DurationMs is the command execution time in milliseconds.
	DurationMs int64 `json:"duration_ms"`

	// Error describes any execution error.
	Error string `json:"error,omitempty"`
}

// PatchReviewManager handles the complete patch review pipeline.
type PatchReviewManager interface {
	// Review executes the full review pipeline on a worktree's patch:
	// PatchPreview -> RuleReview -> RiskScoring -> Optional TestValidation -> Optional ModelReview -> Recommendation
	Review(ctx context.Context, req PatchReviewRequest) (PatchReviewReport, error)
}
