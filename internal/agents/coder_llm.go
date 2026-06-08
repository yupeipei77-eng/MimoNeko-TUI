package agents

import (
	"context"
	"fmt"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/contextengine"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/modelrouter"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"
)

// CoderLLM calls an LLM to generate a patch intent.
// It does NOT write files, generate real patches, or execute tools.
type CoderLLM struct {
	router modelrouter.ModelRouter
}

// NewCoderLLM creates a new CoderLLM.
func NewCoderLLM(router modelrouter.ModelRouter) *CoderLLM {
	return &CoderLLM{router: router}
}

// GeneratePatchIntent calls the LLM to generate a patch intent.
// Returns an error if the LLM call fails or the output cannot be parsed.
func (c *CoderLLM) GeneratePatchIntent(ctx context.Context, plan *AgentPlan, bundle contextengine.Bundle) (*CoderPatchIntent, error) {
	// Validate plan
	if plan.ImplementationStatus != ImplementationStatusPlanOnly {
		return nil, fmt.Errorf("coder: plan implementation_status must be %q, got %q", ImplementationStatusPlanOnly, plan.ImplementationStatus)
	}

	// Sanitize inputs for prompt
	sanitizedGoal := security.SanitizeText(plan.Goal)
	sanitizedSummary := security.SanitizeText(plan.Summary)

	// Build prompt with constraints
	prompt := buildCoderPrompt(sanitizedGoal, sanitizedSummary, plan.Steps)

	// Build completion request
	req := modelrouter.CompletionRequest{
		Bundle:          bundle,
		MaxOutputTokens: 4096,
		Temperature:     0.3,
	}

	// Update the current input with the coder prompt
	req.Bundle.CurrentInput = []byte(prompt)

	// Call LLM
	resp, err := c.router.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("coder: LLM call failed: %w", err)
	}

	// Parse response
	intent, err := ParseCoderIntentResponse(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("coder: %w", err)
	}

	// Validate intent safety
	if err := ValidateCoderIntent(intent); err != nil {
		return nil, fmt.Errorf("coder: validation failed: %w", err)
	}

	// Sanitize intent fields
	intent.Goal = security.SanitizeText(intent.Goal)
	intent.PlanSummary = security.SanitizeText(intent.PlanSummary)

	return intent, nil
}

// buildCoderPrompt builds the prompt for the Coder LLM.
func buildCoderPrompt(goal string, planSummary string, steps []PlanStep) string {
	stepsText := ""
	for i, step := range steps {
		stepsText += fmt.Sprintf("%d. [%s] %s\n   %s\n", i+1, step.RiskLevel, step.Title, step.Description)
	}

	return fmt.Sprintf(`You are a coding agent. Your ONLY job is to generate a patch intent.

STRICT CONSTRAINTS:
- You can ONLY generate a patch intent
- You CANNOT write files
- You CANNOT generate real diffs
- You CANNOT apply patches
- You CANNOT run commands
- You CANNOT claim to have modified code
- You MUST output structured JSON
- implementation_status MUST be "intent_only"
- no_file_writes MUST be true

USER GOAL:
%s

PLAN SUMMARY:
%s

PLAN STEPS:
%s

OUTPUT FORMAT (JSON only):
{
  "goal": "the user's goal",
  "plan_summary": "brief summary of the plan",
  "implementation_status": "intent_only",
  "files_to_change": [
    {
      "path": "file.go",
      "change_type": "edit",
      "reason": "why this file needs changes",
      "risk_level": "low|medium|high"
    }
  ],
  "changes": [
    {
      "id": "change_1",
      "file_path": "file.go",
      "description": "what to change",
      "expected_effect": "what this change achieves",
      "safety_notes": "safety considerations"
    }
  ],
  "risks": ["potential risk 1"],
  "validation_suggestions": ["run tests"],
  "no_file_writes": true
}

IMPORTANT:
- implementation_status MUST be "intent_only"
- no_file_writes MUST be true
- Do NOT include any real diffs
- Do NOT include any file modifications
- Do NOT include any tool calls
- Do NOT include any shell commands
- Do NOT include "diff --git" or unified diff format
- Output ONLY the JSON, no other text`, goal, planSummary, stepsText)
}

// RunCoderWithLLM runs the Coder with LLM integration.
func RunCoderWithLLM(ctx context.Context, plan *AgentPlan, coderLLM *CoderLLM, bundle contextengine.Bundle) (*CoderPatchIntent, error) {
	// Validate plan
	if plan.ImplementationStatus != ImplementationStatusPlanOnly {
		return nil, fmt.Errorf("coder: plan implementation_status must be %q, got %q", ImplementationStatusPlanOnly, plan.ImplementationStatus)
	}

	// Generate patch intent
	intent, err := coderLLM.GeneratePatchIntent(ctx, plan, bundle)
	if err != nil {
		return nil, err
	}

	return intent, nil
}
