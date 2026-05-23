# ReasonForge

ReasonForge is a local-first AI coding agent runtime foundation.

The current repository is intentionally an MVP skeleton. It defines stable contracts for context assembly, byte-stable immutable prefixes, append-only logs, volatile scratchpads, OpenAI-compatible model routing, tool execution, task contracts, memory, and repository indexing.

## Commands

```sh
reasonforge version
reasonforge init
reasonforge doctor
reasonforge models
reasonforge cache-report
reasonforge tools
reasonforge tool-run <tool-name> [--key value ...]
```

## Design Constraints

- Local state lives under `.reasonforge/` by default.
- Model providers must be OpenAI-compatible.
- Immutable prefix bytes must stay stable across runs.
- Dynamic context belongs in volatile scratchpad or conversation tail.
- Conversation and task events are append-only.
- Memory records are retrieved into volatile context, never injected directly into the immutable prefix.
- Worktree isolation is a task contract requirement, not an afterthought.

## Status

Phase 1 (Context Engine + Cache Engine), Phase 2 (Model Router + Usage Accounting), and Phase 3 (Tool Runtime) are implemented.

Phase 3 adds a secure local tool execution layer with:
- ToolRuntime, ToolRegistry, SafetyGuard, and AuditLog
- Built-in tools: file_read, file_write, file_patch, git_diff, test_run
- Workspace root confinement, sensitive path protection, output truncation, timeout enforcement
- JSONL audit logging with crypto/rand IDs and content redaction
- CLI: `reasonforge tools` and `reasonforge tool-run`
