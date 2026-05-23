package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ToolPolicy defines the security policy for tool execution.
type ToolPolicy struct {
	// MaxOutputBytes caps tool output. 0 means use DefaultMaxOutputBytes.
	MaxOutputBytes int

	// DefaultTimeoutSeconds is the default execution timeout. 0 means use DefaultTimeoutSeconds.
	DefaultTimeoutSeconds int

	// DenyWritePaths are glob patterns for paths that must never be written.
	DenyWritePaths []string

	// DenyReadPaths are glob patterns for paths that must never be read.
	DenyReadPaths []string
}

const (
	DefaultMaxOutputBytes = 65536
	DefaultTimeoutSeconds = 30
	DefaultMaxReadBytes   = 256 * 1024 // 256KB
)

// SafetyGuard enforces workspace root boundaries and sensitive path protections.
type SafetyGuard struct {
	policy ToolPolicy
}

// NewSafetyGuard creates a SafetyGuard with the given policy.
func NewSafetyGuard(policy ToolPolicy) *SafetyGuard {
	return &SafetyGuard{policy: policy}
}

// SafePath joins root and rel, then verifies the result stays within root.
func (g *SafetyGuard) SafePath(root, rel string) (string, error) {
	return safePath(root, rel)
}

// IsWriteDenied checks whether writing to the given repo-relative path is
// prohibited by the deny-write policy.
func (g *SafetyGuard) IsWriteDenied(rel string) bool {
	return matchesAnyGlob(rel, g.policy.DenyWritePaths)
}

// IsReadDenied checks whether reading the given repo-relative path is
// prohibited by the deny-read policy.
func (g *SafetyGuard) IsReadDenied(rel string) bool {
	return matchesAnyGlob(rel, g.policy.DenyReadPaths)
}

// MaxOutput returns the effective max output bytes.
func (g *SafetyGuard) MaxOutput(requested int) int {
	if requested > 0 {
		return requested
	}
	if g.policy.MaxOutputBytes > 0 {
		return g.policy.MaxOutputBytes
	}
	return DefaultMaxOutputBytes
}

// Timeout returns the effective timeout in seconds.
func (g *SafetyGuard) Timeout(requested int) int {
	if requested > 0 {
		return requested
	}
	if g.policy.DefaultTimeoutSeconds > 0 {
		return g.policy.DefaultTimeoutSeconds
	}
	return DefaultTimeoutSeconds
}

// Policy returns the current policy (read-only copy).
func (g *SafetyGuard) Policy() ToolPolicy {
	return g.policy
}

// safePath joins root and rel, then verifies the result stays within root.
// It rejects absolute paths in rel (both Unix / and Windows drive letters)
// and path traversal via "..".
func safePath(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is absolute, must be relative", rel)
	}
	if len(rel) > 0 && rel[0] == '/' {
		return "", fmt.Errorf("path %q starts with /, must be relative", rel)
	}
	if len(rel) >= 2 && rel[1] == ':' && ((rel[0] >= 'a' && rel[0] <= 'z') || (rel[0] >= 'A' && rel[0] <= 'Z')) {
		return "", fmt.Errorf("path %q contains a Windows drive letter, must be relative", rel)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	joined := filepath.Join(absRoot, rel)
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if !strings.HasPrefix(absJoined, absRoot+string(os.PathSeparator)) && absJoined != absRoot {
		return "", fmt.Errorf("path %q escapes root %q", rel, absRoot)
	}
	return absJoined, nil
}

// matchesAnyGlob checks if rel matches any of the glob patterns.
// Patterns are evaluated against the forward-slash form of rel to
// keep behaviour consistent across platforms.
func matchesAnyGlob(rel string, patterns []string) bool {
	normalized := filepath.ToSlash(rel)
	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, normalized)
		if matched {
			return true
		}
		// If pattern has no slash, also match against the basename
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

// DefaultDenyWritePaths returns the built-in deny-write patterns.
func DefaultDenyWritePaths() []string {
	return []string{
		".git",
		".reasonforge",
		".env",
		"*.pem",
		"*.key",
		"id_rsa",
		"id_ed25519",
	}
}

// DefaultDenyReadPaths returns the built-in deny-read patterns.
func DefaultDenyReadPaths() []string {
	return []string{
		".git",
		".reasonforge",
		".env",
		"*.pem",
		"*.key",
		"id_rsa",
		"id_ed25519",
	}
}

// IsSensitiveFilePath checks if a relative path looks like a sensitive file.
func IsSensitiveFilePath(rel string) bool {
	normalized := filepath.ToSlash(rel)
	parts := strings.Split(normalized, "/")
	base := parts[len(parts)-1]

	sensitiveBases := map[string]bool{
		".env":       true,
		"id_rsa":     true,
		"id_ed25519": true,
	}
	if sensitiveBases[base] {
		return true
	}

	sensitiveExts := []string{".pem", ".key"}
	for _, ext := range sensitiveExts {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}
	return false
}

// IsUnderProtectedDir checks if a relative path is under a protected directory.
func IsUnderProtectedDir(rel string) bool {
	normalized := filepath.ToSlash(rel)
	protectedDirs := []string{".git/", ".reasonforge/"}
	for _, dir := range protectedDirs {
		if strings.HasPrefix(normalized, dir) {
			return true
		}
		if normalized == dir[:len(dir)-1] {
			return true
		}
	}
	return false
}
