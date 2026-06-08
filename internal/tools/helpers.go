package tools

import (
	"fmt"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
)

// toolError returns a ToolResponse indicating failure.
func toolError(toolName, msg string) ToolResponse {
	return ToolResponse{
		ToolName: toolName,
		Success:  false,
		ExitCode: 1,
		Error:    msg,
	}
}

// safetyGuardFromRequest creates a SafetyGuard with default policy values.
func safetyGuardFromRequest(req ToolRequest) *SafetyGuard {
	return NewSafetyGuard(ToolPolicy{
		MaxOutputBytes:        req.MaxOutputBytes,
		DefaultTimeoutSeconds: req.TimeoutSeconds,
		DenyWritePaths:        DefaultDenyWritePaths(),
		DenyReadPaths:         DefaultDenyReadPaths(),
	})
}

// SafetyGuardFromConfig creates a SafetyGuard from the config's tool policy.
func SafetyGuardFromConfig(cfg *config.Root) *SafetyGuard {
	policy := ToolPolicy{
		DenyWritePaths: DefaultDenyWritePaths(),
		DenyReadPaths:  DefaultDenyReadPaths(),
	}
	if cfg.Tools.Policy.MaxOutputBytes > 0 {
		policy.MaxOutputBytes = cfg.Tools.Policy.MaxOutputBytes
	} else {
		policy.MaxOutputBytes = DefaultMaxOutputBytes
	}
	if cfg.Tools.Policy.DefaultTimeoutSeconds > 0 {
		policy.DefaultTimeoutSeconds = cfg.Tools.Policy.DefaultTimeoutSeconds
	} else {
		policy.DefaultTimeoutSeconds = DefaultTimeoutSeconds
	}
	if len(cfg.Tools.Policy.DenyWritePaths) > 0 {
		policy.DenyWritePaths = cfg.Tools.Policy.DenyWritePaths
	}
	if len(cfg.Tools.Policy.DenyReadPaths) > 0 {
		policy.DenyReadPaths = cfg.Tools.Policy.DenyReadPaths
	}
	return NewSafetyGuard(policy)
}

// TestCommandsFromConfig converts config test commands to TestCommandDef map.
func TestCommandsFromConfig(cfg *config.Root) map[string]TestCommandDef {
	m := make(map[string]TestCommandDef)
	for _, tc := range cfg.Tools.TestCommands {
		m[tc.Name] = TestCommandDef{
			Command:        tc.Command,
			TimeoutSeconds: tc.TimeoutSeconds,
		}
	}
	return m
}

// EnabledToolsFromConfig returns a map of tool name to enabled status.
func EnabledToolsFromConfig(cfg *config.Root) map[string]bool {
	m := make(map[string]bool)
	for _, tc := range cfg.Tools.Tools {
		m[tc.Name] = tc.Enabled
	}
	return m
}

// RegisterBuiltinTools registers all built-in tools with the given registry.
func RegisterBuiltinTools(registry ToolRegistry, testCommands map[string]TestCommandDef) error {
	tools := []Tool{
		&FileReadTool{},
		&FileWriteTool{},
		&FilePatchTool{},
		&GitDiffTool{},
		&TestRunTool{Commands: testCommands},
	}
	for _, t := range tools {
		if err := registry.Register(t); err != nil {
			return fmt.Errorf("register tool %q: %w", t.Name(), err)
		}
	}
	return nil
}

// ToolResponseToScratchpadItem converts a ToolResponse to a scratchpad Item.
// This is provided for Phase 4 Agent Runtime integration.
// It does NOT automatically write to the scratchpad.
func ToolResponseToScratchpadItem(resp ToolResponse, taskID string) ScratchpadItemCompat {
	return ScratchpadItemCompat{
		Kind:    "tool_output",
		Content: resp.Stdout,
		TaskID:  taskID,
	}
}

// ScratchpadItemCompat is a lightweight struct for Phase 4 compatibility.
// Phase 4 will use scratchpad.Item directly.
type ScratchpadItemCompat struct {
	Kind    string
	Content string
	TaskID  string
}
