# Phase 9.5 Init Scaffolding And First-Run Experience

Phase 9.5 makes a fresh MimoNeko checkout usable immediately after `MimoNeko init`.

## What Init Creates

`MimoNeko init` creates `.mimoneko/*.yaml` configuration files and default prefix source scaffolding:

- `prompts/system.md`
- `prompts/coding_rules.md`
- `schemas/tools.json`

These files are referenced by `.mimoneko/prefix.yaml` and are required by the Context Engine before a model call can be made.

Default prompt content is intentionally small and safety-focused. It reminds the agent to keep changes minimal, avoid secrets, and never auto-commit, auto-push, or apply patches.

## Repair

Older workspaces can be repaired with:

```sh
MimoNeko init --repair
```

Repair behavior:

- Creates missing default scaffolding files.
- Creates missing `.mimoneko/*.yaml` defaults.
- Does not overwrite existing config files.
- Does not overwrite existing prompts or schemas.
- Does not write API key values.
- Does not change an existing `models.yaml` provider or `default_model`.

`MimoNeko doctor` detects missing required prefix sources and suggests:

```sh
MimoNeko init --repair
```

## Windows First Run

```bat
cd /d D:\Desktop\MimoNeko
MimoNeko init
MimoNeko model setup --preset mimo --provider mimo --model mimo-v2.5-pro --set-default
setx MIMO_API_KEY "your-key"
mimoneko model test --provider mimo --model mimo-v2.5-pro --prompt "Reply OK only"
mimoneko run --goal "Reply OK only to test the model connection." --dry-run
MimoNeko serve
```

MimoNeko reads API keys from environment variables only. It does not write secrets to YAML, EventStore, checkpoints, or logs.
