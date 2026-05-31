package review

import (
	"testing"
)

func TestRiskScorer_Violations_Critical(t *testing.T) {
	scorer := NewRiskScorer(DefaultRiskScorerConfig())

	preview := PreviewData{
		Violations: []ViolationInfo{
			{Path: ".env", Reason: "sensitive"},
		},
	}

	score := scorer.Score(preview, nil)
	if score.Level != "critical" {
		t.Errorf("expected critical, got %s", score.Level)
	}
	if score.Score != 100 {
		t.Errorf("expected score 100, got %d", score.Score)
	}
}

func TestRiskScorer_HighRiskFileCount(t *testing.T) {
	scorer := NewRiskScorer(DefaultRiskScorerConfig())

	preview := PreviewData{
		Summary: SummaryInfo{
			FilesChanged: 25,
		},
	}

	score := scorer.Score(preview, nil)
	if score.Level != "high" {
		t.Errorf("expected high, got %s", score.Level)
	}
}

func TestRiskScorer_MediumRiskFileCount(t *testing.T) {
	scorer := NewRiskScorer(DefaultRiskScorerConfig())

	preview := PreviewData{
		Summary: SummaryInfo{
			FilesChanged: 8,
		},
	}

	score := scorer.Score(preview, nil)
	if score.Level != "medium" {
		t.Errorf("expected medium, got %s", score.Level)
	}
}

func TestRiskScorer_HighRiskLineCount(t *testing.T) {
	scorer := NewRiskScorer(DefaultRiskScorerConfig())

	preview := PreviewData{
		Summary: SummaryInfo{
			FilesChanged: 1,
			Additions:    400,
			Deletions:    200,
		},
	}

	score := scorer.Score(preview, nil)
	if score.Level != "high" {
		t.Errorf("expected high, got %s", score.Level)
	}
}

func TestRiskScorer_LowRisk(t *testing.T) {
	scorer := NewRiskScorer(DefaultRiskScorerConfig())

	preview := PreviewData{
		Summary: SummaryInfo{
			FilesChanged: 1,
			Additions:    5,
			Deletions:    2,
		},
	}

	score := scorer.Score(preview, nil)
	if score.Level != "low" {
		t.Errorf("expected low, got %s", score.Level)
	}
}

func TestRiskScorer_BinaryFiles(t *testing.T) {
	scorer := NewRiskScorer(DefaultRiskScorerConfig())

	preview := PreviewData{
		Summary: SummaryInfo{
			FilesChanged: 1,
			HasBinary:    true,
		},
	}

	score := scorer.Score(preview, nil)
	if score.Level != "medium" {
		t.Errorf("expected medium, got %s", score.Level)
	}
}

func TestRiskScorer_CriticalFindings(t *testing.T) {
	scorer := NewRiskScorer(DefaultRiskScorerConfig())

	preview := PreviewData{
		Summary: SummaryInfo{
			FilesChanged: 1,
		},
	}

	findings := []ReviewFinding{
		{Severity: SeverityCritical, Category: CategorySecurity, Message: "critical issue"},
	}

	score := scorer.Score(preview, findings)
	if score.Level != "critical" {
		t.Errorf("expected critical, got %s", score.Level)
	}
}

func TestRiskScorer_DiffRedacted(t *testing.T) {
	scorer := NewRiskScorer(DefaultRiskScorerConfig())

	preview := PreviewData{
		DiffRedacted: true,
		Summary: SummaryInfo{
			FilesChanged: 1,
		},
	}

	score := scorer.Score(preview, nil)
	if score.Level != "medium" {
		t.Errorf("expected medium, got %s", score.Level)
	}
}

func TestRiskScorer_Reasons(t *testing.T) {
	scorer := NewRiskScorer(DefaultRiskScorerConfig())

	preview := PreviewData{
		Summary: SummaryInfo{
			FilesChanged: 25,
			Additions:    400,
			Deletions:    200,
		},
	}

	score := scorer.Score(preview, nil)
	if len(score.Reasons) == 0 {
		t.Error("expected reasons to be populated")
	}
}

func TestRiskScorer_ScoreCap(t *testing.T) {
	scorer := NewRiskScorer(DefaultRiskScorerConfig())

	preview := PreviewData{
		Summary: SummaryInfo{
			FilesChanged: 50,
			Additions:    1000,
			Deletions:    500,
			HasBinary:    true,
		},
		DiffRedacted: true,
	}

	findings := []ReviewFinding{
		{Severity: SeverityCritical, Category: CategorySecurity, Message: "critical"},
	}

	score := scorer.Score(preview, findings)
	if score.Score > 100 {
		t.Errorf("score should be capped at 100, got %d", score.Score)
	}
}
