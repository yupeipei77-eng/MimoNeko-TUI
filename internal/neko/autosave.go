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
	saveIntentPattern  = regexp.MustCompile(`(?i)(保存到|写入到|生成文件到|存放位置|保存为|写到|存到|落盘到|保存为文件|生成脚本|生成代码|帮我写|帮我生成|创建.*脚本|创建.*文件|save\s+to|write\s+to|create\s+file|generate\s+file|脚本|代码|BAT脚本|修改.*路径)`)
	windowsPathPattern = regexp.MustCompile(`[A-Za-z]:[\\/][^\r\n"'` + "`" + `<>|?*，。；;、]*`)
	envPathPattern     = regexp.MustCompile(`%[A-Za-z_][A-Za-z0-9_]*%[\\/][^\r\n"'` + "`" + `<>|?*，。；;、]*`)
	envVarPattern      = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)
	driveWordPattern   = regexp.MustCompile(`(?i)([A-Za-z])\s*盘`)
	fileNamePattern    = regexp.MustCompile(`(?i)([A-Za-z0-9_.-]+)\.(bat|cmd|ps1|sh|py|js|ts|go|md|txt|json|yaml|yml|html|css|csv)`)
	namedFilePattern   = regexp.MustCompile(`(?i)(?:文件名(?:为|叫)?|命名为|取名为|保存为)\s*([A-Za-z0-9_.-]{2,64})`)
	fencedBlockPattern = regexp.MustCompile("(?s)```([A-Za-z0-9_+.-]*)\\s*\\r?\\n(.*?)\\r?\\n```")
)

