# Phase 9.6 - NekoForge Terminal Console

NekoForge is a local terminal AI coding workbench powered by NekoMIMO. It is a terminal console brand only; NekoMIMO remains the underlying engine and project name.

## Start

```sh
neko
neko --dir .
neko --mode single
neko --mode multi
neko --model mimo-v2.5-pro
neko --reasoning high
neko --dry-run
neko --no-color
neko --new-window
NekoMIMO neko
```

The startup view uses a compact `NekoForge` header with a small cold-cyan mascot mark, an OpenCode-style runtime composer, and a dim status bar. Interactive startup clears the old shell scrollback view and sets the terminal title so the console feels like a standalone workbench. Submitted prompts render into the terminal event stream, while the composer stays ready for the next command. `--no-color` keeps the same layout without ANSI escape codes.

Runtime feedback is terminal-native rather than web-chat-like:

- `thinking...`
- `requesting model...`
- `planning...`
- `executing agent runtime...`
- `generating response...`
- `done · <latency>`
- `+ Thought: <latency>`

The folded thought line intentionally exposes only execution-event summaries and never hidden reasoning.

The status bar includes context usage, tool count, memory message count, model, provider, latency/session, optional cost, and optional reasoning. `ctrl+p` cycles reasoning when the current model supports it, while `/` opens the command palette.

Defaults are intentionally safe:

- `mode=multi`
- `dry-run=true`
- multi-agent mode uses worktree isolation
- no auto-apply
- no auto-commit
- no auto-push

## Console Commands

```text
/help
/
/agents
/model
/models
/model test
/model enrich
/mode single
/mode multi
/new
/reasoning
/reasoning low
/reasoning medium
/reasoning high
/runs
/run fix a README typo
/preview wt_xxx
/review wt_xxx
/discard wt_xxx
/exit
```

Plain text input is chat with the configured model. Use `/run <goal>` when you want to execute an agent task. Empty input does not run anything.

If a chat message explicitly says to save, write, or generate a file, NekoForge writes the generated code block or response text directly to the project directory or the specified directory/file path. This is intentionally scoped to explicit file-writing language and does not perform patch apply, commit, or push operations.

## Model Display

The console shows:

- provider
- model
- base URL host
- `api_key_status=configured|missing`
- context length when known
- reasoning level
- token usage
- estimated CNY cost when pricing is configured

API keys are never printed. The console displays only whether the configured environment variable is present.

## Capabilities And Cost

Model profiles can include optional metadata:

```yaml
models:
  - name: mimo-v2.5-pro
    purpose: coding
    max_output_tokens: 4096
    max_context_tokens: 1000000
    reasoning_level: high
    supports_prefix_cache: false
    capability_source: preset
    pricing:
      currency: CNY
      input_per_1m_tokens: 0
      cached_input_per_1m_tokens: 0
      output_per_1m_tokens: 0
      source: user
```

NekoMIMO only writes known capability presets. Unknown models remain unknown; the console does not guess context length, model reasoning behavior, or pricing. Pricing is never fetched from the network.

Capability helpers:

```sh
NekoMIMO model discover --provider mimo --write-capabilities
NekoMIMO model enrich --provider mimo
NekoMIMO model enrich --model mimo-v2.5-pro
NekoMIMO model enrich --all
```

`model enrich` fills missing capability fields without overwriting user-provided values and without writing API keys.

## Patch Workflow

NekoForge can show patch next steps:

```text
/preview wt_xxx
/review wt_xxx
/discard wt_xxx
```

It does not provide an apply action. Applying remains an explicit CLI command:

```sh
NekoMIMO patch apply wt_xxx
```

## Safety

NekoForge does not:

- print API keys
- print Authorization or Bearer tokens
- show hidden reasoning or chain-of-thought
- display sensitive diff content
- auto-apply patches
- auto-commit
- auto-push
- start a web dashboard
- introduce desktop, Electron, Tauri, cloud sync, login, or accounts
