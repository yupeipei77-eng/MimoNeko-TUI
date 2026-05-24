package patch

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/reasonforge/reasonforge/internal/task"
	"github.com/reasonforge/reasonforge/internal/tools"
	"github.com/reasonforge/reasonforge/internal/worktree"
)

// deniedPatchPaths lists paths that must never be modified by a patch apply.
// These supplement the TaskContract checks with hard-coded safety boundaries.
var deniedPatchPaths = []string{
	".git",
	".reasonforge",
	".env",
	"*.pem",
	"*.key",
	"id_rsa",
	"id_ed25519",
}

// GitPatchManager implements PatchManager using git commands.
type GitPatchManager struct {
	worktreeMgr      worktree.WorktreeManager
	maxDiffBytes     int
	requireCleanMain bool
	allowBinary      bool
}

// GitPatchManagerConfig configures the GitPatchManager.
type GitPatchManagerConfig struct {
	MaxDiffBytes     int
	RequireCleanMain bool
	AllowBinary      bool
}

// DefaultGitPatchManagerConfig returns safe defaults.
func DefaultGitPatchManagerConfig() GitPatchManagerConfig {
	return GitPatchManagerConfig{
		MaxDiffBytes:     131072,
		RequireCleanMain: true,
		AllowBinary:      false,
	}
}

// NewGitPatchManager creates a GitPatchManager.
func NewGitPatchManager(worktreeMgr worktree.WorktreeManager, cfg GitPatchManagerConfig) *GitPatchManager {
	if cfg.MaxDiffBytes <= 0 {
		cfg.MaxDiffBytes = 131072
	}
	return &GitPatchManager{
		worktreeMgr:      worktreeMgr,
		maxDiffBytes:     cfg.MaxDiffBytes,
		requireCleanMain: cfg.RequireCleanMain,
		allowBinary:      cfg.AllowBinary,
	}
}

// Preview generates a diff preview for the worktree's changes.
func (m *GitPatchManager) Preview(ctx context.Context, req PatchPreviewRequest) (PatchPreview, error) {
	// 1. Get worktree info
	info, err := m.worktreeMgr.Get(ctx, req.WorktreeID)
	if err != nil {
		return PatchPreview{}, fmt.Errorf("patch: get worktree: %w", err)
	}

	// 2. Parse file changes FIRST (no file content read yet)
	filesChanged, err := m.parseChangedFiles(ctx, info.RepoRoot, info.Path)
	if err != nil {
		return PatchPreview{}, fmt.Errorf("patch: parse changed files: %w", err)
	}

	// 3. Check for violations BEFORE generating any diff content
	violations := m.checkViolations(filesChanged, req.Contract)

	// 4. Compute summary (from file metadata only, not diff content)
	summary := m.computeSummary(filesChanged)

	// 5. Determine risk level
	riskLevel := m.assessRisk(filesChanged, violations, summary)

	// 6. Generate diff only if there are no violations.
	// If violations exist, the diff is redacted to prevent leaking
	// sensitive file content (.env, *.pem, etc.) through Preview.Diff.
	var diff string
	if len(violations) > 0 {
		diff = "[diff redacted due to policy violations]"
		riskLevel = "high"
	} else {
		diff, err = m.generateDiff(ctx, info.RepoRoot, info.Path)
		if err != nil {
			return PatchPreview{}, fmt.Errorf("patch: generate diff: %w", err)
		}

		// Truncate diff if too large
		if len(diff) > m.maxDiffBytes {
			diff = diff[:m.maxDiffBytes]
			riskLevel = "high" // truncated diff is always high risk
		}
	}

	preview := PatchPreview{
		WorktreeID:   req.WorktreeID,
		FilesChanged: filesChanged,
		Diff:         diff,
		Summary:      summary,
		RiskLevel:    riskLevel,
		Violations:   violations,
		GeneratedAt:  time.Now().UTC(),
	}

	return preview, nil
}

