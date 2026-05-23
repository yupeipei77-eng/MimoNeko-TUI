# internal/config

## Responsibilities

- Load and validate `models.yaml`, `tools.yaml`, `security.yaml`, and `prefix.yaml`.
- Create safe default local configuration through `Init`.
- Enforce prefix safety checks that block dynamic sources from immutable prefix configuration.

## Boundaries

- Configuration parsing is local filesystem IO only.
- This package defines config shape, not runtime behavior.
- Provider entries describe OpenAI-compatible endpoints but do not call them.

## Forbidden

- Do not read secrets directly.
- Do not make network calls.
- Do not allow memory, RAG results, tool output, task state, or conversation history into immutable prefix sources.
