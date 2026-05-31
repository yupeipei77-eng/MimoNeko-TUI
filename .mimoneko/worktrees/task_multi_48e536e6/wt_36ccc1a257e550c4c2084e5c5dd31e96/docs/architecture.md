# NekoMIMO Architecture

## Mission

NekoMIMO is a local-first, cache-native AI coding agent runtime. The MVP does not try to be a complete agent product. It establishes stable boundaries for later implementations.

## Principles

1. Local First: config, indexes, memory, logs, and task state default to local storage.
2. OpenAI Compatible First: model calls go through one OpenAI-compatible provider abstraction.
3. Cache Native: the context engine is designed around prefix cache fingerprints.
4. Byte-Stable Prefix: system prompt, tool schema, and coding rules must be byte-stable.
5. Append-only Log: conversation and task events are appended, not rewritten.
6. Volatile Scratchpad: dynamic RAG, tool output, temporary reasoning, and repo snippets are volatile.
7. Worktree Safe: task contracts carry worktree isolation policy.
8. Observable Agent: model calls, tool calls, patches, tests, and rollbacks are events.
9. Minimal MVP: interfaces and boundaries first, complex behavior later.

## Directory Structure

```text
cmd/NekoMIMO/              CLI entry point
internal/cli/                 Minimal command handling
internal/config/              Local YAML config loading and validation
internal/prefix/              Byte-stable immutable prefix contract
internal/cache/               Prefix cache registry contract
internal/contextengine/       Context assembly contract
internal/conversation/        Append-only event log contract
internal/scratchpad/          Volatile context contract
internal/memory/              Local durable memory contract
internal/repoindex/           Local repository index contract
internal/model/               OpenAI-compatible model router contract
internal/modelrouter/         Model router implementation with fallback chain
internal/toolruntime/         Tool runtime compatibility aliases
internal/tools/               Tool runtime, registry, safety guard, audit log, built-in tools
internal/task/                Task contract and worktree policy
internal/worktree/            Git worktree isolation manager
internal/patch/               Patch preview, apply, and discard manager
internal/review/              Patch review pipeline (rule review, risk scoring, model review)
internal/validation/          Test validation runner
internal/agent/               Agent runtime orchestration contract
internal/multiagent/          Multi-agent runtime (Planner â†?Coder â†?Reviewer loop)
internal/version/             Version metadata
docs/adr/                     Architecture decision records
```

Every module has a local README describing responsibilities, boundaries, and forbidden behavior.

## Runtime Layers

```mermaid
flowchart TD
  CLI["CLI"] --> Agent["AgentRuntime"]
  Agent --> Task["TaskContract"]
  Agent --> Ctx["ContextEngine"]
  Agent --> Model["ModelRouter"]
  Agent --> Tools["ToolRuntime"]
  Agent --> Log["ConversationLog"]
  Ctx --> Prefix["PrefixBuilder"]
  Ctx --> Cache["CacheRegistry"]
  Ctx --> Scratch["Scratchpad"]
  Ctx --> ConvTail["Conversation tail"]
  Scratch --> Memory["MemoryStore search results"]
  Scratch --> Repo["RepoIndexer results"]
  Model --> |"BundleToMessages"| Provider["Provider (OpenAI-compatible)"]
  Model --> |"Fallback chain"| Provider
  Provider --> |"UsageToObservation"| Cache
```

## Context Model

NekoMIMO context is split into two layers.

Immutable prefix:

- System prompt.
- Sorted tool schemas.
- Coding rules.
- Byte-stable generated metadata.
- Prefix fingerprint and cache lookup key.

Volatile context:

- Conversation tail.
- Memory search results.
- RAG results.
- Tool outputs.
- Temporary reasoning.
- Repository index snippets.
- Task state summaries.

Dynamic material must never be merged into the immutable prefix. This is enforced at the configuration layer for `prefix.yaml` and should also be enforced by future prefix builder implementations.

