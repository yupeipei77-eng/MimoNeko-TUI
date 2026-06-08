# MimoNeko Quickstart

MimoNeko is a local Go TUI for MIMO-first AI coding workflows. It defaults to the
`mimo-v2.5-pro` model and keeps secrets outside project files.

## Prerequisites

- Go 1.22 or newer
- Git
- A MIMO API key in `MIMO_API_KEY`

## Build

```powershell
go build -o mimoneko.exe ./cmd/mimoneko
```

## Configure

```powershell
$env:MIMO_API_KEY = "your-api-key"
go run ./cmd/mimoneko init
go run ./cmd/mimoneko doctor
```

For persistent Windows configuration:

```powershell
setx MIMO_API_KEY "your-api-key"
```

## Start The TUI

```powershell
go run ./cmd/mimoneko
```

Inside the TUI:

- Type normally to chat.
- Type `/` or press `ctrl+p` to open commands.
- Use `/models` to switch model.
- Use `/connect` to connect a provider.
- Use `/agents` to switch agent mode.
- Use `/cache` to inspect context and cache statistics.
- Use `/exit` to quit.

## Useful CLI Commands

```powershell
go run ./cmd/mimoneko models
go run ./cmd/mimoneko model test --provider mimo --model mimo-v2.5-pro --prompt "OK"
go run ./cmd/mimoneko patch list
go run ./cmd/mimoneko patch preview <worktree-id>
go run ./cmd/mimoneko patch apply <worktree-id> --approve
```

Patch apply requires `MIMONEKO_PERMISSION_MODE=apply-with-approval` and the
explicit `--approve` flag. MimoNeko does not auto commit or auto push.

## Safe Defaults

- Default provider: `mimo`
- Default model: `mimo-v2.5-pro`
- Default permission mode: `patch-preview`
- API keys are masked in doctor output and never written to YAML.
- Direct writes are blocked unless permission mode and explicit approval allow them.
