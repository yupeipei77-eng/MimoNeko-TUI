package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/reasonforge/reasonforge/internal/contextengine"
	"github.com/reasonforge/reasonforge/internal/modelrouter"
	"github.com/reasonforge/reasonforge/internal/patch"
)

// DefaultModelReviewer implements ModelReviewer using ModelRouter.
type DefaultModelReviewer struct {
	router modelrouter.ModelRouter
}

// NewDefaultModelReviewer creates a new DefaultModelReviewer.
func NewDefaultModelReviewer(router modelrouter.ModelRouter) *DefaultModelReviewer {
	return &DefaultModelReviewer{router: router}
}

// Review sends a review prompt to the model and parses the response.
//
// Safety guarantees:
//   - If PatchPreview.Violations is non-empty, the raw diff is NOT sent to the model.
//   - If PatchPreview.Diff is a redacted marker, the original diff is NOT sent.
//   - Only safe diff content (when no violations) is included.
//   - The prompt contains files changed, additions/deletions, risk summary,
//     and violations summary. Safe diff is only sent when there are no violations.
func (m *DefaultModelReviewer) Review(ctx context.Context, req ModelReviewRequest) (ModelReviewResult, error) {
	if m.router == nil {
		return ModelReviewResult{}, fmt.Errorf("model reviewer: no model router configured")
	}

	// Build the review prompt
	prompt := buildReviewPrompt(req)

	// Create a minimal context bundle with just the review prompt
	bundle := contextengine.Bundle{
		CurrentInput: []byte(prompt),
		Volatile:     contextengine.VolatileContext{},
	}

	model := req.Model
	if model == "" {
		model = "default"
	}

	resp, err := m.router.Complete(ctx, modelrouter.CompletionRequest{
		Model:    model,
		Bundle:   bundle,
		Metadata: map[string]string{"source": "patch_review"},
	})
	if err != nil {
		return ModelReviewResult{}, fmt.Errorf("model reviewer: complete: %w", err)
	}

	// Parse the model response
	result := parseModelResponse(resp, req.Preview)
	return result, nil
}

// buildReviewPrompt constructs the review prompt for the model.
// It never includes raw diff content when violations exist.
func buildReviewPrompt(req ModelReviewRequest) string {
	var b strings.Builder

	b.WriteString("You are a code review assistant. Review the following patch and provide your assessment.\n\n")

	// Files changed summary
	b.WriteString("## Files Changed\n")
	for _, f := range req.Preview.FilesChanged {
		b.WriteString(fmt.Sprintf("- %s (%s, +%d/-%d)\n", f.Path, f.Status, f.Additions, f.Deletions))
	}

	// Summary
	s := req.Preview.Summary
	b.WriteString(fmt.Sprintf("\n## Summary\nFiles: %d, Additions: %d, Deletions: %d, Has Binary: %v\n",
		s.FilesChanged, s.Additions, s.Deletions, s.HasBinary))

	// Violations summary
	if len(req.Preview.Violations) > 0 {
		b.WriteString("\n## Violations\n")
		for _, v := range req.Preview.Violations {
			b.WriteString(fmt.Sprintf("- %s: %s\n", v.Path, v.Reason))
		}
		b.WriteString("\nNOTE: Diff content is not provided due to policy violations.\n")
	} else {
		// Rule findings summary
		if len(req.RuleFindings) > 0 {
			b.WriteString("\n## Rule-Based Findings\n")
			for _, f := range req.RuleFindings {
				b.WriteString(fmt.Sprintf("- [%s/%s] %s\n", f.Severity, f.Category, f.Message))
			}
		}

		// Safe diff - only when no violations
		if req.Preview.Diff != "" && req.Preview.Diff != "[diff redacted due to policy violations]" {
			b.WriteString("\n## Diff\n```\n")
			// Limit diff size to prevent token overflow
			diff := req.Preview.Diff
			maxDiffLen := 32768
			if len(diff) > maxDiffLen {
				diff = diff[:maxDiffLen] + "\n... (truncated)"
			}
			b.WriteString(diff)
			b.WriteString("\n```\n")
		}
	}

	// Request structured output
	b.WriteString("\n## Instructions\n")
	b.WriteString("Provide your review in the following JSON format:\n")
	b.WriteString("```json\n")
	b.WriteString(`{"summary": "brief summary", "findings": [{"severity": "info|warning|error|critical", "category": "security|test|style|contract|risk|model", "path": "", "message": "description"}], "recommendation": "approve|request_changes|reject"}`)
	b.WriteString("\n```\n")

	return b.String()
}

// parseModelResponse extracts the review result from the model's text output.
func parseModelResponse(resp modelrouter.CompletionResponse, preview patch.PatchPreview) ModelReviewResult {
	result := ModelReviewResult{
		Provider:       resp.Provider,
		Model:          resp.Model,
		Summary:        "model review completed",
		Recommendation: RecommendationApprove,
	}

	text := strings.TrimSpace(resp.Text)

	// Try to extract JSON from the response
	jsonStr := extractJSON(text)
	if jsonStr == "" {
		result.Summary = truncateString(text, 512)
		return result
	}

	var parsed struct {
		Summary  string `json:"summary"`
		Findings []struct {
			Severity string `json:"severity"`
			Category string `json:"category"`
			Path     string `json:"path"`
			Message  string `json:"message"`
		} `json:"findings"`
		Recommendation string `json:"recommendation"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		result.Summary = truncateString(text, 512)
		return result
	}

	result.Summary = truncateString(parsed.Summary, 512)

	// Parse recommendation
	switch strings.ToLower(parsed.Recommendation) {
	case "reject":
		result.Recommendation = RecommendationReject
	case "request_changes":
		result.Recommendation = RecommendationRequestChanges
	default:
		result.Recommendation = RecommendationApprove
	}

	// Parse findings
	for _, f := range parsed.Findings {
		severity := parseSeverity(f.Severity)
		category := parseCategory(f.Category)
		result.Findings = append(result.Findings, ReviewFinding{
			Severity: severity,
			Category: category,
			Path:     f.Path,
			Message:  truncateString(f.Message, 256),
		})
	}

	return result
}

// extractJSON tries to extract a JSON object from the model response.
func extractJSON(text string) string {
	// Look for JSON block in markdown code fences
	if idx := strings.Index(text, "```json"); idx >= 0 {
		start := idx + 7
		end := strings.Index(text[start:], "```")
		if end >= 0 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Look for raw JSON object
	start := strings.Index(text, "{")
	if start < 0 {
		return ""
	}
	end := strings.LastIndex(text, "}")
	if end <= start {
		return ""
	}
	return text[start : end+1]
}

func parseSeverity(s string) ReviewFindingSeverity {
	switch strings.ToLower(s) {
	case "critical":
		return SeverityCritical
	case "error":
		return SeverityError
	case "warning":
		return SeverityWarning
	default:
		return SeverityInfo
	}
}

func parseCategory(s string) ReviewFindingCategory {
	switch strings.ToLower(s) {
	case "security":
		return CategorySecurity
	case "test":
		return CategoryTest
	case "style":
		return CategoryStyle
	case "contract":
		return CategoryContract
	case "risk":
		return CategoryRisk
	case "model":
		return CategoryModel
	default:
		return CategoryRisk
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Ensure DefaultModelReviewer implements ModelReviewer.
var _ ModelReviewer = (*DefaultModelReviewer)(nil)
