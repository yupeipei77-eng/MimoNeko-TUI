# internal/cache

## Responsibilities

- Define the `CacheRegistry` contract for prefix cache metadata.
- Track provider cache observations by byte-stable prefix fingerprint.
- Keep cache accounting observable across model calls.

## Boundaries

- The registry stores metadata and provider references, not prompt content.
- Provider cache behavior is observed, not assumed.

## Forbidden

- Do not mutate prefix bytes.
- Do not store volatile scratchpad content as cache keys.
- Do not hide model call accounting from the conversation log.
