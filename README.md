# MimoNeko

MimoNeko is a local-first terminal AI coding workbench optimized for MIMO
models. It combines an OpenCode-inspired TUI, MIMO-first provider setup,
context-cache visibility, safe tool execution, patch preview, and dry-run agent
modes in one small Go binary.

Current version: `0.1.4-beta`

MimoNeko is safe by default: it does not auto commit, auto push, or write API
keys into the project directory. Write-capable flows are gated by permission
mode, patch preview, and explicit approval.

## Quick Start

After installing, open a new terminal and run:

```bash
mimoneko
```

For a fresh checkout:

```bash
mimoneko init
mimoneko doctor
mimoneko model setup --preset mimo --provider mimo --model mimo-v2.5-pro --set-default
```

API keys are stored in user-level configuration or environment variables. The
project directory stores provider metadata only, such as provider name, base URL,
model name, and API-key environment variable name.

## Common Commands

```bash
mimoneko                         # Start the interactive TUI
mimoneko "fix the failing tests" # Run a single goal
mimoneko run "inspect this repo" # Explicit agent run
mimoneko model setup             # Configure provider/model
mimoneko model test              # Test provider connection
mimoneko doctor                  # Check local setup
mimoneko --help                  # Show CLI help
```

Common TUI commands:

```text
/              Open command panel
/models        Switch model
/connect       Connect provider
/cache         Show context/cache usage
/agents        Switch agent mode
/diff          Show diff
/editor        Open editor entry
/new           New session
/help          Help
/exit          Exit
```

## MIMO-First Provider Setup

Default provider preset:

- `mimo`

Optional OpenAI-compatible presets:

- `openai`
- `glm`
- `custom-openai-compatible`

Example:

```bash
mimoneko model setup --preset mimo --provider mimo --model mimo-v2.5-pro --set-default
mimoneko model discover --provider mimo
mimoneko model use mimo-v2.5-pro
```

## TUI Agent Modes

`Build`

- Multi-agent/worktree-oriented engineering workflow.
- Uses patch-preview-first behavior.
- Intended for planning, reviewing, and safe implementation flow.

`Single`

- Single-agent direct chat and lightweight task mode.
- Better for explanations, quick Q&A, and small repo checks.

`Explore`, `Plan`, `Builder`, and `Reviewer`

- Structured foundations for the future real AgentLoop.
- Each mode declares description, allowed tools, write permission, and worktree
  behavior.
- Current implementation remains conservative and dry-run friendly.

## Safety Model

MimoNeko defines five permission modes:

- `chat`
- `read-only`
- `plan`
- `patch-preview`
- `apply-with-approval`

Protected paths are denied by default, including `.git/`, `.env`, `.env.*`,
private-key filenames, `secrets.*`, and writes outside the project root.

Patch application requires:

1. patch preview
2. `MIMONEKO_PERMISSION_MODE=apply-with-approval`
3. explicit approval

## Cache Visibility

Use `/cache` in the TUI or `mimoneko cache-report` in the CLI to inspect context
usage, input tokens, cached tokens, and cache hit rate. If a provider does not
return cached-token data, MimoNeko reports `unsupported` instead of failing.

## Local Development

```bash
go test ./...
go vet ./...
git diff --check
```

Build locally:

```bash
go build ./cmd/mimoneko
go build ./cmd/neko
```

Build release archives:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1 v0.1.4-beta
```

## Current Focus

- Stabilize the terminal workbench experience.
- Keep MIMO cache behavior observable.
- Harden permission checks before enabling real write-capable AgentLoop flows.
- Preserve `neko` as a compatible entry while standardizing docs and commands on
  `mimoneko`.
