# Cache Design

MioNeko is optimized for MiMo prefix-cache behavior. The cache strategy starts with keeping the reusable prefix stable and observable before introducing any write-capable automation.

## Context Layers

MioNeko treats context as three layers:

- `immutable_prefix`: system prompt, tool schema, repo policy, and model profile.
- `semi_stable_context`: repo index, dependency summary, and current task plan.
- `volatile_context`: user input, recent tool outputs, current diff, and error logs.

Only `immutable_prefix` contributes to the prefix fingerprint used by current read-only observability.

## Prefix Fingerprint

The prefix fingerprint is a SHA-256 hash of stable bytes generated from the immutable prefix layer.

The fingerprint must not include:

- timestamps
- random IDs
- absolute paths
- map iteration order
- environment-specific values
- volatile user input or tool output

Map and slice entries are sorted before bytes are emitted. This keeps the fingerprint stable when the same logical prefix is assembled in a different order.

## Read-Only Stats

`neko cache stats` reports:

```json
{
  "prefix_fingerprint": "...",
  "immutable_bytes": 0,
  "semi_stable_bytes": 0,
  "volatile_bytes": 0,
  "estimated_cache_hit_ratio": 0,
  "implementation_status": "stub"
}
```

The current implementation is a read-only observability skeleton. It does not call an LLM, write files, mutate the cache registry, or apply patches.

## Why Byte Stability Matters

Provider-side prefix caches only help when repeated requests preserve the same byte prefix. Seemingly harmless changes such as reordered map keys, absolute workspace paths, generated timestamps, or volatile tool output can produce a different prefix and reduce cache reuse.

The stable fingerprint makes these changes visible before they become expensive runtime behavior.
