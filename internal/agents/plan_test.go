package agents

import (
	"encoding/json"
	"testing"
)

func TestAgentPlanJSONSerialization(t *testing.T) {
	plan := &AgentPlan{
		Goal:    "test goal",
		Summary: "test summary",
		Steps: []PlanStep{
			{
				ID:             "step_1",
				Title:          "test step",
				Description:    "test description",
				RiskLevel:      "low",
				ExpectedFiles:  []string{"file.go"},
				ValidationHint: "run tests",
			},
		},
		Risks:                 []string{"test risk"},
		FilesMaybeAffected:    []string{"file.go"},
		ValidationSuggestions: []string{"run tests"},
		ImplementationStatus:  "plan_only",
		PrefixFingerprint:     "abc123",
		ContextBytes:          1024,
	}

	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got AgentPlan
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Goal != plan.Goal {
		t.Errorf("Goal = %q, want %q", got.Goal, plan.Goal)
	}
	if got.Summary != plan.Summary {
		t.Errorf("Summary = %q, want %q", got.Summary, plan.Summary)
	}
	if len(got.Steps) != len(plan.Steps) {
		t.Errorf("Steps length = %d, want %d", len(got.Steps), len(plan.Steps))
	}
	if got.ImplementationStatus != plan.ImplementationStatus {
		t.Errorf("ImplementationStatus = %q, want %q", got.ImplementationStatus, plan.ImplementationStatus)
	}
}

func TestPlanStepJSONSerialization(t *testing.T) {
	step := PlanStep{
		ID:             "step_1",
		Title:          "test step",
		Description:    "test description",
		RiskLevel:      "medium",
		ExpectedFiles:  []string{"file1.go", "file2.go"},
		ValidationHint: "run tests",
	}

	data, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got PlanStep
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ID != step.ID {
		t.Errorf("ID = %q, want %q", got.ID, step.ID)
	}
	if got.Title != step.Title {
		t.Errorf("Title = %q, want %q", got.Title, step.Title)
	}
	if len(got.ExpectedFiles) != len(step.ExpectedFiles) {
		t.Errorf("ExpectedFiles length = %d, want %d", len(got.ExpectedFiles), len(step.ExpectedFiles))
	}
}

func TestParsePlanResponseValidJSON(t *testing.T) {
	input := `{
		"goal": "test goal",
		"summary": "test summary",
		"steps": [{"id": "step_1", "title": "test", "description": "desc", "risk_level": "low"}],
		"risks": [],
		"files_maybe_affected": [],
		"validation_suggestions": [],
		"implementation_status": "plan_only"
	}`

	plan, err := ParsePlanResponse(input)
	if err != nil {
		t.Fatalf("ParsePlanResponse() error = %v", err)
	}

	if plan.Goal != "test goal" {
		t.Errorf("Goal = %q, want %q", plan.Goal, "test goal")
	}
	if plan.ImplementationStatus != "plan_only" {
		t.Errorf("ImplementationStatus = %q, want %q", plan.ImplementationStatus, "plan_only")
	}
}

func TestParsePlanResponseMarkdownJSON(t *testing.T) {
	input := "```json\n{\"goal\": \"test\", \"summary\": \"sum\", \"steps\": [{\"id\": \"s1\", \"title\": \"t\", \"description\": \"d\", \"risk_level\": \"low\"}], \"risks\": [], \"files_maybe_affected\": [], \"validation_suggestions\": [], \"implementation_status\": \"plan_only\"}\n```"

	plan, err := ParsePlanResponse(input)
	if err != nil {
		t.Fatalf("ParsePlanResponse() error = %v", err)
	}

	if plan.Goal != "test" {
		t.Errorf("Goal = %q, want %q", plan.Goal, "test")
	}
}

func TestParsePlanResponseInvalidJSON(t *testing.T) {
	input := "this is not json"

	_, err := ParsePlanResponse(input)
	if err == nil {
		t.Error("ParsePlanResponse() should return error for invalid JSON")
	}
}

