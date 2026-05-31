package cache

import (
	"testing"
	"time"

	"github.com/nekonomimo/nekonomimo/internal/prefix"
)

func TestReportEmpty(t *testing.T) {
	report := buildReport(nil, time.Now())
	if len(report.ByFingerprint) != 0 {
		t.Errorf("ByFingerprint length = %d, want 0", len(report.ByFingerprint))
	}
	if report.GlobalSummary.TotalObservations != 0 {
		t.Errorf("TotalObservations = %d, want 0", report.GlobalSummary.TotalObservations)
	}
}

func TestReportFullHit(t *testing.T) {
	observations := []Observation{
		{Fingerprint: prefix.Fingerprint{SHA256: "hash1"}, InputTokens: 1000, CachedTokens: 1000, Provider: "p1", Model: "m1", ObservedAt: time.Now()},
		{Fingerprint: prefix.Fingerprint{SHA256: "hash1"}, InputTokens: 1000, CachedTokens: 1000, Provider: "p1", Model: "m1", ObservedAt: time.Now()},
	}

	report := buildReport(observations, time.Now())
	if len(report.ByFingerprint) != 1 {
		t.Fatalf("ByFingerprint length = %d, want 1", len(report.ByFingerprint))
	}

	fp := report.ByFingerprint[0]
	if fp.HitRate != 1.0 {
		t.Errorf("HitRate = %f, want 1.0", fp.HitRate)
	}
	if fp.ReuseCount != 1 {
		t.Errorf("ReuseCount = %d, want 1", fp.ReuseCount)
	}
}

func TestReportPartialHit(t *testing.T) {
	observations := []Observation{
		{Fingerprint: prefix.Fingerprint{SHA256: "hash1"}, InputTokens: 1000, CachedTokens: 600, Provider: "p1", Model: "m1", ObservedAt: time.Now()},
	}

	report := buildReport(observations, time.Now())
	fp := report.ByFingerprint[0]

	if fp.HitRate != 0.6 {
		t.Errorf("HitRate = %f, want 0.6", fp.HitRate)
	}
	if fp.UncachedTokens != 400 {
		t.Errorf("UncachedTokens = %d, want 400", fp.UncachedTokens)
	}
}

func TestReportMissNoPriorObservation(t *testing.T) {
	observations := []Observation{
		{Fingerprint: prefix.Fingerprint{SHA256: "hash1"}, InputTokens: 1000, CachedTokens: 0, Provider: "p1", Model: "m1", ObservedAt: time.Now()},
	}

	report := buildReport(observations, time.Now())
	fp := report.ByFingerprint[0]

	found := false
	for _, r := range fp.PossibleMissReasons {
		if r == MissReasonNoPriorObservation {
			found = true
		}
	}
	if !found {
		t.Errorf("PossibleMissReasons = %v, want no_prior_observation", fp.PossibleMissReasons)
	}
}

func TestReportGlobalSummary(t *testing.T) {
	observations := []Observation{
		{Fingerprint: prefix.Fingerprint{SHA256: "hash1"}, InputTokens: 1000, CachedTokens: 600, Provider: "p1", Model: "m1", ObservedAt: time.Now()},
		{Fingerprint: prefix.Fingerprint{SHA256: "hash2"}, InputTokens: 2000, CachedTokens: 1800, Provider: "p1", Model: "m1", ObservedAt: time.Now()},
	}

	report := buildReport(observations, time.Now())

	if report.GlobalSummary.TotalObservations != 2 {
		t.Errorf("TotalObservations = %d, want 2", report.GlobalSummary.TotalObservations)
	}
	if report.GlobalSummary.TotalTokens != 3000 {
		t.Errorf("TotalTokens = %d, want 3000", report.GlobalSummary.TotalTokens)
	}
	if report.GlobalSummary.TotalCachedTokens != 2400 {
		t.Errorf("TotalCachedTokens = %d, want 2400", report.GlobalSummary.TotalCachedTokens)
	}
	if report.GlobalSummary.OverallHitRate != 0.8 {
		t.Errorf("OverallHitRate = %f, want 0.8", report.GlobalSummary.OverallHitRate)
	}
}

func TestReportMultipleFingerprints(t *testing.T) {
	observations := []Observation{
		{Fingerprint: prefix.Fingerprint{SHA256: "hash1"}, InputTokens: 100, CachedTokens: 80, Provider: "p1", Model: "m1", ObservedAt: time.Now()},
		{Fingerprint: prefix.Fingerprint{SHA256: "hash2"}, InputTokens: 200, CachedTokens: 100, Provider: "p1", Model: "m1", ObservedAt: time.Now()},
	}

	report := buildReport(observations, time.Now())
	if len(report.ByFingerprint) != 2 {
		t.Fatalf("ByFingerprint length = %d, want 2", len(report.ByFingerprint))
	}

	// Hashes should be sorted alphabetically
	if report.ByFingerprint[0].PrefixHash != "hash1" {
		t.Errorf("First fingerprint = %q, want hash1", report.ByFingerprint[0].PrefixHash)
	}
	if report.ByFingerprint[1].PrefixHash != "hash2" {
		t.Errorf("Second fingerprint = %q, want hash2", report.ByFingerprint[1].PrefixHash)
	}
}
