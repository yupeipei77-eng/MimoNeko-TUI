# Phase 9.5 Init Scaffolding And First-Run Experience

Phase 9.5 makes a fresh NekoMIMO checkout usable immediately after `NekoMIMO init`.

## What Init Creates

`NekoMIMO init` creates `.nekonomimo/*.yaml` configuration files and default prefix source scaffolding:

- `prompts/system.md`
- `prompts/coding_rules.md`
- `schemas/tools.json`

These files are referenced by `.nekonomimo/prefix.yaml` and are required by the Context Engine before a model call can be made.

Default prompt content is intentionally small and safety-focused. It reminds the agent to keep changes minimal, avoid secrets, and never auto-commit, auto-push, or apply patches.

## Repair

Older workspaces can be repaired with:

```sh
NekoMIMO init --repair
```

Repair behavior:

- Creates missing default scaffolding files.
- Creates missing `.nekonomimo/*.yaml` defaults.
- Does not overwrite existing config files.
- Does not overwrite existing prompts or schemas.
- Does not write API key values.
- Does not change an existing `models.yaml` provider or `default_model`.

`NekoMIMO doctor` detects missing required prefix sources and suggests:

```sh
NekoMIMO init --repair
```

## Windows First Run

```bat
cd /d D:\Desktop\NekoMIMO
NekoMIMO init
NekoMIMO model setup --preset mimo --provider mimo --model mimo-v2.5-pro --set-default
setx MIMO_API_KEY "your-key"
NekoMIMO model test --provider mimo --model mimo-v2.5-pro --prompt "只回�?OK"
NekoMIMO run --goal "只回�?OK，用来测试模型连接�? --dry-run
NekoMIMO serve
```

NekoMIMO reads API keys from environment variables only. It does not write secrets to YAML, EventStore, checkpoints, or logs.
