# review

Patch review pipeline for NekoMIMO.

## Responsibilities

- PatchReviewManager: orchestrates the full review pipeline
- RuleBasedReviewer: rule-based review without model calls
- RiskScorer: deterministic risk scoring
- DefaultModelReviewer: optional AI review via ModelRouter

## Boundaries

- Review must be based on PatchManager.Preview; never reads sensitive files directly
- Never bypasses PatchManager violation rules
- Deterministic safety rules always override model suggestions
- Sensitive diff content is never sent to models when violations exist

## Forbidden

- Do not implement multi-agent review
- Do not auto-apply patches
- Do not call real external model APIs in tests
- Do not send raw diff content to models when violations exist
