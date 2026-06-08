package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	guard := NewSafetyGuard(ToolPolicy{})

	tests := []struct {
		name string
		rel  string
	}{
		{"parent traversal", "../outside"},
		{"nested traversal", "sub/../../outside"},
		{"double dot", ".."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := guard.SafePath(root, tt.rel)
			if err == nil {
				t.Fatalf("SafePath(%q) should reject traversal", tt.rel)
			}
		})
	}
}

func TestSafePathRejectsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	guard := NewSafetyGuard(ToolPolicy{})

	_, err := guard.SafePath(root, "/etc/passwd")
	if err == nil {
		t.Fatal("SafePath should reject absolute Unix path")
	}
}

func TestSafePathRejectsWindowsDriveLetter(t *testing.T) {
	root := t.TempDir()
	guard := NewSafetyGuard(ToolPolicy{})

	_, err := guard.SafePath(root, "C:\\Windows\\System32")
	if err == nil {
		t.Fatal("SafePath should reject Windows drive letter path")
	}
}

func TestSafePathRejectsRepoRootEscape(t *testing.T) {
	root := t.TempDir()
	guard := NewSafetyGuard(ToolPolicy{})

	_, err := guard.SafePath(root, "../../etc/passwd")
	if err == nil {
		t.Fatal("SafePath should reject paths escaping repo root")
	}
}

func TestSafePathAcceptsValidRelative(t *testing.T) {
	root := t.TempDir()
	guard := NewSafetyGuard(ToolPolicy{})

	resolved, err := guard.SafePath(root, "src/main.go")
	if err != nil {
		t.Fatalf("SafePath should accept valid relative path: %v", err)
	}
	expected := filepath.Join(root, "src", "main.go")
	if resolved != expected {
		t.Fatalf("SafePath = %q, want %q", resolved, expected)
	}
}

func TestIsWriteDenied(t *testing.T) {
	guard := NewSafetyGuard(ToolPolicy{
		DenyWritePaths: DefaultDenyWritePaths(),
	})

	tests := []struct {
		path   string
		denied bool
	}{
		{".env", true},
		{".env.local", true},
		{"nested/.env.production", true},
		{"secrets.json", true},
		{".git", true},
		{".mimoneko", true},
		{"server.pem", true},
		{"private.key", true},
		{"id_rsa", true},
		{"id_ed25519", true},
		{"README.md", false},
		{"src/main.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := guard.IsWriteDenied(tt.path)
			if got != tt.denied {
				t.Fatalf("IsWriteDenied(%q) = %v, want %v", tt.path, got, tt.denied)
			}
		})
	}
}

func TestIsReadDenied(t *testing.T) {
	guard := NewSafetyGuard(ToolPolicy{
		DenyReadPaths: DefaultDenyReadPaths(),
	})

	tests := []struct {
		path   string
		denied bool
	}{
		{".env", true},
		{".env.local", true},
		{"nested/.env.production", true},
		{"secrets.json", true},
		{"server.pem", true},
		{"private.key", true},
		{"id_rsa", true},
		{"id_ed25519", true},
		{".git", true},
		{".mimoneko", true},
		{"README.md", false},
		{"src/main.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := guard.IsReadDenied(tt.path)
			if got != tt.denied {
				t.Fatalf("IsReadDenied(%q) = %v, want %v", tt.path, got, tt.denied)
			}
		})
	}
}

func TestIsSensitiveFilePath(t *testing.T) {
	tests := []struct {
		path      string
		sensitive bool
	}{
		{".env", true},
		{"config/.env", true},
		{".env.local", true},
		{"secrets.json", true},
		{"id_rsa", true},
		{"id_ed25519", true},
		{"cert.pem", true},
		{"server.key", true},
		{"README.md", false},
		{"main.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsSensitiveFilePath(tt.path)
			if got != tt.sensitive {
				t.Fatalf("IsSensitiveFilePath(%q) = %v, want %v", tt.path, got, tt.sensitive)
			}
		})
	}
}

func TestIsUnderProtectedDir(t *testing.T) {
	tests := []struct {
		path      string
		protected bool
	}{
		{".git/config", true},
		{".git/HEAD", true},
		{".mimoneko/tools.yaml", true},
		{".git", true},
		{".mimoneko", true},
		{"src/main.go", false},
		{"README.md", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsUnderProtectedDir(tt.path)
			if got != tt.protected {
				t.Fatalf("IsUnderProtectedDir(%q) = %v, want %v", tt.path, got, tt.protected)
			}
		})
	}
}

func TestMaxOutputDefault(t *testing.T) {
	guard := NewSafetyGuard(ToolPolicy{})
	if got := guard.MaxOutput(0); got != DefaultMaxOutputBytes {
		t.Fatalf("MaxOutput(0) = %d, want %d", got, DefaultMaxOutputBytes)
	}
	if got := guard.MaxOutput(100); got != 100 {
		t.Fatalf("MaxOutput(100) = %d, want 100", got)
	}
}

func TestTimeoutDefault(t *testing.T) {
	guard := NewSafetyGuard(ToolPolicy{})
	if got := guard.Timeout(0); got != DefaultTimeoutSeconds {
		t.Fatalf("Timeout(0) = %d, want %d", got, DefaultTimeoutSeconds)
	}
	if got := guard.Timeout(60); got != 60 {
		t.Fatalf("Timeout(60) = %d, want 60", got)
	}
}

func TestSafePathExistingSubdir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	guard := NewSafetyGuard(ToolPolicy{})
	resolved, err := guard.SafePath(root, "subdir/file.txt")
	if err != nil {
		t.Fatalf("SafePath existing subdir: %v", err)
	}
	expected := filepath.Join(root, "subdir", "file.txt")
	if resolved != expected {
		t.Fatalf("SafePath = %q, want %q", resolved, expected)
	}
}
