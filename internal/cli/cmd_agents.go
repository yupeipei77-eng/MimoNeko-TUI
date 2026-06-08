package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/agents"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/contextengine"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/events"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"
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
	case "validate":
		return c.runValidate(args[1:], env)
	case "run":
		return c.runAgentsRun(args[1:], env)
	case "reports":
		return c.runReports(args[1:], env)
	case "report":
		return c.runReport(args[1:], env)
	case "patch-preview":
		return c.runPatchPreview(args[1:], env)
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
	fmt.Fprintln(env.Stdout, "  validate --review-file <file> --intent-file <file> [--llm] [--json] 生成验证建议")
	fmt.Fprintln(env.Stdout, "  run --goal \"...\" --dry-run [--llm] [--json] [--save-report] 端到端 dry-run")
	fmt.Fprintln(env.Stdout, "  reports                           列出最近 dry-run reports")
	fmt.Fprintln(env.Stdout, "  report <workflow_id> [--json]      显示指定 report")
	fmt.Fprintln(env.Stdout, "  patch-preview --intent-file <file> [--json] 预览 patch")
	fmt.Fprintln(env.Stdout, "  patch-preview --report <id> [--json] 从 report 预览 patch")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "示例:")
	fmt.Fprintln(env.Stdout, "  mimoneko agents")
	fmt.Fprintln(env.Stdout, "  mimoneko agents plan --goal \"修复 README 拼写错误\"")
	fmt.Fprintln(env.Stdout, "  mimoneko agents plan --goal \"优化 README\" --llm")
	fmt.Fprintln(env.Stdout, "  mimoneko agents code --goal \"优化 README\" --plan-file plan.json --llm")
	fmt.Fprintln(env.Stdout, "  mimoneko agents review --intent-file intent.json --llm")
	fmt.Fprintln(env.Stdout, "  mimoneko agents validate --review-file review.json --intent-file intent.json --llm")
	fmt.Fprintln(env.Stdout, "  mimoneko agents run --goal \"优化 README\" --dry-run --llm --save-report")
	fmt.Fprintln(env.Stdout, "  mimoneko agents reports")
	fmt.Fprintln(env.Stdout, "  mimoneko agents report <workflow_id>")
	fmt.Fprintln(env.Stdout, "  mimoneko agents patch-preview --intent-file intent.json")
	fmt.Fprintln(env.Stdout, "  mimoneko agents patch-preview --report <workflow_id>")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "注意: --dry-run 必须显式开启，不写文件、不执行测试、不执行工具。")
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

func (c *AgentsCommand) runValidate(args []string, env Env) int {
	fs := flag.NewFlagSet("agents validate", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	reviewFile := fs.String("review-file", "", "path to ReviewerIntentReview JSON file")
	intentFile := fs.String("intent-file", "", "path to CoderPatchIntent JSON file")
	useLLM := fs.Bool("llm", false, "use LLM for validator (suggestions only, no tests executed)")
	jsonOutput := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}

	if *reviewFile == "" {
		fmt.Fprintln(env.Stderr, "错误: --review-file 是必需的")
		fmt.Fprintln(env.Stderr, "用法: mimoneko agents validate --review-file <file> --intent-file <file> [--llm] [--json]")
		return 1
	}

	if *intentFile == "" {
		fmt.Fprintln(env.Stderr, "错误: --intent-file 是必需的")
		fmt.Fprintln(env.Stderr, "用法: mimoneko agents validate --review-file <file> --intent-file <file> [--llm] [--json]")
		return 1
	}

	// 读取 review 文件
	reviewData, err := os.ReadFile(*reviewFile)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: 无法读取 review 文件: %v\n", err)
		return 1
	}

	// 解析 review
	var review agents.ReviewerIntentReview
	if err := json.Unmarshal(reviewData, &review); err != nil {
		fmt.Fprintf(env.Stderr, "错误: review 文件不是有效的 JSON: %v\n", err)
		return 1
	}

	// 验证 review
	if review.ImplementationStatus != agents.ImplementationStatusReviewOnly {
		fmt.Fprintf(env.Stderr, "错误: review implementation_status 必须是 %q, 当前是 %q\n", agents.ImplementationStatusReviewOnly, review.ImplementationStatus)
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

	// 运行 validator
	var suggestions *agents.ValidatorSuggestions
	var validateErr error

	if *useLLM {
		// LLM 模式
		suggestions, validateErr = c.runValidateWithLLM(&intent, &review, eventEmitter, env)
	} else {
		// Skeleton 模式
		suggestions, validateErr = c.runValidateSkeleton(&intent, &review)
	}

	if validateErr != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", validateErr)
		return 1
	}

	// 输出
	if *jsonOutput {
		jsonStr, err := agents.FormatValidatorSuggestionsJSON(suggestions)
		if err != nil {
			fmt.Fprintf(env.Stderr, "错误: %v\n", err)
			return 1
		}
		fmt.Fprintln(env.Stdout, jsonStr)
	} else {
		fmt.Fprint(env.Stdout, agents.FormatValidatorSuggestions(suggestions))
	}

	return 0
}

