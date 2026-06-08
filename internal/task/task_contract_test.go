package task

import (
	"testing"
)

func TestDefaultContract(t *testing.T) {
	c := DefaultContract("/repo", "read the README")

	if c.Goal != "read the README" {
		t.Errorf("Goal = %q, want %q", c.Goal, "read the README")
	}
	if c.RepoRoot != "/repo" {
		t.Errorf("RepoRoot = %q, want %q", c.RepoRoot, "/repo")
	}
	if c.MaxSteps != 5 {
		t.Errorf("MaxSteps = %d, want 5", c.MaxSteps)
	}
	if c.MaxToolCalls != 10 {
		t.Errorf("MaxToolCalls = %d, want 10", c.MaxToolCalls)
	}
	if !c.DryRun {
		t.Error("DryRun = false, want true")
	}
	if len(c.ID) == 0 {
		t.Error("ID is empty")
	}
	if !c.RequiresApproval("medium") {
		t.Error("medium risk should require approval")
	}
	if c.RequiresApproval("low") {
		t.Error("low risk should not require approval")
	}

	// Default allowed tools
	allowedTools := map[string]bool{
		"file_read":  true,
		"list_files": true,
		"git_diff":   true,
		"test_run":   true,
	}
	for _, tool := range c.AllowedTools {
		if !allowedTools[tool] {
			t.Errorf("unexpected allowed tool %q", tool)
		}
	}

	// Default denied tools
	deniedTools := map[string]bool{
		"file_write": true,
		"file_patch": true,
	}
	for _, tool := range c.DeniedTools {
		if !deniedTools[tool] {
			t.Errorf("unexpected denied tool %q", tool)
		}
	}
}

func TestIsToolAllowed(t *testing.T) {
	tests := []struct {
		name     string
		contract TaskContract
		toolName string
		want     bool
	}{
		{
			name: "allowed tool in allowed list",
			contract: TaskContract{
				AllowedTools: []string{"file_read", "git_diff"},
				DeniedTools:  []string{},
			},
			toolName: "file_read",
			want:     true,
		},
		{
			name: "tool not in allowed list",
			contract: TaskContract{
				AllowedTools: []string{"file_read", "git_diff"},
				DeniedTools:  []string{},
			},
			toolName: "file_write",
			want:     false,
		},
		{
			name: "tool in denied list",
			contract: TaskContract{
				AllowedTools: []string{},
				DeniedTools:  []string{"file_write"},
			},
			toolName: "file_write",
			want:     false,
		},
		{
			name: "tool in both allowed and denied (denied wins)",
			contract: TaskContract{
				AllowedTools: []string{"file_read"},
				DeniedTools:  []string{"file_read"},
			},
			toolName: "file_read",
			want:     false,
		},
		{
			name: "no restrictions allows all",
			contract: TaskContract{
				AllowedTools: []string{},
				DeniedTools:  []string{},
			},
			toolName: "any_tool",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.contract.IsToolAllowed(tt.toolName)
			if got != tt.want {
				t.Errorf("IsToolAllowed(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestIsPathAllowed(t *testing.T) {
	tests := []struct {
		name     string
		contract TaskContract
		path     string
		want     bool
	}{
		{
			name: "no restrictions",
			contract: TaskContract{
				AllowedPaths: nil,
				DeniedPaths:  nil,
			},
			path: "src/main.go",
			want: true,
		},
		{
			name: "denied path pattern",
			contract: TaskContract{
				AllowedPaths: nil,
				DeniedPaths:  []string{".env"},
			},
			path: ".env",
			want: false,
		},
		{
			name: "path matching allowed pattern",
			contract: TaskContract{
				AllowedPaths: []string{"src/*"},
				DeniedPaths:  nil,
			},
			path: "src/main.go",
			want: true,
		},
		{
			name: "path not matching allowed pattern",
			contract: TaskContract{
				AllowedPaths: []string{"src/*"},
				DeniedPaths:  nil,
			},
			path: "pkg/util.go",
			want: false,
		},
		{
			name: "denied wins over allowed",
			contract: TaskContract{
				AllowedPaths: []string{"*"},
				DeniedPaths:  []string{".git"},
			},
			path: ".git",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.contract.IsPathAllowed(tt.path)
			if got != tt.want {
				t.Errorf("IsPathAllowed(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestRequiresApproval(t *testing.T) {
	c := TaskContract{
		RequireApprovalForRisk: []string{"medium"},
	}

	if !c.RequiresApproval("medium") {
		t.Error("medium should require approval")
	}
	if c.RequiresApproval("low") {
		t.Error("low should not require approval")
	}
	if c.RequiresApproval("high") {
		t.Error("high should not require approval (not in list)")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		contract TaskContract
		wantErr  bool
	}{
		{
			name: "valid contract",
			contract: TaskContract{
				Goal:     "read files",
				RepoRoot: "/repo",
				MaxSteps: 5,
			},
			wantErr: false,
		},
		{
			name: "missing goal",
			contract: TaskContract{
				Goal:     "",
				RepoRoot: "/repo",
				MaxSteps: 5,
			},
			wantErr: true,
		},
		{
			name: "missing repo_root",
			contract: TaskContract{
				Goal:     "read files",
				RepoRoot: "",
				MaxSteps: 5,
			},
			wantErr: true,
		},
		{
			name: "zero max_steps",
			contract: TaskContract{
				Goal:     "read files",
				RepoRoot: "/repo",
				MaxSteps: 0,
			},
			wantErr: true,
		},
		{
			name: "tool in both allowed and denied",
			contract: TaskContract{
				Goal:         "read files",
				RepoRoot:     "/repo",
				MaxSteps:     5,
				AllowedTools: []string{"file_read", "file_write"},
				DeniedTools:  []string{"file_write"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultContractIDsAreUnique(t *testing.T) {
	c1 := DefaultContract("/repo", "task 1")
	c2 := DefaultContract("/repo", "task 2")
	if c1.ID == c2.ID {
		t.Error("two default contracts should have different IDs")
	}
}
