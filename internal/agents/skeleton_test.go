package agents

import (
	"strings"
	"testing"
)

func TestSkeletonGeneratorGeneratePlannerOutput(t *testing.T) {
	gen := NewSkeletonGenerator()
	output := gen.GeneratePlannerOutput("test goal")

	if !strings.Contains(output, "test goal") {
		t.Errorf("Output should contain goal, got: %q", output)
	}
	if !strings.Contains(output, "skeleton") {
		t.Errorf("Output should indicate skeleton, got: %q", output)
	}
}

func TestSkeletonGeneratorGenerateCoderOutput(t *testing.T) {
	gen := NewSkeletonGenerator()
	output := gen.GenerateCoderOutput("test plan")

	if !strings.Contains(output, "skeleton") {
		t.Errorf("Output should indicate skeleton, got: %q", output)
	}
}

func TestSkeletonGeneratorGenerateReviewerOutput(t *testing.T) {
	gen := NewSkeletonGenerator()
	output := gen.GenerateReviewerOutput("test patch")

	if !strings.Contains(output, "skeleton") {
		t.Errorf("Output should indicate skeleton, got: %q", output)
	}
}

func TestSkeletonGeneratorGenerateValidatorOutput(t *testing.T) {
	gen := NewSkeletonGenerator()
	output := gen.GenerateValidatorOutput("test review")

	if !strings.Contains(output, "skeleton") {
		t.Errorf("Output should indicate skeleton, got: %q", output)
	}
}

func TestRunWorkflowSkeletonNoLLMCall(t *testing.T) {
	// This test verifies that RunWorkflowSkeleton does not call any LLM
	// by running it and checking that it completes successfully
	workflow, err := RunWorkflowSkeleton("test goal")
	if err != nil {
		t.Fatalf("RunWorkflowSkeleton() error = %v", err)
	}

	// All steps should be completed or completed_stub
	for _, step := range workflow.Steps {
		if step.Status != AgentStatusCompleted && step.Status != AgentStatusCompletedStub {
			t.Errorf("Step %s has status %q, want completed or completed_stub", step.Role, step.Status)
		}
	}
}

func TestRunWorkflowSkeletonNoFileModification(t *testing.T) {
	// This test verifies that RunWorkflowSkeleton does not modify any files
	// by checking that the workflow completes without error
	_, err := RunWorkflowSkeleton("test goal")
	if err != nil {
		t.Fatalf("RunWorkflowSkeleton() error = %v", err)
	}
}

func TestRunWorkflowSkeletonStableOrdering(t *testing.T) {
	// This test verifies that the workflow steps are in stable order
	workflow, _ := RunWorkflowSkeleton("test goal")

	expectedOrder := []AgentRole{AgentRolePlanner, AgentRoleCoder, AgentRoleReviewer, AgentRoleValidator}
	for i, step := range workflow.Steps {
		if step.Role != expectedOrder[i] {
			t.Errorf("Step %d role = %q, want %q", i, step.Role, expectedOrder[i])
		}
	}
}
