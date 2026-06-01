// Package security provides security enforcement for MioNeko.
//
// This file implements configurable security enforcement policies.
// Default mode is 'warn' - detection with warnings, no blocking.
package security

import (
	"fmt"
	"os"
	"strings"
)

// EnforcementMode represents the security enforcement level.
type EnforcementMode string

const (
	// ModeOff disables enforcement. Only audit candidate events are recorded.
	ModeOff EnforcementMode = "off"

	// ModeWarn enables warnings but does not block. Emits security.warning events.
	ModeWarn EnforcementMode = "warn"

	// ModeEnforce enables blocking for critical paths and high-risk tools.
	ModeEnforce EnforcementMode = "enforce"
)

// DefaultEnforcementMode is the default enforcement mode.
const DefaultEnforcementMode = ModeWarn

// SecurityModeEnvVar is the environment variable for setting the enforcement mode.
const SecurityModeEnvVar = "MIMONEKO_SECURITY_MODE"

// EnforcementResult represents the result of an enforcement check.
type EnforcementResult struct {
	// Allowed indicates whether the operation is allowed to proceed.
	Allowed bool

	// Mode is the current enforcement mode.
	Mode EnforcementMode

	// Reason explains why the operation was allowed/denied.
	Reason string

	// RequiresApproval indicates if the operation needs approval.
	RequiresApproval bool

	// Violations contains any path violations detected.
	Violations []PathViolation

	// EventType is the event type to emit (empty if no event needed).
	EventType string

	// ShouldWarn indicates if a warning should be displayed.
	ShouldWarn bool
}

// GetEnforcementMode returns the current enforcement mode from environment
// or the default.
func GetEnforcementMode() EnforcementMode {
	mode := strings.TrimSpace(os.Getenv(SecurityModeEnvVar))
	switch EnforcementMode(mode) {
	case ModeOff:
		return ModeOff
	case ModeEnforce:
		return ModeEnforce
	case ModeWarn:
		return ModeWarn
	default:
		return DefaultEnforcementMode
	}
}

// ValidateEnforcementMode checks if a mode string is valid.
func ValidateEnforcementMode(mode string) bool {
	switch EnforcementMode(mode) {
	case ModeOff, ModeWarn, ModeEnforce:
		return true
	default:
		return false
	}
}

// EnforcementConfig holds the enforcement configuration for a tool runtime.
type EnforcementConfig struct {
	Mode EnforcementMode
}

// NewEnforcementConfig creates a new enforcement config with the current mode.
func NewEnforcementConfig() *EnforcementConfig {
	return &EnforcementConfig{
		Mode: GetEnforcementMode(),
	}
}

// CheckToolExecution checks if a tool execution should be allowed based on
// the enforcement mode and tool metadata.
//
// Parameters:
//   - toolName: name of the tool being executed
//   - riskLevel: risk level from tool metadata (low/medium/high/critical)
//   - requiresApproval: whether the tool requires approval
//   - pathArgs: map of argument keys that contain paths
//   - args: tool arguments
//
// Returns:
//   - EnforcementResult with the decision
func (cfg *EnforcementConfig) CheckToolExecution(
	toolName string,
	riskLevel string,
	requiresApproval bool,
	pathArgKeys []string,
	args map[string]string,
) EnforcementResult {
	result := EnforcementResult{
		Allowed: true,
		Mode:    cfg.Mode,
	}

	// Check path violations first
	for _, key := range pathArgKeys {
		pathValue, ok := args[key]
		if !ok || pathValue == "" {
			continue
		}

		violations := ValidatePath(pathValue)
		if len(violations) == 0 {
			continue
		}

		result.Violations = append(result.Violations, violations...)

		// Process violations based on mode
		for _, v := range violations {
			pathResult := cfg.checkPathViolation(v)
			if !pathResult.Allowed {
				result.Allowed = false
				result.Reason = pathResult.Reason
				result.EventType = pathResult.EventType
				return result
			}
			if pathResult.ShouldWarn {
				result.ShouldWarn = true
				if result.EventType == "" {
					result.EventType = pathResult.EventType
				}
			}
		}
	}

	// Check tool risk level
	toolResult := cfg.checkToolRisk(toolName, riskLevel, requiresApproval)
	if !toolResult.Allowed {
		result.Allowed = false
		result.Reason = toolResult.Reason
		result.EventType = toolResult.EventType
		result.RequiresApproval = toolResult.RequiresApproval
		return result
	}
	if toolResult.ShouldWarn {
		result.ShouldWarn = true
		if result.EventType == "" {
			result.EventType = toolResult.EventType
		}
	}
	result.RequiresApproval = toolResult.RequiresApproval

	return result
}

