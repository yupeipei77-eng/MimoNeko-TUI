package security

import (
	"testing"
)

func TestValidatePathGitDirectory(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantRule string
		wantSev  ViolationSeverity
	}{
		{
			name:     ".git directory",
			path:     ".git",
			wantRule: "git-directory",
			wantSev:  SeverityCritical,
		},
		{
			name:     ".git/config",
			path:     ".git/config",
			wantRule: "git-directory",
			wantSev:  SeverityCritical,
		},
		{
			name:     ".git/objects/pack",
			path:     ".git/objects/pack",
			wantRule: "git-directory",
			wantSev:  SeverityCritical,
		},
		{
			name:     "nested .git",
			path:     "project/.git/config",
			wantRule: "git-directory",
			wantSev:  SeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) == 0 {
				t.Fatalf("ValidatePath(%q) = [], want violation with rule %q", tt.path, tt.wantRule)
			}

			found := false
			for _, v := range violations {
				if v.Rule == tt.wantRule && v.Severity == tt.wantSev {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ValidatePath(%q) = %v, want rule %q severity %q", tt.path, violations, tt.wantRule, tt.wantSev)
			}
		})
	}
}

func TestValidatePathEnvFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantRule string
	}{
		{
			name:     ".env file",
			path:     ".env",
			wantRule: "env-file",
		},
		{
			name:     ".env.local",
			path:     ".env.local",
			wantRule: "env-file-variant",
		},
		{
			name:     ".env.production",
			path:     ".env.production",
			wantRule: "env-file-variant",
		},
		{
			name:     "nested .env",
			path:     "config/.env",
			wantRule: "env-file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) == 0 {
				t.Fatalf("ValidatePath(%q) = [], want violation with rule %q", tt.path, tt.wantRule)
			}

			found := false
			for _, v := range violations {
				if v.Rule == tt.wantRule {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ValidatePath(%q) = %v, want rule %q", tt.path, violations, tt.wantRule)
			}
		})
	}
}

func TestValidatePathSSHKeys(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: ".ssh directory", path: ".ssh"},
		{name: "id_rsa", path: "id_rsa"},
		{name: "id_ed25519", path: "id_ed25519"},
		{name: "id_dsa", path: "id_dsa"},
		{name: "id_ecdsa", path: "id_ecdsa"},
		{name: ".ssh/id_rsa", path: ".ssh/id_rsa"},
		{name: "home/.ssh/id_ed25519", path: "home/.ssh/id_ed25519"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) == 0 {
				t.Fatalf("ValidatePath(%q) = [], want violation", tt.path)
			}

			hasSSH := false
			for _, v := range violations {
				if v.Rule == "ssh-directory" || v.Rule == "ssh-private-key" {
					hasSSH = true
					break
				}
			}
			if !hasSSH {
				t.Errorf("ValidatePath(%q) = %v, want SSH-related violation", tt.path, violations)
			}
		})
	}
}

func TestValidatePathTokenFiles(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantRule string
	}{
		{name: "token file", path: "token", wantRule: "token-file"},
		{name: ".token file", path: ".token", wantRule: "token-file"},
		{name: "access_token", path: "access_token", wantRule: "token-file"},
		{name: "refresh_token", path: "refresh_token", wantRule: "token-file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) == 0 {
				t.Fatalf("ValidatePath(%q) = [], want violation with rule %q", tt.path, tt.wantRule)
			}

			found := false
			for _, v := range violations {
				if v.Rule == tt.wantRule {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ValidatePath(%q) = %v, want rule %q", tt.path, violations, tt.wantRule)
			}
		})
	}
}

