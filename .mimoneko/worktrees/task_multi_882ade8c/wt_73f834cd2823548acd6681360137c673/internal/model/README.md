# internal/model

## Responsibilities

- Define the `ModelRouter` contract.
- Route requests to OpenAI-compatible providers.
- Keep immutable prefix bytes separate from volatile messages in completion requests.

## Boundaries

- Provider configuration comes from `models.yaml`.
- HTTP transport and retry policy are future implementation details.

## Forbidden

- Do not introduce provider-specific call sites outside this abstraction.
- Do not merge volatile context into immutable prefix.
- Do not depend on LangChain.
