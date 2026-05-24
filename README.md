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
reasonforge run --goal "read the README" [--dry-run] [--max-steps 5] [--auto-approve-medium]
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

## Status

Phase 1 (Context Engine + Cache Engine), Phase 2 (Model Router + Usage Accounting), Phase 3 (Tool Runtime), and Phase 3.1 (Security Hardening) are implemented.

Phase 4 adds the Agent Runtime with:
- TaskContract: execution boundary for every agent run (allowed/denied tools, paths, max steps, approval requirements)
- AgentRuntime interface and SingleAgentRuntime implementation
- Agent loop: ContextEngine.Build -> ModelRouter.Complete -> Parse ToolCall -> ApprovalPolicy -> ToolRuntime.Run -> Scratchpad injection -> Checkpoint
- AgentState lifecycle: pending, running, waiting_approval, succeeded, failed, cancelled
- ToolCall parser for `{"tool_call": {"name": "...", "args": {...}}}` format
- JSONL CheckpointStore for run state persistence
- ApprovalPolicy: risk-based approval (auto-approve low, require approval for medium, block high)
- Interactive approval prompts for CLI
- CLI: `reasonforge run`
