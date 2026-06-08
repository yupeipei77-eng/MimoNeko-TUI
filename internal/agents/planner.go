package agents

import (
	"context"
	"fmt"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/contextengine"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/modelrouter"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"
)

// PlannerLLM calls an LLM to generate a structured plan.
// It does NOT write files, generate patches, or execute tools.
type PlannerLLM struct {
	router modelrouter.ModelRouter
}

// NewPlannerLLM creates a new PlannerLLM.
func NewPlannerLLM(router modelrouter.ModelRouter) *PlannerLLM {
	return &PlannerLLM{router: router}
}

// GeneratePlan calls the LLM to generate a structured plan.
// Returns an error if the LLM call fails or the output cannot be parsed.
func (p *PlannerLLM) GeneratePlan(ctx context.Context, goal string, bundle contextengine.Bundle) (*AgentPlan, error) {
	// Sanitize goal for prompt
	sanitizedGoal := security.SanitizeText(goal)

	// Build prompt with constraints
	prompt := buildPlannerPrompt(sanitizedGoal)

	// Build completion request
	req := modelrouter.CompletionRequest{
		Bundle:          bundle,
		MaxOutputTokens: 4096,
		Temperature:     0.3,
	}

	// Update the current input with the planner prompt
	req.Bundle.CurrentInput = []byte(prompt)

	// Call LLM
	resp, err := p.router.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("planner: LLM call failed: %w", err)
	}

	// Parse response
	plan, err := ParsePlanResponse(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("planner: %w", err)
	}

	// Sanitize plan fields
	plan.Goal = security.SanitizeText(plan.Goal)
	plan.Summary = security.SanitizeText(plan.Summary)

	// Add context info if available
	if bundle.CacheFingerprint.SHA256 != "" {
		plan.PrefixFingerprint = bundle.CacheFingerprint.SHA256
	}
	plan.ContextBytes = bundle.Report.TotalTokens * 4 // rough estimate

	return plan, nil
}

// buildPlannerPrompt builds the prompt for the Planner LLM.
func buildPlannerPrompt(goal string) string {
	return fmt.Sprintf(`You are a planning agent. Your ONLY job is to create a structured plan.

STRICT CONSTRAINTS:
- You can ONLY create a plan
- You CANNOT write code
- You CANNOT generate patches
- You CANNOT request immediate command execution
- You CANNOT claim to have modified files
- You MUST output structured JSON

USER GOAL:
%s

OUTPUT FORMAT (JSON only):
{
  "goal": "the user's goal",
  "summary": "brief summary of the plan",
  "steps": [
    {
      "id": "step_1",
      "title": "step title",
      "description": "what to do",
      "risk_level": "low|medium|high",
      "expected_files": ["file1.go"],
      "validation_hint": "how to verify"
    }
  ],
  "risks": ["potential risk 1"],
  "files_maybe_affected": ["file1.go"],
  "validation_suggestions": ["run tests"],
  "implementation_status": "plan_only"
}

IMPORTANT:
- implementation_status MUST be "plan_only"
- Do NOT include any file modifications
- Do NOT include any tool calls
- Do NOT include any shell commands
- Output ONLY the JSON, no other text`, goal)
}