// runValidateSkeleton 运行 skeleton 模式的 validator
func (c *AgentsCommand) runValidateSkeleton(intent *agents.CoderPatchIntent, review *agents.ReviewerIntentReview) (*agents.ValidatorSuggestions, error) {
	suggestions := &agents.ValidatorSuggestions{
		Goal:                 security.SanitizeText(intent.Goal),
		ValidationStatus:     "pending",
		ImplementationStatus: agents.ImplementationStatusSuggestionsOnly,
		Summary:              "Skeleton validation suggestions (no real analysis)",
		Checks: func() []agents.ValidationCheck {
			files := []string{}
			if len(intent.FilesToChange) > 0 {
				files = []string{intent.FilesToChange[0].Path}
			}
			return []agents.ValidationCheck{
				{
					ID:             "check_1",
					Category:       "unit_test",
					Description:    "Run unit tests",
					ExpectedSignal: "All tests pass",
					Priority:       "high",
					RelatedFiles:   files,
				},
			}
		}(),
		Risks:               []string{"Skeleton validation - actual LLM integration pending"},
		RecommendedCommands: []string{"go test ./...", "go vet ./..."},
		ManualChecks:        []string{"Check README examples match CLI behavior"},
		NoFileWrites:        true,
		NoTestsExecuted:     true,
		NoToolsExecuted:     true,
	}

	return suggestions, nil
}

// runValidateWithLLM 运行 LLM 模式的 validator
func (c *AgentsCommand) runValidateWithLLM(intent *agents.CoderPatchIntent, review *agents.ReviewerIntentReview, eventEmitter events.EventEmitter, env Env) (*agents.ValidatorSuggestions, error) {
	// 目前使用 skeleton 模式作为 placeholder
	suggestions, err := c.runValidateSkeleton(intent, review)
	if err != nil {
		return nil, err
	}

	// 发送事件
	wfEmitter := agents.NewWorkflowEventEmitter(eventEmitter)
	ctx := context.Background()

	// 创建临时 workflow 用于事件发送
	workflow := &agents.AgentWorkflow{
		RunID: "validator_run",
		Goal:  security.SanitizeText(intent.Goal),
	}

	// 找到 validator step
	validatorStep, _ := workflow.FindStep(agents.AgentRoleValidator)
	if validatorStep != nil {
		validatorStep.Status = agents.AgentStatusCompleted
		wfEmitter.EmitStepCompleted(ctx, workflow, *validatorStep)
	}

	return suggestions, nil
}

func (c *AgentsCommand) runAgentsRun(args []string, env Env) int {
	fs := flag.NewFlagSet("agents run", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	goal := fs.String("goal", "", "goal description")
	useLLM := fs.Bool("llm", false, "use LLM for all agents")
	dryRun := fs.Bool("dry-run", false, "dry-run mode (required)")
	jsonOutput := fs.Bool("json", false, "output as JSON")
	saveReport := fs.Bool("save-report", false, "save report to .mimoneko/agent_runs/")
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
		fmt.Fprintln(env.Stderr, "用法: mimoneko agents run --goal \"...\" --dry-run [--llm] [--json] [--save-report]")
		return 1
	}

	if !*dryRun {
		fmt.Fprintln(env.Stderr, "错误: agents run 当前需要 --dry-run 参数")
		fmt.Fprintln(env.Stderr, "用法: mimoneko agents run --goal \"...\" --dry-run [--llm] [--json] [--save-report]")
		return 1
	}

	// 创建 event emitter
	eventEmitter := c.createEventEmitter(env)

	// 运行 dry-run
	var report *agents.AgentDryRunReport
	var err error

	if *useLLM {
		// LLM 模式
		report, err = c.runDryRunWithLLM(*goal, eventEmitter, env)
	} else {
		// Skeleton 模式
		report, err = c.runDryRunSkeleton(*goal, eventEmitter)
	}

	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	// 保存报告（如果请求）
	if *saveReport {
		root, rootErr := resolveRoot("", env)
		if rootErr == nil {
			reportStore := agents.NewReportStore(root)
			if saveErr := reportStore.Save(report); saveErr != nil {
				fmt.Fprintf(env.Stderr, "警告: 无法保存报告: %v\n", saveErr)
			} else {
				fmt.Fprintf(env.Stdout, "\n报告已保存到 .mimoneko/agent_runs/%s.json\n", report.WorkflowID)
			}
		}
	}

	// 输出
	if *jsonOutput {
		jsonStr, err := agents.FormatDryRunReportJSON(report)
		if err != nil {
			fmt.Fprintf(env.Stderr, "错误: %v\n", err)
			return 1
		}
		fmt.Fprintln(env.Stdout, jsonStr)
	} else {
		fmt.Fprint(env.Stdout, agents.FormatDryRunReport(report))
	}

	return 0
}

