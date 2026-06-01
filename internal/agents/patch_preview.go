package agents

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mimoneko/mimoneko/internal/security"
)

// ImplementationStatusPreviewOnly 是 PatchPreview 的唯一允许状态
const ImplementationStatusPreviewOnly = "preview_only"

// PatchPreviewSource 表示 preview 的来源
type PatchPreviewSource string

const (
	PatchPreviewSourceIntent PatchPreviewSource = "intent"
	PatchPreviewSourceReport PatchPreviewSource = "dry-run report"
)

// PatchPreview 表示一个 patch 预览
type PatchPreview struct {
	ID                  string              `json:"id"`
	WorkflowID          string              `json:"workflow_id"`
	Goal                string              `json:"goal"`
	Source              PatchPreviewSource  `json:"source"`
	ImplementationStatus string             `json:"implementation_status"`
	Files               []PatchPreviewFile  `json:"files"`
	Changes             []PatchPreviewChange `json:"changes"`
	Risks               []string            `json:"risks"`
	ValidationSuggestions []string          `json:"validation_suggestions"`
	NoFileWrites        bool                `json:"no_file_writes"`
	NoPatchApplied      bool                `json:"no_patch_applied"`
	NoToolsExecuted     bool                `json:"no_tools_executed"`
	CreatedAt           time.Time           `json:"created_at"`
}

// PatchPreviewFile 表示 preview 中的一个文件
type PatchPreviewFile struct {
	Path           string `json:"path"`
	ChangeType     string `json:"change_type"`
	Reason         string `json:"reason"`
	RiskLevel      string `json:"risk_level"`
	PreviewSummary string `json:"preview_summary"`
}

// PatchPreviewChange 表示 preview 中的一个变更
type PatchPreviewChange struct {
	ID             string `json:"id"`
	FilePath       string `json:"file_path"`
	Description    string `json:"description"`
	ExpectedEffect string `json:"expected_effect"`
	SafetyNotes    string `json:"safety_notes"`
}

// Validate 检查 preview 是否满足安全约束
func (p *PatchPreview) Validate() error {
	if p.ImplementationStatus != ImplementationStatusPreviewOnly {
		return fmt.Errorf("preview: implementation_status must be %q, got %q", ImplementationStatusPreviewOnly, p.ImplementationStatus)
	}
	if !p.NoFileWrites {
		return fmt.Errorf("preview: no_file_writes must be true")
	}
	if !p.NoPatchApplied {
		return fmt.Errorf("preview: no_patch_applied must be true")
	}
	if !p.NoToolsExecuted {
		return fmt.Errorf("preview: no_tools_executed must be true")
	}
	return nil
}

// GeneratePreviewFromIntent 从 CoderPatchIntent 生成 preview
func GeneratePreviewFromIntent(intent *CoderPatchIntent) (*PatchPreview, error) {
	// 验证 intent
	if intent.ImplementationStatus != ImplementationStatusIntentOnly {
		return nil, fmt.Errorf("preview: intent implementation_status must be %q, got %q", ImplementationStatusIntentOnly, intent.ImplementationStatus)
	}
	if !intent.NoFileWrites {
		return nil, fmt.Errorf("preview: intent no_file_writes must be true")
	}

	// 生成 preview ID
	previewID, err := generatePreviewID()
	if err != nil {
		return nil, err
	}

	// 转换文件
	files := make([]PatchPreviewFile, len(intent.FilesToChange))
	for i, f := range intent.FilesToChange {
		files[i] = PatchPreviewFile{
			Path:           security.SanitizeText(f.Path),
			ChangeType:     f.ChangeType,
			Reason:         security.SanitizeText(f.Reason),
			RiskLevel:      f.RiskLevel,
			PreviewSummary: fmt.Sprintf("%s %s", f.ChangeType, security.SanitizeText(f.Path)),
		}
	}

	// 转换变更
	changes := make([]PatchPreviewChange, len(intent.Changes))
	for i, c := range intent.Changes {
		changes[i] = PatchPreviewChange{
			ID:             c.ID,
			FilePath:       security.SanitizeText(c.FilePath),
			Description:    security.SanitizeText(c.Description),
			ExpectedEffect: security.SanitizeText(c.ExpectedEffect),
			SafetyNotes:    security.SanitizeText(c.SafetyNotes),
		}
	}

	preview := &PatchPreview{
		ID:                    previewID,
		Goal:                  security.SanitizeText(intent.Goal),
		Source:                PatchPreviewSourceIntent,
		ImplementationStatus:  ImplementationStatusPreviewOnly,
		Files:                 files,
		Changes:               changes,
		Risks:                 sanitizeStrings(intent.Risks),
		ValidationSuggestions: sanitizeStrings(intent.ValidationSuggestions),
		NoFileWrites:          true,
		NoPatchApplied:        true,
		NoToolsExecuted:       true,
		CreatedAt:             time.Now().UTC(),
	}

	return preview, nil
}

