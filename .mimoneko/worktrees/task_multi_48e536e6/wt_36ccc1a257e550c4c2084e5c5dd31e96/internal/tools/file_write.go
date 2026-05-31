package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FileWriteTool writes file contents within the workspace root.
type FileWriteTool struct{}

func (t *FileWriteTool) Name() string        { return "file_write" }
func (t *FileWriteTool) Description() string  { return "Write file contents within the workspace root" }
func (t *FileWriteTool) RiskLevel() string    { return "medium" }

func (t *FileWriteTool) Run(ctx context.Context, req ToolRequest) (ToolResponse, error) {
	path, ok := req.Args["path"]
	if !ok || path == "" {
		return toolError("file_write", "arg 'path' is required"), nil
	}
	content, ok := req.Args["content"]
	if !ok {
		return toolError("file_write", "arg 'content' is required"), nil
	}

	guard := safetyGuardFromRequest(req)

	// Check write deny policy
	if guard.IsWriteDenied(path) {
		return toolError("file_write", fmt.Sprintf("path %q is denied by write policy", path)), nil
	}

	// Check sensitive file patterns
	if IsSensitiveFilePath(path) {
		return toolError("file_write", fmt.Sprintf("path %q is a sensitive file, writing is denied", path)), nil
	}

	// Check protected directories
	if IsUnderProtectedDir(path) {
		return toolError("file_write", fmt.Sprintf("path %q is under a protected directory", path)), nil
	}

	// Resolve safe path
	absPath, err := guard.SafePath(req.RepoRoot, path)
	if err != nil {
		return toolError("file_write", err.Error()), nil
	}

	// Compute content hash for artifact (even in dry-run)
	hash := sha256.Sum256([]byte(content))
	contentHash := fmt.Sprintf("%x", hash)

	// DryRun: report what would be written without writing
	if req.DryRun {
		resp := ToolResponse{
			ToolName: "file_write",
			Success:  true,
			ExitCode: 0,
			Stdout:   fmt.Sprintf("[dry-run] would write %d bytes to %q", len(content), path),
			Artifacts: []ToolArtifact{
				{
					Kind:        "file_would_create",
					Path:        path,
					ContentHash: contentHash,
				},
			},
		}
		return resp, nil
	}

	// Create parent directories if requested
	createDirs := true // default
	if v, ok := req.Args["create_dirs"]; ok {
		createDirs, _ = strconv.ParseBool(v)
	}

	if createDirs {
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return toolError("file_write", fmt.Sprintf("create dirs for %q: %v", path, err)), nil
		}
	}

	// Write file
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return toolError("file_write", fmt.Sprintf("write %q: %v", path, err)), nil
	}

	// Verify context
	if ctx.Err() != nil {
		return toolError("file_write", "cancelled after write"), nil
	}

	resp := ToolResponse{
		ToolName: "file_write",
		Success:  true,
		ExitCode: 0,
		Stdout:   fmt.Sprintf("wrote %d bytes to %q", len(content), path),
		Artifacts: []ToolArtifact{
			{
				Kind:        "file_created",
				Path:        path,
				ContentHash: contentHash,
			},
		},
	}
	return resp, nil
}

// FilePatchTool applies a simple text replacement patch to a file.
type FilePatchTool struct{}

func (t *FilePatchTool) Name() string        { return "file_patch" }
func (t *FilePatchTool) Description() string  { return "Apply a simple text replacement patch to a file" }
func (t *FilePatchTool) RiskLevel() string    { return "medium" }

func (t *FilePatchTool) Run(ctx context.Context, req ToolRequest) (ToolResponse, error) {
	path, ok := req.Args["path"]
	if !ok || path == "" {
		return toolError("file_patch", "arg 'path' is required"), nil
	}
	old, ok := req.Args["old"]
	if !ok {
		return toolError("file_patch", "arg 'old' is required"), nil
	}
	newStr, ok := req.Args["new"]
	if !ok {
		return toolError("file_patch", "arg 'new' is required"), nil
	}

	guard := safetyGuardFromRequest(req)

	// Check write deny policy
	if guard.IsWriteDenied(path) {
		return toolError("file_patch", fmt.Sprintf("path %q is denied by write policy", path)), nil
	}

	// Check sensitive file patterns
	if IsSensitiveFilePath(path) {
		return toolError("file_patch", fmt.Sprintf("path %q is a sensitive file, patching is denied", path)), nil
	}

	// Check protected directories
	if IsUnderProtectedDir(path) {
		return toolError("file_patch", fmt.Sprintf("path %q is under a protected directory", path)), nil
	}

	// Resolve safe path
	absPath, err := guard.SafePath(req.RepoRoot, path)
	if err != nil {
		return toolError("file_patch", err.Error()), nil
	}

	// Read current content
	data, err := os.ReadFile(absPath)
	if err != nil {
		return toolError("file_patch", fmt.Sprintf("read %q: %v", path, err)), nil
	}

	content := string(data)

	// Check old string exists
	count := strings.Count(content, old)
	if count == 0 {
		return toolError("file_patch", fmt.Sprintf("old string not found in %q", path)), nil
	}
	if count > 1 {
		return toolError("file_patch", fmt.Sprintf("old string appears %d times in %q, must be unique for safe patch", count, path)), nil
	}

	// Apply replacement
	patched := strings.Replace(content, old, newStr, 1)

	// Compute content hash
	hash := sha256.Sum256([]byte(patched))
	contentHash := fmt.Sprintf("%x", hash)

	// DryRun
	if req.DryRun {
		resp := ToolResponse{
			ToolName: "file_patch",
			Success:  true,
			ExitCode: 0,
			Stdout:   fmt.Sprintf("[dry-run] would patch %q: replace %d bytes with %d bytes", path, len(old), len(newStr)),
			Artifacts: []ToolArtifact{
				{
					Kind:        "file_would_patch",
					Path:        path,
					ContentHash: contentHash,
				},
			},
		}
		return resp, nil
	}

	// Write patched content
	if err := os.WriteFile(absPath, []byte(patched), 0o644); err != nil {
		return toolError("file_patch", fmt.Sprintf("write patched %q: %v", path, err)), nil
	}

	resp := ToolResponse{
		ToolName: "file_patch",
		Success:  true,
		ExitCode: 0,
		Stdout:   fmt.Sprintf("patched %q: replaced %d bytes with %d bytes", path, len(old), len(newStr)),
		Artifacts: []ToolArtifact{
			{
				Kind:        "file_patched",
				Path:        path,
				ContentHash: contentHash,
			},
		},
	}
	return resp, nil
}
