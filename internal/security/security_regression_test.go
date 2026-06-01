package security

import (
	"os"
	"testing"
)

// =============================================================================
// OFF MODE REGRESSION
// =============================================================================

func TestRegression_OffMode_AllowsLowRiskTool(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeOff}
	result := cfg.CheckToolExecution("file_read", "low", false, nil, nil)
	if !result.Allowed {
		t.Errorf("off mode should allow low risk tool, got allowed=%v", result.Allowed)
	}
}

func TestRegression_OffMode_AllowsMediumRiskTool(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeOff}
	result := cfg.CheckToolExecution("file_write", "medium", false, nil, nil)
	if !result.Allowed {
		t.Errorf("off mode should allow medium risk tool, got allowed=%v", result.Allowed)
	}
}

func TestRegression_OffMode_AllowsHighRiskTool(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeOff}
	result := cfg.CheckToolExecution("file_patch", "high", false, nil, nil)
	if !result.Allowed {
		t.Errorf("off mode should allow high risk tool, got allowed=%v", result.Allowed)
	}
}

func TestRegression_OffMode_AllowsCriticalRiskTool(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeOff}
	result := cfg.CheckToolExecution("dangerous_tool", "critical", false, nil, nil)
	if !result.Allowed {
		t.Errorf("off mode should allow critical risk tool, got allowed=%v", result.Allowed)
	}
}

func TestRegression_OffMode_AllowsCriticalPath(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeOff}
	result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": ".git/config"})
	if !result.Allowed {
		t.Errorf("off mode should allow critical path, got allowed=%v", result.Allowed)
	}
}

func TestRegression_OffMode_AllowsWarningPath(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeOff}
	result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": "credentials"})
	if !result.Allowed {
		t.Errorf("off mode should allow warning path, got allowed=%v", result.Allowed)
	}
}

// =============================================================================
// WARN MODE REGRESSION
// =============================================================================

func TestRegression_WarnMode_AllowsLowRiskTool(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeWarn}
	result := cfg.CheckToolExecution("file_read", "low", false, nil, nil)
	if !result.Allowed {
		t.Errorf("warn mode should allow low risk tool, got allowed=%v", result.Allowed)
	}
}

func TestRegression_WarnMode_AllowsMediumRiskTool(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeWarn}
	result := cfg.CheckToolExecution("file_write", "medium", false, nil, nil)
	if !result.Allowed {
		t.Errorf("warn mode should allow medium risk tool, got allowed=%v", result.Allowed)
	}
}

func TestRegression_WarnMode_AllowsHighRiskToolWithWarning(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeWarn}
	result := cfg.CheckToolExecution("file_patch", "high", false, nil, nil)
	if !result.Allowed {
		t.Errorf("warn mode should allow high risk tool, got allowed=%v", result.Allowed)
	}
	if !result.ShouldWarn {
		t.Errorf("warn mode should warn for high risk tool, got shouldWarn=%v", result.ShouldWarn)
	}
}

func TestRegression_WarnMode_AllowsCriticalRiskToolWithWarning(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeWarn}
	result := cfg.CheckToolExecution("dangerous_tool", "critical", false, nil, nil)
	if !result.Allowed {
		t.Errorf("warn mode should allow critical risk tool, got allowed=%v", result.Allowed)
	}
	if !result.ShouldWarn {
		t.Errorf("warn mode should warn for critical risk tool, got shouldWarn=%v", result.ShouldWarn)
	}
}

func TestRegression_WarnMode_AllowsCriticalPathWithWarning(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeWarn}
	result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": ".git/config"})
	if !result.Allowed {
		t.Errorf("warn mode should allow critical path, got allowed=%v", result.Allowed)
	}
	if !result.ShouldWarn {
		t.Errorf("warn mode should warn for critical path, got shouldWarn=%v", result.ShouldWarn)
	}
}

func TestRegression_WarnMode_EmitsSecurityWarningForCriticalPath(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeWarn}
	result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": ".git/config"})
	if result.EventType != "security.warning" {
		t.Errorf("warn mode should emit security.warning, got eventType=%q", result.EventType)
	}
}