// GeneratePreviewFromReport 从 AgentDryRunReport 生成 preview
func GeneratePreviewFromReport(report *AgentDryRunReport) (*PatchPreview, error) {
	// 验证 report
	if report.CoderIntent == nil {
		return nil, fmt.Errorf("preview: report missing coder_intent")
	}
	if !report.NoFileWrites {
		return nil, fmt.Errorf("preview: report no_file_writes must be true")
	}
	if !report.NoPatchGenerated {
		return nil, fmt.Errorf("preview: report no_patch_generated must be true")
	}
	if !report.NoToolsExecuted {
		return nil, fmt.Errorf("preview: report no_tools_executed must be true")
	}
	if !report.NoTestsExecuted {
		return nil, fmt.Errorf("preview: report no_tests_executed must be true")
	}

	// 生成 preview
	preview, err := GeneratePreviewFromIntent(report.CoderIntent)
	if err != nil {
		return nil, err
	}

	// 设置 workflow ID 和来源
	preview.WorkflowID = report.WorkflowID
	preview.Source = PatchPreviewSourceReport

	return preview, nil
}

// FormatPatchPreview 格式化 PatchPreview 用于显示
func FormatPatchPreview(preview *PatchPreview) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "Patch Preview:\n")
	fmt.Fprintf(&buf, "  Status: %s\n", preview.ImplementationStatus)
	fmt.Fprintf(&buf, "  Source: %s\n", preview.Source)
	if preview.WorkflowID != "" {
		fmt.Fprintf(&buf, "  Workflow: %s\n", preview.WorkflowID)
	}
	fmt.Fprintf(&buf, "  Goal: %s\n", preview.Goal)
	fmt.Fprintf(&buf, "\n")

	if len(preview.Files) > 0 {
		fmt.Fprintf(&buf, "Files:\n")
		for _, f := range preview.Files {
			fmt.Fprintf(&buf, "  - %s\n", f.Path)
			fmt.Fprintf(&buf, "    change_type: %s\n", f.ChangeType)
			fmt.Fprintf(&buf, "    reason: %s\n", f.Reason)
			fmt.Fprintf(&buf, "    risk: %s\n", f.RiskLevel)
		}
		fmt.Fprintf(&buf, "\n")
	}

	if len(preview.Changes) > 0 {
		fmt.Fprintf(&buf, "Changes:\n")
		for i, c := range preview.Changes {
			fmt.Fprintf(&buf, "  %d. %s\n", i+1, c.Description)
			fmt.Fprintf(&buf, "     File: %s\n", c.FilePath)
			fmt.Fprintf(&buf, "     Effect: %s\n", c.ExpectedEffect)
			if c.SafetyNotes != "" {
				fmt.Fprintf(&buf, "     Safety: %s\n", c.SafetyNotes)
			}
		}
		fmt.Fprintf(&buf, "\n")
	}

	if len(preview.Risks) > 0 {
		fmt.Fprintf(&buf, "Risks:\n")
		for _, risk := range preview.Risks {
			fmt.Fprintf(&buf, "  - %s\n", risk)
		}
		fmt.Fprintf(&buf, "\n")
	}

	if len(preview.ValidationSuggestions) > 0 {
		fmt.Fprintf(&buf, "Validation Suggestions:\n")
		for _, suggestion := range preview.ValidationSuggestions {
			fmt.Fprintf(&buf, "  - %s\n", suggestion)
		}
		fmt.Fprintf(&buf, "\n")
	}

	fmt.Fprintf(&buf, "No files were modified.\n")
	fmt.Fprintf(&buf, "No patch was applied.\n")
	fmt.Fprintf(&buf, "No tools were executed.\n")

	return buf.String()
}

// FormatPatchPreviewJSON 格式化 PatchPreview 为 JSON
func FormatPatchPreviewJSON(preview *PatchPreview) (string, error) {
	data, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		return "", fmt.Errorf("preview: marshal JSON: %w", err)
	}
	return string(data), nil
}

// sanitizeStrings 脱敏字符串切片
func sanitizeStrings(strs []string) []string {
	result := make([]string, len(strs))
	for i, s := range strs {
		result[i] = security.SanitizeText(s)
	}
	return result
}

// generatePreviewID 生成 preview ID
func generatePreviewID() (string, error) {
	id := generateWorkflowID()
	return "preview_" + strings.TrimPrefix(id, "wf_"), nil
}
