package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PermissionMode is the coarse-grained runtime permission profile used by
// TUI and agent entry points.
type PermissionMode string

const (
	PermissionChat              PermissionMode = "chat"
	PermissionReadOnly          PermissionMode = "read-only"
	PermissionPlan              PermissionMode = "plan"
	PermissionPatchPreview      PermissionMode = "patch-preview"
	PermissionApplyWithApproval PermissionMode = "apply-with-approval"
)

const DefaultPermissionMode = PermissionPatchPreview
const PermissionModeEnvVar = "MIMONEKO_PERMISSION_MODE"

// GetPermissionMode returns the configured permission mode. Invalid values
// fall back to the safe patch-preview default.
func GetPermissionMode() PermissionMode {
	mode := PermissionMode(strings.TrimSpace(os.Getenv(PermissionModeEnvVar)))
	if ValidatePermissionMode(string(mode)) {
		return mode
	}
	return DefaultPermissionMode
}

func ValidatePermissionMode(mode string) bool {
	switch PermissionMode(strings.TrimSpace(mode)) {
	case PermissionChat, PermissionReadOnly, PermissionPlan, PermissionPatchPreview, PermissionApplyWithApproval:
		return true
	default:
		return false
	}
}

func (m PermissionMode) AllowsShell() bool {
	switch m {
	case PermissionReadOnly, PermissionPlan, PermissionPatchPreview, PermissionApplyWithApproval:
		return true
	default:
		return false
	}
}

func (m PermissionMode) AllowsDirectWrite(approved bool) bool {
	return m == PermissionApplyWithApproval && approved
}

func (m PermissionMode) AllowsPatchApply(approved bool) bool {
	return m == PermissionApplyWithApproval && approved
}

func (m PermissionMode) AllowsPatchPreview() bool {
	switch m {
	case PermissionPatchPreview, PermissionApplyWithApproval:
		return true
	default:
		return false
	}
}

// CheckProjectWritePath validates that a write target stays within the project
// and does not hit hard-denied secret or VCS paths.
func CheckProjectWritePath(projectRoot, target string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("empty write path")
	}
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}
	joined := target
	if !filepath.IsAbs(joined) {
		joined = filepath.Join(root, joined)
	}
	absTarget, err := filepath.Abs(joined)
	if err != nil {
		return fmt.Errorf("resolve write path: %w", err)
	}
	rootReal := resolveExistingPath(root)
	resolvedTarget := resolveWriteTarget(absTarget)
	if !pathWithin(rootReal, resolvedTarget) {
		return fmt.Errorf("write path %q is outside project root", target)
	}
	rel, err := filepath.Rel(rootReal, resolvedTarget)
	if err != nil {
		return fmt.Errorf("resolve relative write path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if IsHardDeniedWritePath(rel) {
		return fmt.Errorf("write path %q is protected", rel)
	}
	return nil
}

func resolveExistingPath(path string) string {
	if evaluated, err := filepath.EvalSymlinks(path); err == nil {
		if abs, err := filepath.Abs(evaluated); err == nil {
			return abs
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

func resolveWriteTarget(absTarget string) string {
	if evaluated, err := filepath.EvalSymlinks(absTarget); err == nil {
		return resolveExistingPath(evaluated)
	}
	parent := filepath.Dir(absTarget)
	if evaluatedParent, err := filepath.EvalSymlinks(parent); err == nil {
		return resolveExistingPath(filepath.Join(evaluatedParent, filepath.Base(absTarget)))
	}
	return resolveExistingPath(absTarget)
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

func IsHardDeniedWritePath(rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(strings.TrimSpace(rel)))
	normalized = strings.TrimPrefix(normalized, "./")
	base := filepath.Base(normalized)
	if normalized == ".git" || strings.HasPrefix(normalized, ".git/") {
		return true
	}
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return true
	}
	switch base {
	case "id_rsa", "id_ed25519":
		return true
	}
	if strings.HasPrefix(base, "secrets.") {
		return true
	}
	return false
}
