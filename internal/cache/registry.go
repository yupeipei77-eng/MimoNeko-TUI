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

	"github.com/reasonforge/reasonforge/internal/prefix"
)

// JSONLCacheRegistry is a JSONL-backed implementation of CacheRegistry.
// Observations are appended to a single file and scanned on lookup.
type JSONLCacheRegistry struct {
	mu   sync.Mutex
	path string
}

// NewJSONLCacheRegistry creates a new registry at the given file path.
// The parent directory is created if it does not exist.
func NewJSONLCacheRegistry(path string) (*JSONLCacheRegistry, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache registry dir: %w", err)
	}
	return &JSONLCacheRegistry{path: path}, nil
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

	f, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open cache registry: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
		return fmt.Errorf("write observation: %w", err)
	}

	return f.Sync()
}

// Lookup returns the aggregated cache entry for a given fingerprint.
func (r *JSONLCacheRegistry) Lookup(ctx context.Context, fingerprint prefix.Fingerprint) (Entry, bool, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, false, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	observations, err := r.readAll()
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
			ExpiresAt: obs.ObservedAt.Add(1 * time.Hour), // estimated TTL
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

	observations, err := r.readAll()
	if err != nil {
		return CacheReport{}, err
	}

	return buildReport(observations, time.Now()), nil
}

func (r *JSONLCacheRegistry) readAll() ([]Observation, error) {
	f, err := os.Open(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open cache registry: %w", err)
	}
	defer f.Close()

	var observations []Observation
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var obs Observation
		if err := json.Unmarshal(scanner.Bytes(), &obs); err != nil {
			continue // skip malformed lines
		}
		observations = append(observations, obs)
	}

	return observations, scanner.Err()
}
