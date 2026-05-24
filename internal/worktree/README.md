# worktree — Git Worktree Isolation Layer

This package provides isolated git worktree management for ReasonForge's agent
runtime. Each agent task that modifies files operates inside a separate git
worktree, ensuring the main workspace remains untouched until the user
explicitly approves a patch.

## Core Types

| Type | File | Purpose |
|------|------|---------|
| `WorktreeManager` | `manager.go` | Interface: Create / Remove / Get / List / UpdateState |
| `GitWorktreeManager` | `git_manager.go` | Production implementation using `git worktree` commands |
| `Registry` | `registry.go` | JSONL append-only store for worktree metadata |

## Security Guarantees

- **ID generation**: `crypto/rand` produces `wt_` + 16-byte hex identifiers.
- **Path traversal prevention**: `SanitizeID` strips `..`, absolute paths, and
  special characters; `IsPathTraversal` rejects unsafe paths outright.
- **Branch sanitization**: `SanitizeBranchName` enforces
  `reasonforge/<sanitized_task_id>/<short_id>` format.
- **Registry permissions**: Directory `0700`, file `0600` (Unix). Metadata
  keys are whitelisted; API keys are never persisted.
- **Hard-coded deny**: Worktrees may never be placed under `.git`,
  `.reasonforge`, `.env`, `*.pem`, or `*.key` paths.

## Configuration

Loaded from `worktree.yaml` (optional, missing file uses defaults):

```yaml
enabled: true
root: ".reasonforge/worktrees"
branch_prefix: "reasonforge"
keep_failed: false
keep_cancelled: false
max_active: 5
```

## Usage

```go
mgr := worktree.NewGitWorktreeManager(repoRoot, registry, cfg)
info, err := mgr.Create(ctx, worktree.CreateWorktreeRequest{
    TaskID: "my-task",
    BaseRef: "HEAD",
})
```
