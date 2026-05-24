package agent

import (
	"bytes"
	"strings"
	"testing"
)

func TestDefaultApprovalPolicy_LowRisk(t *testing.T) {
	policy := DefaultApprovalPolicy()
	decision := policy.Check("file_read", "low", func() bool { return true })
	if decision != ApprovalAutoApproved {
		t.Errorf("low risk = %q, want %q", decision, ApprovalAutoApproved)
	}
}

func TestDefaultApprovalPolicy_MediumRisk(t *testing.T) {
	policy := DefaultApprovalPolicy()
	decision := policy.Check("file_write", "medium", func() bool { return true })
	if decision != ApprovalRequiresApproval {
		t.Errorf("medium risk = %q, want %q", decision, ApprovalRequiresApproval)
	}
}

func TestDefaultApprovalPolicy_HighRisk(t *testing.T) {
	policy := DefaultApprovalPolicy()
	decision := policy.Check("shell_exec", "high", func() bool { return true })
	if decision != ApprovalDenied {
		t.Errorf("high risk = %q, want %q", decision, ApprovalDenied)
	}
}

func TestDefaultApprovalPolicy_ContractDenied(t *testing.T) {
	policy := DefaultApprovalPolicy()
	decision := policy.Check("file_read", "low", func() bool { return false })
	if decision != ApprovalDenied {
		t.Errorf("contract denied = %q, want %q", decision, ApprovalDenied)
	}
}

func TestApprovalPolicy_AutoApproveMedium(t *testing.T) {
	policy := ApprovalPolicy{
		AutoApproveLowRisk:    true,
		AutoApproveMediumRisk: true,
		BlockHighRisk:         true,
	}
	decision := policy.Check("file_write", "medium", func() bool { return true })
	if decision != ApprovalAutoApproved {
		t.Errorf("medium risk with auto_approve = %q, want %q", decision, ApprovalAutoApproved)
	}
}

func TestApprovalPolicy_UnknownRisk(t *testing.T) {
	policy := DefaultApprovalPolicy()
	decision := policy.Check("unknown_tool", "critical", func() bool { return true })
	if decision != ApprovalRequiresApproval {
		t.Errorf("unknown risk = %q, want %q", decision, ApprovalRequiresApproval)
	}
}

func TestApprovalPolicy_DontBlockHighRisk(t *testing.T) {
	policy := ApprovalPolicy{
		AutoApproveLowRisk:    true,
		AutoApproveMediumRisk: false,
		BlockHighRisk:         false,
	}
	decision := policy.Check("shell_exec", "high", func() bool { return true })
	if decision != ApprovalRequiresApproval {
		t.Errorf("high risk with BlockHighRisk=false = %q, want %q", decision, ApprovalRequiresApproval)
	}
}

func TestApprovalPolicy_LowRiskNotAutoApproved(t *testing.T) {
	policy := ApprovalPolicy{
		AutoApproveLowRisk:    false,
		AutoApproveMediumRisk: false,
		BlockHighRisk:         true,
	}
	decision := policy.Check("file_read", "low", func() bool { return true })
	if decision != ApprovalRequiresApproval {
		t.Errorf("low risk not auto-approved = %q, want %q", decision, ApprovalRequiresApproval)
	}
}

func TestRequestInteractiveApproval_NonInteractive(t *testing.T) {
	policy := DefaultApprovalPolicy() // stdin is nil
	var buf bytes.Buffer
	approved := policy.RequestInteractiveApproval(&buf, "file_write", "medium", map[string]string{"path": "test.go"})
	if approved {
		t.Error("non-interactive policy should not approve")
	}
}

func TestRequestInteractiveApproval_Approve(t *testing.T) {
	policy := InteractiveApprovalPolicy(strings.NewReader("y\n"))
	var buf bytes.Buffer
	approved := policy.RequestInteractiveApproval(&buf, "file_write", "medium", map[string]string{"path": "test.go"})
	if !approved {
		t.Error("user input 'y' should approve")
	}
}

func TestRequestInteractiveApproval_Deny(t *testing.T) {
	policy := InteractiveApprovalPolicy(strings.NewReader("n\n"))
	var buf bytes.Buffer
	approved := policy.RequestInteractiveApproval(&buf, "file_write", "medium", map[string]string{"path": "test.go"})
	if approved {
		t.Error("user input 'n' should deny")
	}
}

