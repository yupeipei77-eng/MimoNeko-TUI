package worktree

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitWorktreeManager implements WorktreeManager using git worktree commands.
type GitWorktreeManager struct {
	registry      *Registry
	branchPrefix  string
	worktreeRoot  string // relative path under repo, e.g. ".mimoneko/worktrees"
	maxActive     int
	keepFailed    bool
	keepCancelled bool
}

// GitWorktreeManagerConfig configures the GitWorktreeManager.
type GitWorktreeManagerConfig struct {
	// BranchPrefix is the prefix for worktree branch names (default: "MimoNeko").
	BranchPrefix string

	// MaxActive is the maximum number of active worktrees (default: 10).
	MaxActive int

	// KeepFailed determines whether to keep failed worktrees for debugging.
	KeepFailed bool

	// KeepCancelled determines whether to keep cancelled worktrees for debugging.
	KeepCancelled bool
}

// DefaultGitWorktreeManagerConfig returns safe defaults.
func DefaultGitWorktreeManagerConfig() GitWorktreeManagerConfig {
	return GitWorktreeManagerConfig{
		BranchPrefix:  "MimoNeko",
		MaxActive:     10,
		KeepFailed:    true,
		KeepCancelled: true,
	}
}

// NewGitWorktreeManager creates a GitWorktreeManager with the given registry and config.
func NewGitWorktreeManager(registry *Registry, cfg GitWorktreeManagerConfig) *GitWorktreeManager {
	if cfg.BranchPrefix == "" {
		cfg.BranchPrefix = "MimoNeko"
	}
	if cfg.MaxActive <= 0 {
		cfg.MaxActive = 10
	}
	return &GitWorktreeManager{
		registry:      registry,
		branchPrefix:  cfg.BranchPrefix,
		worktreeRoot:  ".mimoneko/worktrees",
		maxActive:     cfg.MaxActive,
		keepFailed:    cfg.KeepFailed,
		keepCancelled: cfg.KeepCancelled,
	}
}

// Create creates a new isolated git worktree for the given task.
func (m *GitWorktreeManager) Create(ctx context.Context, req CreateWorktreeRequest) (WorktreeInfo, error) {
	// 1. Verify RepoRoot is a git repository
	if err := verifyGitRepo(req.RepoRoot); err != nil {
		return WorktreeInfo{}, fmt.Errorf("worktree: %w", err)
	}

	// 2. Sanitize task_id
	safeTaskID, err := SanitizeID(req.TaskID, 64)
	if err != nil {
		return WorktreeInfo{}, fmt.Errorf("worktree: sanitize task_id: %w", err)
	}

	// 3. Check max active worktrees
	activeCount, err := m.countActive(ctx)
	if err != nil {
		return WorktreeInfo{}, fmt.Errorf("worktree: count active: %w", err)
	}
	if activeCount >= m.maxActive {
		return WorktreeInfo{}, fmt.Errorf("worktree: max_active (%d) reached, discard or apply existing worktrees first", m.maxActive)
	}

	// 4. Generate unique worktree ID
	wtID, err := generateWorktreeID()
	if err != nil {
		return WorktreeInfo{}, fmt.Errorf("worktree: generate id: %w", err)
	}

	// 5. Compute short ID for branch name
	shortID := wtID[len("wt_") : len("wt_")+8] // first 8 hex chars

	// 6. Compute branch name: MimoNeko/<task_id>/<short_id>
	sanitizedTaskID, _ := SanitizeBranchName(safeTaskID, 40)
	branchName := fmt.Sprintf("%s/%s/%s", m.branchPrefix, sanitizedTaskID, shortID)

	// 7. Compute worktree path: .mimoneko/worktrees/<task_id>/<worktree_id>
	wtPath := filepath.Join(req.RepoRoot, m.worktreeRoot, safeTaskID, wtID)

	// 8. Verify path is within .mimoneko/worktrees
	absWtPath, err := filepath.Abs(wtPath)
	if err != nil {
		return WorktreeInfo{}, fmt.Errorf("worktree: resolve path: %w", err)
	}
	absExpectedRoot, err := filepath.Abs(filepath.Join(req.RepoRoot, m.worktreeRoot))
	if err != nil {
		return WorktreeInfo{}, fmt.Errorf("worktree: resolve root: %w", err)
	}
	if !strings.HasPrefix(absWtPath, absExpectedRoot+string(os.PathSeparator)) {
		return WorktreeInfo{}, fmt.Errorf("worktree: path escapes .mimoneko/worktrees")
	}

	// 9. Create parent directory
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o700); err != nil {
		return WorktreeInfo{}, fmt.Errorf("worktree: create directory: %w", err)
	}

	// 10. Determine base ref
	baseRef := req.BaseRef
	if baseRef == "" {
		baseRef = "HEAD"
	}

	// 11. Create git worktree with new branch
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branchName, wtPath, baseRef)
	cmd.Dir = req.RepoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return WorktreeInfo{}, fmt.Errorf("worktree: git worktree add: %w: %s", err, string(output))
	}

	// 12. Build info
	info := WorktreeInfo{
		ID:        wtID,
		TaskID:    req.TaskID,
		RepoRoot:  req.RepoRoot,
		Path:      wtPath,
		Branch:    branchName,
		BaseRef:   baseRef,
		CreatedAt: nowUTC(),
		State:     WorktreeStateActive,
		Metadata:  req.Metadata,
	}

	// 13. Record in registry
	if err := m.registry.Record(info); err != nil {
		// Attempt cleanup on registry failure
		_ = m.removeWorktreeFiles(ctx, req.RepoRoot, wtPath, branchName)
		return WorktreeInfo{}, fmt.Errorf("worktree: registry record: %w", err)
	}

	return info, nil
}

