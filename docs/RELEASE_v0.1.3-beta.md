# Release v0.1.3-beta

Date: 2026-06-01
Status: beta

## Summary

MioNeko v0.1.3-beta focuses on CLI Modern UX for first-run onboarding. It does not change Agent Runtime, model protocol, or cache behavior.

## Changes

- Fixed `mimoneko` with no arguments showing `Usage` instead of a first-run entry point.
- When no user-level model configuration exists, `mimoneko` now opens the welcome page and setup wizard.
- When configuration already exists, `mimoneko` now shows a friendly `MioNeko Ready` entry page.
- In real TTY terminals, first-run setup uses `survey/v2` arrow-key menus.
- API Key input is hidden in the interactive setup flow.
- Model selection uses an arrow-key menu with the default model first.
- Non-TTY, CI, and piped input keep the line-input fallback for scriptability and tests.
- `mimoneko --help`, `mimoneko -h`, and `mimoneko help` still show `Usage`.
- `mimoneko "Reply OK"` continues to route to `mimoneko run --goal "Reply OK"`.

## Compatibility

- No Agent Runtime changes.
- No cache algorithm changes.
- No model protocol changes.
- No MiMo API type selection; that remains planned for v0.1.4-beta.
- No Web UI changes.

## Verification

Recommended release checks:

```powershell
go test ./...
git diff --check
go build -o mimoneko.exe ./cmd/mimoneko
.\mimoneko.exe version
```

Expected version output:

```text
MimoNeko 0.1.3-beta
```
