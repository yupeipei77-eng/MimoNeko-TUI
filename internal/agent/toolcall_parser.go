package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseToolCall attempts to extract a ToolCall from model output text.
// Returns nil if no tool call is found.
//
// The model output is expected to contain a JSON block like:
//
//	{"tool_call": {"name": "file_read", "args": {"path": "README.md"}}}
//
// The parser finds candidate JSON objects by locating balanced braces
// and checking if they contain a "tool_call" key. This is a minimal
// Phase 4 implementation; future phases may support structured
// function calling APIs.
func ParseToolCall(text string) (*ToolCall, error) {
	// Find candidate JSON objects by scanning for balanced braces
	start := -1
	depth := 0
	var found []*ToolCall
	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if text[i] == '}' {
			depth--
			if depth == 0 && start >= 0 {
				candidate := text[start : i+1]
				tc, err := tryParseToolCall(candidate)
				if err == nil && tc != nil {
					found = append(found, tc)
				}
				start = -1
			}
		}
	}

	switch len(found) {
	case 0:
		return nil, nil
	case 1:
		return found[0], nil
	default:
		return nil, fmt.Errorf("agent: multiple tool_call blocks found (%d), only one is allowed per step", len(found))
	}
}

// toolCallWrapper is used to unmarshal a tool_call JSON block.
type toolCallWrapper struct {
	ToolCall struct {
		Name string            `json:"name"`
		Args map[string]string `json:"args"`
	} `json:"tool_call"`
}

func tryParseToolCall(candidate string) (*ToolCall, error) {
	var wrapper toolCallWrapper
	if err := json.Unmarshal([]byte(candidate), &wrapper); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(wrapper.ToolCall.Name)
	if name == "" {
		return nil, nil
	}

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
	tc, err := ParseToolCall(text)
	if err != nil {
		return false
	}
	return tc != nil
}

// ExtractModelText returns the model output with the tool_call JSON block removed.
// This gives the "reasoning" or "explanatory" text the model produced alongside
// the tool call.
func ExtractModelText(text string) string {
	// Find and remove the tool_call JSON object
	start := -1
	depth := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if text[i] == '}' {
			depth--
			if depth == 0 && start >= 0 {
				candidate := text[start : i+1]
				var wrapper toolCallWrapper
				if err := json.Unmarshal([]byte(candidate), &wrapper); err == nil && wrapper.ToolCall.Name != "" {
					// Found the tool_call block, remove it
					before := text[:start]
					after := text[i+1:]
					return strings.TrimSpace(before + after)
				}
				start = -1
			}
		}
	}
	return strings.TrimSpace(text)
}

// FormatToolCall formats a ToolCall as the expected JSON string.
func FormatToolCall(tc ToolCall) string {
	argsJSON, _ := json.Marshal(tc.Args)
	return fmt.Sprintf(`{"tool_call": {"name": "%s", "args": %s}}`, tc.Name, string(argsJSON))
}