// Remove removes a worktree and its branch.
// It only removes worktrees that are in the registry.
func (m *GitWorktreeManager) Remove(ctx context.Context, id string) error {
	// 1. Look up in registry
	entries, err := m.registry.Load()
	if err != nil {
		return fmt.Errorf("worktree: load registry: %w", err)
	}

	info, found := entries[id]
	if !found {
		return fmt.Errorf("worktree: id %q not found in registry", id)
	}

	// 2. Remove worktree files
	if err := m.removeWorktreeFiles(ctx, info.RepoRoot, info.Path, info.Branch); err != nil {
		return fmt.Errorf("worktree: remove: %w", err)
	}

	// 3. Record discarded state
	info.State = WorktreeStateDiscarded
	if err := m.registry.Record(info); err != nil {
		return fmt.Errorf("worktree: record discard: %w", err)
	}

	return nil
}

// Get returns information about a specific worktree.
func (m *GitWorktreeManager) Get(ctx context.Context, id string) (WorktreeInfo, error) {
	entries, err := m.registry.Load()
	if err != nil {
		return WorktreeInfo{}, fmt.Errorf("worktree: load registry: %w", err)
	}

	info, found := entries[id]
	if !found {
		return WorktreeInfo{}, fmt.Errorf("worktree: id %q not found in registry", id)
	}

	return info, nil
}

// List returns all worktrees managed by this manager.
func (m *GitWorktreeManager) List(ctx context.Context) ([]WorktreeInfo, error) {
	entries, err := m.registry.Load()
	if err != nil {
		return nil, fmt.Errorf("worktree: load registry: %w", err)
	}

	result := make([]WorktreeInfo, 0, len(entries))
	for _, info := range entries {
		result = append(result, info)
	}
	return result, nil
}

// UpdateState updates the state of a worktree in the registry.
func (m *GitWorktreeManager) UpdateState(ctx context.Context, id string, state WorktreeState) error {
	entries, err := m.registry.Load()
	if err != nil {
		return fmt.Errorf("worktree: load registry: %w", err)
	}

	info, found := entries[id]
	if !found {
		return fmt.Errorf("worktree: id %q not found in registry", id)
	}

	info.State = state
	if err := m.registry.Record(info); err != nil {
		return fmt.Errorf("worktree: record state update: %w", err)
	}

	return nil
}

// countActive returns the number of worktrees in the active state.
func (m *GitWorktreeManager) countActive(ctx context.Context) (int, error) {
	entries, err := m.registry.Load()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, info := range entries {
		if info.State == WorktreeStateActive {
			count++
		}
	}
	return count, nil
}

// removeWorktreeFiles removes the git worktree and branch.
func (m *GitWorktreeManager) removeWorktreeFiles(ctx context.Context, repoRoot, wtPath, branch string) error {
	// 1. Remove git worktree
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", wtPath)
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		// If worktree directory doesn't exist, that's fine
		if !os.IsNotExist(err) {
			return fmt.Errorf("worktree: git worktree remove: %w: %s", err, string(output))
		}
	}

	// 2. Delete the branch
	cmd = exec.CommandContext(ctx, "git", "branch", "-D", branch)
	cmd.Dir = repoRoot
	// Ignore branch delete errors (branch may already be gone)
	_ = cmd.Run()

	// 3. Clean up empty parent directories
	parentDir := filepath.Dir(wtPath)
	if entries, err := os.ReadDir(parentDir); err == nil && len(entries) == 0 {
		_ = os.Remove(parentDir)
	}

	return nil
}

// verifyGitRepo checks that the given path is inside a git repository.
func verifyGitRepo(path string) error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("path %q is not a git repository: %w: %s", path, err, string(output))
	}
	return nil
}

// IsWorktreePathSafe checks that a worktree path is within the expected
// .mimoneko/worktrees directory.
func IsWorktreePathSafe(repoRoot, wtPath string) bool {
	absWtPath, err := filepath.Abs(wtPath)
	if err != nil {
		return false
	}
	absExpectedRoot, err := filepath.Abs(filepath.Join(repoRoot, ".mimoneko", "worktrees"))
	if err != nil {
		return false
	}
	return strings.HasPrefix(absWtPath, absExpectedRoot+string(os.PathSeparator))
}
