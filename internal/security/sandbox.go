// Package security provides path sandbox detection for MioNeko.
//
// This file implements DETECTION ONLY. It does NOT:
//   - Block or deny tool execution
//   - Interrupt runtime behavior
//   - Enforce approval policies
//
// All functions return detection results; tool execution is never modified.
package security

import (
	"path/filepath"
	"strings"
)

// ViolationSeverity represents the severity level of a path violation.
type ViolationSeverity string

const (
	SeverityInfo     ViolationSeverity = "info"
	SeverityWarning  ViolationSeverity = "warning"
	SeverityCritical ViolationSeverity = "critical"
)

// PathViolation represents a detected path violation.
type PathViolation struct {
	Path      string            `json:"path"`
	Rule      string            `json:"rule"`
	Severity  ViolationSeverity `json:"severity"`
	Candidate bool              `json:"candidate"`
}

// sensitivePatterns defines patterns that indicate sensitive paths.
// Each pattern has a rule name and severity level.
type sensitivePattern struct {
	pattern  string
	rule     string
	severity ViolationSeverity
}

// sensitiveRules is the list of sensitive path patterns.
var sensitiveRules = []sensitivePattern{
	// Git directory
	{".git", "git-directory", SeverityCritical},
	{".git/", "git-directory", SeverityCritical},

	// Environment files
	{".env", "env-file", SeverityCritical},
	{".env.", "env-file-variant", SeverityCritical},

	// SSH keys
	{".ssh", "ssh-directory", SeverityCritical},
	{"id_rsa", "ssh-private-key", SeverityCritical},
	{"id_ed25519", "ssh-private-key", SeverityCritical},
	{"id_dsa", "ssh-private-key", SeverityCritical},
	{"id_ecdsa", "ssh-private-key", SeverityCritical},

	// Credential files
	{"credentials", "credentials-file", SeverityWarning},
	{".credentials", "credentials-file", SeverityWarning},
	{"credentials.json", "credentials-file", SeverityWarning},
	{"credentials.yaml", "credentials-file", SeverityWarning},
	{"credentials.yml", "credentials-file", SeverityWarning},

	// Token files
	{"token", "token-file", SeverityWarning},
	{".token", "token-file", SeverityWarning},
	{"access_token", "token-file", SeverityWarning},
	{"refresh_token", "token-file", SeverityWarning},

	// Secret files
	{"secrets", "secrets-file", SeverityWarning},
	{".secrets", "secrets-file", SeverityWarning},
	{"secrets.json", "secrets-file", SeverityWarning},
	{"secrets.yaml", "secrets-file", SeverityWarning},
	{"secrets.yml", "secrets-file", SeverityWarning},

	// Key files
	{".pem", "key-file", SeverityWarning},
	{".key", "key-file", SeverityWarning},
	{"*.pem", "key-file", SeverityWarning},
	{"*.key", "key-file", SeverityWarning},

	// Config files with potential secrets
	{".netrc", "netrc-file", SeverityWarning},
	{"netrc", "netrc-file", SeverityWarning},
	{".npmrc", "npmrc-file", SeverityInfo},
	{".pypirc", "pypirc-file", SeverityInfo},
}

// traversalPatterns defines path traversal patterns.
var traversalPatterns = []string{
	"..",
	"../",
	"..\\",
}

// ValidatePath checks a path for sensitive patterns and returns violations.
//
// This is DETECTION ONLY. The returned violations indicate potential security
// concerns but do NOT cause any blocking or denial of tool execution.
//
// Parameters:
//   - path: The file or directory path to validate
//
// Returns:
//   - A slice of PathViolation for each detected issue
//   - An empty slice if the path is safe
func ValidatePath(path string) []PathViolation {
	if path == "" {
		return nil
	}

	var violations []PathViolation

	// Normalize path for cross-platform comparison
	normalized := normalizePath(path)

	// Check for path traversal
	if hasPathTraversal(path) {
		violations = append(violations, PathViolation{
			Path:      path,
			Rule:      "path-traversal",
			Severity:  SeverityWarning,
			Candidate: true,
		})
	}

	// Check against sensitive rules
	for _, rule := range sensitiveRules {
		if matchesSensitivePattern(normalized, rule.pattern) {
			violations = append(violations, PathViolation{
				Path:      path,
				Rule:      rule.rule,
				Severity:  rule.severity,
				Candidate: true,
			})
		}
	}

	return violations
}

// IsSensitivePath checks if a path is sensitive (any severity).
//
// Returns true if the path matches any sensitive pattern.
// This is a convenience function for quick checks.
func IsSensitivePath(path string) bool {
	violations := ValidatePath(path)
	return len(violations) > 0
}

// IsCriticalPath checks if a path has critical severity violations.
//
// Returns true if the path matches critical patterns like .git, .env, or SSH keys.
func IsCriticalPath(path string) bool {
	violations := ValidatePath(path)
	for _, v := range violations {
		if v.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// normalizePath normalizes a path for cross-platform comparison.
func normalizePath(path string) string {
	// Convert to forward slashes
	normalized := filepath.ToSlash(path)

	// Convert to lowercase for case-insensitive comparison
	normalized = strings.ToLower(normalized)

	// Remove leading ./ or .\
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, ".\\")

	// Remove trailing slash
	normalized = strings.TrimSuffix(normalized, "/")

	return normalized
}

// matchesSensitivePattern checks if a normalized path matches a sensitive pattern.
func matchesSensitivePattern(normalized, pattern string) bool {
	// Exact match
	if normalized == pattern {
		return true
	}

	// Check if path starts with the pattern (e.g., .git/config matches .git)
	if strings.HasPrefix(normalized, pattern+"/") {
		return true
	}

	// Check if path ends with the pattern
	if strings.HasSuffix(normalized, "/"+pattern) {
		return true
	}

	// Check if path contains the pattern as a directory component
	if strings.Contains(normalized, "/"+pattern+"/") {
		return true
	}

	// Check base name match
	base := filepath.Base(normalized)
	if base == pattern {
		return true
	}

	// Check for .env.* pattern
	if pattern == ".env." && strings.HasPrefix(base, ".env.") {
		return true
	}

	// Check for wildcard patterns
	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(normalized, suffix)
	}

	return false
}

// hasPathTraversal checks if a path contains traversal sequences.
func hasPathTraversal(path string) bool {
	// Check for Unix-style traversal
	if strings.Contains(path, "..") {
		// Verify it's actually a traversal (not just .. in a filename)
		normalized := filepath.ToSlash(path)
		parts := strings.Split(normalized, "/")
		for _, part := range parts {
			if part == ".." {
				return true
			}
		}
	}

	// Check for Windows-style traversal
	if strings.Contains(path, "..\\") {
		return true
	}

	return false
}

// GetSensitiveRules returns the list of sensitive rules for inspection.
func GetSensitiveRules() []sensitivePattern {
	return sensitiveRules
}

// GetViolationSummary returns a human-readable summary of violations.
func GetViolationSummary(violations []PathViolation) string {
	if len(violations) == 0 {
		return "allowed"
	}

	hasCritical := false
	hasWarning := false

	for _, v := range violations {
		switch v.Severity {
		case SeverityCritical:
			hasCritical = true
		case SeverityWarning:
			hasWarning = true
		}
	}

	if hasCritical {
		return "blocked_candidate"
	}
	if hasWarning {
		return "warning"
	}
	return "info"
}
