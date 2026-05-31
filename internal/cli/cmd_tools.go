package cli

import (
	"flag"
	"fmt"

	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/tools"
)

type ToolsCommand struct{}

func (c *ToolsCommand) Name() string { return "tools" }

func (c *ToolsCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("tools", flag.ContinueOnError)
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
		fmt.Fprintf(env.Stderr, "tools failed: %v\n", err)
		return 1
	}

	registry := tools.NewMemoryRegistry()
	testCmds := tools.TestCommandsFromConfig(cfg)

	if err := tools.RegisterBuiltinTools(registry, testCmds); err != nil {
		fmt.Fprintf(env.Stderr, "tools failed: %v\n", err)
		return 1
	}

	enabledMap := tools.EnabledToolsFromConfig(cfg)

	fmt.Fprintln(env.Stdout, "MimoNeko Tools")
	for _, info := range registry.List() {
		enabled := "true"
		if e, ok := enabledMap[info.Name]; ok && !e {
			enabled = "false"
		}
		fmt.Fprintf(env.Stdout, "%-12s enabled=%-5s risk=%s\n", info.Name, enabled, info.RiskLevel)
	}
	return 0
}

func init() {
	commands.Register(&ToolsCommand{})
}
