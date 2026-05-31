package worktree

import (
	"errors"
	"regexp"
	"strings"
)

// safeNameRe matches characters allowed in safe identifiers.
// Only alphanumeric, hyphens, and underscores are permitted.
var safeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

var errEmptyAfterSanitize = errors.New("worktree: identifier is empty after sanitization")

// SanitizeID sanitizes a task ID or user-provided identifier for use in
// file system paths and git branch names. It strips any character that
// is not alphanumeric, a hyphen, or an underscore, then truncates to
// maxLength. It returns an error if the result would be empty.
func SanitizeID(id string, maxLength int) (string, error) {
	if maxLength <= 0 {
		maxLength = 64
	}
	s := safeNameRe.ReplaceAllString(id, "_")
	s = strings.Trim(s, "_-. ")
	if len(s) > maxLength {
		s = s[:maxLength]
	}
	s = strings.TrimRight(s, "_-. ")
	if s == "" {
		return "", errEmptyAfterSanitize
	}
	return s, nil
}

// SanitizeBranchName sanitizes a string for use as a git branch name
// component. Git branch names cannot contain spaces, colons, or
// various special characters. This function ensures the name is safe.
func SanitizeBranchName(name string, maxLength int) (string, error) {
	if maxLength <= 0 {
		maxLength = 80
	}
	s := safeNameRe.ReplaceAllString(name, "-")
	s = strings.Trim(s, "-. ")
	// Collapse consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if len(s) > maxLength {
		s = s[:maxLength]
	}
	s = strings.TrimRight(s, "-. ")
	if s == "" {
		return "", errEmptyAfterSanitize
	}
	return s, nil
}

// IsPathTraversal checks whether a path component contains directory
// traversal sequences that could escape the intended parent directory.
func IsPathTraversal(p string) bool {
	cleaned := strings.ReplaceAll(p, `\`, "/")
	return strings.Contains(cleaned, "..") ||
		strings.HasPrefix(cleaned, "/") ||
		strings.HasPrefix(cleaned, "~/")
}