func TestParsePlanResponseMissingFields(t *testing.T) {
	input := `{"goal": "test"}`

	_, err := ParsePlanResponse(input)
	if err == nil {
		t.Error("ParsePlanResponse() should return error for missing fields")
	}
}

func TestParsePlanResponseForcesPlanOnly(t *testing.T) {
	input := `{
		"goal": "test",
		"summary": "sum",
		"steps": [{"id": "s1", "title": "t", "description": "d", "risk_level": "low"}],
		"risks": [],
		"files_maybe_affected": [],
		"validation_suggestions": [],
		"implementation_status": "implemented"
	}`

	plan, err := ParsePlanResponse(input)
	if err != nil {
		t.Fatalf("ParsePlanResponse() error = %v", err)
	}

	if plan.ImplementationStatus != "plan_only" {
		t.Errorf("ImplementationStatus = %q, want %q", plan.ImplementationStatus, "plan_only")
	}
}

func TestAgentPlanValidate(t *testing.T) {
	tests := []struct {
		name    string
		plan    AgentPlan
		wantErr bool
	}{
		{
			name: "valid plan",
			plan: AgentPlan{
				Goal:                 "test",
				Summary:              "sum",
				Steps:                []PlanStep{{ID: "s1", Title: "t", Description: "d", RiskLevel: "low"}},
				ImplementationStatus: "plan_only",
			},
			wantErr: false,
		},
		{
			name: "missing goal",
			plan: AgentPlan{
				Summary:              "sum",
				Steps:                []PlanStep{{ID: "s1", Title: "t", Description: "d", RiskLevel: "low"}},
				ImplementationStatus: "plan_only",
			},
			wantErr: true,
		},
		{
			name: "missing summary",
			plan: AgentPlan{
				Goal:                 "test",
				Steps:                []PlanStep{{ID: "s1", Title: "t", Description: "d", RiskLevel: "low"}},
				ImplementationStatus: "plan_only",
			},
			wantErr: true,
		},
		{
			name: "missing steps",
			plan: AgentPlan{
				Goal:                 "test",
				Summary:              "sum",
				ImplementationStatus: "plan_only",
			},
			wantErr: true,
		},
		{
			name: "wrong implementation status",
			plan: AgentPlan{
				Goal:                 "test",
				Summary:              "sum",
				Steps:                []PlanStep{{ID: "s1", Title: "t", Description: "d", RiskLevel: "low"}},
				ImplementationStatus: "implemented",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.plan.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "pure JSON",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "markdown code block",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "code block without language",
			input: "```\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON with surrounding text",
			input: "Here is the plan:\n{\"key\": \"value\"}\nDone.",
			want:  `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatPlan(t *testing.T) {
	plan := &AgentPlan{
		Goal:    "test goal",
		Summary: "test summary",
		Steps: []PlanStep{
			{
				ID:             "step_1",
				Title:          "test step",
				Description:    "test description",
				RiskLevel:      "low",
				ExpectedFiles:  []string{"file.go"},
				ValidationHint: "run tests",
			},
		},
		ImplementationStatus: "plan_only",
	}

	output := FormatPlan(plan)

	if output == "" {
		t.Error("FormatPlan() returned empty string")
	}
	if !containsString(output, "test goal") {
		t.Error("FormatPlan() should contain goal")
	}
	if !containsString(output, "No files were modified") {
		t.Error("FormatPlan() should contain 'No files were modified'")
	}
}

func TestFormatPlanJSON(t *testing.T) {
	plan := &AgentPlan{
		Goal:                 "test goal",
		Summary:              "test summary",
		Steps:                []PlanStep{{ID: "step_1", Title: "t", Description: "d", RiskLevel: "low"}},
		ImplementationStatus: "plan_only",
	}

	jsonStr, err := FormatPlanJSON(plan)
	if err != nil {
		t.Fatalf("FormatPlanJSON() error = %v", err)
	}

	if jsonStr == "" {
		t.Error("FormatPlanJSON() returned empty string")
	}

	// Verify it's valid JSON
	var got AgentPlan
	if err := json.Unmarshal([]byte(jsonStr), &got); err != nil {
		t.Errorf("FormatPlanJSON() returned invalid JSON: %v", err)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
