package agents

import (
	"encoding/json"
	"testing"
)

func TestReviewerIntentReviewJSONSerialization(t *testing.T) {
	review := &ReviewerIntentReview{
		Goal:                 "test goal",
		ReviewStatus:         ReviewStatusApproved,
		ImplementationStatus: "review_only",
		Summary:              "test summary",
		Approved:             true,
		Issues: []ReviewIssue{
			{
				ID:             "issue_1",
				Severity:       "low",
				FilePath:       "README.md",
				Description:    "minor formatting",
				Recommendation: "fix formatting",
			},
		},
		Risks:                 []string{"minor risk"},
		RequiredChanges:       []string{},
		ValidationSuggestions: []string{"run tests"},
		NoFileWrites:          true,
		NoPatchGenerated:      true,
	}

	data, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got ReviewerIntentReview
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Goal != review.Goal {
		t.Errorf("Goal = %q, want %q", got.Goal, review.Goal)
	}
	if got.ReviewStatus != review.ReviewStatus {
		t.Errorf("ReviewStatus = %q, want %q", got.ReviewStatus, review.ReviewStatus)
	}
	if got.ImplementationStatus != review.ImplementationStatus {
		t.Errorf("ImplementationStatus = %q, want %q", got.ImplementationStatus, review.ImplementationStatus)
	}
	if got.Approved != review.Approved {
		t.Errorf("Approved = %v, want %v", got.Approved, review.Approved)
	}
	if got.NoFileWrites != review.NoFileWrites {
		t.Errorf("NoFileWrites = %v, want %v", got.NoFileWrites, review.NoFileWrites)
	}
	if got.NoPatchGenerated != review.NoPatchGenerated {
		t.Errorf("NoPatchGenerated = %v, want %v", got.NoPatchGenerated, review.NoPatchGenerated)
	}
}

func TestReviewIssueJSONSerialization(t *testing.T) {
	issue := ReviewIssue{
		ID:             "issue_1",
		Severity:       "medium",
		FilePath:       "README.md",
		Description:    "missing validation",
		Recommendation: "add validation step",
	}

	data, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got ReviewIssue
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ID != issue.ID {
		t.Errorf("ID = %q, want %q", got.ID, issue.ID)
	}
	if got.Severity != issue.Severity {
		t.Errorf("Severity = %q, want %q", got.Severity, issue.Severity)
	}
}

func TestParseReviewerIntentReviewResponseValidJSON(t *testing.T) {
	input := `{
		"goal": "test goal",
		"review_status": "approved",
		"implementation_status": "review_only",
		"summary": "test summary",
		"approved": true,
		"issues": [],
		"risks": [],
		"required_changes": [],
		"validation_suggestions": [],
		"no_file_writes": true,
		"no_patch_generated": true
	}`

	review, err := ParseReviewerIntentReviewResponse(input)
	if err != nil {
		t.Fatalf("ParseReviewerIntentReviewResponse() error = %v", err)
	}

	if review.Goal != "test goal" {
		t.Errorf("Goal = %q, want %q", review.Goal, "test goal")
	}
	if review.ImplementationStatus != "review_only" {
		t.Errorf("ImplementationStatus = %q, want %q", review.ImplementationStatus, "review_only")
	}
	if !review.NoFileWrites {
		t.Errorf("NoFileWrites = false, want true")
	}
	if !review.NoPatchGenerated {
		t.Errorf("NoPatchGenerated = false, want true")
	}
}

func TestParseReviewerIntentReviewResponseMarkdownJSON(t *testing.T) {
	input := "```json\n{\"goal\": \"test\", \"review_status\": \"approved\", \"implementation_status\": \"review_only\", \"summary\": \"sum\", \"approved\": true, \"issues\": [], \"risks\": [], \"required_changes\": [], \"validation_suggestions\": [], \"no_file_writes\": true, \"no_patch_generated\": true}\n```"

	review, err := ParseReviewerIntentReviewResponse(input)
	if err != nil {
		t.Fatalf("ParseReviewerIntentReviewResponse() error = %v", err)
	}

	if review.Goal != "test" {
		t.Errorf("Goal = %q, want %q", review.Goal, "test")
	}
}

func TestParseReviewerIntentReviewResponseInvalidJSON(t *testing.T) {
	input := "this is not json"

	_, err := ParseReviewerIntentReviewResponse(input)
	if err == nil {
		t.Error("ParseReviewerIntentReviewResponse() should return error for invalid JSON")
	}
}

