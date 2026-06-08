# MimoNeko Repository Hygiene Cleanup Audit

Date: 2026-06-01

Scope: Git-tracked `.mimoneko` runtime state in the current repository tree.

This cleanup removed `.mimoneko` from the Git index only. It did not delete local
files, did not remove user local configuration, did not commit, did not push, and
did not create a tag.

## Summary

| Check | Result |
|-------|--------|
| Cleanup paused release | v0.1.2-beta remains paused during cleanup |
| Cleanup method | `git rm --cached -r .mimoneko` |
| Local files deleted | No |
| Pre-cleanup tracked `.mimoneko` files | 669 |
| Post-cleanup `git ls-files .mimoneko` | 0 |

Conclusion: `.mimoneko` is runtime/local state and should not be tracked by Git.

## Pre-cleanup Classification

Before cleanup, `git ls-files .mimoneko` reported 669 tracked files.

| Category | Count | Should Git Track? | Notes |
|----------|------:|-------------------|-------|
| Config templates | 0 | No current template found | Top-level YAML files were concrete local config, not templates. |
| Local project config | 10 | No | Generated project config belongs in local state. |
| Test data | 0 | No | No intentional test fixture directory under `.mimoneko`. |
| Example data | 0 | No | No intentional example data under `.mimoneko`. |
| Logs / events | 2 | No | Runtime event and tool logs. |
| Cache | 1 | No | Provider/cache observation data. |
| Checkpoint | 2 | No | Agent/multi-agent checkpoint state. |
| Runtime state / worktrees | 654 | No | Generated worktree registry and copied repository snapshots. |
| Total | 669 | No | Removed from Git index. |

## Cleanup Performed

The following command was executed:

```powershell
git rm --cached -r .mimoneko
```

This removes `.mimoneko` from Git tracking while preserving local files on disk.

Verification:

```powershell
git ls-files .mimoneko
```

Result:

```text
0 files
```

## .gitignore

`.gitignore` now includes the runtime/build-state paths required for this cleanup:

```gitignore
.mimoneko/
logs/
cache/
/dist/
dist/
```

This means future local runtime files should stay untracked.

## Migration Plan

| Category | Former Tracked Path | Keep Local Runtime Path | New Tracked Path | `.gitignore` Rule |
|----------|---------------------|-------------------------|------------------|-------------------|
| Local project config | `.mimoneko/*.yaml` | `.mimoneko/*.yaml` | Optional sanitized examples in `docs/examples/mimoneko/` | `.mimoneko/` |
| Logs / events | `.mimoneko/logs/`, `.mimoneko/events/run_events.jsonl` | `.mimoneko/logs/` or `.mimoneko/runtime/logs/` | None | `.mimoneko/`, `logs/` |
| Cache | `.mimoneko/cache/` | `.mimoneko/cache/` or OS cache dir | None | `.mimoneko/`, `cache/` |
| Checkpoints | `.mimoneko/checkpoints/`, `.mimoneko/logs/checkpoints.jsonl` | `.mimoneko/checkpoints/` or `.mimoneko/runtime/checkpoints/` | None | `.mimoneko/` |
| Worktree runtime state | `.mimoneko/worktrees/` | `.mimoneko/worktrees/` as local-only state | None | `.mimoneko/` |

If sanitized config examples are useful, add them under a non-runtime path such
as:

```text
docs/examples/mimoneko/
```

Do not track runtime files under `.mimoneko/`.

## Security Notes

- Exact scan for the currently configured real MiMo API Key found no project hits.
- Generic key-pattern scan finds only test fixture style fake keys in test files.
- `docs/REAL_WORLD_BENCHMARK.md` does not contain the real API Key.

## Release Impact

The `.mimoneko` current-tree tracking blocker is cleared once this cleanup is
committed, because `git ls-files .mimoneko` now returns zero files.

Before creating `v0.1.2-beta`, still run the standard release checks:

```powershell
go test ./...
git diff --check
```

Do not tag until this cleanup and the CLI UX changes are committed and reviewed.
