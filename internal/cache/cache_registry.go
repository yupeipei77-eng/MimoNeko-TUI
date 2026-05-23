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
	Fingerprint     prefix.Fingerprint
	Provider        string
	Model           string
	RequestID       string
	InputTokens     int
	CachedTokens    int
	ObservedAt      time.Time
	ProviderCacheID string
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
