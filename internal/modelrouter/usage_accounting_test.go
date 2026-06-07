package modelrouter

import (
	"testing"
	"time"

	"github.com/mimoneko/mimoneko/internal/contextengine"
	"github.com/mimoneko/mimoneko/internal/prefix"
)

func TestUsageToObservationMapsFields(t *testing.T) {
	usage := Usage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		CachedTokens: 30,
		Estimated:    false,
	}

	bundle := contextengine.Bundle{
		CacheFingerprint: prefix.Fingerprint{SHA256: "sha256abc", Version: 2},
		Report: contextengine.ContextReport{
			PrefixTokens:       40,
			ConversationTokens: 30,
			ScratchpadTokens:   20,
			CurrentInputTokens: 10,
			TotalTokens:        100,
		},
	}

	obs := UsageToObservation(usage, bundle, "deepseek", "deepseek-chat", "req-001")

	if obs.Provider != "deepseek" {
		t.Errorf("Provider = %q, want deepseek", obs.Provider)
	}
	if obs.Model != "deepseek-chat" {
		t.Errorf("Model = %q, want deepseek-chat", obs.Model)
	}
	if obs.RequestID != "req-001" {
		t.Errorf("RequestID = %q, want req-001", obs.RequestID)
	}
	if obs.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", obs.InputTokens)
	}
	if obs.CachedTokens != 30 {
		t.Errorf("CachedTokens = %d, want 30", obs.CachedTokens)
	}
	if obs.Estimated != false {
		t.Errorf("Estimated = %v, want false", obs.Estimated)
	}
	if obs.Fingerprint.SHA256 != "sha256abc" {
		t.Errorf("Fingerprint.SHA256 = %q, want sha256abc", obs.Fingerprint.SHA256)
	}
	if obs.Fingerprint.Version != 2 {
		t.Errorf("Fingerprint.Version = %d, want 2", obs.Fingerprint.Version)
	}
	if obs.PrefixTokens != 40 {
		t.Errorf("PrefixTokens = %d, want 40", obs.PrefixTokens)
	}
	if obs.ConversationTokens != 30 {
		t.Errorf("ConversationTokens = %d, want 30", obs.ConversationTokens)
	}
	if obs.ScratchpadTokens != 20 {
		t.Errorf("ScratchpadTokens = %d, want 20", obs.ScratchpadTokens)
	}
	if obs.CurrentInputTokens != 10 {
		t.Errorf("CurrentInputTokens = %d, want 10", obs.CurrentInputTokens)
	}
	if obs.ObservedAt.IsZero() {
		t.Error("ObservedAt should not be zero")
	}
}

func TestUsageToObservationEstimatedWhenNoCachedTokens(t *testing.T) {
	usage := Usage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		CachedTokens: 0,
		Estimated:    true,
	}

	bundle := contextengine.Bundle{
		CacheFingerprint: prefix.Fingerprint{SHA256: "abc", Version: 1},
		Report:           contextengine.ContextReport{TotalTokens: 100},
	}

	obs := UsageToObservation(usage, bundle, "p", "m", "r")

	if obs.CachedTokens != 0 {
		t.Errorf("CachedTokens = %d, want 0", obs.CachedTokens)
	}
	if obs.Estimated != true {
		t.Errorf("Estimated = %v, want true", obs.Estimated)
	}
}

func TestUsageToObservationFallsBackToBundleTotalWhenInputZero(t *testing.T) {
	usage := Usage{
		InputTokens:  0,
		OutputTokens: 0,
		TotalTokens:  0,
		CachedTokens: 0,
		Estimated:    true,
	}

	bundle := contextengine.Bundle{
		CacheFingerprint: prefix.Fingerprint{SHA256: "abc", Version: 1},
		Report: contextengine.ContextReport{
			TotalTokens: 200,
		},
	}

	obs := UsageToObservation(usage, bundle, "p", "m", "r")

	if obs.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200 (from Bundle.Report.TotalTokens)", obs.InputTokens)
	}
}

func TestUsageToObservationCachedTokensFromProvider(t *testing.T) {
	usage := Usage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		CachedTokens: 40,
		Estimated:    false,
	}

	bundle := contextengine.Bundle{
		CacheFingerprint: prefix.Fingerprint{SHA256: "abc", Version: 1},
		Report:           contextengine.ContextReport{TotalTokens: 100},
	}

	obs := UsageToObservation(usage, bundle, "p", "m", "r")

	if obs.CachedTokens != 40 {
		t.Errorf("CachedTokens = %d, want 40", obs.CachedTokens)
	}
	if obs.Estimated != false {
		t.Errorf("Estimated = %v, want false", obs.Estimated)
	}
}

func TestUsageToObservationMapsNativeCacheMetrics(t *testing.T) {
	usage := Usage{
		InputTokens:      1000,
		OutputTokens:     50,
		TotalTokens:      1050,
		CachedTokens:     900,
		CacheHitTokens:   900,
		CacheMissTokens:  100,
		NativeCacheKnown: true,
		Estimated:        false,
	}

	bundle := contextengine.Bundle{
		CacheFingerprint: prefix.Fingerprint{SHA256: "mimo-native", Version: 1},
		Report:           contextengine.ContextReport{TotalTokens: 1000},
	}

	obs := UsageToObservation(usage, bundle, "mimo", "mimo-v2.5-pro", "req-native")

	if !obs.NativeCacheKnown {
		t.Fatal("NativeCacheKnown = false, want true")
	}
	if obs.CacheHitTokens != 900 || obs.CacheMissTokens != 100 {
		t.Fatalf("native cache = hit %d miss %d, want 900/100", obs.CacheHitTokens, obs.CacheMissTokens)
	}
}

func TestUsageToObservationTimestampIsRecent(t *testing.T) {
	usage := Usage{InputTokens: 1, Estimated: true}
	bundle := contextengine.Bundle{
		CacheFingerprint: prefix.Fingerprint{SHA256: "abc", Version: 1},
		Report:           contextengine.ContextReport{},
	}

	before := time.Now()
	obs := UsageToObservation(usage, bundle, "p", "m", "r")
	after := time.Now()

	if obs.ObservedAt.Before(before) || obs.ObservedAt.After(after) {
		t.Errorf("ObservedAt = %v, want between %v and %v", obs.ObservedAt, before, after)
	}
}
