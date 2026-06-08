package cli

import (
	"fmt"
	"strings"
)

func isReadOnlyNekoGoal(goal string) bool {
	text := strings.ToLower(strings.TrimSpace(goal))
	if text == "" {
		return false
	}
	writeMarkers := []string{
		"fix", "change", "modify", "update", "edit", "write", "implement",
		"add", "remove", "delete", "refactor", "apply", "patch", "create", "generate", "rename",
		"\u4fee\u590d", "\u4fee\u6539", "\u66f4\u65b0", "\u7f16\u8f91", "\u5b9e\u73b0",
		"\u65b0\u589e", "\u6dfb\u52a0", "\u5220\u9664", "\u91cd\u6784", "\u5e94\u7528",
		"\u521b\u5efa", "\u751f\u6210", "\u91cd\u547d\u540d", "\u6539\u540d",
	}
	for _, marker := range writeMarkers {
		if strings.Contains(text, marker) {
			return false
		}
	}
	readOnlyMarkers := []string{
		"inspect", "check", "analyze", "analyse", "review", "explain",
		"summarize", "summarise", "scan", "list", "show", "read", "audit",
		"\u68c0\u67e5", "\u67e5\u770b", "\u5206\u6790", "\u5ba1\u67e5", "\u89e3\u91ca",
		"\u603b\u7ed3", "\u626b\u63cf", "\u5217\u51fa", "\u9605\u8bfb",
	}
	for _, marker := range readOnlyMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func extractRunCommandResult(output string) string {
	lines := splitCLILines(output)
	for i, line := range lines {
		if strings.TrimSpace(line) != "Result:" {
			continue
		}
		var result []string
		for _, candidate := range lines[i+1:] {
			trimmed := strings.TrimSpace(candidate)
			if trimmed == "Run ID:" || strings.HasPrefix(trimmed, "Cache ") || strings.Contains(trimmed, "Patch generated") {
				break
			}
			result = append(result, candidate)
		}
		return strings.TrimSpace(strings.Join(result, "\n"))
	}
	return stripCLIApplyInstructions(output)
}

func extractRunCommandID(output string) string {
	return extractValueAfterLabel(output, "Run ID:")
}

func extractRunCommandWorktreeID(output string) string {
	lines := splitCLILines(output)
	for i, line := range lines {
		if strings.TrimSpace(line) != "Worktree:" {
			continue
		}
		for _, candidate := range lines[i+1:] {
			trimmed := strings.TrimSpace(candidate)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "ID") {
				return strings.TrimSpace(strings.TrimPrefix(trimmed, "ID"))
			}
			return ""
		}
	}
	return ""
}

func extractRunCommandState(output string) string {
	for _, line := range splitCLILines(output) {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "\u2713"))
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "\u221a"))
		switch strings.ToLower(trimmed) {
		case "completed":
			return "succeeded"
		case "failed":
			return "failed"
		}
	}
	return ""
}

func extractValueAfterLabel(output, label string) string {
	lines := splitCLILines(output)
	for i, line := range lines {
		if strings.TrimSpace(line) == label && i+1 < len(lines) {
			return strings.TrimSpace(lines[i+1])
		}
	}
	return ""
}

func summarizeMultiRunOutputForTUI(output string) string {
	lines := splitCLILines(output)
	var plan []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "step ") {
			plan = append(plan, strings.TrimPrefix(trimmed, "step "))
		}
	}
	if len(plan) == 0 {
		return stripCLIApplyInstructions(output)
	}
	var summary strings.Builder
	fmt.Fprintln(&summary, "Plan:")
	for _, step := range plan {
		fmt.Fprintf(&summary, "%s\n", step)
	}
	return strings.TrimSpace(summary.String())
}

func stripCLIApplyInstructions(output string) string {
	lines := splitCLILines(output)
	cleaned := make([]string, 0, len(lines))
	skip := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, "To apply changes, run:") {
			skip = true
			continue
		}
		if skip {
			if trimmed == "" || strings.HasPrefix(trimmed, "mimoneko patch apply ") {
				continue
			}
			skip = false
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func splitCLILines(output string) []string {
	normalized := strings.ReplaceAll(output, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.Split(normalized, "\n")
}
