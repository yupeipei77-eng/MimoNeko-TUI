package review

// RiskScorerConfig configures the risk scoring thresholds.
type RiskScorerConfig struct {
	// HighRiskFileCount is the threshold for high risk based on file count.
	HighRiskFileCount int

	// MediumRiskFileCount is the threshold for medium risk based on file count.
	MediumRiskFileCount int

	// HighRiskLineCount is the threshold for high risk based on total lines changed.
	HighRiskLineCount int

	// MediumRiskLineCount is the threshold for medium risk based on total lines changed.
	MediumRiskLineCount int
}

// DefaultRiskScorerConfig returns safe defaults.
func DefaultRiskScorerConfig() RiskScorerConfig {
	return RiskScorerConfig{
		HighRiskFileCount:   20,
		MediumRiskFileCount: 5,
		HighRiskLineCount:   500,
		MediumRiskLineCount: 100,
	}
}

// RiskScorer computes a RiskScore from preview data and rule findings.
type RiskScorer struct {
	cfg RiskScorerConfig
}

// NewRiskScorer creates a new RiskScorer.
func NewRiskScorer(cfg RiskScorerConfig) *RiskScorer {
	return &RiskScorer{cfg: cfg}
}

// Score computes the risk score based on preview data and findings.
// The scoring is deterministic: same inputs always produce the same score.
func (s *RiskScorer) Score(preview PreviewData, findings []ReviewFinding) RiskScore {
	score := 0
	var reasons []string

	// Violations => critical
	if len(preview.Violations) > 0 {
		score = 100
		reasons = append(reasons, "patch contains policy violations")
		return RiskScore{
			Level:   "critical",
			Score:   score,
			Reasons: reasons,
		}
	}

	// Check for critical findings
	hasCritical := false
	for _, f := range findings {
		if f.Severity == SeverityCritical {
			hasCritical = true
			break
		}
	}
	if hasCritical {
		score += 80
		reasons = append(reasons, "critical findings detected")
	}

	// Binary files
	if preview.Summary.HasBinary {
		score += 35
		reasons = append(reasons, "patch contains binary file changes")
	}

	// File count scoring — each tier can independently reach its risk level
	if preview.Summary.FilesChanged > s.cfg.HighRiskFileCount {
		score += 60
		reasons = append(reasons, "files changed exceeds high risk threshold")
	} else if preview.Summary.FilesChanged > s.cfg.MediumRiskFileCount {
		score += 30
		reasons = append(reasons, "files changed exceeds medium risk threshold")
	}

	// Line count scoring — each tier can independently reach its risk level
	totalLines := preview.Summary.Additions + preview.Summary.Deletions
	if totalLines > s.cfg.HighRiskLineCount {
		score += 60
		reasons = append(reasons, "total line changes exceed high risk threshold")
	} else if totalLines > s.cfg.MediumRiskLineCount {
		score += 30
		reasons = append(reasons, "total line changes exceed medium risk threshold")
	}

	// Diff redacted/truncated
	if preview.DiffRedacted {
		score += 30
		reasons = append(reasons, "diff is redacted or truncated")
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	// Determine level
	level := "low"
	switch {
	case score >= 80:
		level = "critical"
	case score >= 60:
		level = "high"
	case score >= 30:
		level = "medium"
	}

	return RiskScore{
		Level:   level,
		Score:   score,
		Reasons: reasons,
	}
}
