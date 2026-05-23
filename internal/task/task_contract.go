package task

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// TaskContract defines the execution boundary for an agent run.
// Every agent run MUST have a TaskContract. If the user does not
// provide one explicitly, DefaultContract() generates a safe default.
//
// Contract is the agent-level boundary. ToolRuntime is the system-level
// security boundary. Both must pass for a tool invocation to succeed.
type TaskContract struct {
	// ID is a unique identifier for this contract.
	ID string `json:"id"`

	// Goal describes the task objective.
	Goal string `json:"goal"`

	// RepoRoot is the workspace root directory.
	RepoRoot string `json:"repo_root"`

	// AllowedPaths are glob patterns for paths that may be accessed.
	// If non-empty, only matching paths are allowed.
	AllowedPaths []string `json:"allowed_paths,omitempty"`

	// DeniedPaths are glob patterns for paths that must never be accessed.
	DeniedPaths []string `json:"denied_paths,omitempty"`

	// AllowedTools lists tool names that may be invoked.
	// If non-empty, only these tools may be used.
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// DeniedTools lists tool names that must never be invoked.
	DeniedTools []string `json:"denied_tools,omitempty"`

	// MaxSteps is the maximum number of agent loop iterations.
	MaxSteps int `json:"max_steps"`

	// MaxToolCalls is the maximum number of tool invocations across the run.
	MaxToolCalls int `json:"max_tool_calls"`

	// MaxOutputBytes caps total tool output bytes across the run.
	MaxOutputBytes int `json:"max_output_bytes"`

	// RequireApprovalForRisk lists risk levels that require explicit approval.
	// e.g. ["medium"] means medium-risk tools need approval before execution.
	RequireApprovalForRisk []string `json:"require_approval_for_risk,omitempty"`

	// DryRun indicates the agent should report what it would do without
	// performing side effects.
	DryRun bool `json:"dry_run"`

	// CreatedAt is the time the contract was created.
	CreatedAt time.Time `json:"created_at"`
}

// DefaultContract returns a safe default TaskContract.
//
// Default contract:
//   - AllowedTools: file_read, git_diff, test_run
//   - DeniedTools: file_write, file_patch
//   - RequireApprovalForRisk: medium
//   - MaxSteps: 5
//   - DryRun: true
func DefaultContract(repoRoot, goal string) TaskContract {
	id, _ := generateContractID()
	return TaskContract{
		ID:                     id,
		Goal:                   goal,
		RepoRoot:               repoRoot,
		AllowedTools:           []string{"file_read", "git_diff", "test_run"},
		DeniedTools:            []string{"file_write", "file_patch"},
		MaxSteps:               5,
		MaxToolCalls:           10,
		MaxOutputBytes:         65536,
		RequireApprovalForRisk: []string{"medium"},
		DryRun:                 true,
		CreatedAt:              time.Now().UTC(),
	}
}

// IsToolAllowed checks whether a tool name is permitted by this contract.
// A tool is denied if it appears in DeniedTools or if AllowedTools is
// non-empty and the tool is not in it.
func (c TaskContract) IsToolAllowed(toolName string) bool {
	// Check denied list first
	for _, denied := range c.DeniedTools {
		if denied == toolName {
			return false
		}
	}
	// If allowed list is specified, tool must be in it
	if len(c.AllowedTools) > 0 {
		for _, allowed := range c.AllowedTools {
			if allowed == toolName {
				return true
			}
		}
		return false
	}
	return true
}

// IsPathAllowed checks whether a file path is permitted by this contract.
// A path is denied if it matches any DeniedPaths pattern or if
// AllowedPaths is non-empty and the path doesn't match any of them.
func (c TaskContract) IsPathAllowed(relPath string) bool {
	normalized := filepath.ToSlash(relPath)

	// Check denied patterns first
	for _, pattern := range c.DeniedPaths {
		if matched, _ := filepath.Match(pattern, normalized); matched {
			return false
		}
		if !strings.Contains(pattern, "/") {
			if matched, _ := filepath.Match(pattern, filepath.Base(normalized)); matched {
				return false
			}
		}
	}

	// If allowed paths is specified, path must match at least one
	if len(c.AllowedPaths) > 0 {
		for _, pattern := range c.AllowedPaths {
			if matched, _ := filepath.Match(pattern, normalized); matched {
				return true
			}
			if !strings.Contains(pattern, "/") {
				if matched, _ := filepath.Match(pattern, filepath.Base(normalized)); matched {
					return true
				}
			}
		}
		return false
	}
	return true
}

// RequiresApproval checks whether the given risk level requires approval
// under this contract.
func (c TaskContract) RequiresApproval(riskLevel string) bool {
	for _, level := range c.RequireApprovalForRisk {
		if level == riskLevel {
			return true
		}
	}
	return false
}

// Validate checks the contract for internal consistency.
func (c TaskContract) Validate() error {
	if strings.TrimSpace(c.Goal) == "" {
		return fmt.Errorf("task contract: goal is required")
	}
	if strings.TrimSpace(c.RepoRoot) == "" {
		return fmt.Errorf("task contract: repo_root is required")
	}
	if c.MaxSteps <= 0 {
		return fmt.Errorf("task contract: max_steps must be positive")
	}
	// Check for contradictions: a tool in both allowed and denied
	allowedSet := make(map[string]bool, len(c.AllowedTools))
	for _, t := range c.AllowedTools {
		allowedSet[t] = true
	}
	for _, t := range c.DeniedTools {
		if allowedSet[t] {
			return fmt.Errorf("task contract: tool %q is both allowed and denied", t)
		}
	}
	return nil
}

func generateContractID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate contract id: %w", err)
	}
	return "tc_" + hex.EncodeToString(b), nil
}
