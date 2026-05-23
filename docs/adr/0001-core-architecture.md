# ADR 0001: Core Runtime Architecture

## Status

Accepted

## Context

ReasonForge needs a stable foundation for a local-first AI coding agent runtime. The runtime must keep token costs low through prefix cache awareness, remain observable, support future Git worktree isolation, and avoid mixing dynamic context into byte-stable prompt material.

Agent systems often become difficult to reason about when model calls, tool execution, memory retrieval, and prompt construction are coupled. ReasonForge starts by separating these surfaces into explicit interfaces.

## Decision

ReasonForge will use a modular Go architecture with these core contracts:

- `ContextEngine`
- `PrefixBuilder`
- `ConversationLog`
- `Scratchpad`
- `CacheRegistry`
- `ModelRouter`
- `ToolRuntime`
- `AgentRuntime`
- `TaskContract`
- `MemoryStore`
- `RepoIndexer`

All model calls must go through `ModelRouter`, which represents OpenAI-compatible providers. The context engine must keep immutable prefix bytes separate from volatile context. Memory and repository search output may be retrieved, but only as volatile context. Conversation and task history are append-only events.

Immutable prefix configuration uses a strict source-kind allowlist instead of filename substring heuristics. The MVP permits only `static_file` and `generated_schema` as immutable source kinds, so static files can safely describe memory or retrieval rules without being misclassified as dynamic content.

Local configuration lives under `.reasonforge/` and is split across:

- `models.yaml`
- `tools.yaml`
- `security.yaml`
- `prefix.yaml`

The MVP provides only `reasonforge version`, `reasonforge init`, and `reasonforge doctor`.

## Consequences

Positive:

- Stable interface boundaries can be implemented incrementally.
- Prefix cache behavior has a first-class representation.
- Dynamic context has a clear place that is not the immutable prefix.
- Append-only event logging supports auditing and rollback reasoning.
- Worktree isolation can be enforced before future mutation features are added.

Negative:

- The MVP does not run real agent tasks yet.
- There is no provider HTTP client yet.
- There is no durable storage implementation beyond config initialization.
- More packages exist up front, but each package has a narrow purpose.

## Follow-up Work

- Implement deterministic prefix building.
- Implement local JSONL conversation and cache registries.
- Implement OpenAI-compatible HTTP transport.
- Implement guarded tool execution.
- Implement local memory and repository indexing.
- Implement worktree preparation and cleanup.
- Add trace IDs across context, model, tool, patch, test, and rollback events.

## Non-goals

- Do not introduce LangChain.
- Do not introduce a microservice architecture.
- Do not add UI.
- Do not add Docker, K8s, SSH, or DB tools.
- Do not put dynamic content into immutable prefix bytes.
- Do not put memory directly into the main prompt.
