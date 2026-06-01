package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/contextengine"
	"github.com/mimoneko/mimoneko/internal/events"
	"github.com/mimoneko/mimoneko/internal/pathutil"
)

type gitWorktreeCounts struct {
	Staged    int
	Unstaged  int
	Untracked int
}

func (c gitWorktreeCounts) Clean() bool {
	return c.Staged == 0 && c.Unstaged == 0 && c.Untracked == 0
}

func runNekoWorkflowStatus(args []string, env Env) int {
	fs := flag.NewFlagSet("neko status", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}

	root, err := resolveWorkflowGitRoot(*dir, env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "neko status failed: %v\n", err)
		return 1
	}
	branch, err := gitOutput(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		fmt.Fprintf(env.Stderr, "neko status failed: %v\n", err)
		return 1
	}
	counts, err := gitWorktreeStatus(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "neko status failed: %v\n", err)
		return 1
	}

	PrintHeader(env.Stdout, "Neko Status")
	PrintKV(env.Stdout, "Git:", []KV{
		{Key: "Root", Value: root},
		{Key: "Branch", Value: strings.TrimSpace(branch)},
		{Key: "Clean", Value: strconv.FormatBool(counts.Clean())},
	})
	fmt.Fprintln(env.Stdout)
	PrintKV(env.Stdout, "Changes:", []KV{
		{Key: "Staged", Value: strconv.Itoa(counts.Staged)},
		{Key: "Unstaged", Value: strconv.Itoa(counts.Unstaged)},
		{Key: "Untracked", Value: strconv.Itoa(counts.Untracked)},
	})
	fmt.Fprintln(env.Stdout)
	PrintKV(env.Stdout, "Latest Run:", latestRunRows(root))
	return 0
}

func runNekoWorkflowDiff(args []string, env Env) int {
	fs := flag.NewFlagSet("neko diff", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	staged := fs.Bool("staged", false, "show staged diff")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}

	root, err := resolveWorkflowGitRoot(*dir, env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "neko diff failed: %v\n", err)
		return 1
	}
	gitArgs := []string{"diff", "--no-ext-diff"}
	if *staged {
		gitArgs = append(gitArgs, "--cached")
	}
	gitArgs = append(gitArgs, "--")
	diff, err := gitOutput(root, gitArgs...)
	if err != nil {
		fmt.Fprintf(env.Stderr, "neko diff failed: %v\n", err)
		return 1
	}
	if strings.TrimSpace(diff) == "" {
		fmt.Fprintln(env.Stdout, "No diff.")
		return 0
	}
	fmt.Fprint(env.Stdout, diff)
	return 0
}

func runNekoWorkflowPlan(args []string, env Env) int {
	fs := flag.NewFlagSet("neko plan", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	goal := fs.String("goal", "", "goal to plan")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}
	if strings.TrimSpace(*goal) == "" {
		fmt.Fprintln(env.Stderr, "neko plan requires --goal")
		return 2
	}

	root, err := workflowStartDir(*dir, env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "neko plan failed: %v\n", err)
		return 1
	}
	stats := contextengine.DefaultObservableSnapshot(*goal).Stats()
	plan := nekoPlanSkeleton{
		Command:              "neko plan",
		Goal:                 strings.TrimSpace(*goal),
		ProjectRoot:          root,
		PrefixFingerprint:    stats.PrefixFingerprint,
		ImplementationStatus: "stub",
		WritesFiles:          false,
		CallsModel:           false,
		Steps: []nekoPlanStep{
			{ID: "understand", Title: "Clarify goal and constraints", Status: "pending"},
			{ID: "inspect", Title: "Inspect relevant repository context", Status: "pending"},
			{ID: "propose", Title: "Propose a small, reviewable change set", Status: "pending"},
			{ID: "verify", Title: "List validation commands before implementation", Status: "pending"},
		},
		NextAction: "Review this plan skeleton before running implementation commands.",
	}

	encoder := json.NewEncoder(env.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(plan); err != nil {
		fmt.Fprintf(env.Stderr, "neko plan failed: %v\n", err)
		return 1
	}
	return 0
}

