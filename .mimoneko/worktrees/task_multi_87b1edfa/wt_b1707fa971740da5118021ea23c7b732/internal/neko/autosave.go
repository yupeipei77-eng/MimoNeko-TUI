package neko

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type autoSaveResult struct {
	Path string
}

var (
	saveIntentPattern  = regexp.MustCompile(`(?i)(保存到|写入到|生成文件到|存放位置|保存为|写到|存到|落盘到|save\s+to|write\s+to|create\s+file|generate\s+file)`)
	windowsPathPattern = regexp.MustCompile(`[A-Za-z]:[\\/][^\r\n"'<>|?*，。；;、]*`)
	fileNamePattern    = regexp.MustCompile(`(?i)([A-Za-z0-9_.-]+)\.(bat|cmd|ps1|sh|py|js|ts|go|md|txt|json|yaml|yml|html|css|csv)`)
	fencedBlockPattern = regexp.MustCompile("(?s)```([A-Za-z0-9_+.-]*)\\s*\\r?\\n(.*?)\\r?\\n```")
)

func maybeAutoSaveChatResponse(root, message, response string) (autoSaveResult, bool, error) {
	if !saveIntentPattern.MatchString(message) {
		return autoSaveResult{}, false, nil
	}
	content, language := extractSaveContent(response)
	target := resolveAutoSavePath(root, message, language)
	if strings.TrimSpace(content) == "" {
		return autoSaveResult{}, true, fmt.Errorf("generated content is empty")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return autoSaveResult{}, true, err
	}
	if err := os.WriteFile(target, []byte(normalizeFileContent(content)), 0o600); err != nil {
		return autoSaveResult{}, true, err
	}
	return autoSaveResult{Path: target}, true, nil
}

func extractSaveContent(response string) (content, language string) {
	matches := fencedBlockPattern.FindStringSubmatch(response)
	if len(matches) == 3 {
		return strings.TrimSpace(matches[2]), strings.ToLower(strings.TrimSpace(matches[1]))
	}
	return strings.TrimSpace(response), ""
}

func resolveAutoSavePath(root, message, language string) string {
	if path, ok := explicitFilePath(message); ok {
		return normalizeTargetPath(root, path)
	}
	dir := root
	if path, ok := explicitDirectoryPath(message); ok {
		dir = normalizeTargetPath(root, path)
	}
	filename := explicitFileName(message)
	if filename == "" {
		filename = "neko_generated" + inferExtension(message, language)
	}
	return filepath.Join(dir, filepath.Base(filename))
}

func explicitFilePath(message string) (string, bool) {
	for _, candidate := range candidatePaths(message) {
		if filepath.Ext(candidate) != "" {
			return candidate, true
		}
	}
	return "", false
}

func explicitDirectoryPath(message string) (string, bool) {
	for _, candidate := range candidatePaths(message) {
		if filepath.Ext(candidate) == "" {
			return candidate, true
		}
	}
	return "", false
}

func candidatePaths(message string) []string {
	var out []string
	for _, match := range windowsPathPattern.FindAllString(message, -1) {
		match = strings.TrimSpace(strings.TrimRight(match, ".，。；;、"))
		if match != "" {
			out = append(out, match)
		}
	}
	return out
}

func explicitFileName(message string) string {
	for _, match := range fileNamePattern.FindAllString(message, -1) {
		if strings.Contains(match, `:\`) || strings.Contains(match, `:/`) {
			continue
		}
		return match
	}
	return ""
}

func normalizeTargetPath(root, target string) string {
	target = strings.TrimSpace(strings.Trim(target, `"'`))
	target = filepath.FromSlash(target)
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Join(root, target)
}

func inferExtension(message, language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "bat", "batch":
		return ".bat"
	case "cmd":
		return ".cmd"
	case "powershell", "ps1":
		return ".ps1"
	case "bash", "shell", "sh":
		return ".sh"
	case "python", "py":
		return ".py"
	case "javascript", "js":
		return ".js"
	case "typescript", "ts":
		return ".ts"
	case "go", "golang":
		return ".go"
	case "markdown", "md":
		return ".md"
	case "json":
		return ".json"
	case "yaml", "yml":
		return ".yaml"
	case "html":
		return ".html"
	case "css":
		return ".css"
	case "csv":
		return ".csv"
	}
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "bat") || strings.Contains(message, "批处理"):
		return ".bat"
	case strings.Contains(lower, "cmd"):
		return ".cmd"
	case strings.Contains(lower, "powershell") || strings.Contains(lower, "ps1"):
		return ".ps1"
	case strings.Contains(lower, "python") || strings.Contains(lower, "py"):
		return ".py"
	case strings.Contains(lower, "markdown") || strings.Contains(lower, "readme"):
		return ".md"
	case strings.Contains(lower, "json"):
		return ".json"
	case strings.Contains(lower, "html"):
		return ".html"
	default:
		return ".txt"
	}
}

func normalizeFileContent(content string) string {
	content = strings.Trim(content, "\r\n")
	return strings.ReplaceAll(content, "\n", "\r\n") + "\r\n"
}
