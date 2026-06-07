# internal/version

## Responsibilities

- Hold build-time name and version information.
- Provide a stable string for `MimoNeko version`.

## Boundaries

- This package has no dependency on configuration, runtime state, or Git metadata.
- Release builds may override `Version` with `go build -ldflags "-X github.com/mimoneko/mimoneko/internal/version.Version=vX.Y.Z"`.

## Forbidden

- Do not perform filesystem or network IO.
- Do not infer versions from dynamic runtime data.