`prefix.yaml` uses a strict immutable-source kind allowlist. MVP immutable sources may only be `static_file` or `generated_schema`. Filenames are not inspected for dynamic-content keywords because legitimate static policy files may discuss memory, retrieval, or tool behavior.

## Prefix Cache Design

`PrefixBuilder` produces a byte sequence and SHA-256 fingerprint. `CacheRegistry` stores observed provider cache references and cache usage metadata by that fingerprint.

The registry does not own prompt content. It stores observations such as provider, model, request ID, cached tokens, and provider cache references. This keeps prefix cache behavior observable without making provider-specific assumptions.

TODO:

- ~~Implement a deterministic prefix builder with LF normalization.~~ (Done in Phase 1)
- ~~Add schema sorting and canonical JSON generation for tool schemas.~~ (Done in Phase 1)
- ~~Add provider cache hit/miss metrics.~~ (Done in Phase 1)
- ~~Add local JSONL cache registry storage.~~ (Done in Phase 1)

## Model Router

The Model Router (`internal/modelrouter/`) is responsible for converting a `ContextEngine.Bundle` into an OpenAI-compatible completion request, selecting a provider via fallback chain, calling the provider, and recording usage into the CacheRegistry.

Key components:

- **ModelRouter interface**: `Complete(ctx, CompletionRequest) (CompletionResponse, error)` â€?the main entry point.
- **Provider interface**: `Complete`, `Name`, `Supports` â€?implemented by `OpenAICompatibleProvider` and `MockProvider`.
- **BundleToMessages**: Converts `Bundle.Layers` into OpenAI-compatible messages with stable ordering (immutable_prefix â†?conversation_log â†?scratchpad â†?current_input).
- **DefaultModelRouter**: Implements fallback chain logic. Tries providers in configured order; returns `FallbackError` on all-fail.
- **UsageToObservation**: Converts `Usage` + `Bundle` into `cache.Observation` for CacheRegistry writeback.
- **OpenAICompatibleProvider**: HTTP-based provider using `net/http`. Reads API keys from environment variables. Never logs or exposes keys.

The router does NOT read project files directly or bypass `ContextEngine.Bundle`.

Fallback chain configuration lives in `models.yaml` under `routing.fallback_chain`. If missing, a single-entry chain is derived from `default_model`.

API key security: keys are read from environment variables, never logged, never included in error messages, and the `NekoMIMO models` command only shows `configured`/`missing` status.

See `docs/phase-2-model-router.md` for full documentation.

## Tool Runtime

The Tool Runtime (`internal/tools/`) provides a secure local tool execution layer. All tool invocations must go through `ToolRuntime.Run()` â€?no business code calls a Tool implementation directly.

Key components:

- **ToolRuntime interface**: `Run(ctx, ToolRequest) (ToolResponse, error)` â€?the central orchestrator.
- **Tool interface**: `Name`, `Description`, `RiskLevel`, `Run` â€?implemented by each built-in tool.
- **ToolRegistry**: In-memory registry with duplicate rejection. `Register`, `Get`, `List`.
- **SafetyGuard**: Enforces workspace root confinement, sensitive path protection, output limits, timeouts.
- **AuditLog**: JSONL audit trail with crypto/rand IDs and content redaction.
- **Built-in tools**: `file_read`, `file_write`, `file_patch`, `git_diff`, `test_run`.

Safety boundaries:
- All file paths must stay within RepoRoot (no `..`, no absolute paths, no Windows drive letters).
- `.git/`, `.nekonomimo/`, `.env`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519` are write-denied.
- `.env`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519` are read-denied.
- `test_run` only executes predefined commands from `tools.yaml`; no arbitrary shell commands.
- Minimal environment inheritance (no API keys passed to subprocesses).
- Audit log directory: 0700, file: 0600.

See `docs/phase-3-tool-runtime.md` for full documentation.

## Append-only Log

`ConversationLog` exposes append and read operations. There is no update or delete method. Events include:

