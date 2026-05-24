# validation

Test validation runner for ReasonForge.

## Responsibilities

- Execute test commands through ToolRuntime (test_run tool)
- Structure test results for review pipeline
- Sanitize output to prevent API key leakage

## Boundaries

- Must execute through ToolRuntime, never directly exec
- TestCommands must be command_names from tools.yaml
- RepoRoot should be worktree path, not main workspace
- Output capped by MaxOutputBytes
- Timeout enforced
- ValidationResult must not leak API keys

## Forbidden

- Do not execute arbitrary shell commands
- Do not bypass ToolRuntime
- Do not leak API keys in validation results
