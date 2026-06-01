package agents

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ImplementationStatusSuggestionsOnly 是 Validator 的唯一允许状态
const ImplementationStatusSuggestionsOnly = "suggestions_only"

// DangerousValidationStatuses 是 Validator 必须拒绝的状态
var DangerousValidationStatuses = []string{
	"implemented",
	"applied",
	"done",
	"patched",
	"executed",
	"modified",
	"committed",
	"pushed",
	"tested",
	"validated",
	"verified",
}

// DangerousValidationContentPatterns 是危险内容模式
var DangerousValidationContentPatterns = []string{
	"test passed",
	"tests passed",
	"executed",
	"command executed",
	"validation complete",
	"verification complete",
	"all tests passed",
	"diff --git",
	"unified diff",
	"git apply",
	"patch applied",
}

// ValidatorSuggestions 表示 Validator 生成的验证建议
type ValidatorSuggestions struct {
	Goal                 string            `json:"goal"`
	ValidationStatus     string            `json:"validation_status"`
	ImplementationStatus string            `json:"implementation_status"`
	Summary              string            `json:"summary"`
	Checks               []ValidationCheck `json:"checks"`
	Risks                []string          `json:"risks"`
	RecommendedCommands  []string          `json:"recommended_commands"`
	ManualChecks         []string          `json:"manual_checks"`
	NoFileWrites         bool              `json:"no_file_writes"`
	NoTestsExecuted      bool              `json:"no_tests_executed"`
	NoToolsExecuted      bool              `json:"no_tools_executed"`
}

// ValidationCheck 表示一个验证检查项
type ValidationCheck struct {
	ID             string   `json:"id"`
	Category       string   `json:"category"`
	Description    string   `json:"description"`
	ExpectedSignal string   `json:"expected_signal"`
	Priority       string   `json:"priority"`
	RelatedFiles   []string `json:"related_files"`
}

// Validate 检查 suggestions 是否有所有必需字段且安全
func (v *ValidatorSuggestions) Validate() error {
	if v.Goal == "" {
		return fmt.Errorf("validator: goal is required")
	}
	if v.ValidationStatus == "" {
		return fmt.Errorf("validator: validation_status is required")
	}
	if v.Summary == "" {
		return fmt.Errorf("validator: summary is required")
	}
	if v.ImplementationStatus != ImplementationStatusSuggestionsOnly {
		return fmt.Errorf("validator: implementation_status must be %q, got %q", ImplementationStatusSuggestionsOnly, v.ImplementationStatus)
	}
	if !v.NoFileWrites {
		return fmt.Errorf("validator: no_file_writes must be true")
	}
	if !v.NoTestsExecuted {
		return fmt.Errorf("validator: no_tests_executed must be true")
	}
	if !v.NoToolsExecuted {
		return fmt.Errorf("validator: no_tools_executed must be true")
	}
	return nil
}

// ParseValidatorSuggestionsResponse 解析 LLM 响应为 ValidatorSuggestions
func ParseValidatorSuggestionsResponse(text string) (*ValidatorSuggestions, error) {
	// 尝试从 markdown code block 提取 JSON
	jsonStr := extractJSON(text)

	// 解析 JSON
	var suggestions ValidatorSuggestions
	if err := json.Unmarshal([]byte(jsonStr), &suggestions); err != nil {
		return nil, fmt.Errorf("validator: invalid JSON: %w", err)
	}

	// 强制安全值
	suggestions.ImplementationStatus = ImplementationStatusSuggestionsOnly
	suggestions.NoFileWrites = true
	suggestions.NoTestsExecuted = true
	suggestions.NoToolsExecuted = true

	// 验证
	if err := suggestions.Validate(); err != nil {
		return nil, err
	}

	return &suggestions, nil
}