- User and assistant messages.
- Model calls.
- Tool calls.
- Patches.
- Test runs.
- Rollbacks.
- Task state transitions.

TODO:

- ~~Implement a local JSONL conversation log.~~ (Done in Phase 1)
- Add event IDs and integrity checks.
- Add log compaction as a separate derived artifact, never as in-place history rewrite.

## Config

`NekoMIMO init` creates `.nekonomimo/` with:

- `models.yaml`
- `tools.yaml`
- `security.yaml`
- `prefix.yaml`
- `worktree.yaml` (Phase 5)
- `patch.yaml` (Phase 5)
- `review.yaml` (Phase 6)
- `validation.yaml` (Phase 6)
- `multiagent.yaml` (Phase 7)

`NekoMIMO doctor` loads all files with strict YAML parsing and validates key safety constraints. The default model provider is OpenAI-compatible and local by default.

`NekoMIMO init` writes default config files with owner-only permissions where the platform supports them. The files should contain provider names, environment variable names, and policies, not secret values.

TODO:

- Add config version migration.
- Add config provenance and loaded file digests.
- Add security profile validation for tool permissions.

## Worktree Isolation

`WorktreeManager` (`internal/worktree/`) creates and manages isolated git worktrees for agent task execution. All worktrees live under `.nekonomimo/worktrees/` with a JSONL registry for tracking.

Key components:
- **WorktreeManager interface**: `Create`, `Remove`, `Get`, `List`, `UpdateState`
- **GitWorktreeManager**: Implements worktree operations using git commands
- **Registry**: Append-only JSONL with 0700/0600 permissions, no API keys

Branch naming follows the pattern: `NekoMIMO/<sanitized_task_id>/<short_id>`

## Patch Manager

`PatchManager` (`internal/patch/`) handles diff preview, violation checking, and patch application from worktrees to the main workspace.

Key components:
- **PatchManager interface**: `Preview`, `Apply`, `Discard`
- **GitPatchManager**: Uses git diff/apply for patch operations
- **Violation checking**: TaskContract AllowedPaths/DeniedPaths + hard-coded deny list
- **Safety**: Apply refuses if violations exist or main workspace is dirty

CLI commands:
- `NekoMIMO run --worktree` - Run agent in isolated worktree
- `NekoMIMO patch list` - List managed worktrees
- `NekoMIMO patch preview <id>` - Show diff and violations
- `NekoMIMO patch apply <id>` - Apply changes to main workspace
- `NekoMIMO patch discard <id>` - Remove worktree

See `docs/phase-5-worktree-patch-manager.md` for full documentation.

## Patch Review and Validation

`PatchReviewManager` (`internal/review/`) orchestrates the full patch review pipeline, producing a deterministic `approve / request_changes / reject` recommendation.

Pipeline: PatchPreview â†?RuleBasedReview â†?RiskScoring â†?Optional TestValidation â†?Optional ModelReview â†?Recommendation

Key components:
- **PatchReviewManager interface**: `Review(ctx, PatchReviewRequest) (PatchReviewReport, error)`
- **RuleBasedReviewer**: Rule-based checks (violations, diff size, file/line counts, binary files, sensitive paths, test coverage, generated files)
- **RiskScorer**: Numeric risk score (0-100) with deterministic thresholds
- **ValidationRunner** (`internal/validation/`): Test execution through ToolRuntime (test_run), output truncation, API key sanitization
- **DefaultModelReviewer**: Optional AI review via ModelRouter with sensitive diff guard

Safety rules (always override model suggestions):
1. Critical findings â†?reject
2. Violations â†?reject
3. Validation failure â†?request_changes
4. Critical risk â†?reject
5. High risk â†?request_changes
6. Model reject â†?reject
7. Model request_changes â†?request_changes
8. Otherwise â†?approve

CLI commands:
- `NekoMIMO patch validate <id>` - Rule review + test validation (no model)
- `NekoMIMO patch review <id>` - Full review with optional model review

