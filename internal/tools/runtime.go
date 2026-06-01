package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/mimoneko/mimoneko/internal/events"
	"github.com/mimoneko/mimoneko/internal/security"
)

// ToolRuntime is the central orchestrator for tool execution.
// All tool invocations must go through Run(); business code must never
// call a Tool directly.
type ToolRuntime interface {
	Run(ctx context.Context, req ToolRequest) (ToolResponse, error)
}

// DefaultToolRuntime implements ToolRuntime with safety checks, timeout,
// output truncation, audit logging, and optional event emission.
type DefaultToolRuntime struct {
	registry     ToolRegistry
	guard        *SafetyGuard
	audit        *AuditLog
	enabled      map[string]bool // tool name -> enabled
	eventEmitter events.EventEmitter
}

// NewDefaultToolRuntime creates a ToolRuntime with the given dependencies.
// enabled maps tool names to their enabled status from config.
func NewDefaultToolRuntime(registry ToolRegistry, guard *SafetyGuard, audit *AuditLog, enabled map[string]bool) *DefaultToolRuntime {
	return &DefaultToolRuntime{
		registry:     registry,
		guard:        guard,
		audit:        audit,
		enabled:      enabled,
		eventEmitter: &events.NoopEventEmitter{},
	}
}

// SetEventEmitter sets the optional event emitter for tool events.
// When set, the ToolRuntime emits tool.started and tool.finished events
// for every tool execution. Emit failures do not affect tool execution.
func (rt *DefaultToolRuntime) SetEventEmitter(emitter events.EventEmitter) {
	if emitter != nil {
		rt.eventEmitter = emitter
	}
}

// RegisterMetadata registers review metadata for a tool. It does not change
// runtime safety decisions or execution behaviour.
func (rt *DefaultToolRuntime) RegisterMetadata(metadata ToolMetadata) error {
	return RegisterToolMetadata(rt.registry, metadata)
}

// Metadata returns review metadata for a registered tool when available.
func (rt *DefaultToolRuntime) Metadata(name string) (ToolMetadata, bool) {
	return LookupToolMetadata(rt.registry, name)
}

// ListMetadata returns all registered tool metadata sorted by tool name.
func (rt *DefaultToolRuntime) ListMetadata() []ToolMetadata {
	return ListToolMetadata(rt.registry)
}

func (rt *DefaultToolRuntime) toolMetadata(tool Tool) ToolMetadata {
	if metadata, ok := LookupToolMetadata(rt.registry, tool.Name()); ok {
		return metadata
	}
	return metadataFromTool(tool)
}

