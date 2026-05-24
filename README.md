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
reasonforge patch validate <worktree_id> [--test-command go-test]
reasonforge patch review <worktree_id> [--model-review] [--test-command go-test] [--no-tests]
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
- Patch review recommendation is deterministic; safety rules override model suggestions.
- Sensitive diff content is never sent to AI models when violations exist.

## Status

Phase 1 (Context Engine + Cache Engine), Phase 2 (Model Router + Usage Accounting), Phase 3 (Tool Runtime + Security Hardening), Phase 4 (Agent Runtime), and Phase 5 (Worktree Isolation + Patch Manager) are implemented.

Phase 6 adds Patch Review and Validation Pipeline:
- PatchReviewManager: full review pipeline (rule review → risk scoring → test validation → optional model review → recommendation)
- RuleBasedReviewer: violation detection, diff size, file/line counts, binary files, sensitive paths, test coverage, generated files
- RiskScorer: numeric risk score (0-100) with low/medium/high/critical levels
- ValidationRunner: test execution through ToolRuntime (test_run), output truncation, API key sanitization
- ModelReviewer: optional AI review via ModelRouter with sensitive diff guard
- Deterministic recommendation: approve / request_changes / reject
- CLI: `reasonforge patch validate`, `reasonforge patch review`
- Configuration: review.yaml, validation.yaml with safe defaults
