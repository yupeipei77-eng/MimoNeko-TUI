package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mimoneko/mimoneko/internal/agents"
	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/events"
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
	fmt.Fprintln(env.Stdout, "  list                    列出可用 agent 角色")
	fmt.Fprintln(env.Stdout, "  plan --goal \"...\"       创建 workflow skeleton")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "示例:")
	fmt.Fprintln(env.Stdout, "  mimoneko agents")
	fmt.Fprintln(env.Stdout, "  mimoneko agents plan --goal \"修复 README 拼写错误\"")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "注意: 当前阶段是 skeleton，不调用 LLM，不修改代码。")
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
	fmt.Fprintln(env.Stdout, "使用 'mimoneko agents plan --goal \"...\"' 创建 workflow skeleton")

	return 0
}

func (c *AgentsCommand) runPlan(args []string, env Env) int {
	goal := extractGoalFromArgs(args)
	if goal == "" {
		fmt.Fprintln(env.Stderr, "用法: mimoneko agents plan --goal \"...\"")
		return 1
	}

	// Create event emitter with EventStore integration
	eventEmitter := c.createEventEmitter(env)

	// Run skeleton workflow
	workflow, err := agents.RunWorkflowSkeleton(goal)
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

	// Print summary
	fmt.Fprint(env.Stdout, agents.FormatWorkflowSummary(workflow))

	return 0
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

// extractGoalFromArgs extracts the goal from --goal argument.
func extractGoalFromArgs(args []string) string {
	for i, arg := range args {
		if arg == "--goal" && i+1 < len(args) {
			return strings.TrimSpace(args[i+1])
		}
	}
	// Also support: mimoneko agents plan "goal"
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		return strings.TrimSpace(args[0])
	}
	return ""
}

func init() {
	commands.Register(&AgentsCommand{})
}
