# ADR 0002: Phase 1 Context Engine + Cache Engine Implementation

## Status

Accepted

## Context

ReasonForge has an interface-only skeleton for context assembly and cache management. Phase 1 must fill in concrete implementations that establish byte-stable prefix construction, append-only logging, volatile scratchpad, cache statistics, and token budget enforcement.

## Decision

1. **Canonicalization as foundation**: Pure functions in `internal/prefix/canonical.go` provide NormalizeLineEndings, CanonicalText, CanonicalJSON, CanonicalTools, StableHash, and EstimateTokens. All other modules depend on these for determinism.

2. **JSONL storage format**: Both ConversationLog and CacheRegistry use JSONL files for local-first storage. One file per conversation for logs; one file for all cache observations. Append-only writes with fsync for durability.

3. **Token estimation heuristic**: `len(data) / 4` (~4 chars/token) as the project-wide heuristic. No external tokenizer dependency in Phase 1.

4. **In-memory scratchpad**: VolatileScratchpad stores items only in process memory. This matches the "volatile" semantic — items may be lost on restart. No disk persistence.

5. **Cache report by prefix_hash**: CacheReport groups observations by fingerprint SHA-256, computing per-hash hit rates, reuse counts, and miss reasons. GlobalSummary provides an aggregate view.

6. **Budget config in prefix.yaml**: Token budget thresholds (warn_ratio, block_ratio) are configured alongside other prefix settings, validated at config load time.

7. **CurrentInput on BuildRequest**: Added `CurrentInput []byte` to `BuildRequest` rather than requiring it to go through ConversationLog. CurrentInput is the current turn's input; it gets appended to ConversationLog only after a successful model call.

8. **Report method on concrete type**: CacheReport is generated via `JSONLCacheRegistry.Report()` (concrete type), not added to the `CacheRegistry` interface, to avoid changing interface signatures.

## Consequences

Positive:
- Byte-stable prefix guarantees enable reliable prefix cache behavior
- Append-only log prevents accidental history corruption
- Priority-based scratchpad eviction keeps high-value context
- Token budget guard provides configurable safety net
- All implementations satisfy existing interface contracts

Negative:
- JSONL storage has no indexing — queries scan entire files
- Scratchpad items are lost on process restart
- Token estimation is approximate, not model-specific
- No compaction mechanism for conversation logs yet
