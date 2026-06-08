# Phase 3: Tool Runtime

## Architecture

The Tool Runtime is MimoNeko's secure local tool execution layer. It provides a unified, safety-guarded pipeline for all tool operations:

```
ToolRequest -> SafetyGuard -> ToolRuntime -> Tool -> ToolResponse -> AuditLog
```

No business code may call a Tool implementation directly. All invocations must go through `ToolRuntime.Run()`.

## Core Types

### ToolRuntime

```go
type ToolRuntime interface {
    Run(ctx context.Context, req ToolRequest) (ToolResponse, error)
}
```

The central orchestrator. It handles:
1. Tool lookup in the registry
2. Enabled/disabled check
3. RepoRoot validation
4. Audit ID generation and pre-execution logging
5. Timeout context creation
6. Tool execution
7. Output truncation
8. Post-execution audit logging

### Tool

```go
type Tool interface {
    Name() string
    Description() string
    RiskLevel() string
    Run(ctx context.Context, req ToolRequest) (ToolResponse, error)
}
```

Every tool implements this interface. The `RiskLevel` must be `"low"` or `"medium"`.

### ToolRegistry

```go
type ToolRegistry interface {
    Register(tool Tool) error
    Get(name string) (Tool, bool)
    List() []ToolInfo
}
```

In-memory registry. Duplicate registration by name is rejected.

### ToolRequest

| Field | Type | Description |
|-------|------|-------------|
| ToolName | string | Which tool to invoke |
| RepoRoot | string | Workspace root (required) |
| TaskID | string | Calling task for audit tracing |
| Args | map[string]string | Tool-specific arguments |
| MaxOutputBytes | int | Output cap (0 = use policy default) |
| TimeoutSeconds | int | Execution timeout (0 = use policy default) |
| DryRun | bool | Report without side effects |
| Metadata | map[string]string | Optional caller metadata |

### ToolResponse

| Field | Type | Description |
|-------|------|-------------|
| ToolName | string | Echoes the invoked tool |
| Success | bool | Whether the tool completed |
| ExitCode | int | Process-like exit code |
| Stdout | string | Primary output |
| Stderr | string | Error/diagnostic output |
| OutputBytes | int | Bytes before truncation |
| Truncated | bool | Output was cut |
| Artifacts | []ToolArtifact | Files created/modified |
| AuditID | string | Links to audit log |
| Error | string | Human-readable error |

## Safety Boundaries

### Workspace Root Restriction

All file operations must stay within `RepoRoot`. The `safePath()` function:
- Rejects absolute paths (Unix `/` and Windows drive letters)
- Rejects path traversal (`..`)
- Verifies resolved path is under root

### Sensitive Path Protection

**Write denied** (default):
- `.git`, `.mimoneko`
- `.env`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519`

**Read denied** (default):
- `.env`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519`

### Output Limits

Each tool has `MaxOutputBytes` (default: 65536). Output exceeding this is truncated and `Truncated=true` is set.

### Timeout

Each tool has `TimeoutSeconds` (default: 30). Expired contexts are cancelled.

### Risk Levels

| Tool | Risk Level |
|------|-----------|
| file_read | low |
| git_diff | low |
| file_write | medium |
| file_patch | medium |
| test_run | medium |

## Built-in Tools

### file_read

**Args:** `path` (required), `max_bytes` (optional)

- Reads file within RepoRoot
- Default max: 256KB
- Rejects sensitive files (.env, *.pem, *.key, id_rsa, id_ed25519)
- Truncates if exceeds max_bytes

### file_write

**Args:** `path` (required), `content` (required), `create_dirs` (optional, default true)

- Writes file within RepoRoot
- Rejects .git, .mimoneko, sensitive paths
- Supports DryRun (reports without writing)
- Creates parent directories by default
- File permissions: 0644

### file_patch

**Args:** `path` (required), `old` (required), `new` (required)

- Simple text replacement (not unified diff)
- `old` must exist exactly once (fail if 0 or >1 occurrences)
- Supports DryRun
- Records artifact

