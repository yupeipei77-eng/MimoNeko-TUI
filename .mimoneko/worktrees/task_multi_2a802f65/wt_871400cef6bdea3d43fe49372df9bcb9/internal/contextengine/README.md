# internal/contextengine

## Responsibilities

- Define the `ContextEngine` contract.
- Assemble immutable prefix, conversation tail, and volatile scratchpad into a model-ready bundle.
- Record model-call cache observations.
- Enforce token budget limits with OK/WARN/BLOCK status.
- Report per-layer token usage and budget status in ContextReport.

## Implementations

- `DefaultContextEngine` — Assembles context in order: Immutable Prefix → Conversation Log → Scratchpad → Current User Input.
- `BudgetGuard` — Checks token usage against configurable thresholds from `prefix.yaml`.

## Boundaries

- Immutable prefix and volatile context must remain separate.
- Memory and repository search results must enter through scratchpad-like volatile context.
- Token budgets are planning inputs, not permission to rewrite history.
- `CurrentInput` on `BuildRequest` is the current turn input, separate from Conversation Log history.

## Forbidden

- Do not put dynamic RAG, memory, tool output, or reasoning into immutable prefix bytes.
- Do not rewrite conversation events.
- Do not make provider-specific assumptions outside `internal/model` and `internal/cache`.