func TestRequestInteractiveApproval_EmptyInput(t *testing.T) {
	policy := InteractiveApprovalPolicy(strings.NewReader("\n"))
	var buf bytes.Buffer
	approved := policy.RequestInteractiveApproval(&buf, "file_write", "medium", nil)
	if approved {
		t.Error("empty input should deny")
	}
}

// --- Approval args redaction tests ---

func TestFormatArgs_NoContentInCleartext(t *testing.T) {
	args := map[string]string{
		"path":    "/repo/main.go",
		"content": "package main\nfunc main() {}\n",
	}
	output := formatArgs(args)

	if strings.Contains(output, "package main") {
		t.Errorf("formatArgs should not output content in cleartext, got: %s", output)
	}
	if !strings.Contains(output, "content=<redacted>") {
		t.Errorf("formatArgs should redact content, got: %s", output)
	}
}

func TestFormatArgs_NoOldNewInCleartext(t *testing.T) {
	args := map[string]string{
		"path": "src.go",
		"old":  "old secret code",
		"new":  "new secret code",
	}
	output := formatArgs(args)

	if strings.Contains(output, "secret") {
		t.Errorf("formatArgs should not output old/new in cleartext, got: %s", output)
	}
	if !strings.Contains(output, "old=<redacted>") {
		t.Errorf("formatArgs should redact old, got: %s", output)
	}
	if !strings.Contains(output, "new=<redacted>") {
		t.Errorf("formatArgs should redact new, got: %s", output)
	}
}

func TestFormatArgs_PathRetained(t *testing.T) {
	args := map[string]string{
		"path":    "/repo/README.md",
		"content": "sensitive",
	}
	output := formatArgs(args)

	if !strings.Contains(output, "path=/repo/README.md") {
		t.Errorf("formatArgs should retain path, got: %s", output)
	}
}

func TestFormatArgs_CommandNameRetained(t *testing.T) {
	args := map[string]string{
		"command_name": "go test ./...",
		"env_vars":     "SECRET=leaked",
	}
	output := formatArgs(args)

	if !strings.Contains(output, "command_name=go test ./...") {
		t.Errorf("formatArgs should retain command_name, got: %s", output)
	}
	if !strings.Contains(output, "env_vars=<redacted>") {
		t.Errorf("formatArgs should redact env_vars, got: %s", output)
	}
}

func TestFormatArgs_MaxBytesRetained(t *testing.T) {
	args := map[string]string{
		"path":      "/repo/file.go",
		"max_bytes": "4096",
	}
	output := formatArgs(args)

	if !strings.Contains(output, "max_bytes=4096") {
		t.Errorf("formatArgs should retain max_bytes, got: %s", output)
	}
}

func TestFormatArgs_CreateDirsRetained(t *testing.T) {
	args := map[string]string{
		"path":        "/repo/newdir",
		"create_dirs": "true",
	}
	output := formatArgs(args)

	if !strings.Contains(output, "create_dirs=true") {
		t.Errorf("formatArgs should retain create_dirs, got: %s", output)
	}
}

func TestFormatArgs_EmptyArgs(t *testing.T) {
	output := formatArgs(nil)
	if output != "(none)" {
		t.Errorf("formatArgs(nil) = %q, want %q", output, "(none)")
	}
}

func TestInteractiveApproval_NoLeakOldNewContent(t *testing.T) {
	policy := InteractiveApprovalPolicy(strings.NewReader("n\n"))
	var buf bytes.Buffer

	args := map[string]string{
		"path":    "secret.go",
		"content": "TOP_SECRET_CODE_HERE",
		"old":     "OLD_SECRET",
		"new":     "NEW_SECRET",
	}
	policy.RequestInteractiveApproval(&buf, "file_write", "medium", args)

	output := buf.String()
	if strings.Contains(output, "TOP_SECRET") {
		t.Errorf("interactive prompt should not leak content, got: %s", output)
	}
	if strings.Contains(output, "OLD_SECRET") || strings.Contains(output, "NEW_SECRET") {
		t.Errorf("interactive prompt should not leak old/new, got: %s", output)
	}
	if !strings.Contains(output, "<redacted>") {
		t.Errorf("interactive prompt should show <redacted> for sensitive args, got: %s", output)
	}
	// path should be visible
	if !strings.Contains(output, "secret.go") {
		t.Errorf("interactive prompt should show path, got: %s", output)
	}
}
