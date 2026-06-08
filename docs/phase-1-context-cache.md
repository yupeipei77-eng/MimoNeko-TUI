# Phase 1: Context Engine and Cache Engine

## Purpose

Phase 1 establishes MimoNeko's deterministic context pipeline and cache
observability foundation. The goal is to make repeated MIMO requests cache
friendly by keeping reusable prefix content byte-stable and by recording token
usage after model calls.

## Context Layers

`DefaultContextEngine` assembles context in this order:

1. Immutable prefix
2. Conversation log tail
3. Volatile scratchpad
4. Current user input

The current user input is always the final user-facing layer. Scratchpad content
is dynamic and must not be mixed into the immutable prefix.

## Immutable Prefix Rules

- Normalize line endings to LF.
- Remove trailing whitespace.
- Sort tool schemas by name.
- Use canonical JSON for structured sources.
- Exclude dynamic values such as time, session IDs, random IDs, and retrieval
  results.
- Hash the final prefix bytes with SHA-256.

These rules keep the prefix stable across runs and improve MIMO prefix-cache
reuse.

## Storage Rules

- Conversation logs are append-only JSONL files.
- Cache observations are append-only JSONL records.
- Archiving is represented by metadata, not by deleting historical records.
- Future compaction should add summary events instead of rewriting history.

## Scratchpad Rules

- Scratchpad items live in memory only.
- Items are returned by priority and constrained by token budget.
- Expired items are skipped.
- Scratchpad data can inform the current request but cannot become prefix data.

## Cache Report

Cache reports are grouped by prefix fingerprint and include:

- input tokens
- cached tokens
- uncached tokens
- hit rate
- reuse count
- possible miss reasons

Global summaries aggregate these values across all observations.

## Budget Guard

`prefix.yaml` can define token budget thresholds:

```yaml
budget:
  warn_ratio: 0.8
  block_ratio: 1.0
```

The guard reports `ok`, `warn`, or `block` based on utilization. A zero or
negative budget disables blocking.

## Known Limits

- Token counts use a heuristic rather than a provider-specific tokenizer.
- JSONL report generation scans files directly.
- Scratchpad state is not persisted.
- Conversation compaction is reserved for a later phase.
