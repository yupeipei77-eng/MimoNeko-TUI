package review

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/reasonforge/reasonforge/internal/modelrouter"
	"github.com/reasonforge/reasonforge/internal/patch"
)

func TestModelReviewer_UseModelReviewFalse(t *testing.T) {
	// When UseModelReview is false, ModelRouter should not be called
	// This is tested at the manager level in manager_test.go
	// Here we test that DefaultModelReviewer works when called directly
	mockProvider := modelrouter.NewMockProvider("mock")
	router := modelrouter.NewDefaultModelRouter(
		map[string]modelrouter.Provider{"mock": mockProvider},
		nil,
		"mock-model",
		nil,
	)

	reviewer := NewDefaultModelReviewer(router)
	_ = reviewer // Verify it can be created without error
}

func TestModelReviewer_WithViolations_NoDiffSent(t *testing.T) {
	// When violations exist, diff should NOT be sent to the model
	mockProvider := modelrouter.NewMockProvider("mock").WithText(`{"summary": "has violations", "findings": [], "recommendation": "reject"}`)
	router := modelrouter.NewDefaultModelRouter(
		map[string]modelrouter.Provider{"mock": mockProvider},
		nil,
		"mock-model",
		nil,
	)

	reviewer := NewDefaultModelReviewer(router)

	preview := patch.PatchPreview{
		WorktreeID: "wt_test",
		FilesChanged: []patch.FileChange{
			{Path: ".env", Status: "modified", Additions: 1},
		},
		Diff: "[diff redacted due to policy violations]",
		Violations: []patch.PatchViolation{
			{Path: ".env", Reason: "sensitive file"},
		},
		Summary:     patch.DiffSummary{FilesChanged: 1, Additions: 1},
		GeneratedAt: time.Now().UTC(),
	}

	result, err := reviewer.Review(context.Background(), ModelReviewRequest{
		WorktreeID:   "wt_test",
		Preview:      preview,
		RuleFindings: []ReviewFinding{},
		Model:        "mock-model",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The model should have been called
	if mockProvider.CallCount() == 0 {
		t.Error("expected model to be called")
	}

	// The prompt should NOT contain the diff content
	lastReq := mockProvider.LastRequest()
	if lastReq != nil {
		prompt := string(lastReq.Bundle.CurrentInput)
		if strings.Contains(prompt, "[diff redacted due to policy violations]") {
			// The prompt should contain "Violations" but not the raw diff
			if strings.Contains(prompt, "## Diff") {
				t.Error("prompt should not contain Diff section when violations exist")
			}
		}
	}

	_ = result
}

func TestModelReviewer_NoViolations_SafeDiffSent(t *testing.T) {
	// When no violations exist, safe diff can be sent
	mockProvider := modelrouter.NewMockProvider("mock").WithText(`{"summary": "looks good", "findings": [], "recommendation": "approve"}`)
	router := modelrouter.NewDefaultModelRouter(
		map[string]modelrouter.Provider{"mock": mockProvider},
		nil,
		"mock-model",
		nil,
	)

	reviewer := NewDefaultModelReviewer(router)

	preview := patch.PatchPreview{
		WorktreeID: "wt_test",
		FilesChanged: []patch.FileChange{
			{Path: "main.go", Status: "modified", Additions: 5, Deletions: 2},
		},
		Diff:        "diff --git a/main.go b/main.go\n+new line",
		Summary:     patch.DiffSummary{FilesChanged: 1, Additions: 5, Deletions: 2},
		GeneratedAt: time.Now().UTC(),
	}

	result, err := reviewer.Review(context.Background(), ModelReviewRequest{
		WorktreeID:   "wt_test",
		Preview:      preview,
		RuleFindings: []ReviewFinding{},
		Model:        "mock-model",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The prompt SHOULD contain the diff section
	lastReq := mockProvider.LastRequest()
	if lastReq != nil {
		prompt := string(lastReq.Bundle.CurrentInput)
		if !strings.Contains(prompt, "## Diff") {
			t.Error("prompt should contain Diff section when no violations exist")
		}
	}

	if result.Recommendation != RecommendationApprove {
		t.Errorf("expected approve, got %s", result.Recommendation)
	}
}

func TestModelReviewer_ModelFailure_WarningFinding(t *testing.T) {
	// When model review fails, it should produce a warning finding
	// This is tested at the manager level
	// Here we verify the reviewer returns an error
	mockProvider := modelrouter.NewMockProvider("mock").WithError(fmt.Errorf("network error"))
	router := modelrouter.NewDefaultModelRouter(
		map[string]modelrouter.Provider{"mock": mockProvider},
		nil,
		"mock-model",
		nil,
	)

	reviewer := NewDefaultModelReviewer(router)

	preview := patch.PatchPreview{
		WorktreeID:  "wt_test",
		Summary:     patch.DiffSummary{FilesChanged: 1},
		GeneratedAt: time.Now().UTC(),
	}

	_, err := reviewer.Review(context.Background(), ModelReviewRequest{
		WorktreeID:   "wt_test",
		Preview:      preview,
		RuleFindings: []ReviewFinding{},
		Model:        "mock-model",
	})
	if err == nil {
		t.Error("expected error when model fails")
	}
}

func TestModelReviewer_ParseApproveResponse(t *testing.T) {
	mockProvider := modelrouter.NewMockProvider("mock").WithText(`{"summary": "code looks fine", "findings": [], "recommendation": "approve"}`)
	router := modelrouter.NewDefaultModelRouter(
		map[string]modelrouter.Provider{"mock": mockProvider},
		nil,
		"mock-model",
		nil,
	)

	reviewer := NewDefaultModelReviewer(router)

	preview := patch.PatchPreview{
		WorktreeID:  "wt_test",
		Summary:     patch.DiffSummary{FilesChanged: 1},
		GeneratedAt: time.Now().UTC(),
	}

	result, err := reviewer.Review(context.Background(), ModelReviewRequest{
		WorktreeID:   "wt_test",
		Preview:      preview,
		RuleFindings: []ReviewFinding{},
		Model:        "mock-model",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Recommendation != RecommendationApprove {
		t.Errorf("expected approve, got %s", result.Recommendation)
	}
	if result.Provider != "mock" {
		t.Errorf("expected provider mock, got %s", result.Provider)
	}
}

func TestModelReviewer_ParseRejectResponse(t *testing.T) {
	mockProvider := modelrouter.NewMockProvider("mock").WithText(`{"summary": "security issue found", "findings": [{"severity": "critical", "category": "security", "path": ".env", "message": "exposed secrets"}], "recommendation": "reject"}`)
	router := modelrouter.NewDefaultModelRouter(
		map[string]modelrouter.Provider{"mock": mockProvider},
		nil,
		"mock-model",
		nil,
	)

	reviewer := NewDefaultModelReviewer(router)

	preview := patch.PatchPreview{
		WorktreeID:  "wt_test",
		Summary:     patch.DiffSummary{FilesChanged: 1},
		GeneratedAt: time.Now().UTC(),
	}

	result, err := reviewer.Review(context.Background(), ModelReviewRequest{
		WorktreeID:   "wt_test",
		Preview:      preview,
		RuleFindings: []ReviewFinding{},
		Model:        "mock-model",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Recommendation != RecommendationReject {
		t.Errorf("expected reject, got %s", result.Recommendation)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}
	if result.Findings[0].Severity != SeverityCritical {
		t.Errorf("expected critical severity, got %s", result.Findings[0].Severity)
	}
}

func TestModelReviewer_ParseMarkdownJSON(t *testing.T) {
	response := `Here is my review:

` + "```json" + `
{"summary": "needs work", "findings": [{"severity": "warning", "category": "style", "message": "missing docs"}], "recommendation": "request_changes"}
` + "```" + `

That's all.`

	mockProvider := modelrouter.NewMockProvider("mock").WithText(response)
	router := modelrouter.NewDefaultModelRouter(
		map[string]modelrouter.Provider{"mock": mockProvider},
		nil,
		"mock-model",
		nil,
	)

	reviewer := NewDefaultModelReviewer(router)

	preview := patch.PatchPreview{
		WorktreeID:  "wt_test",
		Summary:     patch.DiffSummary{FilesChanged: 1},
		GeneratedAt: time.Now().UTC(),
	}

	result, err := reviewer.Review(context.Background(), ModelReviewRequest{
		WorktreeID:   "wt_test",
		Preview:      preview,
		RuleFindings: []ReviewFinding{},
		Model:        "mock-model",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Recommendation != RecommendationRequestChanges {
		t.Errorf("expected request_changes, got %s", result.Recommendation)
	}
}

func TestModelReviewer_NoRouter(t *testing.T) {
	reviewer := NewDefaultModelReviewer(nil)

	preview := patch.PatchPreview{
		WorktreeID:  "wt_test",
		GeneratedAt: time.Now().UTC(),
	}

	_, err := reviewer.Review(context.Background(), ModelReviewRequest{
		Preview: preview,
	})
	if err == nil {
		t.Error("expected error when no router configured")
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantJSON bool
	}{
		{
			name:     "markdown json block",
			input:    "```json\n{\"key\": \"value\"}\n```",
			wantJSON: true,
		},
		{
			name:     "raw json",
			input:    `{"key": "value"}`,
			wantJSON: true,
		},
		{
			name:     "no json",
			input:    "no json here",
			wantJSON: false,
		},
		{
			name:     "embedded json",
			input:    "result: {\"a\": 1} done",
			wantJSON: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if tt.wantJSON && result == "" {
				t.Error("expected JSON to be extracted")
			}
			if !tt.wantJSON && result != "" {
				t.Errorf("did not expect JSON, got: %s", result)
			}
		})
	}
}

func TestBuildReviewPrompt_NoViolations(t *testing.T) {
	preview := patch.PatchPreview{
		WorktreeID: "wt_test",
		FilesChanged: []patch.FileChange{
			{Path: "main.go", Status: "modified", Additions: 5, Deletions: 2},
		},
		Diff:    "safe diff content",
		Summary: patch.DiffSummary{FilesChanged: 1, Additions: 5, Deletions: 2},
	}

	prompt := buildReviewPrompt(ModelReviewRequest{
		Preview:      preview,
		RuleFindings: []ReviewFinding{},
	})

	if !strings.Contains(prompt, "## Diff") {
		t.Error("prompt should contain Diff section when no violations")
	}
	if !strings.Contains(prompt, "safe diff content") {
		t.Error("prompt should contain safe diff content")
	}
}

func TestBuildReviewPrompt_WithViolations(t *testing.T) {
	preview := patch.PatchPreview{
		WorktreeID: "wt_test",
		FilesChanged: []patch.FileChange{
			{Path: ".env", Status: "modified", Additions: 1},
		},
		Diff: "[diff redacted due to policy violations]",
		Violations: []patch.PatchViolation{
			{Path: ".env", Reason: "sensitive file"},
		},
		Summary: patch.DiffSummary{FilesChanged: 1, Additions: 1},
	}

	prompt := buildReviewPrompt(ModelReviewRequest{
		Preview:      preview,
		RuleFindings: []ReviewFinding{},
	})

	if strings.Contains(prompt, "## Diff") {
		t.Error("prompt should NOT contain Diff section when violations exist")
	}
	if !strings.Contains(prompt, "## Violations") {
		t.Error("prompt should contain Violations section")
	}
	if !strings.Contains(prompt, "Diff content is not provided") {
		t.Error("prompt should note diff is not provided")
	}
}
