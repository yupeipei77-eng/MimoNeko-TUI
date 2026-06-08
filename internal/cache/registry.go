package cache

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/prefix"
)

// JSONLCacheRegistry is a JSONL-backed implementation of CacheRegistry.
// Observations are appended to a single file and scanned on lookup.
type JSONLCacheRegistry struct {
	mu   sync.Mutex
	path string
	ttl  time.Duration
}

// NewJSONLCacheRegistry creates a new registry at the given file path.
// The parent directory is created if it does not exist.
func NewJSONLCacheRegistry(path string) (*JSONLCacheRegistry, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create cache registry dir: %w", err)
	}
	return &JSONLCacheRegistry{path: path}, nil
}

// SetTTL configures the estimated cache TTL for ExpiresAt calculations.
func (r *JSONLCacheRegistry) SetTTL(ttl time.Duration) {
	r.ttl = ttl
}

// Record appends a cache observation to the registry file.
func (r *JSONLCacheRegistry) Record(ctx context.Context, observation Observation) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	line, err := json.Marshal(observation)
	if err != nil {
		return fmt.Errorf("marshal observation: %w", err)
	}

	f, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open cache registry: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
		return fmt.Errorf("write observation: %w", err)
	}

	return f.Sync()
}

// ReadStats contains statistics about a JSONL read operation.
type ReadStats struct {
	CorruptLineCount int
}

// Lookup returns the aggregated cache entry for a given fingerprint.
func (r *JSONLCacheRegistry) Lookup(ctx context.Context, fingerprint prefix.Fingerprint) (Entry, bool, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, false, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	observations, _, err := r.readAllWithStats()
	if err != nil {
		return Entry{}, false, err
	}

	var matching []Observation
	for _, obs := range observations {
		if obs.Fingerprint.SHA256 == fingerprint.SHA256 && obs.Fingerprint.Version == fingerprint.Version {
			matching = append(matching, obs)
		}
	}

	if len(matching) == 0 {
		return Entry{}, false, nil
	}

	// Aggregate refs: latest per provider+model
	refMap := make(map[string]ProviderCacheRef)
	var lastSeen time.Time
	for _, obs := range matching {
		key := obs.Provider + "/" + obs.Model
		ref := ProviderCacheRef{
			Provider:  obs.Provider,
			Model:     obs.Model,
			CacheKey:  obs.ProviderCacheID,
			ExpiresAt: obs.ObservedAt.Add(r.estimatedTTL()),
		}
		if existing, ok := refMap[key]; !ok || ref.ExpiresAt.After(existing.ExpiresAt) {
			refMap[key] = ref
		}
		if obs.ObservedAt.After(lastSeen) {
			lastSeen = obs.ObservedAt
		}
	}

	refs := make([]ProviderCacheRef, 0, len(refMap))
	for _, ref := range refMap {
		refs = append(refs, ref)
	}

	return Entry{
		Fingerprint: fingerprint,
		Refs:        refs,
		LastSeenAt:  lastSeen,
	}, true, nil
}

// Report generates a CacheReport with per-fingerprint and global statistics.
func (r *JSONLCacheRegistry) Report(ctx context.Context) (CacheReport, error) {
	if err := ctx.Err(); err != nil {
		return CacheReport{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	observations, stats, err := r.readAllWithStats()
	if err != nil {
		return CacheReport{}, err
	}

	report := buildReport(observations, time.Now())
	report.GlobalSummary.CorruptLineCount = stats.CorruptLineCount
	return report, nil
}

// estimatedTTL returns the configured estimated TTL, defaulting to 1 hour.
func (r *JSONLCacheRegistry) estimatedTTL() time.Duration {
	if r.ttl > 0 {
		return r.ttl
	}
	return 1 * time.Hour
}

func (r *JSONLCacheRegistry) readAll() ([]Observation, error) {
	obs, _, err := r.readAllWithStats()
	return obs, err
}

func (r *JSONLCacheRegistry) readAllWithStats() ([]Observation, ReadStats, error) {
	f, err := os.Open(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ReadStats{}, nil
		}
		return nil, ReadStats{}, fmt.Errorf("open cache registry: %w", err)
	}
	defer f.Close()

	var observations []Observation
	var corruptCount int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var obs Observation
		if err := json.Unmarshal(scanner.Bytes(), &obs); err != nil {
			corruptCount++
			continue
		}
		observations = append(observations, obs)
	}

	if err := scanner.Err(); err != nil {
		return nil, ReadStats{}, err
	}

	return observations, ReadStats{CorruptLineCount: corruptCount}, nil
}
