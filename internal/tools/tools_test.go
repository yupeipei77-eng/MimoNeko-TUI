package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileReadNormal(t *testing.T) {
	root := t.TempDir()
	content := "hello world"
	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &FileReadTool{}
	resp, err := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "test.txt"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}
	if resp.Stdout != content {
		t.Fatalf("Stdout = %q, want %q", resp.Stdout, content)
	}
}

func TestFileReadPathTraversal(t *testing.T) {
	root := t.TempDir()
	tool := &FileReadTool{}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "../../etc/passwd"},
	})
	if resp.Success {
		t.Fatal("file_read should reject path traversal")
	}
}

func TestFileReadRepoRootEscape(t *testing.T) {
	root := t.TempDir()
	tool := &FileReadTool{}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "../outside.txt"},
	})
	if resp.Success {
		t.Fatal("file_read should reject repo root escape")
	}
}

func TestFileReadSensitiveFile(t *testing.T) {
	root := t.TempDir()
	tool := &FileReadTool{}

	// .env
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": ".env"},
	})
	if resp.Success {
		t.Fatal("file_read should reject .env")
	}

	// *.pem
	resp, _ = tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "server.pem"},
	})
	if resp.Success {
		t.Fatal("file_read should reject *.pem")
	}

	// id_rsa
	resp, _ = tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "id_rsa"},
	})
	if resp.Success {
		t.Fatal("file_read should reject id_rsa")
	}
}

func TestFileReadRejectsGitDir(t *testing.T) {
	root := t.TempDir()
	tool := &FileReadTool{}

	// .git/config
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": ".git/config"},
	})
	if resp.Success {
		t.Fatal("file_read should reject .git/config")
	}

	// .git itself
	resp, _ = tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": ".git"},
	})
	if resp.Success {
		t.Fatal("file_read should reject .git")
	}
}

func TestFileReadRejectsMimoNekoDir(t *testing.T) {
	root := t.TempDir()
	tool := &FileReadTool{}

	// .mimoneko/logs/tools.jsonl
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": ".mimoneko/logs/tools.jsonl"},
	})
	if resp.Success {
		t.Fatal("file_read should reject .mimoneko/logs/tools.jsonl")
	}

	// .mimoneko itself
	resp, _ = tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": ".mimoneko"},
	})
	if resp.Success {
		t.Fatal("file_read should reject .mimoneko")
	}
}

func TestFileReadTruncation(t *testing.T) {
	root := t.TempDir()
	largeContent := strings.Repeat("x", 300*1024) // 300KB
	if err := os.WriteFile(filepath.Join(root, "large.txt"), []byte(largeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &FileReadTool{}
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "large.txt"},
	})
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}
	if !resp.Truncated {
		t.Fatal("large file should be truncated")
	}
	if len(resp.Stdout) > DefaultMaxReadBytes {
		t.Fatalf("Stdout len = %d, want <= %d", len(resp.Stdout), DefaultMaxReadBytes)
	}
}

func TestFileReadMissingPath(t *testing.T) {
	root := t.TempDir()
	tool := &FileReadTool{}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{},
	})
	if resp.Success {
		t.Fatal("file_read should require path arg")
	}
}

func TestListFilesNormal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "tools", "tool.go"), []byte("package tools"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &ListFilesTool{}
	resp, err := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": ".", "max_depth": "2"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}
	for _, want := range []string{"README.md", "internal/"} {
		if !strings.Contains(resp.Stdout, want) {
			t.Fatalf("list_files output = %q, want %q", resp.Stdout, want)
		}
	}
}

func TestListFilesSkipsSensitiveAndProtectedEntries(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".git", ".mimoneko/logs"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		"visible.txt":                "ok",
		".env":                       "secret",
		"id_rsa":                     "secret",
		".git/config":                "secret",
		".mimoneko/logs/tools.jsonl": "secret",
		"nested/visible_child.txt":   "ok",
		"nested/.env.local":          "secret",
		"nested/id_ed25519":          "secret",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tool := &ListFilesTool{}
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": ".", "max_depth": "3"},
	})
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}
	for _, want := range []string{"visible.txt", "nested/"} {
		if !strings.Contains(resp.Stdout, want) {
			t.Fatalf("list_files output = %q, want %q", resp.Stdout, want)
		}
	}
	for _, forbidden := range []string{".env", "id_rsa", ".git", ".mimoneko", "id_ed25519"} {
		if strings.Contains(resp.Stdout, forbidden) {
			t.Fatalf("list_files output = %q, should not contain %q", resp.Stdout, forbidden)
		}
	}
}

func TestListFilesRejectsProtectedTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	tool := &ListFilesTool{}
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": ".git"},
	})
	if resp.Success {
		t.Fatal("list_files should reject protected target")
	}
}

func TestFileWriteNormal(t *testing.T) {
	root := t.TempDir()
	tool := &FileWriteTool{}

	resp, err := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "output.txt", "content": "hello"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}

	data, err := os.ReadFile(filepath.Join(root, "output.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("file content = %q, want hello", string(data))
	}
}

func TestFileWriteDryRun(t *testing.T) {
	root := t.TempDir()
	tool := &FileWriteTool{}

	resp, err := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "dry.txt", "content": "test"},
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}

	// File should NOT exist
	if _, err := os.Stat(filepath.Join(root, "dry.txt")); !os.IsNotExist(err) {
		t.Fatal("dry-run should not write file")
	}
}

func TestFileWriteCreateDirs(t *testing.T) {
	root := t.TempDir()
	tool := &FileWriteTool{}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args: map[string]string{
			"path":        "sub/dir/output.txt",
			"content":     "nested",
			"create_dirs": "true",
		},
	})
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}

	data, err := os.ReadFile(filepath.Join(root, "sub", "dir", "output.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "nested" {
		t.Fatalf("file content = %q, want nested", string(data))
	}
}

func TestFileWriteRejectsGitDir(t *testing.T) {
	root := t.TempDir()
	tool := &FileWriteTool{}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": ".git/config", "content": "evil"},
	})
	if resp.Success {
		t.Fatal("file_write should reject .git writes")
	}
}

func TestFileWriteRejectsEnvFile(t *testing.T) {
	root := t.TempDir()
	tool := &FileWriteTool{}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": ".env", "content": "KEY=secret"},
	})
	if resp.Success {
		t.Fatal("file_write should reject .env writes")
	}
}

func TestFileWriteRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	tool := &FileWriteTool{}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "../outside.txt", "content": "evil"},
	})
	if resp.Success {
		t.Fatal("file_write should reject path traversal")
	}
}

func TestFilePatchSingleReplace(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte("foo bar baz"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &FilePatchTool{}
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "code.go", "old": "bar", "new": "BAR"},
	})
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}

	data, err := os.ReadFile(filepath.Join(root, "code.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "foo BAR baz" {
		t.Fatalf("file content = %q, want %q", string(data), "foo BAR baz")
	}
}

func TestFilePatchOldNotFound(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte("foo bar"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &FilePatchTool{}
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "code.go", "old": "notfound", "new": "X"},
	})
	if resp.Success {
		t.Fatal("file_patch should fail when old string not found")
	}
}

func TestFilePatchOldMultiple(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte("foo foo foo"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &FilePatchTool{}
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "code.go", "old": "foo", "new": "X"},
	})
	if resp.Success {
		t.Fatal("file_patch should fail when old string appears multiple times")
	}
}

func TestFilePatchDryRun(t *testing.T) {
	root := t.TempDir()
	original := "foo bar baz"
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &FilePatchTool{}
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "code.go", "old": "bar", "new": "BAR"},
		DryRun:   true,
	})
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}

	// File should NOT be modified
	data, err := os.ReadFile(filepath.Join(root, "code.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatal("dry-run should not modify file")
	}
}

func TestFilePatchRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	tool := &FilePatchTool{}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "../outside.go", "old": "a", "new": "b"},
	})
	if resp.Success {
		t.Fatal("file_patch should reject path traversal")
	}
}

func TestGitDiffNormal(t *testing.T) {
	root := t.TempDir()
	tool := &GitDiffTool{}

	// Initialize git repo
	initGitRepo(t, root)

	// Create a file and stage it
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{},
	})
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}
	// Output may be empty if nothing is staged, that's fine
}

