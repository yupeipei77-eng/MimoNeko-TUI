package agents

import (
	"encoding/json"
	"testing"
)

func TestValidatorSuggestionsJSONSerialization(t *testing.T) {
	suggestions := &ValidatorSuggestions{
		Goal:                 "test goal",
		ValidationStatus:     "pending",
		ImplementationStatus: "suggestions_only",
		Summary:              "test summary",
		Checks: []ValidationCheck{
			{
				ID:             "check_1",
				Category:       "unit_test",
				Description:    "run unit tests",
				ExpectedSignal: "all tests pass",
				Priority:       "high",
				RelatedFiles:   []string{"main.go"},
			},
		},
		Risks:               []string{"test coverage gap"},
		RecommendedCommands: []string{"go test ./..."},
		ManualChecks:        []string{"check README"},
		NoFileWrites:        true,
		NoTestsExecuted:     true,
		NoToolsExecuted:     true,
	}

	data, err := json.Marshal(suggestions)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got ValidatorSuggestions
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Goal != suggestions.Goal {
		t.Errorf("Goal = %q, want %q", got.Goal, suggestions.Goal)
	}
	if got.ImplementationStatus != suggestions.ImplementationStatus {
		t.Errorf("ImplementationStatus = %q, want %q", got.ImplementationStatus, suggestions.ImplementationStatus)
	}
	if got.NoFileWrites != suggestions.NoFileWrites {
		t.Errorf("NoFileWrites = %v, want %v", got.NoFileWrites, suggestions.NoFileWrites)
	}
	if got.NoTestsExecuted != suggestions.NoTestsExecuted {
		t.Errorf("NoTestsExecuted = %v, want %v", got.NoTestsExecuted, suggestions.NoTestsExecuted)
	}
	if got.NoToolsExecuted != suggestions.NoToolsExecuted {
		t.Errorf("NoToolsExecuted = %v, want %v", got.NoToolsExecuted, suggestions.NoToolsExecuted)
	}
}

func TestValidationCheckJSONSerialization(t *testing.T) {
	check := ValidationCheck{
		ID:             "check_1",
		Category:       "unit_test",
		Description:    "run unit tests",
		ExpectedSignal: "all tests pass",
		Priority:       "high",
		RelatedFiles:   []string{"main.go", "utils.go"},
	}

	data, err := json.Marshal(check)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got ValidationCheck
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ID != check.ID {
		t.Errorf("ID = %q, want %q", got.ID, check.ID)
	}
	if got.Category != check.Category {
		t.Errorf("Category = %q, want %q", got.Category, check.Category)
	}
	if len(got.RelatedFiles) != len(check.RelatedFiles) {
		t.Errorf("RelatedFiles length = %d, want %d", len(got.RelatedFiles), len(check.RelatedFiles))
	}
}

func TestParseValidatorSuggestionsResponseValidJSON(t *testing.T) {
	input := `{
		"goal": "test goal",
		"validation_status": "pending",
		"implementation_status": "suggestions_only",
		"summary": "test summary",
		"checks": [],
		"risks": [],
		"recommended_commands": ["go test ./..."],
		"manual_checks": [],
		"no_file_writes": true,
		"no_tests_executed": true,
		"no_tools_executed": true
	}`

	suggestions, err := ParseValidatorSuggestionsResponse(input)
	if err != nil {
		t.Fatalf("ParseValidatorSuggestionsResponse() error = %v", err)
	}

	if suggestions.Goal != "test goal" {
		t.Errorf("Goal = %q, want %q", suggestions.Goal, "test goal")
	}
	if suggestions.ImplementationStatus != "suggestions_only" {
		t.Errorf("ImplementationStatus = %q, want %q", suggestions.ImplementationStatus, "suggestions_only")
	}
	if !suggestions.NoFileWrites {
		t.Errorf("NoFileWrites = false, want true")
	}
	if !suggestions.NoTestsExecuted {
		t.Errorf("NoTestsExecuted = false, want true")
	}
	if !suggestions.NoToolsExecuted {
		t.Errorf("NoToolsExecuted = false, want true")
	}
}

func TestParseValidatorSuggestionsResponseMarkdownJSON(t *testing.T) {
	input := "```json\n{\"goal\": \"test\", \"validation_status\": \"pending\", \"implementation_status\": \"suggestions_only\", \"summary\": \"sum\", \"checks\": [], \"risks\": [], \"recommended_commands\": [], \"manual_checks\": [], \"no_file_writes\": true, \"no_tests_executed\": true, \"no_tools_executed\": true}\n```"

	suggestions, err := ParseValidatorSuggestionsResponse(input)
	if err != nil {
		t.Fatalf("ParseValidatorSuggestionsResponse() error = %v", err)
	}

	if suggestions.Goal != "test" {
		t.Errorf("Goal = %q, want %q", suggestions.Goal, "test")
	}
}

