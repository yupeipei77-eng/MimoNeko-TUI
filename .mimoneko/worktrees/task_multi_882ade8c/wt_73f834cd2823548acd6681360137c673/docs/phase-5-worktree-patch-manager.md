# Phase 5: Worktree Isolation + Patch Manager

## Overview

Phase 5 implements Git Worktree isolation and Patch Manager, ensuring that Agent file modifications never directly pollute the main workspace.

Core workflow:

```
TaskContract
-> Create isolated git worktree
-> Agent runs tools inside worktree
-> Generate diff summary
-> User reviews patch
-> Apply patch to main workspace or discard worktree
```

## Architecture

### WorktreeManager

**Interface:** `internal/worktree/manager.go`

```go
type WorktreeManager interface {
    Create(ctx context.Context, req CreateWorktreeRequest) (WorktreeInfo, error)
    Remove(ctx context.Context, id string) error
    Get(ctx context.Context, id string) (WorktreeInfo, error)
    List(ctx context.Context) ([]WorktreeInfo, error)
    UpdateState(ctx context.Context, id string, state WorktreeState) error
}
```

**Implementation:** `internal/worktree/git_manager.go` - GitWorktreeManager

Key behaviors:
- Worktrees are created under `.nekonomimo/worktrees/<task_id>/<worktree_id>`
- Branch naming: `NekoMIMO/<sanitized_task_id>/<short_id>`
- Uses `git worktree add -b` to create isolated directories
- Registry stored as append-only JSONL at `.nekonomimo/worktrees/registry.jsonl`
- Registry directory: 0700, file: 0600
- No API keys recorded in registry

### PatchManager

**Interface:** `internal/patch/manager.go`

```go
type PatchManager interface {
    Preview(ctx context.Context, req PatchPreviewRequest) (PatchPreview, error)
    Apply(ctx context.Context, req PatchApplyRequest) (PatchApplyResult, error)
    Discard(ctx context.Context, req PatchDiscardRequest) error
}
```

**Implementation:** `internal/patch/git_patch.go` - GitPatchManager

Key behaviors:
- Preview generates `git diff` and parses changed files
- Preview checks TaskContract AllowedPaths/DeniedPaths
- Preview checks hard-coded deny list (.git, .nekonomimo, .env, *.pem, *.key)
- Apply refuses if violations exist
- Apply refuses if main workspace is dirty
- Apply uses `git apply` to apply changes (no auto-commit)
- DryRun mode outputs diff without modifying main workspace
- Discard removes worktree without affecting main workspace

### AgentRuntime Integration

AgentRunRequest additions:
- `UseWorktree bool` - enable worktree isolation
- `WorktreeID string` - optional existing worktree to use

AgentRunResult additions:
- `WorktreeID string` - ID of the worktree used
- `PatchPreview *patch.PatchPreview` - diff preview from worktree

When UseWorktree=true:
1. Agent creates or reuses a worktree before the loop
2. RepoRoot is redirected to the worktree path
3. All ToolRuntime operations happen inside the worktree
4. Agent does NOT auto-apply after completion
5. Failed/cancelled agents preserve the worktree for debugging
6. PatchPreview is generated if PatchMgr is available

## Security Boundaries

1. Worktree path cannot be specified by the model
2. Worktree path must be under `.nekonomimo/worktrees`
3. Worktree ID uses crypto/rand (16 bytes)
4. task_id is sanitized (no path traversal, no special characters)
5. Branch names are sanitized
6. Remove only removes worktrees in the registry
7. Patch apply checks changed files against TaskContract
8. Patch apply refuses to modify .git, .nekonomimo, .env, *.pem, *.key
9. Patch apply requires clean main workspace (configurable)
10. Binary file patches are denied by default
11. Registry does not record API keys
12. CLI never prints full API key values

## Configuration

### worktree.yaml

```yaml
enabled: true
root: .nekonomimo/worktrees
branch_prefix: NekoMIMO
keep_failed: true
keep_cancelled: true
max_active: 10
```

### patch.yaml

```yaml
max_diff_bytes: 131072
require_clean_main: true
allow_binary: false
```

Both files are optional. Missing files use safe defaults. `NekoMIMO init` creates them with default values.

## CLI Usage

### Run with worktree isolation

```sh
NekoMIMO run --goal "fix typo in README" --worktree
```

This creates a worktree, runs the agent inside it, and outputs:
- `worktree_id=wt_xxxx`
- `patch_preview:` with files_changed, additions/deletions, risk_level
- `review with: NekoMIMO patch preview wt_xxxx`

### List worktrees

```sh
NekoMIMO patch list
```

Output: worktree id, task id, state, path, created_at

### Preview a patch

```sh
NekoMIMO patch preview wt_xxxx
```

Output: files changed, additions/deletions, violations, risk level, diff

### Apply a patch

```sh
NekoMIMO patch apply wt_xxxx
```

Behavior: checks violations, verifies clean main, applies diff (no commit)

```sh
NekoMIMO patch apply wt_xxxx --dry-run
```

Behavior: shows what would be applied without modifying files

### Discard a worktree

```sh
NekoMIMO patch discard wt_xxxx
```

Behavior: deletes worktree, marks state=discarded, no effect on main workspace

## Phase 6 Extension Points

Phase 6 (Multi-Agent) can extend this foundation:
- Multiple agents sharing a single worktree
- Per-agent branches within a worktree
- Patch merge/rebase for multi-agent outputs
- Repo Indexer integration for smarter patch review
- Memory RAG for context-aware patch assessment
