package cli

import (
	"fmt"
	"path/filepath"

	"github.com/mimoneko/mimoneko/internal/approval"
	"github.com/mimoneko/mimoneko/internal/security"
)

// approvalsStorePath returns the path to the approvals JSON file.
func approvalsStorePath(root string) string {
	return filepath.Join(root, ".mimoneko", "approvals.json")
}

// snapshotStorePath returns the path to the snapshots JSON file.
func snapshotStorePath(root string) string {
	return filepath.Join(root, ".mimoneko", "approval_snapshots.json")
}

// loadApprovalsStore loads the approvals file store from the project root.
func loadApprovalsStore(root string) (*approval.FileStore, error) {
	path := approvalsStorePath(root)
	store := approval.NewFileStore(path)
	if err := store.Load(); err != nil {
		return nil, fmt.Errorf("加载审批记录失败: %w", err)
	}
	return store, nil
}

// loadSnapshotStore loads the snapshot file store from the project root.
func loadSnapshotStore(root string) (*approval.SnapshotStore, error) {
	path := snapshotStorePath(root)
	store := approval.NewSnapshotStore(path)
	if err := store.Load(); err != nil {
		return nil, fmt.Errorf("加载快照失败: %w", err)
	}
	return store, nil
}

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
	case "snapshot":
		return c.runSnapshot(args[1:], env)
	case "preview":
		return c.runPreview(args[1:], env)
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
	fmt.Fprintln(env.Stdout, "  snapshot <id>     显示恢复快照（脱敏）")
	fmt.Fprintln(env.Stdout, "  preview <id>      预览批准后将执行的操作")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "示例:")
	fmt.Fprintln(env.Stdout, "  mimoneko approvals list")
	fmt.Fprintln(env.Stdout, "  mimoneko approvals show apr_xxx")
	fmt.Fprintln(env.Stdout, "  mimoneko approvals approve apr_xxx")
	fmt.Fprintln(env.Stdout, "  mimoneko approvals reject apr_xxx")
	fmt.Fprintln(env.Stdout, "  mimoneko approvals snapshot apr_xxx")
	fmt.Fprintln(env.Stdout, "  mimoneko approvals preview apr_xxx")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "存储路径: .mimoneko/approvals.json")
	fmt.Fprintln(env.Stdout, "快照路径: .mimoneko/approval_snapshots.json")
}

func (c *ApprovalsCommand) runList(args []string, env Env) int {
	root, err := resolveRoot("", env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	store, err := loadApprovalsStore(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	requests := store.List()

	if len(requests) == 0 {
		fmt.Fprintln(env.Stdout, "no pending approvals")
		return 0
	}

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

	root, err := resolveRoot("", env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	store, err := loadApprovalsStore(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	id := args[0]
	req, err := store.Get(id)
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

	root, err := resolveRoot("", env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	store, err := loadApprovalsStore(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	id := args[0]
	req, err := store.Get(id)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	if err := req.Approve("cli-user"); err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	if err := store.Update(req); err != nil {
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

	root, err := resolveRoot("", env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	store, err := loadApprovalsStore(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	id := args[0]
	req, err := store.Get(id)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	if err := req.Reject("cli-user"); err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	if err := store.Update(req); err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "✓ 已拒绝 %s\n", id)
	return 0
}

func (c *ApprovalsCommand) runSnapshot(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "用法: mimoneko approvals snapshot <id>")
		return 1
	}

	root, err := resolveRoot("", env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	store, err := loadSnapshotStore(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	id := args[0]
	snap, err := store.Get(id)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	fmt.Fprintf(env.Stdout, "Approval ID: %s\n", snap.ApprovalID)
	fmt.Fprintf(env.Stdout, "Run ID: %s\n", snap.RunID)
	fmt.Fprintf(env.Stdout, "Tool: %s\n", snap.ToolName)
	fmt.Fprintf(env.Stdout, "Risk Level: %s\n", snap.RiskLevel)
	fmt.Fprintf(env.Stdout, "Reason: %s\n", security.SanitizeText(snap.Reason))
	if snap.Path != "" {
		fmt.Fprintf(env.Stdout, "Path: %s\n", security.SanitizeText(snap.Path))
	}
	if snap.Command != "" {
		fmt.Fprintf(env.Stdout, "Command: %s\n", security.SanitizeText(snap.Command))
	}
	fmt.Fprintf(env.Stdout, "Created: %s\n", snap.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "Preview (sanitized):")
	fmt.Fprintln(env.Stdout, snap.SanitizedPreview)

	return 0
}

func (c *ApprovalsCommand) runPreview(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "用法: mimoneko approvals preview <id>")
		return 1
	}

	root, err := resolveRoot("", env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	// Load approval
	approvalStore, err := loadApprovalsStore(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	id := args[0]
	req, err := approvalStore.Get(id)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	// Check approval status
	switch req.Status {
	case approval.StatusPending:
		fmt.Fprintf(env.Stdout, "approval still pending: %s\n", id)
		return 0
	case approval.StatusRejected:
		fmt.Fprintf(env.Stdout, "approval rejected: %s\n", id)
		return 0
	case approval.StatusExpired:
		fmt.Fprintf(env.Stdout, "approval expired: %s\n", id)
		return 0
	case approval.StatusApproved:
		// Allow preview
	default:
		fmt.Fprintf(env.Stdout, "unknown status: %s\n", req.Status)
		return 1
	}

	// Load snapshot
	snapshotStore, err := loadSnapshotStore(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: %v\n", err)
		return 1
	}

	snap, err := snapshotStore.Get(id)
	if err != nil {
		fmt.Fprintf(env.Stderr, "错误: snapshot not found for approval %s\n", id)
		return 1
	}

	// Display preview (sanitized)
	fmt.Fprintln(env.Stdout, "Approval Preview")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintf(env.Stdout, "Approval ID:\n%s\n\n", snap.ApprovalID)
	fmt.Fprintf(env.Stdout, "Tool:\n%s\n\n", snap.ToolName)
	fmt.Fprintf(env.Stdout, "Risk:\n%s\n\n", snap.RiskLevel)
	fmt.Fprintf(env.Stdout, "Status:\n%s\n\n", req.Status)
	fmt.Fprintf(env.Stdout, "Reason:\n%s\n\n", security.SanitizeText(snap.Reason))
	if snap.Path != "" {
		fmt.Fprintf(env.Stdout, "Path:\n%s\n\n", security.SanitizeText(snap.Path))
	}
	if snap.Command != "" {
		fmt.Fprintf(env.Stdout, "Command:\n%s\n\n", security.SanitizeText(snap.Command))
	}
	fmt.Fprintf(env.Stdout, "Preview:\n%s\n\n", snap.SanitizedPreview)
	fmt.Fprintln(env.Stdout, "This command has NOT been executed.")

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
