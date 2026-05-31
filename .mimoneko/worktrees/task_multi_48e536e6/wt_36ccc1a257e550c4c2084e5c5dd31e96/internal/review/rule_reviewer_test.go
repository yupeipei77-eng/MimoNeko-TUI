package review

import (
	"testing"
)

func TestRuleBasedReviewer_Violations(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	reviewer := NewRuleBasedReviewer(cfg)

	preview := PreviewData{
		WorktreeID: "wt_test",
		Violations: []ViolationInfo{
			{Path: ".env", Reason: "sensitive file"},
		},
		Summary: SummaryInfo{
			FilesChanged: 1,
		},
	}

	findings := reviewer.Review(preview)

	if len(findings) == 0 {
		t.Fatal("expected findings for violations")
	}
	found := false
	for _, f := range findings {
		if f.Severity == SeverityCritical && f.Category == CategorySecurity {
			found = true
		}
	}
	if !found {
		t.Error("expected critical security finding for violations")
	}
}

func TestRuleBasedReviewer_HighRiskFileCount(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	reviewer := NewRuleBasedReviewer(cfg)

	preview := PreviewData{
		WorktreeID: "wt_test",
		Summary: SummaryInfo{
			FilesChanged: 25,
		},
	}

	findings := reviewer.Review(preview)

	found := false
	for _, f := range findings {
		if f.Category == CategoryRisk && f.Message == "files changed exceeds high risk threshold" {
			found = true
		}
	}
	if !found {
		t.Error("expected high risk file count finding")
	}
}

func TestRuleBasedReviewer_MediumRiskFileCount(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	reviewer := NewRuleBasedReviewer(cfg)

	preview := PreviewData{
		WorktreeID: "wt_test",
		Summary: SummaryInfo{
			FilesChanged: 8,
		},
	}

	findings := reviewer.Review(preview)

	found := false
	for _, f := range findings {
		if f.Category == CategoryRisk && f.Message == "files changed exceeds medium risk threshold" {
			found = true
		}
	}
	if !found {
		t.Error("expected medium risk file count finding")
	}
}

func TestRuleBasedReviewer_HighRiskLineCount(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	reviewer := NewRuleBasedReviewer(cfg)

	preview := PreviewData{
		WorktreeID: "wt_test",
		Summary: SummaryInfo{
			FilesChanged: 1,
			Additions:    400,
			Deletions:    200,
		},
	}

	findings := reviewer.Review(preview)

	found := false
	for _, f := range findings {
		if f.Category == CategoryRisk && f.Message == "total line changes exceed high risk threshold" {
			found = true
		}
	}
	if !found {
		t.Error("expected high risk line count finding")
	}
}

func TestRuleBasedReviewer_MediumRiskLineCount(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	reviewer := NewRuleBasedReviewer(cfg)

	preview := PreviewData{
		WorktreeID: "wt_test",
		Summary: SummaryInfo{
			FilesChanged: 1,
			Additions:    80,
			Deletions:    30,
		},
	}

	findings := reviewer.Review(preview)

	found := false
	for _, f := range findings {
		if f.Category == CategoryRisk && f.Message == "total line changes exceed medium risk threshold" {
			found = true
		}
	}
	if !found {
		t.Error("expected medium risk line count finding")
	}
}

func TestRuleBasedReviewer_BinaryFilesNotAllowed(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	cfg.AllowBinary = false
	reviewer := NewRuleBasedReviewer(cfg)

	preview := PreviewData{
		WorktreeID: "wt_test",
		Summary: SummaryInfo{
			FilesChanged: 1,
			HasBinary:    true,
		},
	}

	findings := reviewer.Review(preview)

	found := false
	for _, f := range findings {
		if f.Severity == SeverityCritical && f.Category == CategorySecurity {
			found = true
		}
	}
	if !found {
		t.Error("expected critical finding for binary files when not allowed")
	}
}

func TestRuleBasedReviewer_BinaryFilesAllowed(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	cfg.AllowBinary = true
	reviewer := NewRuleBasedReviewer(cfg)

	preview := PreviewData{
		WorktreeID: "wt_test",
		Summary: SummaryInfo{
			FilesChanged: 1,
			HasBinary:    true,
		},
	}

	findings := reviewer.Review(preview)

	found := false
	for _, f := range findings {
		if f.Severity == SeverityWarning && f.Category == CategoryRisk {
			found = true
		}
	}
	if !found {
		t.Error("expected warning finding for binary files when allowed")
	}
}

