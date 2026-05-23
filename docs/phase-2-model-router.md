# Phase 2: Model Router + Usage Accounting

## Overview

Phase 2 implements the Model Router layer, enabling ReasonForge to convert a `ContextEngine.Bundle` into an OpenAI-compatible completion request, route it through a provider, and record usage into the CacheRegistry.

This phase only builds the model routing foundation. It does not implement multi-agent, tool runtime, repo indexer, memory RAG, or UI.

## Architecture

```
ContextEngine.Build() → Bundle → BundleToMessages() → OpenAI Messages
                                                       ↓
                                              ModelRouter.Complete()
                                              ├── Resolve provider/model
                                              ├── Fallback chain iteration
                                              ├── Provider.Complete()
                                              └── UsageToObservation() → CacheRegistry.Record()
```

### Core Types

| Type | Package | Purpose |
|------|---------|---------|
| `ModelRouter` | `modelrouter` | Interface: route + call + record |
| `Provider` | `modelrouter` | Interface: call a model provider |
| `CompletionRequest` | `modelrouter` | Input: Bundle + model + params |
| `CompletionResponse` | `modelrouter` | Output: text + usage + metadata |
| `Usage` | `modelrouter` | Token consumption tracking |
| `MockProvider` | `modelrouter` | Test-only provider implementation |
| `OpenAICompatibleProvider` | `modelrouter` | OpenAI API provider skeleton |
| `DefaultModelRouter` | `modelrouter` | Fallback chain implementation |
| `FallbackEntry` | `modelrouter` | Single fallback chain entry |

## Provider Interface

```go
type Provider interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    Name() string
    Supports(model string) bool
}
```

Every provider must:
- Accept a `CompletionRequest` with a `Bundle` as input core.
- Return a `CompletionResponse` with `Usage`.
- Report its name and model support via `Name()` and `Supports()`.

## Bundle → Messages Conversion

`BundleToMessages()` converts a `contextengine.Bundle` into OpenAI-compatible chat messages with a stable ordering:

| Bundle Layer | Message Role | Notes |
|-------------|-------------|-------|
| `immutable_prefix` | `system` | System instructions from prefix builder |
| `conversation_log` | `assistant` | Condensed conversation history |
| `scratchpad` | `system` | Prefixed with `[volatile context]` |
| `current_input` | `user` | Always the last message |

Rules:
- `CurrentInput` must always be the last user message.
- `ImmutablePrefix` maps to a `system` role message.
- Scratchpad is marked as volatile context and must NOT enter the immutable prefix.
- Empty layers are skipped.
- The original `Bundle.Layers` order is preserved.

## Usage Accounting Rules

After a successful model call, `UsageToObservation()` creates a `cache.Observation` for the CacheRegistry.

### Field Mapping

| Observation Field | Source |
|------------------|--------|
| `Fingerprint` | `Bundle.CacheFingerprint` |
| `Provider` | Response provider name |
| `Model` | Response model name |
| `RequestID` | Provider's request ID |
| `InputTokens` | `Usage.InputTokens` (fallback: `Bundle.Report.TotalTokens`) |
| `CachedTokens` | `Usage.CachedTokens` (0 if missing) |
| `ObservedAt` | `time.Now()` |
| `Estimated` | Determined by provider response |
| `PrefixTokens` | `Bundle.Report.PrefixTokens` |
| `ConversationTokens` | `Bundle.Report.ConversationTokens` |
| `ScratchpadTokens` | `Bundle.Report.ScratchpadTokens` |
| `CurrentInputTokens` | `Bundle.Report.CurrentInputTokens` |

### Estimated Flag Rules

| Condition | `Estimated` |
|-----------|------------|
| Provider returns `cached_tokens` | `false` |
| Provider returns usage without `cached_tokens` | `true` |
| Provider returns no usage at all | `true` (estimated from `Bundle.Report.TotalTokens`) |

## CacheRegistry Writeback

The `DefaultModelRouter` records usage to the CacheRegistry after each successful completion. This is best-effort — a writeback failure does not fail the request.

The recorded observation enables:
- Prefix cache hit rate tracking per fingerprint.
- Provider-specific cache reuse analysis.
- Token budget estimation improvements.

## Fallback Chain Strategy

