package review

import (
	"path/filepath"
	"strings"

	"github.com/mimoneko/mimoneko/internal/config"
)

// RuleBasedReviewerConfig configures the rule-based reviewer thresholds.
type RuleBasedReviewerConfig struct {
	// MaxDiffBytes caps the diff output; exceeding this raises risk to high.
	MaxDiffBytes int

	// HighRiskFileCount is the file count threshold for high risk.
	HighRiskFileCount int

	// MediumRiskFileCount is the file count threshold for medium risk.
	MediumRiskFileCount int

	// HighRiskLineCount is the total additions+deletions threshold for high risk.
	HighRiskLineCount int

	// MediumRiskLineCount is the total additions+deletions threshold for medium risk.
	MediumRiskLineCount int

	// AllowBinary controls whether binary files are allowed in patches.
	AllowBinary bool

	// RequireTestsForCodeChanges produces a warning when source code is modified
	// without corresponding test changes.
	RequireTestsForCodeChanges bool
}

// DefaultRuleBasedReviewerConfig returns safe defaults.
func DefaultRuleBasedReviewerConfig() RuleBasedReviewerConfig {
	return RuleBasedReviewerConfig{
		MaxDiffBytes:               131072,
		HighRiskFileCount:          20,
		MediumRiskFileCount:        5,
		HighRiskLineCount:          500,
		MediumRiskLineCount:        100,
		AllowBinary:                false,
		RequireTestsForCodeChanges: false,
	}
}

// RuleBasedReviewer performs rule-based review without any model calls.
// It checks violations, diff size, file counts, line counts, binary files,
// sensitive paths, test coverage, and generated files.
type RuleBasedReviewer struct {
	cfg RuleBasedReviewerConfig
}

// NewRuleBasedReviewer creates a new RuleBasedReviewer.
func NewRuleBasedReviewer(cfg RuleBasedReviewerConfig) *RuleBasedReviewer {
	return &RuleBasedReviewer{cfg: cfg}
}

// Review performs the rule-based review and returns findings.
func (r *RuleBasedReviewer) Review(preview PreviewData) []ReviewFinding {
	var findings []ReviewFinding

	// 1. Check PatchPreview.Violations
	if len(preview.Violations) > 0 {
		for _, v := range preview.Violations {
			findings = append(findings, ReviewFinding{
				Severity: SeverityCritical,
				Category: CategorySecurity,
				Path:     v.Path,
				Message:  v.Reason,
			})
		}
	}

	// 2. Diff size check
	if len(preview.Diff) > r.cfg.MaxDiffBytes || preview.DiffRedacted {
		findings = append(findings, ReviewFinding{
			Severity: SeverityWarning,
			Category: CategoryRisk,
			Message:  "diff exceeds max_diff_bytes or is redacted/truncated",
		})
	}

	// 3. Files changed count
	if preview.Summary.FilesChanged > r.cfg.HighRiskFileCount {
		findings = append(findings, ReviewFinding{
			Severity: SeverityWarning,
			Category: CategoryRisk,
			Message:  "files changed exceeds high risk threshold",
		})
	} else if preview.Summary.FilesChanged > r.cfg.MediumRiskFileCount {
		findings = append(findings, ReviewFinding{
			Severity: SeverityInfo,
			Category: CategoryRisk,
			Message:  "files changed exceeds medium risk threshold",
		})
	}

	// 4. Additions/deletions
	totalLines := preview.Summary.Additions + preview.Summary.Deletions
	if totalLines > r.cfg.HighRiskLineCount {
		findings = append(findings, ReviewFinding{
			Severity: SeverityWarning,
			Category: CategoryRisk,
			Message:  "total line changes exceed high risk threshold",
		})
	} else if totalLines > r.cfg.MediumRiskLineCount {
		findings = append(findings, ReviewFinding{
			Severity: SeverityInfo,
			Category: CategoryRisk,
			Message:  "total line changes exceed medium risk threshold",
		})
	}

	// 5. Binary files
	if preview.Summary.HasBinary {
		if !r.cfg.AllowBinary {
			findings = append(findings, ReviewFinding{
				Severity: SeverityCritical,
				Category: CategorySecurity,
				Message:  "binary file changes not allowed (patch.allow_binary=false)",
			})
		} else {
			findings = append(findings, ReviewFinding{
				Severity: SeverityWarning,
				Category: CategoryRisk,
				Message:  "patch contains binary file changes",
			})
		}
	}

	// 6. Sensitive paths (violations already cover this, but double-check)
	for _, f := range preview.FilesChanged {
		if isSensitivePath(f.Path) {
			findings = append(findings, ReviewFinding{
				Severity: SeverityCritical,
				Category: CategorySecurity,
				Path:     f.Path,
				Message:  "file is on a sensitive path",
			})
		}
	}

	// 7. Test files - warn if source changes without test changes
	if r.cfg.RequireTestsForCodeChanges {
		hasSourceChanges := false
		hasTestChanges := false
		for _, f := range preview.FilesChanged {
			if isSourceFile(f.Path) {
				hasSourceChanges = true
			}
			if isTestFile(f.Path) {
				hasTestChanges = true
			}
		}
		if hasSourceChanges && !hasTestChanges {
			findings = append(findings, ReviewFinding{
				Severity: SeverityWarning,
				Category: CategoryTest,
				Message:  "source code modified without corresponding test changes",
			})
		}
	} else {
		// Even when not required, still produce an info-level finding
		hasSourceChanges := false
		hasTestChanges := false
		for _, f := range preview.FilesChanged {
			if isSourceFile(f.Path) {
				hasSourceChanges = true
			}
			if isTestFile(f.Path) {
				hasTestChanges = true
			}
		}
		if hasSourceChanges && !hasTestChanges {
			findings = append(findings, ReviewFinding{
				Severity: SeverityInfo,
				Category: CategoryTest,
				Message:  "source code modified without corresponding test changes",
			})
		}
	}

	// 8. Generated files - warning
	for _, f := range preview.FilesChanged {
		if isGeneratedFile(f.Path) {
			findings = append(findings, ReviewFinding{
				Severity: SeverityInfo,
				Category: CategoryStyle,
				Path:     f.Path,
				Message:  "file is a generated file (consider adding to .gitignore)",
			})
		}
	}

	return findings
}

