package agent

import (
	"fmt"
	"io"
	"strings"
)

// ApprovalDecision represents the outcome of an approval check.
type ApprovalDecision string

const (
	// ApprovalAutoApproved means the tool call is approved automatically.
	ApprovalAutoApproved ApprovalDecision = "auto_approved"

	// ApprovalRequiresApproval means the tool call needs human approval.
	ApprovalRequiresApproval ApprovalDecision = "requires_approval"

	// ApprovalDenied means the tool call is denied.
	ApprovalDenied ApprovalDecision = "denied"
)

// ApprovalPolicy determines whether a tool call requires human approval.
//
// Risk-based rules:
//   - "low" risk tools are auto-approved.
//   - "medium" risk tools require approval unless auto_approve_medium is true.
//   - "high" risk tools always require approval.
//
// Additional rules:
//   - Tools not in the contract's AllowedTools list are denied.
//   - Tools in the contract's DeniedTools list are denied.
type ApprovalPolicy struct {
	// AutoApproveLowRisk auto-approves low-risk tools.
	AutoApproveLowRisk bool

	// AutoApproveMediumRisk auto-approves medium-risk tools.
	// By default this is false; medium-risk tools require approval.
	AutoApproveMediumRisk bool

	// BlockHighRisk always blocks high-risk tools.
	// By default this is true.
	BlockHighRisk bool

	// stdin is used to read user input for interactive approval.
	// If nil, approval requests are automatically denied.
	stdin io.Reader
}

// DefaultApprovalPolicy returns a safe default ApprovalPolicy.
//   - Low risk: auto-approved
//   - Medium risk: requires approval
//   - High risk: blocked
func DefaultApprovalPolicy() ApprovalPolicy {
	return ApprovalPolicy{
		AutoApproveLowRisk:    true,
		AutoApproveMediumRisk: false,
		BlockHighRisk:         true,
		stdin:                 nil, // non-interactive: deny by default
	}
}

// InteractiveApprovalPolicy returns an ApprovalPolicy that reads from stdin
// for interactive approval prompts.
func InteractiveApprovalPolicy(stdin io.Reader) ApprovalPolicy {
	return ApprovalPolicy{
		AutoApproveLowRisk:    true,
		AutoApproveMediumRisk: false,
		BlockHighRisk:         true,
		stdin:                 stdin,
	}
}

// Check evaluates whether a tool call should be approved, requires approval,
// or is denied based on the tool's risk level and the contract.
func (p ApprovalPolicy) Check(toolName, riskLevel string, contractCheck func() bool) ApprovalDecision {
	// First check contract-level denial
	if contractCheck != nil && !contractCheck() {
		return ApprovalDenied
	}

	switch riskLevel {
	case "low":
		if p.AutoApproveLowRisk {
			return ApprovalAutoApproved
		}
		return ApprovalRequiresApproval

	case "medium":
		if p.AutoApproveMediumRisk {
			return ApprovalAutoApproved
		}
		return ApprovalRequiresApproval

	case "high":
		if p.BlockHighRisk {
			return ApprovalDenied
		}
		return ApprovalRequiresApproval

	default:
		// Unknown risk level: require approval
		return ApprovalRequiresApproval
	}
}

// RequestInteractiveApproval prompts the user for approval and returns
// whether the tool call is approved.
//
// If stdin is nil (non-interactive mode), it returns false.
func (p ApprovalPolicy) RequestInteractiveApproval(stdout io.Writer, toolName, riskLevel string, args map[string]string) bool {
	if p.stdin == nil {
		return false
	}

	fmt.Fprintf(stdout, "\n[APPROVAL REQUIRED] Tool: %s (risk: %s)\n", toolName, riskLevel)
	fmt.Fprintf(stdout, "  Args: %v\n", formatArgs(args))
	fmt.Fprintf(stdout, "  Approve? [y/N]: ")

	var response string
	fmt.Fscanln(p.stdin, &response)

	return strings.ToLower(strings.TrimSpace(response)) == "y" ||
		strings.ToLower(strings.TrimSpace(response)) == "yes"
}

func formatArgs(args map[string]string) string {
	if len(args) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(args))
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%s", k, redactArgValue(k, v)))
	}
	return strings.Join(parts, ", ")
}

// safeArgKeys are the keys allowed in cleartext in approval prompts.
var safeArgKeys = map[string]bool{
	"path":         true,
	"command_name": true,
	"max_bytes":    true,
	"create_dirs":  true,
}

// redactArgValue returns "<redacted>" for keys not in the safe list.
func redactArgValue(key, value string) string {
	if safeArgKeys[key] {
		return value
	}
	return "<redacted>"
}
