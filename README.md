# ReasonForge

ReasonForge is a local-first AI coding agent runtime foundation.

The current repository is intentionally an MVP skeleton. It defines stable contracts for context assembly, byte-stable immutable prefixes, append-only logs, volatile scratchpads, OpenAI-compatible model routing, tool execution, task contracts, memory, and repository indexing.

## Commands

```sh
reasonforge version
reasonforge init
reasonforge doctor
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

This is the foundation pass. Complex features are represented as interfaces and TODOs, not hidden partial implementations.
