package cli

import (
	"fmt"
	"sort"

	"github.com/mimoneko/mimoneko/internal/approval"
	"github.com/mimoneko/mimoneko/internal/security"
)

// demoStore is a global in-memory store for demo/testing purposes.
// In production, this would be replaced with a persistent store.
var demoStore = approval.NewMemoryStore()

type ApprovalsCommand struct{}

func (c *ApprovalsCommand) Name() string { return "approvals" }

func (c *ApprovalsCommand) Run(args []string, env Env) int {
	if len(args) == 0 {
		printApprovalsHelp(env)
		return 0
	}

	switch args[0] {
	case "list":
		return c.runList(args[1:], env)
	case "show":
		return c.runShow(args[1:], env)
	case "approve":
		return c.runApprove(args[1:], env)
	case "reject":
		return c.runReject(args[1:], env)
	default:
		fmt.Fprintf(env.Stderr, "未知命令 '%s'\n\n", args[0])
		printApprovalsHelp(env)
		return 1
	}
}

func printApprovalsHelp(env Env) {
	fmt.Fprintln(env.Stdout, "用法: mimoneko approvals <命令>")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "命令:")
	fmt.Fprintln(env.Stdout, "  list              列出所有待审批请求")
	fmt.Fprintln(env.Stdout, "  show <id>         显示审批请求详情")
	fmt.Fprintln(env.Stdout, "  approve <id>      批准请求")
	fmt.Fprintln(env.Stdout, "  reject <id>       拒绝请求")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "示例:")
	fmt.Fprintln(env.Stdout, "  mimoneko approvals list")
	fmt.Fprintln(env.Stdout, "  mimoneko approvals show apr_xxx")
	fmt.Fprintln(env.Stdout, "  mimoneko approvals approve apr_xxx")
	fmt.Fprintln(env.Stdout, "  mimoneko approvals reject apr_xxx")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "注意: 当前是 stub 实现，不接 Runtime，不持久化。")
}

func (c *ApprovalsCommand) runList(args []string, env Env) int {
	requests := demoStore.List()

	if len(requests) == 0 {
		fmt.Fprintln(env.Stdout, "no pending approvals")
		return 0
	}

	// Sort by creation time
	sort.Slice(requests, func(i, j int) bool {
		return requests[i].CreatedAt.Before(requests[j].CreatedAt)
	})

	fmt.Fprintln(env.Stdout, "Approval Requests")
	fmt.Fprintf(env.Stdout, "%-36s %-14s %-12s %-10s %-20s %s\n", "ID", "TOOL", "SCOPE", "STATUS", "CREATED", "REASON")
	for _, req := range requests {
		fmt.Fprintf(env.Stdout, "%-36s %-14s %-12s %-10s %-20s %s\n",
			req.ID,
			req.ToolName,
			req.Scope,
			req.Status,
			req.CreatedAt.Format("2006-01-02 15:04:05"),
			security.SanitizeText(truncateString(req.Reason, 50)),
		)
	}
	return 0
}

func (c *ApprovalsCommand) runShow(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "用法: mimoneko approvals show <id>")
		return 1
	}

	id := args[0]
	req, err := demoStore.Get(id)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "ID: %s\n", req.ID)
	fmt.Fprintf(env.Stdout, "Run ID: %s\n", req.RunID)
	fmt.Fprintf(env.Stdout, "Tool: %s\n", req.ToolName)
	fmt.Fprintf(env.Stdout, "Scope: %s\n", req.Scope)
	fmt.Fprintf(env.Stdout, "Status: %s\n", req.Status)
	fmt.Fprintf(env.Stdout, "Risk Level: %s\n", req.RiskLevel)
	fmt.Fprintf(env.Stdout, "Reason: %s\n", security.SanitizeText(req.Reason))
	if req.Path != "" {
		fmt.Fprintf(env.Stdout, "Path: %s\n", security.SanitizeText(req.Path))
	}
	if req.Command != "" {
		fmt.Fprintf(env.Stdout, "Command: %s\n", security.SanitizeText(req.Command))
	}
	if req.PatchID != "" {
		fmt.Fprintf(env.Stdout, "Patch ID: %s\n", req.PatchID)
	}
	fmt.Fprintf(env.Stdout, "Created: %s\n", req.CreatedAt.Format("2006-01-02 15:04:05"))
	if !req.ExpiresAt.IsZero() {
		fmt.Fprintf(env.Stdout, "Expires: %s\n", req.ExpiresAt.Format("2006-01-02 15:04:05"))
	}
	if !req.DecidedAt.IsZero() {
		fmt.Fprintf(env.Stdout, "Decided: %s\n", req.DecidedAt.Format("2006-01-02 15:04:05"))
	}
	if req.DecidedBy != "" {
		fmt.Fprintf(env.Stdout, "Decided By: %s\n", req.DecidedBy)
	}

	return 0
}

func (c *ApprovalsCommand) runApprove(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "用法: mimoneko approvals approve <id>")
		return 1
	}

	id := args[0]
	req, err := demoStore.Get(id)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	if err := req.Approve("cli-user"); err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	if err := demoStore.Update(req); err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "✓ 已批准 %s\n", id)
	return 0
}

func (c *ApprovalsCommand) runReject(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "用法: mimoneko approvals reject <id>")
		return 1
	}

	id := args[0]
	req, err := demoStore.Get(id)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	if err := req.Reject("cli-user"); err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	if err := demoStore.Update(req); err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "✓ 已拒绝 %s\n", id)
	return 0
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	commands.Register(&ApprovalsCommand{})
}
