package cli

import (
	"flag"
	"fmt"
	"sort"

	"github.com/mimoneko/mimoneko/internal/cache"
	"github.com/mimoneko/mimoneko/internal/config"
)

type CacheReportCommand struct{}

func (c *CacheReportCommand) Name() string { return "cache-report" }

func (c *CacheReportCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("cache-report", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		PrintErrorDetails(env.Stderr, "Cache report failed", "加载项目配置失败。", "运行: mimoneko init", err.Error())
		return 1
	}

	registryPath := cacheRegistryPath(root, cfg)
	registry, err := NewCacheRegistryForCLI(registryPath, cfg.Prefix.Cache)
	if err != nil {
		PrintErrorDetails(env.Stderr, "Cache report failed", "无法打开缓存 registry。", "确认当前目录已初始化。", err.Error())
		return 1
	}

	report, err := registry.Report()
	if err != nil {
		PrintErrorDetails(env.Stderr, "Cache report failed", "读取缓存统计失败。", "检查 cache registry 文件是否损坏。", err.Error())
		return 1
	}

	PrintHeader(env.Stdout, "Cache Report")
	PrintKV(env.Stdout, "", []KV{
		{Key: "Total Requests", Value: fmt.Sprintf("%d", report.GlobalSummary.TotalObservations)},
		{Key: "Input Tokens", Value: fmt.Sprintf("%d", report.GlobalSummary.TotalTokens)},
		{Key: "Cached Tokens", Value: fmt.Sprintf("%d", report.GlobalSummary.TotalCachedTokens)},
		{Key: "MIMO Native Samples", Value: fmt.Sprintf("%d", report.GlobalSummary.NativeCacheObservations)},
		{Key: "MIMO Cache Hit", Value: fmt.Sprintf("%d", report.GlobalSummary.TotalCacheHitTokens)},
		{Key: "MIMO Cache Miss", Value: fmt.Sprintf("%d", report.GlobalSummary.TotalCacheMissTokens)},
		{Key: "Hit Rate", Value: percent(report.GlobalSummary.OverallHitRate)},
		{Key: "Fingerprints", Value: fmt.Sprintf("%d", len(report.ByFingerprint))},
	})
	observations := readCacheObservations(registryPath)
	if len(observations) > 0 {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, "Trend:")
		for _, row := range cacheTrendRows(observations) {
			fmt.Fprintf(env.Stdout, "%-4s %s\n", row.Key, row.Value)
		}
	}
	fmt.Fprintln(env.Stdout)
	PrintInfo(env.Stdout, "MIMO hit rate uses prompt_cache_hit_tokens / (hit + miss) when available.")
	PrintInfo(env.Stdout, "Other providers fall back to prompt_tokens_details.cached_tokens.")

	return 0
}

func init() {
	commands.Register(&CacheReportCommand{})
}

func cacheTrendRows(observations []cache.Observation) []KV {
	if len(observations) == 0 {
		return nil
	}
	sort.SliceStable(observations, func(i, j int) bool {
		return observations[i].ObservedAt.Before(observations[j].ObservedAt)
	})
	targets := []int{1, 10, 30, 50}
	seen := make(map[int]bool)
	var rows []KV
	for _, target := range targets {
		if target <= len(observations) {
			inputTokens, cachedTokens := sumCacheObservations(observations[:target])
			rate := 0.0
			if inputTokens > 0 {
				rate = float64(cachedTokens) / float64(inputTokens)
			}
			rows = append(rows, KV{Key: fmt.Sprintf("%d", target), Value: percent(rate)})
			seen[target] = true
		}
	}
	if !seen[len(observations)] {
		inputTokens, cachedTokens := sumCacheObservations(observations)
		rate := 0.0
		if inputTokens > 0 {
			rate = float64(cachedTokens) / float64(inputTokens)
		}
		rows = append(rows, KV{Key: fmt.Sprintf("%d", len(observations)), Value: percent(rate)})
	}
	return rows
}
