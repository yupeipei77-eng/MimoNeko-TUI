package agents

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPatchPreviewJSONSerialization(t *testing.T) {
	preview := &PatchPreview{
		ID:                    "preview_test",
		WorkflowID:            "wf_test",
		Goal:                  "test goal",
		Source:                PatchPreviewSourceIntent,
		ImplementationStatus:  "preview_only",
		Files:                 []PatchPreviewFile{{Path: "README.md", ChangeType: "edit", Reason: "update", RiskLevel: "low", PreviewSummary: "edit README.md"}},
		Changes:               []PatchPreviewChange{{ID: "c1", FilePath: "README.md", Description: "update docs", ExpectedEffect: "better docs", SafetyNotes: ""}},
		Risks:                 []string{"minor risk"},
		ValidationSuggestions: []string{"run tests"},
		NoFileWrites:          true,
		NoPatchApplied:        true,
		NoToolsExecuted:       true,
		CreatedAt:             time.Now().UTC(),
	}

	data, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got PatchPreview
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ID != preview.ID {
		t.Errorf("ID = %q, want %q", got.ID, preview.ID)
	}
	if got.ImplementationStatus != preview.ImplementationStatus {
		t.Errorf("ImplementationStatus = %q, want %q", got.ImplementationStatus, preview.ImplementationStatus)
	}
	if got.NoFileWrites != preview.NoFileWrites {
		t.Errorf("NoFileWrites = %v, want %v", got.NoFileWrites, preview.NoFileWrites)
	}
}

func TestGeneratePreviewFromIntent(t *testing.T) {
	intent := &CoderPatchIntent{
		Goal:                 "test goal",
		PlanSummary:          "test summary",
		ImplementationStatus: "intent_only",
		FilesToChange:        []PatchIntentFile{{Path: "README.md", ChangeType: "edit", Reason: "update", RiskLevel: "low"}},
		Changes:              []PatchIntentChange{{ID: "c1", FilePath: "README.md", Description: "update docs"}},
		Risks:                []string{"minor risk"},
		ValidationSuggestions: []string{"run tests"},
		NoFileWrites:         true,
	}

	preview, err := GeneratePreviewFromIntent(intent)
	if err != nil {
		t.Fatalf("GeneratePreviewFromIntent() error = %v", err)
	}

	if preview.ImplementationStatus != "preview_only" {
		t.Errorf("ImplementationStatus = %q, want %q", preview.ImplementationStatus, "preview_only")
	}
	if !preview.NoFileWrites {
		t.Error("NoFileWrites should be true")
	}
	if !preview.NoPatchApplied {
		t.Error("NoPatchApplied should be true")
	}
	if !preview.NoToolsExecuted {
		t.Error("NoToolsExecuted should be true")
	}
	if len(preview.Files) != 1 {
		t.Errorf("Files length = %d, want 1", len(preview.Files))
	}
	if len(preview.Changes) != 1 {
		t.Errorf("Changes length = %d, want 1", len(preview.Changes))
	}
}

func TestGeneratePreviewFromIntentInvalidStatus(t *testing.T) {
	intent := &CoderPatchIntent{
		Goal:                 "test",
		PlanSummary:          "sum",
		ImplementationStatus: "implemented",
		FilesToChange:        []PatchIntentFile{{Path: "f.go", ChangeType: "edit", Reason: "r", RiskLevel: "low"}},
		Changes:              []PatchIntentChange{{ID: "c1", FilePath: "f.go", Description: "d"}},
		NoFileWrites:         true,
	}

	_, err := GeneratePreviewFromIntent(intent)
	if err == nil {
		t.Error("GeneratePreviewFromIntent() should return error for invalid status")
	}
}

func TestGeneratePreviewFromIntentNoFileWritesFalse(t *testing.T) {
	intent := &CoderPatchIntent{
		Goal:                 "test",
		PlanSummary:          "sum",
		ImplementationStatus: "intent_only",
		FilesToChange:        []PatchIntentFile{{Path: "f.go", ChangeType: "edit", Reason: "r", RiskLevel: "low"}},
		Changes:              []PatchIntentChange{{ID: "c1", FilePath: "f.go", Description: "d"}},
		NoFileWrites:         false,
	}

	_, err := GeneratePreviewFromIntent(intent)
	if err == nil {
		t.Error("GeneratePreviewFromIntent() should return error when no_file_writes is false")
	}
}