// PreviewData is a reduced view of PatchPreview used by the rule-based reviewer.
// This abstraction prevents the reviewer from depending on the full PatchPreview
// which may contain sensitive diff content.
type PreviewData struct {
	WorktreeID   string
	FilesChanged []FileChangeInfo
	Diff         string
	DiffRedacted bool
	Summary      SummaryInfo
	Violations   []ViolationInfo
}

// FileChangeInfo describes a changed file for rule-based review.
type FileChangeInfo struct {
	Path      string
	Status    string
	Additions int
	Deletions int
}

// SummaryInfo describes the aggregate summary for rule-based review.
type SummaryInfo struct {
	FilesChanged int
	Additions    int
	Deletions    int
	HasBinary    bool
}

// ViolationInfo describes a violation for rule-based review.
type ViolationInfo struct {
	Path   string
	Reason string
}

// sensitivePaths lists paths that must never be modified.
var sensitivePaths = []string{
	".env",
	"*.pem",
	"*.key",
	".git",
	config.DirName(),
}

// isSensitivePath checks whether a path matches sensitive patterns.
func isSensitivePath(relPath string) bool {
	normalized := filepath.ToSlash(relPath)
	for _, pattern := range sensitivePaths {
		matched, _ := filepath.Match(pattern, normalized)
		if matched {
			return true
		}
		if !strings.Contains(pattern, "/") {
			base := filepath.Base(normalized)
			matched, _ = filepath.Match(pattern, base)
			if matched {
				return true
			}
		}
		// Check if path is under a sensitive directory
		if strings.HasPrefix(normalized, pattern+"/") {
			return true
		}
	}
	return false
}

// sourceExtensions lists file extensions considered source code.
var sourceExtensions = map[string]bool{
	".go": true, ".py": true, ".ts": true, ".js": true,
	".java": true, ".rs": true, ".c": true, ".cpp": true,
	".rb": true, ".php": true, ".cs": true, ".swift": true,
}

// isSourceFile checks whether a file is a source code file.
func isSourceFile(relPath string) bool {
	ext := strings.ToLower(filepath.Ext(relPath))
	return sourceExtensions[ext]
}

// testFilePatterns lists patterns for test files.
var testFilePatterns = []string{
	"*_test.go",
	"test_*.py",
	"*.spec.ts",
	"*.test.ts",
	"*_test.js",
	"*.test.js",
	"*_test.rs",
	"*Test.java",
	"*_test.rb",
}

// isTestFile checks whether a file is a test file.
func isTestFile(relPath string) bool {
	base := filepath.Base(relPath)
	for _, pattern := range testFilePatterns {
		matched, _ := filepath.Match(pattern, base)
		if matched {
			return true
		}
	}
	return false
}

// generatedFilePatterns lists patterns for generated files.
var generatedFilePatterns = []string{
	"package-lock.json",
	"go.sum",
}

// generatedFilePrefixes lists path prefixes for generated files.
var generatedFilePrefixes = []string{
	"dist/",
	"build/",
}

// isGeneratedFile checks whether a file is a generated file.
func isGeneratedFile(relPath string) bool {
	normalized := filepath.ToSlash(relPath)
	base := filepath.Base(normalized)
	for _, pattern := range generatedFilePatterns {
		if base == pattern {
			return true
		}
	}
	for _, prefix := range generatedFilePrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}
