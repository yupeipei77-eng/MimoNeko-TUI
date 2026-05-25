# Phase 9.3 Model Provider Setup

Phase 9.3 adds local CLI workflows for managing OpenAI-compatible model providers without storing API keys in `.reasonforge/models.yaml`.

## Commands

```sh
reasonforge model setup
reasonforge model list
reasonforge model discover
reasonforge model test
reasonforge model use
reasonforge model remove
```

The existing `reasonforge models` command remains available and unchanged.

## Setup

Interactive setup prompts for a provider preset, provider name, base URL, API key environment variable, model name, purpose, output limit, prefix-cache support, and whether to set the model as default.

For first-run use, run `reasonforge init` first. It creates the prompt and schema files required by the Context Engine before model calls:

- `prompts/system.md`
- `prompts/coding_rules.md`
- `schemas/tools.json`

Non-interactive example for Mimo:

```sh
reasonforge model setup ^
  --preset mimo ^
  --provider mimo ^
  --base-url https://token-plan-cn.xiaomimimo.com/v1 ^
  --api-key-env MIMO_API_KEY ^
  --model mimo-v2.5-pro ^
  --purpose coding ^
  --max-output-tokens 4096 ^
  --set-default
```

If `MIMO_API_KEY` is missing, ReasonForge prints a safe hint:

```powershell
setx MIMO_API_KEY "your-key"
```

It never asks the user to put the key in YAML.

## Discover And Test

`reasonforge model discover --provider mimo` calls the provider's OpenAI-compatible `/models` endpoint and prints returned model IDs. It does not save discovered models.

`reasonforge model test` sends a tiny `chat/completions` request:

```text
Reply with OK only.
```

Custom smoke-test prompts are supported:

```sh
reasonforge model test --prompt "只回复 OK"
```

The prompt is sent only for that request and is not written back to `models.yaml`.

Only status, latency, model/provider names, and a bounded sanitized model response are printed.

## Use And Remove

`reasonforge model use <model>` validates the model exists, updates `routing.default_model`, and updates the first fallback chain entry.

`reasonforge model remove --model <name>` or `reasonforge model remove --provider <name>` removes only config entries. It does not delete environment variables or secrets. Removing the current default model is rejected until the user switches defaults.

## Security

- API keys are read from environment variables only.
- `models.yaml` stores only `api_key_env`.
- CLI output shows key status as `configured` or `missing`.
- Authorization headers and bearer tokens are not printed.
- Error responses are limited to safe status summaries.
- No shell profile is modified automatically.
- No account system, cloud sync, Web setup page, Desktop, Electron, or Tauri is introduced.
