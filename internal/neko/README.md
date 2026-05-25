# internal/neko

`internal/neko` implements the NekoForge terminal console.

Scope:

- standard-library terminal I/O
- cat-themed ANSI rendering with `--no-color`
- slash command parsing
- model/profile display without API key leakage
- token and configured-pricing cost display
- safe handoff to existing ReasonForge run, patch, model, and runs commands

Out of scope:

- desktop apps
- web workspaces
- auto-apply
- auto-commit
- auto-push
- hidden reasoning display
