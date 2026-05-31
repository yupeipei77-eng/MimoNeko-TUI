# internal/task

## Responsibilities

- Define the `TaskContract` interface.
- Capture objective, repo root, allowed tools, security profile, and worktree policy.

## Boundaries

- A task contract is the runtime permission envelope.
- Worktree isolation is represented here before execution begins.

## Forbidden

- Do not start agent work without a task contract.
- Do not silently disable required worktree isolation.
- Do not store task state by rewriting conversation history.