### git_diff

**Args:** `path` (optional)

- Only executes `git diff` (no arbitrary git commands)
- Optional path filter (validated against RepoRoot)
- Output subject to MaxOutputBytes

### test_run

**Args:** `command_name` (required)

- Only executes commands predefined in `tools.yaml` under `test_commands`
- No arbitrary user-input commands
- Minimal environment (PATH, HOME, USER, TEMP, SYSTEMROOT only; no API keys)
- Working directory: RepoRoot
- Command-specific timeout (default: 120s)

## Audit Log

Every `ToolRuntime.Run()` writes two audit events (pre- and post-execution) as JSONL.

**Default path:** `.mimoneko/logs/tools.jsonl`

**Directory permissions:** 0700
**File permissions:** 0600

**ToolAuditEvent structure:**

| Field | Description |
|-------|-------------|
| ID | crypto/rand 16-byte hex |
| Timestamp | Time of event |
| ToolName | Tool invoked |
| TaskID | Task identifier |
| RepoRoot | Workspace root |
| ArgsRedacted | Args with sensitive values replaced |
| Success | Execution result |
| ExitCode | Process exit code |
| OutputBytes | Output size |
| Error | Error message |
| DurationMs | Execution duration |
| RiskLevel | Tool risk classification |
| DryRun | Whether dry-run mode |

**Redaction:** The `content` argument is always replaced with `"<redacted>"`.

**Failure policy:**
- Audit start failure -> tool execution is blocked (returns error)
- Audit finish failure -> tool result is returned with an error

## Configuration (tools.yaml)

```yaml
tools:
  - name: file_read
    kind: builtin
    enabled: true
    risk_level: low
  - name: file_write
    kind: builtin
    enabled: true
    risk_level: medium
  - name: file_patch
    kind: builtin
    enabled: true
    risk_level: medium
  - name: git_diff
    kind: builtin
    enabled: true
    risk_level: low
  - name: test_run
    kind: builtin
    enabled: true
    risk_level: medium

test_commands:
  - name: go-test
    command: ["go", "test", "./..."]
    timeout_seconds: 120

policy:
  max_output_bytes: 65536
  default_timeout_seconds: 30
  deny_write_paths:
    - ".git"
    - ".mimoneko"
    - ".env"
    - "*.pem"
    - "*.key"
    - "id_rsa"
    - "id_ed25519"
  deny_read_paths:
    - ".env"
    - "*.pem"
    - "*.key"
    - "id_rsa"
    - "id_ed25519"
```

## CLI

### MimoNeko tools

Lists available tools, their enabled status, and risk levels.

### MimoNeko tool-run

Executes a tool with arguments. Subject to all safety policies.

```sh
MimoNeko tool-run file_read --path README.md
MimoNeko tool-run git_diff
MimoNeko tool-run test_run --command_name go-test
MimoNeko tool-run file_write --path output.txt --content "hello" --dry-run
```

## Phase 4 Integration: Agent Runtime

The Tool Runtime is designed for Phase 4 Agent Runtime integration:

1. **ToolResponse -> Scratchpad:** `ToolResponseToScratchpadItem()` converts a ToolResponse to a scratchpad-compatible item. The Agent Runtime will inject this into the Volatile Scratchpad layer.

2. **Approval mechanism:** `ToolResponse` and `ToolAuditEvent` have fields (`RiskLevel`, `DryRun`) that support future human-approval workflows. The Agent Runtime can implement a `requires_approval` check based on risk level.

3. **Tool result injection:** The Agent Runtime will call `ToolRuntime.Run()` when the model emits a tool call, then inject the ToolResponse back into the conversation via the ContextEngine's Bundle layering.

4. **No Scratchpad pollution:** Tool outputs are NOT automatically written to the Scratchpad. The Agent Runtime decides what to persist.

5. **No Immutable Prefix impact:** Tool results belong in the Volatile layer, never in the Immutable Prefix.
