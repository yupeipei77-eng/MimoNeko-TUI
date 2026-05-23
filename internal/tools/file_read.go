package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
)

// FileReadTool reads file contents within the workspace root.
type FileReadTool struct{}

func (t *FileReadTool) Name() string        { return "file_read" }
func (t *FileReadTool) Description() string  { return "Read file contents within the workspace root" }
func (t *FileReadTool) RiskLevel() string    { return "low" }

func (t *FileReadTool) Run(ctx context.Context, req ToolRequest) (ToolResponse, error) {
	path, ok := req.Args["path"]
	if !ok || path == "" {
		return toolError("file_read", "arg 'path' is required"), nil
	}

	// Build safety guard from policy defaults
	guard := safetyGuardFromRequest(req)

	// Check if reading this path is denied
	if guard.IsReadDenied(path) {
		return toolError("file_read", fmt.Sprintf("path %q is denied by read policy", path)), nil
	}

	// Check sensitive file patterns
	if IsSensitiveFilePath(path) {
		return toolError("file_read", fmt.Sprintf("path %q is a sensitive file, reading is denied", path)), nil
	}

	// Check protected directories (.git, .reasonforge)
	if IsUnderProtectedDir(path) {
		return toolError("file_read", fmt.Sprintf("path %q is under a protected directory, reading is denied", path)), nil
	}

	// Resolve safe path
	absPath, err := guard.SafePath(req.RepoRoot, path)
	if err != nil {
		return toolError("file_read", err.Error()), nil
	}

	// Determine max bytes
	maxBytes := DefaultMaxReadBytes
	if mb, ok := req.Args["max_bytes"]; ok {
		if v, err := strconv.Atoi(mb); err == nil && v > 0 {
			maxBytes = v
		}
	}

	// Read file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return toolError("file_read", fmt.Sprintf("read %q: %v", path, err)), nil
	}

	// Check context cancellation
	if ctx.Err() != nil {
		return toolError("file_read", "cancelled"), nil
	}

	// Truncate if needed
	truncated := false
	if len(data) > maxBytes {
		data = data[:maxBytes]
		truncated = true
	}

	// Compute content hash
	hash := sha256.Sum256(data)

	resp := ToolResponse{
		ToolName:   "file_read",
		Success:    true,
		ExitCode:   0,
		Stdout:     string(data),
		OutputBytes: len(data),
		Truncated:  truncated,
		Artifacts: []ToolArtifact{
			{
				Kind:        "file_read",
				Path:        path,
				ContentHash: fmt.Sprintf("%x", hash),
			},
		},
	}
	return resp, nil
}
