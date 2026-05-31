# Phase 6: Patch Review + Validation Pipeline

## Overview

Phase 6 implements the Patch Review and Validation Pipeline, enabling NekoMIMO to automatically perform structured review before applying a patch:

```
PatchPreview â†?RuleBasedReview â†?RiskScoring â†?TestValidation â†?Optional ModelReview â†?PatchReviewReport â†?Recommendation
```

The pipeline produces a deterministic `approve / request_changes / reject` recommendation. Safety rules always take priority over model suggestions.

## Architecture

### PatchReviewManager

The central orchestrator that runs the full review pipeline:

```go
type PatchReviewManager interface {
    Review(ctx context.Context, req PatchReviewRequest) (PatchReviewReport, error)
}
```

**Flow:**
1. Generate `PatchPreview` via `PatchManager.Preview`
2. Convert to `PreviewData` for rule-based review
3. Run `RuleBasedReviewer.Review` â†?findings
4. Run `RiskScorer.Score` â†?risk score
5. Optionally run `ValidationRunner.Validate` â†?validation result
6. Optionally run `ModelReviewer.Review` â†?model review result
7. Compute deterministic recommendation via `computeRecommendation`

### RuleBasedReviewer

Rule-based review without model calls. Checks:

| Rule | Severity | Condition |
|------|----------|-----------|
| Violations | critical | `PatchPreview.Violations` non-empty |
| Diff size | warning | Diff exceeds `max_diff_bytes` or is redacted |
| Files changed | warning/info | >20 files (high), >5 files (medium) |
| Lines changed | warning/info | >500 lines (high), >100 lines (medium) |
| Binary files | critical/warning | Not allowed â†?critical; allowed â†?warning |
| Sensitive paths | critical | `.env`, `*.pem`, `*.key`, `.git`, `.nekonomimo` |
| Test coverage | warning/info | Source changes without test changes |
| Generated files | info | `package-lock.json`, `go.sum`, `dist/**`, `build/**` |

### RiskScorer

Computes a numeric risk score (0-100) and level (`low/medium/high/critical`):

| Factor | Points |
|--------|--------|
| Violations present | 100 (critical) |
| Critical findings | +80 |
| Binary files | +35 |
| High file count (>20) | +60 |
| Medium file count (>5) | +30 |
| High line count (>500) | +60 |
| Medium line count (>100) | +30 |
| Diff redacted | +30 |

**Score thresholds:**
- `critical`: â‰?0
- `high`: â‰?0
- `medium`: â‰?0
- `low`: <30

### ValidationRunner

Executes test commands through `ToolRuntime` (test_run tool), never directly:

```go
type ValidationRunner interface {
    Validate(ctx context.Context, req ValidationRequest) (ValidationResult, error)
}
```

**Safety guarantees:**
- Must execute through `ToolRuntime`, never directly `exec`
- `TestCommands` must be `command_name` values from `tools.yaml`
- `RepoRoot` should be the worktree path, not the main workspace
- Output is capped by `MaxOutputBytes`
- Timeout is enforced
- `ValidationResult` is sanitized to not leak API keys

### ModelReview

Optional AI review via `ModelRouter`:

```go
type ModelReviewer interface {
    Review(ctx context.Context, req ModelReviewRequest) (ModelReviewResult, error)
}
```

**Safety boundaries:**
- `UseModelReview=false` â†?never calls `ModelRouter`
- If `PatchPreview.Violations` non-empty â†?raw diff NOT sent to model
- If diff is redacted marker â†?original diff NOT requested
- Safe diff is only included when no violations exist
- Model review failure produces a warning finding (unless `strict_model_review=true`)
- Tests use `MockProvider` / fake `ModelRouter`, never real API calls

### Recommendation Rules

Deterministic priority order (safety rules > model suggestions):

| Priority | Condition | Recommendation |
|----------|-----------|----------------|
| 1 | Critical finding exists | `reject` |
| 2 | Violations exist | `reject` |
| 3 | Validation failed | `request_changes` |
| 4 | Risk score = critical | `reject` |
| 5 | Risk score = high | `request_changes` |
| 6 | Model recommends reject | `reject` |
| 7 | Model recommends request_changes | `request_changes` |
| 8 | Otherwise | `approve` |

**Key invariant:** Model suggestions can never override deterministic safety rules.

## CLI Commands

### `NekoMIMO patch validate <worktree_id>`

Execute rule review + test validation (no model review):

```sh
NekoMIMO patch validate wt_xxx --test-command go-test
NekoMIMO patch validate wt_xxx --max-output-bytes 32768 --timeout-seconds 60
```

**Parameters:**
- `--dir` - Project root
- `--test-command` - Test command name (repeatable)
- `--max-output-bytes` - Max output per command
- `--timeout-seconds` - Validation timeout

**Output:** Validation summary, findings, and recommendation.

### `NekoMIMO patch review <worktree_id>`

Execute full review pipeline with optional model review and test validation:

```sh
NekoMIMO patch review wt_xxx --model-review
NekoMIMO patch review wt_xxx --test-command go-test --model-review
NekoMIMO patch review wt_xxx --no-tests
```

**Parameters:**
- `--dir` - Project root
- `--test-command` - Test command name (repeatable)
- `--model` - Model name for review
- `--model-review` - Enable AI model review
- `--no-tests` - Skip test validation
- `--max-output-bytes` - Max output per command
- `--timeout-seconds` - Validation timeout

**Output:** Full review report with recommendation.

**Safety:** Never auto-applies, never auto-commits, never auto-pushes.

## Configuration

### review.yaml

```yaml
max_diff_bytes: 131072
high_risk_file_count: 20
medium_risk_file_count: 5
high_risk_line_count: 500
medium_risk_line_count: 100
require_tests_for_code_changes: false
strict_model_review: false
```

### validation.yaml

```yaml
default_test_commands:
  - go-test
max_output_bytes: 65536
timeout_seconds: 120
```

Both files are optional. Missing values use safe defaults. `NekoMIMO init` generates them. `NekoMIMO doctor` checks them.

## How Phase 7 Extends This

Phase 7 (Multi-Agent) can build on the review pipeline by:

1. **Reviewer Agent**: A dedicated agent role that uses `PatchReviewManager` to review other agents' work
2. **Multi-Agent Workflow**: Planner â†?Coder â†?Reviewer loop, where Reviewer uses `PatchReviewReport` to decide whether changes need revision
3. **Review History**: Store `PatchReviewReport` results for tracking review decisions over time
4. **Custom Rules**: Allow agents to define project-specific review rules beyond the built-in rules
5. **Review Policy**: Configure which review stages are required for different risk levels

The `PatchReviewManager` interface is designed to be composable â€?Phase 7 agents can use it directly or wrap it with additional logic.
