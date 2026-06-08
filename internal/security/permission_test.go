package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPermissionModeDefaultsToPatchPreview(t *testing.T) {
	t.Setenv(PermissionModeEnvVar, "")
	if got := GetPermissionMode(); got != PermissionPatchPreview {
		t.Fatalf("GetPermissionMode() = %q, want %q", got, PermissionPatchPreview)
	}

	t.Setenv(PermissionModeEnvVar, "not-a-mode")
	if got := GetPermissionMode(); got != PermissionPatchPreview {
		t.Fatalf("invalid GetPermissionMode() = %q, want %q", got, PermissionPatchPreview)
	}
}

func TestCheckProjectWritePathRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	linkPath := filepath.Join(root, "linked-outside")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	target := filepath.Join(linkPath, "created-through-link.txt")
	if err := CheckProjectWritePath(root, target); err == nil {
		t.Fatalf("CheckProjectWritePath(%q) should reject symlink escape", target)
	}
}

func TestPermissionModeWriteRequiresApproval(t *testing.T) {
	if PermissionPatchPreview.AllowsDirectWrite(true) {
		t.Fatal("patch-preview must not allow direct writes")
	}
	if PermissionApplyWithApproval.AllowsDirectWrite(false) {
		t.Fatal("apply-with-approval must require explicit approval")
	}
	if !PermissionApplyWithApproval.AllowsDirectWrite(true) {
		t.Fatal("apply-with-approval with approval should allow direct writes")
	}
}

func TestCheckProjectWritePath(t *testing.T) {
	root := t.TempDir()

	for _, rel := range []string{"notes/out.txt", "src/main.go"} {
		if err := CheckProjectWritePath(root, rel); err != nil {
			t.Fatalf("CheckProjectWritePath(%q) error = %v", rel, err)
		}
	}

	denied := []string{
		".git/config",
		".env",
		".env.local",
		"id_rsa",
		"nested/id_ed25519",
		"secrets.json",
		filepath.Join(root, "..", "outside.txt"),
	}
	for _, target := range denied {
		if err := CheckProjectWritePath(root, target); err == nil {
			t.Fatalf("CheckProjectWritePath(%q) should be denied", target)
		}
	}
}
