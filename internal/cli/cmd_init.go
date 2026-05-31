package cli

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/mimoneko/mimoneko/internal/config"
)

type InitCommand struct{}

func (c *InitCommand) Name() string { return "init" }

func (c *InitCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	repair := fs.Bool("repair", false, "repair missing MimoNeko scaffolding without overwriting existing files")
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

	result, err := config.InitDetailed(root)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	if *repair {
		fmt.Fprintf(env.Stdout, "Repaired MimoNeko scaffolding at %s\n", config.ConfigDir(root))
	} else if len(result.Created) == 0 {
		fmt.Fprintf(env.Stdout, "MimoNeko already initialized at %s\n", config.ConfigDir(root))
	} else {
		fmt.Fprintf(env.Stdout, "Initialized MimoNeko at %s\n", config.ConfigDir(root))
	}
	for _, path := range result.Created {
		fmt.Fprintf(env.Stdout, "created %s\n", filepath.ToSlash(path))
	}
	for _, path := range result.Skipped {
		fmt.Fprintf(env.Stdout, "skipped %s\n", filepath.ToSlash(path))
	}
	printInitNextSteps(env.Stdout)
	return 0
}

func init() {
	commands.Register(&InitCommand{})
}
