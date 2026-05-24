package patch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reasonforge/reasonforge/internal/task"
	"github.com/reasonforge/reasonforge/internal/worktree"
)

// setupPatchTest creates a git repo with a worktree that has modifications.
func setupPatchTest(t *testing.T) (string, *GitPatchManager, string) {
	t.Helper()

	// Create git repo
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	// Create initial files
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	// Set up worktree manager
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())

	// Create a worktree
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "patch-test",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create worktree: %v", err)
	}

	// Modify a file in the worktree
	modifiedPath := filepath.Join(info.Path, "README.md")
	if err := os.WriteFile(modifiedPath, []byte("# Hello World\nModified by agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a new file in the worktree
	newFilePath := filepath.Join(info.Path, "new_file.txt")
	if err := os.WriteFile(newFilePath, []byte("new content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create patch manager
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	return root, patchMgr, info.ID
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, string(output))
	}
}

func TestPreviewListsChangedFiles(t *testing.T) {
	root, patchMgr, wtID := setupPatchTest(t)

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: wtID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if preview.WorktreeID != wtID {
		t.Fatalf("WorktreeID = %q, want %q", preview.WorktreeID, wtID)
	}

	if len(preview.FilesChanged) == 0 {
		t.Fatal("expected at least one changed file")
	}

	// Find the README.md change
	found := false
	for _, f := range preview.FilesChanged {
		if f.Path == "README.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected README.md in changed files, got %v", preview.FilesChanged)
	}
}

func TestPreviewGeneratesDiffSummary(t *testing.T) {
	root, patchMgr, wtID := setupPatchTest(t)

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: wtID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if preview.Summary.FilesChanged == 0 {
		t.Fatal("expected at least one file changed in summary")
	}

	if preview.Diff == "" {
		t.Fatal("expected non-empty diff")
	}
}

func TestPreviewDetectsDeniedPathsViolation(t *testing.T) {
	root, patchMgr, wtID := setupPatchTest(t)

	// Contract that denies README.md
	contract := task.TaskContract{
		ID:          "tc_test",
		Goal:        "test",
		RepoRoot:    root,
		DeniedPaths: []string{"README.md"},
		MaxSteps:    5,
		CreatedAt:   timeNow(),
	}

	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: wtID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if len(preview.Violations) == 0 {
		t.Fatal("expected violations for denied paths")
	}

	found := false
	for _, v := range preview.Violations {
		if v.Path == "README.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected README.md violation, got %v", preview.Violations)
	}
}

func TestPreviewDetectsAllowedPathsViolation(t *testing.T) {
	root, patchMgr, wtID := setupPatchTest(t)

	// Contract that only allows main.go
	contract := task.TaskContract{
		ID:           "tc_test",
		Goal:         "test",
		RepoRoot:     root,
		AllowedPaths: []string{"main.go"},
		MaxSteps:     5,
		CreatedAt:    timeNow(),
	}

	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: wtID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if len(preview.Violations) == 0 {
		t.Fatal("expected violations for paths outside AllowedPaths")
	}
}

func TestApplyWithViolationsRefuses(t *testing.T) {
	root, patchMgr, wtID := setupPatchTest(t)

	contract := task.TaskContract{
		ID:          "tc_test",
		Goal:        "test",
		RepoRoot:    root,
		DeniedPaths: []string{"README.md"},
		MaxSteps:    5,
		CreatedAt:   timeNow(),
	}

	_, err := patchMgr.Apply(context.Background(), PatchApplyRequest{
		RepoRoot:   root,
		WorktreeID: wtID,
		Contract:   contract,
	})
	if err == nil {
		t.Fatal("expected error when applying with violations")
	}
	if !strings.Contains(err.Error(), "violations") {
		t.Fatalf("error = %q, want violations", err.Error())
	}
}

