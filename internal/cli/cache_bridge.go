package cli

import (
	"context"
	"time"

	"github.com/reasonforge/reasonforge/internal/cache"
	"github.com/reasonforge/reasonforge/internal/config"
)

// cacheRegistryBridge wraps JSONLCacheRegistry to expose Report for CLI use.
type cacheRegistryBridge struct {
	registry *cache.JSONLCacheRegistry
}

// NewCacheRegistryForCLI creates a cache registry bridge for the CLI.
// If the config specifies an estimated_ttl, it is applied to the registry.
func NewCacheRegistryForCLI(path string, cacheCfg config.PrefixCacheConfig) (*cacheRegistryBridge, error) {
	registry, err := cache.NewJSONLCacheRegistry(path)
	if err != nil {
		return nil, err
	}
	if cacheCfg.EstimatedTTL != "" {
		if ttl, err := time.ParseDuration(cacheCfg.EstimatedTTL); err == nil {
			registry.SetTTL(ttl)
		}
	}
	return &cacheRegistryBridge{registry: registry}, nil
}

// Report returns the cache report from the underlying registry.
func (b *cacheRegistryBridge) Report() (cache.CacheReport, error) {
	return b.registry.Report(context.Background())
}
