package agents

import (
	"encoding/json"
	"testing"
)

func TestCoderPatchIntentJSONSerialization(t *testing.T) {
	intent := &CoderPatchIntent{
		Goal:                 "test goal",
		PlanSummary:          "test plan summary",
		ImplementationStatus: "intent_only",
		FilesToChange: []PatchIntentFile{
			{
				Path:       "README.md",
				ChangeType: "edit",
				Reason:     "update docs",
				RiskLevel:  "low",
			},
		},
		Changes: []PatchIntentChange{
			{
				ID:             "change_1",
				FilePath:       "README.md",
				Description:    "add usage example",
				ExpectedEffect: "improve documentation",
				SafetyNotes:    "no breaking changes",
			},
		},
		Risks:                 []string{"minor formatting change"},
		ValidationSuggestions: []string{"run tests"},
		NoFileWrites:          true,
	}

	data, err := json.Marshal(intent)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got CoderPatchIntent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Goal != intent.Goal {
		t.Errorf("Goal = %q, want %q", got.Goal, intent.Goal)
	}
	if got.ImplementationStatus != intent.ImplementationStatus {
		t.Errorf("ImplementationStatus = %q, want %q", got.ImplementationStatus, intent.ImplementationStatus)
	}
	if got.NoFileWrites != intent.NoFileWrites {
		t.Errorf("NoFileWrites = %v, want %v", got.NoFileWrites, intent.NoFileWrites)
	}
}

func TestPatchIntentFileJSONSerialization(t *testing.T) {
	file := PatchIntentFile{
		Path:       "README.md",
		ChangeType: "edit",
		Reason:     "update docs",
		RiskLevel:  "low",
	}

	data, err := json.Marshal(file)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got PatchIntentFile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Path != file.Path {
		t.Errorf("Path = %q, want %q", got.Path, file.Path)
	}
	if got.ChangeType != file.ChangeType {
		t.Errorf("ChangeType = %q, want %q", got.ChangeType, file.ChangeType)
	}
}

func TestPatchIntentChangeJSONSerialization(t *testing.T) {
	change := PatchIntentChange{
		ID:             "change_1",
		FilePath:       "README.md",
		Description:    "add usage example",
		ExpectedEffect: "improve documentation",
		SafetyNotes:    "no breaking changes",
	}

	data, err := json.Marshal(change)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got PatchIntentChange
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ID != change.ID {
		t.Errorf("ID = %q, want %q", got.ID, change.ID)
	}
	if got.FilePath != change.FilePath {
		t.Errorf("FilePath = %q, want %q", got.FilePath, change.FilePath)
	}
}

func TestParseCoderIntentResponseValidJSON(t *testing.T) {
	input := `{
		"goal": "test goal",
		"plan_summary": "test summary",
		"implementation_status": "intent_only",
		"files_to_change": [{"path": "README.md", "change_type": "edit", "reason": "docs", "risk_level": "low"}],
		"changes": [{"id": "c1", "file_path": "README.md", "description": "update", "expected_effect": "better docs", "safety_notes": ""}],
		"risks": [],
		"validation_suggestions": [],
		"no_file_writes": true
	}`

	intent, err := ParseCoderIntentResponse(input)
	if err != nil {
		t.Fatalf("ParseCoderIntentResponse() error = %v", err)
	}

	if intent.Goal != "test goal" {
		t.Errorf("Goal = %q, want %q", intent.Goal, "test goal")
	}
	if intent.ImplementationStatus != "intent_only" {
		t.Errorf("ImplementationStatus = %q, want %q", intent.ImplementationStatus, "intent_only")
	}
	if !intent.NoFileWrites {
		t.Errorf("NoFileWrites = false, want true")
	}
}

func TestParseCoderIntentResponseMarkdownJSON(t *testing.T) {
	input := "```json\n{\"goal\": \"test\", \"plan_summary\": \"sum\", \"implementation_status\": \"intent_only\", \"files_to_change\": [{\"path\": \"f.go\", \"change_type\": \"edit\", \"reason\": \"r\", \"risk_level\": \"low\"}], \"changes\": [{\"id\": \"c1\", \"file_path\": \"f.go\", \"description\": \"d\", \"expected_effect\": \"e\", \"safety_notes\": \"\"}], \"risks\": [], \"validation_suggestions\": [], \"no_file_writes\": true}\n```"

	intent, err := ParseCoderIntentResponse(input)
	if err != nil {
		t.Fatalf("ParseCoderIntentResponse() error = %v", err)
	}

	if intent.Goal != "test" {
		t.Errorf("Goal = %q, want %q", intent.Goal, "test")
	}
}

