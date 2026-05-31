package cache

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/nekonomimo/nekonomimo/internal/prefix"
)

func TestRecordAndLookup(t *testing.T) {
	reg, err := NewJSONLCacheRegistry(t.TempDir() + "/cache.jsonl")
	if err != nil {
		t.Fatalf("NewJSONLCacheRegistry() error: %v", err)
	}

	fp := prefix.Fingerprint{SHA256: "abc123", Version: 1}
	obs := Observation{
		Fingerprint:     fp,
		Provider:        "openai",
		Model:           "gpt-4",
		RequestID:       "req-1",
		InputTokens:     1000,
		CachedTokens:    800,
		ObservedAt:      time.Now().UTC(),
		ProviderCacheID: "cache-1",
	}

	if err := reg.Record(context.Background(), obs); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	entry, found, err := reg.Lookup(context.Background(), fp)
	if err != nil {
		t.Fatalf("Lookup() error: %v", err)
	}
	if !found {
		t.Fatal("Lookup() not found, want found")
	}
	if entry.Fingerprint.SHA256 != "abc123" {
		t.Errorf("Entry fingerprint = %q, want abc123", entry.Fingerprint.SHA256)
	}
}

func TestLookupNotFound(t *testing.T) {
	reg, err := NewJSONLCacheRegistry(t.TempDir() + "/cache.jsonl")
	if err != nil {
		t.Fatalf("NewJSONLCacheRegistry() error: %v", err)
	}

	fp := prefix.Fingerprint{SHA256: "nonexistent", Version: 1}
	_, found, err := reg.Lookup(context.Background(), fp)
	if err != nil {
		t.Fatalf("Lookup() error: %v", err)
	}
	if found {
		t.Error("Lookup() found, want not found")
	}
}

func TestMultipleProviders(t *testing.T) {
	reg, err := NewJSONLCacheRegistry(t.TempDir() + "/cache.jsonl")
	if err != nil {
		t.Fatalf("NewJSONLCacheRegistry() error: %v", err)
	}

	fp := prefix.Fingerprint{SHA256: "shared", Version: 1}
	_ = reg.Record(context.Background(), Observation{
		Fingerprint: fp, Provider: "openai", Model: "gpt-4", InputTokens: 100, CachedTokens: 80, ObservedAt: time.Now().UTC(),
	})
	_ = reg.Record(context.Background(), Observation{
		Fingerprint: fp, Provider: "anthropic", Model: "claude-3", InputTokens: 100, CachedTokens: 90, ObservedAt: time.Now().UTC().Add(1 * time.Second),
	})

	entry, found, _ := reg.Lookup(context.Background(), fp)
	if !found {
		t.Fatal("Lookup() not found")
	}
	if len(entry.Refs) != 2 {
		t.Errorf("Entry has %d refs, want 2", len(entry.Refs))
	}
}

func TestLastSeenAtUpdated(t *testing.T) {
	reg, err := NewJSONLCacheRegistry(t.TempDir() + "/cache.jsonl")
	if err != nil {
		t.Fatalf("NewJSONLCacheRegistry() error: %v", err)
	}

	fp := prefix.Fingerprint{SHA256: "timecheck", Version: 1}
	t1 := time.Now().UTC().Add(-1 * time.Hour)
	t2 := time.Now().UTC()

	_ = reg.Record(context.Background(), Observation{
		Fingerprint: fp, Provider: "p1", InputTokens: 100, CachedTokens: 50, ObservedAt: t1,
	})
	_ = reg.Record(context.Background(), Observation{
		Fingerprint: fp, Provider: "p1", InputTokens: 100, CachedTokens: 80, ObservedAt: t2,
	})

	entry, _, _ := reg.Lookup(context.Background(), fp)
	if !entry.LastSeenAt.Equal(t2) {
		t.Errorf("LastSeenAt = %v, want %v", entry.LastSeenAt, t2)
	}
}

