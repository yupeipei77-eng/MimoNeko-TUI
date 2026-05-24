# patch — Patch Preview, Apply & Discard

This package implements the patch approval workflow for ReasonForge. After an
agent finishes modifying files inside an isolated worktree, the patch manager
produces a diff summary, checks for policy violations, and provides
apply/discard operations.

## Core Types

| Type | File | Purpose |
|------|------|---------|
| `PatchManager` | `manager.go` | Interface: Preview / Apply / Discard |
| `GitPatchManager` | `git_patch.go` | Production implementation using `git diff` / `git apply` |
| `PatchPreview` | `manager.go` | Diff summary + violations + risk level |
| `PatchApplyResult` | `manager.go` | Outcome of applying a patch |

## Workflow

1. **Preview** — Generates a diff between the worktree and its base ref,
   lists changed files, checks for violations, and assesses risk.
2. **Apply** — Copies approved changes into the main workspace via
   `git apply`. Supports `DryRun` mode. Refuses if violations exist or the
   main workspace is dirty (`.reasonforge/` paths are ignored).
3. **Discard** — Removes the worktree entirely; the main workspace is
   unaffected.

## Violation Checks

- **TaskContract**: `AllowedPaths` / `DeniedPaths` from the agent's contract.
- **Hard-coded deny**: `.git`, `.reasonforge`, `.env`, `*.pem`, `*.key`.
- **Sensitive files**: `credentials`, `secrets`, `token`, `password`.
- **Protected directories**: `.github/workflows`, `.ci`, `deploy`.
- **Binary files**: Rejected by default (`AllowBinary: false`).

## Risk Assessment

| Level | Condition |
|-------|-----------|
| Low | No violations, text-only, ≤ 5 files, ≤ 200 lines changed |
| Medium | No violations but exceeds low thresholds |
| High | Any violation detected |

## Configuration

Loaded from `patch.yaml` (optional, missing file uses defaults):

```yaml
max_diff_bytes: 1048576
require_clean_main: true
allow_binary: false
```
