package cli

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/events"
)

func runNekoWorkflowEvents(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "Usage: neko events tools|agents [--dir <project_root>] [--limit n]")
		return 2
	}
	switch args[0] {
	case "tools":
		return runNekoWorkflowEventsTools(args[1:], env)
	case "agents":
		return runNekoWorkflowEventsAgents(args[1:], env)
	case "-h", "--help", "help":
		fmt.Fprintln(env.Stdout, "Usage: neko events tools|agents [--dir <project_root>] [--limit n]")
		return 0
	default:
		fmt.Fprintf(env.Stderr, "unknown neko events command %q\n", args[0])
		fmt.Fprintln(env.Stderr, "Usage: neko events tools|agents [--dir <project_root>] [--limit n]")
		return 2
	}
}

func runNekoWorkflowEventsTools(args []string, env Env) int {
	fs := flag.NewFlagSet("neko events tools", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	limit := fs.Int("limit", 20, "max number of tool events to show")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}

	root, err := workflowStartDir(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stdout, "unavailable")
		return 0
	}
	cfg, err := config.Load(root)
	if err != nil || !cfg.Events.Enabled {
		fmt.Fprintln(env.Stdout, "unavailable")
		return 0
	}

	eventStorePath := cfg.Events.StorePath
	if !filepath.IsAbs(eventStorePath) {
		eventStorePath = filepath.Join(root, eventStorePath)
	}
	store, err := events.OpenJSONLRunEventStoreReadOnly(eventStorePath)
	if err != nil {
		fmt.Fprintln(env.Stdout, "unavailable")
		return 0
	}
	defer store.Close()

	toolEvents, err := recentToolEvents(store, *limit)
	if err != nil {
		fmt.Fprintln(env.Stdout, "unavailable")
		return 0
	}
	if len(toolEvents) == 0 {
		fmt.Fprintln(env.Stdout, "no tool events")
		return 0
	}

	fmt.Fprintln(env.Stdout, "MimoNeko Tool Events")
	fmt.Fprintf(env.Stdout, "%-20s %-18s %-14s %-12s %-8s %-8s %s\n", "TIME", "TYPE", "TOOL", "RISK", "APPROVAL", "MS", "STATUS")
	for _, evt := range toolEvents {
		status := firstNonEmpty(evt.ResultStatus, evt.Status)
		if errMsg := firstNonEmpty(evt.ErrorMessage, evt.Error); errMsg != "" {
			status = status + " " + truncateForTable(errMsg, 36)
		}
		fmt.Fprintf(
			env.Stdout,
			"%-20s %-18s %-14s %-12s %-8s %-8d %s\n",
			formatEventTime(evt),
			evt.Type,
			firstNonEmpty(evt.ToolName, evt.Metadata["tool_name"]),
			firstNonEmpty(evt.RiskLevel, evt.Metadata["risk_level"]),
			formatApproval(evt.RequiresApproval),
			evt.DurationMs,
			status,
		)
	}
	return 0
}

func recentToolEvents(store events.EventStore, limit int) ([]events.RunEvent, error) {
	summaries, err := store.ListRuns(context.Background())
	if err != nil {
		return nil, err
	}
	var toolEvents []events.RunEvent
	for _, summary := range summaries {
		runEvents, err := store.ListEvents(context.Background(), summary.RunID)
		if err != nil {
			return nil, err
		}
		for _, evt := range runEvents {
			if isNekoToolEvent(evt.Type) {
				toolEvents = append(toolEvents, evt)
			}
		}
	}
	slices.SortFunc(toolEvents, func(a, b events.RunEvent) int {
		return cmp.Compare(eventTime(b).UnixNano(), eventTime(a).UnixNano())
	})
	if limit > 0 && len(toolEvents) > limit {
		toolEvents = toolEvents[:limit]
	}
	return toolEvents, nil
}

