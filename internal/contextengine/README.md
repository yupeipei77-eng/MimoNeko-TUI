# internal/contextengine

## Responsibilities

- Define the `ContextEngine` contract.
- Assemble immutable prefix, conversation tail, and volatile scratchpad into a model-ready bundle.
- Record model-call cache observations.

## Boundaries

- Immutable prefix and volatile context must remain separate.
- Memory and repository search results must enter through scratchpad-like volatile context.
- Token budgets are planning inputs, not permission to rewrite history.

## Forbidden

- Do not put dynamic RAG, memory, tool output, or reasoning into immutable prefix bytes.
- Do not rewrite conversation events.
- Do not make provider-specific assumptions outside `internal/model` and `internal/cache`.