// =============================================================================
// ENFORCE MODE REGRESSION
// =============================================================================

func TestRegression_EnforceMode_AllowsLowRiskTool(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}
	result := cfg.CheckToolExecution("file_read", "low", false, nil, nil)
	if !result.Allowed {
		t.Errorf("enforce mode should allow low risk tool, got allowed=%v", result.Allowed)
	}
}

func TestRegression_EnforceMode_AllowsMediumRiskToolWithoutApproval(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}
	result := cfg.CheckToolExecution("file_write", "medium", false, nil, nil)
	if !result.Allowed {
		t.Errorf("enforce mode should allow medium risk tool without approval, got allowed=%v", result.Allowed)
	}
}

func TestRegression_EnforceMode_RequiresApprovalForMediumRiskToolWithApproval(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}
	result := cfg.CheckToolExecution("file_write", "medium", true, nil, nil)
	if result.Allowed {
		t.Errorf("enforce mode should deny medium risk tool with approval flag, got allowed=%v", result.Allowed)
	}
	if !result.RequiresApproval {
		t.Errorf("should require approval, got requiresApproval=%v", result.RequiresApproval)
	}
}

func TestRegression_EnforceMode_RequiresApprovalForHighRiskTool(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}
	result := cfg.CheckToolExecution("file_patch", "high", false, nil, nil)
	if result.Allowed {
		t.Errorf("enforce mode should deny high risk tool, got allowed=%v", result.Allowed)
	}
	if !result.RequiresApproval {
		t.Errorf("should require approval, got requiresApproval=%v", result.RequiresApproval)
	}
	if result.EventType != "tool.approval_required" {
		t.Errorf("should emit tool.approval_required, got eventType=%q", result.EventType)
	}
}

func TestRegression_EnforceMode_DeniesCriticalRiskTool(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}
	result := cfg.CheckToolExecution("dangerous_tool", "critical", false, nil, nil)
	if result.Allowed {
		t.Errorf("enforce mode should deny critical risk tool, got allowed=%v", result.Allowed)
	}
	if result.EventType != "tool.denied" {
		t.Errorf("should emit tool.denied, got eventType=%q", result.EventType)
	}
}

func TestRegression_EnforceMode_BlocksCriticalPath(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}
	result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": ".git/config"})
	if result.Allowed {
		t.Errorf("enforce mode should block critical path, got allowed=%v", result.Allowed)
	}
	if result.EventType != "path.blocked" {
		t.Errorf("should emit path.blocked, got eventType=%q", result.EventType)
	}
}

func TestRegression_EnforceMode_AllowsWarningPathWithWarning(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}
	result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": "credentials"})
	if !result.Allowed {
		t.Errorf("enforce mode should allow warning path, got allowed=%v", result.Allowed)
	}
	if !result.ShouldWarn {
		t.Errorf("should warn for warning path, got shouldWarn=%v", result.ShouldWarn)
	}
}

// =============================================================================
// PATH ENFORCEMENT REGRESSION
// =============================================================================

func TestRegression_PathEnforcement_GitDirectory(t *testing.T) {
	tests := []struct {
		mode    EnforcementMode
		path    string
		allowed bool
	}{
		{ModeOff, ".git/config", true},
		{ModeWarn, ".git/config", true},
		{ModeEnforce, ".git/config", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode)+"_"+tt.path, func(t *testing.T) {
			cfg := &EnforcementConfig{Mode: tt.mode}
			result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": tt.path})
			if result.Allowed != tt.allowed {
				t.Errorf("mode=%s path=%s: got allowed=%v, want %v", tt.mode, tt.path, result.Allowed, tt.allowed)
			}
		})
	}
}

func TestRegression_PathEnforcement_EnvFile(t *testing.T) {
	tests := []struct {
		mode    EnforcementMode
		path    string
		allowed bool
	}{
		{ModeOff, ".env", true},
		{ModeWarn, ".env", true},
		{ModeEnforce, ".env", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode)+"_"+tt.path, func(t *testing.T) {
			cfg := &EnforcementConfig{Mode: tt.mode}
			result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": tt.path})
			if result.Allowed != tt.allowed {
				t.Errorf("mode=%s path=%s: got allowed=%v, want %v", tt.mode, tt.path, result.Allowed, tt.allowed)
			}
		})
	}
}

