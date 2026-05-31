# modelrouter

Model Router implements the routing layer between ContextEngine Bundles and model providers.

## Responsibilities

- Convert `contextengine.Bundle` into OpenAI-compatible chat messages via `BundleToMessages`.
- Select provider/model via configurable fallback chain.
- Call providers through the `Provider` interface.
- Record token usage into `cache.CacheRegistry` via `UsageToObservation`.
- Expose `NekoMIMO models` CLI command output.

## Boundaries

- The router accepts `CompletionRequest` with a `Bundle` as input core.
- The router does NOT read project files directly.
- The router does NOT bypass `ContextEngine.Bundle`.
- The router does NOT log or expose API keys.

## Key Types

| Type | File | Purpose |
|------|------|---------|
| `ModelRouter` | types.go | Main interface |
| `Provider` | types.go | Provider interface |
| `CompletionRequest` | types.go | Input with Bundle |
| `CompletionResponse` | types.go | Output with Usage |
| `Usage` | types.go | Token consumption |
| `Message`, `Role` | bundle_to_messages.go | OpenAI message types |
| `MockProvider` | mock_provider.go | Test provider |
| `FallbackError` | mock_provider.go | Aggregated fallback error |
| `DefaultModelRouter` | router.go | Fallback chain router |
| `FallbackEntry` | router.go | Chain entry config |
| `OpenAICompatibleProvider` | openai_provider.go | OpenAI API provider |

## Forbidden Behavior

- Do not call real external APIs in unit tests.
- Do not print API keys in logs, errors, or CLI output.
- Do not add LangChain or heavy dependencies.
- Do not let the router reassemble context outside of Bundle.
