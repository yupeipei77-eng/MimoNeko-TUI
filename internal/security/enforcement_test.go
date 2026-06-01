package security

import (
	"os"
	"testing"
)

func TestDefaultModeIsWarn(t *testing.T) {
	// Clear any existing env var
	os.Unsetenv(SecurityModeEnvVar)
	defer os.Unsetenv(SecurityModeEnvVar)

	mode := GetEnforcementMode()
	if mode != ModeWarn {
		t.Errorf("GetEnforcementMode() = %q, want %q", mode, ModeWarn)
	}
}

func TestModeFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want EnforcementMode
	}{
		{name: "off", env: "off", want: ModeOff},
		{name: "warn", env: "warn", want: ModeWarn},
		{name: "enforce", env: "enforce", want: ModeEnforce},
		{name: "empty defaults to warn", env: "", want: ModeWarn},
		{name: "invalid defaults to warn", env: "invalid", want: ModeWarn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env == "" {
				os.Unsetenv(SecurityModeEnvVar)
			} else {
				os.Setenv(SecurityModeEnvVar, tt.env)
			}
			defer os.Unsetenv(SecurityModeEnvVar)

			got := GetEnforcementMode()
			if got != tt.want {
				t.Errorf("GetEnforcementMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOffModeAllowsCriticalPath(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeOff}

	result := cfg.CheckToolExecution(
		"file_read",
		"low",
		false,
		[]string{"path"},
		map[string]string{"path": ".git/config"},
	)

	if !result.Allowed {
		t.Errorf("off mode should allow critical path, got allowed=%v", result.Allowed)
	}
}

func TestWarnModeAllowsCriticalPathButEmitsWarning(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeWarn}

	result := cfg.CheckToolExecution(
		"file_read",
		"low",
		false,
		[]string{"path"},
		map[string]string{"path": ".git/config"},
	)

	if !result.Allowed {
		t.Errorf("warn mode should allow critical path, got allowed=%v", result.Allowed)
	}
	if !result.ShouldWarn {
		t.Errorf("warn mode should warn for critical path, got shouldWarn=%v", result.ShouldWarn)
	}
	if result.EventType != "security.warning" {
		t.Errorf("warn mode should emit security.warning, got eventType=%q", result.EventType)
	}
}

func TestEnforceModeBlocksCriticalPath(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}

	result := cfg.CheckToolExecution(
		"file_read",
		"low",
		false,
		[]string{"path"},
		map[string]string{"path": ".git/config"},
	)

	if result.Allowed {
		t.Errorf("enforce mode should block critical path, got allowed=%v", result.Allowed)
	}
	if result.EventType != "path.blocked" {
		t.Errorf("enforce mode should emit path.blocked, got eventType=%q", result.EventType)
	}
}

func TestEnforceModeApprovalRequiredForHighRiskTool(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}

	result := cfg.CheckToolExecution(
		"file_write",
		"high",
		false,
		nil,
		nil,
	)

	if result.Allowed {
		t.Errorf("enforce mode should deny high-risk tool, got allowed=%v", result.Allowed)
	}
	if !result.RequiresApproval {
		t.Errorf("enforce mode should require approval for high-risk tool, got requiresApproval=%v", result.RequiresApproval)
	}
	if result.EventType != "tool.approval_required" {
		t.Errorf("should emit tool.approval_required, got eventType=%q", result.EventType)
	}
}

func TestEnforceModeDeniesCriticalRiskTool(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}

	result := cfg.CheckToolExecution(
		"dangerous_tool",
		"critical",
		false,
		nil,
		nil,
	)

	if result.Allowed {
		t.Errorf("enforce mode should deny critical-risk tool, got allowed=%v", result.Allowed)
	}
	if result.EventType != "tool.denied" {
		t.Errorf("should emit tool.denied, got eventType=%q", result.EventType)
	}
}

func TestLowRiskToolAllowedInEnforceMode(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}

	result := cfg.CheckToolExecution(
		"file_read",
		"low",
		false,
		nil,
		nil,
	)

	if !result.Allowed {
		t.Errorf("low-risk tool should be allowed in enforce mode, got allowed=%v", result.Allowed)
	}
}

func TestWarningLevelPathAllowedInEnforceModeWithWarning(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}

	result := cfg.CheckToolExecution(
		"file_read",
		"low",
		false,
		[]string{"path"},
		map[string]string{"path": "credentials"},
	)

	if !result.Allowed {
		t.Errorf("warning-level path should be allowed in enforce mode, got allowed=%v", result.Allowed)
	}
	if !result.ShouldWarn {
		t.Errorf("should warn for warning-level path, got shouldWarn=%v", result.ShouldWarn)
	}
}

func TestValidateEnforcementMode(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{"off", true},
		{"warn", true},
		{"enforce", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			got := ValidateEnforcementMode(tt.mode)
			if got != tt.want {
				t.Errorf("ValidateEnforcementMode(%q) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestEnforcementResultWithViolations(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}

	result := cfg.CheckToolExecution(
		"file_read",
		"low",
		false,
		[]string{"path"},
		map[string]string{"path": ".git/config"},
	)

	if len(result.Violations) == 0 {
		t.Error("expected violations to be populated")
	}

	if result.Violations[0].Rule != "git-directory" {
		t.Errorf("expected rule 'git-directory', got %q", result.Violations[0].Rule)
	}
}

func TestNoPathArgDoesNotCheckPath(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}

	result := cfg.CheckToolExecution(
		"file_read",
		"low",
		false,
		nil, // No path arg keys
		map[string]string{"other": "value"},
	)

	if !result.Allowed {
		t.Errorf("should allow when no path arg, got allowed=%v", result.Allowed)
	}
}

func TestGetSecurityStatus(t *testing.T) {
	status := GetSecurityStatus(
		10,
		[]string{"file_write"},
		[]string{"dangerous_tool"},
		[]string{"file_write"},
	)

	if status.RegisteredTools != 10 {
		t.Errorf("expected 10 tools, got %d", status.RegisteredTools)
	}

	if len(status.HighRiskTools) != 1 {
		t.Errorf("expected 1 high-risk tool, got %d", len(status.HighRiskTools))
	}

	if len(status.CriticalRiskTools) != 1 {
		t.Errorf("expected 1 critical-risk tool, got %d", len(status.CriticalRiskTools))
	}
}