The fallback chain is configured in `models.yaml` under `routing.fallback_chain`.

### Behavior

1. Iterate through the `fallback_chain` in configured order.
2. If the current provider returns an error, try the next entry.
3. If all providers fail, return a `FallbackError` with aggregated attempt details.
4. No infinite retries — each entry is tried at most once.
5. Provider/model names are logged in errors, but API keys are never exposed.

### FallbackError

```go
type FallbackError struct {
    Attempts []FallbackAttempt
}

type FallbackAttempt struct {
    Provider string
    Model    string
    Err      error
}
```

The error message includes which provider/model pairs were tried and why each failed, but never includes API keys or request body content.

### Configuration

When `fallback_chain` is configured:

```yaml
routing:
  default_model: deepseek-chat
  fallback_chain:
    - provider: deepseek
      model: deepseek-chat
    - provider: local-openai-compatible
      model: local-coder
```

When `fallback_chain` is missing, the router derives a single-entry chain from the `default_model`'s provider.

## API Key Security

### Principles

1. API keys are read from environment variables specified in `api_key_env`.
2. Keys are never logged, printed to CLI output, or included in error messages.
3. The `reasonforge models` command shows only `configured` or `missing` status.
4. HTTP request bodies in tests never contain actual keys.
5. The `Authorization` header is never included in error output.
6. Non-200 error responses from providers omit the response body to prevent key leakage.

### Error Handling

- Missing API key: `"provider X: API key not found in environment variable Y"`
- The error names the environment variable, not its value.

## OpenAI-Compatible Provider

The `OpenAICompatibleProvider` implements the `Provider` interface for any OpenAI-compatible API.

### Features

- Configurable `base_url` for any compatible endpoint.
- API key read from environment variable via `api_key_env`.
- Request building from `Bundle` via `BundleToMessages`.
- Response parsing with usage extraction.
- `prompt_tokens_details.cached_tokens` support.
- Context cancellation and timeout support via `net/http`.
- Non-200 status error handling (status code only, no body leakage).

### Configuration

```yaml
providers:
  - name: deepseek
    type: openai-compatible
    base_url: https://api.deepseek.com/v1
    api_key_env: DEEPSEEK_API_KEY
    models:
      - name: deepseek-chat
        purpose: coding
        max_output_tokens: 4096
        supports_prefix_cache: true
```

## CLI: reasonforge models

The `reasonforge models` command displays provider configuration:

```
ReasonForge Models
default_model=deepseek-chat

provider=deepseek
type=openai-compatible
base_url=https://api.deepseek.com/v1
api_key_env=DEEPSEEK_API_KEY
api_key_status=missing
models=deepseek-chat

fallback_chain:
1. deepseek/deepseek-chat
2. local-openai-compatible/local-coder
```

API key status shows `configured` or `missing` — never the actual key value.

## Phase 3 Integration: Tool Runtime

The Model Router is designed to be extended by Phase 3 (Tool Runtime). The integration points are:

1. **CompletionRequest.Metadata**: Can carry tool execution context.
2. **CompletionResponse**: Can be extended with tool call information.
3. **Bundle.Layers**: Tool schemas are already part of the immutable prefix.
4. **Usage Accounting**: Tool call token costs can be recorded as additional observations.
5. **Provider Interface**: Can be extended to support tool-use response formats.

The current implementation does NOT include tool runtime, but the architecture is ready for it.

## Test Coverage

| Test Area | Count | Key Tests |
|-----------|-------|-----------|
| BundleToMessages | 6 | Order, CurrentInput last, ImmutablePrefix=system, Scratchpad isolation |
| MockProvider | 10 | Text, Usage, Error, Delay, Context cancellation, Supports |
| OpenAICompatibleProvider | 14 | API key missing, Request building, Usage parsing, CachedTokens, Estimated, Context cancellation |
| DefaultModelRouter | 6 | Single provider, Fallback, All-fail error, Default model, Usage recording, Provider skip |
| UsageToObservation | 5 | Field mapping, Estimated flags, Bundle fallback, CachedTokens |
| CLI models | 5 | Output format, API key safety, Extra args, Missing config |
| Config fallback_chain | 4 | Valid chain, Invalid provider, Invalid model, Default without chain |
