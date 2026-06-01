package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mimoneko/mimoneko/internal/agents"
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

	// Sanitize goal for display
	sanitizedGoal := security.SanitizeText(goal)

	// Create event emitter (skeleton phase uses noop)
	// Real event emission will be added when integrating with EventStore
	var eventEmitter events.EventEmitter = &events.NoopEventEmitter{}

	// Emit workflow started event
	events.SafeEmit(eventEmitter, context.Background(), events.RunEvent{
		ID:      mustGenerateEventID(),
		RunID:   "skeleton",
		Type:    events.EventWorkflowStarted,
		Source:  "agents",
		Status:  "started",
		Message: fmt.Sprintf("Workflow started: %s", sanitizedGoal),
		Metadata: map[string]string{
			"goal": sanitizedGoal,
		},
	})

	// Run skeleton workflow
	workflow, err := agents.RunWorkflowSkeleton(goal)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	// Emit step events
	for _, step := range workflow.Steps {
		// Step started
		events.SafeEmit(eventEmitter, context.Background(), events.RunEvent{
			ID:      mustGenerateEventID(),
			RunID:   workflow.RunID,
			Type:    events.EventStepStarted,
			Source:  "agents",
			Status:  "started",
			Message: fmt.Sprintf("Step started: %s", step.Role),
			Metadata: map[string]string{
				"workflow_id": workflow.ID,
				"role":        string(step.Role),
			},
		})

		// Step completed
		events.SafeEmit(eventEmitter, context.Background(), events.RunEvent{
			ID:      mustGenerateEventID(),
			RunID:   workflow.RunID,
			Type:    events.EventStepCompleted,
			Source:  "agents",
			Status:  "completed",
			Message: fmt.Sprintf("Step completed: %s", step.Role),
			Metadata: map[string]string{
				"workflow_id": workflow.ID,
				"role":        string(step.Role),
				"status":      string(step.Status),
			},
		})
	}

	// Emit workflow completed event
	events.SafeEmit(eventEmitter, context.Background(), events.RunEvent{
		ID:      mustGenerateEventID(),
		RunID:   workflow.RunID,
		Type:    events.EventWorkflowCompleted,
		Source:  "agents",
		Status:  "completed",
		Message: fmt.Sprintf("Workflow completed: %s", sanitizedGoal),
		Metadata: map[string]string{
			"workflow_id": workflow.ID,
			"goal":        sanitizedGoal,
			"status":      string(workflow.Status),
		},
	})

	// Print summary
	fmt.Fprint(env.Stdout, agents.FormatWorkflowSummary(workflow))

	return 0
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

// mustGenerateEventID generates an event ID or returns a fallback.
func mustGenerateEventID() string {
	id, err := events.GenerateEventID()
	if err != nil {
		return "evt_error"
	}
	return id
}

func init() {
	commands.Register(&AgentsCommand{})
}
