package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/reasonforge/reasonforge/internal/contextengine"
	"github.com/reasonforge/reasonforge/internal/modelrouter"
	"github.com/reasonforge/reasonforge/internal/task"
)

// PlannerAgent generates a TaskPlan from a user Goal.
//
// Rules:
//   - Must call ModelRouter (not ToolRuntime)
//   - Cannot modify files
//   - Cannot override TaskContract
//   - Output must be strictly JSON
//   - If output is not valid JSON, returns an error
//   - Only generates a plan; does not execute it
type PlannerAgent struct {
	modelRouter   modelrouter.ModelRouter
	contextEngine contextengine.ContextEngine
	model         string // optional model override
}

// NewPlannerAgent creates a new PlannerAgent.
func NewPlannerAgent(modelRouter modelrouter.ModelRouter, contextEngine contextengine.ContextEngine, model string) *PlannerAgent {
	return &PlannerAgent{
		modelRouter:   modelRouter,
		contextEngine: contextEngine,
		model:         model,
	}
}

// PlanRequest is the input to PlannerAgent.Plan.
type PlanRequest struct {
	Goal     string
	Contract task.TaskContract
	RepoRoot string
}

// PlanResult is the output of PlannerAgent.Plan.
type PlanResult struct {
	Plan    TaskPlan
	Message AgentMessage
}

// Plan generates a TaskPlan by calling the model via ModelRouter.
func (p *PlannerAgent) Plan(ctx context.Context, req PlanRequest) (PlanResult, error) {
	// Build the planner prompt as the current input
	prompt := buildPlannerPrompt(req.Goal, req.Contract)

	// Build context bundle using ContextEngine
	bundle, err := p.contextEngine.Build(ctx, contextengine.BuildRequest{
		TaskID:   req.Contract.ID,
		RepoRoot: req.RepoRoot,
		Budget: contextengine.TokenBudget{
			ImmutablePrefix: 100000,
			Conversation:    50000,
			Scratchpad:      30000,
			Output:          4096,
		},
		CurrentInput: []byte(prompt),
	})
	if err != nil {
		return PlanResult{}, fmt.Errorf("planner: context build failed: %w", err)
	}

	completionReq := modelrouter.CompletionRequest{
		Bundle:          bundle,
		MaxOutputTokens: 4096,
	}
	if p.model != "" {
		completionReq.Model = p.model
	}

	resp, err := p.modelRouter.Complete(ctx, completionReq)
	if err != nil {
		return PlanResult{}, fmt.Errorf("planner: model call failed: %w", err)
	}

	// Parse the model output as strict JSON
	plan, err := parseTaskPlanJSON(resp.Text)
	if err != nil {
		return PlanResult{}, fmt.Errorf("planner: invalid plan output: %w", err)
	}

	// Enforce: planner cannot override TaskContract - always use the original goal
	plan.Goal = req.Goal

	msg := AgentMessage{
		Role:      AgentRolePlanner,
		Content:   fmt.Sprintf("Plan generated with %d steps, risk_level=%s", len(plan.Steps), plan.RiskLevel),
		Metadata:  map[string]string{"step_count": fmt.Sprintf("%d", len(plan.Steps)), "risk_level": plan.RiskLevel},
		CreatedAt: time.Now().UTC(),
	}

	return PlanResult{
		Plan:    plan,
		Message: msg,
	}, nil
}

// buildPlannerPrompt constructs the system prompt for the planner.
func buildPlannerPrompt(goal string, contract task.TaskContract) string {
	var sb strings.Builder
	sb.WriteString("You are a planning agent. Your job is to break down a coding goal into a concrete execution plan.\n")
	sb.WriteString("\n")
	sb.WriteString("IMPORTANT RULES:\n")
	sb.WriteString("1. You must respond with ONLY valid JSON. No markdown, no explanation outside JSON.\n")
	sb.WriteString("2. You cannot modify files. You only generate a plan.\n")
	sb.WriteString("3. You cannot override the task contract.\n")
	sb.WriteString("4. Do not suggest tools or commands. Only describe what needs to be done.\n")
	sb.WriteString("\n")
	sb.WriteString("Respond with the following JSON format:\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"goal\": \"string - the goal\",\n")
	sb.WriteString("  \"steps\": [\n")
	sb.WriteString("    {\n")
	sb.WriteString("      \"index\": 0,\n")
	sb.WriteString("      \"title\": \"string - step title\",\n")
	sb.WriteString("      \"description\": \"string - what to do\",\n")
	sb.WriteString("      \"target_paths\": [\"path/to/file\"],\n")
	sb.WriteString("      \"expected_outcome\": \"string - what should happen\"\n")
	sb.WriteString("    }\n")
	sb.WriteString("  ],\n")
	sb.WriteString("  \"risk_level\": \"low|medium|high\",\n")
	sb.WriteString("  \"notes\": \"string - additional notes\"\n")
	sb.WriteString("}\n")
	sb.WriteString("\n")
	sb.WriteString("Goal: ")
	sb.WriteString(goal)
	sb.WriteString("\n")

	if len(contract.AllowedPaths) > 0 {
		sb.WriteString("Allowed paths: ")
		sb.WriteString(strings.Join(contract.AllowedPaths, ", "))
		sb.WriteString("\n")
	}
	if len(contract.DeniedPaths) > 0 {
		sb.WriteString("Denied paths: ")
		sb.WriteString(strings.Join(contract.DeniedPaths, ", "))
		sb.WriteString("\n")
	}

	return sb.String()
}

// parseTaskPlanJSON parses a JSON string into a TaskPlan.
// It requires strictly valid JSON - markdown fences are stripped if present.
func parseTaskPlanJSON(text string) (TaskPlan, error) {
	// Strip markdown code fences if present (common model behavior)
	cleaned := strings.TrimSpace(text)
	if strings.HasPrefix(cleaned, "```") {
		// Remove opening fence
		if idx := strings.Index(cleaned[3:], "\n"); idx >= 0 {
			cleaned = cleaned[3+idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(cleaned, "```"); idx >= 0 {
			cleaned = cleaned[:idx]
		}
		cleaned = strings.TrimSpace(cleaned)
	}

	var plan TaskPlan
	if err := json.Unmarshal([]byte(cleaned), &plan); err != nil {
		return TaskPlan{}, fmt.Errorf("planner output is not valid JSON: %w", err)
	}

	// Validate required fields
	if strings.TrimSpace(plan.Goal) == "" {
		return TaskPlan{}, fmt.Errorf("planner output missing required field: goal")
	}
	if len(plan.Steps) == 0 {
		return TaskPlan{}, fmt.Errorf("planner output missing required field: steps (must have at least one)")
	}
	if plan.RiskLevel != "low" && plan.RiskLevel != "medium" && plan.RiskLevel != "high" {
		return TaskPlan{}, fmt.Errorf("planner output has invalid risk_level %q (must be low, medium, or high)", plan.RiskLevel)
	}

	// Assign indices if missing
	for i := range plan.Steps {
		if plan.Steps[i].Index == 0 {
			plan.Steps[i].Index = i
		}
	}

	return plan, nil
}