func TestApplyDirtyMainRefuses(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "dirty-test",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Modify file in worktree
	if err := os.WriteFile(filepath.Join(info.Path, "README.md"), []byte("# Modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Make main workspace dirty
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	_, err = patchMgr.Apply(context.Background(), PatchApplyRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err == nil {
		t.Fatal("expected error when main workspace is dirty")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("error = %q, want 'uncommitted changes'", err.Error())
	}
}

func TestApplyDryRunNoModification(t *testing.T) {
	root, patchMgr, wtID := setupPatchTest(t)

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	result, err := patchMgr.Apply(context.Background(), PatchApplyRequest{
		RepoRoot:   root,
		WorktreeID: wtID,
		Contract:   contract,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("Apply dry-run: %v", err)
	}

	if result.Applied {
		t.Fatal("dry-run should not apply changes")
	}

	// Verify main workspace README.md is unchanged
	content, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "Modified by agent") {
		t.Fatal("dry-run should not modify main workspace files")
	}
}

func TestApplySuccessModifiesMain(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "apply-test",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Modify file in worktree
	if err := os.WriteFile(filepath.Join(info.Path, "README.md"), []byte("# Hello World\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	result, err := patchMgr.Apply(context.Background(), PatchApplyRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if !result.Applied {
		t.Fatal("expected patch to be applied")
	}

	// Verify main workspace README.md now has the change
	content, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Hello World") {
		t.Fatalf("main workspace should have updated content, got %q", string(content))
	}
}

func TestDiscardRemovesWorktree(t *testing.T) {
	root, patchMgr, wtID := setupPatchTest(t)

	// Get worktree info to check path
	wtMgr := patchMgr.worktreeMgr
	info, err := wtMgr.Get(context.Background(), wtID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(info.Path); os.IsNotExist(err) {
		t.Fatal("worktree should exist before discard")
	}

	err = patchMgr.Discard(context.Background(), PatchDiscardRequest{
		RepoRoot:   root,
		WorktreeID: wtID,
	})
	if err != nil {
		t.Fatalf("Discard: %v", err)
	}

	// Verify worktree directory is gone
	if _, err := os.Stat(info.Path); !os.IsNotExist(err) {
		t.Fatal("worktree directory should be removed after discard")
	}
}

func TestDiscardDoesNotAffectMain(t *testing.T) {
	root, patchMgr, wtID := setupPatchTest(t)

	// Read main workspace file before discard
	content, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	beforeContent := string(content)

	err = patchMgr.Discard(context.Background(), PatchDiscardRequest{
		RepoRoot:   root,
		WorktreeID: wtID,
	})
	if err != nil {
		t.Fatalf("Discard: %v", err)
	}

	// Verify main workspace file is unchanged
	content, err = os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != beforeContent {
		t.Fatal("discard should not modify main workspace files")
	}
}

func TestBinaryDiffDenied(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	cfg := DefaultGitPatchManagerConfig()
	cfg.AllowBinary = false
	patchMgr := NewGitPatchManager(wtMgr, cfg)

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "binary-test",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Write binary data in worktree
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header bytes
	if err := os.WriteFile(filepath.Join(info.Path, "image.png"), binaryData, 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	// Should have binary flag in summary
	if preview.Summary.HasBinary {
		// Binary detected and not allowed -> should be violation
		foundBinaryViolation := false
		for _, v := range preview.Violations {
			if strings.Contains(v.Reason, "binary") {
				foundBinaryViolation = true
				break
			}
		}
		if !foundBinaryViolation {
			t.Fatal("expected binary violation when allow_binary=false")
		}
	}
}

func TestSensitiveFileViolation(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "sensitive-test",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create a .env file in worktree
	if err := os.WriteFile(filepath.Join(info.Path, ".env"), []byte("SECRET=leaked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	// .env should be a violation
	foundEnvViolation := false
	for _, v := range preview.Violations {
		if v.Path == ".env" {
			foundEnvViolation = true
			break
		}
	}
	if !foundEnvViolation {
		t.Fatalf("expected .env violation, got %v", preview.Violations)
	}
}

// timeNow returns current time for test convenience.
func timeNow() time.Time {
	return time.Now().UTC()
}

// failingUpdateStateManager wraps a WorktreeManager and always returns an
// error from UpdateState. All other methods are delegated.
type failingUpdateStateManager struct {
	worktree.WorktreeManager
	failErr error
}

func (m *failingUpdateStateManager) UpdateState(_ context.Context, _ string, _ worktree.WorktreeState) error {
	return m.failErr
}

func TestApplyNewUntrackedFileSuccess(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "new-file-test",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create a new untracked file in the worktree
	newContent := "brand new file content\nline two\n"
	if err := os.WriteFile(filepath.Join(info.Path, "brand_new.txt"), []byte(newContent), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	// Preview should see the file as added
	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	foundAdded := false
	for _, f := range preview.FilesChanged {
		if f.Path == "brand_new.txt" && f.Status == "added" {
			foundAdded = true
			break
		}
	}
	if !foundAdded {
		t.Fatalf("expected brand_new.txt as added, got %v", preview.FilesChanged)
	}

	// Apply should create the file in main workspace
	result, err := patchMgr.Apply(context.Background(), PatchApplyRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !result.Applied {
		t.Fatal("expected patch to be applied")
	}

	// Verify main workspace has the new file with correct content
	appliedContent, err := os.ReadFile(filepath.Join(root, "brand_new.txt"))
	if err != nil {
		t.Fatalf("new file should exist in main workspace: %v", err)
	}
	// Normalize line endings (git apply may convert LF to CRLF on Windows)
	got := strings.ReplaceAll(string(appliedContent), "\r\n", "\n")
	if got != newContent {
		t.Fatalf("content mismatch: got %q, want %q", got, newContent)
	}
}

func TestPreviewDiffIncludesUntrackedFile(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "diff-new-file",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create a new untracked file in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, "new_file.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	// The diff must contain new-file markers for the untracked file
	if !strings.Contains(preview.Diff, "new file mode") {
		t.Fatal("expected 'new file mode' in diff for untracked file")
	}
	if !strings.Contains(preview.Diff, "/dev/null") {
		t.Fatal("expected '/dev/null' in diff for untracked file")
	}
	if !strings.Contains(preview.Diff, "new_file.txt") {
		t.Fatal("expected 'new_file.txt' in diff")
	}
}

func TestApplyNewSensitiveFileRejected(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "sensitive-new",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create .env in worktree (sensitive new file)
	if err := os.WriteFile(filepath.Join(info.Path, ".env"), []byte("SECRET=leaked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	// Preview should detect violation
	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	foundEnvViolation := false
	for _, v := range preview.Violations {
		if v.Path == ".env" {
			foundEnvViolation = true
			break
		}
	}
	if !foundEnvViolation {
		t.Fatalf("expected .env violation, got %v", preview.Violations)
	}

	// Apply should refuse
	_, err = patchMgr.Apply(context.Background(), PatchApplyRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err == nil {
		t.Fatal("expected error when applying with sensitive new file")
	}
	if !strings.Contains(err.Error(), "violations") {
		t.Fatalf("error = %q, want 'violations'", err.Error())
	}

	// Main workspace must not have .env
	if _, err := os.Stat(filepath.Join(root, ".env")); !os.IsNotExist(err) {
		t.Fatal(".env should not exist in main workspace after rejected apply")
	}
}

func TestApplyNewFileDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "dryrun-new",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create a new untracked file in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, "dry_new.txt"), []byte("dry run content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	result, err := patchMgr.Apply(context.Background(), PatchApplyRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("Apply dry-run: %v", err)
	}
	if result.Applied {
		t.Fatal("dry-run should not apply changes")
	}

	// Main workspace must not have the new file
	if _, err := os.Stat(filepath.Join(root, "dry_new.txt")); !os.IsNotExist(err) {
		t.Fatal("dry_new.txt should not exist in main workspace after dry-run")
	}
}

func TestApplyStateUpdatedToApplied(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "state-test",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Modify a tracked file in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, "README.md"), []byte("# Updated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	result, err := patchMgr.Apply(context.Background(), PatchApplyRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !result.Applied {
		t.Fatal("expected patch to be applied")
	}
	if result.StateUpdateError != "" {
		t.Fatalf("unexpected state update error: %s", result.StateUpdateError)
	}

	// Verify registry shows state=applied
	updated, err := wtMgr.Get(context.Background(), info.ID)
	if err != nil {
		t.Fatalf("Get after apply: %v", err)
	}
	if updated.State != worktree.WorktreeStateApplied {
		t.Fatalf("state = %q, want %q", updated.State, worktree.WorktreeStateApplied)
	}
}

func TestApplyStateUpdateFailureObservable(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())

	// Wrap with a manager that fails UpdateState
	failErr := fmt.Errorf("registry write failed")
	failingMgr := &failingUpdateStateManager{
		WorktreeManager: wtMgr,
		failErr:         failErr,
	}

	patchMgr := NewGitPatchManager(failingMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "state-fail",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Modify a tracked file in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, "README.md"), []byte("# Updated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	result, err := patchMgr.Apply(context.Background(), PatchApplyRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Apply should succeed even if state update fails: %v", err)
	}
	if !result.Applied {
		t.Fatal("patch should be applied even if state update failed")
	}
	if result.StateUpdateError == "" {
		t.Fatal("expected StateUpdateError to be populated when state update fails")
	}
	if !strings.Contains(result.StateUpdateError, "registry write failed") {
		t.Fatalf("StateUpdateError = %q, want to contain 'registry write failed'", result.StateUpdateError)
	}
}

func TestPreviewSensitiveUntrackedFileDoesNotExposeContent(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "secret-untracked",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create .env with secret content in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, ".env"), []byte("SECRET_VALUE=super_secret_data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	// Must have violations
	if len(preview.Violations) == 0 {
		t.Fatal("expected violations for .env")
	}

	// Diff must NOT contain the secret content
	if strings.Contains(preview.Diff, "SECRET_VALUE") {
		t.Fatal("Preview.Diff must not expose content of sensitive files")
	}
	if strings.Contains(preview.Diff, "super_secret_data") {
		t.Fatal("Preview.Diff must not expose content of sensitive files")
	}

	// Diff should be redacted marker
	if !strings.Contains(preview.Diff, "redacted") {
		t.Fatalf("Preview.Diff should contain redacted marker, got %q", preview.Diff)
	}
}

func TestPreviewSafeUntrackedFileStillIncludesDiff(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "safe-new-file",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create a safe new file in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, "safe.txt"), []byte("safe content here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	// No violations for safe file
	if len(preview.Violations) != 0 {
		t.Fatalf("expected no violations for safe file, got %v", preview.Violations)
	}

	// Diff must contain new-file markers
	if !strings.Contains(preview.Diff, "new file mode") {
		t.Fatal("expected 'new file mode' in diff for safe untracked file")
	}
	if !strings.Contains(preview.Diff, "/dev/null") {
		t.Fatal("expected '/dev/null' in diff for safe untracked file")
	}

	// Diff must contain the safe file content
	if !strings.Contains(preview.Diff, "safe content here") {
		t.Fatal("expected safe.txt content in diff")
	}
}

func TestApplySafeUntrackedFileStillWorks(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "apply-safe-new",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create a safe new untracked file in the worktree
	safeContent := "safe apply content\n"
	if err := os.WriteFile(filepath.Join(info.Path, "safe_apply.txt"), []byte(safeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	// Apply should succeed
	result, err := patchMgr.Apply(context.Background(), PatchApplyRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !result.Applied {
		t.Fatal("expected patch to be applied")
	}

	// Verify main workspace has the safe new file
	appliedContent, err := os.ReadFile(filepath.Join(root, "safe_apply.txt"))
	if err != nil {
		t.Fatalf("safe_apply.txt should exist in main workspace: %v", err)
	}
	got := strings.ReplaceAll(string(appliedContent), "\r\n", "\n")
	if got != safeContent {
		t.Fatalf("content mismatch: got %q, want %q", got, safeContent)
	}
}

func TestApplySensitiveUntrackedFileRejected(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "reject-sensitive",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create sensitive files in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, ".env"), []byte("DB_PASSWORD=hunter2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(info.Path, "server.pem"), []byte("-----BEGIN PRIVATE KEY-----\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	// Preview should have violations
	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(preview.Violations) == 0 {
		t.Fatal("expected violations for sensitive files")
	}

	// Apply should refuse
	_, err = patchMgr.Apply(context.Background(), PatchApplyRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err == nil {
		t.Fatal("expected error when applying sensitive files")
	}
	if !strings.Contains(err.Error(), "violations") {
		t.Fatalf("error = %q, want 'violations'", err.Error())
	}

	// Main workspace must not have .env or server.pem
	if _, err := os.Stat(filepath.Join(root, ".env")); !os.IsNotExist(err) {
		t.Fatal(".env should not exist in main workspace")
	}
	if _, err := os.Stat(filepath.Join(root, "server.pem")); !os.IsNotExist(err) {
		t.Fatal("server.pem should not exist in main workspace")
	}
}

func TestPreviewTrackedSensitiveModificationDoesNotExposeContent(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@reasonforge.dev")
	runGit(t, root, "config", "user.name", "Test")

	// Create and commit a tracked .env file
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("OLD_KEY=old_value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial with .env")

	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	patchMgr := NewGitPatchManager(wtMgr, DefaultGitPatchManagerConfig())

	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "tracked-secret-mod",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Modify the tracked .env file in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, ".env"), []byte("NEW_SECRET=leaked_value\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := task.TaskContract{
		ID:        "tc_test",
		Goal:      "test",
		RepoRoot:  root,
		MaxSteps:  5,
		CreatedAt: timeNow(),
	}

	preview, err := patchMgr.Preview(context.Background(), PatchPreviewRequest{
		RepoRoot:   root,
		WorktreeID: info.ID,
		Contract:   contract,
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	// Must have violations for .env
	foundEnvViolation := false
	for _, v := range preview.Violations {
		if v.Path == ".env" {
			foundEnvViolation = true
			break
		}
	}
	if !foundEnvViolation {
		t.Fatalf("expected .env violation, got %v", preview.Violations)
	}

	// Diff must NOT contain the sensitive content
	if strings.Contains(preview.Diff, "NEW_SECRET") {
		t.Fatal("Preview.Diff must not expose content of modified sensitive files")
	}
	if strings.Contains(preview.Diff, "leaked_value") {
		t.Fatal("Preview.Diff must not expose content of modified sensitive files")
	}
	if strings.Contains(preview.Diff, "OLD_KEY") {
		t.Fatal("Preview.Diff must not expose old content of sensitive files either")
	}

	// Diff should be redacted marker
	if !strings.Contains(preview.Diff, "redacted") {
		t.Fatalf("Preview.Diff should contain redacted marker, got %q", preview.Diff)
	}
}