// Run executes a tool request through the full safety + audit pipeline:
//
//  1. Look up tool in registry
//  2. Check if tool is enabled
//  3. Apply safety guard
//  4. Create timeout context
//  5. Emit tool.called and tool.started events
//  6. Execute tool
//  7. Truncate output if needed
//  8. Record audit log
//  9. Emit tool.completed/tool.failed and tool.finished events
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
	metadata := rt.toolMetadata(tool)

	// 4. Generate audit ID
	auditID, err := generateAuditID()
	if err != nil {
		return ToolResponse{}, fmt.Errorf("tools: generate audit id: %w", err)
	}

	// 5. Write audit start (pre-execution)
	preEvent := ToolAuditEvent{
		ID:           auditID,
		Timestamp:    start,
		ToolName:     req.ToolName,
		TaskID:       req.TaskID,
		RepoRoot:     req.RepoRoot,
		ArgsRedacted: redactArgs(req.Args),
		DryRun:       req.DryRun,
		RiskLevel:    string(metadata.RiskLevel),
	}
	if rt.audit != nil {
		if err := rt.audit.Record(preEvent); err != nil {
			return ToolResponse{}, fmt.Errorf("tools: audit start failed: %w", err)
		}
	}

	// 6. Emit tool.called and tool.started events
	rc := events.RunContextFrom(ctx)
	taskID := req.TaskID
	if taskID == "" {
		taskID = rc.TaskID
	}
	toolStartedMetadata := toolEventMetadata(req, metadata)

	// 6.5. Path sandbox detection (detection only, no blocking)
	rt.detectPathViolations(ctx, req, rc, taskID)
	events.SafeEmit(rt.eventEmitter, ctx, events.RunEvent{
		ID:               mustGenerateToolEventID(),
		RunID:            rc.RunID,
		TaskID:           taskID,
		WorktreeID:       rc.WorktreeID,
		Timestamp:        start.UTC(),
		ToolName:         req.ToolName,
		RiskLevel:        string(metadata.RiskLevel),
		RequiresApproval: boolPtr(metadata.RequiresApproval),
		ResultStatus:     "called",
		Type:             events.EventToolCalled,
		Source:           "tool",
		Status:           "called",
		Message:          fmt.Sprintf("Tool %s called", req.ToolName),
		StartedAt:        start.UTC(),
		Metadata:         toolStartedMetadata,
	})
	events.SafeEmit(rt.eventEmitter, ctx, events.RunEvent{
		ID:               mustGenerateToolEventID(),
		RunID:            rc.RunID,
		TaskID:           taskID,
		WorktreeID:       rc.WorktreeID,
		Timestamp:        start.UTC(),
		ToolName:         req.ToolName,
		RiskLevel:        string(metadata.RiskLevel),
		RequiresApproval: boolPtr(metadata.RequiresApproval),
		ResultStatus:     "started",
		Type:             events.EventToolStarted,
		Source:           "tool",
		Status:           "started",
		Message:          fmt.Sprintf("Tool %s started", req.ToolName),
		StartedAt:        start.UTC(),
		Metadata:         toolStartedMetadata,
	})

	// 7. Create timeout context
	timeoutSec := rt.guard.Timeout(req.TimeoutSeconds)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// 8. Execute tool
	resp, toolErr := tool.Run(ctx, req)
	resp.AuditID = auditID
	resp.ToolName = req.ToolName

	// 9. Truncate output
	maxOut := rt.guard.MaxOutput(req.MaxOutputBytes)
	resp = truncateResponse(resp, maxOut)

	// 10. Record audit finish
	durationMs := time.Since(start).Milliseconds()
	postEvent := ToolAuditEvent{
		ID:           auditID,
		Timestamp:    time.Now(),
		ToolName:     req.ToolName,
		TaskID:       req.TaskID,
		RepoRoot:     req.RepoRoot,
		ArgsRedacted: redactArgs(req.Args),
		Success:      resp.Success,
		ExitCode:     resp.ExitCode,
		OutputBytes:  resp.OutputBytes,
		Error:        resp.Error,
		DurationMs:   durationMs,
		RiskLevel:    string(metadata.RiskLevel),
		DryRun:       req.DryRun,
	}
	if rt.audit != nil {
		if err := rt.audit.Record(postEvent); err != nil {
			return resp, fmt.Errorf("tools: audit finish failed: %w", err)
		}
	}

	// 11. Emit tool.completed/tool.failed and tool.finished events
	toolFinishedStatus := "succeeded"
	toolFinishedError := ""
	if !resp.Success || toolErr != nil {
		toolFinishedStatus = "failed"
		if resp.Error != "" {
			toolFinishedError = resp.Error
		} else if toolErr != nil {
			toolFinishedError = toolErr.Error()
		}
	}
	finishedAt := time.Now().UTC()
	auditEventType := events.EventToolCompleted
	if toolFinishedStatus == "failed" {
		auditEventType = events.EventToolFailed
	}
	events.SafeEmit(rt.eventEmitter, ctx, events.RunEvent{
		ID:               mustGenerateToolEventID(),
		RunID:            rc.RunID,
		TaskID:           taskID,
		WorktreeID:       rc.WorktreeID,
		Timestamp:        finishedAt,
		ToolName:         req.ToolName,
		RiskLevel:        string(metadata.RiskLevel),
		RequiresApproval: boolPtr(metadata.RequiresApproval),
		ResultStatus:     toolFinishedStatus,
		ErrorMessage:     toolFinishedError,
		Type:             auditEventType,
		Source:           "tool",
		Status:           toolFinishedStatus,
		Message:          fmt.Sprintf("Tool %s %s", req.ToolName, toolFinishedStatus),
		StartedAt:        start.UTC(),
		FinishedAt:       finishedAt,
		DurationMs:       durationMs,
		Error:            toolFinishedError,
		Metadata:         toolStartedMetadata,
	})
	events.SafeEmit(rt.eventEmitter, ctx, events.RunEvent{
		ID:               mustGenerateToolEventID(),
		RunID:            rc.RunID,
		TaskID:           taskID,
		WorktreeID:       rc.WorktreeID,
		Timestamp:        finishedAt,
		ToolName:         req.ToolName,
		RiskLevel:        string(metadata.RiskLevel),
		RequiresApproval: boolPtr(metadata.RequiresApproval),
		ResultStatus:     toolFinishedStatus,
		ErrorMessage:     toolFinishedError,
		Type:             events.EventToolFinished,
		Source:           "tool",
		Status:           toolFinishedStatus,
		Message:          fmt.Sprintf("Tool %s finished", req.ToolName),
		StartedAt:        start.UTC(),
		FinishedAt:       finishedAt,
		DurationMs:       durationMs,
		Error:            toolFinishedError,
		Metadata:         toolStartedMetadata,
	})

	if toolErr != nil {
		return resp, toolErr
	}
	return resp, nil
}

