package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GitDiffTool runs git diff within the workspace root.
type GitDiffTool struct{}

func (t *GitDiffTool) Name() string        { return "git_diff" }
func (t *GitDiffTool) Description() string  { return "Run git diff within the workspace root" }
func (t *GitDiffTool) RiskLevel() string    { return "low" }

func (t *GitDiffTool) Run(ctx context.Context, req ToolRequest) (ToolResponse, error) {
	guard := safetyGuardFromRequest(req)

	// Build git diff command - only "git diff" is allowed
	args := []string{"diff"}

	// Optional path filter
	if path, ok := req.Args["path"]; ok && path != "" {
		// Validate path stays within repoRoot
		_, err := guard.SafePath(req.RepoRoot, path)
		if err != nil {
			return toolError("git_diff", err.Error()), nil
		}
		args = append(args, "--", path)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = req.RepoRoot

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	// git diff returns exit code 1 when there are differences, which is not an error
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			// Exit code 1 from git diff just means there are changes, not a real error
			if exitCode == 1 && stderr.Len() == 0 {
				exitCode = 0
				err = nil
			}
		} else {
			return toolError("git_diff", fmt.Sprintf("git diff: %v", err)), nil
		}
	}

	out := stdout.String()
	se := stderr.String()
	outputBytes := len(out) + len(se)

	resp := ToolResponse{
		ToolName:   "git_diff",
		Success:    true,
		ExitCode:   exitCode,
		Stdout:     out,
		Stderr:     se,
		OutputBytes: outputBytes,
	}
	return resp, nil
}
