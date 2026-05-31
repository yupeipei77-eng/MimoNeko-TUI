# internal/scratchpad

## Responsibilities

- Define the volatile scratchpad contract.
- Hold temporary RAG results, tool output, transient reasoning, and repository context.
- Provide snapshots for context assembly.

## Boundaries

- Scratchpad data may expire or be cleared.
- Scratchpad data is not canonical history.
- Durable evidence belongs in append-only conversation events.

## Forbidden

- Do not persist scratchpad items as immutable prefix.
- Do not treat scratchpad as memory.
- Do not assume scratchpad survives process restarts unless a future implementation explicitly says so.
