package task

type WorktreeMode string

const (
	WorktreeDisabled WorktreeMode = "disabled"
	WorktreeRequired WorktreeMode = "required"
)

type WorktreePolicy struct {
	Mode       WorktreeMode
	BaseRef    string
	Cleanup    bool
	BranchName string
}

type TaskContract interface {
	ID() string
	Objective() string
	RepoRoot() string
	Worktree() WorktreePolicy
	AllowedTools() []string
	SecurityProfile() string
}
