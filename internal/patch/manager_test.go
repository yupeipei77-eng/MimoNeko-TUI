package patch

import (
	"context"
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