// runDryRunSkeleton 运行 skeleton dry-run
func (c *AgentsCommand) runDryRunSkeleton(goal string, eventEmitter events.EventEmitter) (*agents.AgentDryRunReport, error) {
	runner := agents.NewWorkflowRunner(nil)
	report, err := runner.RunSkeletonDryRun(goal)
	if err != nil {
		return nil, err
	}

	// 发送事件
	c.emitDryRunEvents(eventEmitter, report)

	return report, nil
}

// runDryRunWithLLM 运行 LLM dry-run
func (c *AgentsCommand) runDryRunWithLLM(goal string, eventEmitter events.EventEmitter, env Env) (*agents.AgentDryRunReport, error) {
	// 加载配置
	root, err := resolveRoot("", env)
	if err != nil {
		return nil, fmt.Errorf("无法解析项目根目录: %w", err)
	}

	cfg, err := config.Load(root)
	if err != nil {
		return nil, fmt.Errorf("无法加载配置: %w", err)
	}

	// 构建 ModelRouter
	modelRouter := BuildModelRouterFromConfig(cfg)
	if modelRouter == nil {
		return nil, fmt.Errorf("模型未配置: 请运行 'mimoneko auth login' 或检查 .mimoneko/models.yaml")
	}

	// 获取 provider/model 信息
	provider, model := BuildProviderModelInfo(cfg)
	if provider == "" || model == "" {
		return nil, fmt.Errorf("模型配置不完整: provider=%q model=%q", provider, model)
	}

	// 检查 API key 是否配置
	apiKeyEnv := ""
	for _, p := range cfg.Models.Providers {
		if p.Name == provider {
			apiKeyEnv = p.APIKeyEnv
			break
		}
	}
	if apiKeyEnv != "" {
		apiKey := security.GetEnvOrDefault(apiKeyEnv, "")
		if apiKey == "" {
			return nil, fmt.Errorf("API key 未配置: 请设置环境变量 %s", apiKeyEnv)
		}
	}

	// 创建 WorkflowRunner
	runner := agents.NewWorkflowRunner(modelRouter)

	// 构建 context bundle
	// 对于 dry-run，我们使用最小化的 bundle
	bundle := contextengine.Bundle{}

	// 运行 dry-run
	report, err := runner.RunDryRun(context.Background(), goal, bundle, provider, model)
	if err != nil {
		return nil, err
	}

	// 发送事件
	c.emitDryRunEvents(eventEmitter, report)

	return report, nil
}

// emitDryRunEvents 发送 dry-run 事件
func (c *AgentsCommand) emitDryRunEvents(eventEmitter events.EventEmitter, report *agents.AgentDryRunReport) {
	wfEmitter := agents.NewWorkflowEventEmitter(eventEmitter)
	ctx := context.Background()

	// 创建临时 workflow 用于事件发送
	workflow := &agents.AgentWorkflow{
		RunID: report.RunID,
		Goal:  report.Goal,
	}

	// 发送 workflow started
	wfEmitter.EmitWorkflowStarted(ctx, workflow)

	// 发送 planner 事件
	if report.PlannerPlan != nil {
		plannerStep, _ := workflow.FindStep(agents.AgentRolePlanner)
		if plannerStep != nil {
			plannerStep.Status = agents.AgentStatusCompleted
			wfEmitter.EmitStepCompleted(ctx, workflow, *plannerStep)
		}
	}

	// 发送 coder 事件
	if report.CoderIntent != nil {
		coderStep, _ := workflow.FindStep(agents.AgentRoleCoder)
		if coderStep != nil {
			coderStep.Status = agents.AgentStatusCompleted
			wfEmitter.EmitStepCompleted(ctx, workflow, *coderStep)
		}
	}

	// 发送 reviewer 事件
	if report.ReviewerReview != nil {
		reviewerStep, _ := workflow.FindStep(agents.AgentRoleReviewer)
		if reviewerStep != nil {
			reviewerStep.Status = agents.AgentStatusCompleted
			wfEmitter.EmitStepCompleted(ctx, workflow, *reviewerStep)
		}
	}

	// 发送 validator 事件
	if report.ValidatorSuggestions != nil {
		validatorStep, _ := workflow.FindStep(agents.AgentRoleValidator)
		if validatorStep != nil {
			validatorStep.Status = agents.AgentStatusCompleted
			wfEmitter.EmitStepCompleted(ctx, workflow, *validatorStep)
		}
	}

	// 发送 workflow completed
	if report.Status == agents.WorkflowStatusCompleted {
		workflow.Status = agents.AgentStatusCompleted
		wfEmitter.EmitWorkflowCompleted(ctx, workflow)
	} else {
		workflow.Status = agents.AgentStatusFailed
		wfEmitter.EmitWorkflowFailed(ctx, workflow, fmt.Errorf("%s", report.ErrorMessage))
	}
}

