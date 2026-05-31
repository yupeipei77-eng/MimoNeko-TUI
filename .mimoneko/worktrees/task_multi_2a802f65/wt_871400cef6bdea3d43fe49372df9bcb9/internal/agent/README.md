# internal/agent

## Responsibilities

- Define the `AgentRuntime` contract.
- Coordinate context assembly, model routing, tool runtime, conversation log, and task contract.

## Boundaries

- The agent runtime orchestrates dependencies but should not own their storage details.
- Every model call, tool call, patch, test, and rollback must be observable through append-only events.

## Forbidden

- Do not implement hidden provider-specific calls.
- Do not bypass the context engine.
- Do not mutate repository state without task contract permission and event logging.