// Apply applies the worktree's changes to the main workspace.
func (m *GitPatchManager) Apply(ctx context.Context, req PatchApplyRequest) (PatchApplyResult, error) {
	// 1. Get worktree info
	info, err := m.worktreeMgr.Get(ctx, req.WorktreeID)
	if err != nil {
		return PatchApplyResult{}, fmt.Errorf("patch: get worktree: %w", err)
	}

	// 2. Preview first to check violations
	preview, err := m.Preview(ctx, PatchPreviewRequest{
		RepoRoot:   req.RepoRoot,
		WorktreeID: req.WorktreeID,
		Contract:   req.Contract,
	})
	if err != nil {
		return PatchApplyResult{}, fmt.Errorf("patch: preview: %w", err)
	}

	// 3. Refuse if violations exist
	if len(preview.Violations) > 0 {
		reasons := make([]string, 0, len(preview.Violations))
		for _, v := range preview.Violations {
			reasons = append(reasons, fmt.Sprintf("%s: %s", v.Path, v.Reason))
		}
		return PatchApplyResult{}, fmt.Errorf("patch: refused due to violations: %s", strings.Join(reasons, "; "))
	}

	// 4. Check main workspace is clean
	if m.requireCleanMain {
		if err := m.checkCleanMain(ctx, req.RepoRoot); err != nil {
			return PatchApplyResult{}, fmt.Errorf("patch: main workspace not clean: %w", err)
		}
	}

	// 5. If dry run, just return the preview info
	if req.DryRun {
		return PatchApplyResult{
			WorktreeID:   req.WorktreeID,
			Applied:      false,
			FilesChanged: preview.FilesChanged,
			Summary:      preview.Summary,
		}, nil
	}

	// 6. Apply the diff using git apply
	diff, err := m.generateDiff(ctx, info.RepoRoot, info.Path)
	if err != nil {
		return PatchApplyResult{}, fmt.Errorf("patch: generate diff: %w", err)
	}

	if strings.TrimSpace(diff) == "" {
		return PatchApplyResult{
			WorktreeID: req.WorktreeID,
			Applied:    false,
			Summary:    DiffSummary{},
		}, nil
	}

	// Write diff to a temp file and apply
	tmpFile, err := os.CreateTemp("", "reasonforge-patch-*.diff")
	if err != nil {
		return PatchApplyResult{}, fmt.Errorf("patch: create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(diff); err != nil {
		tmpFile.Close()
		return PatchApplyResult{}, fmt.Errorf("patch: write temp file: %w", err)
	}
	tmpFile.Close()

	cmd := exec.CommandContext(ctx, "git", "apply", "--ignore-space-change", tmpFile.Name())
	cmd.Dir = req.RepoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return PatchApplyResult{}, fmt.Errorf("patch: git apply: %w: %s", err, string(output))
	}

	// 7. Update worktree state
	var stateUpdateErr string
	if err := m.worktreeMgr.UpdateState(ctx, req.WorktreeID, worktree.WorktreeStateApplied); err != nil {
		// Patch was applied but state update failed - don't fail the operation,
		// but surface the error so callers can observe the inconsistency.
		stateUpdateErr = err.Error()
	}

	return PatchApplyResult{
		WorktreeID:       req.WorktreeID,
		Applied:          true,
		FilesChanged:     preview.FilesChanged,
		Summary:          preview.Summary,
		StateUpdateError: stateUpdateErr,
	}, nil
}

// Discard removes the worktree and marks it as discarded.
func (m *GitPatchManager) Discard(ctx context.Context, req PatchDiscardRequest) error {
	// Get worktree info before removing (for verification)
	info, err := m.worktreeMgr.Get(ctx, req.WorktreeID)
	if err != nil {
		return fmt.Errorf("patch: get worktree: %w", err)
	}

	// Verify the worktree path is safe
	if !worktree.IsWorktreePathSafe(req.RepoRoot, info.Path) {
		return fmt.Errorf("patch: worktree path %q is not under .reasonforge/worktrees, refusing to discard", info.Path)
	}

	// Remove the worktree
	if err := m.worktreeMgr.Remove(ctx, req.WorktreeID); err != nil {
		return fmt.Errorf("patch: discard worktree: %w", err)
	}

	return nil
}

// generateDiff runs git diff between the main workspace and the worktree,
// including untracked new files which are not covered by plain git diff.
func (m *GitPatchManager) generateDiff(ctx context.Context, repoRoot, worktreePath string) (string, error) {
	// 1. Tracked changes via git diff
	var trackedDiff string
	cmd := exec.CommandContext(ctx, "git", "diff")
	cmd.Dir = worktreePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// git diff may return exit code 1 when there are differences
			trackedDiff = string(output)
		} else {
			return "", fmt.Errorf("git diff: %w: %s", err, string(output))
		}
	} else {
		trackedDiff = string(output)
	}

	// 2. Untracked new files - not included in git diff output
	newFileDiff, err := m.generateNewFileDiffs(ctx, worktreePath)
	if err != nil {
		return "", fmt.Errorf("generate new file diffs: %w", err)
	}

	return trackedDiff + newFileDiff, nil
}

