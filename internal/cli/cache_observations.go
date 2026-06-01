package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mimoneko/mimoneko/internal/cache"
	"github.com/mimoneko/mimoneko/internal/config"
)

func cacheRegistryPath(root string, cfg *config.Root) string {
	path := cfg.Prefix.Cache.RegistryPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return path
}

func countCacheObservationLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count
}

func readCacheObservations(path string) []cache.Observation {
	return readCacheObservationsAfter(path, 0)
}

func readCacheObservationsAfter(path string, skip int) []cache.Observation {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var observations []cache.Observation
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if lineNo <= skip {
			continue
		}
		var obs cache.Observation
		if err := json.Unmarshal(scanner.Bytes(), &obs); err == nil {
			observations = append(observations, obs)
		}
	}
	return observations
}

func sumCacheObservations(observations []cache.Observation) (inputTokens int, cachedTokens int) {
	for _, obs := range observations {
		inputTokens += obs.InputTokens
		cachedTokens += obs.CachedTokens
	}
	return inputTokens, cachedTokens
}
