# Agent Workflow

MioNeko keeps the workflow safe by default: inspect first, plan second, then apply only after explicit review. This phase adds read-only CLI entry points that mirror the review loop used by modern coding agents.

## Current Commands

```bash
neko status
neko diff
neko diff --staged
neko plan --goal "..."
```

The same commands are available through the main binary:

```bash
mimoneko neko status
mimoneko neko diff
mimoneko neko plan --goal "Update docs"
```

## `neko status`

`neko status` reports repository state without invoking the model:

- current git branch
- whether the working tree is clean
- staged, unstaged, and untracked file counts
- latest run status when the project already has an event store

If run events are not configured or no run event file exists, the latest run section reports `unavailable`.

## `neko diff`

`neko diff` prints the working tree diff. It is intended for user review and never applies patches, commits changes, or calls the model.

Use `--staged` to inspect the staged diff:

```bash
neko diff --staged
```

## `neko plan`

`neko plan --goal "..."` prints a structured plan skeleton. The current implementation is intentionally marked as:

```json
{
  "implementation_status": "stub",
  "writes_files": false,
  "calls_model": false
}
```

This lets users review intent without allowing file writes or tool execution.

## Planned Commands

The following write-capable commands are intentionally left for a later phase:

- `neko approve <patch_id>`
- `neko rollback <run_id>`

They require stronger patch identity, event linkage, and rollback safety checks before being enabled.
