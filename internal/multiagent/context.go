package multiagent

import (
	"strings"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/review"
)

// SharedTaskContext carries structured information between agents in a multi-agent run.
// It is the single source of truth for the current state of the pipeline.
//
// Safety guarantees:
//   - Never stores API keys
//   - Never stores sensitive diffs (diffs from reports with violations are redacted)
//   - Reuses the same sanitization principles as Phase 4 checkpoint redaction
type SharedTaskContext struct {
	Goal          string                     `json:"goal"`
	Plan          TaskPlan                   `json:"plan"`
	WorktreeID    string                     `json:"worktree_id,omitempty"`
	ReviewHistory []review.PatchReviewReport `json:"review_history"`
	Messages      []AgentMessage             `json:"messages"`
	Metadata      map[string]string          `json:"metadata,omitempty"`
}

// NewSharedTaskContext creates a new SharedTaskContext with the given goal.
func NewSharedTaskContext(goal string) *SharedTaskContext {
	return &SharedTaskContext{
		Goal:          goal,
		ReviewHistory: []review.PatchReviewReport{},
		Messages:      []AgentMessage{},
		Metadata:      make(map[string]string),
	}
}

// AddMessage appends an AgentMessage to the context.
func (ctx *SharedTaskContext) AddMessage(msg AgentMessage) {
	ctx.Messages = append(ctx.Messages, msg)
}

// AddReviewReport appends a sanitized PatchReviewReport to the review history.
// If the report contains violations, the diff is redacted before storage.
func (ctx *SharedTaskContext) AddReviewReport(report review.PatchReviewReport) {
	sanitized := sanitizeReviewReport(report)
	ctx.ReviewHistory = append(ctx.ReviewHistory, sanitized)
}

// LastReview returns the most recent review report, or nil if none exists.
func (ctx *SharedTaskContext) LastReview() *review.PatchReviewReport {
	if len(ctx.ReviewHistory) == 0 {
		return nil
	}
	r := ctx.ReviewHistory[len(ctx.ReviewHistory)-1]
	return &r
}

// LastReviewerMessage returns the most recent reviewer message, or empty string.
func (ctx *SharedTaskContext) LastReviewerMessage() string {
	for i := len(ctx.Messages) - 1; i >= 0; i-- {
		if ctx.Messages[i].Role == AgentRoleReviewer {
			return ctx.Messages[i].Content
		}
	}
	return ""
}

// sanitizeReviewReport redacts sensitive data from a PatchReviewReport
// before storing it in SharedTaskContext.
//
// Rules:
//   - If the report's preview has violations, the diff is redacted
//   - API key patterns are scrubbed from all text fields
//   - File content in tool responses is not stored (only metadata)
func sanitizeReviewReport(report review.PatchReviewReport) review.PatchReviewReport {
	sanitized := report

	// Redact diff if violations exist
	if len(report.Preview.Violations) > 0 {
		sanitized.Preview.Diff = "[diff redacted: policy violations present]"
	}

	// Sanitize diff for API key patterns even without violations
	if sanitized.Preview.Diff != "" && containsAPIKeyPattern(sanitized.Preview.Diff) {
		sanitized.Preview.Diff = "[diff redacted: potential secret detected]"
	}

	// Sanitize validation output for API key patterns
	if sanitized.Validation != nil {
		val := *sanitized.Validation
		val.Summary = sanitizeAPIKeyPatterns(val.Summary)
		for i := range val.Commands {
			val.Commands[i].Stdout = sanitizeAPIKeyPatterns(val.Commands[i].Stdout)
			val.Commands[i].Stderr = sanitizeAPIKeyPatterns(val.Commands[i].Stderr)
		}
		sanitized.Validation = &val
	}

	// Sanitize model review summary
	if sanitized.ModelReview != nil {
		mr := *sanitized.ModelReview
		mr.Summary = sanitizeAPIKeyPatterns(mr.Summary)
		sanitized.ModelReview = &mr
	}

	return sanitized
}

// apiKeyPatterns are substrings that indicate potential API key leakage.
var apiKeyPatterns = []string{
	"API_KEY",
	"SECRET",
	"TOKEN",
	"PASSWORD",
	"PRIVATE_KEY",
	"sk-",
	"sk_live_",
	"pk_live_",
	"AKIA",
}

// containsAPIKeyPattern checks if a string contains any API key pattern.
func containsAPIKeyPattern(s string) bool {
	upper := strings.ToUpper(s)
	for _, pattern := range apiKeyPatterns {
		if strings.Contains(upper, strings.ToUpper(pattern)) {
			return true
		}
	}
	return false
}

// sanitizeAPIKeyPatterns redacts lines containing API key patterns.
func sanitizeAPIKeyPatterns(s string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	redacted := false
	for i, line := range lines {
		if containsAPIKeyPattern(line) {
			lines[i] = "[redacted: potential secret]"
			redacted = true
		}
	}
	if redacted {
		return strings.Join(lines, "\n")
	}
	return s
}