func TestParseValidatorSuggestionsResponseInvalidJSON(t *testing.T) {
	input := "this is not json"

	_, err := ParseValidatorSuggestionsResponse(input)
	if err == nil {
		t.Error("ParseValidatorSuggestionsResponse() should return error for invalid JSON")
	}
}

func TestParseValidatorSuggestionsResponseMissingFields(t *testing.T) {
	input := `{"goal": "test"}`

	_, err := ParseValidatorSuggestionsResponse(input)
	if err == nil {
		t.Error("ParseValidatorSuggestionsResponse() should return error for missing fields")
	}
}

func TestParseValidatorSuggestionsResponseForcesSafeValues(t *testing.T) {
	input := `{
		"goal": "test",
		"validation_status": "pending",
		"implementation_status": "validated",
		"summary": "sum",
		"checks": [],
		"risks": [],
		"recommended_commands": [],
		"manual_checks": [],
		"no_file_writes": false,
		"no_tests_executed": false,
		"no_tools_executed": false
	}`

	suggestions, err := ParseValidatorSuggestionsResponse(input)
	if err != nil {
		t.Fatalf("ParseValidatorSuggestionsResponse() error = %v", err)
	}

	if suggestions.ImplementationStatus != "suggestions_only" {
		t.Errorf("ImplementationStatus = %q, want %q", suggestions.ImplementationStatus, "suggestions_only")
	}
	if !suggestions.NoFileWrites {
		t.Errorf("NoFileWrites = false, want true")
	}
	if !suggestions.NoTestsExecuted {
		t.Errorf("NoTestsExecuted = false, want true")
	}
	if !suggestions.NoToolsExecuted {
		t.Errorf("NoToolsExecuted = false, want true")
	}
}

