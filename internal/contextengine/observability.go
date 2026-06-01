package contextengine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

const (
	LayerImmutablePrefix = "immutable_prefix"
	LayerSemiStable      = "semi_stable_context"
	LayerVolatile        = "volatile_context"
)

type ObservableEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ObservableSnapshot struct {
	ImmutablePrefix []ObservableEntry `json:"immutable_prefix"`
	SemiStable      []ObservableEntry `json:"semi_stable_context"`
	Volatile        []ObservableEntry `json:"volatile_context"`
}

type CacheObservationStats struct {
	PrefixFingerprint     string  `json:"prefix_fingerprint"`
	ImmutableBytes        int     `json:"immutable_bytes"`
	SemiStableBytes       int     `json:"semi_stable_bytes"`
	VolatileBytes         int     `json:"volatile_bytes"`
	EstimatedCacheHitRate float64 `json:"estimated_cache_hit_ratio"`
	ImplementationStatus  string  `json:"implementation_status"`
}

func NewObservableSnapshot(immutable, semiStable, volatile map[string]string) ObservableSnapshot {
	return ObservableSnapshot{
		ImmutablePrefix: StableEntries(immutable),
		SemiStable:      StableEntries(semiStable),
		Volatile:        StableEntries(volatile),
	}
}

func DefaultObservableSnapshot(userInput string) ObservableSnapshot {
	return NewObservableSnapshot(
		map[string]string{
			"model_profile": "mimo-first",
			"repo_policy":   "project-config",
			"system_prompt": "project-config",
			"tool_schema":   "project-config",
		},
		map[string]string{
			"current_task_plan":  "stub",
			"dependency_summary": "unavailable",
			"repo_index":         "unavailable",
		},
		map[string]string{
			"current_diff":        "not_included",
			"error_logs":          "not_included",
			"recent_tool_outputs": "not_included",
			"user_input":          userInput,
		},
	)
}

func StableEntries(values map[string]string) []ObservableEntry {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	entries := make([]ObservableEntry, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, ObservableEntry{Key: key, Value: values[key]})
	}
	return entries
}

func (s ObservableSnapshot) PrefixFingerprint() string {
	sum := sha256.Sum256(StableLayerBytes(LayerImmutablePrefix, s.ImmutablePrefix))
	return hex.EncodeToString(sum[:])
}

func (s ObservableSnapshot) Stats() CacheObservationStats {
	immutableBytes := len(StableLayerBytes(LayerImmutablePrefix, s.ImmutablePrefix))
	semiStableBytes := len(StableLayerBytes(LayerSemiStable, s.SemiStable))
	volatileBytes := len(StableLayerBytes(LayerVolatile, s.Volatile))
	return CacheObservationStats{
		PrefixFingerprint:     s.PrefixFingerprint(),
		ImmutableBytes:        immutableBytes,
		SemiStableBytes:       semiStableBytes,
		VolatileBytes:         volatileBytes,
		EstimatedCacheHitRate: estimateObservableCacheHitRatio(immutableBytes, semiStableBytes, volatileBytes),
		ImplementationStatus:  "stub",
	}
}

func StableLayerBytes(layer string, entries []ObservableEntry) []byte {
	sorted := append([]ObservableEntry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Key == sorted[j].Key {
			return sorted[i].Value < sorted[j].Value
		}
		return sorted[i].Key < sorted[j].Key
	})

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "layer:%s\n", layer)
	for _, entry := range sorted {
		fmt.Fprintf(&buf, "key:%d:%s\n", len([]byte(entry.Key)), entry.Key)
		fmt.Fprintf(&buf, "value:%d:%s\n", len([]byte(entry.Value)), entry.Value)
	}
	return buf.Bytes()
}

func estimateObservableCacheHitRatio(immutableBytes, semiStableBytes, volatileBytes int) float64 {
	total := immutableBytes + semiStableBytes + volatileBytes
	if total <= 0 {
		return 0
	}
	return float64(immutableBytes) / float64(total)
}
