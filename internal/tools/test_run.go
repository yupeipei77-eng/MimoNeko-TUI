package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// TestRunTool executes predefined test commands from the configuration.
// It does NOT allow arbitrary user-input commands.
type TestRunTool struct {
	// Commands maps command_name to the preconfigured command slice.
	Commands map[string]TestCommandDef
}

// TestCommandDef defines a predefined test command.
type TestCommandDef struct {
	Command        []string
	TimeoutSeconds int
}

func (t *TestRunTool) Name() string { return "test_run" }
func (t *TestRunTool) Description() string {
	return "Execute predefined test commands from configuration"
}
func (t *TestRunTool) RiskLevel() string             { return "medium" }
func (t *TestRunTool) Concurrency() ConcurrencyClass { return ConcurrencyReadOnly }

func (t *TestRunTool) Run(ctx context.Context, req ToolRequest) (ToolResponse, error) {
	commandName, ok := req.Args["command_name"]
	if !ok || commandName == "" {
		return toolError("test_run", "arg 'command_name' is required"), nil
	}

	// Look up the predefined command
	def, found := t.Commands[commandName]
	if !found {
		return toolError("test_run", fmt.Sprintf("command_name %q is not configured; allowed commands: %v", commandName, t.configuredNames())), nil
	}

	if len(def.Command) == 0 {
		return toolError("test_run", fmt.Sprintf("command %q has empty command definition", commandName)), nil
	}

	// Determine timeout
	timeoutSec := def.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 120
	}

	timeout := time.Duration(timeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute the command with minimal environment
	cmd := exec.CommandContext(ctx, def.Command[0], def.Command[1:]...)
	cmd.Dir = req.RepoRoot
	cmd.Env = minimalEnv()

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return toolError("test_run", fmt.Sprintf("command %q timed out after %d seconds", commandName, timeoutSec)), nil
		} else {
			return toolError("test_run", fmt.Sprintf("command %q: %v", commandName, err)), nil
		}
	}

	out := stdout.String()
	se := stderr.String()

	resp := ToolResponse{
		ToolName:    "test_run",
		Success:     exitCode == 0,
		ExitCode:    exitCode,
		Stdout:      out,
		Stderr:      se,
		OutputBytes: len(out) + len(se),
	}
	if exitCode != 0 {
		resp.Error = fmt.Sprintf("command %q exited with code %d", commandName, exitCode)
	}
	return resp, nil
}

func (t *TestRunTool) configuredNames() []string {
	names := make([]string, 0, len(t.Commands))
	for name := range t.Commands {
		names = append(names, name)
	}
	return names
}

// minimalEnv returns a minimal set of environment variables safe to pass to test commands.
// It does NOT pass API keys or full environment.
func minimalEnv() []string {
	// Only inherit PATH and system basics
	var env []string
	for _, e := range os.Environ() {
		upper := strings.ToUpper(e)
		// Only pass PATH, HOME, USER, TEMP, TMP, SYSTEMROOT, COMSPEC
		if strings.HasPrefix(upper, "PATH=") ||
			strings.HasPrefix(upper, "HOME=") ||
			strings.HasPrefix(upper, "USER=") ||
			strings.HasPrefix(upper, "TEMP=") ||
			strings.HasPrefix(upper, "TMP=") ||
			strings.HasPrefix(upper, "SYSTEMROOT=") ||
			strings.HasPrefix(upper, "COMSPEC=") {
			// Never pass API key values
			if !strings.Contains(upper, "API_KEY") &&
				!strings.Contains(upper, "SECRET") &&
				!strings.Contains(upper, "TOKEN") &&
				!strings.Contains(upper, "PASSWORD") {
				env = append(env, e)
			}
		}
	}
	return env
}
