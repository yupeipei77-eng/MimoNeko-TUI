package agent

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	xmlToolCallBlockPattern = regexp.MustCompile(`(?is)<tool_call>(.*?)</tool_call>`)
	xmlFunctionTagPattern   = regexp.MustCompile(`(?is)<function=([^>\s]+)>`)
	xmlFunctionBodyPattern  = regexp.MustCompile(`(?is)<function>(.*?)</function>`)
	xmlParameterPattern     = regexp.MustCompile(`(?is)<parameter=([^>\s]+)>(.*?)</parameter>`)
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
	for i := range len(text) {
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
		return parseXMLToolCall(text)
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

	return normalizeToolCall(&ToolCall{
		Name: name,
		Args: args,
	}), nil
}

func parseXMLToolCall(text string) (*ToolCall, error) {
	matches := xmlToolCallBlockPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil, nil
	}
	for _, match := range matches {
		tc := tryParseXMLToolCallBlock(match[1])
		if tc != nil {
			return tc, nil
		}
	}
	return &ToolCall{Name: "unsupported_tool_call", Args: map[string]string{"format": "xml"}}, nil
}

func tryParseXMLToolCallBlock(block string) *ToolCall {
	name := ""
	if match := xmlFunctionTagPattern.FindStringSubmatch(block); len(match) == 2 {
		name = strings.TrimSpace(match[1])
	} else if match := xmlFunctionBodyPattern.FindStringSubmatch(block); len(match) == 2 {
		name = strings.TrimSpace(match[1])
	}
	if name == "" {
		return nil
	}
	args := make(map[string]string)
	for _, match := range xmlParameterPattern.FindAllStringSubmatch(block, -1) {
		if len(match) != 3 {
			continue
		}
		key := strings.TrimSpace(match[1])
		value := strings.TrimSpace(html.UnescapeString(match[2]))
		if key != "" {
			args[key] = value
		}
	}
	return normalizeToolCall(&ToolCall{Name: name, Args: args})
}

func normalizeToolCall(tc *ToolCall) *ToolCall {
	if tc == nil {
		return nil
	}
	name := strings.ToLower(strings.TrimSpace(tc.Name))
	args := tc.Args
	if args == nil {
		args = make(map[string]string)
	}
	switch name {
	case "read_file":
		return &ToolCall{Name: "file_read", Args: args}
	case "diff", "gitdiff":
		return &ToolCall{Name: "git_diff", Args: args}
	case "bash", "shell", "sh":
		return normalizeShellLikeToolCall(args)
	default:
		return &ToolCall{Name: name, Args: args}
	}
}

func normalizeShellLikeToolCall(args map[string]string) *ToolCall {
	command := strings.TrimSpace(firstNonEmptyArg(args, "command", "cmd", "shell"))
	if command == "" {
		return &ToolCall{Name: "unsupported_shell", Args: map[string]string{"reason": "missing command"}}
	}
	lower := strings.ToLower(command)
	if shellCommandLooksUnsafe(lower) {
		return &ToolCall{Name: "unsupported_shell", Args: map[string]string{"command": command}}
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return &ToolCall{Name: "unsupported_shell", Args: map[string]string{"reason": "empty command"}}
	}
	switch strings.ToLower(fields[0]) {
	case "pwd", "cd":
		return &ToolCall{Name: "list_files", Args: map[string]string{"path": "."}}
	case "ls", "dir":
		mapped := map[string]string{"path": "."}
		if path := listFilesPathFromShellFields(fields[1:]); path != "" {
			mapped["path"] = path
		}
		if strings.Contains(lower, "-r") || strings.Contains(lower, "/s") {
			mapped["max_depth"] = "3"
		}
		return &ToolCall{Name: "list_files", Args: mapped}
	default:
		return &ToolCall{Name: "unsupported_shell", Args: map[string]string{"command": command}}
	}
}

func shellCommandLooksUnsafe(command string) bool {
	for _, token := range []string{";", "&&", "||", "|", ">", "<", "`", "$(", "\n"} {
		if strings.Contains(command, token) {
			return true
		}
	}
	return false
}

func listFilesPathFromShellFields(fields []string) string {
	path := ""
	for _, field := range fields {
		trimmed := strings.Trim(strings.TrimSpace(field), `"'`)
		if trimmed == "" || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "/") {
			continue
		}
		path = trimmed
	}
	return path
}

func firstNonEmptyArg(args map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(args[key]); value != "" {
			return value
		}
	}
	return ""
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
	for i := range len(text) {
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
	return strings.TrimSpace(xmlToolCallBlockPattern.ReplaceAllString(text, ""))
}

// FormatToolCall formats a ToolCall as the expected JSON string.
func FormatToolCall(tc ToolCall) string {
	argsJSON, _ := json.Marshal(tc.Args)
	return fmt.Sprintf(`{"tool_call": {"name": "%s", "args": %s}}`, tc.Name, string(argsJSON))
}
