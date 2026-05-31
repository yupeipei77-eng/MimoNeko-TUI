package cli

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/dashboard"
	"github.com/mimoneko/mimoneko/internal/events"
)

type DashboardCommand struct{}

func (c *DashboardCommand) Name() string { return "dashboard" }

func (c *DashboardCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	runID := fs.String("run", "", "show detail for a specific run ID")
	limit := fs.Int("limit", 20, "max number of runs to show")
	watch := fs.Bool("watch", false, "auto-refresh every 2 seconds")
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
		fmt.Fprintf(env.Stderr, "dashboard failed: %v\n", err)
		return 1
	}

	if !cfg.Events.Enabled {
		fmt.Fprintf(env.Stderr, "Events system is disabled. Enable it in %s/events.yaml to use the dashboard.\n", config.DirName())
		return 1
	}

	store, cleanup, err := openEventStoreForRead(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "dashboard failed: could not open event store: %v\n", err)
		fmt.Fprintln(env.Stderr, "Hint: run a command (e.g. 'mimoneko run --goal ...') to create events first.")
		return 1
	}
	defer cleanup()

	if *watch {
		return c.runWatch(env, store, *runID, *limit)
	}

	if *runID != "" {
		return c.runDetail(env, store, *runID)
	}

	return c.runList(env, store, *limit)
}

func (c *DashboardCommand) runList(env Env, store *events.JSONLRunEventStore, limit int) int {
	runs, err := store.ListRuns(context.Background())
	if err != nil {
		fmt.Fprintf(env.Stderr, "dashboard failed: %v\n", err)
		return 1
	}

	if len(runs) == 0 {
		fmt.Fprintln(env.Stdout, "MimoNeko Dashboard")
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, "No runs found. Run a command to create events:")
		fmt.Fprintln(env.Stdout, "  mimoneko run --goal \"your task\"")
		return 0
	}

	dashboard.RenderRunsListWithProgress(env.Stdout, store, runs, limit)
	return 0
}

func (c *DashboardCommand) runDetail(env Env, store *events.JSONLRunEventStore, runID string) int {
	if err := dashboard.RenderRunDetail(env.Stdout, store, runID); err != nil {
		fmt.Fprintf(env.Stderr, "dashboard: %v\n", err)
		return 1
	}
	return 0
}

func (c *DashboardCommand) runWatch(env Env, store *events.JSONLRunEventStore, runID string, limit int) int {
	for {
		fmt.Fprint(env.Stdout, "\033[2J\033[H")

		if runID != "" {
			if err := dashboard.RenderRunDetail(env.Stdout, store, runID); err != nil {
				fmt.Fprintf(env.Stderr, "dashboard: %v\n", err)
				return 1
			}
		} else {
			runs, err := store.ListRuns(context.Background())
			if err != nil {
				fmt.Fprintf(env.Stderr, "dashboard failed: %v\n", err)
				return 1
			}
			dashboard.RenderRunsListWithProgress(env.Stdout, store, runs, limit)
		}

		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, "(watching - refreshes every 2s - press Ctrl+C to exit)")

		time.Sleep(2 * time.Second)
	}
}

func init() {
	commands.Register(&DashboardCommand{})
}