func TestValidatorSuggestionsValidate(t *testing.T) {
	tests := []struct {
		name        string
		suggestions ValidatorSuggestions
		wantErr     bool
	}{
		{
			name: "valid suggestions",
			suggestions: ValidatorSuggestions{
				Goal:                 "test",
				ValidationStatus:     "pending",
				ImplementationStatus: "suggestions_only",
				Summary:              "sum",
				NoFileWrites:         true,
				NoTestsExecuted:      true,
				NoToolsExecuted:      true,
			},
			wantErr: false,
		},
		{
			name: "missing goal",
			suggestions: ValidatorSuggestions{
				ValidationStatus:     "pending",
				ImplementationStatus: "suggestions_only",
				Summary:              "sum",
				NoFileWrites:         true,
				NoTestsExecuted:      true,
				NoToolsExecuted:      true,
			},
			wantErr: true,
		},
		{
			name: "wrong implementation status",
			suggestions: ValidatorSuggestions{
				Goal:                 "test",
				ValidationStatus:     "pending",
				ImplementationStatus: "validated",
				Summary:              "sum",
				NoFileWrites:         true,
				NoTestsExecuted:      true,
				NoToolsExecuted:      true,
			},
			wantErr: true,
		},
		{
			name: "no_file_writes false",
			suggestions: ValidatorSuggestions{
				Goal:                 "test",
				ValidationStatus:     "pending",
				ImplementationStatus: "suggestions_only",
				Summary:              "sum",
				NoFileWrites:         false,
				NoTestsExecuted:      true,
				NoToolsExecuted:      true,
			},
			wantErr: true,
		},
		{
			name: "no_tests_executed false",
			suggestions: ValidatorSuggestions{
				Goal:                 "test",
				ValidationStatus:     "pending",
				ImplementationStatus: "suggestions_only",
				Summary:              "sum",
				NoFileWrites:         true,
				NoTestsExecuted:      false,
				NoToolsExecuted:      true,
			},
			wantErr: true,
		},
		{
			name: "no_tools_executed false",
			suggestions: ValidatorSuggestions{
				Goal:                 "test",
				ValidationStatus:     "pending",
				ImplementationStatus: "suggestions_only",
				Summary:              "sum",
				NoFileWrites:         true,
				NoTestsExecuted:      true,
				NoToolsExecuted:      false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.suggestions.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateValidatorSuggestionsDangerousStatus(t *testing.T) {
	dangerousStatuses := []string{"implemented", "applied", "done", "patched", "executed", "modified", "committed", "pushed", "tested", "validated", "verified"}

	for _, status := range dangerousStatuses {
		suggestions := &ValidatorSuggestions{
			Goal:                 "test",
			ValidationStatus:     "pending",
			ImplementationStatus: status,
			Summary:              "sum",
			NoFileWrites:         true,
			NoTestsExecuted:      true,
			NoToolsExecuted:      true,
		}

		err := ValidateValidatorSuggestions(suggestions)
		if err == nil {
			t.Errorf("ValidateValidatorSuggestions() should return error for status %q", status)
		}
	}
}

func TestValidateValidatorSuggestionsNoFileWritesFalse(t *testing.T) {
	suggestions := &ValidatorSuggestions{
		Goal:                 "test",
		ValidationStatus:     "pending",
		ImplementationStatus: "suggestions_only",
		Summary:              "sum",
		NoFileWrites:         false,
		NoTestsExecuted:      true,
		NoToolsExecuted:      true,
	}

	err := ValidateValidatorSuggestions(suggestions)
	if err == nil {
		t.Error("ValidateValidatorSuggestions() should return error when no_file_writes is false")
	}
}

func TestValidateValidatorSuggestionsNoTestsExecutedFalse(t *testing.T) {
	suggestions := &ValidatorSuggestions{
		Goal:                 "test",
		ValidationStatus:     "pending",
		ImplementationStatus: "suggestions_only",
		Summary:              "sum",
		NoFileWrites:         true,
		NoTestsExecuted:      false,
		NoToolsExecuted:      true,
	}

	err := ValidateValidatorSuggestions(suggestions)
	if err == nil {
		t.Error("ValidateValidatorSuggestions() should return error when no_tests_executed is false")
	}
}

func TestValidateValidatorSuggestionsTestPassedContent(t *testing.T) {
	suggestions := &ValidatorSuggestions{
		Goal:                 "test",
		ValidationStatus:     "pending",
		ImplementationStatus: "suggestions_only",
		Summary:              "sum",
		Checks: []ValidationCheck{
			{
				ID:             "check_1",
				Category:       "unit_test",
				Description:    "test passed successfully",
				ExpectedSignal: "all tests pass",
				Priority:       "high",
			},
		},
		NoFileWrites:    true,
		NoTestsExecuted: true,
		NoToolsExecuted: true,
	}

	err := ValidateValidatorSuggestions(suggestions)
	if err == nil {
		t.Error("ValidateValidatorSuggestions() should return error for test passed content")
	}
}

func TestValidateValidatorSuggestionsCommandExecutedContent(t *testing.T) {
	suggestions := &ValidatorSuggestions{
		Goal:                 "test",
		ValidationStatus:     "pending",
		ImplementationStatus: "suggestions_only",
		Summary:              "sum",
		RecommendedCommands:  []string{"command executed successfully"},
		NoFileWrites:         true,
		NoTestsExecuted:      true,
		NoToolsExecuted:      true,
	}

	err := ValidateValidatorSuggestions(suggestions)
	if err == nil {
		t.Error("ValidateValidatorSuggestions() should return error for command executed content")
	}
}

func TestFormatValidatorSuggestions(t *testing.T) {
	suggestions := &ValidatorSuggestions{
		Goal:                 "test goal",
		ValidationStatus:     "pending",
		ImplementationStatus: "suggestions_only",
		Summary:              "test summary",
		Checks: []ValidationCheck{
			{
				ID:             "check_1",
				Category:       "unit_test",
				Description:    "run unit tests",
				ExpectedSignal: "all tests pass",
				Priority:       "high",
				RelatedFiles:   []string{"main.go"},
			},
		},
		RecommendedCommands: []string{"go test ./..."},
		ManualChecks:        []string{"check README"},
		NoFileWrites:        true,
		NoTestsExecuted:     true,
		NoToolsExecuted:     true,
	}

	output := FormatValidatorSuggestions(suggestions)

	if output == "" {
		t.Error("FormatValidatorSuggestions() returned empty string")
	}
	if !containsString(output, "test goal") {
		t.Error("FormatValidatorSuggestions() should contain goal")
	}
	if !containsString(output, "No files were modified") {
		t.Error("FormatValidatorSuggestions() should contain 'No files were modified'")
	}
	if !containsString(output, "No tests were executed") {
		t.Error("FormatValidatorSuggestions() should contain 'No tests were executed'")
	}
	if !containsString(output, "validation suggestion only") {
		t.Error("FormatValidatorSuggestions() should contain 'validation suggestion only'")
	}
}

func TestFormatValidatorSuggestionsJSON(t *testing.T) {
	suggestions := &ValidatorSuggestions{
		Goal:                 "test goal",
		ValidationStatus:     "pending",
		ImplementationStatus: "suggestions_only",
		Summary:              "test summary",
		NoFileWrites:         true,
		NoTestsExecuted:      true,
		NoToolsExecuted:      true,
	}

	jsonStr, err := FormatValidatorSuggestionsJSON(suggestions)
	if err != nil {
		t.Fatalf("FormatValidatorSuggestionsJSON() error = %v", err)
	}

	if jsonStr == "" {
		t.Error("FormatValidatorSuggestionsJSON() returned empty string")
	}

	// 验证是有效的 JSON
	var got ValidatorSuggestions
	if err := json.Unmarshal([]byte(jsonStr), &got); err != nil {
		t.Errorf("FormatValidatorSuggestionsJSON() returned invalid JSON: %v", err)
	}
}
