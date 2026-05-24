// Package validation implements the test validation runner for ReasonForge.
//
// ValidationRunner executes test commands through ToolRuntime (test_run tool),
// never directly executing arbitrary shell commands. All test commands must
// be pre-configured in tools.yaml.
//
// Safety guarantees:
//   - Must execute through ToolRuntime, never directly exec.
//   - TestCommands must be command_names from tools.yaml config.
//   - RepoRoot should be a worktree path, not the main workspace path.
//   - Output must be capped by MaxOutputBytes.
//   - Timeout must be enforced.
//   - ValidationResult must not leak API keys.
package validation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/reasonforge/reasonforge/internal/review"
	"github.com/reasonforge/reasonforge/internal/tools"
)

// DefaultValidationConfig returns safe defaults for validation.
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		DefaultTestCommands: []string{"go-test"},
		MaxOutputBytes:      65536,
		TimeoutSeconds:      120,
	}
}

// ValidationConfig configures the validation runner.
type ValidationConfig struct {
	// DefaultTestCommands lists the default test command names.
	DefaultTestCommands []string `yaml:"default_test_commands"`

	// MaxOutputBytes caps output per command.
	MaxOutputBytes int `yaml:"max_output_bytes"`

	// TimeoutSeconds caps total validation duration.
	TimeoutSeconds int `yaml:"timeout_seconds"`
}

// ValidationRunner executes test commands through ToolRuntime.
type ValidationRunner struct {
	toolRuntime tools.ToolRuntime
	cfg         ValidationConfig
}

// NewValidationRunner creates a new ValidationRunner.
func NewValidationRunner(toolRuntime tools.ToolRuntime, cfg ValidationConfig) *ValidationRunner {
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = 65536
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 120
	}
	return &ValidationRunner{
		toolRuntime: toolRuntime,
		cfg:         cfg,
	}
}

// Validate executes test commands and returns a structured result.
func (r *ValidationRunner) Validate(ctx context.Context, req review.ValidationRequest) (review.ValidationResult, error) {
	if len(req.TestCommands) == 0 {
		return review.ValidationResult{
			Success: false,
			Summary: "no test commands configured",
		}, nil
	}

	// Apply defaults from config
	maxOutputBytes := req.MaxOutputBytes
	if maxOutputBytes <= 0 {
		maxOutputBytes = r.cfg.MaxOutputBytes
	}
	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = r.cfg.TimeoutSeconds
	}

	// Create timeout context for the entire validation
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	var commands []review.CommandValidationResult
	allSuccess := true

	for _, cmdName := range req.TestCommands {
		start := time.Now()

		toolReq := tools.ToolRequest{
			ToolName:       "test_run",
			RepoRoot:       req.RepoRoot,
			TaskID:         req.TaskID,
			MaxOutputBytes: maxOutputBytes,
			TimeoutSeconds: timeoutSeconds,
			Args:           map[string]string{"command_name": cmdName},
			Metadata:       map[string]string{"source": "validation_runner"},
		}

		resp, err := r.toolRuntime.Run(ctx, toolReq)
		durationMs := time.Since(start).Milliseconds()

		if err != nil {
			// Tool runtime error (tool not found, disabled, etc.)
			commands = append(commands, review.CommandValidationResult{
				CommandName: cmdName,
				Success:     false,
				ExitCode:    -1,
				DurationMs:  durationMs,
				Error:       sanitizeOutput(err.Error()),
			})
			allSuccess = false
			continue
		}

		cmdResult := review.CommandValidationResult{
			CommandName: cmdName,
			Success:     resp.Success,
			ExitCode:    resp.ExitCode,
			Stdout:      sanitizeOutput(resp.Stdout),
			Stderr:      sanitizeOutput(resp.Stderr),
			DurationMs:  durationMs,
		}

		if !resp.Success {
			cmdResult.Error = sanitizeOutput(resp.Error)
			allSuccess = false
		}

		commands = append(commands, cmdResult)
	}

	summary := "all tests passed"
	if !allSuccess {
		failed := 0
		for _, c := range commands {
			if !c.Success {
				failed++
			}
		}
		summary = fmt.Sprintf("%d of %d commands failed", failed, len(commands))
	}

	return review.ValidationResult{
		Success:  allSuccess,
		Commands: commands,
		Summary:  summary,
	}, nil
}

// apiKeyPatterns are substrings that indicate potential API key leakage.
var apiKeyPatterns = []string{
	"API_KEY",
	"SECRET",
	"TOKEN",
	"PASSWORD",
	"PRIVATE_KEY",
	"sk-",
	"sk_live_",
	"pk_live_",
	"AKIA",
}

// sanitizeOutput removes potential API keys from output strings.
func sanitizeOutput(s string) string {
	for _, pattern := range apiKeyPatterns {
		idx := strings.Index(strings.ToUpper(s), strings.ToUpper(pattern))
		if idx >= 0 {
			// Redact the line containing the pattern
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if strings.Contains(strings.ToUpper(line), strings.ToUpper(pattern)) {
					lines[i] = "[redacted: potential secret]"
				}
			}
			s = strings.Join(lines, "\n")
		}
	}
	return s
}
