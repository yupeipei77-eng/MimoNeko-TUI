# cmd/NekoMIMO

## Responsibilities

- Provide the `NekoMIMO` executable entry point.
- Delegate all command behavior to `internal/cli`.

## Boundaries

- This package should only wire process IO, args, and exit codes.
- CLI behavior belongs in `internal/cli`.

## Forbidden

- Do not load model providers here.
- Do not perform agent orchestration here.
- Do not write runtime state outside the CLI contract.
