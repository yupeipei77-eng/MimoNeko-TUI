# internal/prefix

## Responsibilities

- Define the `PrefixBuilder` contract.
- Represent byte-stable immutable prefix documents and fingerprints.
- Keep system prompt, tool schema, and coding rules as stable inputs.

## Boundaries

- Prefix bytes are immutable inputs to model calls.
- Prefix fingerprints may be used by cache registries.
- Implementations should normalize bytes deterministically.

## Forbidden

- Do not include memory search results.
- Do not include scratchpad items.
- Do not include tool outputs.
- Do not include conversation tail or task state.