func TestParseCoderIntentResponseInvalidJSON(t *testing.T) {
	input := "this is not json"

	_, err := ParseCoderIntentResponse(input)
	if err == nil {
		t.Error("ParseCoderIntentResponse() should return error for invalid JSON")
	}
}

func TestParseCoderIntentResponseMissingFields(t *testing.T) {
	input := `{"goal": "test"}`

	_, err := ParseCoderIntentResponse(input)
	if err == nil {
		t.Error("ParseCoderIntentResponse() should return error for missing fields")
	}
}

func TestParseCoderIntentResponseForcesIntentOnly(t *testing.T) {
	input := `{
		"goal": "test",
		"plan_summary": "sum",
		"implementation_status": "implemented",
		"files_to_change": [{"path": "f.go", "change_type": "edit", "reason": "r", "risk_level": "low"}],
		"changes": [{"id": "c1", "file_path": "f.go", "description": "d", "expected_effect": "e", "safety_notes": ""}],
		"risks": [],
		"validation_suggestions": [],
		"no_file_writes": false
	}`

	intent, err := ParseCoderIntentResponse(input)
	if err != nil {
		t.Fatalf("ParseCoderIntentResponse() error = %v", err)
	}

	if intent.ImplementationStatus != "intent_only" {
		t.Errorf("ImplementationStatus = %q, want %q", intent.ImplementationStatus, "intent_only")
	}
	if !intent.NoFileWrites {
		t.Errorf("NoFileWrites = false, want true")
	}
}