// mustGenerateToolEventID generates a unique event ID for tool events.
func mustGenerateToolEventID() string {
	id, err := events.GenerateEventID()
	if err != nil {
		return "evt_error"
	}
	return id
}

func toolEventMetadata(req ToolRequest, metadata ToolMetadata) map[string]string {
	eventMetadata := map[string]string{
		"tool_name":  req.ToolName,
		"risk_level": string(metadata.RiskLevel),
	}
	if cmdName, ok := req.Args["command_name"]; ok {
		eventMetadata["command_name"] = cmdName
	}
	return eventMetadata
}

func boolPtr(value bool) *bool {
	return &value
}

// pathArgKeys are the argument keys that may contain file paths.
var pathArgKeys = []string{
	"path", "file", "filename", "filepath",
	"dir", "directory", "cwd", "target",
}

// detectPathViolations checks tool arguments for sensitive paths and emits
// path.violation_candidate events. This is DETECTION ONLY - it does NOT
// block or modify tool execution in any way.
func (rt *DefaultToolRuntime) detectPathViolations(ctx context.Context, req ToolRequest, rc events.RunContext, taskID string) {
	// Skip if no event emitter
	if rt.eventEmitter == nil {
		return
	}

	// Check each path argument
	for _, key := range pathArgKeys {
		pathValue, ok := req.Args[key]
		if !ok || pathValue == "" {
			continue
		}

		// Sanitize the path value for the event
		sanitizedPath := security.SanitizeText(pathValue)

		// Validate path for sensitive patterns
		violations := security.ValidatePath(pathValue)
		if len(violations) == 0 {
			continue
		}

		// Emit violation candidate events (detection only, no blocking)
		for _, v := range violations {
			metadata := map[string]string{
				"tool_name": req.ToolName,
				"path":      sanitizedPath,
				"rule":      v.Rule,
				"severity":  string(v.Severity),
				"candidate": fmt.Sprintf("%v", v.Candidate),
				"arg_key":   key,
			}

			// Sanitize metadata values
			sanitizedMetadata := security.SanitizeMap(metadata)

			events.SafeEmit(rt.eventEmitter, ctx, events.RunEvent{
				ID:         mustGenerateToolEventID(),
				RunID:      rc.RunID,
				TaskID:     taskID,
				WorktreeID: rc.WorktreeID,
				Timestamp:  time.Now().UTC(),
				ToolName:   req.ToolName,
				Type:       "path.violation_candidate",
				Source:     "sandbox",
				Status:     "detected",
				Message:    fmt.Sprintf("Path violation candidate: %s (%s)", sanitizedPath, v.Rule),
				Metadata:   sanitizedMetadata,
			})
		}
	}
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
