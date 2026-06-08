# Phase 4: Agent Runtime

## Purpose

Phase 4 introduces the safe single-agent execution loop. The runtime connects
task contracts, context building, model calls, tool calls, approval checks, and
checkpointing.

## Loop

```text
TaskContract
  -> ContextEngine.Build
  -> ModelRouter.Complete
  -> parse tool call
  -> ApprovalPolicy
  -> ToolRuntime.Run
  -> Scratchpad item
  -> Checkpoint
```

The agent must not read or write files directly. All local actions go through
registered tools and the tool runtime safety layer.

## Default Safety

The default contract allows read-only work:

- allowed tools: `file_read`, `git_diff`, `test_run`
- denied tools: `file_write`, `file_patch`
- max steps: 5
- dry run: true
- medium-risk tools require approval
- high-risk tools are blocked

System-level `PermissionMode` checks still apply even when a task contract would
allow an action.

## Checkpoints

`JSONLCheckpointStore` writes append-only checkpoints under
`.mimoneko/logs/checkpoints.jsonl`. Checkpoints capture model and tool steps so
future agent loops can resume or audit previous runs.

## CLI

```sh
mimoneko run --goal "Read the README and summarize it"
mimoneko run --goal "Run the tests" --dir /path/to/project
```

Write-capable flows remain gated by permission mode and explicit approval.

## Current Limits

- Structured tool calling is still text/JSON based.
- Multi-agent orchestration is handled by later phases.
- Real write application must go through patch preview or approval modes.
