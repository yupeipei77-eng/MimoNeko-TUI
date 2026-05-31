# internal/neko

`internal/neko` implements the NekoForge terminal console.

Scope:

- standard-library terminal I/O
- compact cold-cyan ANSI branding with `--no-color`
- OpenCode-style runtime composer and compact terminal stream rendering
- `/` command palette plus `/agents`, `/models`, `/new`, and `/reasoning` cycling
- runtime event stream and folded thought summary
- dim status bar with context usage/tools/memory message count/model/provider/session hints
- slash command parsing
- plain-text chat separate from `/run` agent execution
- explicit save/write/generate-file chat requests auto-save generated content
- model/profile display without API key leakage
- token and configured-pricing cost display
- safe handoff to existing NekoMIMO run, patch, model, and runs commands

Out of scope:

- desktop apps
- web workspaces
- auto-apply
- auto-commit
- auto-push
- hidden reasoning display
