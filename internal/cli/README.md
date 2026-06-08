# internal/cli

## Responsibilities

- Implement the minimal CLI commands: `version`, `init`, and `doctor`.
- Keep command output deterministic and easy to test.
- Call configuration setup and validation code.

## Boundaries

- The CLI may create local config files under `.mimoneko/`.
- The CLI may validate configuration shape and safety constraints.
- Runtime execution belongs in `internal/agent`.

## Forbidden

- Do not call models from CLI commands.
- Do not execute tools from CLI commands.
- Do not mutate append-only logs from `doctor`.
