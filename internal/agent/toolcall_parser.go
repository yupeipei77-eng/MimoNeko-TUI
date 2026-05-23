package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// toolCallPattern matches a JSON tool_call block in model output.
// Expected format:
//
//	{"tool_call": {"name": "tool_name", "args": {"key": "value"}}}
//
// The parser is intentionally simple: it finds the first valid JSON object
// containing a "tool_call" key. This is a minimal Phase 4 implementation;
// future phases may support structured function calling APIs.
var toolCallRegex = regexp.MustCompile(`\{[^{}]*"tool_call"\s*:\s*\{[^{}]*\}[^{}]*\}`)

// ParseToolCall attempts to extract a ToolCall from model output text.
// Returns nil if no tool call is found.
//
// The model output is expected to contain a JSON block like:
//
//	{"tool_call": {"name": "file_read", "args": {"path": "README.md"}}}
//
// If no tool_call block is found, the model's output is treated as
// plain text (no tool invocation).
func ParseToolCall(text string) (*ToolCall, error) {
	// Find the first JSON object containing "tool_call"
	match := toolCallRegex.FindString(text)
	if match == "" {
		return nil, nil
	}

	var wrapper struct {
		ToolCall struct {
			Name string            `json:"name"`
			Args map[string]string `json:"args"`
		} `json:"tool_call"`
	}

	if err := json.Unmarshal([]byte(match), &wrapper); err != nil {
		return nil, fmt.Errorf("agent: parse tool_call JSON: %w", err)
	}

	name := strings.TrimSpace(wrapper.ToolCall.Name)
	if name == "" {
		return nil, nil
	}

	// Ensure args is non-nil
	args := wrapper.ToolCall.Args
	if args == nil {
		args = make(map[string]string)
	}

	return &ToolCall{
		Name: name,
		Args: args,
	}, nil
}

// HasToolCall checks whether the model output text contains a tool_call block.
func HasToolCall(text string) bool {
	return toolCallRegex.FindString(text) != ""
}

// ExtractModelText returns the model output with the tool_call JSON block removed.
// This gives the "reasoning" or "explanatory" text the model produced alongside
// the tool call.
func ExtractModelText(text string) string {
	return strings.TrimSpace(toolCallRegex.ReplaceAllString(text, ""))
}
