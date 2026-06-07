# internal/server

`internal/server` implements the local Web Dashboard for MimoNeko.

The package is intentionally small and read-only:

- Uses Go standard library `net/http`, `html/template`, and `encoding/json`
- Reads run data from the EventStore
- Reuses `RunTimeline`, `ProgressState`, and `events.ComputeProgressState`
- Serves local HTML pages and JSON APIs
- Does not execute tools or call models
- Does not read repository source file contents
- Does not auto-apply, auto-commit, or auto-push

Default CLI entrypoint:

```sh
MimoNeko serve
```

Default listen address:

```text
http://127.0.0.1:8765
```

Routes:

- `GET /`
- `GET /runs/{run_id}`
- `GET /healthz`
- `GET /api/runs`
- `GET /api/runs/{run_id}`
- `GET /api/runs/{run_id}/events`

The server defensively sanitizes events before returning JSON or rendering HTML.
