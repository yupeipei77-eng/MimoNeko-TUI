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
		fmt.Fprintf(env.Stderr, "run failed: %v\n", err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run failed: %v\n", err)
		return 1
	}

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
		fmt.Fprintf(env.Stderr, "run failed: %v\n", err)
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

	fmt.Fprintf(env.Stdout, "MimoNeko Agent\n")
	fmt.Fprintf(env.Stdout, "run_id=pending goal=%q max_steps=%d dry_run=%v worktree=%v\n", *goal, contract.MaxSteps, contract.DryRun, *useWorktree)

	result, err := rt.Run(context.Background(), req)
	if err != nil {
		fmt.Fprintf(env.Stderr, "run failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "run_id=%s state=%s steps=%d\n", result.RunID, result.State, len(result.Steps))
	if result.WorktreeID != "" {
		fmt.Fprintf(env.Stdout, "worktree_id=%s\n", result.WorktreeID)
	}
	if result.FinalMessage != "" {
		fmt.Fprintf(env.Stdout, "message=%s\n", result.FinalMessage)
	}
	if result.Error != "" {
		fmt.Fprintf(env.Stdout, "error=%s\n", result.Error)
	}
	if result.PatchPreview != nil {
		fmt.Fprintf(env.Stdout, "patch_preview:\n")
		fmt.Fprintf(env.Stdout, "  files_changed=%d\n", result.PatchPreview.Summary.FilesChanged)
		fmt.Fprintf(env.Stdout, "  additions=%d deletions=%d\n", result.PatchPreview.Summary.Additions, result.PatchPreview.Summary.Deletions)
		fmt.Fprintf(env.Stdout, "  risk_level=%s\n", result.PatchPreview.RiskLevel)
		if len(result.PatchPreview.Violations) > 0 {
			fmt.Fprintf(env.Stdout, "  violations=%d\n", len(result.PatchPreview.Violations))
		}
		fmt.Fprintf(env.Stdout, "  review with: mimoneko patch preview %s\n", result.WorktreeID)
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
