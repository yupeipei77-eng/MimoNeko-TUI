# internal/tools

## Responsibilities

- Define and implement the Tool Runtime: `ToolRuntime`, `Tool`, `ToolRegistry`, `SafetyGuard`, `AuditLog`.
- Provide built-in tools: `file_read`, `file_write`, `file_patch`, `git_diff`, `test_run`.
- Enforce workspace root confinement, sensitive path protection, output truncation, and timeout.
- Write JSONL audit logs for every tool execution.

## Boundaries

- All tool invocations must go through `ToolRuntime.Run()`. Business code must never call a `Tool` implementation directly.
- Tool results are dynamic and belong in volatile scratchpad or append-only events, never in the immutable prefix.
- The `SafetyGuard` is the single authority for path safety decisions.

## Forbidden

- Do not allow arbitrary shell command execution.
- Do not allow tools to access paths outside RepoRoot.
- Do not allow tools to read or write sensitive files (`.env`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519`).
- Do not allow `test_run` to execute commands not predefined in `tools.yaml`.
- Do not store tool output in the immutable prefix.
- Do not add Docker, K8s, SSH, DB, or Browser tools.