func TestCoderPatchIntentValidate(t *testing.T) {
	tests := []struct {
		name    string
		intent  CoderPatchIntent
		wantErr bool
	}{
		{
			name: "valid intent",
			intent: CoderPatchIntent{
				Goal:                 "test",
				PlanSummary:          "sum",
				ImplementationStatus: "intent_only",
				FilesToChange:        []PatchIntentFile{{Path: "f.go", ChangeType: "edit", Reason: "r", RiskLevel: "low"}},
				Changes:              []PatchIntentChange{{ID: "c1", FilePath: "f.go", Description: "d", ExpectedEffect: "e"}},
				NoFileWrites:         true,
			},
			wantErr: false,
		},
		{
			name: "missing goal",
			intent: CoderPatchIntent{
				PlanSummary:          "sum",
				ImplementationStatus: "intent_only",
				FilesToChange:        []PatchIntentFile{{Path: "f.go", ChangeType: "edit", Reason: "r", RiskLevel: "low"}},
				Changes:              []PatchIntentChange{{ID: "c1", FilePath: "f.go", Description: "d", ExpectedEffect: "e"}},
				NoFileWrites:         true,
			},
			wantErr: true,
		},
		{
			name: "wrong implementation status",
			intent: CoderPatchIntent{
				Goal:                 "test",
				PlanSummary:          "sum",
				ImplementationStatus: "implemented",
				FilesToChange:        []PatchIntentFile{{Path: "f.go", ChangeType: "edit", Reason: "r", RiskLevel: "low"}},
				Changes:              []PatchIntentChange{{ID: "c1", FilePath: "f.go", Description: "d", ExpectedEffect: "e"}},
				NoFileWrites:         true,
			},
			wantErr: true,
		},
		{
			name: "no_file_writes false",
			intent: CoderPatchIntent{
				Goal:                 "test",
				PlanSummary:          "sum",
				ImplementationStatus: "intent_only",
				FilesToChange:        []PatchIntentFile{{Path: "f.go", ChangeType: "edit", Reason: "r", RiskLevel: "low"}},
				Changes:              []PatchIntentChange{{ID: "c1", FilePath: "f.go", Description: "d", ExpectedEffect: "e"}},
				NoFileWrites:         false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.intent.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCoderIntentDangerousStatus(t *testing.T) {
	dangerousStatuses := []string{"implemented", "applied", "done", "patched", "executed", "modified", "committed", "pushed"}

	for _, status := range dangerousStatuses {
		intent := &CoderPatchIntent{
			Goal:                 "test",
			PlanSummary:          "sum",
			ImplementationStatus: status,
			FilesToChange:        []PatchIntentFile{{Path: "f.go", ChangeType: "edit", Reason: "r", RiskLevel: "low"}},
			Changes:              []PatchIntentChange{{ID: "c1", FilePath: "f.go", Description: "d", ExpectedEffect: "e"}},
			NoFileWrites:         true,
		}

		err := ValidateCoderIntent(intent)
		if err == nil {
			t.Errorf("ValidateCoderIntent() should return error for status %q", status)
		}
	}
}

func TestValidateCoderIntentNoFileWritesFalse(t *testing.T) {
	intent := &CoderPatchIntent{
		Goal:                 "test",
		PlanSummary:          "sum",
		ImplementationStatus: "intent_only",
		FilesToChange:        []PatchIntentFile{{Path: "f.go", ChangeType: "edit", Reason: "r", RiskLevel: "low"}},
		Changes:              []PatchIntentChange{{ID: "c1", FilePath: "f.go", Description: "d", ExpectedEffect: "e"}},
		NoFileWrites:         false,
	}

	err := ValidateCoderIntent(intent)
	if err == nil {
		t.Error("ValidateCoderIntent() should return error when no_file_writes is false")
	}
}

func TestValidateCoderIntentDiffPatchContent(t *testing.T) {
	intent := &CoderPatchIntent{
		Goal:                 "test",
		PlanSummary:          "sum",
		ImplementationStatus: "intent_only",
		FilesToChange:        []PatchIntentFile{{Path: "f.go", ChangeType: "edit", Reason: "r", RiskLevel: "low"}},
		Changes: []PatchIntentChange{
			{
				ID:             "c1",
				FilePath:       "f.go",
				Description:    "diff --git a/f.go b/f.go",
				ExpectedEffect: "apply patch",
			},
		},
		NoFileWrites: true,
	}

	err := ValidateCoderIntent(intent)
	if err == nil {
		t.Error("ValidateCoderIntent() should return error for diff patch content")
	}
}

func TestValidateCoderIntentCommandExecution(t *testing.T) {
	intent := &CoderPatchIntent{
		Goal:                 "test",
		PlanSummary:          "sum",
		ImplementationStatus: "intent_only",
		FilesToChange:        []PatchIntentFile{{Path: "f.go", ChangeType: "edit", Reason: "r", RiskLevel: "low"}},
		Changes: []PatchIntentChange{
			{
				ID:             "c1",
				FilePath:       "f.go",
				Description:    "command executed successfully",
				ExpectedEffect: "verify changes",
			},
		},
		NoFileWrites: true,
	}

	err := ValidateCoderIntent(intent)
	if err == nil {
		t.Error("ValidateCoderIntent() should return error for command execution wording")
	}
}

func TestFormatCoderIntent(t *testing.T) {
	intent := &CoderPatchIntent{
		Goal:                 "test goal",
		PlanSummary:          "test summary",
		ImplementationStatus: "intent_only",
		FilesToChange: []PatchIntentFile{
			{
				Path:       "README.md",
				ChangeType: "edit",
				Reason:     "update docs",
				RiskLevel:  "low",
			},
		},
		Changes: []PatchIntentChange{
			{
				ID:             "change_1",
				FilePath:       "README.md",
				Description:    "add usage example",
				ExpectedEffect: "improve documentation",
				SafetyNotes:    "no breaking changes",
			},
		},
		NoFileWrites: true,
	}

	output := FormatCoderIntent(intent)

	if output == "" {
		t.Error("FormatCoderIntent() returned empty string")
	}
	if !containsString(output, "test goal") {
		t.Error("FormatCoderIntent() should contain goal")
	}
	if !containsString(output, "No files were modified") {
		t.Error("FormatCoderIntent() should contain 'No files were modified'")
	}
}

func TestFormatCoderIntentJSON(t *testing.T) {
	intent := &CoderPatchIntent{
		Goal:                 "test goal",
		PlanSummary:          "test summary",
		ImplementationStatus: "intent_only",
		FilesToChange:        []PatchIntentFile{{Path: "f.go", ChangeType: "edit", Reason: "r", RiskLevel: "low"}},
		Changes:              []PatchIntentChange{{ID: "c1", FilePath: "f.go", Description: "d", ExpectedEffect: "e"}},
		NoFileWrites:         true,
	}

	jsonStr, err := FormatCoderIntentJSON(intent)
	if err != nil {
		t.Fatalf("FormatCoderIntentJSON() error = %v", err)
	}

	if jsonStr == "" {
		t.Error("FormatCoderIntentJSON() returned empty string")
	}

	// Verify it's valid JSON
	var got CoderPatchIntent
	if err := json.Unmarshal([]byte(jsonStr), &got); err != nil {
		t.Errorf("FormatCoderIntentJSON() returned invalid JSON: %v", err)
	}
}
