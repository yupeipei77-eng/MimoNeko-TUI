package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mimoneko/mimoneko/internal/agent"
	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/task"
)

type RunCommand struct{}

func (c *RunCommand) Name() string { return "run" }

func (c *RunCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	goal := fs.String("goal", "", "task goal (required)")
	maxSteps := fs.Int("max-steps", 0, "max agent loop steps (default: from contract)")
	dryRun := fs.Bool("dry-run", true, "dry run mode (no side effects)")
	autoApproveMedium := fs.Bool("auto-approve-medium", false, "auto-approve medium-risk tools without prompting")
	taskID := fs.String("task-id", "", "task ID (auto-generated if empty)")
	useWorktree := fs.Bool("worktree", false, "run in isolated git worktree")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	hasGoalFlag := strings.TrimSpace(*goal) != ""
	hasPositionalGoal := len(remaining) > 0 && strings.TrimSpace(remaining[0]) != ""
	if hasGoalFlag && hasPositionalGoal {
		fmt.Fprintln(env.Stderr, "run accepts either --goal or positional goal, not both")
		return 2
	}
	if !hasGoalFlag && hasPositionalGoal {
		*goal = strings.TrimSpace(strings.Join(remaining, " "))
	}

	if strings.TrimSpace(*goal) == "" {
		fmt.Fprintln(env.Stderr, "run requires --goal")
		fmt.Fprintln(env.Stderr, "Usage: mimoneko run \"fix typo in README\"")
		fmt.Fprintln(env.Stderr, "   or: mimoneko \"fix typo in README\"")
		return 2
	}

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	if err := ensureProjectConfigForRun(root); err != nil {
		PrintErrorDetails(env.Stderr, "Run failed", "项目配置初始化失败。", "检查当前目录权限后重试。", err.Error())
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		PrintErrorDetails(env.Stderr, "Run failed", "加载项目配置失败。", "运行: mimoneko init", err.Error())
		return 1
	}
	cachePath := cacheRegistryPath(root, cfg)
	cacheLineCount := countCacheObservationLines(cachePath)

	contract := task.DefaultContract(root, *goal)
	contract.DryRun = *dryRun
	if *maxSteps > 0 {
		contract.MaxSteps = *maxSteps
	}

	tid := *taskID
	if tid == "" {
		tid = "task_cli_" + generateShortID()
	}

	deps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		PrintErrorDetails(env.Stderr, "Run failed", "无法构建运行依赖。", "运行: mimoneko doctor", err.Error())
		return 1
	}
	defer cleanup()

	rt := agent.NewSingleAgentRuntime(deps)
	rt.SetOutput(env.Stdout)

	policy := agent.InteractiveApprovalPolicy(os.Stdin)
	policy.AutoApproveMediumRisk = *autoApproveMedium
	rt.SetApprovalPolicy(policy)

	req := agent.AgentRunRequest{
		TaskID:      tid,
		RepoRoot:    root,
		Goal:        *goal,
		Contract:    contract,
		MaxSteps:    *maxSteps,
		DryRun:      *dryRun,
		UseWorktree: *useWorktree,
	}

	ui := newCLIUI()
	ui.PrintHeader(env.Stdout, "MioNeko Run")
	fmt.Fprintln(env.Stdout, "Goal:")
	fmt.Fprintln(env.Stdout, *goal)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, ui.Icon("model")+" Model:")
	fmt.Fprintln(env.Stdout, cfg.Models.Routing.DefaultModel)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "%s Running...\n", ui.Icon("gear"))

	result, err := rt.Run(context.Background(), req)
	if err != nil {
		reason, suggestion, details := friendlyModelError(err.Error())
		PrintErrorDetails(env.Stderr, "Run failed", reason, suggestion, details)
		return 1
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, statusValue(result.State == agent.AgentStateSucceeded, "Completed", "Failed"))
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "Result:")
	if result.FinalMessage != "" {
		fmt.Fprintln(env.Stdout, result.FinalMessage)
	} else if result.Error != "" {
		fmt.Fprintln(env.Stdout, result.Error)
	} else {
		fmt.Fprintln(env.Stdout, "(no final message)")
	}
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "Run ID:")
	fmt.Fprintln(env.Stdout, result.RunID)
	if result.WorktreeID != "" {
		fmt.Fprintln(env.Stdout)
		PrintKV(env.Stdout, "Worktree:", []KV{{Key: "ID", Value: result.WorktreeID}})
	}
	printRunTokens(env, cachePath, cacheLineCount)
	if result.PatchPreview != nil {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "%s Patch generated\n", ui.Icon("patch"))
		PrintKV(env.Stdout, "", []KV{
			{Key: "Files", Value: fmt.Sprintf("%d", result.PatchPreview.Summary.FilesChanged)},
			{Key: "Changes", Value: fmt.Sprintf("+%d / -%d", result.PatchPreview.Summary.Additions, result.PatchPreview.Summary.Deletions)},
			{Key: "Risk", Value: result.PatchPreview.RiskLevel},
		})
		if len(result.PatchPreview.Violations) > 0 {
			PrintWarning(env.Stdout, fmt.Sprintf("%d validation violation(s)", len(result.PatchPreview.Violations)))
		}
		fmt.Fprintln(env.Stdout, "Run:")
		fmt.Fprintf(env.Stdout, "mimoneko patch preview %s\n", result.WorktreeID)
		fmt.Fprintf(env.Stdout, "mimoneko patch apply %s\n", result.WorktreeID)
	}

	switch result.State {
	case agent.AgentStateSucceeded:
		return 0
	case agent.AgentStateFailed:
		return 1
	case agent.AgentStateCancelled:
		return 130
	case agent.AgentStateWaitingApproval:
		return 2
	default:
		return 1
	}
}

func init() {
	commands.Register(&RunCommand{})
}

func printRunTokens(env Env, cachePath string, beforeLines int) {
	observations := readCacheObservationsAfter(cachePath, beforeLines)
	inputTokens, cachedTokens := sumCacheObservations(observations)
	value := "unavailable"
	if inputTokens > 0 {
		value = percent(float64(cachedTokens) / float64(inputTokens))
	}
	rows := []KV{
		{Key: "Input", Value: tokenValue(inputTokens)},
		{Key: "Cached", Value: tokenValue(cachedTokens)},
		{Key: "Hit Rate", Value: value},
	}
	fmt.Fprintln(env.Stdout)
	PrintKV(env.Stdout, "Tokens:", rows)
}

func tokenValue(value int) string {
	if value <= 0 {
		return "0"
	}
	return fmt.Sprintf("%d", value)
}
