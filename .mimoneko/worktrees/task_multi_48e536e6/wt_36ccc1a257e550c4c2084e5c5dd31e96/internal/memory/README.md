# internal/memory

## Responsibilities

- Define the `MemoryStore` contract for local-first durable memory.
- Support scoped put, get, and search operations.

## Boundaries

- Memory search results are candidate context only.
- Context assembly must route memory through volatile context with clear provenance.

## Forbidden

- Do not inject memory directly into the main prompt.
- Do not include memory in immutable prefix bytes.
- Do not rewrite memory history without an explicit future retention policy.
