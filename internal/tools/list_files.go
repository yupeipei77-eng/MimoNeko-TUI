package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ListFilesTool lists non-sensitive files and directories within the workspace.
type ListFilesTool struct{}

func (t *ListFilesTool) Name() string { return "list_files" }
func (t *ListFilesTool) Description() string {
	return "List files and directories within the workspace root"
}
func (t *ListFilesTool) RiskLevel() string             { return "low" }
func (t *ListFilesTool) Concurrency() ConcurrencyClass { return ConcurrencyReadOnly }

func (t *ListFilesTool) Run(ctx context.Context, req ToolRequest) (ToolResponse, error) {
	path := strings.TrimSpace(req.Args["path"])
	if path == "" {
		path = "."
	}
	if IsSensitiveFilePath(path) || IsUnderProtectedDir(path) {
		return toolError("list_files", fmt.Sprintf("path %q is denied by list policy", path)), nil
	}

	guard := safetyGuardFromRequest(req)
	absPath, err := guard.SafePath(req.RepoRoot, path)
	if err != nil {
		return toolError("list_files", err.Error()), nil
	}

	maxDepth := positiveIntArg(req.Args, "max_depth", 2)
	maxEntries := positiveIntArg(req.Args, "max_entries", 200)

	info, err := os.Stat(absPath)
	if err != nil {
		return toolError("list_files", fmt.Sprintf("stat %q: %v", path, err)), nil
	}
	if !info.IsDir() {
		rel, err := filepath.Rel(req.RepoRoot, absPath)
		if err != nil {
			return toolError("list_files", fmt.Sprintf("rel %q: %v", path, err)), nil
		}
		return ToolResponse{
			ToolName:    "list_files",
			Success:     true,
			ExitCode:    0,
			Stdout:      filepath.ToSlash(rel) + "\n",
			OutputBytes: len(rel) + 1,
		}, nil
	}

	var lines []string
	truncated := false
	err = filepath.WalkDir(absPath, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if current == absPath {
			return nil
		}
		rel, err := filepath.Rel(req.RepoRoot, current)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if shouldHideListedPath(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		depth := pathDepth(rel) - pathDepth(filepath.ToSlash(strings.Trim(path, "./\\")))
		if depth > maxDepth {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		display := rel
		if entry.IsDir() {
			display += "/"
		}
		lines = append(lines, display)
		if len(lines) >= maxEntries {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return toolError("list_files", err.Error()), nil
	}
	if truncated {
		lines = append(lines, fmt.Sprintf("... truncated at %d entries", maxEntries))
	}
	output := strings.Join(lines, "\n")
	if output != "" {
		output += "\n"
	}
	return ToolResponse{
		ToolName:    "list_files",
		Success:     true,
		ExitCode:    0,
		Stdout:      output,
		OutputBytes: len(output),
		Truncated:   truncated,
	}, nil
}

func positiveIntArg(args map[string]string, key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(args[key]))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func shouldHideListedPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return true
	}
	return IsSensitiveFilePath(path) || IsUnderProtectedDir(path)
}

func pathDepth(path string) int {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" || path == "." {
		return 0
	}
	return strings.Count(path, "/") + 1
}
