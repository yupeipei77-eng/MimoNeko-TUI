package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
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
	case "code":
		return c.runCode(args[1:], env)
	case "review":
		return c.runReview(args[1:], env)
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
	fmt.Fprintln(env.Stdout, "  code --goal \"...\" --plan-file <file> [--llm] [--json] 生成 patch intent")
	fmt.Fprintln(env.Stdout, "  review --intent-file <file> [--llm] [--json] 审查 patch intent")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "示例:")
	fmt.Fprintln(env.Stdout, "  mimoneko agents")
	fmt.Fprintln(env.Stdout, "  mimoneko agents plan --goal \"修复 README 拼写错误\"")
	fmt.Fprintln(env.Stdout, "  mimoneko agents plan --goal \"优化 README\" --llm")
	fmt.Fprintln(env.Stdout, "  mimoneko agents code --goal \"优化 README\" --plan-file plan.json --llm")
	fmt.Fprintln(env.Stdout, "  mimoneko agents review --intent-file intent.json --llm")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "注意: --llm 只生成计划/意图/审查，不写文件、不生成 patch、不执行工具。")
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
	fmt.Fprintln(env.Stdout, "使用 'mimoneko agents code --goal \"...\" --plan-file <file>' 生成 patch intent")
	fmt.Fprintln(env.Stdout, "添加 --llm 使用真实 LLM")

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

func (c *AgentsCommand) runCode(args []string, env Env) int {
	fs := flag.NewFlagSet("agents code", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	goal := fs.String("goal", "", "goal description")
	planFile := fs.String("plan-file", "", "path to AgentPlan JSON file")
	useLLM := fs.Bool("llm", false, "use LLM for coder (intent only, no file writes)")
	jsonOutput := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}

	if *goal == "" {
		fmt.Fprintln(env.Stderr, "用法: mimoneko agents code --goal \"...\" --plan-file <file> [--llm] [--json]")
		return 1
	}

	if *planFile == "" {
		fmt.Fprintln(env.Stderr, "错误: --plan-file 是必需的")
		fmt.Fprintln(env.Stderr, "用法: mimoneko agents code --goal \"...\" --plan-file <file> [--llm] [--json]")
		return 1
	}

	// Load plan file
	planData, err := os.ReadFile(*planFile)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: 无法读取 plan 文件: %v\n", err)
		return 1
	}

	// Parse plan
	var plan agents.AgentPlan
	if err := json.Unmarshal(planData, &plan); err != nil {
		fmt.Fprintf(env.Stderr, "错误: plan 文件不是有效的 JSON: %v\n", err)
		return 1
	}

	// Validate plan implementation_status
	if plan.ImplementationStatus != agents.ImplementationStatusPlanOnly {
		fmt.Fprintf(env.Stderr, "错误: plan implementation_status 必须是 %q, 当前是 %q\n", agents.ImplementationStatusPlanOnly, plan.ImplementationStatus)
		return 1
	}

	// Create event emitter with EventStore integration
	eventEmitter := c.createEventEmitter(env)

	// Run coder
	var intent *agents.CoderPatchIntent
	var coderErr error

	if *useLLM {
		// LLM mode
		intent, coderErr = c.runCodeWithLLM(*goal, &plan, eventEmitter, env)
	} else {
		// Skeleton mode
		intent, coderErr = c.runCodeSkeleton(*goal, &plan)
	}

	if coderErr != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", coderErr)
		return 1
	}

	// Output
	if *jsonOutput {
		jsonStr, err := agents.FormatCoderIntentJSON(intent)
		if err != nil {
			fmt.Fprintf(env.Stderr, "错误: %v\n", err)
			return 1
		}
		fmt.Fprintln(env.Stdout, jsonStr)
	} else {
		fmt.Fprint(env.Stdout, agents.FormatCoderIntent(intent))
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

// runCodeSkeleton runs the coder in skeleton mode.
func (c *AgentsCommand) runCodeSkeleton(goal string, plan *agents.AgentPlan) (*agents.CoderPatchIntent, error) {
	// Create skeleton intent
	intent := &agents.CoderPatchIntent{
		Goal:                 security.SanitizeText(goal),
		PlanSummary:          plan.Summary,
		ImplementationStatus: agents.ImplementationStatusIntentOnly,
		FilesToChange: []agents.PatchIntentFile{
			{
				Path:       "placeholder.go",
				ChangeType: "edit",
				Reason:     "skeleton placeholder",
				RiskLevel:  "low",
			},
		},
		Changes: []agents.PatchIntentChange{
			{
				ID:             "change_1",
				FilePath:       "placeholder.go",
				Description:    "skeleton change (no real modification)",
				ExpectedEffect: "placeholder for testing",
				SafetyNotes:    "no file writes",
			},
		},
		Risks:                 []string{"Skeleton intent - actual LLM integration pending"},
		ValidationSuggestions: []string{"Run tests after implementation"},
		NoFileWrites:          true,
	}

	return intent, nil
}

// runCodeWithLLM runs the coder with LLM integration.
func (c *AgentsCommand) runCodeWithLLM(goal string, plan *agents.AgentPlan, eventEmitter events.EventEmitter, env Env) (*agents.CoderPatchIntent, error) {
	// For now, we'll use skeleton mode with a placeholder
	// In production, this would use the actual ModelRouter
	intent, err := c.runCodeSkeleton(goal, plan)
	if err != nil {
		return nil, err
	}

	// Emit events
	wfEmitter := agents.NewWorkflowEventEmitter(eventEmitter)
	ctx := context.Background()

	// Create a temporary workflow for event emission
	workflow := &agents.AgentWorkflow{
		RunID: "coder_run",
		Goal:  security.SanitizeText(goal),
	}

	// Find coder step
	coderStep, _ := workflow.FindStep(agents.AgentRoleCoder)
	if coderStep != nil {
		coderStep.Status = agents.AgentStatusCompleted
		wfEmitter.EmitStepCompleted(ctx, workflow, *coderStep)
	}

	return intent, nil
}

func (c *AgentsCommand) runReview(args []string, env Env) int {
	fs := flag.NewFlagSet("agents review", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	intentFile := fs.String("intent-file", "", "path to CoderPatchIntent JSON file")
	useLLM := fs.Bool("llm", false, "use LLM for reviewer (review only, no file writes)")
	jsonOutput := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}

	if *intentFile == "" {
		fmt.Fprintln(env.Stderr, "错误: --intent-file 是必需的")
		fmt.Fprintln(env.Stderr, "用法: mimoneko agents review --intent-file <file> [--llm] [--json]")
		return 1
	}

	// 读取 intent 文件
	intentData, err := os.ReadFile(*intentFile)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: 无法读取 intent 文件: %v\n", err)
		return 1
	}

	// 解析 intent
	var intent agents.CoderPatchIntent
	if err := json.Unmarshal(intentData, &intent); err != nil {
		fmt.Fprintf(env.Stderr, "错误: intent 文件不是有效的 JSON: %v\n", err)
		return 1
	}

	// 验证 intent
	if intent.ImplementationStatus != agents.ImplementationStatusIntentOnly {
		fmt.Fprintf(env.Stderr, "错误: intent implementation_status 必须是 %q, 当前是 %q\n", agents.ImplementationStatusIntentOnly, intent.ImplementationStatus)
		return 1
	}
	if !intent.NoFileWrites {
		fmt.Fprintln(env.Stderr, "错误: intent no_file_writes 必须是 true")
		return 1
	}

	// 创建 event emitter
	eventEmitter := c.createEventEmitter(env)

	// 运行 reviewer
	var review *agents.ReviewerIntentReview
	var reviewErr error

	if *useLLM {
		// LLM 模式
		review, reviewErr = c.runReviewWithLLM(&intent, eventEmitter, env)
	} else {
		// Skeleton 模式
		review, reviewErr = c.runReviewSkeleton(&intent)
	}

	if reviewErr != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", reviewErr)
		return 1
	}

	// 输出
	if *jsonOutput {
		jsonStr, err := agents.FormatReviewerReviewJSON(review)
		if err != nil {
			fmt.Fprintf(env.Stderr, "错误: %v\n", err)
			return 1
		}
		fmt.Fprintln(env.Stdout, jsonStr)
	} else {
		fmt.Fprint(env.Stdout, agents.FormatReviewerReview(review))
	}

	return 0
}

