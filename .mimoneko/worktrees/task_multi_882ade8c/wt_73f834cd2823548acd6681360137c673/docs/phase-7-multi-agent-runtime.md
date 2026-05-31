# Phase 7: Multi-Agent Runtime

## Overview

Phase 7 implements NekoMIMO's minimal multi-agent collaboration runtime. It orchestrates three specialized agents in an iteration loop:

```
Planner Agent -> Coder Agent -> Reviewer Agent -> (iterate or stop)
```

The multi-agent runtime does **not** reimplement any Phase 4-6 components. It composes them:

- **PlannerAgent** uses ModelRouter to generate a TaskPlan
- **CoderAgent** delegates to SingleAgentRuntime for code modifications
- **ReviewerAgent** delegates to PatchReviewManager for patch review
- **IterationLoop** drives the cycle until approve/reject/max_iterations

## Core Types

### MultiAgentRuntime

```go
type MultiAgentRuntime interface {
    Run(ctx context.Context, req MultiAgentRunRequest) (MultiAgentRunResult, error)
}
```

### AgentRole

```go
const (
    AgentRolePlanner AgentRole = "planner"
    AgentRoleCoder   AgentRole = "coder"
    AgentRoleReviewer AgentRole = "reviewer"
)
```

### MultiAgentState

```go
const (
    MultiAgentStatePending         MultiAgentState = "pending"
    MultiAgentStateRunning         MultiAgentState = "running"
    MultiAgentStateSucceeded       MultiAgentState = "succeeded"
    MultiAgentStateRequestChanges  MultiAgentState = "request_changes"
    MultiAgentStateRejected        MultiAgentState = "rejected"
    MultiAgentStateFailed          MultiAgentState = "failed"
    MultiAgentStateCancelled       MultiAgentState = "cancelled"
)
```

### TaskPlan

```json
{
  "goal": "Fix typo in README",
  "steps": [
    {
      "index": 0,
      "title": "Fix typo",
      "description": "Change 'teh' to 'the' in README.md",
      "target_paths": ["README.md"],
      "expected_outcome": "README has correct spelling"
    }
  ],
  "risk_level": "low",
  "notes": "Simple text fix"
}
```

## Agent Details

### PlannerAgent

- Calls ModelRouter (not ToolRuntime)
- Cannot modify files
- Cannot override TaskContract (goal is always from the contract)
- Output must be strict JSON (with optional markdown fence stripping)
- Non-JSON output returns an error

### CoderAgent

- Delegates to SingleAgentRuntime.Run
- Must use UseWorktree=true
- Reuses WorktreeID across iterations
- Does not apply patch
- Does not commit/push
- Receives reviewer feedback from previous iterations

### ReviewerAgent

- Delegates to PatchReviewManager.Review
- Cannot override deterministic reject rules
- When validation fails, recommendation must be at least request_changes
- Does not apply patch
- Can optionally generate natural language summary via model (without overriding recommendation)

## Iteration Loop

```
1. Planner generates TaskPlan
2. Coder executes one coding pass
3. Reviewer reviews the patch
4. Based on recommendation:
   - approve: MultiAgentStateSucceeded, stop
   - reject: MultiAgentStateRejected, stop
   - request_changes: if iteration < MaxIterations, give feedback to Coder, iterate
   - MaxIterations reached: MultiAgentStateRequestChanges, stop
```

Defaults:
- MaxIterations = 2
- Maximum allowed = 5
- Default worktree = true
- Default dry-run = true

## SharedTaskContext

Carries structured information between agents:

```go
type SharedTaskContext struct {
    Goal          string
    Plan          TaskPlan
    WorktreeID    string
    ReviewHistory []review.PatchReviewReport
    Messages      []AgentMessage
    Metadata      map[string]string
}
```

Safety guarantees:
- Never stores API keys
- Never stores sensitive diffs (diffs from reports with violations are redacted)
- API key patterns are scrubbed from all text fields

## Checkpoint Store

Path: `.nekonomimo/checkpoints/multi_agent_runs.jsonl`

Properties:
- JSONL append-only
- Directory permissions: 0700
- File permissions: 0600
- Checkpoint write failure causes run failure
- No API keys, no sensitive diffs
- Separate schema from single-agent checkpoints

## CLI

```sh
NekoMIMO multi-run "fix typo in README" [flags]
```

Flags:
- `--dir` - project root
- `--model` - model name
- `--max-iterations` - max iterations (default: 2, max: 5)
- `--dry-run` - dry run mode (default: true)
- `--worktree` - use worktree isolation (default: true)
- `--approve-medium` - auto-approve medium-risk tools
- `--model-review` - use AI model review in reviewer

Behavior:
- Default worktree=true, dry-run=true
- Does NOT auto-apply
- Does NOT auto-commit
- Does NOT auto-push
- If approved, suggests: `NekoMIMO patch apply <worktree_id>`

## Configuration

`multiagent.yaml`:

```yaml
max_iterations: 2
max_allowed_iterations: 5
default_worktree: true
default_dry_run: true
planner_model: ""
coder_model: ""
reviewer_model: ""
reviewer_use_model_review: false
```

## Safety Rules

1. Coder must use worktree isolation; never writes to main workspace
2. Reviewer cannot override deterministic reject
3. Max iterations capped at 5
4. No auto-apply, auto-commit, auto-push
5. Sensitive data redacted from SharedTaskContext and checkpoints
6. Planner cannot override TaskContract
7. Only registered tools allowed (inherited from Phase 4)
8. No API keys in output or logs
9. Context cancellation returns cancelled state
10. Checkpoint failure causes run failure