func isNekoToolEvent(eventType events.EventType) bool {
	switch eventType {
	case events.EventToolCalled, events.EventToolCompleted, events.EventToolFailed,
		events.EventToolStarted, events.EventToolFinished,
		events.EventPathViolationCandidate:
		return true
	default:
		return false
	}
}

func eventTime(evt events.RunEvent) time.Time {
	if !evt.Timestamp.IsZero() {
		return evt.Timestamp
	}
	if !evt.FinishedAt.IsZero() {
		return evt.FinishedAt
	}
	return evt.StartedAt
}

func formatEventTime(evt events.RunEvent) string {
	ts := eventTime(evt)
	if ts.IsZero() {
		return "-"
	}
	return ts.Format("2006-01-02 15:04:05")
}

func formatApproval(value *bool) string {
	if value == nil {
		return "-"
	}
	if *value {
		return "true"
	}
	return "false"
}

func truncateForTable(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return value[:maxLen-3] + "..."
}

func runNekoWorkflowEventsAgents(args []string, env Env) int {
	fs := flag.NewFlagSet("neko events agents", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	limit := fs.Int("limit", 20, "max number of agent events to show")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}

	root, err := workflowStartDir(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stdout, "unavailable")
		return 0
	}
	cfg, err := config.Load(root)
	if err != nil || !cfg.Events.Enabled {
		fmt.Fprintln(env.Stdout, "unavailable")
		return 0
	}

	eventStorePath := cfg.Events.StorePath
	if !filepath.IsAbs(eventStorePath) {
		eventStorePath = filepath.Join(root, eventStorePath)
	}
	store, err := events.OpenJSONLRunEventStoreReadOnly(eventStorePath)
	if err != nil {
		fmt.Fprintln(env.Stdout, "unavailable")
		return 0
	}
	defer store.Close()

	agentEvents, err := recentAgentEvents(store, *limit)
	if err != nil {
		fmt.Fprintln(env.Stdout, "unavailable")
		return 0
	}
	if len(agentEvents) == 0 {
		fmt.Fprintln(env.Stdout, "no agent events")
		return 0
	}

	fmt.Fprintln(env.Stdout, "MimoNeko Agent Workflow Events")
	fmt.Fprintf(env.Stdout, "%-20s %-24s %-12s %-12s %s\n", "TIME", "TYPE", "ROLE", "STATUS", "MESSAGE")
	for _, evt := range agentEvents {
		role := evt.Metadata["role"]
		status := firstNonEmpty(evt.ResultStatus, evt.Status)
		message := truncateForTable(evt.Message, 50)
		fmt.Fprintf(
			env.Stdout,
			"%-20s %-24s %-12s %-12s %s\n",
			formatEventTime(evt),
			evt.Type,
			role,
			status,
			message,
		)
	}
	return 0
}

func recentAgentEvents(store events.EventStore, limit int) ([]events.RunEvent, error) {
	summaries, err := store.ListRuns(context.Background())
	if err != nil {
		return nil, err
	}
	var agentEvents []events.RunEvent
	for _, summary := range summaries {
		runEvents, err := store.ListEvents(context.Background(), summary.RunID)
		if err != nil {
			return nil, err
		}
		for _, evt := range runEvents {
			if isAgentWorkflowEvent(evt.Type) {
				agentEvents = append(agentEvents, evt)
			}
		}
	}
	slices.SortFunc(agentEvents, func(a, b events.RunEvent) int {
		return cmp.Compare(eventTime(a).UnixNano(), eventTime(b).UnixNano())
	})
	if limit > 0 && len(agentEvents) > limit {
		agentEvents = agentEvents[:limit]
	}
	return agentEvents, nil
}

func isAgentWorkflowEvent(eventType events.EventType) bool {
	switch eventType {
	case events.EventWorkflowStarted, events.EventStepStarted,
		events.EventStepCompleted, events.EventStepFailed,
		events.EventWorkflowCompleted, events.EventWorkflowFailed:
		return true
	default:
		return false
	}
}
