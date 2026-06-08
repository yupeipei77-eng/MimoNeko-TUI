package cli

import (
	"flag"
	"fmt"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/tools"
)

type ToolsCommand struct{}

func (c *ToolsCommand) Name() string { return "tools" }

func (c *ToolsCommand) Run(args []string, env Env) int {
	return runToolsList(args, env)
}

func runToolsList(args []string, env Env) int {
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
	for _, metadata := range tools.ListToolMetadata(registry) {
		enabled := "true"
		if e, ok := enabledMap[metadata.Name]; ok && !e {
			enabled = "false"
		}
		fmt.Fprintf(
			env.Stdout,
			"%-12s risk=%-8s approval=%-5t timeout=%s enabled=%s\n",
			metadata.Name,
			metadata.RiskLevel,
			metadata.RequiresApproval,
			metadata.Timeout,
			enabled,
		)
	}
	return 0
}

func init() {
	commands.Register(&ToolsCommand{})
}
