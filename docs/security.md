# Security

MioNeko is a local AI coding agent. Treat model credentials, provider tokens, run logs, patches, and tool output as sensitive by default.

## Secret Handling

- Do not commit real API keys, tokens, cookies, private keys, or authorization headers.
- Keep local credentials in user-level config or environment variables.
- Keep `.env.example` as a template only. Do not commit `.env` or `.env.*` files with real values.
- Project config should reference secret environment variable names, not store secret values.
- Run output and event logs must redact API Key, Token, Cookie, and Authorization values.

## Secret Scanning

Before release or before accepting agent-generated changes, run a repository scan for common secret patterns:

```powershell
rg -n "tp-[A-Za-z0-9][A-Za-z0-9_-]{10,}|sk-(?:proj-)?[A-Za-z0-9][A-Za-z0-9_-]{10,}|(?i)(api[_-]?key|secret|password|token)\s*[:=]\s*[^\s\"']{8,}" .
```

Expected result:

- Real MiMo API keys: `0`
- Real OpenAI API keys: `0`
- Non-fixture secrets: `0`

Allowed matches should be limited to test fixtures, examples, documentation placeholders, or redaction tests.

## Repository Hygiene

The repository must not track:

- `.env` or `.env.*`
- Build outputs such as `dist/`, `build/`, `bin/`, `*.exe`, `*.dll`, `*.so`, or `*.dylib`
- Runtime state such as `.mimoneko/`, `.nekonomimo/`, `.neko/`, `logs/`, or `cache/`
- Temporary files such as `*.tmp`, `*.temp`, `*.bak`, `*.orig`, or editor swap files

Use `git rm --cached` when a local file should remain on disk but should no longer be tracked by Git.
