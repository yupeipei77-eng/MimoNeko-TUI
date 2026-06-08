# MimoNeko User Test Plan

This checklist is for validating a fresh release candidate before publishing.

## Install

Preferred release path:

1. Download the archive for your platform from the release page.
2. Extract it.
3. Run `install.ps1` on Windows or `install.sh` on macOS/Linux.
4. Open a new terminal.

Source build path:

```sh
git clone <repository-url>
cd MimoNeko
go build -o mimoneko ./cmd/mimoneko
go build -o neko ./cmd/neko
```

Module install path:

```sh
go install github.com/yupeipei77-eng/MimoNeko-TUI/cmd/mimoneko@main
```

Checks:

- `mimoneko version` prints the current release version.
- `mimoneko --help` shows MimoNeko commands.
- `neko` still opens the compatible TUI entry.

## First Run

```sh
mimoneko init
mimoneko doctor
```

Checks:

- User config path is reported.
- Project config is created under `.mimoneko/`.
- Doctor does not print raw API keys.
- Missing provider/API key messages are actionable.

## MIMO Provider Setup

```sh
mimoneko model setup --preset mimo --provider mimo --model mimo-v2.5-pro --set-default
mimoneko model test --provider mimo --model mimo-v2.5-pro --prompt "Reply OK only"
```

Checks:

- API key input is masked.
- Base URL defaults to the MIMO-compatible endpoint.
- Default model becomes `mimo-v2.5-pro`.
- Failed connection errors do not leak secrets.

## TUI Smoke Tests

```sh
mimoneko
```

Checks:

- Chinese input such as `你好` renders without fake cursor artifacts.
- Emoji and wide characters do not break cursor placement.
- `/models` opens the model picker.
- Selecting `Connect provider` starts the provider flow without an overlay form.
- API key entry is masked.
- `/agents` opens the agent mode picker.
- `/cache` displays usage or `unsupported` when provider cache metrics are absent.
- `Esc` closes pickers and flows cleanly.
- Resizing the terminal does not leave stale modal fragments.

## Safety Smoke Tests

Default mode must be safe:

```sh
mimoneko security
mimoneko patch apply wt_fake --dry-run
```

Checks:

- Write operations require patch preview or explicit approval.
- Direct writes to `.git/`, `.env`, `.env.*`, private keys, and `secrets.*` are denied.
- Auto commit and auto push are not performed.

## Validation Commands

Run before publishing:

```sh
go test ./...
go vet ./...
git diff --check
```

Optional release smoke:

```sh
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1 v0.1.4-beta
```

## Known Release Note

The public `go install` path is aligned with the GitHub repository path and the
`go.mod` module path on `main`. Until a newer semver tag is published, use
`@main`; older tags still point at the pre-rename module path.

```sh
go install github.com/yupeipei77-eng/MimoNeko-TUI/cmd/mimoneko@main
```
