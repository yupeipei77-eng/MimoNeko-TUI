# internal/toolruntime

## Responsibilities

- Define the `ToolRuntime` contract.
- Expose byte-stable tool schemas.
- Execute allowed tool calls and return observable results.

## Boundaries

- Tool schemas may feed immutable prefix if sorted and byte-stable.
- Tool results are dynamic and belong in scratchpad or append-only events.

## Forbidden

- Do not enable dangerous tools by default.
- Do not store tool output in immutable prefix.
- Do not add Docker, K8s, SSH, or DB tools in the MVP.
