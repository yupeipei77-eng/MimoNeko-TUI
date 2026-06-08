# MimoNeko TUI Workbench

MimoNeko provides a terminal-native AI coding workbench focused on MIMO models.
The TUI is the primary experience for chat, model switching, provider setup,
agent mode selection, and cache visibility.

## Start

```powershell
mimoneko
mimoneko neko
mimoneko neko --dir .
mimoneko neko --mode single
mimoneko neko --mode multi
mimoneko neko --model mimo-v2.5-pro
mimoneko neko --reasoning high
mimoneko neko --no-color
```

The startup view uses a warm OpenCode-style composer and a centered MimoNeko
wordmark. The composer uses the terminal's native cursor instead of a drawn fake
cursor, so Chinese wide characters and emoji do not place a `|` inside text.

## Commands

```text
/
/help
/agents
/cache
/connect
/model
/models
/model test
/model enrich
/mode single
/mode multi
/new
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

Plain text input is chat with the configured model. Use `/run <goal>` when you
want to execute the agent runtime.

## Provider Setup

`/models` keeps the model picker. Selecting `Connect provider`, or running
`/connect`, starts a step-based provider flow in the TUI instead of opening a
large overlay form:

```text
Provider name
Base URL
API key
Header type
Discovering models...
Provider saved
```

API keys are masked while typing and in all status output.

## Cache Display

`/cache` shows:

- context usage
- input tokens
- cached tokens
- cache hit rate

If a provider does not return cached-token statistics, the TUI shows
`unsupported` instead of treating it as an error.

## Agent Modes

`/agents` lists the available mode definitions and their safety envelope:

- Build
- Single
- Explore
- Plan
- Builder
- Reviewer

Each mode declares a name, description, allowed tools, write permission, and
whether it expects a worktree.

## Safety

Defaults are intentionally conservative:

- no auto commit
- no auto push
- no direct writes by default
- patch apply requires explicit approval
- sensitive files and paths are blocked by the path guard