func TestGitDiffNoChanges(t *testing.T) {
	root := t.TempDir()
	tool := &GitDiffTool{}

	initGitRepo(t, root)

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{},
	})
	if !resp.Success {
		t.Fatalf("Run() success=false, error=%q", resp.Error)
	}
	// Empty output is expected when there are no changes
	if resp.Stdout != "" {
		t.Logf("git_diff output: %q", resp.Stdout)
	}
}

func TestGitDiffPathTraversal(t *testing.T) {
	root := t.TempDir()
	tool := &GitDiffTool{}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "../../etc"},
	})
	if resp.Success {
		t.Fatal("git_diff should reject path traversal")
	}
}

func TestGitDiffPathFilterSuccess(t *testing.T) {
	root := t.TempDir()
	tool := &GitDiffTool{}

	// Initialize git repo
	initGitRepo(t, root)

	// Create two files
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello from a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("hello from b"), 0o644); err != nil {
		t.Fatal(err)
	}

	// git diff without path filter
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{},
	})
	if !resp.Success {
		t.Fatalf("git_diff without path filter failed: %q", resp.Error)
	}

	// git diff with path filter for a.txt only
	resp, _ = tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "a.txt"},
	})
	if !resp.Success {
		t.Fatalf("git_diff with path filter failed: %q", resp.Error)
	}

	// Output should contain a.txt reference and should NOT contain b.txt
	if resp.Stdout != "" {
		if !strings.Contains(resp.Stdout, "a.txt") {
			t.Fatalf("git_diff --path a.txt output should mention a.txt, got: %q", resp.Stdout)
		}
		if strings.Contains(resp.Stdout, "b.txt") {
			t.Fatalf("git_diff --path a.txt output should not mention b.txt, got: %q", resp.Stdout)
		}
	}

	// Path traversal via path filter should still fail
	resp, _ = tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"path": "../../etc/passwd"},
	})
	if resp.Success {
		t.Fatal("git_diff should reject path traversal in path filter")
	}
}

func TestTestRunOnlyConfiguredCommand(t *testing.T) {
	root := t.TempDir()
	tool := &TestRunTool{
		Commands: map[string]TestCommandDef{
			"go-test": {
				Command:        []string{"go", "test", "./..."},
				TimeoutSeconds: 120,
			},
		},
	}

	// Configured command should work (may fail if not a Go project, but should not be rejected)
	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"command_name": "go-test"},
	})
	// We don't check success because there's no go code to test
	// but we check it didn't reject the command name
	if resp.Error != "" && strings.Contains(resp.Error, "not configured") {
		t.Fatal("go-test should be a configured command")
	}
}

func TestTestRunUnconfiguredCommand(t *testing.T) {
	root := t.TempDir()
	tool := &TestRunTool{
		Commands: map[string]TestCommandDef{},
	}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{"command_name": "arbitrary-command"},
	})
	if resp.Success {
		t.Fatal("test_run should reject unconfigured command")
	}
	if !strings.Contains(resp.Error, "not configured") {
		t.Fatalf("error = %q, want 'not configured'", resp.Error)
	}
}

func TestTestRunMissingCommandName(t *testing.T) {
	root := t.TempDir()
	tool := &TestRunTool{}

	resp, _ := tool.Run(context.Background(), ToolRequest{
		RepoRoot: root,
		Args:     map[string]string{},
	})
	if resp.Success {
		t.Fatal("test_run should require command_name arg")
	}
}

func TestToolResponseOutputTruncation(t *testing.T) {
	resp := ToolResponse{
		Success: true,
		Stdout:  strings.Repeat("x", 100),
		Stderr:  strings.Repeat("y", 100),
	}
	truncated := truncateResponse(resp, 50)
	if !truncated.Truncated {
		t.Fatal("expected truncation")
	}
	total := len(truncated.Stdout) + len(truncated.Stderr)
	if total > 50 {
		t.Fatalf("total output = %d, want <= 50", total)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.email", "test@test.com")
	runCmd(t, dir, "git", "config", "user.name", "Test")
}

func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("run %s %v: %v", name, args, err)
	}
}