func TestParseReviewerIntentReviewResponseMissingFields(t *testing.T) {
	input := `{"goal": "test"}`

	_, err := ParseReviewerIntentReviewResponse(input)
	if err == nil {
		t.Error("ParseReviewerIntentReviewResponse() should return error for missing fields")
	}
}

func TestParseReviewerIntentReviewResponseForcesSafeValues(t *testing.T) {
	input := `{
		"goal": "test",
		"review_status": "approved",
		"implementation_status": "implemented",
		"summary": "sum",
		"approved": true,
		"issues": [],
		"risks": [],
		"required_changes": [],
		"validation_suggestions": [],
		"no_file_writes": false,
		"no_patch_generated": false
	}`

	review, err := ParseReviewerIntentReviewResponse(input)
	if err != nil {
		t.Fatalf("ParseReviewerIntentReviewResponse() error = %v", err)
	}

	if review.ImplementationStatus != "review_only" {
		t.Errorf("ImplementationStatus = %q, want %q", review.ImplementationStatus, "review_only")
	}
	if !review.NoFileWrites {
		t.Errorf("NoFileWrites = false, want true")
	}
	if !review.NoPatchGenerated {
		t.Errorf("NoPatchGenerated = false, want true")
	}
}

func TestReviewerIntentReviewValidate(t *testing.T) {
	tests := []struct {
		name    string
		review  ReviewerIntentReview
		wantErr bool
	}{
		{
			name: "valid review",
			review: ReviewerIntentReview{
				Goal:                 "test",
				ReviewStatus:         ReviewStatusApproved,
				ImplementationStatus: "review_only",
				Summary:              "sum",
				Approved:             true,
				NoFileWrites:         true,
				NoPatchGenerated:     true,
			},
			wantErr: false,
		},
		{
			name: "missing goal",
			review: ReviewerIntentReview{
				ReviewStatus:         ReviewStatusApproved,
				ImplementationStatus: "review_only",
				Summary:              "sum",
				Approved:             true,
				NoFileWrites:         true,
				NoPatchGenerated:     true,
			},
			wantErr: true,
		},
		{
			name: "wrong implementation status",
			review: ReviewerIntentReview{
				Goal:                 "test",
				ReviewStatus:         ReviewStatusApproved,
				ImplementationStatus: "implemented",
				Summary:              "sum",
				Approved:             true,
				NoFileWrites:         true,
				NoPatchGenerated:     true,
			},
			wantErr: true,
		},
		{
			name: "no_file_writes false",
			review: ReviewerIntentReview{
				Goal:                 "test",
				ReviewStatus:         ReviewStatusApproved,
				ImplementationStatus: "review_only",
				Summary:              "sum",
				Approved:             true,
				NoFileWrites:         false,
				NoPatchGenerated:     true,
			},
			wantErr: true,
		},
		{
			name: "no_patch_generated false",
			review: ReviewerIntentReview{
				Goal:                 "test",
				ReviewStatus:         ReviewStatusApproved,
				ImplementationStatus: "review_only",
				Summary:              "sum",
				Approved:             true,
				NoFileWrites:         true,
				NoPatchGenerated:     false,
			},
			wantErr: true,
		},
		{
			name: "invalid review_status",
			review: ReviewerIntentReview{
				Goal:                 "test",
				ReviewStatus:         "invalid",
				ImplementationStatus: "review_only",
				Summary:              "sum",
				Approved:             true,
				NoFileWrites:         true,
				NoPatchGenerated:     true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.review.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateReviewerReviewDangerousStatus(t *testing.T) {
	dangerousStatuses := []string{"implemented", "applied", "done", "patched", "executed", "modified", "committed", "pushed", "tested"}

	for _, status := range dangerousStatuses {
		review := &ReviewerIntentReview{
			Goal:                 "test",
			ReviewStatus:         ReviewStatusApproved,
			ImplementationStatus: status,
			Summary:              "sum",
			Approved:             true,
			NoFileWrites:         true,
			NoPatchGenerated:     true,
		}

		err := ValidateReviewerReview(review)
		if err == nil {
			t.Errorf("ValidateReviewerReview() should return error for status %q", status)
		}
	}
}

func TestValidateReviewerReviewNoFileWritesFalse(t *testing.T) {
	review := &ReviewerIntentReview{
		Goal:                 "test",
		ReviewStatus:         ReviewStatusApproved,
		ImplementationStatus: "review_only",
		Summary:              "sum",
		Approved:             true,
		NoFileWrites:         false,
		NoPatchGenerated:     true,
	}

	err := ValidateReviewerReview(review)
	if err == nil {
		t.Error("ValidateReviewerReview() should return error when no_file_writes is false")
	}
}

func TestValidateReviewerReviewNoPatchGeneratedFalse(t *testing.T) {
	review := &ReviewerIntentReview{
		Goal:                 "test",
		ReviewStatus:         ReviewStatusApproved,
		ImplementationStatus: "review_only",
		Summary:              "sum",
		Approved:             true,
		NoFileWrites:         true,
		NoPatchGenerated:     false,
	}

	err := ValidateReviewerReview(review)
	if err == nil {
		t.Error("ValidateReviewerReview() should return error when no_patch_generated is false")
	}
}

func TestValidateReviewerReviewDiffPatchContent(t *testing.T) {
	review := &ReviewerIntentReview{
		Goal:                 "test",
		ReviewStatus:         ReviewStatusApproved,
		ImplementationStatus: "review_only",
		Summary:              "sum",
		Approved:             true,
		Issues: []ReviewIssue{
			{
				ID:             "issue_1",
				Severity:       "medium",
				FilePath:       "f.go",
				Description:    "diff --git a/f.go b/f.go",
				Recommendation: "fix this",
			},
		},
		NoFileWrites:     true,
		NoPatchGenerated: true,
	}

	err := ValidateReviewerReview(review)
	if err == nil {
		t.Error("ValidateReviewerReview() should return error for diff patch content")
	}
}

func TestValidateReviewerReviewCommandExecution(t *testing.T) {
	review := &ReviewerIntentReview{
		Goal:                 "test",
		ReviewStatus:         ReviewStatusApproved,
		ImplementationStatus: "review_only",
		Summary:              "sum",
		Approved:             true,
		Issues: []ReviewIssue{
			{
				ID:             "issue_1",
				Severity:       "medium",
				FilePath:       "f.go",
				Description:    "command executed successfully",
				Recommendation: "fix this",
			},
		},
		NoFileWrites:     true,
		NoPatchGenerated: true,
	}

	err := ValidateReviewerReview(review)
	if err == nil {
		t.Error("ValidateReviewerReview() should return error for command execution wording")
	}
}

func TestFormatReviewerReview(t *testing.T) {
	review := &ReviewerIntentReview{
		Goal:                 "test goal",
		ReviewStatus:         ReviewStatusApproved,
		ImplementationStatus: "review_only",
		Summary:              "test summary",
		Approved:             true,
		Issues: []ReviewIssue{
			{
				ID:             "issue_1",
				Severity:       "low",
				FilePath:       "README.md",
				Description:    "minor formatting",
				Recommendation: "fix formatting",
			},
		},
		NoFileWrites:     true,
		NoPatchGenerated: true,
	}

	output := FormatReviewerReview(review)

	if output == "" {
		t.Error("FormatReviewerReview() returned empty string")
	}
	if !containsString(output, "test goal") {
		t.Error("FormatReviewerReview() should contain goal")
	}
	if !containsString(output, "No files were modified") {
		t.Error("FormatReviewerReview() should contain 'No files were modified'")
	}
	if !containsString(output, "intent review only") {
		t.Error("FormatReviewerReview() should contain 'intent review only'")
	}
}

func TestFormatReviewerReviewJSON(t *testing.T) {
	review := &ReviewerIntentReview{
		Goal:                 "test goal",
		ReviewStatus:         ReviewStatusApproved,
		ImplementationStatus: "review_only",
		Summary:              "test summary",
		Approved:             true,
		NoFileWrites:         true,
		NoPatchGenerated:     true,
	}

	jsonStr, err := FormatReviewerReviewJSON(review)
	if err != nil {
		t.Fatalf("FormatReviewerReviewJSON() error = %v", err)
	}

	if jsonStr == "" {
		t.Error("FormatReviewerReviewJSON() returned empty string")
	}

	// 验证是有效的 JSON
	var got ReviewerIntentReview
	if err := json.Unmarshal([]byte(jsonStr), &got); err != nil {
		t.Errorf("FormatReviewerReviewJSON() returned invalid JSON: %v", err)
	}
}