func TestJSONLPersistence(t *testing.T) {
	path := t.TempDir() + "/cache.jsonl"
	reg1, _ := NewJSONLCacheRegistry(path)

	fp := prefix.Fingerprint{SHA256: "persist", Version: 1}
	_ = reg1.Record(context.Background(), Observation{
		Fingerprint: fp, Provider: "openai", InputTokens: 500, CachedTokens: 400, ObservedAt: time.Now().UTC(),
	})

	// Create a new registry instance pointing to the same file
	reg2, _ := NewJSONLCacheRegistry(path)
	entry, found, _ := reg2.Lookup(context.Background(), fp)
	if !found {
		t.Fatal("Lookup() on new instance not found, want found")
	}
	if entry.Fingerprint.SHA256 != "persist" {
		t.Errorf("Fingerprint = %q, want persist", entry.Fingerprint.SHA256)
	}
}

// Test 6 (cache): JSONL file permissions are 0600
func TestCacheRegistryFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support Unix permission bits")
	}
	path := t.TempDir() + "/cache.jsonl"
	reg, _ := NewJSONLCacheRegistry(path)

	_ = reg.Record(context.Background(), Observation{
		Fingerprint: prefix.Fingerprint{SHA256: "test", Version: 1},
		Provider:    "openai", InputTokens: 100, CachedTokens: 80, ObservedAt: time.Now().UTC(),
	})

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("File permissions = %o, want 0600", perm)
	}
}

// Test 7 (cache): Malformed JSONL lines are counted in CorruptLineCount
func TestCacheRegistryMalformedJSONLCounted(t *testing.T) {
	path := t.TempDir() + "/cache.jsonl"
	reg, _ := NewJSONLCacheRegistry(path)

	// Write a valid observation
	_ = reg.Record(context.Background(), Observation{
		Fingerprint: prefix.Fingerprint{SHA256: "valid", Version: 1},
		Provider:    "p1", InputTokens: 100, CachedTokens: 80, ObservedAt: time.Now().UTC(),
	})

	// Manually append malformed lines
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("OpenFile() error: %v", err)
	}
	f.WriteString("not json\n")
	f.WriteString("also not json\n")
	f.Close()

	report, err := reg.Report(context.Background())
	if err != nil {
		t.Fatalf("Report() error: %v", err)
	}

	if report.GlobalSummary.CorruptLineCount != 2 {
		t.Errorf("CorruptLineCount = %d, want 2", report.GlobalSummary.CorruptLineCount)
	}
}

// Test 8: Cache TTL is read from configuration via SetTTL
func TestCacheTTLFromConfig(t *testing.T) {
	path := t.TempDir() + "/cache.jsonl"
	reg, _ := NewJSONLCacheRegistry(path)

	// Default TTL should be 1 hour
	defaultTTL := reg.estimatedTTL()
	if defaultTTL != 1*time.Hour {
		t.Errorf("Default TTL = %v, want 1h", defaultTTL)
	}

	// Set custom TTL
	reg.SetTTL(30 * time.Minute)

	obsTime := time.Now().UTC()
	_ = reg.Record(context.Background(), Observation{
		Fingerprint:     prefix.Fingerprint{SHA256: "ttl_test", Version: 1},
		Provider:        "p1",
		InputTokens:     100,
		CachedTokens:    80,
		ObservedAt:      obsTime,
		ProviderCacheID: "cache-key",
	})

	entry, found, _ := reg.Lookup(context.Background(), prefix.Fingerprint{SHA256: "ttl_test", Version: 1})
	if !found {
		t.Fatal("Lookup() not found")
	}

	expectedExpiry := obsTime.Add(30 * time.Minute)
	if len(entry.Refs) == 0 {
		t.Fatal("Expected at least one ref")
	}
	if !entry.Refs[0].ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("ExpiresAt = %v, want %v (ObservedAt + 30m)", entry.Refs[0].ExpiresAt, expectedExpiry)
	}
}
