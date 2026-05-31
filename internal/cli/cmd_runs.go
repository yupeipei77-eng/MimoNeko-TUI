package cli

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/events"
)

type RunsCommand struct{}

func (c *RunsCommand) Name() string { return "runs" }

func (c *RunsCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("runs", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	limit := fs.Int("limit", 20, "max number of runs to show")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "runs failed: %v\n", err)
		return 1
	}

	if !cfg.Events.Enabled {
		fmt.Fprintf(env.Stderr, "events system is disabled; enable in %s/events.yaml\n", config.DirName())
		return 1
	}

	store, cleanup, err := openEventStoreForRead(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "runs failed: %v\n", err)
		return 1
	}
	defer cleanup()

	summaries, err := store.ListRuns(context.Background())
	if err != nil {
		fmt.Fprintf(env.Stderr, "runs failed: %v\n", err)
		return 1
	}

	if len(summaries) == 0 {
		fmt.Fprintln(env.Stdout, "No runs found.")
		return 0
	}

	if *limit > 0 && len(summaries) > *limit {
		summaries = summaries[:*limit]
	}

	fmt.Fprintln(env.Stdout, "MimoNeko Runs")
	fmt.Fprintf(env.Stdout, "%-36s %-12s %-20s %s\n", "RUN ID", "STATE", "STARTED", "LAST EVENT")
	for _, s := range summaries {
		started := "-"
		if !s.StartedAt.IsZero() {
			started = s.StartedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(env.Stdout, "%-36s %-12s %-20s %s\n", s.RunID, s.State, started, s.LastEventType)
	}

	return 0
}

type RunStatusCommand struct{}

func (c *RunStatusCommand) Name() string { return "run-status" }

func (c *RunStatusCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("run-status", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "run-status requires a run ID")
		return 2
	}
	runID := remaining[0]

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-status failed: %v\n", err)
		return 1
	}

	if !cfg.Events.Enabled {
		fmt.Fprintf(env.Stderr, "events system is disabled; enable in %s/events.yaml\n", config.DirName())
		return 1
	}

	store, cleanup, err := openEventStoreForRead(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-status failed: %v\n", err)
		return 1
	}
	defer cleanup()

	timeline, err := store.GetTimeline(context.Background(), runID)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-status failed: %v\n", err)
		return 1
	}

	if timeline.RunID == "" {
		fmt.Fprintf(env.Stderr, "run %q not found\n", runID)
		return 1
	}

	progress := events.ComputeProgressState(timeline)

	fmt.Fprintf(env.Stdout, "run_id=%s\n", progress.RunID)
	fmt.Fprintf(env.Stdout, "state=%s\n", progress.State)
	if progress.CurrentPhase != "" {
		fmt.Fprintf(env.Stdout, "current_phase=%s\n", progress.CurrentPhase)
	}
	fmt.Fprintf(env.Stdout, "percent=%d\n", progress.Percent)
	fmt.Fprintf(env.Stdout, "completed_steps=%d\n", progress.CompletedSteps)
	fmt.Fprintf(env.Stdout, "total_steps=%d\n", progress.TotalSteps)
	fmt.Fprintf(env.Stdout, "last_event=%s\n", progress.LastEvent.Type)
	if progress.LastEvent.WorktreeID != "" {
		fmt.Fprintf(env.Stdout, "worktree_id=%s\n", progress.LastEvent.WorktreeID)
	}
	if !timeline.StartedAt.IsZero() {
		fmt.Fprintf(env.Stdout, "started_at=%s\n", timeline.StartedAt.Format(time.RFC3339))
	}
	if !timeline.FinishedAt.IsZero() {
		fmt.Fprintf(env.Stdout, "finished_at=%s\n", timeline.FinishedAt.Format(time.RFC3339))
	}
	if timeline.DurationMs > 0 {
		fmt.Fprintf(env.Stdout, "duration_ms=%d\n", timeline.DurationMs)
	}

	return 0
}

type RunEventsCommand struct{}

func (c *RunEventsCommand) Name() string { return "run-events" }

func (c *RunEventsCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("run-events", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(env.Stderr, "run-events requires a run ID")
		return 2
	}
	runID := remaining[0]

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-events failed: %v\n", err)
		return 1
	}

	if !cfg.Events.Enabled {
		fmt.Fprintf(env.Stderr, "events system is disabled; enable in %s/events.yaml\n", config.DirName())
		return 1
	}

	store, cleanup, err := openEventStoreForRead(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-events failed: %v\n", err)
		return 1
	}
	defer cleanup()

	evts, err := store.ListEvents(context.Background(), runID)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run-events failed: %v\n", err)
		return 1
	}

	if len(evts) == 0 {
		fmt.Fprintf(env.Stderr, "no events found for run %q\n", runID)
		return 1
	}

	fmt.Fprintln(env.Stdout, "MimoNeko Run Events")
	fmt.Fprintf(env.Stdout, "%-20s %-24s %-12s %s\n", "TIME", "TYPE", "STATUS", "MESSAGE")
	for _, evt := range evts {
		ts := "-"
		if !evt.StartedAt.IsZero() {
			ts = evt.StartedAt.Format("15:04:05.000")
		} else if !evt.FinishedAt.IsZero() {
			ts = evt.FinishedAt.Format("15:04:05.000")
		}
		msg := evt.Message
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		fmt.Fprintf(env.Stdout, "%-20s %-24s %-12s %s\n", ts, evt.Type, evt.Status, msg)
	}

	return 0
}

func init() {
	commands.Register(&RunsCommand{})
	commands.Register(&RunStatusCommand{})
	commands.Register(&RunEventsCommand{})
}
