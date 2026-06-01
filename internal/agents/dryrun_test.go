package agents

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAgentDryRunReportJSONSerialization(t *testing.T) {
	report := &AgentDryRunReport{
		Goal:             "test goal",
		WorkflowID:       "wf_test",
		RunID:            "run_test",
		Provider:         "mimo",
		Model:            "mimo-v2.5-pro",
		Status:           WorkflowStatusCompleted,
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
		CompletedAt:      time.Now().UTC(),
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got AgentDryRunReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Goal != report.Goal {
		t.Errorf("Goal = %q, want %q", got.Goal, report.Goal)
	}
	if got.Status != report.Status {
		t.Errorf("Status = %q, want %q", got.Status, report.Status)
	}
	if got.NoFileWrites != report.NoFileWrites {
		t.Errorf("NoFileWrites = %v, want %v", got.NoFileWrites, report.NoFileWrites)
	}
}

func TestAgentDryRunReportValidate(t *testing.T) {
	tests := []struct {
		name    string
		report  AgentDryRunReport
		wantErr bool
	}{
		{
			name: "valid report",
			report: AgentDryRunReport{
				Goal:             "test",
				NoFileWrites:     true,
				NoPatchGenerated: true,
				NoToolsExecuted:  true,
				NoTestsExecuted:  true,
			},
			wantErr: false,
		},
		{
			name: "no_file_writes false",
			report: AgentDryRunReport{
				Goal:             "test",
				NoFileWrites:     false,
				NoPatchGenerated: true,
				NoToolsExecuted:  true,
				NoTestsExecuted:  true,
			},
			wantErr: true,
		},
		{
			name: "no_patch_generated false",
			report: AgentDryRunReport{
				Goal:             "test",
				NoFileWrites:     true,
				NoPatchGenerated: false,
				NoToolsExecuted:  true,
				NoTestsExecuted:  true,
			},
			wantErr: true,
		},
		{
			name: "no_tools_executed false",
			report: AgentDryRunReport{
				Goal:             "test",
				NoFileWrites:     true,
				NoPatchGenerated: true,
				NoToolsExecuted:  false,
				NoTestsExecuted:  true,
			},
			wantErr: true,
		},
		{
			name: "no_tests_executed false",
			report: AgentDryRunReport{
				Goal:             "test",
				NoFileWrites:     true,
				NoPatchGenerated: true,
				NoToolsExecuted:  true,
				NoTestsExecuted:  false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.report.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatDryRunReport(t *testing.T) {
	report := &AgentDryRunReport{
		Goal:             "test goal",
		WorkflowID:       "wf_test",
		RunID:            "run_test",
		Provider:         "mimo",
		Model:            "mimo-v2.5-pro",
		Status:           WorkflowStatusCompleted,
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
		CompletedAt:      time.Now().UTC(),
	}

	output := FormatDryRunReport(report)

	if output == "" {
		t.Error("FormatDryRunReport() returned empty string")
	}
	if !containsString(output, "test goal") {
		t.Error("FormatDryRunReport() should contain goal")
	}
	if !containsString(output, "No files were modified") {
		t.Error("FormatDryRunReport() should contain 'No files were modified'")
	}
	if !containsString(output, "end-to-end dry run") {
		t.Error("FormatDryRunReport() should contain 'end-to-end dry run'")
	}
}

func TestFormatDryRunReportJSON(t *testing.T) {
	report := &AgentDryRunReport{
		Goal:             "test goal",
		WorkflowID:       "wf_test",
		RunID:            "run_test",
		Provider:         "mimo",
		Model:            "mimo-v2.5-pro",
		Status:           WorkflowStatusCompleted,
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
		CreatedAt:        time.Now().UTC(),
		CompletedAt:      time.Now().UTC(),
	}

	jsonStr, err := FormatDryRunReportJSON(report)
	if err != nil {
		t.Fatalf("FormatDryRunReportJSON() error = %v", err)
	}

	if jsonStr == "" {
		t.Error("FormatDryRunReportJSON() returned empty string")
	}

	// 验证是有效的 JSON
	var got AgentDryRunReport
	if err := json.Unmarshal([]byte(jsonStr), &got); err != nil {
		t.Errorf("FormatDryRunReportJSON() returned invalid JSON: %v", err)
	}
}

func TestRunSkeletonDryRun(t *testing.T) {
	runner := NewWorkflowRunner(nil)
	report, err := runner.RunSkeletonDryRun("test goal")
	if err != nil {
		t.Fatalf("RunSkeletonDryRun() error = %v", err)
	}

	if report.Status != WorkflowStatusCompleted {
		t.Errorf("Status = %q, want %q", report.Status, WorkflowStatusCompleted)
	}
	if !report.NoFileWrites {
		t.Error("NoFileWrites should be true")
	}
	if !report.NoPatchGenerated {
		t.Error("NoPatchGenerated should be true")
	}
	if !report.NoToolsExecuted {
		t.Error("NoToolsExecuted should be true")
	}
	if !report.NoTestsExecuted {
		t.Error("NoTestsExecuted should be true")
	}
	if report.PlannerPlan == nil {
		t.Error("PlannerPlan should not be nil")
	}
	if report.CoderIntent == nil {
		t.Error("CoderIntent should not be nil")
	}
	if report.ReviewerReview == nil {
		t.Error("ReviewerReview should not be nil")
	}
	if report.ValidatorSuggestions == nil {
		t.Error("ValidatorSuggestions should not be nil")
	}
}
