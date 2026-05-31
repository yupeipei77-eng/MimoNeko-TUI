package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupGitRepo creates a temporary git repository for testing.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Init git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, string(output))
	}

	// Configure git user
	cmd = exec.Command("git", "config", "user.email", "test@NekoMIMO.dev")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config email: %v: %s", err, string(output))
	}
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config name: %v: %s", err, string(output))
	}

	// Create initial commit
	testFile := filepath.Join(root, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, string(output))
	}
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, string(output))
	}

	return root
}

// setupManager creates a GitWorktreeManager for testing.
func setupManager(t *testing.T, repoRoot string) *GitWorktreeManager {
	t.Helper()
	registryPath := DefaultRegistryPath(repoRoot)
	registry, err := NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	cfg := DefaultGitWorktreeManagerConfig()
	return NewGitWorktreeManager(registry, cfg)
}

func TestCreateWorktree(t *testing.T) {
	root := setupGitRepo(t)
	mgr := setupManager(t, root)

	info, err := mgr.Create(context.Background(), CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "test-task-1",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if info.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if !strings.HasPrefix(info.ID, "wt_") {
		t.Fatalf("ID should start with wt_, got %q", info.ID)
	}
	if info.TaskID != "test-task-1" {
		t.Fatalf("TaskID = %q, want test-task-1", info.TaskID)
	}
	if info.RepoRoot != root {
		t.Fatalf("RepoRoot = %q, want %q", info.RepoRoot, root)
	}
	if info.State != WorktreeStateActive {
		t.Fatalf("State = %q, want active", info.State)
	}
	if !strings.HasPrefix(info.Branch, "NekoMIMO/test-task-1/") {
		t.Fatalf("Branch = %q, want prefix NekoMIMO/test-task-1/", info.Branch)
	}

	// Worktree directory should exist
	if _, err := os.Stat(info.Path); os.IsNotExist(err) {
		t.Fatalf("worktree path %q does not exist", info.Path)
	}

	// Worktree path should be under .nekonomimo/worktrees
	if !IsWorktreePathSafe(root, info.Path) {
		t.Fatalf("worktree path %q is not under .nekonomimo/worktrees", info.Path)
	}
}

func TestCreateWorktreeRequiresGitRepo(t *testing.T) {
	root := t.TempDir() // not a git repo
	registryPath := filepath.Join(root, ".nekonomimo", "worktrees", "registry.jsonl")
	registry, err := NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer registry.Close()

	mgr := NewGitWorktreeManager(registry, DefaultGitWorktreeManagerConfig())
	_, err = mgr.Create(context.Background(), CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "test-task",
	})
	if err == nil {
		t.Fatal("expected error for non-git repo")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("error = %q, want 'not a git repository'", err.Error())
	}
}

