# internal/conversation

## Responsibilities

- Define the `ConversationLog` append-only event contract.
- Represent model calls, tool calls, patches, tests, rollbacks, messages, and task state as events.

## Boundaries

- Readers may query and tail events.
- Writers may only append new events.
- Storage implementation is intentionally deferred.

## Forbidden

- Do not expose update or delete operations.
- Do not compact history in place.
- Do not store volatile scratchpad as canonical history unless it is explicitly logged as an event.