// runReviewSkeleton 运行 skeleton 模式的 reviewer
func (c *AgentsCommand) runReviewSkeleton(intent *agents.CoderPatchIntent) (*agents.ReviewerIntentReview, error) {
	review := &agents.ReviewerIntentReview{
		Goal:                  security.SanitizeText(intent.Goal),
		ReviewStatus:          agents.ReviewStatusApproved,
		ImplementationStatus:  agents.ImplementationStatusReviewOnly,
		Summary:               "Skeleton review (no real analysis)",
		Approved:              true,
		Issues:                []agents.ReviewIssue{},
		Risks:                 []string{"Skeleton review - actual LLM integration pending"},
		RequiredChanges:       []string{},
		ValidationSuggestions: []string{"Run tests after implementation"},
		NoFileWrites:          true,
		NoPatchGenerated:      true,
	}

	return review, nil
}

// runReviewWithLLM 运行 LLM 模式的 reviewer
func (c *AgentsCommand) runReviewWithLLM(intent *agents.CoderPatchIntent, eventEmitter events.EventEmitter, env Env) (*agents.ReviewerIntentReview, error) {
	// 目前使用 skeleton 模式作为 placeholder
	review, err := c.runReviewSkeleton(intent)
	if err != nil {
		return nil, err
	}

	// 发送事件
	wfEmitter := agents.NewWorkflowEventEmitter(eventEmitter)
	ctx := context.Background()

	// 创建临时 workflow 用于事件发送
	workflow := &agents.AgentWorkflow{
		RunID: "reviewer_run",
		Goal:  security.SanitizeText(intent.Goal),
	}

	// 找到 reviewer step
	reviewerStep, _ := workflow.FindStep(agents.AgentRoleReviewer)
	if reviewerStep != nil {
		reviewerStep.Status = agents.AgentStatusCompleted
		wfEmitter.EmitStepCompleted(ctx, workflow, *reviewerStep)
	}

	return review, nil
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
