package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"version"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(version) code = %d", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "reasonforge 0.1.0-dev" {
		t.Fatalf("version output = %q", got)
	}
}

func TestInitThenDoctor(t *testing.T) {
	root := t.TempDir()
	var initOut bytes.Buffer

	code := Run([]string{"init", "--dir", root}, Env{Stdout: &initOut})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	if !strings.Contains(initOut.String(), "Initialized ReasonForge") {
		t.Fatalf("init output = %q", initOut.String())
	}

	var doctorOut bytes.Buffer
	var doctorErr bytes.Buffer
	code = Run([]string{"doctor", "--dir", root}, Env{
		Stdout: &doctorOut,
		Stderr: &doctorErr,
	})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d, stderr = %q", code, doctorErr.String())
	}
	if !strings.Contains(doctorOut.String(), "ReasonForge doctor OK") {
		t.Fatalf("doctor output = %q", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "config_dir=") {
		t.Fatalf("doctor output = %q, want config_dir line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "default_model=local-coder") {
		t.Fatalf("doctor output = %q, want default_model line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "immutable_prefix_sources=3") {
		t.Fatalf("doctor output = %q, want immutable_prefix_sources line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "prefix_canonicalization=enabled") {
		t.Fatalf("doctor output = %q, want prefix_canonicalization line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "budget_warn_ratio=") {
		t.Fatalf("doctor output = %q, want budget_warn_ratio line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "budget_block_ratio=") {
		t.Fatalf("doctor output = %q, want budget_block_ratio line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "cache_estimated_ttl=") {
		t.Fatalf("doctor output = %q, want cache_estimated_ttl line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "event_id_collision_resistant=true") {
		t.Fatalf("doctor output = %q, want event_id_collision_resistant line", doctorOut.String())
	}
}

func TestNoArgsReturnsUsageError(t *testing.T) {
	var stderr bytes.Buffer
	code := Run(nil, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(nil) code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "Usage: reasonforge <command>") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestUnknownCommandReturnsUsageError(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"frobnicate"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(unknown) code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown command", stderr.String())
	}
}

func TestHelpWritesUsageToStdout(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"help"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(help) code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Commands:") {
		t.Fatalf("stdout = %q, want commands", stdout.String())
	}
}

func TestInitReportsWorkingDirectoryError(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"init"}, Env{
		Stderr: &stderr,
		Getwd:  func() (string, error) { return "", errors.New("boom") },
	})
	if code != 1 {
		t.Fatalf("Run(init) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "resolve working directory") {
		t.Fatalf("stderr = %q, want working directory error", stderr.String())
	}
}

func TestDoctorReportsMissingConfig(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	code := Run([]string{"doctor", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(doctor) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "doctor failed") {
		t.Fatalf("stderr = %q, want doctor failure", stderr.String())
	}
}

func TestCommandsRejectExtraPositionalArgs(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name string
		args []string
	}{
		{name: "version", args: []string{"version", "extra"}},
		{name: "init", args: []string{"init", "--dir", root, "extra"}},
		{name: "doctor", args: []string{"doctor", "--dir", root, "extra"}},
		{name: "help", args: []string{"help", "extra"}},
		{name: "cache-report", args: []string{"cache-report", "--dir", root, "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			code := Run(tt.args, Env{Stderr: &stderr})
			if code != 2 {
				t.Fatalf("Run(%v) code = %d, want 2", tt.args, code)
			}
			if !strings.Contains(stderr.String(), "accepts") {
				t.Fatalf("stderr = %q, want positional argument error", stderr.String())
			}
		})
	}
}

func TestCacheReportCommand(t *testing.T) {
	root := t.TempDir()

	// Init first
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"cache-report", "--dir", root}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(cache-report) code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "total_observations=") {
		t.Fatalf("cache-report output = %q, want total_observations", stdout.String())
	}
	if !strings.Contains(stdout.String(), "hit_rate=") {
		t.Fatalf("cache-report output = %q, want hit_rate", stdout.String())
	}
}

func TestCacheReportReportsMissingConfig(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	code := Run([]string{"cache-report", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(cache-report) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "cache-report failed") {
		t.Fatalf("stderr = %q, want cache-report failure", stderr.String())
	}
}

func TestModelsCommand(t *testing.T) {
	root := t.TempDir()

	// Init first
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"models", "--dir", root}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(models) code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "ReasonForge Models") {
		t.Fatalf("models output = %q, want ReasonForge Models header", output)
	}
	if !strings.Contains(output, "default_model=local-coder") {
		t.Fatalf("models output = %q, want default_model line", output)
	}
	if !strings.Contains(output, "provider=local-openai-compatible") {
		t.Fatalf("models output = %q, want provider line", output)
	}
	if !strings.Contains(output, "type=openai-compatible") {
		t.Fatalf("models output = %q, want type line", output)
	}
	if !strings.Contains(output, "base_url=http://127.0.0.1:11434/v1") {
		t.Fatalf("models output = %q, want base_url line", output)
	}
	if !strings.Contains(output, "api_key_env=REASONFORGE_API_KEY") {
		t.Fatalf("models output = %q, want api_key_env line", output)
	}
	if !strings.Contains(output, "api_key_status=") {
		t.Fatalf("models output = %q, want api_key_status line", output)
	}
	if !strings.Contains(output, "models=local-coder") {
		t.Fatalf("models output = %q, want models line", output)
	}
	if !strings.Contains(output, "fallback_chain:") {
		t.Fatalf("models output = %q, want fallback_chain section", output)
	}

	// Verify no API key values leaked
	if strings.Contains(output, "Bearer ") {
		t.Fatalf("models output contains Authorization header, possible key leak")
	}
}

func TestModelsCommandDoesNotLeakAPIKey(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	code = Run([]string{"models", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(models) code = %d", code)
	}

	output := stdout.String()
	// Should show "configured" or "missing", never the actual key value
	if strings.Contains(output, "sk-") {
		t.Fatalf("models output contains possible API key: %q", output)
	}
}

func TestModelsCommandRejectsExtraArgs(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"models", "extra"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(models extra) code = %d, want 2", code)
	}
}

func TestModelsCommandReportsMissingConfig(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	code := Run([]string{"models", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(models) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "models failed") {
		t.Fatalf("stderr = %q, want models failure", stderr.String())
	}
}

func TestAPIKeyStatusFunction(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		want   string
		setEnv bool
	}{
		{name: "empty env var", envVar: "", want: "missing"},
		{name: "unset env var", envVar: "NONEXISTENT_VAR_XYZ_999", want: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := apiKeyStatus(tt.envVar)
			if got != tt.want {
				t.Errorf("apiKeyStatus(%q) = %q, want %q", tt.envVar, got, tt.want)
			}
		})
	}
}

func TestToolsCommand(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"tools", "--dir", root}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(tools) code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "ReasonForge Tools") {
		t.Fatalf("tools output = %q, want ReasonForge Tools header", output)
	}
	if !strings.Contains(output, "file_read") {
		t.Fatalf("tools output = %q, want file_read", output)
	}
	if !strings.Contains(output, "file_write") {
		t.Fatalf("tools output = %q, want file_write", output)
	}
	if !strings.Contains(output, "file_patch") {
		t.Fatalf("tools output = %q, want file_patch", output)
	}
	if !strings.Contains(output, "git_diff") {
		t.Fatalf("tools output = %q, want git_diff", output)
	}
	if !strings.Contains(output, "test_run") {
		t.Fatalf("tools output = %q, want test_run", output)
	}
	if !strings.Contains(output, "risk=") {
		t.Fatalf("tools output = %q, want risk levels", output)
	}
}

func TestToolRunFileRead(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a test file
	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"tool-run", "--dir", root, "file_read", "--path", "test.txt"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(tool-run file_read) code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "hello world") {
		t.Fatalf("tool-run file_read output = %q, want 'hello world'", output)
	}
}

func TestToolRunDoesNotLeakSensitiveContent(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a .env file with a secret
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET_KEY=supersecret"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code = Run([]string{"tool-run", "--dir", root, "file_read", "--path", ".env"}, Env{
		Stderr: &stderr,
	})
	if code == 0 {
		t.Fatal("tool-run file_read .env should fail")
	}

	// The error should not contain the secret
	errOutput := stderr.String()
	if strings.Contains(errOutput, "supersecret") {
		t.Fatalf("tool-run output leaks sensitive content: %q", errOutput)
	}
}

func TestToolsCommandRejectsExtraArgs(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"tools", "--dir", root, "extra"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(tools extra) code = %d, want 2", code)
	}
}

func TestToolRunRequiresToolName(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"tool-run", "--dir", root}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(tool-run without name) code = %d, want 2", code)
	}
}

func TestToolRunReportsMissingConfig(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	code := Run([]string{"tool-run", "--dir", root, "file_read"}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(tool-run missing config) code = %d, want 1", code)
	}
}