func TestValidatePathCredentialsFiles(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "credentials", path: "credentials"},
		{name: ".credentials", path: ".credentials"},
		{name: "credentials.json", path: "credentials.json"},
		{name: "credentials.yaml", path: "credentials.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) == 0 {
				t.Fatalf("ValidatePath(%q) = [], want violation", tt.path)
			}

			hasCredentials := false
			for _, v := range violations {
				if v.Rule == "credentials-file" {
					hasCredentials = true
					break
				}
			}
			if !hasCredentials {
				t.Errorf("ValidatePath(%q) = %v, want credentials-file violation", tt.path, violations)
			}
		})
	}
}

func TestValidatePathAllowedPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "readme", path: "README.md"},
		{name: "source file", path: "src/main.go"},
		{name: "test file", path: "internal/test.go"},
		{name: "docs", path: "docs/architecture.md"},
		{name: "config yaml", path: "config.yaml"},
		{name: "go mod", path: "go.mod"},
		{name: "go sum", path: "go.sum"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) != 0 {
				t.Errorf("ValidatePath(%q) = %v, want [] (no violations)", tt.path, violations)
			}
		})
	}
}

func TestValidatePathWindowsPaths(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantRule string
	}{
		{
			name:     "windows .git",
			path:     "C:\\Users\\test\\.git\\config",
			wantRule: "git-directory",
		},
		{
			name:     "windows .env",
			path:     "C:\\Projects\\.env",
			wantRule: "env-file",
		},
		{
			name:     "windows id_rsa",
			path:     "C:\\Users\\test\\.ssh\\id_rsa",
			wantRule: "ssh-private-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) == 0 {
				t.Fatalf("ValidatePath(%q) = [], want violation with rule %q", tt.path, tt.wantRule)
			}

			found := false
			for _, v := range violations {
				if v.Rule == tt.wantRule {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ValidatePath(%q) = %v, want rule %q", tt.path, violations, tt.wantRule)
			}
		})
	}
}

func TestValidatePathLinuxPaths(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantRule string
	}{
		{
			name:     "linux .git",
			path:     "/home/user/project/.git/config",
			wantRule: "git-directory",
		},
		{
			name:     "linux .env",
			path:     "/home/user/project/.env",
			wantRule: "env-file",
		},
		{
			name:     "linux id_rsa",
			path:     "/home/user/.ssh/id_rsa",
			wantRule: "ssh-private-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) == 0 {
				t.Fatalf("ValidatePath(%q) = [], want violation with rule %q", tt.path, tt.wantRule)
			}

			found := false
			for _, v := range violations {
				if v.Rule == tt.wantRule {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ValidatePath(%q) = %v, want rule %q", tt.path, violations, tt.wantRule)
			}
		})
	}
}

func TestValidatePathTraversal(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "unix traversal", path: "../secrets"},
		{name: "windows traversal", path: "..\\secrets"},
		{name: "nested traversal", path: "dir/../../etc/passwd"},
		{name: "deep traversal", path: "a/b/c/../../../secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) == 0 {
				t.Fatalf("ValidatePath(%q) = [], want violation", tt.path)
			}

			hasTraversal := false
			for _, v := range violations {
				if v.Rule == "path-traversal" {
					hasTraversal = true
					break
				}
			}
			if !hasTraversal {
				t.Errorf("ValidatePath(%q) = %v, want path-traversal violation", tt.path, violations)
			}
		})
	}
}

func TestValidatePathNestedPaths(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantRule string
	}{
		{
			name:     "nested .env in config",
			path:     "config/environments/.env.production",
			wantRule: "env-file-variant",
		},
		{
			name:     "nested credentials",
			path:     "config/credentials.json",
			wantRule: "credentials-file",
		},
		{
			name:     "nested secrets",
			path:     "config/secrets.yaml",
			wantRule: "secrets-file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) == 0 {
				t.Fatalf("ValidatePath(%q) = [], want violation with rule %q", tt.path, tt.wantRule)
			}

			found := false
			for _, v := range violations {
				if v.Rule == tt.wantRule {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ValidatePath(%q) = %v, want rule %q", tt.path, violations, tt.wantRule)
			}
		})
	}
}

func TestIsSensitivePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: ".git is sensitive", path: ".git", want: true},
		{name: ".env is sensitive", path: ".env", want: true},
		{name: "id_rsa is sensitive", path: "id_rsa", want: true},
		{name: "readme is not sensitive", path: "README.md", want: false},
		{name: "source is not sensitive", path: "src/main.go", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSensitivePath(tt.path)
			if got != tt.want {
				t.Errorf("IsSensitivePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsCriticalPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: ".git is critical", path: ".git", want: true},
		{name: ".env is critical", path: ".env", want: true},
		{name: "id_rsa is critical", path: "id_rsa", want: true},
		{name: "credentials is warning not critical", path: "credentials", want: false},
		{name: "readme is not critical", path: "README.md", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCriticalPath(tt.path)
			if got != tt.want {
				t.Errorf("IsCriticalPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestValidatePathEmptyPath(t *testing.T) {
	violations := ValidatePath("")
	if violations != nil {
		t.Errorf("ValidatePath(\"\") = %v, want nil", violations)
	}
}

func TestGetViolationSummary(t *testing.T) {
	tests := []struct {
		name       string
		violations []PathViolation
		want       string
	}{
		{
			name:       "no violations",
			violations: nil,
			want:       "allowed",
		},
		{
			name: "critical violation",
			violations: []PathViolation{
				{Path: ".git", Rule: "git-directory", Severity: SeverityCritical, Candidate: true},
			},
			want: "blocked_candidate",
		},
		{
			name: "warning violation",
			violations: []PathViolation{
				{Path: "credentials", Rule: "credentials-file", Severity: SeverityWarning, Candidate: true},
			},
			want: "warning",
		},
		{
			name: "info violation",
			violations: []PathViolation{
				{Path: ".npmrc", Rule: "npmrc-file", Severity: SeverityInfo, Candidate: true},
			},
			want: "info",
		},
		{
			name: "mixed violations",
			violations: []PathViolation{
				{Path: ".git", Rule: "git-directory", Severity: SeverityCritical, Candidate: true},
				{Path: "credentials", Rule: "credentials-file", Severity: SeverityWarning, Candidate: true},
			},
			want: "blocked_candidate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetViolationSummary(tt.violations)
			if got != tt.want {
				t.Errorf("GetViolationSummary(%v) = %q, want %q", tt.violations, got, tt.want)
			}
		})
	}
}

func TestValidatePathViolationCandidate(t *testing.T) {
	violations := ValidatePath(".git/config")
	for _, v := range violations {
		if !v.Candidate {
			t.Errorf("Violation for .git/config should have Candidate=true, got %v", v)
		}
	}
}

func TestValidatePathSecretsFiles(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "secrets", path: "secrets"},
		{name: ".secrets", path: ".secrets"},
		{name: "secrets.json", path: "secrets.json"},
		{name: "secrets.yaml", path: "secrets.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) == 0 {
				t.Fatalf("ValidatePath(%q) = [], want violation", tt.path)
			}

			hasSecrets := false
			for _, v := range violations {
				if v.Rule == "secrets-file" {
					hasSecrets = true
					break
				}
			}
			if !hasSecrets {
				t.Errorf("ValidatePath(%q) = %v, want secrets-file violation", tt.path, violations)
			}
		})
	}
}

func TestValidatePathKeyFiles(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "server.pem", path: "server.pem"},
		{name: "server.key", path: "server.key"},
		{name: "cert.pem", path: "cert.pem"},
		{name: "private.key", path: "private.key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidatePath(tt.path)
			if len(violations) == 0 {
				t.Fatalf("ValidatePath(%q) = [], want violation", tt.path)
			}

			hasKey := false
			for _, v := range violations {
				if v.Rule == "key-file" {
					hasKey = true
					break
				}
			}
			if !hasKey {
				t.Errorf("ValidatePath(%q) = %v, want key-file violation", tt.path, violations)
			}
		})
	}
}
