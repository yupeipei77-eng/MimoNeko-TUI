# internal/toolruntime

## Responsibilities

- Provide compatibility aliases for `internal/tools` package types.
- Allow `internal/agent/` and other existing code to import `toolruntime.ToolRuntime` without migration.

## Boundaries

- This package is a thin compatibility layer over `internal/tools`.
- New code should import `github.com/yupeipei77-eng/MimoNeko-TUI/internal/tools` directly.

## Forbidden

- Do not add new types or logic here; add them to `internal/tools` instead.
