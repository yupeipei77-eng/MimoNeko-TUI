package cli

import (
	"fmt"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"
)

type SandboxCommand struct{}

func (c *SandboxCommand) Name() string { return "sandbox" }

func (c *SandboxCommand) Run(args []string, env Env) int {
	if len(args) == 0 {
		printSandboxHelp(env)
		return 0
	}

	switch args[0] {
	case "check":
		return c.runCheck(args[1:], env)
	default:
		fmt.Fprintf(env.Stderr, "未知命令 '%s'\n\n", args[0])
		printSandboxHelp(env)
		return 1
	}
}

func printSandboxHelp(env Env) {
	fmt.Fprintln(env.Stdout, "用法: mimoneko sandbox <命令>")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "命令:")
	fmt.Fprintln(env.Stdout, "  check <path>  检查路径是否敏感（仅检测，不阻断）")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "示例:")
	fmt.Fprintln(env.Stdout, "  mimoneko sandbox check .git/config")
	fmt.Fprintln(env.Stdout, "  mimoneko sandbox check README.md")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "注意: 此命令仅用于检测，不会阻断任何操作。")
}

func (c *SandboxCommand) runCheck(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "用法: mimoneko sandbox check <path>")
		return 1
	}

	path := args[0]
	violations := security.ValidatePath(path)

	if len(violations) == 0 {
		fmt.Fprintln(env.Stdout, "allowed")
		return 0
	}

	summary := security.GetViolationSummary(violations)
	fmt.Fprintln(env.Stdout, summary)

	for _, v := range violations {
		fmt.Fprintf(env.Stdout, "  - rule: %s\n", v.Rule)
		fmt.Fprintf(env.Stdout, "    severity: %s\n", v.Severity)
		fmt.Fprintf(env.Stdout, "    candidate: %v\n", v.Candidate)
	}

	return 0
}

func init() {
	commands.Register(&SandboxCommand{})
}
