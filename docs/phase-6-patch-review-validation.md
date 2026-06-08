# Phase 6: Patch Review and Validation

## Purpose

Phase 6 adds deterministic patch review before changes can be applied. The
pipeline favors local rules and validation results over model suggestions.

## Pipeline

```text
PatchPreview
  -> RuleBasedReviewer
  -> RiskScorer
  -> ValidationRunner
  -> optional ModelReviewer
  -> PatchReviewReport
  -> recommendation
```

Recommendations are `approve`, `request_changes`, or `reject`.

## Rule Review

The rule reviewer checks:

- explicit preview violations
- large diffs
- high file or line counts
- binary files
- sensitive paths such as `.env`, private keys, `.git`, and project secrets
- source changes without tests
- generated files

Critical findings reject the patch.

## Risk Scoring

The scorer assigns a 0-100 risk score and a level:

- `critical` for preview violations or critical findings
- `high` for large or risky changes
- `medium` for moderate file/line count risk
- `low` for small safe patches

The score is capped at 100.

## Validation

Validation runs test commands through `ToolRuntime` using registered
`test_run` command names from `tools.yaml`. It does not call `exec` directly.

Outputs are capped, timeouts are enforced, and API keys are sanitized.

## Model Review

Model review is optional. It never receives raw diffs when preview violations
exist or when the diff is redacted. A model recommendation cannot override
deterministic safety rules.

## CLI

```sh
mimoneko patch validate wt_xxx --test-command go-test
mimoneko patch review wt_xxx --model-review
mimoneko patch apply wt_xxx --dry-run
mimoneko patch apply wt_xxx --approve
```

Patch application requires `PermissionMode=apply-with-approval` and explicit
approval. MimoNeko never auto commits or auto pushes.
