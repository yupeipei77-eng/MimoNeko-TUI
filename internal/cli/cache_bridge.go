package cli

import (
	"context"

	"github.com/reasonforge/reasonforge/internal/cache"
)

// cacheRegistryBridge wraps JSONLCacheRegistry to expose Report for CLI use.
type cacheRegistryBridge struct {
	registry *cache.JSONLCacheRegistry
}

// NewCacheRegistryForCLI creates a cache registry bridge for the CLI.
func NewCacheRegistryForCLI(path string) (*cacheRegistryBridge, error) {
	registry, err := cache.NewJSONLCacheRegistry(path)
	if err != nil {
		return nil, err
	}
	return &cacheRegistryBridge{registry: registry}, nil
}

// Report returns the cache report from the underlying registry.
func (b *cacheRegistryBridge) Report() (cache.CacheReport, error) {
	return b.registry.Report(context.Background())
}
