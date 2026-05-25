# ReasonForge

ReasonForge is a local-first AI coding agent runtime foundation.

The current repository is intentionally an MVP skeleton. It defines stable contracts for context assembly, byte-stable immutable prefixes, append-only logs, volatile scratchpads, OpenAI-compatible model routing, tool execution, task contracts, agent runtime, memory, and repository indexing.

## Commands

```sh
reasonforge version
reasonforge init
reasonforge init --repair
reasonforge doctor
reasonforge models
reasonforge model setup
reasonforge model discover --provider mimo
reasonforge model discover --provider mimo --write-capabilities
reasonforge model enrich --provider mimo
reasonforge model test [--prompt "只回复 OK"]
reasonforge model use mimo-v2.5-pro
reasonforge cache-report
reasonforge tools
reasonforge tool-run <tool-name> [--key value ...]
reasonforge run --goal "read the README" [--dry-run] [--max-steps 5] [--auto-approve-medium] [--worktree]
reasonforge multi-run --goal "fix typo in README" [--max-iterations 2] [--dry-run] [--worktree] [--model-review]
reasonforge multi-run "fix typo in README"
reasonforge runs
reasonforge run-status <run_id>
reasonforge run-events <run_id>
reasonforge dashboard
reasonforge serve [--port 9000] [--open]
neko
reasonforge neko
reasonforge patch list
reasonforge patch preview <worktree_id>
reasonforge patch validate <worktree_id> [--test-command go-test]
reasonforge patch review <worktree_id> [--model-review] [--test-command go-test] [--no-tests]
reasonforge patch apply <worktree_id> [--dry-run]
reasonforge patch discard <worktree_id>
```

## Model Provider Setup

ReasonForge stores model provider profiles in `.reasonforge/models.yaml`. API keys are never stored in YAML; the config only stores the environment variable name.

```sh
reasonforge model setup
reasonforge model setup ^
  --preset mimo ^
  --provider mimo ^
  --model mimo-v2.5-pro ^
  --set-default
reasonforge model list
reasonforge model discover --provider mimo
reasonforge model discover --provider mimo --write-capabilities
reasonforge model enrich --provider mimo
reasonforge model test
reasonforge model test --prompt "只回复 OK"
reasonforge model use mimo-v2.5-pro
```

For Mimo on Windows, set the key outside ReasonForge:

```powershell
setx MIMO_API_KEY "your-key"
```

`models.yaml` stores `api_key_env: MIMO_API_KEY`, not the key value. ReasonForge does not modify shell profiles or write secrets to EventStore, checkpoints, or logs.

Model profiles can also store optional capability metadata such as `max_context_tokens`, `reasoning_level`, `capability_source`, and `pricing`. ReasonForge only writes known capability presets or user-provided values; it does not guess unknown model limits and does not hardcode pricing. Pricing is used only for local estimated display.

## NekoForge Terminal Console

NekoForge is ReasonForge's local terminal AI coding workbench. It keeps ReasonForge as the underlying engine and adds a cat-themed console entry point:

```sh
neko
neko --mode single --dry-run
neko --mode multi --model mimo-v2.5-pro --reasoning high
neko --no-color
reasonforge neko
```

Inside the console:

```text
/model
/model test
/model enrich
/mode multi
/run fix a README typo
/preview wt_xxx
/review wt_xxx
/discard wt_xxx
/exit
```

Defaults are safe: `dry-run=true`, multi-agent mode uses worktree isolation, and NekoForge does not auto-apply, auto-commit, or auto-push. Patch application remains an explicit CLI action outside the console. Token usage and CNY cost are estimates from local usage and configured model pricing; if pricing is missing, cost is shown as unavailable.

## First Run

`reasonforge init` creates both `.reasonforge/*.yaml` config files and the default prefix source scaffolding:

- `prompts/system.md`
- `prompts/coding_rules.md`
- `schemas/tools.json`

If an older checkout is missing these files, run:

```sh
reasonforge init --repair
```

Windows first-run example:

```bat
cd /d D:\Desktop\ReasonForge
reasonforge init
reasonforge model setup --preset mimo --provider mimo --model mimo-v2.5-pro --set-default
setx MIMO_API_KEY "your-key"
reasonforge model test --provider mimo --model mimo-v2.5-pro --prompt "只回复 OK"
reasonforge run --goal "只回复 OK，用来测试模型连接。" --dry-run
reasonforge serve
```

`init` and `init --repair` never write API key values and never overwrite existing user prompts, schemas, or model provider configuration.

## Local Dashboards

ReasonForge records sanitized run events in the local EventStore. You can inspect progress in the terminal or browser:

```sh
reasonforge dashboard
reasonforge serve
reasonforge serve --port 9000
reasonforge serve --open
```

`reasonforge serve` starts a local read-only Web Dashboard. By default it listens on:

```text
http://127.0.0.1:8765
```

The dashboard data comes from EventStore and shows recent runs, run detail, timelines, progress state, and event lists. Terminal runs display explicit states and phases such as `succeeded` with `completed`, `failed` with `failed`, and `cancelled` with `cancelled`. It does not execute tools, call models, read source file contents, auto-apply patches, commit, or push.

Empty patch validation is quiet by default. When `patch preview` reports `files_changed=0`, `patch validate` and `patch review` skip default validation commands and print:

```text
validation_skipped=true
reason=no changes
```

Passing `--test-command` explicitly still runs validation.

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
- Multi-agent iteration loop has a hard cap of 5 iterations.
- Coder agent must use worktree isolation; never writes to main workspace.
- Reviewer agent cannot override deterministic reject rules.
- No auto-apply, auto-commit, or auto-push in multi-agent runs.
- Local dashboards are read-only and default to `127.0.0.1`.

## Status

Phase 1 (Context Engine + Cache Engine), Phase 2 (Model Router + Usage Accounting), Phase 3 (Tool Runtime + Security Hardening), Phase 4 (Agent Runtime), Phase 5 (Worktree Isolation + Patch Manager), and Phase 6 (Patch Review + Validation Pipeline) are implemented.

Phase 7 adds Multi-Agent Runtime:
- MultiAgentRuntime: orchestrates Planner -> Coder -> Reviewer agents in an iteration loop
- PlannerAgent: generates TaskPlan via ModelRouter (strict JSON output, no tool calls, no file modifications)
- CoderAgent: delegates to SingleAgentRuntime with UseWorktree=true (never writes main workspace)
- ReviewerAgent: delegates to PatchReviewManager (deterministic reject cannot be overridden)
- IterationLoop: approve ends loop, reject stops, request_changes continues (max 5 iterations)
- SharedTaskContext: structured inter-agent communication with sensitive data redaction
- MultiAgentCheckpointStore: JSONL append-only with API key sanitization
- CLI: `reasonforge multi-run` (default worktree=true, dry-run=true, no auto-apply)
- Configuration: multiagent.yaml with safe defaults
