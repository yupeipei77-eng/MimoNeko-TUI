package neko

import (
	"strings"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"
)

// AgentMode describes an operator-facing TUI mode. New modes can be wired to a
// real AgentLoop later without changing picker or status rendering.
type AgentMode interface {
	ID() string
	Name() string
	Description() string
	AllowedTools() []string
	WritePermission() security.PermissionMode
	UseWorktree() bool
}

type staticAgentMode struct {
	id              string
	name            string
	description     string
	allowedTools    []string
	writePermission security.PermissionMode
	useWorktree     bool
	aliases         []string
}

func (m staticAgentMode) ID() string                               { return m.id }
func (m staticAgentMode) Name() string                             { return m.name }
func (m staticAgentMode) Description() string                      { return m.description }
func (m staticAgentMode) AllowedTools() []string                   { return append([]string(nil), m.allowedTools...) }
func (m staticAgentMode) WritePermission() security.PermissionMode { return m.writePermission }
func (m staticAgentMode) UseWorktree() bool                        { return m.useWorktree }

func defaultAgentModes() []staticAgentMode {
	return []staticAgentMode{
		{
			id:              "multi",
			name:            "Build",
			description:     "multi-agent worktree build",
			allowedTools:    []string{"file_read", "list_files", "git_diff", "test_run", "patch_preview"},
			writePermission: security.PermissionPatchPreview,
			useWorktree:     true,
			aliases:         []string{"build", "builder"},
		},
		{
			id:              "single",
			name:            "Single",
			description:     "single-agent direct chat",
			allowedTools:    []string{"file_read", "list_files"},
			writePermission: security.PermissionReadOnly,
			useWorktree:     false,
		},
		{
			id:              "explore",
			name:            "Explore",
			description:     "read-only repository exploration",
			allowedTools:    []string{"file_read", "list_files", "git_diff"},
			writePermission: security.PermissionReadOnly,
			useWorktree:     false,
		},
		{
			id:              "plan",
			name:            "Plan",
			description:     "produce implementation plans without writes",
			allowedTools:    []string{"file_read", "list_files", "git_diff", "test_run"},
			writePermission: security.PermissionPlan,
			useWorktree:     false,
		},
		{
			id:              "builder",
			name:            "Builder",
			description:     "dry-run builder with patch preview",
			allowedTools:    []string{"file_read", "list_files", "git_diff", "test_run", "patch_preview"},
			writePermission: security.PermissionPatchPreview,
			useWorktree:     true,
			aliases:         []string{"worktree"},
		},
		{
			id:              "reviewer",
			name:            "Reviewer",
			description:     "review patches and validation output",
			allowedTools:    []string{"file_read", "list_files", "git_diff", "test_run"},
			writePermission: security.PermissionReadOnly,
			useWorktree:     false,
			aliases:         []string{"review"},
		},
	}
}

func agentModeByID(id string) (staticAgentMode, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, mode := range defaultAgentModes() {
		if mode.id == id {
			return mode, true
		}
		for _, alias := range mode.aliases {
			if alias == id {
				return mode, true
			}
		}
	}
	return staticAgentMode{}, false
}

func agentModeUsage() string {
	ids := make([]string, 0, len(defaultAgentModes()))
	for _, mode := range defaultAgentModes() {
		ids = append(ids, mode.id)
	}
	return strings.Join(ids, "|")
}