// ValidateValidatorSuggestions 验证 ValidatorSuggestions 是否安全
func ValidateValidatorSuggestions(suggestions *ValidatorSuggestions) error {
	// 检查 implementation_status
	for _, dangerous := range DangerousValidationStatuses {
		if strings.EqualFold(suggestions.ImplementationStatus, dangerous) {
			return fmt.Errorf("validator: dangerous implementation_status %q detected, must be %q", suggestions.ImplementationStatus, ImplementationStatusSuggestionsOnly)
		}
	}

	// 检查 no_file_writes
	if !suggestions.NoFileWrites {
		return fmt.Errorf("validator: no_file_writes must be true")
	}

	// 检查 no_tests_executed
	if !suggestions.NoTestsExecuted {
		return fmt.Errorf("validator: no_tests_executed must be true")
	}

	// 检查 no_tools_executed
	if !suggestions.NoToolsExecuted {
		return fmt.Errorf("validator: no_tools_executed must be true")
	}

	// 检查 checks 中的危险内容
	for _, check := range suggestions.Checks {
		if err := validateValidationCheck(check); err != nil {
			return err
		}
	}

	// 检查 recommended_commands 中的危险内容
	for _, cmd := range suggestions.RecommendedCommands {
		content := strings.ToLower(cmd)
		for _, pattern := range DangerousValidationContentPatterns {
			if strings.Contains(content, strings.ToLower(pattern)) {
				return fmt.Errorf("validator: dangerous content pattern %q detected in recommended_commands", pattern)
			}
		}
	}

	return nil
}

// validateValidationCheck 检查 validation check 不包含危险模式
func validateValidationCheck(check ValidationCheck) error {
	content := strings.ToLower(check.Description + " " + check.ExpectedSignal)
	for _, pattern := range DangerousValidationContentPatterns {
		if strings.Contains(content, strings.ToLower(pattern)) {
			return fmt.Errorf("validator: dangerous content pattern %q detected in check %q", pattern, check.ID)
		}
	}
	return nil
}

// FormatValidatorSuggestions 格式化 ValidatorSuggestions 用于显示
func FormatValidatorSuggestions(suggestions *ValidatorSuggestions) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "Validator Suggestions:\n")
	fmt.Fprintf(&buf, "  Goal: %s\n", suggestions.Goal)
	fmt.Fprintf(&buf, "  Status: %s\n", suggestions.ImplementationStatus)
	fmt.Fprintf(&buf, "  Validation: %s\n", suggestions.ValidationStatus)
	fmt.Fprintf(&buf, "\n")

	fmt.Fprintf(&buf, "Summary:\n")
	fmt.Fprintf(&buf, "  %s\n", suggestions.Summary)

	if len(suggestions.Checks) > 0 {
		fmt.Fprintf(&buf, "\nChecks:\n")
		for i, check := range suggestions.Checks {
			fmt.Fprintf(&buf, "  %d. [%s] %s\n", i+1, check.Priority, check.Category)
			fmt.Fprintf(&buf, "     %s\n", check.Description)
			fmt.Fprintf(&buf, "     Expected: %s\n", check.ExpectedSignal)
			if len(check.RelatedFiles) > 0 {
				fmt.Fprintf(&buf, "     Files: %s\n", strings.Join(check.RelatedFiles, ", "))
			}
		}
	}

	if len(suggestions.RecommendedCommands) > 0 {
		fmt.Fprintf(&buf, "\nRecommended Commands:\n")
		for i, cmd := range suggestions.RecommendedCommands {
			fmt.Fprintf(&buf, "  %d. %s\n", i+1, cmd)
		}
	}

	if len(suggestions.ManualChecks) > 0 {
		fmt.Fprintf(&buf, "\nManual Checks:\n")
		for i, check := range suggestions.ManualChecks {
			fmt.Fprintf(&buf, "  %d. %s\n", i+1, check)
		}
	}

	if len(suggestions.Risks) > 0 {
		fmt.Fprintf(&buf, "\nRisks:\n")
		for _, risk := range suggestions.Risks {
			fmt.Fprintf(&buf, "  - %s\n", risk)
		}
	}

	fmt.Fprintf(&buf, "\nNo files were modified.\n")
	fmt.Fprintf(&buf, "No tests were executed.\n")
	fmt.Fprintf(&buf, "No tools were executed.\n")
	fmt.Fprintf(&buf, "This is a validation suggestion only.\n")

	return buf.String()
}

// FormatValidatorSuggestionsJSON 格式化 ValidatorSuggestions 为 JSON
func FormatValidatorSuggestionsJSON(suggestions *ValidatorSuggestions) (string, error) {
	data, err := json.MarshalIndent(suggestions, "", "  ")
	if err != nil {
		return "", fmt.Errorf("validator: marshal JSON: %w", err)
	}
	return string(data), nil
}