func maybeAutoSaveChatResponse(root, message, response string) (autoSaveResult, bool, error) {
	if !hasAutoSaveIntent(message) {
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

func hasAutoSaveIntent(message string) bool {
	return saveIntentPattern.MatchString(message)
}

func PrepareModelPrompt(message string) string {
	message = strings.TrimSpace(message)
	if !hasAutoSaveIntent(message) {
		return message
	}
	return "Return only one fenced code block that implements the user's request. Do not include explanations, tables, setup steps, or prose outside the code block.\n\nUser request:\n" + message
}

func ModelMaxTokens(message string) int {
	if hasAutoSaveIntent(message) {
		return 2048
	}
	return 0
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
		filename = inferAutoSaveFileName(message, language)
	} else if filepath.Ext(filename) == "" {
		filename += inferExtension(message, language)
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
	if path, ok := naturalDirectoryPath(message); ok {
		return path, true
	}
	return "", false
}

func candidatePaths(message string) []string {
	var out []string
	for _, pattern := range []*regexp.Regexp{windowsPathPattern, envPathPattern} {
		for _, match := range pattern.FindAllString(message, -1) {
			match = cleanPathCandidate(match)
			if match != "" {
				out = append(out, match)
			}
		}
	}
	return out
}

func cleanPathCandidate(value string) string {
	return strings.TrimSpace(strings.Trim(value, " \t\r\n`\"'()[]{}<>.,，。；;、"))
}

func naturalDirectoryPath(message string) (string, bool) {
	compact := strings.ToLower(strings.Join(strings.Fields(message), ""))
	if strings.Contains(compact, "项目目录") || strings.Contains(compact, "当前目录") || strings.Contains(compact, "默认项目目录") {
		return "", false
	}
	if match := driveWordPattern.FindStringSubmatch(message); len(match) == 2 {
		drive := strings.ToUpper(match[1])
		if strings.Contains(compact, strings.ToLower(match[0])+"根目录") {
			return drive + `:\`, true
		}
		if strings.Contains(compact, "桌面") ||
			strings.Contains(compact, "desktop") ||
			strings.Contains(compact, strings.ToLower(match[0])+"桌面") ||
			strings.Contains(compact, strings.ToLower(match[0])+"的桌面") ||
			strings.Contains(compact, strings.ToLower(match[0])+"下桌面") ||
			strings.Contains(compact, strings.ToLower(match[0])+"desktop") {
			return drive + `:\Desktop`, true
		}
		return drive + `:\`, true
	}
	if strings.Contains(compact, "桌面") || strings.Contains(compact, "desktop") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, "Desktop"), true
		}
	}
	if strings.Contains(compact, "下载") || strings.Contains(compact, "downloads") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, "Downloads"), true
		}
	}
	return "", false
}

func explicitFileName(message string) string {
	for _, match := range fileNamePattern.FindAllString(message, -1) {
		if strings.Contains(match, `:\`) || strings.Contains(match, `:/`) {
			continue
		}
		return match
	}
	if match := namedFilePattern.FindStringSubmatch(message); len(match) == 2 {
		name := sanitizeFileBaseName(match[1])
		if name != "" {
			return name
		}
	}
	return ""
}

func inferAutoSaveFileName(message, language string) string {
	ext := inferExtension(message, language)
	base := inferAutoSaveBaseName(message, language)
	if filepath.Ext(base) != "" {
		return sanitizeFileBaseName(base)
	}
	return sanitizeFileBaseName(base) + ext
}

func inferAutoSaveBaseName(message, language string) string {
	compact := strings.ToLower(strings.Join(strings.Fields(message), ""))
	if strings.Contains(compact, "桌面") && (strings.Contains(compact, "迁移") || strings.Contains(compact, "移动")) {
		if match := driveWordPattern.FindStringSubmatch(message); len(match) == 2 {
			return "migrate_desktop_to_" + strings.ToLower(match[1]) + "_drive"
		}
		return "migrate_desktop"
	}
	if strings.Contains(compact, "注册表") && (strings.Contains(compact, "备份") || strings.Contains(compact, "导出")) {
		return "backup_registry"
	}
	if strings.Contains(compact, "注册表") && (strings.Contains(compact, "恢复") || strings.Contains(compact, "还原")) {
		return "restore_registry"
	}
	if strings.Contains(compact, "清理") && (strings.Contains(compact, "临时") || strings.Contains(compact, "temp")) {
		return "cleanup_temp_files"
	}
	if strings.Contains(compact, "下载") && (strings.Contains(compact, "清理") || strings.Contains(compact, "整理")) {
		return "organize_downloads"
	}
	if strings.Contains(strings.ToLower(message), "bat") || strings.Contains(message, "批处理") {
		return "batch_script"
	}
	if strings.Contains(message, "脚本") || isScriptLanguage(language) {
		return "script"
	}
	if strings.Contains(message, "代码") {
		return "code"
	}
	return "generated_file"
}

func sanitizeFileBaseName(name string) string {
	name = strings.TrimSpace(strings.Trim(name, " \t\r\n`\"'()[]{}<>.,，。；;、"))
	if name == "" {
		return "generated_file"
	}
	var out strings.Builder
	lastUnderscore := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			out.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			out.WriteByte('_')
			lastUnderscore = true
		}
	}
	cleaned := strings.Trim(out.String(), "._-")
	if cleaned == "" {
		return "generated_file"
	}
	return strings.ToLower(cleaned)
}

func isScriptLanguage(language string) bool {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "bat", "batch", "cmd", "powershell", "ps1", "bash", "shell", "sh", "python", "py", "javascript", "js", "typescript", "ts":
		return true
	default:
		return false
	}
}

func normalizeTargetPath(root, target string) string {
	target = cleanPathCandidate(expandWindowsEnv(target))
	if strings.HasPrefix(target, "~") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			target = filepath.Join(home, strings.TrimLeft(strings.TrimPrefix(target, "~"), `\/`))
		}
	}
	target = filepath.FromSlash(target)
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Join(root, target)
}

func expandWindowsEnv(value string) string {
	value = envVarPattern.ReplaceAllStringFunc(value, func(match string) string {
		name := strings.Trim(match, "%")
		if expanded, ok := os.LookupEnv(name); ok {
			return expanded
		}
		return match
	})
	return os.ExpandEnv(value)
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