// generateNewFileDiffs generates unified diff entries for untracked (new) files
// in the worktree. These are not included in git diff output but must be part
// of the complete patch so that git apply can create them in the main workspace.
func (m *GitPatchManager) generateNewFileDiffs(ctx context.Context, worktreePath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = worktreePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git ls-files --others: %w: %s", err, string(output))
	}

	var buf strings.Builder
	for _, relPath := range strings.Split(string(output), "\n") {
		relPath = strings.TrimSpace(relPath)
		if relPath == "" {
			continue
		}

		absPath := filepath.Join(worktreePath, filepath.FromSlash(relPath))
		content, err := os.ReadFile(absPath)
		if err != nil {
			return "", fmt.Errorf("read new file %s: %w", relPath, err)
		}

		fmt.Fprintf(&buf, "diff --git a/%s b/%s\n", relPath, relPath)
		fmt.Fprintf(&buf, "new file mode 100644\n")

		if isBinaryContent(content) {
			fmt.Fprintf(&buf, "Binary files /dev/null and b/%s differ\n", relPath)
			continue
		}

		fmt.Fprintf(&buf, "--- /dev/null\n")
		fmt.Fprintf(&buf, "+++ b/%s\n", relPath)

		lineCount := countLines(content)
		if lineCount == 0 {
			// Empty new file - just the header, no hunk
			continue
		}

		fmt.Fprintf(&buf, "@@ -0,0 +1,%d @@\n", lineCount)

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if i == len(lines)-1 && line == "" {
				// Trailing newline produces an empty final element; skip it
				break
			}
			fmt.Fprintf(&buf, "+%s\n", line)
		}

		if len(content) > 0 && content[len(content)-1] != '\n' {
			buf.WriteString("\\ No newline at end of file\n")
		}
	}

	return buf.String(), nil
}

// isBinaryContent checks whether data appears to be binary by looking for
// null bytes within the first 8 KB.
func isBinaryContent(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// countLines returns the number of lines in data.
func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := strings.Count(string(data), "\n")
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}

// parseChangedFiles extracts the list of changed files from git diff.
func (m *GitPatchManager) parseChangedFiles(ctx context.Context, repoRoot, worktreePath string) ([]FileChange, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--numstat")
	cmd.Dir = worktreePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff --numstat: %w: %s", err, string(output))
	}

	var files []FileChange
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}

		adds := 0
		dels := 0
		isBinary := false

		if parts[0] == "-" && parts[1] == "-" {
			// Binary file
			isBinary = true
		} else {
			fmt.Sscanf(parts[0], "%d", &adds)
			fmt.Sscanf(parts[1], "%d", &dels)
		}

		fc := FileChange{
			Path:      parts[2],
			Status:    "modified", // numstat doesn't give status; default to modified
			Additions: adds,
			Deletions: dels,
		}

		if isBinary {
			fc.Status = "binary"
		}

		files = append(files, fc)
	}

	// Also check for untracked (new) files
	cmd = exec.CommandContext(ctx, "git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = worktreePath
	output, err = cmd.CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			fc := FileChange{
				Path:   line,
				Status: "added",
			}

			// Count lines and detect binary for untracked files
			absPath := filepath.Join(worktreePath, filepath.FromSlash(line))
			if content, readErr := os.ReadFile(absPath); readErr == nil {
				if isBinaryContent(content) {
					fc.Status = "binary"
				} else {
					fc.Additions = countLines(content)
				}
			}

			files = append(files, fc)
		}
	}

	// Check for deleted files
	cmd = exec.CommandContext(ctx, "git", "diff", "--name-status")
	cmd.Dir = worktreePath
	output, err = cmd.CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) == 2 && parts[0] == "D" {
				// Update existing entry or add new one
				found := false
				for i, f := range files {
					if f.Path == parts[1] {
						files[i].Status = "deleted"
						found = true
						break
					}
				}
				if !found {
					files = append(files, FileChange{
						Path:   parts[1],
						Status: "deleted",
					})
				}
			} else if len(parts) == 2 && parts[0] == "A" {
				for i, f := range files {
					if f.Path == parts[1] {
						files[i].Status = "added"
						break
					}
				}
			} else if len(parts) == 2 && strings.HasPrefix(parts[0], "R") {
				for i, f := range files {
					if f.Path == parts[1] {
						files[i].Status = "renamed"
						break
					}
				}
			}
		}
	}

	return files, nil
}

