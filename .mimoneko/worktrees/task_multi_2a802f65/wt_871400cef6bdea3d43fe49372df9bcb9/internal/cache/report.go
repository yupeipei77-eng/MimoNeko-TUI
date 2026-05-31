package cache

import (
	"sort"
	"time"
)

// buildReport generates a CacheReport from observations.
// It groups by prefix hash and computes per-fingerprint and global statistics.
func buildReport(observations []Observation, now time.Time) CacheReport {
	if len(observations) == 0 {
		return CacheReport{
			GlobalSummary: GlobalCacheSummary{},
		}
	}

	// Group observations by fingerprint SHA-256
	groups := make(map[string][]Observation)
	var hashes []string
	for _, obs := range observations {
		hash := obs.Fingerprint.SHA256
		if _, exists := groups[hash]; !exists {
			hashes = append(hashes, hash)
		}
		groups[hash] = append(groups[hash], obs)
	}

	// Sort hashes for deterministic output
	sort.Strings(hashes)

	var reports []PerFingerprintReport
	globalTotalTokens := 0
	globalCachedTokens := 0

	for _, hash := range hashes {
		group := groups[hash]
		report := buildFingerprintReport(hash, group, now)
		reports = append(reports, report)
		globalTotalTokens += report.TotalTokens
		globalCachedTokens += report.CachedTokens
	}

	var overallHitRate float64
	var estimatedSaving float64
	if globalTotalTokens > 0 {
		overallHitRate = float64(globalCachedTokens) / float64(globalTotalTokens)
		estimatedSaving = overallHitRate * 100
	}

	return CacheReport{
		ByFingerprint: reports,
		GlobalSummary: GlobalCacheSummary{
			TotalObservations:      len(observations),
			TotalTokens:            globalTotalTokens,
			TotalCachedTokens:      globalCachedTokens,
			OverallHitRate:         overallHitRate,
			EstimatedSavingPercent: estimatedSaving,
		},
	}
}

func buildFingerprintReport(hash string, observations []Observation, now time.Time) PerFingerprintReport {
	totalTokens := 0
	cachedTokens := 0

	for _, obs := range observations {
		totalTokens += obs.InputTokens
		cachedTokens += obs.CachedTokens
	}

	uncachedTokens := totalTokens - cachedTokens

	var hitRate float64
	var estimatedSaving float64
	if totalTokens > 0 {
		hitRate = float64(cachedTokens) / float64(totalTokens)
		estimatedSaving = hitRate * 100
	}

	reuseCount := len(observations) - 1
	if reuseCount < 0 {
		reuseCount = 0
	}

	missReasons := analyzeMissReasons(hash, observations, now)

	return PerFingerprintReport{
		PrefixHash:             hash,
		TotalTokens:            totalTokens,
		CachedTokens:           cachedTokens,
		UncachedTokens:         uncachedTokens,
		HitRate:                hitRate,
		EstimatedSavingPercent: estimatedSaving,
		ReuseCount:             reuseCount,
		PossibleMissReasons:    missReasons,
	}
}

func analyzeMissReasons(hash string, observations []Observation, now time.Time) []MissReason {
	var reasons []MissReason
	reasonSet := make(map[MissReason]bool)

	// Only first observation for this hash → no prior observation
	if len(observations) == 1 {
		if !reasonSet[MissReasonNoPriorObservation] {
			reasons = append(reasons, MissReasonNoPriorObservation)
			reasonSet[MissReasonNoPriorObservation] = true
		}
	}

	// Check for model changes across observations for same fingerprint
	if len(observations) > 1 {
		seen := make(map[string]bool)
		for _, obs := range observations {
			key := obs.Provider + "/" + obs.Model
			if seen[key] && !reasonSet[MissReasonModelChanged] {
				// Same fingerprint observed with different models (shouldn't happen, but indicates provider issue)
			}
			seen[key] = true
		}

		// Check if model changed between consecutive observations
		for i := 1; i < len(observations); i++ {
			prev := observations[i-1]
			curr := observations[i]
			if prev.Model != curr.Model || prev.Provider != curr.Provider {
				if !reasonSet[MissReasonModelChanged] {
					reasons = append(reasons, MissReasonModelChanged)
					reasonSet[MissReasonModelChanged] = true
				}
				break
			}
		}
	}

	// Check for cache expiration
	for _, obs := range observations {
		if !obs.ObservedAt.IsZero() && obs.CachedTokens < obs.InputTokens {
			// Some tokens were not cached — check if this could be due to expiry
			if now.Sub(obs.ObservedAt) > 30*time.Minute {
				if !reasonSet[MissReasonCacheExpired] {
					reasons = append(reasons, MissReasonCacheExpired)
					reasonSet[MissReasonCacheExpired] = true
				}
				break
			}
		}
	}

	// Check for prefix changes (if cached tokens differ significantly between observations)
	if len(observations) > 1 {
		for i := 1; i < len(observations); i++ {
			if observations[i].CachedTokens == 0 && observations[i-1].CachedTokens > 0 {
				if !reasonSet[MissReasonPrefixChanged] {
					reasons = append(reasons, MissReasonPrefixChanged)
					reasonSet[MissReasonPrefixChanged] = true
				}
				break
			}
		}
	}

	return reasons
}
