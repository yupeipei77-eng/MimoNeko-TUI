# internal/cache

## Responsibilities

- Define the `CacheRegistry` contract for prefix cache metadata.
- Track provider cache observations by byte-stable prefix fingerprint.
- Keep cache accounting observable across model calls.
- Generate cache reports with per-fingerprint and global statistics.

## Implementations

- `JSONLCacheRegistry` - JSONL-backed implementation. Observations appended to a single file. Lookup scans by fingerprint. `Report()` generates `CacheReport` with per-hash hit rates, reuse counts, and miss reasons.

## MIMO Cache Metrics

- MIMO is the only provider that uses native cache hit/miss counters by default.
- When MIMO responses include `prompt_cache_hit_tokens` and `prompt_cache_miss_tokens`, hit rate is `hit / (hit + miss)`.
- Other OpenAI-compatible providers keep the conservative fallback based on `prompt_tokens_details.cached_tokens`.
- Native cache observations still store metadata only; prompt content is never written to the registry.

## Report Types

- `PerFingerprintReport` - Per prefix-hash statistics: hit_rate, reuse_count, uncached_tokens, possible_miss_reasons.
- `CacheReport` - Collection of per-fingerprint reports plus `GlobalCacheSummary`.
- `GlobalCacheSummary` - Aggregate: total observations, total tokens, overall hit rate.
- `MissReason` - Enum: `prefix_changed`, `cache_expired`, `model_changed`, `no_prior_observation`.

## Boundaries

- The registry stores metadata and provider references, not prompt content.
- Provider cache behavior is observed, not assumed.
- Report is a method on the concrete type (`JSONLCacheRegistry`), not the interface.

## Forbidden

- Do not mutate prefix bytes.
- Do not store volatile scratchpad content as cache keys.
- Do not hide model call accounting from the conversation log.
