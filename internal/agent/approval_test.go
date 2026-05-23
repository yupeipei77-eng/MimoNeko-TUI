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
