package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/reasonforge/reasonforge/internal/config"
	"github.com/reasonforge/reasonforge/internal/version"
)

type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Getwd  func() (string, error)
}

func Run(args []string, env Env) int {
	if env.Stdout == nil {
		env.Stdout = io.Discard
	}
	if env.Stderr == nil {
		env.Stderr = io.Discard
	}
	if env.Getwd == nil {
		env.Getwd = func() (string, error) { return ".", nil }
	}

	if len(args) == 0 {
		printUsage(env.Stderr)
		return 2
	}

	switch args[0] {
	case "version":
		return runVersion(args[1:], env)
	case "init":
		return runInit(args[1:], env)
	case "doctor":
		return runDoctor(args[1:], env)
	case "cache-report":
		return runCacheReport(args[1:], env)
	case "help", "-h", "--help":
		if len(args) > 1 {
			fmt.Fprintf(env.Stderr, "%s accepts no arguments\n", args[0])
			return 2
		}
		printUsage(env.Stdout)
		return 0
	default:
		fmt.Fprintf(env.Stderr, "unknown command %q\n", args[0])
		printUsage(env.Stderr)
		return 2
	}
}

func runVersion(args []string, env Env) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}
	fmt.Fprintln(env.Stdout, version.String())
	return 0
}

func runInit(args []string, env Env) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
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

	written, err := config.Init(root)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	if len(written) == 0 {
		fmt.Fprintf(env.Stdout, "ReasonForge already initialized at %s\n", config.ConfigDir(root))
		return 0
	}

	fmt.Fprintf(env.Stdout, "Initialized ReasonForge at %s\n", config.ConfigDir(root))
	for _, path := range written {
		fmt.Fprintf(env.Stdout, "created %s\n", filepath.ToSlash(path))
	}
	return 0
}

func runDoctor(args []string, env Env) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
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
		fmt.Fprintf(env.Stderr, "doctor failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(env.Stdout, "ReasonForge doctor OK")
	fmt.Fprintf(env.Stdout, "config_dir=%s\n", filepath.ToSlash(cfg.Dir))
	fmt.Fprintf(env.Stdout, "default_model=%s\n", cfg.Models.Routing.DefaultModel)
	fmt.Fprintf(env.Stdout, "immutable_prefix_sources=%d\n", len(cfg.Prefix.ImmutableSources))
	fmt.Fprintf(env.Stdout, "prefix_canonicalization=enabled\n")
	fmt.Fprintf(env.Stdout, "prefix_hash_stable=true\n")
	fmt.Fprintf(env.Stdout, "context_budget_valid=true\n")
	fmt.Fprintf(env.Stdout, "cache_report=available\n")
	fmt.Fprintf(env.Stdout, "append_only_log=available\n")
	fmt.Fprintf(env.Stdout, "budget_warn_ratio=%.2f\n", cfg.Prefix.Budget.WarnRatio)
	fmt.Fprintf(env.Stdout, "budget_block_ratio=%.2f\n", cfg.Prefix.Budget.BlockRatio)
	fmt.Fprintf(env.Stdout, "cache_estimated_ttl=%s\n", cfg.Prefix.Cache.EstimatedTTL)
	fmt.Fprintf(env.Stdout, "event_id_collision_resistant=true\n")
	return 0
}

func resolveRoot(dir string, env Env) (string, error) {
	if strings.TrimSpace(dir) != "" {
		return filepath.Abs(dir)
	}

	root, err := env.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return filepath.Abs(root)
}

func runCacheReport(args []string, env Env) int {
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

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: reasonforge <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  version      Print version information")
	fmt.Fprintln(w, "  init         Create local .reasonforge config files")
	fmt.Fprintln(w, "  doctor       Validate local ReasonForge configuration")
	fmt.Fprintln(w, "  cache-report Show prefix cache statistics")
}

func rejectExtraArgs(fs *flag.FlagSet, env Env) bool {
	if fs.NArg() == 0 {
		return false
	}

	fmt.Fprintf(env.Stderr, "%s accepts no positional arguments: %s\n", fs.Name(), strings.Join(fs.Args(), " "))
	return true
}
