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

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: reasonforge <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  version   Print version information")
	fmt.Fprintln(w, "  init      Create local .reasonforge config files")
	fmt.Fprintln(w, "  doctor    Validate local ReasonForge configuration")
}

func rejectExtraArgs(fs *flag.FlagSet, env Env) bool {
	if fs.NArg() == 0 {
		return false
	}

	fmt.Fprintf(env.Stderr, "%s accepts no positional arguments: %s\n", fs.Name(), strings.Join(fs.Args(), " "))
	return true
}
