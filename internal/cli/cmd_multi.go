package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/multiagent"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/task"
)

type MultiRunCommand struct{}

func (c *MultiRunCommand) Name() string { return "multi-run" }

func (c *MultiRunCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("multi-run", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	goalFlag := fs.String("goal", "", "task goal")
	model := fs.String("model", "", "model name (default: from config)")
	maxIterations := fs.Int("max-iterations", 0, "max iterations (default: 2, max: 5)")
	dryRun := fs.Bool("dry-run", true, "dry run mode (no side effects)")
	useWorktree := fs.Bool("worktree", true, "run in isolated git worktree (default: true)")
	approveMedium := fs.Bool("approve-medium", false, "auto-approve medium-risk tools for coder agent")
	modelReview := fs.Bool("model-review", false, "use AI model review in reviewer agent")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	remaining := fs.Args()
	hasGoalFlag := strings.TrimSpace(*goalFlag) != ""
	hasPositionalGoal := len(remaining) > 0 && strings.TrimSpace(remaining[0]) != ""
	if hasGoalFlag && hasPositionalGoal {
		fmt.Fprintln(env.Stderr, "multi-run accepts either --goal or positional goal, not both")
		return 2
	}
	if !hasGoalFlag && !hasPositionalGoal {
		fmt.Fprintln(env.Stderr, "multi-run requires a goal argument")
		fmt.Fprintln(env.Stderr, "Usage: mimoneko multi-run --goal \"fix typo in README\"")
		fmt.Fprintln(env.Stderr, "   or: mimoneko multi-run \"fix typo in README\"")
		return 2
	}
	goal := strings.TrimSpace(*goalFlag)
	if goal == "" {
		goal = strings.TrimSpace(remaining[0])
	}

	if !*useWorktree {
		fmt.Fprintln(env.Stderr, "multi-run requires worktree isolation; --worktree=false is not supported")
		return 2
	}

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	if err := ensureProjectConfigForRun(root); err != nil {
		fmt.Fprintf(env.Stderr, "multi-run failed: %v\n", err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "multi-run failed: %v\n", err)
		return 1
	}

	if *maxIterations == 0 {
		*maxIterations = cfg.MultiAgent.MaxIterations
	}
	if !*modelReview {
		*modelReview = cfg.MultiAgent.ReviewerUseModelReview
	}

	validatedMaxIter, err := multiagent.ValidateMaxIterations(*maxIterations)
	if err != nil {
		fmt.Fprintf(env.Stderr, "multi-run failed: %v\n", err)
		return 2
	}

	contract := task.DefaultContract(root, goal)
	contract.DryRun = *dryRun
	if *approveMedium {
		contract.RequireApprovalForRisk = []string{"high"}
	}

	taskID := "task_multi_" + generateShortID()

	agentDeps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		fmt.Fprintf(env.Stderr, "multi-run failed: %v\n", err)
		return 1
	}
	defer cleanup()

	multiDeps, multiCleanup, err := buildMultiAgentDependencies(root, cfg, agentDeps)
	if err != nil {
		fmt.Fprintf(env.Stderr, "multi-run failed: %v\n", err)
		return 1
	}
	defer multiCleanup()

	rt := multiagent.NewDefaultMultiAgentRuntime(multiDeps)

	req := multiagent.MultiAgentRunRequest{
		TaskID:         taskID,
		RepoRoot:       root,
		Goal:           goal,
		Contract:       contract,
		MaxIterations:  validatedMaxIter,
		DryRun:         *dryRun,
		UseWorktree:    *useWorktree,
		UseModelReview: *modelReview,
		Model:          *model,
		Metadata:       map[string]string{"source": "cli"},
	}

	fmt.Fprintf(env.Stdout, "MimoNeko Multi-Agent\n")
	fmt.Fprintf(env.Stdout, "goal=%q max_iterations=%d dry_run=%v worktree=%v\n",
		goal, validatedMaxIter, *dryRun, *useWorktree)

	result, err := rt.Run(context.Background(), req)
	if err != nil {
		fmt.Fprintf(env.Stderr, "multi-run failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "run_id=%s\n", result.RunID)
	fmt.Fprintf(env.Stdout, "state=%s\n", result.State)
	if result.WorktreeID != "" {
		fmt.Fprintf(env.Stdout, "worktree_id=%s\n", result.WorktreeID)
	}

	if len(result.Plan.Steps) > 0 {
		fmt.Fprintf(env.Stdout, "plan_steps=%d risk_level=%s\n", len(result.Plan.Steps), result.Plan.RiskLevel)
		for _, step := range result.Plan.Steps {
			fmt.Fprintf(env.Stdout, "  step %d: %s\n", step.Index, step.Title)
		}
	}

	for _, iter := range result.Iterations {
		fmt.Fprintf(env.Stdout, "iteration %d: recommendation=%s\n", iter.Index, iter.Recommendation)
	}

	fmt.Fprintf(env.Stdout, "final_recommendation=%s\n", result.FinalRecommendation)

	if result.Error != "" {
		fmt.Fprintf(env.Stdout, "error=%s\n", result.Error)
	}

	if result.State == multiagent.MultiAgentStateSucceeded && result.WorktreeID != "" {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "To apply changes, run:\n")
		fmt.Fprintf(env.Stdout, "  mimoneko patch apply %s\n", result.WorktreeID)
	}

	switch result.State {
	case multiagent.MultiAgentStateSucceeded:
		return 0
	case multiagent.MultiAgentStateRejected, multiagent.MultiAgentStateFailed:
		return 1
	case multiagent.MultiAgentStateCancelled:
		return 130
	case multiagent.MultiAgentStateRequestChanges:
		return 1
	default:
		return 1
	}
}

func init() {
	commands.Register(&MultiRunCommand{})
}
