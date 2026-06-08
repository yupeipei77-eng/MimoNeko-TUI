# MimoNeko Architecture

MimoNeko is a local-first Go TUI for MIMO-focused AI coding workflows. The
current v0.1.4-beta goal is to keep the workbench stable and safe while laying
the interfaces for real agent loops, permission handling, and cache-aware model
usage.

## Entry Points

```text
cmd/mimoneko/        Primary CLI and TUI entry point
cmd/neko/            Compatibility entry point, if built
```

Running `mimoneko` with no arguments opens the TUI when a provider is already
configured. Otherwise it starts first-time setup.

## Core Layers

- `internal/neko`: terminal UI, command picker, model picker, provider flow,
  agent picker, status footer, cache report, native cursor placement.
- `internal/config`: default `.mimoneko/*.yaml` configuration and MIMO-first
  defaults.
- `internal/auth`: user provider auth, environment loading, and API key masking.
- `internal/modelrouter`: provider request construction and model usage records.
- `internal/cache`: cache statistics and context usage accounting.
- `internal/tools`: tool execution, enforcement, and permission checks.
- `internal/security`: `PermissionMode` and project path guard.
- `internal/patch`: worktree patch preview/apply pipeline.
- `internal/agent` and `internal/multiagent`: foundations for future agent loops.

## MIMO Defaults

MimoNeko defaults to:

- provider: `mimo`
- model: `mimo-v2.5-pro`
- API key env var: `MIMO_API_KEY`
- base URL: `https://token-plan-cn.xiaomimimo.com/v1`
- reasoning level: `high`
- prefix-cache support enabled when provider usage data is available

Other provider presets may exist for compatibility, but the default experience
is optimized for MIMO.

## Permission Model

Permission modes are explicit:

- `chat`: chat only.
- `read-only`: read tools and safe inspection.
- `plan`: planning and dry-run behavior.
- `patch-preview`: default; can prepare and preview changes.
- `apply-with-approval`: may apply writes only with explicit approval metadata.

Writes are guarded by both permission mode and path checks. The path guard blocks
`.git/`, `.env`, `.env.*`, private key names, `secrets.*`, paths outside the
project root, and user home paths that are not under the active project.

## Patch Flow

Patch apply is never automatic.

```powershell
mimoneko patch list
mimoneko patch preview <worktree-id>
$env:MIMONEKO_PERMISSION_MODE = "apply-with-approval"
mimoneko patch apply <worktree-id> --approve
```

The apply command requires both `apply-with-approval` mode and `--approve`.

## TUI Model

The TUI avoids complex overlay forms for provider creation. `/models` still opens
the model picker, but choosing provider connection starts a step-based composer
flow. This keeps Windows Terminal rendering predictable and avoids overlapping
modal borders.

Input rendering uses the terminal native cursor. Cursor placement is based on
display width, so Chinese wide characters and emoji are handled without fake
cursor artifacts.

## Cache Reporting

The status footer and `/cache` command expose context and cache statistics:

- context usage
- input tokens
- cached tokens
- hit rate

Providers that do not report cached tokens are shown as `unsupported`.

## Agent Mode Foundation

The TUI defines `AgentMode` entries for:

- Build
- Single
- Explore
- Plan
- Builder
- Reviewer

Each mode declares name, description, allowed tools, write permission, and
worktree expectation. The current implementation can display and switch modes;
future AgentLoop work should execute through these declarations.

## Validation

The expected local validation commands are:

```powershell
go test ./...
go vet ./...
git diff --check
```