See `docs/phase-6-patch-review-validation.md` for full documentation.

## Multi-Agent Runtime

The Multi-Agent Runtime (`internal/multiagent/`) orchestrates PlannerAgent â†?CoderAgent â†?ReviewerAgent in an iteration loop, producing a fully reviewed patch or a rejection with actionable feedback.

### Architecture

```mermaid
flowchart TD
  CLI["multi-run"] --> Runtime["DefaultMultiAgentRuntime"]
  Runtime --> Planner["PlannerAgent"]
  Runtime --> Coder["CoderAgent"]
  Runtime --> Reviewer["ReviewerAgent"]
  Planner -->|TaskPlan| Ctx["SharedTaskContext"]
  Coder -->|delegates| Single["SingleAgentRuntime"]
  Reviewer -->|delegates| ReviewMgr["PatchReviewManager"]
  Reviewer -->|optional| ModelReview["ModelReviewer"]
  Runtime -->|append| Checkpoint["MultiAgentCheckpointStore"]
  Ctx -->|redact| Sensitive["API keys, violation diffs"]
```

### Agent Roles

- **PlannerAgent**: Produces a `TaskPlan` (goal + steps + risk_level) via `ModelRouter.Complete`. Strict JSON output; no ToolRuntime access; no file modifications; cannot override the TaskContract goal.
- **CoderAgent**: Delegates to `SingleAgentRuntime.Run()` with `UseWorktree=true`. Builds a goal from the plan and reviewer feedback. Reuses WorktreeID across iterations. No auto-apply/commit/push.
- **ReviewerAgent**: Delegates to `PatchReviewManager.Review()`. Returns the deterministic recommendation. Optional model reviewer for natural language summaries, but cannot override deterministic reject.

### Iteration Loop

1. Validate request (max iterations hard cap = 5, default = 2).
2. PlannerAgent produces `TaskPlan`.
3. Loop:
   - CoderAgent generates code in worktree.
   - ReviewerAgent reviews the patch.
   - Decision: `approve` â†?succeeded, `reject` â†?rejected, `request_changes` â†?next iteration (or failed if max reached).
4. Context cancellation returns `cancelled`.
5. Checkpoint failure causes `failed`.

### SharedTaskContext

Carries the TaskPlan, agent messages, and review reports across iterations. Sensitive data is redacted:
- Diffs with violations are stripped from context.
- API key patterns are sanitized before passing to model.

### MultiAgentCheckpointStore

Append-only JSONL store at `.nekonomimo/checkpoints/multi_agent_runs.jsonl` with 0700/0600 permissions and API key scrubbing. Checkpoints are saved at key state transitions: init, planner_done, coder_done, reviewer_done, loop_end, cancelled, failed.

### CLI

```bash
NekoMIMO multi-run <goal> [--dir .] [--model gpt-4] [--max-iterations 2] [--dry-run] [--worktree] [--approve-medium] [--model-review]
```

Safety: `--dry-run` defaults to `true`; `--worktree` defaults to `true`; no auto-apply/commit/push.

### Configuration

`multiagent.yaml` controls defaults for max iterations, worktree policy, dry-run, per-agent model selection, and model review toggle. Safe defaults are applied when the file is missing or partial.

See `docs/phase-7-multi-agent-runtime.md` for full documentation.

## Observability

Every consequential action should become an append-only event:

- Model request and response metadata.
- Tool invocation and result metadata.
- Patch application.
- Test command and result.
- Rollback decision and result.
- Cache usage observation.

TODO:

- Add structured event payload schemas.
- Add trace IDs that connect context bundles, model calls, tool calls, and patches.
- Add local inspection commands.

## Non-goals

- No LangChain.
- No UI.
- No microservices.
- No Docker, K8s, SSH, or DB tooling in the MVP.
- No remote-first storage assumptions.
- No direct memory injection into the main prompt.
