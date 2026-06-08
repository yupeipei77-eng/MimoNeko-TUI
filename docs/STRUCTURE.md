# MimoNeko Project Structure

This file describes the current v0.1.4-beta layout. Legacy project aliases are
no longer used in active documentation.

```text
MimoNeko/
|-- cmd/
|   |-- mimoneko/          # Primary CLI and TUI entry point
|   `-- neko/              # Compatibility entry point, if built
|-- internal/
|   |-- agent/             # Agent runtime skeletons
|   |-- auth/              # User-level auth config and API key masking
|   |-- cache/             # Cache registry and usage accounting
|   |-- cli/               # CLI command dispatch
|   |-- config/            # Default YAML configuration
|   |-- contextengine/     # Context bundle assembly
|   |-- modelrouter/       # Provider/model routing
|   |-- neko/              # Terminal UI, pickers, composer, agent modes
|   |-- patch/             # Worktree patch preview/apply pipeline
|   |-- security/          # PermissionMode and path guard
|   |-- tools/             # Tool runtime and safety checks
|   `-- worktree/          # Git worktree helpers
|-- docs/                  # Design notes and user documentation
|-- prompts/               # Prompt templates
|-- schemas/               # JSON schemas
|-- .env.example           # Environment variable examples
|-- go.mod                 # Go module definition
`-- README.md
```

## Configuration

Project configuration lives in `.mimoneko/` after `mimoneko init`.

```text
.mimoneko/
|-- models.yaml
|-- tools.yaml
|-- security.yaml
|-- prefix.yaml
|-- worktree.yaml
|-- patch.yaml
|-- review.yaml
|-- validation.yaml
|-- multiagent.yaml
`-- events.yaml
```

User-level provider auth is managed separately by the auth layer. API keys are
read from environment variables or user config and are masked in all status
output.

## Current Safety Model

MimoNeko has five permission modes:

- `chat`
- `read-only`
- `plan`
- `patch-preview`
- `apply-with-approval`

The default is `patch-preview`. Direct writes and patch apply require
`apply-with-approval` plus an explicit approval signal. Path guards block writes
to `.git/`, `.env`, `.env.*`, private key names, `secrets.*`, and paths outside
the project root.

## Validation

Before handing off a change, run:

```powershell
go test ./...
go vet ./...
git diff --check
```