func (c *AgentsCommand) runReports(args []string, env Env) int {
	root, err := resolveRoot("", env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	store := agents.NewReportStore(root)
	reports, err := store.List()
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	if len(reports) == 0 {
		fmt.Fprintln(env.Stdout, "no agent dry-run reports")
		return 0
	}

	fmt.Fprintln(env.Stdout, "Agent Dry-Run Reports")
	fmt.Fprintf(env.Stdout, "%-36s %-20s %-12s %-12s %s\n", "WORKFLOW_ID", "CREATED", "STATUS", "PROVIDER", "GOAL")
	for _, r := range reports {
		fmt.Fprintf(env.Stdout, "%-36s %-20s %-12s %-12s %s\n",
			r.WorkflowID,
			r.CreatedAt.Format("2006-01-02 15:04:05"),
			r.Status,
			r.Provider,
			security.SanitizeText(truncateString(r.Goal, 40)),
		)
	}
	return 0
}

func (c *AgentsCommand) runReport(args []string, env Env) int {
	fs := flag.NewFlagSet("agents report", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	jsonOutput := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(env.Stderr, "用法: mimoneko agents report <workflow_id> [--json]")
		return 1
	}

	workflowID := fs.Arg(0)
	root, err := resolveRoot("", env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	store := agents.NewReportStore(root)
	report, err := store.Load(workflowID)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	if *jsonOutput {
		jsonStr, err := agents.FormatDryRunReportJSON(report)
		if err != nil {
			fmt.Fprintf(env.Stderr, "错误: %v\n", err)
			return 1
		}
		fmt.Fprintln(env.Stdout, jsonStr)
	} else {
		fmt.Fprint(env.Stdout, agents.FormatDryRunReport(report))
	}

	return 0
}

func (c *AgentsCommand) runPatchPreview(args []string, env Env) int {
	fs := flag.NewFlagSet("agents patch-preview", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	intentFile := fs.String("intent-file", "", "path to CoderPatchIntent JSON file")
	reportID := fs.String("report", "", "workflow ID of dry-run report")
	jsonOutput := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return flagExitCode(err)
	}

	if *intentFile == "" && *reportID == "" {
		fmt.Fprintln(env.Stderr, "错误: 需要 --intent-file 或 --report 参数")
		fmt.Fprintln(env.Stderr, "用法: mimoneko agents patch-preview --intent-file <file> [--json]")
		fmt.Fprintln(env.Stderr, "      mimoneko agents patch-preview --report <workflow_id> [--json]")
		return 1
	}

	var preview *agents.PatchPreview
	var err error

	if *intentFile != "" {
		// 从 intent 文件生成 preview
		intentData, readErr := os.ReadFile(*intentFile)
		if readErr != nil {
			fmt.Fprintf(env.Stderr, "错误: 无法读取 intent 文件: %v\n", readErr)
			return 1
		}

		var intent agents.CoderPatchIntent
		if jsonErr := json.Unmarshal(intentData, &intent); jsonErr != nil {
			fmt.Fprintf(env.Stderr, "错误: intent 文件不是有效的 JSON: %v\n", jsonErr)
			return 1
		}

		preview, err = agents.GeneratePreviewFromIntent(&intent)
	} else {
		// 从 report 生成 preview
		root, rootErr := resolveRoot("", env)
		if rootErr != nil {
			fmt.Fprintf(env.Stderr, "错误: %v\n", rootErr)
			return 1
		}

		store := agents.NewReportStore(root)
		report, loadErr := store.Load(*reportID)
		if loadErr != nil {
			fmt.Fprintf(env.Stderr, "错误: %v\n", loadErr)
			return 1
		}

		preview, err = agents.GeneratePreviewFromReport(report)
	}

	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	// 输出
	if *jsonOutput {
		jsonStr, jsonErr := agents.FormatPatchPreviewJSON(preview)
		if jsonErr != nil {
			fmt.Fprintf(env.Stderr, "错误: %v\n", jsonErr)
			return 1
		}
		fmt.Fprintln(env.Stdout, jsonStr)
	} else {
		fmt.Fprint(env.Stdout, agents.FormatPatchPreview(preview))
	}

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

func init() {
	commands.Register(&AgentsCommand{})
}