// computeSummary aggregates file changes into a summary.
func (m *GitPatchManager) computeSummary(files []FileChange) DiffSummary {
	summary := DiffSummary{
		FilesChanged: len(files),
	}
	for _, f := range files {
		summary.Additions += f.Additions
		summary.Deletions += f.Deletions
		if f.Status == "binary" {
			summary.HasBinary = true
		}
	}
	return summary
}

// checkViolations checks changed files against TaskContract and hard-coded deny paths.
func (m *GitPatchManager) checkViolations(files []FileChange, contract task.TaskContract) []PatchViolation {
	var violations []PatchViolation

	for _, f := range files {
		// Check TaskContract DeniedPaths
		if !contract.IsPathAllowed(f.Path) {
			violations = append(violations, PatchViolation{
				Path:   f.Path,
				Reason: "denied by task contract",
			})
			continue
		}

		// Check hard-coded deny paths
		if matchesAnyPattern(f.Path, deniedPatchPaths) {
			violations = append(violations, PatchViolation{
				Path:   f.Path,
				Reason: "path is in hard-coded deny list (.git, .reasonforge, .env, *.pem, *.key)",
			})
			continue
		}

		// Check for binary files if not allowed
		if f.Status == "binary" && !m.allowBinary {
			violations = append(violations, PatchViolation{
				Path:   f.Path,
				Reason: "binary file changes are not allowed (patch.allow_binary=false)",
			})
		}

		// Check for sensitive paths using existing safety module
		if tools.IsSensitiveFilePath(f.Path) {
			violations = append(violations, PatchViolation{
				Path:   f.Path,
				Reason: "path is a sensitive file (.env, *.pem, *.key, id_rsa, id_ed25519)",
			})
		}

		if tools.IsUnderProtectedDir(f.Path) {
			violations = append(violations, PatchViolation{
				Path:   f.Path,
				Reason: "path is under a protected directory (.git/, .reasonforge/)",
			})
		}
	}

	return violations
}

// assessRisk determines the overall risk level of a patch.
func (m *GitPatchManager) assessRisk(files []FileChange, violations []PatchViolation, summary DiffSummary) string {
	if len(violations) > 0 {
		return "high"
	}
	if summary.HasBinary {
		return "high"
	}
	if summary.FilesChanged > 20 || summary.Additions+summary.Deletions > 500 {
		return "high"
	}
	if summary.FilesChanged > 5 || summary.Additions+summary.Deletions > 100 {
		return "medium"
	}
	return "low"
}

// checkCleanMain verifies the main workspace has no uncommitted changes.
func (m *GitPatchManager) checkCleanMain(ctx context.Context, repoRoot string) error {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git status: %w: %s", err, string(output))
	}

	// Filter out .reasonforge/ paths - worktree files live there and are
	// expected to appear as untracked in the main workspace.
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	dirty := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// git status --porcelain format: XY PATH or XY ORIG_PATH -> PATH
		// Extract the path portion (after the 2-char status + space)
		pathPart := line[3:] // skip "XY "
		if strings.HasPrefix(pathPart, ".reasonforge/") || strings.HasPrefix(pathPart, ".reasonforge\\") {
			continue // ignore .reasonforge changes
		}
		dirty = true
		break
	}

	if dirty {
		return fmt.Errorf("main workspace has uncommitted changes; commit or stash before applying a patch")
	}
	return nil
}

// matchesAnyPattern checks if a relative path matches any glob pattern.
func matchesAnyPattern(relPath string, patterns []string) bool {
	normalized := filepath.ToSlash(relPath)
	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, normalized)
		if matched {
			return true
		}
		// If pattern has no slash, also match against basename
		if !strings.Contains(pattern, "/") {
			base := filepath.Base(normalized)
			matched, _ = filepath.Match(pattern, base)
			if matched {
				return true
			}
		}
	}
	return false
}