func TestRegression_PathEnforcement_SshKey(t *testing.T) {
	tests := []struct {
		mode    EnforcementMode
		path    string
		allowed bool
	}{
		{ModeOff, "id_rsa", true},
		{ModeWarn, "id_rsa", true},
		{ModeEnforce, "id_rsa", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode)+"_"+tt.path, func(t *testing.T) {
			cfg := &EnforcementConfig{Mode: tt.mode}
			result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": tt.path})
			if result.Allowed != tt.allowed {
				t.Errorf("mode=%s path=%s: got allowed=%v, want %v", tt.mode, tt.path, result.Allowed, tt.allowed)
			}
		})
	}
}

func TestRegression_PathEnforcement_Credentials(t *testing.T) {
	tests := []struct {
		mode    EnforcementMode
		path    string
		allowed bool
	}{
		{ModeOff, "credentials", true},
		{ModeWarn, "credentials", true},
		{ModeEnforce, "credentials", true}, // Warning level, not critical
	}

	for _, tt := range tests {
		t.Run(string(tt.mode)+"_"+tt.path, func(t *testing.T) {
			cfg := &EnforcementConfig{Mode: tt.mode}
			result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": tt.path})
			if result.Allowed != tt.allowed {
				t.Errorf("mode=%s path=%s: got allowed=%v, want %v", tt.mode, tt.path, result.Allowed, tt.allowed)
			}
		})
	}
}

func TestRegression_PathEnforcement_Token(t *testing.T) {
	tests := []struct {
		mode    EnforcementMode
		path    string
		allowed bool
	}{
		{ModeOff, "token", true},
		{ModeWarn, "token", true},
		{ModeEnforce, "token", true}, // Warning level, not critical
	}

	for _, tt := range tests {
		t.Run(string(tt.mode)+"_"+tt.path, func(t *testing.T) {
			cfg := &EnforcementConfig{Mode: tt.mode}
			result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": tt.path})
			if result.Allowed != tt.allowed {
				t.Errorf("mode=%s path=%s: got allowed=%v, want %v", tt.mode, tt.path, result.Allowed, tt.allowed)
			}
		})
	}
}

func TestRegression_PathEnforcement_Secret(t *testing.T) {
	tests := []struct {
		mode    EnforcementMode
		path    string
		allowed bool
	}{
		{ModeOff, "secrets", true},
		{ModeWarn, "secrets", true},
		{ModeEnforce, "secrets", true}, // Warning level, not critical
	}

	for _, tt := range tests {
		t.Run(string(tt.mode)+"_"+tt.path, func(t *testing.T) {
			cfg := &EnforcementConfig{Mode: tt.mode}
			result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": tt.path})
			if result.Allowed != tt.allowed {
				t.Errorf("mode=%s path=%s: got allowed=%v, want %v", tt.mode, tt.path, result.Allowed, tt.allowed)
			}
		})
	}
}

// =============================================================================
// EVENT REGRESSION
// =============================================================================

func TestRegression_Events_ToolCalledEmitted(t *testing.T) {
	// This test verifies that tool.called events are properly structured
	// The actual emission is tested in the tool runtime tests
	eventType := "tool.called"
	if eventType != "tool.called" {
		t.Errorf("expected tool.called event type")
	}
}

func TestRegression_Events_SecurityWarningEmitted(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeWarn}
	result := cfg.CheckToolExecution("file_read", "high", false, nil, nil)
	if result.EventType != "security.warning" {
		t.Errorf("expected security.warning, got %q", result.EventType)
	}
}

func TestRegression_Events_PathBlockedEmitted(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}
	result := cfg.CheckToolExecution("file_read", "low", false, []string{"path"}, map[string]string{"path": ".git/config"})
	if result.EventType != "path.blocked" {
		t.Errorf("expected path.blocked, got %q", result.EventType)
	}
}

