package cache

import (
	"context"
	"time"

	"github.com/reasonforge/reasonforge/internal/prefix"
)

type ProviderCacheRef struct {
	Provider  string
	Model     string
	CacheKey  string
	ExpiresAt time.Time
}

type Observation struct {
	Fingerprint         prefix.Fingerprint
	Provider            string
	Model               string
	RequestID           string
	InputTokens         int
	CachedTokens        int
	ObservedAt          time.Time
	ProviderCacheID     string
	Estimated           bool
	PrefixTokens        int
	ConversationTokens  int
	ScratchpadTokens    int
	CurrentInputTokens  int
}

type Entry struct {
	Fingerprint prefix.Fingerprint
	Refs        []ProviderCacheRef
	LastSeenAt  time.Time
}

type CacheRegistry interface {
	Lookup(ctx context.Context, fingerprint prefix.Fingerprint) (Entry, bool, error)
	Record(ctx context.Context, observation Observation) error
}

// MissReason describes why a cache miss may have occurred.
type MissReason string

const (
	MissReasonPrefixChanged      MissReason = "prefix_changed"
	MissReasonCacheExpired       MissReason = "cache_expired"
	MissReasonModelChanged       MissReason = "model_changed"
	MissReasonNoPriorObservation MissReason = "no_prior_observation"
)

// PerFingerprintReport is the cache statistics for a single prefix hash.
type PerFingerprintReport struct {
	PrefixHash             string
	TotalTokens            int
	CachedTokens           int
	UncachedTokens         int
	HitRate                float64
	EstimatedSavingPercent float64
	ReuseCount             int
	PossibleMissReasons    []MissReason
}

// CacheReport contains per-fingerprint statistics and a global summary.
type CacheReport struct {
	ByFingerprint []PerFingerprintReport
	GlobalSummary GlobalCacheSummary
}

// GlobalCacheSummary is the aggregate cache statistics across all fingerprints.
type GlobalCacheSummary struct {
	TotalObservations      int
	TotalTokens            int
	TotalCachedTokens      int
	OverallHitRate         float64
	EstimatedSavingPercent float64
	CorruptLineCount       int
}
