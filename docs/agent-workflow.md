# Agent Workflow

MioNeko keeps the workflow safe by default: inspect first, plan second, then apply only after explicit review. This phase adds read-only CLI entry points that mirror the review loop used by modern coding agents.

## Current Commands

```bash
neko status
neko diff
neko diff --staged
neko plan --goal "..."
neko cache stats
neko tools
neko events tools
```

The same commands are available through the main binary:

```bash
mimoneko neko status
mimoneko neko diff
mimoneko neko plan --goal "Update docs"
mimoneko neko events tools
```

## `neko status`

`neko status` reports repository state without invoking the model:

- current git branch
- whether the working tree is clean
- staged, unstaged, and untracked file counts
- latest run status when the project already has an event store

If run events are not configured or no run event file exists, the latest run section reports `unavailable`.

## `neko diff`

`neko diff` prints the working tree diff. It is intended for user review and never applies patches, commits changes, or calls the model.

Use `--staged` to inspect the staged diff:

```bash
neko diff --staged
```

## `neko plan`

`neko plan --goal "..."` prints a structured plan skeleton. The current implementation is intentionally marked as:

```json
{
  "implementation_status": "stub",
  "writes_files": false,
  "calls_model": false
}
```

This lets users review intent without allowing file writes or tool execution.

## `neko events tools`

`neko events tools` shows recent local tool audit events from the event store. It is a read-only inspection command and does not call the model, run tools, apply patches, or write files.

The current audit event types are:

- `tool.called`
- `tool.completed`
- `tool.failed`

These events include the tool metadata introduced in Phase 4.1, including risk level, approval flag, and duration information. They are observational only; approval, rollback, sandbox enforcement, and stronger redaction are reserved for later phases.

## Multi-Agent Workflow Skeleton

Phase 6.1 introduces a skeleton layer for multi-agent workflows. This layer provides:

### Agent Roles

| Role | Description |
|------|-------------|
| `planner` | Decomposes user goal into actionable plan steps |
| `coder` | Generates patch intent based on plan (skeleton: no real patch) |
| `reviewer` | Reviews patch intent for quality and safety (skeleton: no real diff) |
| `validator` | Validates review output (skeleton: no real tests) |

### CLI Commands

```bash
# List available agent roles
mimoneko agents

# Create workflow skeleton
mimoneko agents plan --goal "修复 README 拼写错误"

# View agent workflow events
mimoneko neko events agents
```

### Workflow Output

```bash
$ mimoneko agents plan --goal "修复 README 拼写错误"

Workflow:
  ID: wf_xxx
  Goal: 修复 README 拼写错误
  Status: completed

Steps:
  1. planner      completed
  2. coder        completed_stub
  3. reviewer     completed_stub
  4. validator    completed_stub
```

### EventStore Integration (Phase 6.2)

The workflow now emits events to the EventStore:

- `agent.workflow_started`
- `agent.step_started`
- `agent.step_completed`
- `agent.step_failed`
- `agent.workflow_completed`
- `agent.workflow_failed`

View events with:
```bash
mimoneko neko events agents
```

Example output:
```
MimoNeko Agent Workflow Events
TIME                 TYPE                     ROLE         STATUS       MESSAGE
2024-01-01 12:00:00  agent.workflow_started                started      Workflow started: 修复 README
2024-01-01 12:00:00  agent.step_started       planner      started      Step started: planner
2024-01-01 12:00:00  agent.step_completed     planner      completed    Step completed: planner
...
```

All event fields (goal, input_summary, output_summary, error_message) are sanitized to prevent secret leakage.

EventStore fallback: If EventStore is unavailable, events are silently discarded without affecting workflow execution.

### Important Notes

- **Skeleton only**: The current implementation does NOT call any LLM
- **No file modification**: No business files are modified
- **No patch application**: No patches are applied or committed
- **Stub outputs**: Coder, Reviewer, and Validator steps produce stub outputs

## Planner LLM Integration (Phase 6.3)

Phase 6.3 adds LLM integration for the Planner step only.

### CLI Commands

```bash
# Skeleton mode (default, no LLM call)
mimoneko agents plan --goal "优化 README"

# LLM mode (calls Planner LLM, plan only)
mimoneko agents plan --goal "优化 README" --llm

# LLM mode with JSON output
mimoneko agents plan --goal "优化 README" --llm --json
```

### Important Constraints

- `--llm` must be explicitly enabled
- Without `--llm`, skeleton behavior is preserved
- Planner LLM ONLY generates plans
- ImplementationStatus is ALWAYS `plan_only`
- No files are written
- No patches are generated
- No tools are executed
- Coder/Reviewer/Validator do NOT call LLM

### AgentPlan Output

```json
{
  "goal": "优化 README",
  "summary": "Add project description and usage examples",
  "steps": [
    {
      "id": "step_1",
      "title": "Analyze current README",
      "description": "Review existing content and identify gaps",
      "risk_level": "low",
      "expected_files": ["README.md"],
      "validation_hint": "Check for completeness"
    }
  ],
  "risks": ["May affect existing links"],
  "files_maybe_affected": ["README.md"],
  "validation_suggestions": ["Run tests"],
  "implementation_status": "plan_only"
}
```

### Future Phases

The following capabilities will be added in later phases:

- Real LLM integration for Coder
- Real patch generation
- Real diff review for Reviewer
- Real test execution for Validator

## Planned Commands

The following write-capable commands are intentionally left for a later phase:

- `neko approve <patch_id>`
- `neko rollback <run_id>`

They require stronger patch identity, event linkage, and rollback safety checks before being enabled.