func TestGeneratePreviewFromReport(t *testing.T) {
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
		CoderIntent: &CoderPatchIntent{
			Goal:                 "test goal",
			PlanSummary:          "test summary",
			ImplementationStatus: "intent_only",
			FilesToChange:        []PatchIntentFile{{Path: "README.md", ChangeType: "edit", Reason: "update", RiskLevel: "low"}},
			Changes:              []PatchIntentChange{{ID: "c1", FilePath: "README.md", Description: "update docs"}},
			NoFileWrites:         true,
		},
	}

	preview, err := GeneratePreviewFromReport(report)
	if err != nil {
		t.Fatalf("GeneratePreviewFromReport() error = %v", err)
	}

	if preview.WorkflowID != "wf_test" {
		t.Errorf("WorkflowID = %q, want %q", preview.WorkflowID, "wf_test")
	}
	if preview.Source != PatchPreviewSourceReport {
		t.Errorf("Source = %q, want %q", preview.Source, PatchPreviewSourceReport)
	}
}

func TestGeneratePreviewFromReportMissingCoderIntent(t *testing.T) {
	report := &AgentDryRunReport{
		Goal:             "test",
		WorkflowID:       "wf_test",
		NoFileWrites:     true,
		NoPatchGenerated: true,
		NoToolsExecuted:  true,
		NoTestsExecuted:  true,
	}

	_, err := GeneratePreviewFromReport(report)
	if err == nil {
		t.Error("GeneratePreviewFromReport() should return error when coder_intent is missing")
	}
}

