package cli

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mimoneko/mimoneko/internal/agents"
	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/events"
	"github.com/mimoneko/mimoneko/internal/security"
)

type AgentsCommand struct{}

func (c *AgentsCommand) Name() string { return "agents" }

func (c *AgentsCommand) Run(args []string, env Env) int {
	if len(args) == 0 {
		return c.runList(args, env)
	}

	switch args[0] {
	case "plan":
		return c.runPlan(args[1:], env)
	case "list":
		return c.runList(args[1:], env)
	default:
		fmt.Fprintf(env.Stderr, "未知命令 '%s'\n\n", args[0])
		printAgentsHelp(env)
		return 1
	}
}

func printAgentsHelp(env Env) {
	fmt.Fprintln(env.Stdout, "用法: mimoneko agents <命令>")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "命令:")
	fmt.Fprintln(env.Stdout, "  list                              列出可用 agent 角色")
	fmt.Fprintln(env.Stdout, "  plan --goal \"...\" [--llm] [--json] 创建 workflow plan")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "示例:")
	fmt.Fprintln(env.Stdout, "  mimoneko agents")
	fmt.Fprintln(env.Stdout, "  mimoneko agents plan --goal \"修复 README 拼写错误\"")
	fmt.Fprintln(env.Stdout, "  mimoneko agents plan --goal \"优化 README\" --llm")
	fmt.Fprintln(env.Stdout, "  mimoneko agents plan --goal \"优化 README\" --llm --json")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "注意: --llm 只生成计划，不写文件、不生成 patch、不执行工具。")
}

func (c *AgentsCommand) runList(args []string, env Env) int {
	fmt.Fprintln(env.Stdout, "Multi-Agent Roles")
	fmt.Fprintln(env.Stdout, "=================")
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "当前阶段: skeleton (不调用 LLM，不修改代码)")
	fmt.Fprintln(env.Stdout)

	for _, role := range agents.AllAgentRoles() {
		fmt.Fprintf(env.Stdout, "  %-12s %s\n", role, agents.RoleDescription(role))
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "使用 'mimoneko agents plan --goal \"...\"' 创建 workflow plan")
	fmt.Fprintln(env.Stdout, "添加 --llm 使用真实 LLM 生成计划")

	return 0
}

func (c *AgentsCommand) runPlan(args []string, env Env) int {
	fs := flag.NewFlagSet("agents plan", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	goal := fs.String("goal", "", "goal description")
	useLLM := fs.Bool("llm", false, "use LLM for planner (plan only, no file writes)")
	jsonOutput := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}

	if *goal == "" {
		// Try positional argument
		if fs.NArg() > 0 {
			*goal = strings.TrimSpace(fs.Arg(0))
		}
	}

	if *goal == "" {
		fmt.Fprintln(env.Stderr, "用法: mimoneko agents plan --goal \"...\" [--llm] [--json]")
		return 1
	}

	// Create event emitter with EventStore integration
	eventEmitter := c.createEventEmitter(env)

	// Run workflow
	var workflow *agents.AgentWorkflow
	var agentPlan *agents.AgentPlan
	var err error

	if *useLLM {
		// LLM mode - use Planner LLM
		workflow, agentPlan, err = c.runPlanWithLLM(*goal, eventEmitter, env)
	} else {
		// Skeleton mode
		workflow, err = agents.RunWorkflowSkeleton(*goal)
	}

	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	// Emit events using WorkflowEventEmitter
	wfEmitter := agents.NewWorkflowEventEmitter(eventEmitter)
	ctx := context.Background()

	wfEmitter.EmitWorkflowStarted(ctx, workflow)

	for _, step := range workflow.Steps {
		wfEmitter.EmitStepStarted(ctx, workflow, step.Role)
		wfEmitter.EmitStepCompleted(ctx, workflow, step)
	}

	wfEmitter.EmitWorkflowCompleted(ctx, workflow)

	// Output
	if *jsonOutput && agentPlan != nil {
		jsonStr, err := agents.FormatPlanJSON(agentPlan)
		if err != nil {
			fmt.Fprintf(env.Stderr, "错误: %v\n", err)
			return 1
		}
		fmt.Fprintln(env.Stdout, jsonStr)
	} else if agentPlan != nil {
		fmt.Fprint(env.Stdout, agents.FormatPlan(agentPlan))
	} else {
		fmt.Fprint(env.Stdout, agents.FormatWorkflowSummary(workflow))
	}

	return 0
}

// runPlanWithLLM runs the planner with LLM integration.
func (c *AgentsCommand) runPlanWithLLM(goal string, eventEmitter events.EventEmitter, env Env) (*agents.AgentWorkflow, *agents.AgentPlan, error) {
	// For now, we'll use skeleton mode with a placeholder
	// In production, this would use the actual ModelRouter
	workflow, err := agents.RunWorkflowSkeleton(goal)
	if err != nil {
		return nil, nil, err
	}

	// Create a placeholder plan
	plan := &agents.AgentPlan{
		Goal:    security.SanitizeText(goal),
		Summary: "LLM-generated plan (placeholder - actual LLM integration pending)",
		Steps: []agents.PlanStep{
			{
				ID:             "step_1",
				Title:          "Analyze goal",
				Description:    "Analyze the user goal and identify key requirements",
				RiskLevel:      "low",
				ExpectedFiles:  []string{},
				ValidationHint: "Verify understanding of requirements",
			},
		},
		Risks:                 []string{"Placeholder plan - actual LLM integration pending"},
		FilesMaybeAffected:    []string{},
		ValidationSuggestions: []string{"Run tests after implementation"},
		ImplementationStatus:  agents.ImplementationStatusPlanOnly,
	}

	return workflow, plan, nil
}

// createEventEmitter creates an EventEmitter with EventStore integration.
// Falls back to NoopEventEmitter if EventStore is unavailable.
func (c *AgentsCommand) createEventEmitter(env Env) events.EventEmitter {
	root, err := resolveRoot("", env)
	if err != nil {
		return &events.NoopEventEmitter{}
	}

	cfg, err := config.Load(root)
	if err != nil {
		return &events.NoopEventEmitter{}
	}

	if !cfg.Events.Enabled {
		return &events.NoopEventEmitter{}
	}

	eventStorePath := cfg.Events.StorePath
	if !filepath.IsAbs(eventStorePath) {
		eventStorePath = filepath.Join(root, eventStorePath)
	}

	store, err := events.NewJSONLRunEventStore(eventStorePath)
	if err != nil {
		return &events.NoopEventEmitter{}
	}

	return events.NewEventEmitterFromStore(store)
}

func init() {
	commands.Register(&AgentsCommand{})
}
