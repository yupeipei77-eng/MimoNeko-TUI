# ADR 0002: Context Engine and Cache Engine Implementation

## Status

Accepted

## Context

MimoNeko needs deterministic context assembly and cache reporting for
MIMO-first usage. Prefix cache behavior is sensitive to byte-level changes, so
the reusable prefix must be stable and observable.

## Decision

1. Use canonical prefix helpers for line endings, JSON, tool ordering, hashing,
   and token estimation.
2. Store conversation logs and cache observations as append-only JSONL.
3. Use a simple token heuristic in this phase instead of adding a tokenizer
   dependency.
4. Keep scratchpad items in process memory only.
5. Generate cache reports by grouping observations by prefix fingerprint.
6. Configure budget thresholds in `prefix.yaml`.
7. Carry the current user input on `BuildRequest` so it remains separate from
   persisted conversation history until a model call succeeds.

## Consequences

Positive:

- Stable prefix bytes improve cache reuse.
- Append-only storage reduces accidental history corruption.
- Cache reports expose hit rate and possible miss reasons.
- The design stays local-first and easy to test.

Tradeoffs:

- JSONL scans are simple but not indexed.
- Scratchpad items are lost on restart.
- Token estimation is approximate.
- Conversation compaction is deferred.