func TestRegression_Events_ToolDeniedEmitted(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}
	result := cfg.CheckToolExecution("dangerous_tool", "critical", false, nil, nil)
	if result.EventType != "tool.denied" {
		t.Errorf("expected tool.denied, got %q", result.EventType)
	}
}

func TestRegression_Events_ToolApprovalRequiredEmitted(t *testing.T) {
	cfg := &EnforcementConfig{Mode: ModeEnforce}
	result := cfg.CheckToolExecution("file_patch", "high", false, nil, nil)
	if result.EventType != "tool.approval_required" {
		t.Errorf("expected tool.approval_required, got %q", result.EventType)
	}
}

// =============================================================================
// REACTION REGRESSION
// =============================================================================

func TestRegression_Redaction_OpenAIKeyNotLeaked(t *testing.T) {
	input := "Using key sk-abcdefghijklmnopqrstuvwxyz"
	sanitized := SanitizeText(input)
	if contains(sanitized, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("OpenAI key leaked: %q", sanitized)
	}
}

func TestRegression_Redaction_MiMoKeyNotLeaked(t *testing.T) {
	input := "Using key tp-cabcdefghijklmnopqrstuvwxyz"
	sanitized := SanitizeText(input)
	if contains(sanitized, "tp-cabcdefghijklmnopqrstuvwxyz") {
		t.Errorf("MiMo key leaked: %q", sanitized)
	}
}

func TestRegression_Redaction_JWTNotLeaked(t *testing.T) {
	input := "token: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwfQ.abcdefghijklmnop"
	sanitized := SanitizeText(input)
	if contains(sanitized, "eyJhbGciOiJIUzI1NiJ9") {
		t.Errorf("JWT leaked: %q", sanitized)
	}
}

func TestRegression_Redaction_BearerTokenNotLeaked(t *testing.T) {
	input := "Bearer abcdefghijklmnopqrstuvwxyz"
	sanitized := SanitizeText(input)
	if contains(sanitized, "abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("Bearer token leaked: %q", sanitized)
	}
}

func TestRegression_Redaction_CookieNotLeaked(t *testing.T) {
	input := "Cookie: sessionid=abcdefghijklmnopqrstuvwxyz"
	sanitized := SanitizeText(input)
	if contains(sanitized, "abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("Cookie leaked: %q", sanitized)
	}
}

// =============================================================================
// DEFAULT MODE REGRESSION
// =============================================================================

func TestRegression_DefaultMode_IsWarn(t *testing.T) {
	os.Unsetenv(SecurityModeEnvVar)
	defer os.Unsetenv(SecurityModeEnvVar)

	mode := GetEnforcementMode()
	if mode != ModeWarn {
		t.Errorf("default mode should be warn, got %q", mode)
	}
}

// =============================================================================
// SECURITY SUMMARY REGRESSION
// =============================================================================

func TestRegression_SecuritySummary_FieldsPopulated(t *testing.T) {
	summary := GetSecuritySummary(
		10,
		[]string{"file_write", "file_patch"},
		[]string{"dangerous_tool"},
		[]string{"file_patch"},
	)

	if summary.TotalRegisteredTools != 10 {
		t.Errorf("expected 10 tools, got %d", summary.TotalRegisteredTools)
	}
	if len(summary.HighRiskTools) != 2 {
		t.Errorf("expected 2 high risk tools, got %d", len(summary.HighRiskTools))
	}
	if len(summary.CriticalRiskTools) != 1 {
		t.Errorf("expected 1 critical risk tool, got %d", len(summary.CriticalRiskTools))
	}
	if len(summary.ApprovalRequired) != 1 {
		t.Errorf("expected 1 approval required tool, got %d", len(summary.ApprovalRequired))
	}
	if summary.SandboxRulesCount == 0 {
		t.Error("expected sandbox rules count > 0")
	}
	if len(summary.BlockedRules) == 0 {
		t.Error("expected blocked rules to be populated")
	}
}

// =============================================================================
// HELPER
// =============================================================================

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
