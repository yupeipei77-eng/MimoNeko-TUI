package server

import "html/template"

type homePageData struct {
	Runs           []RunItem
	Message        string
	EventsEnabled  bool
	PollIntervalMS int
}

type runPageData struct {
	Detail         runDetailResponse
	WorktreeID     string
	PollIntervalMS int
}

type messagePageData struct {
	Title   string
	Message string
}

var homeTemplate = template.Must(template.New("home").Parse(baseCSS + `
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ReasonForge Dashboard</title>
</head>
<body>
  <header>
    <h1>ReasonForge Dashboard</h1>
    <p class="muted">Local read-only run monitor</p>
  </header>
  <main>
    <section>
      <div class="section-head">
        <h2>Recent Runs</h2>
        <span id="refresh-state" class="muted">polling</span>
      </div>
      <p id="message" class="notice">{{.Message}}</p>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Run ID</th>
              <th>State</th>
              <th>Progress</th>
              <th>Phase</th>
              <th>Last Event</th>
              <th>Started</th>
            </tr>
          </thead>
          <tbody id="runs-body">
          {{range .Runs}}
            <tr>
              <td><a href="/runs/{{.RunID}}">{{.RunID}}</a></td>
              <td><span class="state state-{{.State}}">{{.State}}</span></td>
              <td><div class="progress"><span style="width: {{.Percent}}%"></span></div><small>{{.Percent}}%</small></td>
              <td>{{.CurrentPhase}}</td>
              <td>{{.LastEvent}}</td>
              <td>{{.StartedAt}}</td>
            </tr>
          {{end}}
          </tbody>
        </table>
      </div>
    </section>
  </main>
  <script>
    const pollInterval = {{.PollIntervalMS}};
    function text(value) { return value || ""; }
    function setMessage(value) {
      const el = document.getElementById("message");
      el.textContent = value || "";
      el.style.display = value ? "block" : "none";
    }
    function renderRuns(data) {
      setMessage(data.message);
      const body = document.getElementById("runs-body");
      body.textContent = "";
      for (const run of data.runs || []) {
        const tr = document.createElement("tr");
        const id = document.createElement("td");
        const link = document.createElement("a");
        link.href = "/runs/" + encodeURIComponent(run.run_id);
        link.textContent = run.run_id;
        id.appendChild(link);
        tr.appendChild(id);

        const state = document.createElement("td");
        const badge = document.createElement("span");
        badge.className = "state state-" + text(run.state);
        badge.textContent = text(run.state);
        state.appendChild(badge);
        tr.appendChild(state);

        const progress = document.createElement("td");
        const bar = document.createElement("div");
        bar.className = "progress";
        const fill = document.createElement("span");
        fill.style.width = Math.max(0, Math.min(100, run.percent || 0)) + "%";
        bar.appendChild(fill);
        const pct = document.createElement("small");
        pct.textContent = (run.percent || 0) + "%";
        progress.appendChild(bar);
        progress.appendChild(pct);
        tr.appendChild(progress);

        for (const key of ["current_phase", "last_event", "started_at"]) {
          const td = document.createElement("td");
          td.textContent = text(run[key]);
          tr.appendChild(td);
        }
        body.appendChild(tr);
      }
    }
    async function refreshRuns() {
      try {
        const res = await fetch("/api/runs", {cache: "no-store"});
        renderRuns(await res.json());
        document.getElementById("refresh-state").textContent = "updated";
      } catch {
        document.getElementById("refresh-state").textContent = "offline";
      }
    }
    setInterval(refreshRuns, pollInterval);
  </script>
</body>
</html>
`))