func TestWorktreePathUnderDotNekoMIMO(t *testing.T) {
	root := setupGitRepo(t)
	mgr := setupManager(t, root)

	info, err := mgr.Create(context.Background(), CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "path-test",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	absPath, _ := filepath.Abs(info.Path)
	absExpected, _ := filepath.Abs(filepath.Join(root, ".nekonomimo", "worktrees"))
	if !strings.HasPrefix(absPath, absExpected+string(os.PathSeparator)) {
		t.Fatalf("path %q is not under %q", absPath, absExpected)
	}
}

func TestTaskIDPathTraversal(t *testing.T) {
	root := setupGitRepo(t)
	mgr := setupManager(t, root)

	tests := []struct {
		name   string
		taskID string
	}{
		{name: "dotdot", taskID: "../etc/passwd"},
		{name: "mixed", taskID: "foo/../../etc"},
		{name: "absolute", taskID: "/etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := mgr.Create(context.Background(), CreateWorktreeRequest{
				RepoRoot: root,
				TaskID:   tt.taskID,
			})
			if err != nil {
				// If creation fails, that's fine (path traversal prevented)
				return
			}
			// If creation succeeds, verify path is still safe
			if !IsWorktreePathSafe(root, info.Path) {
				t.Fatalf("path traversal detected: %q is not safe", info.Path)
			}
		})
	}
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input     string
		want      string
		wantError bool
	}{
		{input: "simple-task", want: "simple-task", wantError: false},
		{input: "task with spaces", want: "task_with_spaces", wantError: false},
		{input: "task/with/slashes", want: "task_with_slashes", wantError: false},
		{input: "../etc/passwd", want: "etc_passwd", wantError: false},
		{input: "", want: "", wantError: true},
		{input: "...", want: "", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := SanitizeID(tt.input, 64)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("SanitizeID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		input     string
		want      string
		wantError bool
	}{
		{input: "my-task", want: "my-task", wantError: false},
		{input: "my task", want: "my-task", wantError: false},
		{input: "task--name", want: "task-name", wantError: false},
		{input: "", want: "", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := SanitizeBranchName(tt.input, 80)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("SanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRemoveWorktree(t *testing.T) {
	root := setupGitRepo(t)
	mgr := setupManager(t, root)

	info, err := mgr.Create(context.Background(), CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "remove-test",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(info.Path); os.IsNotExist(err) {
		t.Fatal("worktree should exist before removal")
	}

	// Remove
	if err := mgr.Remove(context.Background(), info.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Verify worktree directory is gone
	if _, err := os.Stat(info.Path); !os.IsNotExist(err) {
		t.Fatal("worktree directory should be removed")
	}

	// Verify state is discarded
	updated, err := mgr.Get(context.Background(), info.ID)
	if err != nil {
		t.Fatalf("Get after Remove: %v", err)
	}
	if updated.State != WorktreeStateDiscarded {
		t.Fatalf("State = %q, want discarded", updated.State)
	}
}

func TestRemoveOnlyRegistryWorktrees(t *testing.T) {
	root := setupGitRepo(t)
	mgr := setupManager(t, root)

	// Try to remove a worktree not in the registry
	err := mgr.Remove(context.Background(), "wt_nonexistent")
	if err == nil {
		t.Fatal("expected error when removing non-registry worktree")
	}
	if !strings.Contains(err.Error(), "not found in registry") {
		t.Fatalf("error = %q, want 'not found in registry'", err.Error())
	}
}

func TestListWorktrees(t *testing.T) {
	root := setupGitRepo(t)
	mgr := setupManager(t, root)

	// Create two worktrees
	info1, err := mgr.Create(context.Background(), CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "list-test-1",
	})
	if err != nil {
		t.Fatalf("Create 1: %v", err)
	}

	info2, err := mgr.Create(context.Background(), CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "list-test-2",
	})
	if err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	// List
	worktrees, err := mgr.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	ids := make(map[string]bool)
	for _, wt := range worktrees {
		ids[wt.ID] = true
	}

	if !ids[info1.ID] {
		t.Fatalf("worktree %q not found in list", info1.ID)
	}
	if !ids[info2.ID] {
		t.Fatalf("worktree %q not found in list", info2.ID)
	}
}

func TestGetWorktree(t *testing.T) {
	root := setupGitRepo(t)
	mgr := setupManager(t, root)

	info, err := mgr.Create(context.Background(), CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "get-test",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := mgr.Get(context.Background(), info.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != info.ID {
		t.Fatalf("ID = %q, want %q", got.ID, info.ID)
	}
	if got.TaskID != info.TaskID {
		t.Fatalf("TaskID = %q, want %q", got.TaskID, info.TaskID)
	}
}

func TestGetNonexistentWorktree(t *testing.T) {
	root := setupGitRepo(t)
	mgr := setupManager(t, root)

	_, err := mgr.Get(context.Background(), "wt_nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent worktree")
	}
}

func TestUpdateState(t *testing.T) {
	root := setupGitRepo(t)
	mgr := setupManager(t, root)

	info, err := mgr.Create(context.Background(), CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "state-test",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := mgr.UpdateState(context.Background(), info.ID, WorktreeStateFailed); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}

	got, err := mgr.Get(context.Background(), info.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.State != WorktreeStateFailed {
		t.Fatalf("State = %q, want failed", got.State)
	}
}

func TestRegistryPermissions(t *testing.T) {
	// Unix-style permissions are not enforced on Windows
	if os.PathSeparator == '\\' {
		t.Skip("skipping permission test on Windows")
	}

	root := t.TempDir()
	registryPath := filepath.Join(root, ".nekonomimo", "worktrees", "registry.jsonl")

	registry, err := NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer registry.Close()

	// Verify directory permissions
	dir := filepath.Dir(registryPath)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("directory permissions = %o, want 0700", info.Mode().Perm())
	}

	// Verify file permissions
	fileInfo, err := os.Stat(registryPath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("file permissions = %o, want 0600", fileInfo.Mode().Perm())
	}
}

func TestRegistryAppendOnly(t *testing.T) {
	root := t.TempDir()
	registryPath := filepath.Join(root, "registry.jsonl")

	registry, err := NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	info1 := WorktreeInfo{
		ID:        "wt_001",
		TaskID:    "task-1",
		RepoRoot:  "/tmp/repo",
		Path:      "/tmp/repo/.nekonomimo/worktrees/task-1/wt_001",
		Branch:    "NekoMIMO/task-1/001",
		BaseRef:   "HEAD",
		CreatedAt: time.Now().UTC(),
		State:     WorktreeStateActive,
	}

	if err := registry.Record(info1); err != nil {
		t.Fatalf("Record 1: %v", err)
	}

	info2 := WorktreeInfo{
		ID:        "wt_002",
		TaskID:    "task-2",
		RepoRoot:  "/tmp/repo",
		Path:      "/tmp/repo/.nekonomimo/worktrees/task-2/wt_002",
		Branch:    "NekoMIMO/task-2/002",
		BaseRef:   "HEAD",
		CreatedAt: time.Now().UTC(),
		State:     WorktreeStateActive,
	}

	if err := registry.Record(info2); err != nil {
		t.Fatalf("Record 2: %v", err)
	}

	// Load and verify both entries exist
	entries, err := registry.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if _, ok := entries["wt_001"]; !ok {
		t.Fatal("wt_001 not found")
	}
	if _, ok := entries["wt_002"]; !ok {
		t.Fatal("wt_002 not found")
	}

	registry.Close()
}

func TestRegistryNoAPIKeys(t *testing.T) {
	root := t.TempDir()
	registryPath := filepath.Join(root, "registry.jsonl")

	registry, err := NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer registry.Close()

	info := WorktreeInfo{
		ID:       "wt_001",
		TaskID:   "task-1",
		RepoRoot: "/tmp/repo",
		Metadata: map[string]string{
			"source":  "cli",
			"api_key": "sk-secret-key-12345",
			"goal":    "fix bug",
		},
		State: WorktreeStateActive,
	}

	if err := registry.Record(info); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Read file directly and verify API key is redacted
	data, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "sk-secret-key-12345") {
		t.Fatal("registry should not contain API keys")
	}
	// json.Marshal escapes < as \u003c, so check for both forms
	if !strings.Contains(content, "<redacted>") && !strings.Contains(content, "u003credacted") {
		t.Fatal("registry should redact sensitive metadata")
	}
	if !strings.Contains(content, "source") && !strings.Contains(content, "cli") {
		t.Fatal("registry should keep safe metadata")
	}
}

func TestIsPathTraversal(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"../etc/passwd", true},
		{"foo/../../etc", true},
		{"/absolute/path", true},
		{"~/home", true},
		{"normal/path", false},
		{"simple-file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsPathTraversal(tt.path)
			if got != tt.want {
				t.Fatalf("IsPathTraversal(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsWorktreePathSafe(t *testing.T) {
	root := t.TempDir()

	safe := filepath.Join(root, ".nekonomimo", "worktrees", "task-1", "wt_001")
	if !IsWorktreePathSafe(root, safe) {
		t.Fatalf("expected %q to be safe", safe)
	}

	unsafe := filepath.Join(root, "etc", "passwd")
	if IsWorktreePathSafe(root, unsafe) {
		t.Fatalf("expected %q to be unsafe", unsafe)
	}
}

func TestMaxActiveWorktrees(t *testing.T) {
	root := setupGitRepo(t)
	registryPath := DefaultRegistryPath(root)
	registry, err := NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })

	cfg := DefaultGitWorktreeManagerConfig()
	cfg.MaxActive = 2
	mgr := NewGitWorktreeManager(registry, cfg)

	// Create 2 worktrees (should succeed)
	for i := 0; i < 2; i++ {
		_, err := mgr.Create(context.Background(), CreateWorktreeRequest{
			RepoRoot: root,
			TaskID:   "max-test-" + string(rune('A'+i)),
		})
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	// 3rd should fail
	_, err = mgr.Create(context.Background(), CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "max-test-C",
	})
	if err == nil {
		t.Fatal("expected error when exceeding max_active")
	}
	if !strings.Contains(err.Error(), "max_active") {
		t.Fatalf("error = %q, want max_active", err.Error())
	}
}