func TestRuleBasedReviewer_SourceWithoutTest(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	reviewer := NewRuleBasedReviewer(cfg)

	preview := PreviewData{
		WorktreeID: "wt_test",
		FilesChanged: []FileChangeInfo{
			{Path: "main.go", Status: "modified", Additions: 10, Deletions: 0},
		},
		Summary: SummaryInfo{
			FilesChanged: 1,
			Additions:    10,
		},
	}

	findings := reviewer.Review(preview)

	found := false
	for _, f := range findings {
		if f.Category == CategoryTest && f.Message == "source code modified without corresponding test changes" {
			found = true
		}
	}
	if !found {
		t.Error("expected info finding for source changes without tests")
	}
}

func TestRuleBasedReviewer_SourceWithTest(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	reviewer := NewRuleBasedReviewer(cfg)

	preview := PreviewData{
		WorktreeID: "wt_test",
		FilesChanged: []FileChangeInfo{
			{Path: "main.go", Status: "modified", Additions: 10, Deletions: 0},
			{Path: "main_test.go", Status: "modified", Additions: 5, Deletions: 0},
		},
		Summary: SummaryInfo{
			FilesChanged: 2,
			Additions:    15,
		},
	}

	findings := reviewer.Review(preview)

	for _, f := range findings {
		if f.Category == CategoryTest && f.Message == "source code modified without corresponding test changes" {
			t.Error("should not produce test warning when tests are also changed")
		}
	}
}

func TestRuleBasedReviewer_GeneratedFiles(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	reviewer := NewRuleBasedReviewer(cfg)

	preview := PreviewData{
		WorktreeID: "wt_test",
		FilesChanged: []FileChangeInfo{
			{Path: "go.sum", Status: "modified"},
		},
		Summary: SummaryInfo{
			FilesChanged: 1,
		},
	}

	findings := reviewer.Review(preview)

	found := false
	for _, f := range findings {
		if f.Category == CategoryStyle && f.Path == "go.sum" {
			found = true
		}
	}
	if !found {
		t.Error("expected finding for generated file")
	}
}

func TestRuleBasedReviewer_DiffTooLarge(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	cfg.MaxDiffBytes = 10
	reviewer := NewRuleBasedReviewer(cfg)

	preview := PreviewData{
		WorktreeID: "wt_test",
		Diff:       "this is a diff that is longer than ten bytes",
		Summary: SummaryInfo{
			FilesChanged: 1,
		},
	}

	findings := reviewer.Review(preview)

	found := false
	for _, f := range findings {
		if f.Category == CategoryRisk && f.Message == "diff exceeds max_diff_bytes or is redacted/truncated" {
			found = true
		}
	}
	if !found {
		t.Error("expected finding for diff exceeding max bytes")
	}
}

func TestRuleBasedReviewer_SensitivePaths(t *testing.T) {
	cfg := DefaultRuleBasedReviewerConfig()
	reviewer := NewRuleBasedReviewer(cfg)

	paths := []string{".env", "cert.pem", "id_rsa.key", ".git/config", ".nekonomimo/config.yaml"}
	for _, path := range paths {
		preview := PreviewData{
			WorktreeID: "wt_test",
			FilesChanged: []FileChangeInfo{
				{Path: path, Status: "modified"},
			},
			Summary: SummaryInfo{
				FilesChanged: 1,
			},
		}

		findings := reviewer.Review(preview)

		found := false
		for _, f := range findings {
			if f.Severity == SeverityCritical && f.Category == CategorySecurity && f.Path == path {
				found = true
			}
		}
		if !found {
			t.Errorf("expected critical finding for sensitive path %s", path)
		}
	}
}

func TestIsSourceFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"main.go", true},
		{"app.py", true},
		{"index.ts", true},
		{"app.js", true},
		{"readme.md", false},
		{"go.mod", false},
		{"Makefile", false},
	}
	for _, tt := range tests {
		result := isSourceFile(tt.path)
		if result != tt.expected {
			t.Errorf("isSourceFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"main_test.go", true},
		{"test_app.py", true},
		{"app.spec.ts", true},
		{"app.test.ts", true},
		{"main.go", false},
		{"test.go", false},
		{"testing.go", false},
	}
	for _, tt := range tests {
		result := isTestFile(tt.path)
		if result != tt.expected {
			t.Errorf("isTestFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestIsGeneratedFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"package-lock.json", true},
		{"go.sum", true},
		{"dist/bundle.js", true},
		{"build/output.bin", true},
		{"main.go", false},
		{"src/app.ts", false},
	}
	for _, tt := range tests {
		result := isGeneratedFile(tt.path)
		if result != tt.expected {
			t.Errorf("isGeneratedFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}