var runTemplate = template.Must(template.New("run").Parse(baseCSS + `
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ReasonForge Run Detail</title>
</head>
<body>
  <header>
    <h1>ReasonForge Dashboard</h1>
    <p><a href="/">Recent Runs</a></p>
  </header>
  <main>
    <section>
      <div class="section-head">
        <h2 id="run-id">{{.Detail.RunID}}</h2>
        <span id="refresh-state" class="muted">polling</span>
      </div>
      <div class="summary-grid">
        <div><span class="label">State</span><strong id="state">{{.Detail.State}}</strong></div>
        <div><span class="label">Progress</span><strong id="percent">{{.Detail.Progress.Percent}}%</strong></div>
        <div><span class="label">Current Phase</span><strong id="phase">{{.Detail.Progress.CurrentPhase}}</strong></div>
        <div><span class="label">Worktree</span><strong id="worktree">{{.WorktreeID}}</strong></div>
      </div>
      <div class="progress large"><span id="progress-fill" style="width: {{.Detail.Progress.Percent}}%"></span></div>
    </section>
    <section>
      <h2>Timeline</h2>
      <ol id="timeline" class="timeline">
      {{range .Detail.Timeline.Steps}}
        <li><span class="state state-{{.Status}}">{{.Status}}</span> <strong>{{.Type}}</strong> {{.Message}}</li>
      {{end}}
      </ol>
    </section>
    <section>
      <h2>Events</h2>
      <div class="table-wrap">
        <table>
          <thead><tr><th>Type</th><th>Status</th><th>Source</th><th>Message</th></tr></thead>
          <tbody id="events-body">
          {{range .Detail.Timeline.Events}}
            <tr><td>{{.Type}}</td><td>{{.Status}}</td><td>{{.Source}}</td><td>{{.Message}}</td></tr>
          {{end}}
          </tbody>
        </table>
      </div>
    </section>
  </main>
  <script>
    const pollInterval = {{.PollIntervalMS}};
    const runID = {{printf "%q" .Detail.RunID}};
    function text(value) { return value || ""; }
    function renderTimeline(steps) {
      const list = document.getElementById("timeline");
      list.textContent = "";
      for (const step of steps || []) {
        const li = document.createElement("li");
        const badge = document.createElement("span");
        badge.className = "state state-" + text(step.status);
        badge.textContent = text(step.status);
        const strong = document.createElement("strong");
        strong.textContent = " " + text(step.type) + " ";
        li.appendChild(badge);
        li.appendChild(strong);
        li.appendChild(document.createTextNode(text(step.message)));
        list.appendChild(li);
      }
    }
    function renderEvents(events) {
      const body = document.getElementById("events-body");
      body.textContent = "";
      for (const event of events || []) {
        const tr = document.createElement("tr");
        for (const key of ["type", "status", "source", "message"]) {
          const td = document.createElement("td");
          td.textContent = text(event[key]);
          tr.appendChild(td);
        }
        body.appendChild(tr);
      }
    }
    async function refreshRun() {
      try {
        const detailRes = await fetch("/api/runs/" + encodeURIComponent(runID), {cache: "no-store"});
        const detail = await detailRes.json();
        document.getElementById("state").textContent = text(detail.state);
        document.getElementById("percent").textContent = (detail.progress.percent || 0) + "%";
        document.getElementById("phase").textContent = text(detail.progress.current_phase);
        document.getElementById("progress-fill").style.width = Math.max(0, Math.min(100, detail.progress.percent || 0)) + "%";
        renderTimeline((detail.timeline && detail.timeline.steps) || []);

        const eventsRes = await fetch("/api/runs/" + encodeURIComponent(runID) + "/events", {cache: "no-store"});
        renderEvents((await eventsRes.json()).events || []);
        document.getElementById("refresh-state").textContent = "updated";
      } catch {
        document.getElementById("refresh-state").textContent = "offline";
      }
    }
    setInterval(refreshRun, pollInterval);
  </script>
</body>
</html>
`))

var messageTemplate = template.Must(template.New("message").Parse(baseCSS + `
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
</head>
<body>
  <header>
    <h1>ReasonForge Dashboard</h1>
    <p><a href="/">Recent Runs</a></p>
  </header>
  <main>
    <section>
      <h2>{{.Title}}</h2>
      <p class="notice">{{.Message}}</p>
    </section>
  </main>
</body>
</html>
`))

const baseCSS = `
<style>
  :root {
    color-scheme: light;
    --bg: #f7f8fa;
    --panel: #ffffff;
    --text: #18202a;
    --muted: #637083;
    --line: #d9dee7;
    --accent: #24745b;
    --accent-soft: #dff3ea;
    --warn: #8a5a16;
    --bad: #a7333f;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    background: var(--bg);
    color: var(--text);
    font: 14px/1.5 ui-sans-serif, system-ui, -apple-system, Segoe UI, sans-serif;
  }
  header {
    padding: 24px 32px 16px;
    border-bottom: 1px solid var(--line);
    background: var(--panel);
  }
  h1, h2 { margin: 0; font-weight: 700; letter-spacing: 0; }
  h1 { font-size: 24px; }
  h2 { font-size: 18px; }
  main { padding: 24px 32px 48px; max-width: 1180px; }
  section { margin-bottom: 28px; }
  a { color: var(--accent); text-decoration: none; }
  a:hover { text-decoration: underline; }
  .muted { color: var(--muted); }
  .notice {
    display: block;
    color: var(--muted);
    margin: 16px 0;
  }
  .section-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 12px;
  }
  .table-wrap {
    overflow-x: auto;
    border: 1px solid var(--line);
    background: var(--panel);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    min-width: 760px;
  }
  th, td {
    padding: 10px 12px;
    border-bottom: 1px solid var(--line);
    text-align: left;
    vertical-align: top;
    white-space: nowrap;
  }
  th {
    color: var(--muted);
    font-size: 12px;
    text-transform: uppercase;
  }
  .state {
    display: inline-block;
    min-width: 72px;
    padding: 2px 8px;
    border: 1px solid var(--line);
    background: #f4f6f8;
    font-size: 12px;
  }
  .state-succeeded { background: var(--accent-soft); border-color: #afd9c8; color: var(--accent); }
  .state-failed { background: #fde9ec; border-color: #edb7c0; color: var(--bad); }
  .state-running, .state-started { background: #fff4df; border-color: #ecd29b; color: var(--warn); }
  .progress {
    display: inline-block;
    width: 120px;
    height: 8px;
    margin-right: 8px;
    background: #e8ecf1;
    vertical-align: middle;
  }
  .progress span {
    display: block;
    height: 100%;
    background: var(--accent);
  }
  .progress.large {
    display: block;
    width: 100%;
    height: 12px;
    margin-top: 16px;
  }
  .summary-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
    gap: 12px;
    margin-top: 12px;
  }
  .summary-grid > div {
    border: 1px solid var(--line);
    background: var(--panel);
    padding: 12px;
  }
  .label {
    display: block;
    color: var(--muted);
    font-size: 12px;
    text-transform: uppercase;
  }
  .timeline {
    margin: 12px 0 0;
    padding-left: 22px;
  }
  .timeline li {
    margin-bottom: 8px;
    padding: 8px 0;
    border-bottom: 1px solid var(--line);
  }
  @media (max-width: 720px) {
    header, main { padding-left: 16px; padding-right: 16px; }
    .section-head { display: block; }
  }
</style>
`
