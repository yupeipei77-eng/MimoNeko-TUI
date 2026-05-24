# internal/modelprofile

`internal/modelprofile` owns local model provider profile management for Phase 9.3.

Responsibilities:

- Read and write only `.reasonforge/models.yaml`.
- Provide safe presets for OpenAI-compatible providers.
- Add, update, use, and remove provider/model entries.
- Query `/models` and smoke-test `/chat/completions`.
- Redact API keys, bearer tokens, and common secret patterns from CLI-facing text.

Security boundaries:

- API key values are never written to YAML.
- The package stores only `api_key_env`.
- It does not write EventStore records, checkpoints, logs, shell profiles, or secret files.
- HTTP failures report safe status summaries rather than raw response bodies.
