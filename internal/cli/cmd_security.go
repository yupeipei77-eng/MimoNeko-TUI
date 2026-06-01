package cli

import (
	"fmt"

	"github.com/mimoneko/mimoneko/internal/security"
)

type SecurityCommand struct{}

func (c *SecurityCommand) Name() string { return "security" }

func (c *SecurityCommand) Run(args []string, env Env) int {
	if len(args) == 0 {
		printSecurityHelp(env)
		return 0
	}

	switch args[0] {
	case "status":
		return c.runStatus(args[1:], env)
	case "check":
		return c.runCheck(args[1:], env)
	default:
		fmt.Fprintf(env.Stderr, "未知命令 '%s'\n\n", args[0])
		printSecurityHelp(env)
		return 1
	}
}

func printSecurityHelp(env Env) {
	fmt.Fprintln(env.Stdout, "用法: mimoneko security <命令>")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "命令:")
	fmt.Fprintln(env.Stdout, "  status        查看当前安全配置状态")
	fmt.Fprintln(env.Stdout, "  check <path>  检查路径安全（考虑当前模式）")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "示例:")
	fmt.Fprintln(env.Stdout, "  mimoneko security status")
	fmt.Fprintln(env.Stdout, "  mimoneko security check .git/config")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "环境变量:")
	fmt.Fprintln(env.Stdout, "  MIMONEKO_SECURITY_MODE=off|warn|enforce  设置安全模式")
}

func (c *SecurityCommand) runStatus(args []string, env Env) int {
	mode := security.GetEnforcementMode()

	fmt.Fprintln(env.Stdout, "Security Status")
	fmt.Fprintln(env.Stdout, "===============")
	fmt.Fprintf(env.Stdout, "Mode: %s\n", mode)
	fmt.Fprintf(env.Stdout, "Enforcement Enabled: %v\n", mode == security.ModeEnforce)
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "Sandbox Rules:")
	fmt.Fprintf(env.Stdout, "  Total rules: %d\n", len(security.GetSensitiveRules()))
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "Modes:")
	fmt.Fprintln(env.Stdout, "  off     - No blocking, audit candidate only")
	fmt.Fprintln(env.Stdout, "  warn    - No blocking, emit security.warning")
	fmt.Fprintln(env.Stdout, "  enforce - Block critical paths, require approval for high-risk tools")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintf(env.Stdout, "Current mode: %s\n", mode)

	return 0
}

func (c *SecurityCommand) runCheck(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "用法: mimoneko security check <path>")
		return 1
	}

	path := args[0]
	mode := security.GetEnforcementMode()

	// Get violations
	violations := security.ValidatePath(path)

	// Check enforcement result
	config := security.NewEnforcementConfig()
	result := config.CheckToolExecution(
		"manual_check",
		"low", // Default to low risk for manual checks
		false,
		[]string{"path"},
		map[string]string{"path": path},
	)

	fmt.Fprintf(env.Stdout, "Path: %s\n", path)
	fmt.Fprintf(env.Stdout, "Mode: %s\n", mode)
	fmt.Fprintln(env.Stdout, "")

	if len(violations) == 0 {
		fmt.Fprintln(env.Stdout, "Status: allowed")
		fmt.Fprintln(env.Stdout, "No violations detected.")
	} else {
		fmt.Fprintf(env.Stdout, "Status: %s\n", result.EventType)
		fmt.Fprintf(env.Stdout, "Would block: %v\n", !result.Allowed)
		fmt.Fprintf(env.Stdout, "Would warn: %v\n", result.ShouldWarn)
		fmt.Fprintln(env.Stdout, "")
		fmt.Fprintln(env.Stdout, "Violations:")
		for _, v := range violations {
			fmt.Fprintf(env.Stdout, "  - rule: %s\n", v.Rule)
			fmt.Fprintf(env.Stdout, "    severity: %s\n", v.Severity)
			fmt.Fprintf(env.Stdout, "    candidate: %v\n", v.Candidate)
		}
	}

	return 0
}

func init() {
	commands.Register(&SecurityCommand{})
}