// checkPathViolation checks a single path violation against the enforcement mode.
func (cfg *EnforcementConfig) checkPathViolation(v PathViolation) EnforcementResult {
	result := EnforcementResult{
		Allowed: true,
		Mode:    cfg.Mode,
	}

	switch cfg.Mode {
	case ModeOff:
		// Only record candidate, no warning
		result.EventType = "path.violation_candidate"

	case ModeWarn:
		// Allow but warn
		result.ShouldWarn = true
		result.EventType = "security.warning"
		result.Reason = fmt.Sprintf("sensitive path detected: %s (%s)", v.Path, v.Rule)

	case ModeEnforce:
		if v.Severity == SeverityCritical {
			// Block critical paths
			result.Allowed = false
			result.EventType = "path.blocked"
			result.Reason = fmt.Sprintf("critical path blocked: %s (%s)", v.Path, v.Rule)
		} else {
			// Allow warning-level paths but emit warning
			result.ShouldWarn = true
			result.EventType = "security.warning"
			result.Reason = fmt.Sprintf("sensitive path detected: %s (%s)", v.Path, v.Rule)
		}
	}

	return result
}

// checkToolRisk checks tool risk level against the enforcement mode.
func (cfg *EnforcementConfig) checkToolRisk(
	toolName string,
	riskLevel string,
	requiresApproval bool,
) EnforcementResult {
	result := EnforcementResult{
		Allowed: true,
		Mode:    cfg.Mode,
	}

	switch cfg.Mode {
	case ModeOff, ModeWarn:
		// Allow all tools in off/warn mode
		if cfg.Mode == ModeWarn && (riskLevel == "high" || riskLevel == "critical") {
			result.ShouldWarn = true
			result.EventType = "security.warning"
			result.Reason = fmt.Sprintf("high-risk tool: %s (risk: %s)", toolName, riskLevel)
		}

	case ModeEnforce:
		switch riskLevel {
		case "low":
			// Always allowed
			result.Allowed = true

		case "medium":
			if requiresApproval {
				// Require approval
				result.Allowed = false
				result.RequiresApproval = true
				result.EventType = "tool.approval_required"
				result.Reason = fmt.Sprintf("tool requires approval: %s", toolName)
			}
			// Medium without approval is allowed

		case "high":
			// Always requires approval
			result.Allowed = false
			result.RequiresApproval = true
			result.EventType = "tool.approval_required"
			result.Reason = fmt.Sprintf("high-risk tool requires approval: %s", toolName)

		case "critical":
			// Deny by default
			result.Allowed = false
			result.EventType = "tool.denied"
			result.Reason = fmt.Sprintf("critical-risk tool denied: %s", toolName)

		default:
			// Unknown risk level, allow but warn
			result.ShouldWarn = true
			result.EventType = "security.warning"
			result.Reason = fmt.Sprintf("unknown risk level for tool: %s", toolName)
		}
	}

	return result
}

// SecurityStatus holds the current security configuration status.
type SecurityStatus struct {
	Mode               EnforcementMode
	RegisteredTools    int
	HighRiskTools      []string
	CriticalRiskTools  []string
	ApprovalRequired   []string
	SandboxRulesCount  int
	EnforcementEnabled bool
}

// GetSecurityStatus returns the current security status.
func GetSecurityStatus(toolCount int, highRisk, criticalRisk, approvalRequired []string) SecurityStatus {
	mode := GetEnforcementMode()
	return SecurityStatus{
		Mode:               mode,
		RegisteredTools:    toolCount,
		HighRiskTools:      highRisk,
		CriticalRiskTools:  criticalRisk,
		ApprovalRequired:   approvalRequired,
		SandboxRulesCount:  len(sensitiveRules),
		EnforcementEnabled: mode == ModeEnforce,
	}
}

// SecuritySummary provides a comprehensive summary of the security configuration.
type SecuritySummary struct {
	TotalRegisteredTools int             `json:"total_registered_tools"`
	HighRiskTools        []string        `json:"high_risk_tools"`
	CriticalRiskTools    []string        `json:"critical_risk_tools"`
	ApprovalRequired     []string        `json:"approval_required_tools"`
	BlockedRules         []string        `json:"blocked_rules"`
	EnforcementMode      EnforcementMode `json:"enforcement_mode"`
	SandboxRulesCount    int             `json:"sandbox_rules_count"`
	EnforcementEnabled   bool            `json:"enforcement_enabled"`
}

// GetSecuritySummary returns a comprehensive security summary.
func GetSecuritySummary(toolCount int, highRisk, criticalRisk, approvalRequired []string) SecuritySummary {
	mode := GetEnforcementMode()

	// Collect all rule names
	ruleSet := make(map[string]bool)
	for _, rule := range sensitiveRules {
		ruleSet[rule.rule] = true
	}
	var rules []string
	for rule := range ruleSet {
		rules = append(rules, rule)
	}

	return SecuritySummary{
		TotalRegisteredTools: toolCount,
		HighRiskTools:        highRisk,
		CriticalRiskTools:    criticalRisk,
		ApprovalRequired:     approvalRequired,
		BlockedRules:         rules,
		EnforcementMode:      mode,
		SandboxRulesCount:    len(sensitiveRules),
		EnforcementEnabled:   mode == ModeEnforce,
	}
}
