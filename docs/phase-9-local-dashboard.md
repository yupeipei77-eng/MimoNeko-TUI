# Phase 9 Local Dashboard

Phase 9 adds local, read-only dashboard surfaces for ReasonForge run progress.

## Phase 9.1: Local TUI Dashboard

`reasonforge dashboard` renders recent runs and run details in the terminal.

It reads from the EventStore and reuses:

- `RunTimeline`
- `ProgressState`
- `events.ComputeProgressState`

## Phase 9.2: Local Web Dashboard

`reasonforge serve` starts a browser-friendly local dashboard.

Default address:

```sh
reasonforge serve
# http://127.0.0.1:8765
```

Other examples:

```sh
reasonforge serve --port 9000
reasonforge serve --open
reasonforge serve --poll-interval 5s
```

The Web Dashboard provides:

- Recent Runs
- Run Detail
- Timeline
- ProgressState
- Events List
- JavaScript polling refresh

Terminal runs keep an explicit state visible in both Recent Runs and Run Detail. When a run reaches 100%:

- `succeeded` shows current phase `completed`
- `failed` shows current phase `failed`
- `cancelled` shows current phase `cancelled`

This keeps the dashboard readable after polling switches to an offline or idle visual state.

## HTTP API

The local server exposes read-only endpoints:

- `GET /healthz`
- `GET /api/runs`
- `GET /api/runs/{run_id}`
- `GET /api/runs/{run_id}/events`

## Safety

The server is local-only by default:

- Default host is `127.0.0.1`
- It does not execute tools
- It does not call models
- It does not read source file contents
- It does not auto-apply, auto-commit, or auto-push
- It only reads sanitized event data from EventStore
- It does not return raw sensitive diffs, file write content, or file patch old/new values

If `events.enabled=false`, the page shows a friendly disabled message. If the event store does not exist yet, the page shows `No runs yet`.
