# Phase 2: Model Router and Usage Accounting

## Purpose

Phase 2 connects `ContextEngine.Bundle` to OpenAI-compatible model providers and
records usage observations for cache reporting.

This phase does not implement agent workflows or tool execution.

## Flow

```text
ContextEngine.Build
  -> BundleToMessages
  -> ModelRouter.Complete
  -> Provider.Complete
  -> UsageToObservation
  -> CacheRegistry.Record
```

The router never bypasses `ContextEngine.Bundle`. Providers receive messages
derived from the bundle in stable layer order.

## Core Types

| Type | Package | Purpose |
| --- | --- | --- |
| `ModelRouter` | `modelrouter` | Routes a request to the configured provider/model |
| `Provider` | `modelrouter` | Calls a model provider |
| `CompletionRequest` | `modelrouter` | Bundle, model, options, and metadata |
| `CompletionResponse` | `modelrouter` | Text, usage, provider, model, request ID |
| `Usage` | `modelrouter` | Input, output, and cached token counters |
| `DefaultModelRouter` | `modelrouter` | Fallback chain and usage writeback |

## Bundle to Messages

`BundleToMessages` maps context layers to chat messages:

| Layer | Role |
| --- | --- |
| immutable prefix | system |
| conversation log | assistant |
| scratchpad | system |
| current input | user |

Empty layers are skipped. Current input remains the last user message.

## Usage Accounting

`UsageToObservation` records:

- provider and model
- request ID
- prefix fingerprint
- input tokens
- cached tokens when available
- native MIMO cache metrics when available
- estimated flag when provider usage is incomplete

If a provider does not return cached-token data, MimoNeko records the call as
unsupported/estimated rather than failing the request.

## Fallback Chain

The fallback chain comes from `models.yaml`. Each provider/model pair is tried
at most once. API keys and request bodies are never included in user-facing
errors.

## CLI Surface

Use lowercase CLI commands:

```sh
mimoneko models
mimoneko model setup
mimoneko model discover --provider mimo
mimoneko model use mimo-v2.5-pro
```

The models output reports API key status as `configured` or `missing`; it never
prints the key value.