func TestGeneratePreviewFromReportUnsafeFlags(t *testing.T) {
	tests := []struct {
		name   string
		report AgentDryRunReport
	}{
		{
			name: "no_file_writes false",
			report: AgentDryRunReport{
				WorkflowID:       "wf_test",
				NoFileWrites:     false,
				NoPatchGenerated: true,
				NoToolsExecuted:  true,
				NoTestsExecuted:  true,
				CoderIntent:      &CoderPatchIntent{Goal: "t", PlanSummary: "s", ImplementationStatus: "intent_only", FilesToChange: []PatchIntentFile{{Path: "f", ChangeType: "e", Reason: "r", RiskLevel: "low"}}, Changes: []PatchIntentChange{{ID: "c", FilePath: "f", Description: "d"}}, NoFileWrites: true},
			},
		},
		{
			name: "no_patch_generated false",
			report: AgentDryRunReport{
				WorkflowID:       "wf_test",
				NoFileWrites:     true,
				NoPatchGenerated: false,
				NoToolsExecuted:  true,
				NoTestsExecuted:  true,
				CoderIntent:      &CoderPatchIntent{Goal: "t", PlanSummary: "s", ImplementationStatus: "intent_only", FilesToChange: []PatchIntentFile{{Path: "f", ChangeType: "e", Reason: "r", RiskLevel: "low"}}, Changes: []PatchIntentChange{{ID: "c", FilePath: "f", Description: "d"}}, NoFileWrites: true},
			},
		},
		{
			name: "no_tools_executed false",
			report: AgentDryRunReport{
				WorkflowID:       "wf_test",
				NoFileWrites:     true,
				NoPatchGenerated: true,
				NoToolsExecuted:  false,
				NoTestsExecuted:  true,
				CoderIntent:      &CoderPatchIntent{Goal: "t", PlanSummary: "s", ImplementationStatus: "intent_only", FilesToChange: []PatchIntentFile{{Path: "f", ChangeType: "e", Reason: "r", RiskLevel: "low"}}, Changes: []PatchIntentChange{{ID: "c", FilePath: "f", Description: "d"}}, NoFileWrites: true},
			},
		},
		{
			name: "no_tests_executed false",
			report: AgentDryRunReport{
				WorkflowID:       "wf_test",
				NoFileWrites:     true,
				NoPatchGenerated: true,
				NoToolsExecuted:  true,
				NoTestsExecuted:  false,
				CoderIntent:      &CoderPatchIntent{Goal: "t", PlanSummary: "s", ImplementationStatus: "intent_only", FilesToChange: []PatchIntentFile{{Path: "f", ChangeType: "e", Reason: "r", RiskLevel: "low"}}, Changes: []PatchIntentChange{{ID: "c", FilePath: "f", Description: "d"}}, NoFileWrites: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GeneratePreviewFromReport(&tt.report)
			if err == nil {
				t.Errorf("GeneratePreviewFromReport() should return error for %s", tt.name)
			}
		})
	}
}

func TestPatchPreviewValidate(t *testing.T) {
	tests := []struct {
		name    string
		preview PatchPreview
		wantErr bool
	}{
		{
			name: "valid preview",
			preview: PatchPreview{
				ImplementationStatus: "preview_only",
				NoFileWrites:         true,
				NoPatchApplied:       true,
				NoToolsExecuted:      true,
			},
			wantErr: false,
		},
		{
			name: "wrong implementation status",
			preview: PatchPreview{
				ImplementationStatus: "implemented",
				NoFileWrites:         true,
				NoPatchApplied:       true,
				NoToolsExecuted:      true,
			},
			wantErr: true,
		},
		{
			name: "no_file_writes false",
			preview: PatchPreview{
				ImplementationStatus: "preview_only",
				NoFileWrites:         false,
				NoPatchApplied:       true,
				NoToolsExecuted:      true,
			},
			wantErr: true,
		},
		{
			name: "no_patch_applied false",
			preview: PatchPreview{
				ImplementationStatus: "preview_only",
				NoFileWrites:         true,
				NoPatchApplied:       false,
				NoToolsExecuted:      true,
			},
			wantErr: true,
		},
		{
			name: "no_tools_executed false",
			preview: PatchPreview{
				ImplementationStatus: "preview_only",
				NoFileWrites:         true,
				NoPatchApplied:       true,
				NoToolsExecuted:      false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.preview.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatPatchPreview(t *testing.T) {
	preview := &PatchPreview{
		ID:                   "preview_test",
		WorkflowID:           "wf_test",
		Goal:                 "test goal",
		Source:               PatchPreviewSourceIntent,
		ImplementationStatus: "preview_only",
		Files:                []PatchPreviewFile{{Path: "README.md", ChangeType: "edit", Reason: "update", RiskLevel: "low"}},
		Changes:              []PatchPreviewChange{{ID: "c1", FilePath: "README.md", Description: "update docs"}},
		NoFileWrites:         true,
		NoPatchApplied:       true,
		NoToolsExecuted:      true,
	}

	output := FormatPatchPreview(preview)

	if output == "" {
		t.Error("FormatPatchPreview() returned empty string")
	}
	if !containsString(output, "preview_only") {
		t.Error("FormatPatchPreview() should contain 'preview_only'")
	}
	if !containsString(output, "No files were modified") {
		t.Error("FormatPatchPreview() should contain 'No files were modified'")
	}
}

func TestFormatPatchPreviewJSON(t *testing.T) {
	preview := &PatchPreview{
		ID:                   "preview_test",
		Goal:                 "test goal",
		Source:               PatchPreviewSourceIntent,
		ImplementationStatus: "preview_only",
		NoFileWrites:         true,
		NoPatchApplied:       true,
		NoToolsExecuted:      true,
	}

	jsonStr, err := FormatPatchPreviewJSON(preview)
	if err != nil {
		t.Fatalf("FormatPatchPreviewJSON() error = %v", err)
	}

	if jsonStr == "" {
		t.Error("FormatPatchPreviewJSON() returned empty string")
	}

	var got PatchPreview
	if err := json.Unmarshal([]byte(jsonStr), &got); err != nil {
		t.Errorf("FormatPatchPreviewJSON() returned invalid JSON: %v", err)
	}
}
