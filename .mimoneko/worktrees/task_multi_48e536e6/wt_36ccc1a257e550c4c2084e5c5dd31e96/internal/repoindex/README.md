# internal/repoindex

## Responsibilities

- Define the `RepoIndexer` contract for local repository indexing and query.
- Return file matches and snippets for volatile context.

## Boundaries

- Indexes are local-first runtime assets.
- Query results should be provenance-rich and token-budget aware.

## Forbidden

- Do not call remote code search services in the default implementation.
- Do not place snippets directly into immutable prefix.
- Do not mutate repository files.