func runNekoWorkflowCache(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "Usage: neko cache stats [--dir <project_root>]")
		return 2
	}
	switch args[0] {
	case "stats":
		return runNekoWorkflowCacheStats(args[1:], env)
	case "-h", "--help", "help":
		fmt.Fprintln(env.Stdout, "Usage: neko cache stats [--dir <project_root>]")
		return 0
	default:
		fmt.Fprintf(env.Stderr, "unknown neko cache command %q\n", args[0])
		fmt.Fprintln(env.Stderr, "Usage: neko cache stats [--dir <project_root>]")
		return 2
	}
}

func runNekoWorkflowCacheStats(args []string, env Env) int {
	fs := flag.NewFlagSet("neko cache stats", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}
	if _, err := workflowStartDir(*dir, env); err != nil {
		fmt.Fprintf(env.Stderr, "neko cache stats failed: %v\n", err)
		return 1
	}

	stats := contextengine.DefaultObservableSnapshot("").Stats()
	encoder := json.NewEncoder(env.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(stats); err != nil {
		fmt.Fprintf(env.Stderr, "neko cache stats failed: %v\n", err)
		return 1
	}
	return 0
}

func runNekoWorkflowTools(args []string, env Env) int {
	return runToolsList(args, env)
}

type nekoPlanSkeleton struct {
	Command              string         `json:"command"`
	Goal                 string         `json:"goal"`
	ProjectRoot          string         `json:"project_root"`
	PrefixFingerprint    string         `json:"prefix_fingerprint"`
	ImplementationStatus string         `json:"implementation_status"`
	WritesFiles          bool           `json:"writes_files"`
	CallsModel           bool           `json:"calls_model"`
	Steps                []nekoPlanStep `json:"steps"`
	NextAction           string         `json:"next_action"`
}

type nekoPlanStep struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func flagExitCode(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	return 2
}

func resolveWorkflowGitRoot(dir string, env Env) (string, error) {
	start, err := workflowStartDir(dir, env)
	if err != nil {
		return "", err
	}
	root, err := gitOutput(start, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return pathutil.CleanPath(strings.TrimSpace(root)), nil
}

func workflowStartDir(dir string, env Env) (string, error) {
	if strings.TrimSpace(dir) != "" {
		return pathutil.AbsPath(dir), nil
	}
	wd, err := env.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return pathutil.AbsPath(wd), nil
}

func gitWorktreeStatus(root string) (gitWorktreeCounts, error) {
	status, err := gitOutput(root, "status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return gitWorktreeCounts{}, err
	}
	var counts gitWorktreeCounts
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		if x == '?' && y == '?' {
			counts.Untracked++
			continue
		}
		if x != ' ' {
			counts.Staged++
		}
		if y != ' ' {
			counts.Unstaged++
		}
	}
	return counts, nil
}

func latestRunRows(root string) []KV {
	cfg, err := config.Load(root)
	if err != nil || !cfg.Events.Enabled {
		return []KV{{Key: "Status", Value: "unavailable"}}
	}
	eventStorePath := cfg.Events.StorePath
	if !filepath.IsAbs(eventStorePath) {
		eventStorePath = filepath.Join(root, eventStorePath)
	}
	store, err := events.OpenJSONLRunEventStoreReadOnly(eventStorePath)
	if err != nil {
		return []KV{{Key: "Status", Value: "unavailable"}}
	}
	defer store.Close()

	summaries, err := store.ListRuns(context.Background())
	if err != nil || len(summaries) == 0 {
		return []KV{{Key: "Status", Value: "unavailable"}}
	}
	latest := summaries[0]
	rows := []KV{
		{Key: "Run ID", Value: latest.RunID},
		{Key: "State", Value: latest.State},
		{Key: "Last Event", Value: string(latest.LastEventType)},
	}
	if !latest.StartedAt.IsZero() {
		rows = append(rows, KV{Key: "Started", Value: latest.StartedAt.Format("2006-01-02 15:04:05")})
	}
	return rows
}

func gitOutput(root string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", cmdArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		details := strings.TrimSpace(stderr.String())
		if details == "" {
			details = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), details)
	}
	return stdout.String(), nil
}
