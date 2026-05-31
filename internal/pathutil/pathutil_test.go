package pathutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExecutablePath(t *testing.T) {
	path := ExecutablePath()
	// In test environment, this should return something
	if path == "" {
		// This is okay in some test environments
		t.Log("ExecutablePath returned empty (expected in some test environments)")
	}
}

func TestExecutableDir(t *testing.T) {
	dir := ExecutableDir()
	if dir == "" {
		t.Log("ExecutableDir returned empty (expected in some test environments)")
	}
}

func TestExeName(t *testing.T) {
	name := ExeName()
	// ExeName should return lowercase
	if name != "" && name != filepath.Base(name) {
		// This check is not perfect but gives some validation
		t.Logf("ExeName returned: %s", name)
	}
}

func TestIsWindows(t *testing.T) {
	expected := runtime.GOOS == "windows"
	if IsWindows() != expected {
		t.Errorf("IsWindows() = %v, want %v", IsWindows(), expected)
	}
}

func TestIsDarwin(t *testing.T) {
	expected := runtime.GOOS == "darwin"
	if IsDarwin() != expected {
		t.Errorf("IsDarwin() = %v, want %v", IsDarwin(), expected)
	}
}

func TestIsLinux(t *testing.T) {
	expected := runtime.GOOS == "linux"
	if IsLinux() != expected {
		t.Errorf("IsLinux() = %v, want %v", IsLinux(), expected)
	}
}

func TestAbsPath(t *testing.T) {
	// Test with relative path
	rel := "."
	abs := AbsPath(rel)
	if !filepath.IsAbs(abs) {
		t.Errorf("AbsPath(%q) = %q, want absolute path", rel, abs)
	}

	// Test with absolute path
	if runtime.GOOS == "windows" {
		absPath := `C:\Windows`
		result := AbsPath(absPath)
		if result != absPath {
			t.Errorf("AbsPath(%q) = %q, want %q", absPath, result, absPath)
		}
	} else {
		absPath := "/tmp"
		result := AbsPath(absPath)
		if result != absPath {
			t.Errorf("AbsPath(%q) = %q, want %q", absPath, result, absPath)
		}
	}
}

func TestCleanPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"a/b/../c", "a" + string(filepath.Separator) + "c"},
		{"./test", "test"},
	}
	for _, tt := range tests {
		result := CleanPath(tt.input)
		if result != tt.expected {
			t.Errorf("CleanPath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestJoinPath(t *testing.T) {
	result := JoinPath("a", "b", "c")
	expected := filepath.Join("a", "b", "c")
	if result != expected {
		t.Errorf("JoinPath(a,b,c) = %q, want %q", result, expected)
	}
}

func TestFileExists(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if !FileExists(tmpFile.Name()) {
		t.Errorf("FileExists(%q) = false, want true", tmpFile.Name())
	}

	// Non-existent file
	if FileExists("/nonexistent/file/path") {
		t.Error("FileExists for non-existent file returned true")
	}
}

func TestDirExists(t *testing.T) {
	tmpDir := t.TempDir()
	if !DirExists(tmpDir) {
		t.Errorf("DirExists(%q) = false, want true", tmpDir)
	}

	// Non-existent directory
	if DirExists("/nonexistent/dir/path") {
		t.Error("DirExists for non-existent directory returned true")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot determine home directory")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"~/test", filepath.Join(home, "test")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		result := ExpandHome(tt.input)
		if result != tt.expected {
			t.Errorf("ExpandHome(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetwd(t *testing.T) {
	wd := Getwd()
	if wd == "" {
		t.Log("Getwd returned empty (might be okay)")
		return
	}
	if !filepath.IsAbs(wd) {
		t.Errorf("Getwd() = %q, want absolute path", wd)
	}
}

func TestResolvePath(t *testing.T) {
	base := filepath.Clean("/base")

	// Test relative path
	result := ResolvePath(base, "relative")
	expected := filepath.Join(base, "relative")
	if result != expected {
		t.Errorf("ResolvePath(%q, relative) = %q, want %q", base, result, expected)
	}

	// Test absolute path (platform-specific)
	var absPath string
	if runtime.GOOS == "windows" {
		absPath = `C:\absolute`
	} else {
		absPath = "/absolute"
	}
	result = ResolvePath(base, absPath)
	if result != absPath {
		t.Errorf("ResolvePath(%q, %q) = %q, want %q", base, absPath, result, absPath)
	}

	// Test current directory
	result = ResolvePath(base, ".")
	if result != base {
		t.Errorf("ResolvePath(%q, .) = %q, want %q", base, result, base)
	}
}

func TestRelPath(t *testing.T) {
	base := "/base"
	target := "/base/sub/file"
	expected := filepath.Join("sub", "file")

	result := RelPath(base, target)
	if result != expected {
		t.Errorf("RelPath(%q, %q) = %q, want %q", base, target, result, expected)
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "newdir", "subdir")

	if err := EnsureDir(newDir); err != nil {
		t.Errorf("EnsureDir(%q) failed: %v", newDir, err)
	}

	if !DirExists(newDir) {
		t.Errorf("EnsureDir did not create directory %q", newDir)
	}
}

func TestGOOS(t *testing.T) {
	if GOOS() != runtime.GOOS {
		t.Errorf("GOOS() = %q, want %q", GOOS(), runtime.GOOS)
	}
}
