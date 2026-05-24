# ReasonForge

ReasonForge is a local-first AI coding agent runtime foundation.

The current repository is intentionally an MVP skeleton. It defines stable contracts for context assembly, byte-stable immutable prefixes, append-only logs, volatile scratchpads, OpenAI-compatible model routing, tool execution, task contracts, agent runtime, memory, and repository indexing.

## Commands

```sh
reasonforge version
reasonforge init
reasonforge doctor
reasonforge models
reasonforge cache-report
reasonforge tools
reasonforge tool-run <tool-name> [--key value ...]
reasonforge run --goal "read the README" [--dry-run] [--max-steps 5] [--auto-approve-medium] [--worktree]
reasonforge patch list
reasonforge patch preview <worktree_id>
reasonforge patch apply <worktree_id> [--dry-run]
reasonforge patch discard <worktree_id>
```

## Design Constraints

- Local state lives under `.reasonforge/` by default.
- Model providers must be OpenAI-compatible.
- Immutable prefix bytes must stay stable across runs.
- Dynamic context belongs in volatile scratchpad or conversation tail.
- Conversation and task events are append-only.
- Memory records are retrieved into volatile context, never injected directly into the immutable prefix.
- Worktree isolation is a task contract requirement, not an afterthought.
- Agent execution must respect TaskContract boundaries.
- Contract-level and system-level (ToolRuntime) security must both pass.
- No tool execution without ApprovalPolicy consent for medium/high-risk tools.
- Patch apply must not modify .git, .reasonforge, .env, *.pem, *.key files.
- Worktree paths must stay under .reasonforge/worktrees.

## Status

Phase 1 (Context Engine + Cache Engine), Phase 2 (Model Router + Usage Accounting), Phase 3 (Tool Runtime + Security Hardening), and Phase 4 (Agent Runtime) are implemented.

Phase 5 adds Worktree Isolation and Patch Manager:
- WorktreeManager: isolated git worktree creation, removal, listing
- PatchManager: diff preview, violation checking, apply, discard
- AgentRuntime worktree integration: --worktree flag for isolated execution
- CLI: `reasonforge run --worktree`, `reasonforge patch list/preview/apply/discard`
- Safety: worktree IDs use crypto/rand, paths are sanitized, denied paths enforced
