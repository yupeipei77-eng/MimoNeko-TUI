package cli

import (
	"flag"
	"fmt"
	"path/filepath"

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
		fmt.Fprintf(env.Stderr, "cache-report failed: %v\n", err)
		return 1
	}

	registryPath := cfg.Prefix.Cache.RegistryPath
	if !filepath.IsAbs(registryPath) {
		registryPath = filepath.Join(root, registryPath)
	}

	registry, err := NewCacheRegistryForCLI(registryPath, cfg.Prefix.Cache)
	if err != nil {
		fmt.Fprintf(env.Stderr, "cache-report failed: %v\n", err)
		return 1
	}

	report, err := registry.Report()
	if err != nil {
		fmt.Fprintf(env.Stderr, "cache-report failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "total_observations=%d\n", report.GlobalSummary.TotalObservations)
	fmt.Fprintf(env.Stdout, "total_tokens=%d\n", report.GlobalSummary.TotalTokens)
	fmt.Fprintf(env.Stdout, "cached_tokens=%d\n", report.GlobalSummary.TotalCachedTokens)
	fmt.Fprintf(env.Stdout, "hit_rate=%.4f\n", report.GlobalSummary.OverallHitRate)
	fmt.Fprintf(env.Stdout, "estimated_saving_percent=%.2f\n", report.GlobalSummary.EstimatedSavingPercent)
	fmt.Fprintf(env.Stdout, "fingerprint_count=%d\n", len(report.ByFingerprint))

	for _, fp := range report.ByFingerprint {
		fmt.Fprintf(env.Stdout, "  fingerprint=%s hit_rate=%.4f reuse_count=%d uncached_tokens=%d\n",
			fp.PrefixHash, fp.HitRate, fp.ReuseCount, fp.UncachedTokens)
	}

	return 0
}

func init() {
	commands.Register(&CacheReportCommand{})
}
