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
	globalHitTokens := 0
	globalMissTokens := 0
	globalNativeObservations := 0

	for _, hash := range hashes {
		group := groups[hash]
		report := buildFingerprintReport(hash, group, now)
		reports = append(reports, report)
		globalTotalTokens += report.TotalTokens
		globalCachedTokens += report.CachedTokens
		globalHitTokens += report.CacheHitTokens
		globalMissTokens += report.CacheMissTokens
		globalNativeObservations += report.NativeCacheObservations
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
			TotalObservations:       len(observations),
			TotalTokens:             globalTotalTokens,
			TotalCachedTokens:       globalCachedTokens,
			TotalCacheHitTokens:     globalHitTokens,
			TotalCacheMissTokens:    globalMissTokens,
			NativeCacheObservations: globalNativeObservations,
			OverallHitRate:          overallHitRate,
			EstimatedSavingPercent:  estimatedSaving,
		},
	}
}

func buildFingerprintReport(hash string, observations []Observation, now time.Time) PerFingerprintReport {
	totalTokens := 0
	cachedTokens := 0
	cacheHitTokens := 0
	cacheMissTokens := 0
	nativeObservations := 0

	for _, obs := range observations {
		total, hit, miss, native := observationCacheMetrics(obs)
		totalTokens += total
		cachedTokens += hit
		cacheHitTokens += hit
		cacheMissTokens += miss
		if native {
			nativeObservations++
		}
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
		PrefixHash:              hash,
		TotalTokens:             totalTokens,
		CachedTokens:            cachedTokens,
		CacheHitTokens:          cacheHitTokens,
		CacheMissTokens:         cacheMissTokens,
		NativeCacheObservations: nativeObservations,
		UncachedTokens:          uncachedTokens,
		HitRate:                 hitRate,
		EstimatedSavingPercent:  estimatedSaving,
		ReuseCount:              reuseCount,
		PossibleMissReasons:     missReasons,
	}
}

func observationCacheMetrics(obs Observation) (totalTokens, hitTokens, missTokens int, native bool) {
	if obs.NativeCacheKnown {
		hitTokens = obs.CacheHitTokens
		missTokens = obs.CacheMissTokens
		if hitTokens < 0 {
			hitTokens = 0
		}
		if missTokens < 0 {
			missTokens = 0
		}
		totalTokens = hitTokens + missTokens
		if totalTokens == 0 && obs.InputTokens > 0 {
			totalTokens = obs.InputTokens
			if hitTokens > totalTokens {
				hitTokens = totalTokens
			}
			missTokens = totalTokens - hitTokens
		}
		return totalTokens, hitTokens, missTokens, true
	}

	totalTokens = obs.InputTokens
	hitTokens = obs.CachedTokens
	if hitTokens < 0 {
		hitTokens = 0
	}
	if totalTokens < hitTokens {
		totalTokens = hitTokens
	}
	missTokens = totalTokens - hitTokens
	return totalTokens, hitTokens, missTokens, false
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
		total, hit, _, _ := observationCacheMetrics(obs)
		if !obs.ObservedAt.IsZero() && hit < total {
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
			_, currentHit, _, _ := observationCacheMetrics(observations[i])
			_, previousHit, _, _ := observationCacheMetrics(observations[i-1])
			if currentHit == 0 && previousHit > 0 {
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
