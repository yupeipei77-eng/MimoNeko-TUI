package cli

import (
	"flag"
	"fmt"

	"github.com/mimoneko/mimoneko/internal/version"
)

type VersionCommand struct{}

func (c *VersionCommand) Name() string { return "version" }

func (c *VersionCommand) Run(args []string, env Env) int {
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

func init() {
	commands.Register(&VersionCommand{})
}
