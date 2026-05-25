# internal/neko

`internal/neko` implements the NekoForge terminal console.

Scope:

- standard-library terminal I/O
- minimal centered silver-cyan ANSI branding with `--no-color`
- centered input dialog and conversation card rendering
- slash command parsing
- plain-text chat separate from `/run` agent execution
- explicit save/write/generate-file chat requests auto-save generated content
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
