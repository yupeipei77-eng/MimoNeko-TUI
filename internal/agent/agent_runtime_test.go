package agent

import (
	"strings"
	"testing"
)

func TestParseToolCall_ValidToolCall(t *testing.T) {
	text := `I need to read the README file. {"tool_call": {"name": "file_read", "args": {"path": "README.md"}}}`

	tc, err := ParseToolCall(text)
	if err != nil {
		t.Fatalf("ParseToolCall() error = %v", err)
	}
	if tc == nil {
		t.Fatal("ParseToolCall() returned nil, expected ToolCall")
	}
	if tc.Name != "file_read" {
		t.Errorf("Name = %q, want %q", tc.Name, "file_read")
	}
	if tc.Args["path"] != "README.md" {
		t.Errorf("Args[path] = %q, want %q", tc.Args["path"], "README.md")
	}
}

func TestParseToolCall_NoToolCall(t *testing.T) {
	text := "This is just a regular response without any tool calls."

	tc, err := ParseToolCall(text)
	if err != nil {
		t.Fatalf("ParseToolCall() error = %v", err)
	}
	if tc != nil {
		t.Errorf("ParseToolCall() = %+v, want nil", tc)
	}
}

func TestParseToolCall_EmptyName(t *testing.T) {
	text := `{"tool_call": {"name": "", "args": {}}}`

	tc, err := ParseToolCall(text)
	if err != nil {
		t.Fatalf("ParseToolCall() error = %v", err)
	}
	if tc != nil {
		t.Errorf("ParseToolCall() = %+v, want nil for empty name", tc)
	}
}

func TestParseToolCall_NoArgs(t *testing.T) {
	text := `{"tool_call": {"name": "git_diff"}}`

	tc, err := ParseToolCall(text)
	if err != nil {
		t.Fatalf("ParseToolCall() error = %v", err)
	}
	if tc == nil {
		t.Fatal("ParseToolCall() returned nil")
	}
	if tc.Name != "git_diff" {
		t.Errorf("Name = %q, want %q", tc.Name, "git_diff")
	}
	if tc.Args == nil {
		t.Error("Args is nil, should be empty map")
	}
	if len(tc.Args) != 0 {
		t.Errorf("Args = %v, want empty map", tc.Args)
	}
}

func TestParseToolCall_MultipleArgs(t *testing.T) {
	text := `{"tool_call": {"name": "file_write", "args": {"path": "main.go", "content": "package main"}}}`

	tc, err := ParseToolCall(text)
	if err != nil {
		t.Fatalf("ParseToolCall() error = %v", err)
	}
	if tc == nil {
		t.Fatal("ParseToolCall() returned nil")
	}
	if tc.Name != "file_write" {
		t.Errorf("Name = %q, want %q", tc.Name, "file_write")
	}
	if tc.Args["path"] != "main.go" {
		t.Errorf("Args[path] = %q, want %q", tc.Args["path"], "main.go")
	}
	if tc.Args["content"] != "package main" {
		t.Errorf("Args[content] = %q, want %q", tc.Args["content"], "package main")
	}
}

func TestHasToolCall(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "has tool call",
			text: `{"tool_call": {"name": "file_read", "args": {"path": "a.go"}}}`,
			want: true,
		},
		{
			name: "no tool call",
			text: "just plain text",
			want: false,
		},
		{
			name: "tool call embedded in text",
			text: "Let me read that file. {\"tool_call\": {\"name\": \"file_read\", \"args\": {\"path\": \"b.go\"}}}",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasToolCall(tt.text)
			if got != tt.want {
				t.Errorf("HasToolCall() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractModelText(t *testing.T) {
	text := `I need to check the README. {"tool_call": {"name": "file_read", "args": {"path": "README.md"}}} Then continue.`
	modelText := ExtractModelText(text)

	if modelText == "" {
		t.Error("ExtractModelText() returned empty string")
	}
	// Should contain the reasoning text but not the tool_call JSON
	if strings.Contains(modelText, "tool_call") {
		t.Errorf("ExtractModelText() should not contain tool_call, got: %q", modelText)
	}
	if !strings.Contains(modelText, "I need to check the README.") {
		t.Errorf("ExtractModelText() should contain reasoning text, got: %q", modelText)
	}
}

func TestExtractModelText_NoToolCall(t *testing.T) {
	text := "Just plain text without any tool calls."
	modelText := ExtractModelText(text)
	if modelText != text {
		t.Errorf("ExtractModelText() = %q, want %q", modelText, text)
	}
}

func TestAgentStateIsTerminal(t *testing.T) {
	tests := []struct {
		state AgentState
		want  bool
	}{
		{AgentStatePending, false},
		{AgentStateRunning, false},
		{AgentStateWaitingApproval, false},
		{AgentStateSucceeded, true},
		{AgentStateFailed, true},
		{AgentStateCancelled, true},
	}

	for _, tt := range tests {
		got := tt.state.IsTerminal()
		if got != tt.want {
			t.Errorf("AgentState(%q).IsTerminal() = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestGenerateRunID(t *testing.T) {
	id1, err := GenerateRunID()
	if err != nil {
		t.Fatalf("GenerateRunID() error = %v", err)
	}
	if len(id1) == 0 {
		t.Error("GenerateRunID() returned empty ID")
	}
	if id1[:4] != "run_" {
		t.Errorf("GenerateRunID() = %q, want prefix 'run_'", id1)
	}

	id2, err := GenerateRunID()
	if err != nil {
		t.Fatalf("GenerateRunID() error = %v", err)
	}
	if id1 == id2 {
		t.Error("two generated run IDs should be different")
	}
}

func TestGenerateStepID(t *testing.T) {
	id, err := GenerateStepID()
	if err != nil {
		t.Fatalf("GenerateStepID() error = %v", err)
	}
	if len(id) == 0 {
		t.Error("GenerateStepID() returned empty ID")
	}
	if id[:5] != "step_" {
		t.Errorf("GenerateStepID() = %q, want prefix 'step_'", id)
	}
}

func TestParseToolCallRejectsMultipleToolCalls(t *testing.T) {
	text := `I need to do two things. {"tool_call": {"name": "file_read", "args": {"path": "a.go"}}} and {"tool_call": {"name": "file_write", "args": {"path": "b.go", "content": "x"}}}`

	tc, err := ParseToolCall(text)
	if err == nil {
		t.Fatal("ParseToolCall() should return error for multiple tool_call blocks")
	}
	if !strings.Contains(err.Error(), "multiple tool_call") {
		t.Errorf("error should mention multiple tool_call, got: %v", err)
	}
	if tc != nil {
		t.Errorf("ParseToolCall() should return nil ToolCall on error, got: %+v", tc)
	}
}
