package tools

import (
	"context"
	"fmt"
	"time"
)

// ToolRuntime is the central orchestrator for tool execution.
// All tool invocations must go through Run(); business code must never
// call a Tool directly.
type ToolRuntime interface {
	Run(ctx context.Context, req ToolRequest) (ToolResponse, error)
}

// DefaultToolRuntime implements ToolRuntime with safety checks, timeout,
// output truncation, and audit logging.
type DefaultToolRuntime struct {
	registry ToolRegistry
	guard    *SafetyGuard
	audit    *AuditLog
	enabled  map[string]bool // tool name -> enabled
}

// NewDefaultToolRuntime creates a ToolRuntime with the given dependencies.
// enabled maps tool names to their enabled status from config.
func NewDefaultToolRuntime(registry ToolRegistry, guard *SafetyGuard, audit *AuditLog, enabled map[string]bool) *DefaultToolRuntime {
	return &DefaultToolRuntime{
		registry: registry,
		guard:    guard,
		audit:    audit,
		enabled:  enabled,
	}
}

// Run executes a tool request through the full safety + audit pipeline:
//
//	1. Look up tool in registry
//	2. Check if tool is enabled
//	3. Apply safety guard
//	4. Create timeout context
//	5. Execute tool
//	6. Truncate output if needed
//	7. Record audit log
func (rt *DefaultToolRuntime) Run(ctx context.Context, req ToolRequest) (ToolResponse, error) {
	start := time.Now()

	// 1. Look up tool
	tool, found := rt.registry.Get(req.ToolName)
	if !found {
		return ToolResponse{}, fmt.Errorf("tools: unknown tool %q", req.ToolName)
	}

	// 2. Check enabled
	if enabled, ok := rt.enabled[req.ToolName]; ok && !enabled {
		return ToolResponse{}, fmt.Errorf("tools: tool %q is disabled", req.ToolName)
	}

	// 3. Validate RepoRoot
	if req.RepoRoot == "" {
		return ToolResponse{}, fmt.Errorf("tools: repo_root is required")
	}

	// 4. Generate audit ID
	auditID, err := generateAuditID()
	if err != nil {
		return ToolResponse{}, fmt.Errorf("tools: generate audit id: %w", err)
	}

	// 5. Write audit start (pre-execution)
	preEvent := ToolAuditEvent{
		ID:          auditID,
		Timestamp:   start,
		ToolName:    req.ToolName,
		TaskID:      req.TaskID,
		RepoRoot:    req.RepoRoot,
		ArgsRedacted: redactArgs(req.Args),
		DryRun:       req.DryRun,
		RiskLevel:    tool.RiskLevel(),
	}
	if rt.audit != nil {
		if err := rt.audit.Record(preEvent); err != nil {
			return ToolResponse{}, fmt.Errorf("tools: audit start failed: %w", err)
		}
	}

	// 6. Create timeout context
	timeoutSec := rt.guard.Timeout(req.TimeoutSeconds)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// 7. Execute tool
	resp, toolErr := tool.Run(ctx, req)
	resp.AuditID = auditID
	resp.ToolName = req.ToolName

	// 8. Truncate output
	maxOut := rt.guard.MaxOutput(req.MaxOutputBytes)
	resp = truncateResponse(resp, maxOut)

	// 9. Record audit finish
	durationMs := time.Since(start).Milliseconds()
	postEvent := ToolAuditEvent{
		ID:          auditID,
		Timestamp:   time.Now(),
		ToolName:    req.ToolName,
		TaskID:      req.TaskID,
		RepoRoot:    req.RepoRoot,
		ArgsRedacted: redactArgs(req.Args),
		Success:     resp.Success,
		ExitCode:    resp.ExitCode,
		OutputBytes: resp.OutputBytes,
		Error:       resp.Error,
		DurationMs:  durationMs,
		RiskLevel:   tool.RiskLevel(),
		DryRun:      req.DryRun,
	}
	if rt.audit != nil {
		if err := rt.audit.Record(postEvent); err != nil {
			return resp, fmt.Errorf("tools: audit finish failed: %w", err)
		}
	}

	if toolErr != nil {
		return resp, toolErr
	}
	return resp, nil
}

// truncateResponse truncates Stdout and Stderr to fit within maxBytes.
func truncateResponse(resp ToolResponse, maxBytes int) ToolResponse {
	total := len(resp.Stdout) + len(resp.Stderr)
	resp.OutputBytes = total

	if total <= maxBytes {
		return resp
	}

	// Prefer truncating Stdout first
	if len(resp.Stdout) > 0 {
		stdoutBudget := maxBytes - len(resp.Stderr)
		if stdoutBudget < 0 {
			stdoutBudget = 0
		}
		if len(resp.Stdout) > stdoutBudget {
			resp.Stdout = resp.Stdout[:stdoutBudget]
			resp.Truncated = true
		}
	}

	// If still over, truncate Stderr
	total = len(resp.Stdout) + len(resp.Stderr)
	if total > maxBytes && len(resp.Stderr) > 0 {
		stderrBudget := maxBytes - len(resp.Stdout)
		if stderrBudget < 0 {
			stderrBudget = 0
		}
		if len(resp.Stderr) > stderrBudget {
			resp.Stderr = resp.Stderr[:stderrBudget]
			resp.Truncated = true
		}
	}

	return resp
}
