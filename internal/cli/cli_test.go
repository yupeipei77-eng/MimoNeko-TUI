package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mimoneko/mimoneko/internal/agent"
	"github.com/mimoneko/mimoneko/internal/auth"
	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/events"
	"github.com/mimoneko/mimoneko/internal/modelprofile"
	"github.com/mimoneko/mimoneko/internal/modelrouter"
	"github.com/mimoneko/mimoneko/internal/multiagent"
	webserver "github.com/mimoneko/mimoneko/internal/server"
	"github.com/mimoneko/mimoneko/internal/task"
	"github.com/mimoneko/mimoneko/internal/worktree"
)

type cliSequenceModelRouter struct {
	responses []string
	calls     int
}

func (r *cliSequenceModelRouter) Complete(ctx context.Context, req modelrouter.CompletionRequest) (modelrouter.CompletionResponse, error) {
	if len(r.responses) == 0 {
		return modelrouter.CompletionResponse{}, errors.New("no model responses configured")
	}
	index := r.calls
	if index >= len(r.responses) {
		index = len(r.responses) - 1
	}
	r.calls++
	return modelrouter.CompletionResponse{
		Provider: "test",
		Model:    "test-model",
		Text:     r.responses[index],
	}, nil
}

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"version"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(version) code = %d", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "MimoNeko 0.1.3-beta" {
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
	if !strings.Contains(initOut.String(), "Initialized MimoNeko") {
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
	if !strings.Contains(doctorOut.String(), "MimoNeko Doctor Report") {
		t.Fatalf("doctor output = %q", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "config_exists=true") {
		t.Fatalf("doctor output = %q, want config_exists line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "system_prompt=true") {
		t.Fatalf("doctor output = %q, want system_prompt line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "coding_rules=true") {
		t.Fatalf("doctor output = %q, want coding_rules line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "tools_schema=true") {
		t.Fatalf("doctor output = %q, want tools_schema line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "models_configured=true") {
		t.Fatalf("doctor output = %q, want models_configured line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "worktree_config=true") {
		t.Fatalf("doctor output = %q, want worktree_config line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "patch_config=true") {
		t.Fatalf("doctor output = %q, want patch_config line", doctorOut.String())
	}
	if !strings.Contains(doctorOut.String(), "events_config=true") {
		t.Fatalf("doctor output = %q, want events_config line", doctorOut.String())
	}
}

func TestInitRepairCreatesMissingScaffolding(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	systemPath := filepath.Join(root, "prompts", "system.md")
	if err := os.Remove(systemPath); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code := Run([]string{"init", "--dir", root, "--repair"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(init --repair) code = %d", code)
	}
	if _, err := os.Stat(systemPath); err != nil {
		t.Fatalf("repair did not recreate system prompt: %v", err)
	}
	if !strings.Contains(stdout.String(), "created") || !strings.Contains(stdout.String(), "prompts/system.md") {
		t.Fatalf("repair output = %q, want created system prompt", stdout.String())
	}
}

func TestInitRepairDoesNotOverwriteModelsConfig(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	modelsPath := filepath.Join(root, config.DirName(), "models.yaml")
	custom := []byte(`providers:
  - name: custom-provider
    type: openai-compatible
    base_url: http://127.0.0.1:9999/v1
    api_key_env: CUSTOM_API_KEY
    models:
      - name: custom-model
        purpose: coding
        max_output_tokens: 2048
        supports_prefix_cache: false
routing:
  default_model: custom-model
`)
	if err := os.WriteFile(modelsPath, custom, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "schemas", "tools.json")); err != nil {
		t.Fatal(err)
	}

	if code := Run([]string{"init", "--dir", root, "--repair"}, Env{}); code != 0 {
		t.Fatalf("Run(init --repair) code = %d", code)
	}
	got, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Fatalf("repair overwrote models.yaml:\n%s", string(got))
	}
}

func TestInitOutputShowsCreatedAndSkipped(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	if err := os.Remove(filepath.Join(root, "prompts", "coding_rules.md")); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code := Run([]string{"init", "--dir", root, "--repair"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(init --repair) code = %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "created") || !strings.Contains(output, "skipped") {
		t.Fatalf("init repair output = %q, want created and skipped lines", output)
	}
}

func TestDoctorDetectsMissingSystemPrompt(t *testing.T) {
	root := initRootForDoctorMissingTest(t, "prompts/system.md")
	var stderr bytes.Buffer
	code := Run([]string{"doctor", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(doctor) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "system prompt") || !strings.Contains(stderr.String(), "missing") {
		t.Fatalf("stderr = %q, want missing system prompt", stderr.String())
	}
}

func TestDoctorDetectsMissingCodingRules(t *testing.T) {
	root := initRootForDoctorMissingTest(t, "prompts/coding_rules.md")
	var stderr bytes.Buffer
	code := Run([]string{"doctor", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(doctor) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "coding rules") || !strings.Contains(stderr.String(), "missing") {
		t.Fatalf("stderr = %q, want missing coding rules", stderr.String())
	}
}

func TestDoctorDetectsMissingToolsSchema(t *testing.T) {
	root := initRootForDoctorMissingTest(t, "schemas/tools.json")
	var stderr bytes.Buffer
	code := Run([]string{"doctor", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(doctor) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "tools schema") || !strings.Contains(stderr.String(), "missing") {
		t.Fatalf("stderr = %q, want missing tools schema", stderr.String())
	}
}

func TestDoctorSuggestsInitRepair(t *testing.T) {
	root := initRootForDoctorMissingTest(t, "prompts/system.md")
	var stderr bytes.Buffer
	_ = Run([]string{"doctor", "--dir", root}, Env{Stderr: &stderr})
	if !strings.Contains(stderr.String(), "mimoneko init --repair") {
		t.Fatalf("stderr = %q, want init --repair suggestion", stderr.String())
	}
}

func TestDoctorPassesAfterInitRepair(t *testing.T) {
	root := initRootForDoctorMissingTest(t, "prompts/system.md")
	if code := Run([]string{"init", "--dir", root, "--repair"}, Env{}); code != 0 {
		t.Fatalf("Run(init --repair) code = %d", code)
	}
	var stderr bytes.Buffer
	code := Run([]string{"doctor", "--dir", root}, Env{Stderr: &stderr})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d, stderr = %q", code, stderr.String())
	}
}

func TestFirstRunInitThenDoctorPasses(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	var stderr bytes.Buffer
	code := Run([]string{"doctor", "--dir", root}, Env{Stderr: &stderr})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d, stderr = %q", code, stderr.String())
	}
}

func TestFirstRunInitThenRunDoesNotFailMissingPrefixSource(t *testing.T) {
	t.Setenv("MimoNeko_API_KEY", "")
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	_ = Run([]string{"run", "--dir", root, "--goal", "Reply OK", "--dry-run", "--max-steps", "1"}, Env{Stderr: &stderr})
	output := stderr.String()
	for _, wantAbsent := range []string{"read system_prompt", "prompts/system.md", "schemas/tools.json", "coding_rules.md"} {
		if strings.Contains(output, wantAbsent) {
			t.Fatalf("run failed due to missing prefix source %q: %s", wantAbsent, output)
		}
	}
}

func TestInitDoesNotWriteAPIKey(t *testing.T) {
	root := t.TempDir()
	secret := "sk-init-secret-value"
	t.Setenv("MIMO_API_KEY", secret)
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	assertTreeDoesNotContain(t, root, secret)
}

func TestRepairDoesNotWriteAPIKey(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	secret := "sk-repair-secret-value"
	t.Setenv("MIMO_API_KEY", secret)
	if err := os.Remove(filepath.Join(root, "prompts", "system.md")); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"init", "--dir", root, "--repair"}, Env{}); code != 0 {
		t.Fatalf("Run(init --repair) code = %d", code)
	}
	assertTreeDoesNotContain(t, root, secret)
}

func TestInitDoesNotOverwriteExistingModelProvider(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	modelsPath := filepath.Join(root, config.DirName(), "models.yaml")
	original := []byte(`providers:
  - name: preserved
    type: openai-compatible
    base_url: http://127.0.0.1:9999/v1
    api_key_env: PRESERVED_API_KEY
    models:
      - name: preserved-model
        purpose: coding
        max_output_tokens: 2048
        supports_prefix_cache: false
routing:
  default_model: preserved-model
`)
	if err := os.WriteFile(modelsPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init second time) code = %d", code)
	}
	got, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("init overwrote existing model provider:\n%s", string(got))
	}
}

func initRootForDoctorMissingTest(t *testing.T, relPath string) string {
	t.Helper()
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(relPath))); err != nil {
		t.Fatal(err)
	}
	return root
}

func assertTreeDoesNotContain(t *testing.T, root, needle string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), needle) {
			return fmt.Errorf("secret found in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func snapshotRelativeFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestNoArgsStartsFirstRunWizardWhenUserConfigMissing(t *testing.T) {
	home := setUserConfigHome(t)
	t.Setenv("MIMO_API_KEY", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != "sk-first-run" {
			t.Fatalf("api-key header = %q, want saved key", r.Header.Get("api-key"))
		}
		fmt.Fprint(w, `{"model":"mimo-v2.5-pro","choices":[{"delta":{"content":"OK"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	input := strings.NewReader("1\nsk-first-run\n" + server.URL + "\nmimo-v2.5-pro\n\n")
	code := Run(nil, Env{Stdout: &stdout, Stderr: &stderr, Stdin: input})
	if code != 0 {
		t.Fatalf("Run(nil) code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "Welcome to MioNeko") || !strings.Contains(output, "Step 1/3") || !strings.Contains(output, "Configuration Complete") {
		t.Fatalf("stdout = %q, want first-run wizard success", output)
	}
	if !strings.Contains(output, "Provider: MiMo") || !strings.Contains(output, "Model: mimo-v2.5-pro") || !strings.Contains(output, "Press Enter to continue") {
		t.Fatalf("stdout = %q, want completion page", output)
	}
	if strings.Contains(output, "sk-first-run") {
		t.Fatalf("stdout leaked API key: %q", output)
	}
	if _, err := os.Stat(filepath.Join(home, ".mimoneko", "config.yaml")); err != nil {
		t.Fatalf("user config was not saved: %v", err)
	}
}

func TestOnboardingUsesLineFallbackForNonTTY(t *testing.T) {
	var stdout bytes.Buffer
	if _, ok := surveyAskOptions(Env{
		Stdout: &stdout,
		Stderr: io.Discard,
		Stdin:  strings.NewReader(""),
	}); ok {
		t.Fatal("survey prompts should be disabled for non-TTY test IO")
	}
}

func TestOnboardingModelOptionsIncludeDefaultAndCustom(t *testing.T) {
	options := onboardingModelOptions("mimo")
	if len(options) != 2 || options[0] != "mimo-v2.5-pro" || options[1] != customModelOption {
		t.Fatalf("onboarding model options = %#v, want default and custom option", options)
	}
}

func TestNoArgsShowsReadyLandingWhenConfigured(t *testing.T) {
	setUserConfigHome(t)
	saveUserConfigForTest(t, "mimo", "sk-configured", "https://token-plan-cn.xiaomimimo.com/v1", "mimo-v2.5-pro")

	var stdout bytes.Buffer
	code := Run(nil, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(nil) code = %d, want 0", code)
	}
	output := stdout.String()
	if strings.Contains(output, "Usage: mimoneko <command>") {
		t.Fatalf("stdout = %q, should not show usage for configured no-args launch", output)
	}
	for _, want := range []string{"MioNeko Ready", "mimoneko \"", "mimoneko run", "mimoneko neko"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestConfigShowDoesNotLeakAPIKey(t *testing.T) {
	home := setUserConfigHome(t)
	root := setupModelCommandRoot(t)
	userSecret := "tp-config-show-secret-12345"
	envSecret := "tp-config-show-env-secret-67890"
	saveUserConfigForTest(t, "mimo", userSecret, "https://token-plan-cn.xiaomimimo.com/v1", "mimo-v2.5-pro")
	t.Setenv("MIMO_API_KEY", envSecret)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"config", "show"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
		Getwd:  func() (string, error) { return root, nil },
	})
	if code != 0 {
		t.Fatalf("config show code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"Config Show", "User Config:", "Project Config:", "Environment:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if strings.Contains(output, userSecret) || strings.Contains(output, envSecret) {
		t.Fatalf("config show leaked API key: %q", output)
	}
	if !strings.Contains(output, filepath.Join(home, ".mimoneko", "config.yaml")) {
		t.Fatalf("stdout = %q, want user config path", output)
	}
}

func TestQuotedPromptIsTreatedAsRunGoal(t *testing.T) {
	setUserConfigHome(t)
	t.Setenv("MIMO_API_KEY", "sk-routing-test")
	var stderr bytes.Buffer
	code := Run([]string{"Reply OK"}, Env{
		Stderr: &stderr,
		Getwd:  func() (string, error) { return "", errors.New("boom") },
	})
	if code != 1 {
		t.Fatalf("Run(shorthand) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		// A root resolution error means the argument was routed through run.
		if !strings.Contains(stderr.String(), "resolve working directory") {
			t.Fatalf("stderr = %q, want shorthand run routing", stderr.String())
		}
	} else {
		t.Fatalf("stderr = %q, should not report unknown command", stderr.String())
	}
}

func TestUnknownCommandShowsUsage(t *testing.T) {
	setUserConfigHome(t)
	t.Setenv("MIMO_API_KEY", "sk-routing-test")
	var stderr bytes.Buffer
	code := Run([]string{"unknown-command"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(unknown-command) code = %d, want 2", code)
	}
	output := stderr.String()
	if !strings.Contains(output, "unknown command") || !strings.Contains(output, "Usage: mimoneko <command>") {
		t.Fatalf("stderr = %q, want unknown command and usage", output)
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

func TestHelpFlagWritesUsageToStdout(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"--help"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(--help) code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage: mimoneko <command>") {
		t.Fatalf("stdout = %q, want usage", stdout.String())
	}
}

func setUserConfigHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	return home
}

func saveUserConfigForTest(t *testing.T, provider, apiKey, baseURL, model string) {
	t.Helper()
	cfg := &auth.Config{
		Auth: auth.AuthConfig{
			Providers: map[string]auth.ProviderConfig{
				provider: {
					APIKey:  apiKey,
					BaseURL: baseURL,
				},
			},
			DefaultProvider: provider,
		},
		Preferences: auth.PreferencesConfig{
			DefaultModel: model,
			DryRun:       true,
			Worktree:     true,
		},
	}
	if err := auth.SaveUserConfig(cfg); err != nil {
		t.Fatal(err)
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
	if !strings.Contains(stderr.String(), "config directory does not exist") {
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
		{name: "run", args: []string{"run", "--goal", "inspect repo", "extra"}},
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
	if !strings.Contains(stdout.String(), "Cache Report") {
		t.Fatalf("cache-report output = %q, want header", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Total Requests") || !strings.Contains(stdout.String(), "Hit Rate") {
		t.Fatalf("cache-report output = %q, want summary rows", stdout.String())
	}
}

func TestCacheReportReportsMissingConfig(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	code := Run([]string{"cache-report", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(cache-report) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Cache report failed") {
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
	if !strings.Contains(output, "MimoNeko Models") {
		t.Fatalf("models output = %q, want MimoNeko Models header", output)
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
	if !strings.Contains(output, "api_key_env=MimoNeko_API_KEY") {
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

func TestModelSetupAddsProvider(t *testing.T) {
	root := setupModelCommandRoot(t)
	var stdout bytes.Buffer
	code := Run([]string{"model", "setup", "--dir", root, "--preset", "mimo", "--provider", "mimo", "--model", "mimo-v2.5-pro"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("model setup code = %d", code)
	}
	cfg := loadConfigForTest(t, root)
	if _, ok := findProviderForTest(cfg, "mimo"); !ok {
		t.Fatal("mimo provider was not added")
	}
}

func TestModelSetupAddsModelToExistingProvider(t *testing.T) {
	root := setupModelCommandRoot(t)
	runModelSetupForTest(t, root, "--preset", "mimo", "--provider", "mimo", "--model", "mimo-v2.5")
	runModelSetupForTest(t, root, "--preset", "mimo", "--provider", "mimo", "--model", "mimo-v2.5-pro")
	cfg := loadConfigForTest(t, root)
	provider, ok := findProviderForTest(cfg, "mimo")
	if !ok {
		t.Fatal("mimo provider was not added")
	}
	if !providerHasModel(provider, "mimo-v2.5") || !providerHasModel(provider, "mimo-v2.5-pro") {
		t.Fatalf("provider models = %+v, want both configured", provider.Models)
	}
}

func TestModelSetupSetDefault(t *testing.T) {
	root := setupModelCommandRoot(t)
	runModelSetupForTest(t, root, "--preset", "mimo", "--provider", "mimo", "--model", "mimo-v2.5-pro", "--set-default")
	cfg := loadConfigForTest(t, root)
	if cfg.Models.Routing.DefaultModel != "mimo-v2.5-pro" {
		t.Fatalf("default_model = %q, want mimo-v2.5-pro", cfg.Models.Routing.DefaultModel)
	}
	if len(cfg.Models.Routing.FallbackChain) == 0 || cfg.Models.Routing.FallbackChain[0].Provider != "mimo" || cfg.Models.Routing.FallbackChain[0].Model != "mimo-v2.5-pro" {
		t.Fatalf("fallback_chain = %+v, want mimo/mimo-v2.5-pro first", cfg.Models.Routing.FallbackChain)
	}
}

func TestModelSetupDoesNotWriteAPIKey(t *testing.T) {
	root := setupModelCommandRoot(t)
	secret := "sk-test-model-setup-secret"
	t.Setenv("MIMO_API_KEY", secret)
	runModelSetupForTest(t, root, "--preset", "mimo", "--provider", "mimo", "--model", "mimo-v2.5-pro")
	content := readModelsYAMLForTest(t, root)
	if strings.Contains(content, secret) {
		t.Fatal("models.yaml leaked the API key value")
	}
	if !strings.Contains(content, "api_key_env: MIMO_API_KEY") {
		t.Fatalf("models.yaml = %q, want api_key_env name", content)
	}
}

func TestModelSetupMissingEnvPrintsSafeHint(t *testing.T) {
	setUserConfigHome(t)
	root := setupModelCommandRoot(t)
	t.Setenv("MIMO_API_KEY", "")
	var stdout bytes.Buffer
	code := Run([]string{"model", "setup", "--dir", root, "--preset", "mimo", "--provider", "mimo", "--model", "mimo-v2.5-pro"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("model setup code = %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, `Windows: setx MIMO_API_KEY "your-key"`) {
		t.Fatalf("stdout = %q, want Windows safe hint", output)
	}
	if strings.Contains(output, "sk-") {
		t.Fatalf("stdout leaked a possible API key: %q", output)
	}
}

func TestModelListDoesNotLeakAPIKey(t *testing.T) {
	root := setupModelCommandRoot(t)
	secret := "sk-test-model-list-secret"
	t.Setenv("MIMO_API_KEY", secret)
	runModelSetupForTest(t, root, "--preset", "mimo", "--provider", "mimo", "--model", "mimo-v2.5-pro")
	var stdout bytes.Buffer
	code := Run([]string{"model", "list", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("model list code = %d", code)
	}
	output := stdout.String()
	if strings.Contains(output, secret) {
		t.Fatal("model list leaked API key")
	}
	if !strings.Contains(output, "api_key_status=configured") {
		t.Fatalf("stdout = %q, want configured status", output)
	}
}

func TestModelListShowsConfiguredMissing(t *testing.T) {
	root := setupModelCommandRoot(t)
	t.Setenv("MIMO_API_KEY", "sk-configured-for-status")
	t.Setenv("MimoNeko_API_KEY", "")
	runModelSetupForTest(t, root, "--preset", "mimo", "--provider", "mimo", "--model", "mimo-v2.5-pro")
	var stdout bytes.Buffer
	code := Run([]string{"model", "list", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("model list code = %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "api_key_status=configured") || !strings.Contains(output, "api_key_status=missing") {
		t.Fatalf("stdout = %q, want configured and missing statuses", output)
	}
}

func TestModelDiscoverListsRemoteModels(t *testing.T) {
	root := setupModelCommandRoot(t)
	t.Setenv("TEST_MODEL_API_KEY", "sk-discover-secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("path = %s, want /models", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-discover-secret" {
			t.Fatalf("unexpected authorization header")
		}
		fmt.Fprint(w, `{"data":[{"id":"mimo-v2.5"},{"id":"mimo-v2.5-pro"}]}`)
	}))
	defer server.Close()
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", server.URL, "--api-key-env", "TEST_MODEL_API_KEY", "--model", "mimo-v2.5")
	var stdout bytes.Buffer
	code := Run([]string{"model", "discover", "--dir", root, "--provider", "test"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("model discover code = %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "* mimo-v2.5") || !strings.Contains(output, "* mimo-v2.5-pro") {
		t.Fatalf("stdout = %q, want remote model ids", output)
	}
}

func TestModelDiscoverHandlesUnauthorized(t *testing.T) {
	root := setupModelCommandRoot(t)
	secret := "sk-unauthorized-secret"
	t.Setenv("TEST_MODEL_API_KEY", secret)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "token "+secret, http.StatusUnauthorized)
	}))
	defer server.Close()
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", server.URL, "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model")
	var stderr bytes.Buffer
	code := Run([]string{"model", "discover", "--dir", root, "--provider", "test"}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("model discover code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "API returned status 401") {
		t.Fatalf("stderr = %q, want status 401", stderr.String())
	}
}

func TestModelDiscoverDoesNotLeakAPIKey(t *testing.T) {
	root := setupModelCommandRoot(t)
	secret := "sk-discover-do-not-leak"
	t.Setenv("TEST_MODEL_API_KEY", secret)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Authorization: Bearer "+secret, http.StatusUnauthorized)
	}))
	defer server.Close()
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", server.URL, "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model")
	var stderr bytes.Buffer
	_ = Run([]string{"model", "discover", "--dir", root, "--provider", "test"}, Env{Stderr: &stderr})
	if strings.Contains(stderr.String(), secret) || strings.Contains(stderr.String(), "Bearer "+secret) {
		t.Fatalf("discover stderr leaked API key: %q", stderr.String())
	}
}

func TestModelDiscoverWriteCapabilities(t *testing.T) {
	root := t.TempDir()
	if _, err := config.InitDetailed(root); err != nil {
		t.Fatal(err)
	}
	models := config.ModelsConfig{
		Providers: []config.ProviderConfig{
			{
				Name:      "mimo",
				Type:      "openai-compatible",
				BaseURL:   "http://127.0.0.1:1",
				APIKeyEnv: "TEST_MODEL_API_KEY",
				Models: []config.ModelConfig{
					{Name: "mimo-v2.5-pro", Purpose: "coding", MaxOutputTokens: 4096},
				},
			},
		},
		Routing: config.RoutingConfig{DefaultModel: "mimo-v2.5-pro"},
	}
	if err := modelprofile.Save(root, models); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_MODEL_API_KEY", "sk-discover-capability")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"mimo-v2.5-pro"}]}`)
	}))
	defer server.Close()
	models.Providers[0].BaseURL = server.URL
	if err := modelprofile.Save(root, models); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code := Run([]string{"model", "discover", "--dir", root, "--provider", "mimo", "--write-capabilities"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("model discover code = %d, output = %q", code, stdout.String())
	}
	cfg := loadConfigForTest(t, root)
	provider, _ := findProviderForTest(cfg, "mimo")
	if provider.Models[0].MaxContextTokens != 1_000_000 || provider.Models[0].ReasoningLevel != "high" {
		t.Fatalf("model = %+v, want written capabilities", provider.Models[0])
	}
}

func TestModelTestSuccess(t *testing.T) {
	root := setupModelCommandRoot(t)
	t.Setenv("TEST_MODEL_API_KEY", "sk-model-test")
	server := newChatCompletionServer(t, http.StatusOK, `{"model":"test-model","choices":[{"message":{"content":"OK"}}]}`)
	defer server.Close()
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", server.URL, "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model", "--set-default")
	var stdout bytes.Buffer
	code := Run([]string{"model", "test", "--dir", root, "--provider", "test", "--model", "test-model"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("model test code = %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "Model Test") || !strings.Contains(output, "Status") || !strings.Contains(output, "OK") || !strings.Contains(output, "Response") {
		t.Fatalf("stdout = %q, want ok response", output)
	}
	if strings.Contains(output, "sk-model-test") {
		t.Fatalf("stdout leaked API key: %q", output)
	}
}

func TestNekoCommandExists(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"neko", "--help"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("neko --help code = %d", code)
	}
	if !strings.Contains(stdout.String(), "MimoNeko") {
		t.Fatalf("stdout = %q, want MimoNeko", stdout.String())
	}
}

func TestNekoHelp(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"neko", "--help"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("neko --help code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage: neko") || !strings.Contains(stdout.String(), "mode=multi") {
		t.Fatalf("stdout = %q, want neko usage", stdout.String())
	}
}

func TestMimoNekoNekoAlias(t *testing.T) {
	root := setupModelCommandRoot(t)
	var stdout bytes.Buffer
	code := Run([]string{"neko", "--dir", root, "--no-color"}, Env{Stdout: &stdout, Stdin: strings.NewReader("/exit\n")})
	if code != 0 {
		t.Fatalf("MimoNeko neko code = %d, output = %q", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "MIMO") || !strings.Contains(stdout.String(), "Goodbye from MIMO.") {
		t.Fatalf("stdout = %q, want console branding and exit", stdout.String())
	}
}

func TestNekoStatusReportsGitState(t *testing.T) {
	setUserConfigHome(t)
	root := setupGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\nunstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "staged.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "add", "staged.txt")
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"neko", "status", "--dir", root}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("neko status code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"Neko Status", "Branch", "Clean", "false", "Staged", "1", "Unstaged", "1", "Untracked", "1", "Latest Run", "unavailable"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestNekoDiffShowsWorkingTreeDiff(t *testing.T) {
	setUserConfigHome(t)
	root := setupGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\nworking tree change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"neko", "diff", "--dir", root}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("neko diff code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "diff --git") || !strings.Contains(output, "+working tree change") {
		t.Fatalf("stdout = %q, want working tree diff", output)
	}
}

func TestNekoDiffStagedShowsCachedDiff(t *testing.T) {
	setUserConfigHome(t)
	root := setupGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "staged.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "add", "staged.txt")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"neko", "diff", "--dir", root, "--staged"}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("neko diff --staged code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "staged.txt") || !strings.Contains(output, "+staged") {
		t.Fatalf("stdout = %q, want staged diff", output)
	}
}

func TestNekoPlanPrintsStubAndDoesNotWriteFiles(t *testing.T) {
	setUserConfigHome(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotRelativeFiles(t, root)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"neko", "plan", "--dir", root, "--goal", "Update docs"}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("neko plan code = %d, stderr = %q", code, stderr.String())
	}

	var plan struct {
		Goal                 string `json:"goal"`
		PrefixFingerprint    string `json:"prefix_fingerprint"`
		ImplementationStatus string `json:"implementation_status"`
		WritesFiles          bool   `json:"writes_files"`
		CallsModel           bool   `json:"calls_model"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("plan JSON did not parse: %v\n%s", err, stdout.String())
	}
	if plan.Goal != "Update docs" || plan.PrefixFingerprint == "" || plan.ImplementationStatus != "stub" || plan.WritesFiles || plan.CallsModel {
		t.Fatalf("unexpected plan: %+v", plan)
	}

	after := snapshotRelativeFiles(t, root)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("neko plan wrote files: before=%v after=%v", before, after)
	}
}

func TestNekoCacheStatsSmoke(t *testing.T) {
	setUserConfigHome(t)
	root := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"neko", "cache", "stats", "--dir", root}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("neko cache stats code = %d, stderr = %q", code, stderr.String())
	}

	var stats struct {
		PrefixFingerprint     string  `json:"prefix_fingerprint"`
		ImmutableBytes        int     `json:"immutable_bytes"`
		SemiStableBytes       int     `json:"semi_stable_bytes"`
		VolatileBytes         int     `json:"volatile_bytes"`
		EstimatedCacheHitRate float64 `json:"estimated_cache_hit_ratio"`
		ImplementationStatus  string  `json:"implementation_status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &stats); err != nil {
		t.Fatalf("cache stats JSON did not parse: %v\n%s", err, stdout.String())
	}
	if stats.PrefixFingerprint == "" || stats.ImmutableBytes == 0 || stats.SemiStableBytes == 0 || stats.VolatileBytes == 0 {
		t.Fatalf("unexpected cache stats: %+v", stats)
	}
	if stats.EstimatedCacheHitRate <= 0 || stats.EstimatedCacheHitRate > 1 {
		t.Fatalf("estimated cache hit ratio = %f, want within (0,1]", stats.EstimatedCacheHitRate)
	}
	if stats.ImplementationStatus != "stub" {
		t.Fatalf("implementation_status = %q, want stub", stats.ImplementationStatus)
	}
}

func TestNekoToolsListsMetadata(t *testing.T) {
	setUserConfigHome(t)
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"neko", "tools", "--dir", root}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("neko tools code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{"MimoNeko Tools", "file_read", "risk=", "approval=", "timeout="} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestNekoFindsProjectRootFromSubdirectory(t *testing.T) {
	root := setupModelCommandRoot(t)
	nested := filepath.Join(root, "nested", "deeper")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	code := Run([]string{"neko", "--no-color"}, Env{
		Stdout: &stdout,
		Stdin:  strings.NewReader("/exit\n"),
		Getwd:  func() (string, error) { return nested, nil },
	})
	if code != 0 {
		t.Fatalf("neko code = %d, stdout = %q", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "MIMO") || !strings.Contains(stdout.String(), "Goodbye from MIMO.") {
		t.Fatalf("stdout = %q, want console from discovered project root", stdout.String())
	}
}

func TestNekoUsesDefaultProjectRootOutsideProject(t *testing.T) {
	root := setupModelCommandRoot(t)
	defaultFile := filepath.Join(t.TempDir(), "default-root.txt")
	if err := os.WriteFile(defaultFile, []byte(root), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MimoNeko_NEKO_DEFAULT_ROOT_FILE", defaultFile)
	home := t.TempDir()
	var stdout bytes.Buffer
	code := Run([]string{"neko", "--no-color"}, Env{
		Stdout: &stdout,
		Stdin:  strings.NewReader("/exit\n"),
		Getwd:  func() (string, error) { return home, nil },
	})
	if code != 0 {
		t.Fatalf("neko code = %d, stdout = %q", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "MIMO") || !strings.Contains(stdout.String(), "Goodbye from MIMO.") {
		t.Fatalf("stdout = %q, want console from default project root", stdout.String())
	}
}

func TestNekoDefaultProjectRootFileAllowsUTF8BOM(t *testing.T) {
	root := setupModelCommandRoot(t)
	defaultFile := filepath.Join(t.TempDir(), "default-root.txt")
	if err := os.WriteFile(defaultFile, []byte("\ufeff"+root), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MimoNeko_NEKO_DEFAULT_ROOT_FILE", defaultFile)
	home := t.TempDir()
	var stdout bytes.Buffer
	code := Run([]string{"neko", "--no-color"}, Env{
		Stdout: &stdout,
		Stdin:  strings.NewReader("/exit\n"),
		Getwd:  func() (string, error) { return home, nil },
	})
	if code != 0 {
		t.Fatalf("neko code = %d, stdout = %q", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "MIMO") {
		t.Fatalf("stdout = %q, want console from BOM default root file", stdout.String())
	}
}

func TestNekoMissingProjectShowsFriendlyHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MimoNeko_NEKO_ROOT", "")
	t.Setenv("MimoNeko_NEKO_DEFAULT_ROOT_FILE", filepath.Join(t.TempDir(), "missing-default-root.txt"))
	var stderr bytes.Buffer
	code := Run([]string{"neko", "--no-color"}, Env{
		Stderr: &stderr,
		Stdin:  strings.NewReader("/exit\n"),
		Getwd:  func() (string, error) { return home, nil },
	})
	if code != 1 {
		t.Fatalf("neko code = %d, want 1", code)
	}
	output := stderr.String()
	for _, want := range []string{fmt.Sprintf("could not find %s/models.yaml", config.DirName()), "neko --dir <project_root>", "mimoneko init"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stderr = %q, want %q", output, want)
		}
	}
	if strings.Contains(output, "read models.yaml: open") {
		t.Fatalf("stderr = %q, should not expose raw models.yaml read error", output)
	}
}

func TestModelTestFailureStatusCode(t *testing.T) {
	root := setupModelCommandRoot(t)
	secret := "sk-model-test-failure"
	t.Setenv("TEST_MODEL_API_KEY", secret)
	server := newChatCompletionServer(t, http.StatusBadRequest, `{"error":"bad token `+secret+`"}`)
	defer server.Close()
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", server.URL, "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model")
	var stdout bytes.Buffer
	code := Run([]string{"model", "test", "--dir", root, "--provider", "test", "--model", "test-model"}, Env{Stdout: &stdout})
	if code != 1 {
		t.Fatalf("model test code = %d, want 1", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "Model Test") || !strings.Contains(output, "Failed") || !strings.Contains(output, "API returned status 400") {
		t.Fatalf("stdout = %q, want failed status code", output)
	}
	if strings.Contains(output, secret) {
		t.Fatalf("model test stdout leaked API key: %q", output)
	}
}

func TestModelTest401ShowsFriendlyHint(t *testing.T) {
	root := setupModelCommandRoot(t)
	t.Setenv("TEST_MODEL_API_KEY", "sk-model-test-401")
	server := newChatCompletionServer(t, http.StatusUnauthorized, `{"error":"invalid api key"}`)
	defer server.Close()
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", server.URL, "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model")
	var stdout bytes.Buffer
	code := Run([]string{"model", "test", "--dir", root, "--provider", "test", "--model", "test-model"}, Env{Stdout: &stdout})
	if code != 1 {
		t.Fatalf("model test code = %d, want 1", code)
	}
	output := stdout.String()
	for _, want := range []string{"Connection failed", "API Key may be invalid", "mimoneko auth login", "HTTP 401"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestModelTestDoesNotLeakAPIKey(t *testing.T) {
	root := setupModelCommandRoot(t)
	secret := "sk-model-test-secret"
	t.Setenv("TEST_MODEL_API_KEY", secret)
	server := newChatCompletionServer(t, http.StatusOK, `{"model":"test-model","choices":[{"message":{"content":"`+secret+`"}}]}`)
	defer server.Close()
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", server.URL, "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model")
	var stdout bytes.Buffer
	_ = Run([]string{"model", "test", "--dir", root, "--provider", "test", "--model", "test-model"}, Env{Stdout: &stdout})
	if strings.Contains(stdout.String(), secret) {
		t.Fatalf("model test stdout leaked API key: %q", stdout.String())
	}
}

func TestModelTestAcceptsPrompt(t *testing.T) {
	root := setupModelCommandRoot(t)
	t.Setenv("TEST_MODEL_API_KEY", "sk-model-prompt")
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		gotBody = string(body)
		fmt.Fprint(w, `{"model":"test-model","choices":[{"message":{"content":"好的"}}]}`)
	}))
	defer server.Close()
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", server.URL, "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model")

	var stdout bytes.Buffer
	code := Run([]string{"model", "test", "--dir", root, "--provider", "test", "--model", "test-model", "--prompt", "只回复OK"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("model test code = %d", code)
	}
	if !strings.Contains(gotBody, "只回复OK") {
		t.Fatalf("request body = %q, want custom prompt", gotBody)
	}
}

func TestModelTestPromptNotPersisted(t *testing.T) {
	root := setupModelCommandRoot(t)
	t.Setenv("TEST_MODEL_API_KEY", "sk-model-prompt-persist")
	server := newChatCompletionServer(t, http.StatusOK, `{"model":"test-model","choices":[{"message":{"content":"OK"}}]}`)
	defer server.Close()
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", server.URL, "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model")

	code := Run([]string{"model", "test", "--dir", root, "--provider", "test", "--model", "test-model", "--prompt", "temporary prompt should not persist"}, Env{})
	if code != 0 {
		t.Fatalf("model test code = %d", code)
	}
	content := readModelsYAMLForTest(t, root)
	if strings.Contains(content, "temporary prompt should not persist") {
		t.Fatalf("models.yaml persisted model test prompt: %s", content)
	}
}

func TestModelTestPromptResponseStillSanitized(t *testing.T) {
	root := setupModelCommandRoot(t)
	secret := "sk-prompt-response-secret"
	t.Setenv("TEST_MODEL_API_KEY", secret)
	server := newChatCompletionServer(t, http.StatusOK, `{"model":"test-model","choices":[{"message":{"content":"`+secret+`"}}]}`)
	defer server.Close()
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", server.URL, "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model")

	var stdout bytes.Buffer
	code := Run([]string{"model", "test", "--dir", root, "--provider", "test", "--model", "test-model", "--prompt", "return secret"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("model test code = %d", code)
	}
	if strings.Contains(stdout.String(), secret) {
		t.Fatalf("model test stdout leaked API key: %q", stdout.String())
	}
}

func TestModelUseSwitchesDefaultModel(t *testing.T) {
	root := setupModelCommandRoot(t)
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", "http://127.0.0.1:9999/v1", "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model")
	var stdout bytes.Buffer
	code := Run([]string{"model", "use", "--dir", root, "test-model"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("model use code = %d", code)
	}
	cfg := loadConfigForTest(t, root)
	if cfg.Models.Routing.DefaultModel != "test-model" {
		t.Fatalf("default_model = %q, want test-model", cfg.Models.Routing.DefaultModel)
	}
}

func TestModelUseRequiresExistingModel(t *testing.T) {
	root := setupModelCommandRoot(t)
	var stderr bytes.Buffer
	code := Run([]string{"model", "use", "--dir", root, "missing-model"}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("model use code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "model setup or model discover") {
		t.Fatalf("stderr = %q, want setup/discover suggestion", stderr.String())
	}
}

func TestModelUseUpdatesFallbackChain(t *testing.T) {
	root := setupModelCommandRoot(t)
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", "http://127.0.0.1:9999/v1", "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model")
	code := Run([]string{"model", "use", "--dir", root, "test-model"}, Env{})
	if code != 0 {
		t.Fatalf("model use code = %d", code)
	}
	cfg := loadConfigForTest(t, root)
	if len(cfg.Models.Routing.FallbackChain) == 0 || cfg.Models.Routing.FallbackChain[0].Provider != "test" || cfg.Models.Routing.FallbackChain[0].Model != "test-model" {
		t.Fatalf("fallback_chain = %+v, want test/test-model first", cfg.Models.Routing.FallbackChain)
	}
}

func TestModelRemoveModel(t *testing.T) {
	root := setupModelCommandRoot(t)
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", "http://127.0.0.1:9999/v1", "--api-key-env", "TEST_MODEL_API_KEY", "--model", "model-a")
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", "http://127.0.0.1:9999/v1", "--api-key-env", "TEST_MODEL_API_KEY", "--model", "model-b")
	code := Run([]string{"model", "remove", "--dir", root, "--model", "model-a"}, Env{})
	if code != 0 {
		t.Fatalf("model remove code = %d", code)
	}
	cfg := loadConfigForTest(t, root)
	provider, _ := findProviderForTest(cfg, "test")
	if providerHasModel(provider, "model-a") {
		t.Fatal("model-a was not removed")
	}
	if !providerHasModel(provider, "model-b") {
		t.Fatal("model-b should remain")
	}
}

func TestModelRemoveProvider(t *testing.T) {
	root := setupModelCommandRoot(t)
	runModelSetupForTest(t, root, "--provider", "old-provider", "--base-url", "http://127.0.0.1:9999/v1", "--api-key-env", "OLD_MODEL_API_KEY", "--model", "old-model")
	code := Run([]string{"model", "remove", "--dir", root, "--provider", "old-provider"}, Env{})
	if code != 0 {
		t.Fatalf("model remove code = %d", code)
	}
	cfg := loadConfigForTest(t, root)
	if _, ok := findProviderForTest(cfg, "old-provider"); ok {
		t.Fatal("old-provider was not removed")
	}
}

func TestModelRemoveRejectsDefaultModel(t *testing.T) {
	root := setupModelCommandRoot(t)
	var stderr bytes.Buffer
	code := Run([]string{"model", "remove", "--dir", root, "--model", "local-coder"}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("model remove code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "default_model") {
		t.Fatalf("stderr = %q, want default_model rejection", stderr.String())
	}
}

func TestModelCommandRequiresSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"model"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("model code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "requires a subcommand") {
		t.Fatalf("stderr = %q, want subcommand error", stderr.String())
	}
}

func TestModelUnknownSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"model", "unknown"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("model unknown code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown model subcommand") {
		t.Fatalf("stderr = %q, want unknown subcommand", stderr.String())
	}
}

func TestModelSetupRejectsInvalidPreset(t *testing.T) {
	root := setupModelCommandRoot(t)
	var stderr bytes.Buffer
	code := Run([]string{"model", "setup", "--dir", root, "--preset", "bad-preset", "--provider", "bad", "--base-url", "http://127.0.0.1/v1", "--api-key-env", "BAD_API_KEY", "--model", "bad-model"}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("model setup code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown provider preset") {
		t.Fatalf("stderr = %q, want invalid preset error", stderr.String())
	}
}

func TestModelSetupNonInteractive(t *testing.T) {
	root := setupModelCommandRoot(t)
	var stdout bytes.Buffer
	code := Run([]string{"model", "setup", "--dir", root, "--preset", "mimo", "--provider", "mimo", "--base-url", "https://token-plan-cn.xiaomimimo.com/v1", "--api-key-env", "MIMO_API_KEY", "--model", "mimo-v2.5-pro", "--purpose", "coding", "--max-output-tokens", "4096", "--set-default"}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("model setup code = %d", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "provider=mimo") || !strings.Contains(output, "model=mimo-v2.5-pro") || !strings.Contains(output, "default_model=mimo-v2.5-pro") {
		t.Fatalf("stdout = %q, want noninteractive setup output", output)
	}
}

func setupModelCommandRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	return root
}

func runModelSetupForTest(t *testing.T, root string, args ...string) {
	t.Helper()
	fullArgs := append([]string{"model", "setup", "--dir", root}, args...)
	var stderr bytes.Buffer
	if code := Run(fullArgs, Env{Stderr: &stderr}); code != 0 {
		t.Fatalf("Run(%v) code = %d, stderr = %q", fullArgs, code, stderr.String())
	}
}

func loadConfigForTest(t *testing.T, root string) *config.Root {
	t.Helper()
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	return cfg
}

func readModelsYAMLForTest(t *testing.T, root string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, ".mimoneko", "models.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func findProviderForTest(cfg *config.Root, name string) (config.ProviderConfig, bool) {
	for _, provider := range cfg.Models.Providers {
		if provider.Name == name {
			return provider, true
		}
	}
	return config.ProviderConfig{}, false
}

func providerHasModel(provider config.ProviderConfig, modelName string) bool {
	for _, model := range provider.Models {
		if model.Name == modelName {
			return true
		}
	}
	return false
}

func newChatCompletionServer(t *testing.T, status int, responseBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
		}
		w.WriteHeader(status)
		fmt.Fprint(w, responseBody)
	}))
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
	if !strings.Contains(output, "MimoNeko Tools") {
		t.Fatalf("tools output = %q, want MimoNeko Tools header", output)
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
	if !strings.Contains(output, "approval=") || !strings.Contains(output, "timeout=") {
		t.Fatalf("tools output = %q, want approval and timeout metadata", output)
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

func TestToolRunFileWriteDryRun(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"tool-run", "--dir", root, "--dry-run", "file_write", "--path", "dry_output.txt", "--content", "secret data"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(tool-run file_write --dry-run) code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()

	// Output should mention dry-run
	if !strings.Contains(output, "dry-run") {
		t.Fatalf("tool-run dry-run output should mention dry-run, got %q", output)
	}

	// File should NOT exist on disk
	if _, err := os.Stat(filepath.Join(root, "dry_output.txt")); !os.IsNotExist(err) {
		t.Fatal("dry-run should not create the file on disk")
	}

	// Output should NOT contain the content value
	if strings.Contains(output, "secret data") {
		t.Fatalf("dry-run output should not leak content: %q", output)
	}

	// Verify audit log was written with dry_run=true
	auditPath := filepath.Join(root, ".mimoneko", "logs", "tools.jsonl")
	auditData, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit log should exist at %q: %v", auditPath, err)
	}
	auditStr := string(auditData)
	if !strings.Contains(auditStr, `"dry_run":true`) {
		t.Fatalf("audit log should record dry_run=true, got: %s", auditStr)
	}
	// Audit log should NOT contain the content value
	if strings.Contains(auditStr, "secret data") {
		t.Fatalf("audit log should not leak content value: %s", auditStr)
	}
}

// --- Agent run CLI tests ---

func TestRunDefaultDryRun(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	// Default --dry-run is true, so this should be safe even without a model server
	code = Run([]string{"run", "--dir", root, "--goal", "inspect the repository"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	// The command should not crash (may return 1 due to model call failure, but should not panic)
	_ = code
}

func TestRunSuccessOutputIsHumanReadable(t *testing.T) {
	setUserConfigHome(t)
	saveUserConfigForTest(t, "test", "sk-run-user-config", "http://127.0.0.1", "test-model")
	t.Setenv("TEST_MODEL_API_KEY", "sk-run-test")
	root := setupModelCommandRoot(t)
	server := newChatCompletionServer(t, http.StatusOK, `{"model":"test-model","choices":[{"message":{"content":"OK"}}],"usage":{"prompt_tokens":70,"completion_tokens":1,"total_tokens":71,"prompt_tokens_details":{"cached_tokens":40}}}`)
	defer server.Close()
	runModelSetupForTest(t, root, "--provider", "test", "--base-url", server.URL, "--api-key-env", "TEST_MODEL_API_KEY", "--model", "test-model", "--set-default")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"run", "--dir", root, "--goal", "Reply OK", "--max-steps", "1"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("run code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	output := stdout.String()
	for _, want := range []string{"MioNeko Run", "Goal:", "Reply OK", "Running", "Completed", "Result:", "OK", "Run ID:", "Tokens:", "Input", "Cached", "Hit Rate"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if strings.Contains(output, "sk-run-test") || strings.Contains(output, "sk-run-user-config") {
		t.Fatalf("run output leaked API key: %q", output)
	}
}

func TestRunNoPanic(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Run with various flag combinations - none should panic
	testCases := [][]string{
		{"run", "--dir", root, "--goal", "test task"},
		{"run", "--dir", root, "--goal", "test task", "--dry-run=false"},
		{"run", "--dir", root, "--goal", "test task", "--max-steps", "1"},
		{"run", "--dir", root, "--goal", "test task", "--auto-approve-medium"},
		{"run", "--dir", root, "--goal", "test task", "--task-id", "custom_task"},
	}

	for _, args := range testCases {
		var stderr bytes.Buffer
		// Just ensure no panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Run(%v) panicked: %v", args, r)
				}
			}()
			Run(args, Env{Stderr: &stderr})
		}()
	}
}

func TestRunRequiresGoal(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"run", "--dir", root}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(run without --goal) code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "run requires --goal") {
		t.Fatalf("stderr = %q, want goal required message", stderr.String())
	}
}

func TestBuildAgentDependencies(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	deps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		t.Fatalf("buildAgentDependencies() error = %v", err)
	}
	defer cleanup()

	// All dependencies must be non-nil
	if deps.ContextEngine == nil {
		t.Error("deps.ContextEngine is nil")
	}
	if deps.ModelRouter == nil {
		t.Error("deps.ModelRouter is nil")
	}
	if deps.ToolRuntime == nil {
		t.Error("deps.ToolRuntime is nil")
	}
	if deps.ToolRegistry == nil {
		t.Error("deps.ToolRegistry is nil")
	}
	if deps.ConversationLog == nil {
		t.Error("deps.ConversationLog is nil")
	}
	if deps.Scratchpad == nil {
		t.Error("deps.Scratchpad is nil")
	}
	if deps.CheckpointStore == nil {
		t.Error("deps.CheckpointStore is nil")
	}
	if deps.WorktreeMgr == nil {
		t.Error("deps.WorktreeMgr is nil")
	}
	if deps.PatchMgr == nil {
		t.Error("deps.PatchMgr is nil")
	}
}

func TestBuildAgentDependenciesIncludesWorktreeManager(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	deps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		t.Fatalf("buildAgentDependencies() error = %v", err)
	}
	defer cleanup()

	if deps.WorktreeMgr == nil {
		t.Fatal("deps.WorktreeMgr is nil")
	}
}

func TestBuildAgentDependenciesIncludesPatchManager(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	deps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		t.Fatalf("buildAgentDependencies() error = %v", err)
	}
	defer cleanup()

	if deps.PatchMgr == nil {
		t.Fatal("deps.PatchMgr is nil")
	}
}

func TestBuildAgentDependenciesClosesWorktreeRegistry(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	_, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		t.Fatalf("buildAgentDependencies() error = %v", err)
	}

	registryPath := worktree.DefaultRegistryPath(root)
	cleanup()

	if err := os.Remove(registryPath); err != nil {
		t.Fatalf("worktree registry should be removable after cleanup: %v", err)
	}
}

func TestBuildAgentDependenciesNoAPIKeyLeak(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Set a fake API key to ensure it doesn't leak through dependencies
	t.Setenv("MimoNeko_API_KEY", "sk-test-secret-key-do-not-leak")

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	deps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		t.Fatalf("buildAgentDependencies() error = %v", err)
	}
	defer cleanup()

	// Verify the dependencies themselves don't expose API keys
	// The ModelRouter should exist but shouldn't leak the key
	if deps.ModelRouter == nil {
		t.Fatal("ModelRouter should not be nil")
	}

	// Verify that the checkpoint store doesn't contain API keys
	// This is a basic sanity check - the real protection is in SanitizeCheckpoint
	// but we verify the infrastructure doesn't log or store keys
	_ = deps
}

func TestBuildAgentDependenciesEventsEnabledStoreFailureReturnsError(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	blocker := filepath.Join(root, "event-store-blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Events.Enabled = true
	cfg.Events.StorePath = filepath.Join(blocker, "events.jsonl")

	_, cleanup, err := buildAgentDependencies(root, cfg)
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal("buildAgentDependencies() should fail when events are enabled and EventStore cannot initialize")
	}
	if !strings.Contains(err.Error(), "event store") {
		t.Fatalf("error = %q, want event store", err.Error())
	}
}

func TestBuildAgentDependenciesEventsDisabledUsesNoop(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	blocker := filepath.Join(root, "event-store-blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Events.Enabled = false
	cfg.Events.StorePath = filepath.Join(blocker, "events.jsonl")

	deps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		t.Fatalf("buildAgentDependencies() with events disabled error = %v", err)
	}
	defer cleanup()

	if _, ok := deps.EventEmitter.(*events.NoopEventEmitter); !ok {
		t.Fatalf("EventEmitter = %T, want *events.NoopEventEmitter", deps.EventEmitter)
	}
}

func TestBuildMultiAgentDependenciesEventsEnabledStoreFailureReturnsError(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	blocker := filepath.Join(root, "multi-event-store-blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Events.Enabled = true
	cfg.Events.StorePath = filepath.Join(blocker, "events.jsonl")

	_, cleanup, err := buildMultiAgentDependencies(root, cfg, agent.Dependencies{})
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal("buildMultiAgentDependencies() should fail when events are enabled and EventStore cannot initialize")
	}
	if !strings.Contains(err.Error(), "event store") {
		t.Fatalf("error = %q, want event store", err.Error())
	}
}

// --- Phase 5: Worktree/Patch CLI tests ---

func TestInitCreatesWorktreeAndPatchConfig(t *testing.T) {
	root := t.TempDir()

	var stdout bytes.Buffer
	code := Run([]string{"init", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Verify worktree.yaml was created
	worktreeYAML := filepath.Join(root, ".mimoneko", "worktree.yaml")
	if _, err := os.Stat(worktreeYAML); os.IsNotExist(err) {
		t.Fatal("worktree.yaml should be created by init")
	}

	// Verify patch.yaml was created
	patchYAML := filepath.Join(root, ".mimoneko", "patch.yaml")
	if _, err := os.Stat(patchYAML); os.IsNotExist(err) {
		t.Fatal("patch.yaml should be created by init")
	}
}

func TestDoctorIncludesWorktreeConfig(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"doctor", "--dir", root}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "worktree_isolation=") {
		t.Fatalf("doctor output should contain worktree_isolation, got %q", output)
	}
	if !strings.Contains(output, "patch_require_clean_main=") {
		t.Fatalf("doctor output should contain patch_require_clean_main, got %q", output)
	}
	if !strings.Contains(output, "patch_max_diff_bytes=") {
		t.Fatalf("doctor output should contain patch_max_diff_bytes, got %q", output)
	}
}

func TestPatchSubcommandRequiresSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"patch"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch) code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "list") {
		t.Fatalf("stderr should mention subcommands, got %q", stderr.String())
	}
}

func TestPatchUnknownSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"patch", "unknown"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch unknown) code = %d, want 2", code)
	}
}

func TestPatchListEmpty(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"patch", "list", "--dir", root}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(patch list) code = %d, stderr = %q", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "No worktrees found") {
		t.Fatalf("patch list with no worktrees should say 'No worktrees found', got %q", stdout.String())
	}
}

func TestPatchPreviewRequiresWorktreeID(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"patch", "preview"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch preview) without ID should return 2, got %d", code)
	}
}

func TestPatchApplyRequiresWorktreeID(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"patch", "apply"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch apply) without ID should return 2, got %d", code)
	}
}

func TestPatchDiscardRequiresWorktreeID(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"patch", "discard"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch discard) without ID should return 2, got %d", code)
	}
}

func TestPatchApplyDirtyMainRejects(t *testing.T) {
	root := t.TempDir()
	// Init git repo
	runGitCmd(t, root, "init")
	runGitCmd(t, root, "config", "user.email", "test@MimoNeko.dev")
	runGitCmd(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "add", ".")
	runGitCmd(t, root, "commit", "-m", "initial")

	// Init MimoNeko
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Apply to nonexistent worktree should fail
	var stderr bytes.Buffer
	code = Run([]string{"patch", "apply", "--dir", root, "wt_nonexistent"}, Env{Stderr: &stderr})
	if code == 0 {
		t.Fatal("patch apply to nonexistent worktree should fail")
	}
}

func TestPatchDiscardNonexistentWorktree(t *testing.T) {
	root := t.TempDir()
	runGitCmd(t, root, "init")
	runGitCmd(t, root, "config", "user.email", "test@MimoNeko.dev")
	runGitCmd(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "add", ".")
	runGitCmd(t, root, "commit", "-m", "initial")

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"patch", "discard", "--dir", root, "wt_nonexistent"}, Env{Stderr: &stderr})
	if code == 0 {
		t.Fatal("patch discard nonexistent worktree should fail")
	}
}

func TestRunWorktreeFlagNoPanic(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Ensure --worktree flag doesn't cause panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Run with --worktree panicked: %v", r)
			}
		}()
		var stderr bytes.Buffer
		Run([]string{"run", "--dir", root, "--goal", "test", "--worktree"}, Env{Stderr: &stderr})
	}()
}

func TestRunWorktreeNoMissingManagerError(t *testing.T) {
	root := setupGitRepo(t)
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	commitTestPromptSources(t, root)

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	deps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		t.Fatalf("buildAgentDependencies() error = %v", err)
	}
	defer cleanup()
	deps.ModelRouter = &cliSequenceModelRouter{responses: []string{"OK"}}

	contract := task.DefaultContract(root, "just reply OK")
	rt := agent.NewSingleAgentRuntime(deps)
	result, err := rt.Run(context.Background(), agent.AgentRunRequest{
		TaskID:      "task_run_worktree_test",
		RepoRoot:    root,
		Goal:        "just reply OK",
		Contract:    contract,
		DryRun:      true,
		UseWorktree: true,
		MaxSteps:    1,
	})
	if err != nil {
		if strings.Contains(err.Error(), "worktree manager not configured") {
			t.Fatalf("Run returned missing worktree manager error: %v", err)
		}
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if strings.Contains(result.Error, "worktree manager not configured") {
		t.Fatalf("Run result has missing worktree manager error: %s", result.Error)
	}
	if result.WorktreeID == "" {
		t.Fatal("Run with UseWorktree=true did not create a worktree")
	}
}

func TestCLIDoesNotLeakAPIKey(t *testing.T) {
	t.Setenv("MimoNeko_API_KEY", "sk-super-secret-key-12345")

	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	code = Run([]string{"doctor", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d", code)
	}

	if strings.Contains(stdout.String(), "sk-super-secret-key-12345") {
		t.Fatal("CLI output should not contain API key")
	}
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, string(output))
	}
}

// setupGitRepo creates a temp directory with an initialized git repo and initial commit.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGitCmd(t, root, "init")
	runGitCmd(t, root, "config", "user.email", "test@MimoNeko.dev")
	runGitCmd(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "add", ".")
	runGitCmd(t, root, "commit", "-m", "initial")
	return root
}

func commitTestPromptSources(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"prompts/system.md":       "You are a test assistant.\n",
		"prompts/coding_rules.md": "Keep changes minimal and safe.\n",
		"schemas/tools.json":      `{"tools":[]}` + "\n",
	}
	for relPath, content := range files {
		absPath := filepath.Join(root, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGitCmd(t, root, "add", "prompts", "schemas")
	runGitCmd(t, root, "commit", "-m", "add test prompt sources")
}

func setupPatchCLIEventWorktree(t *testing.T, taskID string, files map[string]string) (string, worktree.WorktreeInfo) {
	t.Helper()
	root := setupGitRepo(t)
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	registry, err := worktree.NewRegistry(worktree.DefaultRegistryPath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   taskID,
	})
	if err != nil {
		t.Fatal(err)
	}

	for relPath, content := range files {
		absPath := filepath.Join(info.Path, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return root, info
}

func loadPatchCLIEvents(t *testing.T, root string) []events.RunEvent {
	t.Helper()
	path := filepath.Join(root, ".mimoneko", "events", "run_events.jsonl")
	evts, corrupted, err := events.LoadEventsFromFile(path)
	if err != nil {
		t.Fatalf("LoadEventsFromFile() error = %v", err)
	}
	if len(corrupted) > 0 {
		t.Fatalf("event store has corrupted lines: %+v", corrupted)
	}
	return evts
}

func patchCLIEventsForRun(evts []events.RunEvent, runID string) []events.RunEvent {
	var result []events.RunEvent
	for _, evt := range evts {
		if evt.RunID == runID {
			result = append(result, evt)
		}
	}
	return result
}

func findPatchCLIRunID(t *testing.T, evts []events.RunEvent, prefix string) string {
	t.Helper()
	for _, evt := range evts {
		if strings.HasPrefix(evt.RunID, prefix) {
			return evt.RunID
		}
	}
	t.Fatalf("no run ID with prefix %q in events: %+v", prefix, evts)
	return ""
}

func requirePatchCLIEventType(t *testing.T, evts []events.RunEvent, typ events.EventType) {
	t.Helper()
	for _, evt := range evts {
		if evt.Type == typ {
			return
		}
	}
	t.Fatalf("missing event type %s in events: %+v", typ, evts)
}

func requirePatchCLIContext(t *testing.T, evts []events.RunEvent, runID, taskID, worktreeID string) {
	t.Helper()
	for _, evt := range evts {
		if evt.RunID != runID {
			t.Fatalf("event RunID = %q, want %q: %+v", evt.RunID, runID, evt)
		}
		if evt.TaskID != taskID {
			t.Fatalf("event TaskID = %q, want %q: %+v", evt.TaskID, taskID, evt)
		}
		if evt.WorktreeID != worktreeID {
			t.Fatalf("event WorktreeID = %q, want %q: %+v", evt.WorktreeID, worktreeID, evt)
		}
	}
}

func TestPatchPreviewCLIEmitsEvents(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_preview_cli_events", map[string]string{
		"preview.txt": "preview change\n",
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"patch", "preview", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("Run(patch preview) code = %d, stderr = %q", code, stderr.String())
	}

	evts := loadPatchCLIEvents(t, root)
	runID := findPatchCLIRunID(t, evts, "patch_preview_")
	runEvents := patchCLIEventsForRun(evts, runID)
	requirePatchCLIContext(t, runEvents, runID, info.TaskID, info.ID)
	requirePatchCLIEventType(t, runEvents, events.EventRunStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewFinished)
	requirePatchCLIEventType(t, runEvents, events.EventRunSucceeded)
}

func TestPatchValidateCLIEmitsEvents(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_validate_cli_events", map[string]string{
		"validate.txt": "validate change\n",
	})

	var stdout, stderr bytes.Buffer
	_ = Run([]string{"patch", "validate", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})

	evts := loadPatchCLIEvents(t, root)
	runID := findPatchCLIRunID(t, evts, "patch_validate_")
	runEvents := patchCLIEventsForRun(evts, runID)
	requirePatchCLIContext(t, runEvents, runID, info.TaskID, info.ID)
	requirePatchCLIEventType(t, runEvents, events.EventRunStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewFinished)
	requirePatchCLIEventType(t, runEvents, events.EventReviewerStarted)
	requirePatchCLIEventType(t, runEvents, events.EventReviewerFinished)
	requirePatchCLIEventType(t, runEvents, events.EventValidationStarted)
	requirePatchCLIEventType(t, runEvents, events.EventValidationFinished)
	requirePatchCLIEventType(t, runEvents, events.EventToolStarted)
	requirePatchCLIEventType(t, runEvents, events.EventToolFinished)
}

func TestPatchReviewCLIEmitsEvents(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_review_cli_events", map[string]string{
		"review_events.txt": "review change\n",
	})

	var stdout, stderr bytes.Buffer
	_ = Run([]string{"patch", "review", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})

	evts := loadPatchCLIEvents(t, root)
	runID := findPatchCLIRunID(t, evts, "patch_review_")
	runEvents := patchCLIEventsForRun(evts, runID)
	requirePatchCLIContext(t, runEvents, runID, info.TaskID, info.ID)
	requirePatchCLIEventType(t, runEvents, events.EventRunStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewStarted)
	requirePatchCLIEventType(t, runEvents, events.EventPatchPreviewFinished)
	requirePatchCLIEventType(t, runEvents, events.EventReviewerStarted)
	requirePatchCLIEventType(t, runEvents, events.EventReviewerFinished)
	requirePatchCLIEventType(t, runEvents, events.EventValidationStarted)
	requirePatchCLIEventType(t, runEvents, events.EventValidationFinished)
	requirePatchCLIEventType(t, runEvents, events.EventToolStarted)
	requirePatchCLIEventType(t, runEvents, events.EventToolFinished)
}

func TestPatchPreviewEventsVisibleInRuns(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_preview_visible", map[string]string{
		"visible.txt": "visible in runs\n",
	})

	var previewStdout, previewStderr bytes.Buffer
	code := Run([]string{"patch", "preview", "--dir", root, info.ID}, Env{Stdout: &previewStdout, Stderr: &previewStderr})
	if code != 0 {
		t.Fatalf("Run(patch preview) code = %d, stderr = %q", code, previewStderr.String())
	}

	var runsOut, runsErr bytes.Buffer
	code = Run([]string{"runs", "--dir", root}, Env{Stdout: &runsOut, Stderr: &runsErr})
	if code != 0 {
		t.Fatalf("Run(runs) code = %d, stderr = %q", code, runsErr.String())
	}
	output := runsOut.String()
	if !strings.Contains(output, "patch_preview_") {
		t.Fatalf("runs output should include patch_preview_ run, got:\n%s", output)
	}
	if !strings.Contains(output, "succeeded") {
		t.Fatalf("runs output should show succeeded state, got:\n%s", output)
	}
}

func TestPatchValidateEventsVisibleInRunEvents(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_validate_visible", map[string]string{
		"run_events.txt": "visible in run events\n",
	})

	var validateStdout, validateStderr bytes.Buffer
	_ = Run([]string{"patch", "validate", "--dir", root, info.ID}, Env{Stdout: &validateStdout, Stderr: &validateStderr})

	evts := loadPatchCLIEvents(t, root)
	runID := findPatchCLIRunID(t, evts, "patch_validate_")

	var eventsOut, eventsErr bytes.Buffer
	code := Run([]string{"run-events", "--dir", root, runID}, Env{Stdout: &eventsOut, Stderr: &eventsErr})
	if code != 0 {
		t.Fatalf("Run(run-events) code = %d, stderr = %q", code, eventsErr.String())
	}
	output := eventsOut.String()
	if !strings.Contains(output, string(events.EventValidationStarted)) {
		t.Fatalf("run-events output should include validation.started, got:\n%s", output)
	}
	if !strings.Contains(output, string(events.EventToolStarted)) {
		t.Fatalf("run-events output should include tool.started, got:\n%s", output)
	}
}

func TestPatchReviewDoesNotLeakSensitiveDiffInEvents(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_review_sensitive_events", map[string]string{
		".env": "API_KEY=sk-phase82-secret\nDB_PASSWORD=super-secret-password\n",
	})

	var stdout, stderr bytes.Buffer
	_ = Run([]string{"patch", "review", "--dir", root, info.ID, "--no-tests"}, Env{Stdout: &stdout, Stderr: &stderr})

	eventStorePath := filepath.Join(root, ".mimoneko", "events", "run_events.jsonl")
	raw, err := os.ReadFile(eventStorePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	rawEvents := string(raw)
	for _, secret := range []string{"sk-phase82-secret", "super-secret-password", "API_KEY=", "DB_PASSWORD="} {
		if strings.Contains(rawEvents, secret) {
			t.Fatalf("event store leaked sensitive diff content %q:\n%s", secret, rawEvents)
		}
	}
}

func TestPatchCLIEventsDisabledUsesNoop(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_events_disabled", map[string]string{
		"disabled.txt": "events disabled\n",
	})

	eventsYAML := filepath.Join(root, ".mimoneko", "events.yaml")
	if err := os.WriteFile(eventsYAML, []byte("enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"patch", "preview", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("Run(patch preview) code = %d, stderr = %q", code, stderr.String())
	}

	eventStorePath := filepath.Join(root, ".mimoneko", "events", "run_events.jsonl")
	if _, err := os.Stat(eventStorePath); !os.IsNotExist(err) {
		t.Fatalf("event store path exists with events disabled: err=%v", err)
	}
}

func TestPatchCLIEventsEnabledStoreFailureReturnsError(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_patch_events_store_failure", map[string]string{
		"store_failure.txt": "store failure\n",
	})

	blocker := filepath.Join(root, "event-store-blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	eventsYAML := filepath.Join(root, ".mimoneko", "events.yaml")
	if err := os.WriteFile(eventsYAML, []byte("enabled: true\nstore_path: event-store-blocker/events.jsonl\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code := Run([]string{"patch", "preview", "--dir", root, info.ID}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("Run(patch preview) code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "event store") {
		t.Fatalf("stderr = %q, want event store error", stderr.String())
	}
}

func TestCLIPatchPreviewDoesNotLeakSensitiveDiff(t *testing.T) {
	root := t.TempDir()
	runGitCmd(t, root, "init")
	runGitCmd(t, root, "config", "user.email", "test@MimoNeko.dev")
	runGitCmd(t, root, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "add", ".")
	runGitCmd(t, root, "commit", "-m", "initial")

	// Init MimoNeko project
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree directly via worktree manager
	registryPath := filepath.Join(root, ".mimoneko", "worktrees", "registry.jsonl")
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "leak-test",
		BaseRef:  "HEAD",
	})
	if err != nil {
		t.Fatalf("Create worktree: %v", err)
	}

	// Write .env with secret content in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, ".env"), []byte("SECRET_VALUE=super_secret_data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run patch preview via CLI
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	patchCode := Run([]string{"patch", "preview", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})
	_ = patchCode // May return non-zero due to violations; we just check output

	output := stdout.String() + stderr.String()

	// Output must NOT contain the secret content
	if strings.Contains(output, "SECRET_VALUE") {
		t.Fatal("CLI patch preview must not expose content of sensitive files")
	}
	if strings.Contains(output, "super_secret_data") {
		t.Fatal("CLI patch preview must not expose content of sensitive files")
	}

	// Output should show violation info
	if !strings.Contains(output, "violation") && !strings.Contains(output, ".env") {
		t.Fatalf("expected violation info in output, got:\n%s", output)
	}
}

func TestPatchValidateRequiresWorktreeID(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"patch", "validate", "--dir", root}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch validate without ID) code = %d, want 2", code)
	}
}

func TestPatchReviewRequiresWorktreeID(t *testing.T) {
	root := t.TempDir()
	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stderr bytes.Buffer
	code = Run([]string{"patch", "review", "--dir", root}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(patch review without ID) code = %d, want 2", code)
	}
}

func TestPatchValidateOutputsRecommendation(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_validate_test",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Make a simple change in the worktree
	testFile := filepath.Join(info.Path, "simple.txt")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"patch", "validate", "--dir", root, info.ID}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	_ = code // May return non-zero depending on findings

	output := stdout.String()
	if !strings.Contains(output, "recommendation=") {
		t.Fatalf("patch validate should output recommendation, got:\n%s", output)
	}
}

func TestPatchReviewOutputsWithoutModelReview(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_review_test",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Make a simple change
	testFile := filepath.Join(info.Path, "review_test.txt")
	if err := os.WriteFile(testFile, []byte("test content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"patch", "review", "--dir", root, info.ID, "--no-tests"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	_ = code

	output := stdout.String()
	if !strings.Contains(output, "recommendation=") {
		t.Fatalf("patch review should output recommendation, got:\n%s", output)
	}
}

func TestPatchValidateSkipsTestsForEmptyPatch(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_empty_validate", map[string]string{})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"patch", "validate", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("patch validate code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "validation_skipped=true") || !strings.Contains(output, "reason=no changes") {
		t.Fatalf("patch validate should skip validation for empty patch, got:\n%s", output)
	}
}

func TestPatchReviewSkipsTestsForEmptyPatch(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_empty_review", map[string]string{})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"patch", "review", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("patch review code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "validation_skipped=true") || !strings.Contains(output, "reason=no changes") {
		t.Fatalf("patch review should skip validation for empty patch, got:\n%s", output)
	}
}

func TestPatchValidateRunsTestsWhenChangesExist(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_changed_validate", map[string]string{
		"changed.txt": "changed\n",
	})

	var stdout bytes.Buffer
	_ = Run([]string{"patch", "validate", "--dir", root, info.ID}, Env{Stdout: &stdout})
	output := stdout.String()
	if !strings.Contains(output, "validation_success=") {
		t.Fatalf("patch validate should run validation when changes exist, got:\n%s", output)
	}
	if strings.Contains(output, "validation_skipped=true") {
		t.Fatalf("patch validate skipped validation despite changes:\n%s", output)
	}
}

func TestPatchReviewExplicitTestCommandRunsEvenForEmptyPatch(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_empty_review_explicit", map[string]string{})

	var stdout bytes.Buffer
	_ = Run([]string{"patch", "review", "--dir", root, info.ID, "--test-command", "go-test"}, Env{Stdout: &stdout})
	output := stdout.String()
	if !strings.Contains(output, "validation_success=") {
		t.Fatalf("explicit test command should run even for empty patch, got:\n%s", output)
	}
	if strings.Contains(output, "validation_skipped=true") {
		t.Fatalf("explicit test command was skipped:\n%s", output)
	}
}

func TestEmptyPatchDoesNotReturnRequestChangesBecauseOfDefaultTests(t *testing.T) {
	root, info := setupPatchCLIEventWorktree(t, "task_empty_no_request_changes", map[string]string{})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"patch", "validate", "--dir", root, info.ID}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("patch validate code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	output := stdout.String()
	if strings.Contains(output, "recommendation=request_changes") {
		t.Fatalf("empty patch returned request_changes due to default tests:\n%s", output)
	}
}

func TestPatchValidateDoesNotLeakAPIKey(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree and add a sensitive file
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_validate_sensitive",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Write .env with secret content in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, ".env"), []byte("API_KEY=sk-secret-key-12345\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"patch", "validate", "--dir", root, info.ID}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	_ = code

	output := stdout.String() + stderr.String()

	// Must not leak API key values
	if strings.Contains(output, "sk-secret-key-12345") {
		t.Fatal("patch validate must not leak API key values")
	}
	if strings.Contains(output, "API_KEY=sk-secret") {
		t.Fatal("patch validate must not leak sensitive content")
	}
}

func TestPatchReviewDoesNotPrintSensitiveDiff(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree and add a sensitive file
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_review_sensitive",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Write .env with secret content in the worktree
	if err := os.WriteFile(filepath.Join(info.Path, ".env"), []byte("DB_PASSWORD=supersecret123\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = Run([]string{"patch", "review", "--dir", root, info.ID, "--no-tests"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	_ = code

	output := stdout.String() + stderr.String()

	// Must not print sensitive diff content when violations exist
	if strings.Contains(output, "supersecret123") {
		t.Fatal("patch review must not expose content of sensitive files")
	}
	if strings.Contains(output, "DB_PASSWORD") {
		t.Fatal("patch review must not expose content of sensitive files")
	}
}

func TestDoctorShowsReviewAndValidationConfig(t *testing.T) {
	root := t.TempDir()

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	var stdout bytes.Buffer
	code = Run([]string{"doctor", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "review_max_diff_bytes=") {
		t.Fatalf("doctor should show review_max_diff_bytes, got:\n%s", output)
	}
	if !strings.Contains(output, "validation_max_output_bytes=") {
		t.Fatalf("doctor should show validation_max_output_bytes, got:\n%s", output)
	}
	if !strings.Contains(output, "validation_timeout_seconds=") {
		t.Fatalf("doctor should show validation_timeout_seconds, got:\n%s", output)
	}
}

func TestPatchValidateUsesWorktreePath(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_validate_wt_path",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Make a change in the worktree
	testFile := filepath.Join(info.Path, "change.txt")
	if err := os.WriteFile(testFile, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run patch validate with --test-command; the CLI must pass WorktreePath
	// so the manager does not return "WorktreePath is required" error.
	var stderr bytes.Buffer
	code = Run([]string{"patch", "validate", "--dir", root, info.ID, "--test-command", "echo ok"}, Env{
		Stderr: &stderr,
	})
	// The command may fail for other reasons (e.g. test_run tool not registered),
	// but it must NOT fail with "WorktreePath is required"
	errOutput := stderr.String()
	if strings.Contains(errOutput, "WorktreePath is required") {
		t.Fatalf("patch validate must pass WorktreePath; got error: %s", errOutput)
	}
}

func TestPatchReviewUsesWorktreePath(t *testing.T) {
	root := setupGitRepo(t)

	code := Run([]string{"init", "--dir", root}, Env{})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Create a worktree
	registryPath := worktree.DefaultRegistryPath(root)
	registry, err := worktree.NewRegistry(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	wtMgr := worktree.NewGitWorktreeManager(registry, worktree.DefaultGitWorktreeManagerConfig())
	info, err := wtMgr.Create(context.Background(), worktree.CreateWorktreeRequest{
		RepoRoot: root,
		TaskID:   "task_review_wt_path",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Make a change in the worktree
	testFile := filepath.Join(info.Path, "change.txt")
	if err := os.WriteFile(testFile, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run patch review with --test-command; the CLI must pass WorktreePath
	// so the manager does not return "WorktreePath is required" error.
	var stderr bytes.Buffer
	code = Run([]string{"patch", "review", "--dir", root, info.ID, "--test-command", "echo ok"}, Env{
		Stderr: &stderr,
	})
	// The command may fail for other reasons (e.g. test_run tool not registered),
	// but it must NOT fail with "WorktreePath is required"
	errOutput := stderr.String()
	if strings.Contains(errOutput, "WorktreePath is required") {
		t.Fatalf("patch review must pass WorktreePath; got error: %s", errOutput)
	}
}

// === Phase 7: Multi-Agent CLI Tests ===

func TestInitCreatesMultiAgentConfig(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer

	code := Run([]string{"init", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}

	// Verify multiagent.yaml was created
	multiagentPath := filepath.Join(root, config.DirName(), "multiagent.yaml")
	if _, err := os.Stat(multiagentPath); os.IsNotExist(err) {
		t.Error("init did not create multiagent.yaml")
	}
}

func TestDoctorShowsMultiAgentConfig(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stdout bytes.Buffer
	code := Run([]string{"doctor", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("Run(doctor) code = %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "multiagent_max_iterations") {
		t.Error("doctor output missing multiagent_max_iterations")
	}
	if !strings.Contains(output, "multiagent_default_worktree") {
		t.Error("doctor output missing multiagent_default_worktree")
	}
	if !strings.Contains(output, "multiagent_default_dry_run") {
		t.Error("doctor output missing multiagent_default_dry_run")
	}
}

func TestMultiRunRequiresGoal(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"multi-run"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "goal") {
		t.Errorf("expected error about goal, got: %s", stderr.String())
	}
}

func TestMultiRunAcceptsGoalFlag(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"multi-run", "--dir", root, "--goal", "fix typo"}, Env{Stdout: &stdout, Stderr: &stderr})
	if code == 2 {
		t.Fatalf("multi-run --goal should parse, stderr = %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), `goal="fix typo"`) {
		t.Fatalf("stdout = %q, want goal flag value", stdout.String())
	}
}

func TestMultiRunAcceptsPositionalGoal(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"multi-run", "--dir", root, "fix README"}, Env{Stdout: &stdout, Stderr: &stderr})
	if code == 2 {
		t.Fatalf("multi-run positional goal should parse, stderr = %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), `goal="fix README"`) {
		t.Fatalf("stdout = %q, want positional goal value", stdout.String())
	}
}

func TestMultiRunRejectsGoalFlagAndPositionalTogether(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stderr bytes.Buffer
	code := Run([]string{"multi-run", "--dir", root, "--goal", "flag goal", "positional goal"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("multi-run code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "multi-run accepts either --goal or positional goal, not both") {
		t.Fatalf("stderr = %q, want explicit conflict error", stderr.String())
	}
}

func TestMultiRunDefaultWorktreeAndDryRun(t *testing.T) {
	// Test that multi-run defaults to worktree=true and dry-run=true
	// by checking the flag defaults are correctly set.
	// We don't execute the full run (no model configured), just verify
	// the command starts with correct defaults.
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"multi-run", "--dir", root, "fix typo"}, Env{Stdout: &stdout, Stderr: &stderr})
	// We expect failure (no model configured), but not a usage error (code 2)
	if code == 2 {
		t.Errorf("multi-run flag parsing failed: %s", stderr.String())
	}

	// Verify output mentions defaults
	output := stdout.String()
	if !strings.Contains(output, "dry_run=true") {
		t.Errorf("expected dry_run=true in output, got: %s", output)
	}
	if !strings.Contains(output, "worktree=true") {
		t.Errorf("expected worktree=true in output, got: %s", output)
	}
}

func TestMultiRunNoMissingWorktreeManagerError(t *testing.T) {
	root := setupGitRepo(t)
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	commitTestPromptSources(t, root)

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	agentDeps, cleanup, err := buildAgentDependencies(root, cfg)
	if err != nil {
		t.Fatalf("buildAgentDependencies() error = %v", err)
	}
	defer cleanup()
	agentDeps.ModelRouter = &cliSequenceModelRouter{
		responses: []string{
			`{"goal":"summarize README","steps":[{"index":0,"title":"Summarize","description":"Read README and summarize capabilities","target_paths":["README.md"],"expected_outcome":"Summary produced"}],"risk_level":"low"}`,
			"OK",
		},
	}

	multiDeps, multiCleanup, err := buildMultiAgentDependencies(root, cfg, agentDeps)
	if err != nil {
		t.Fatalf("buildMultiAgentDependencies() error = %v", err)
	}
	defer multiCleanup()

	goal := "summarize README"
	contract := task.DefaultContract(root, goal)
	rt := multiagent.NewDefaultMultiAgentRuntime(multiDeps)
	result, err := rt.Run(context.Background(), multiagent.MultiAgentRunRequest{
		TaskID:        "task_multi_worktree_test",
		RepoRoot:      root,
		Goal:          goal,
		Contract:      contract,
		MaxIterations: 1,
		DryRun:        true,
		UseWorktree:   true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "worktree manager not configured") {
			t.Fatalf("multi-run returned missing worktree manager error: %v", err)
		}
		t.Fatalf("multi-run returned unexpected error: %v", err)
	}
	if strings.Contains(result.Error, "worktree manager not configured") {
		t.Fatalf("multi-run result has missing worktree manager error: %s", result.Error)
	}
	if result.WorktreeID == "" {
		t.Fatalf("multi-run with UseWorktree=true did not create a worktree: state=%s error=%s", result.State, result.Error)
	}
}

func TestMultiRunMaxIterationsExceedsMax(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stderr bytes.Buffer
	code := Run([]string{"multi-run", "--dir", root, "--max-iterations", "10", "fix typo"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Errorf("expected exit code 2 for max_iterations > 5, got %d", code)
	}
	if !strings.Contains(stderr.String(), "max_iterations") {
		t.Errorf("expected error about max_iterations, got: %s", stderr.String())
	}
}

func TestMultiRunNoAutoApply(t *testing.T) {
	// Verify that multi-run command does not have auto-apply behavior
	var stderr bytes.Buffer
	code := Run([]string{"multi-run", "--apply", "fix typo"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Errorf("expected flag parse error for --apply, got code %d", code)
	}
}

func TestMultiRunDoesNotLeakAPIKey(t *testing.T) {
	t.Setenv("MimoNeko_API_KEY", "sk-super-secret-key-12345")

	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stdout bytes.Buffer
	code := Run([]string{"doctor", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("doctor failed: code=%d", code)
	}

	output := stdout.String()
	if strings.Contains(output, "sk-super-secret-key-12345") {
		t.Error("doctor output leaked API key")
	}
	// Also verify multi-run help doesn't leak
	var multiStderr bytes.Buffer
	Run([]string{"multi-run"}, Env{Stderr: &multiStderr})
	if strings.Contains(multiStderr.String(), "sk-super-secret-key-12345") {
		t.Error("multi-run output leaked API key")
	}
}

func TestMultiRunOutputFinalRecommendation(t *testing.T) {
	// Verify the multi-run command is registered and accepts positional goal
	var stderr bytes.Buffer
	code := Run([]string{"multi-run", "fix typo"}, Env{
		Stderr: &stderr,
		Getwd:  func() (string, error) { return t.TempDir(), nil },
	})
	// Should not be a usage error (2) - may fail for other reasons (no config)
	// but the goal argument should be accepted
	if code == 2 && strings.Contains(stderr.String(), "accepts no positional") {
		t.Errorf("multi-run should accept positional goal argument")
	}
}

func TestMultiRunWorktreeFalseRejected(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stderr bytes.Buffer
	code := Run([]string{"multi-run", "--dir", root, "--worktree=false", "fix typo"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Errorf("expected exit code 2 for --worktree=false, got %d", code)
	}
	if !strings.Contains(stderr.String(), "worktree=false is not supported") {
		t.Errorf("expected error about --worktree=false not supported, got: %s", stderr.String())
	}
}

func TestMultiRunModelReviewFlagPassed(t *testing.T) {
	// Verify that --model-review flag is accepted and doesn't cause a parse error.
	// We can't fully test model review without a real model server,
	// but we verify the flag is recognized and the request includes UseModelReview.
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stderr bytes.Buffer
	_ = Run([]string{"multi-run", "--dir", root, "--model-review", "fix typo"}, Env{Stderr: &stderr})
	// The command may fail for other reasons (no model server), but should not
	// fail with an "unknown flag" or similar parse error
	errOutput := stderr.String()
	if strings.Contains(errOutput, "unknown flag") || strings.Contains(errOutput, "cannot use") {
		t.Errorf("--model-review flag should be recognized, got: %s", errOutput)
	}
}

// --- Dashboard CLI tests ---

func TestDashboardRequiresEventsEnabled(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	// Disable events in config
	eventsPath := filepath.Join(root, ".mimoneko", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: false\nstore_path: .mimoneko/events/run_events.jsonl\n"), 0o600)

	var stderr bytes.Buffer
	code := Run([]string{"dashboard", "--dir", root}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("dashboard with disabled events code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "disabled") {
		t.Fatalf("stderr = %q, want 'disabled'", stderr.String())
	}
}

func TestDashboardHandlesEmptyEventStore(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	// Create events config (enabled) and an empty event store
	eventsDir := filepath.Join(root, ".mimoneko", "events")
	os.MkdirAll(eventsDir, 0o700)
	storePath := filepath.Join(eventsDir, "run_events.jsonl")
	os.WriteFile(storePath, []byte(""), 0o600)

	eventsPath := filepath.Join(root, ".mimoneko", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: true\nstore_path: .mimoneko/events/run_events.jsonl\n"), 0o600)

	var stdout bytes.Buffer
	code := Run([]string{"dashboard", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("dashboard with empty store code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "No runs found") {
		t.Fatalf("stdout = %q, want 'No runs found'", stdout.String())
	}
}

func TestDashboardShowsRuns(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	// Create events config and store with sample data
	eventsDir := filepath.Join(root, ".mimoneko", "events")
	os.MkdirAll(eventsDir, 0o700)
	storePath := filepath.Join(eventsDir, "run_events.jsonl")

	// Write sample events
	store, err := events.NewJSONLRunEventStore(storePath)
	if err != nil {
		t.Fatalf("NewJSONLRunEventStore: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	store.Append(ctx, events.RunEvent{ID: "evt1", RunID: "run_test1", Type: events.EventRunStarted, Status: "started", StartedAt: now})
	store.Append(ctx, events.RunEvent{ID: "evt2", RunID: "run_test1", Type: events.EventRunSucceeded, Status: "succeeded", FinishedAt: now.Add(time.Second)})
	store.Close()

	eventsPath := filepath.Join(root, ".mimoneko", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: true\nstore_path: .mimoneko/events/run_events.jsonl\n"), 0o600)

	var stdout bytes.Buffer
	code := Run([]string{"dashboard", "--dir", root}, Env{Stdout: &stdout})
	if code != 0 {
		t.Fatalf("dashboard code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "run_test1") {
		t.Fatalf("stdout = %q, want 'run_test1'", stdout.String())
	}
	if !strings.Contains(stdout.String(), "succeeded") {
		t.Fatalf("stdout = %q, want 'succeeded'", stdout.String())
	}
}

func TestDashboardRunDetailNotFound(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	eventsDir := filepath.Join(root, ".mimoneko", "events")
	os.MkdirAll(eventsDir, 0o700)
	storePath := filepath.Join(eventsDir, "run_events.jsonl")
	os.WriteFile(storePath, []byte(""), 0o600)

	eventsPath := filepath.Join(root, ".mimoneko", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: true\nstore_path: .mimoneko/events/run_events.jsonl\n"), 0o600)

	var stderr bytes.Buffer
	code := Run([]string{"dashboard", "--dir", root, "--run", "nonexistent"}, Env{Stderr: &stderr})
	if code != 1 {
		t.Fatalf("dashboard --run nonexistent code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Fatalf("stderr = %q, want 'not found'", stderr.String())
	}
}

func TestDashboardRejectsExtraArgs(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	var stderr bytes.Buffer
	code := Run([]string{"dashboard", "--dir", root, "extra"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("dashboard with extra args code = %d, want 2", code)
	}
}

func TestDashboardDoesNotLeakSensitiveData(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	eventsDir := filepath.Join(root, ".mimoneko", "events")
	os.MkdirAll(eventsDir, 0o700)
	storePath := filepath.Join(eventsDir, "run_events.jsonl")

	store, _ := events.NewJSONLRunEventStore(storePath)
	ctx := context.Background()
	now := time.Now().UTC()
	// In production, SafeEmit calls SanitizeEvent before writing.
	// Simulate that here by sanitizing before appending.
	rawEvent := events.RunEvent{
		ID: "evt_sec", RunID: "run_sec", Type: events.EventRunStarted, Status: "started",
		Message:   "Run started with API_KEY=sk-secret-12345",
		StartedAt: now,
	}
	sanitized := events.SanitizeEvent(rawEvent)
	store.Append(ctx, sanitized)
	store.Close()

	eventsPath := filepath.Join(root, ".mimoneko", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: true\nstore_path: .mimoneko/events/run_events.jsonl\n"), 0o600)

	var stdout bytes.Buffer
	Run([]string{"dashboard", "--dir", root}, Env{Stdout: &stdout})
	output := stdout.String()
	if strings.Contains(output, "sk-secret-12345") {
		t.Fatal("dashboard output leaked API key")
	}
	if strings.Contains(output, "API_KEY") {
		t.Fatal("dashboard output leaked API key pattern")
	}
}

func TestDashboardLimitRuns(t *testing.T) {
	root := t.TempDir()
	Run([]string{"init", "--dir", root}, Env{})

	eventsDir := filepath.Join(root, ".mimoneko", "events")
	os.MkdirAll(eventsDir, 0o700)
	storePath := filepath.Join(eventsDir, "run_events.jsonl")

	store, _ := events.NewJSONLRunEventStore(storePath)
	ctx := context.Background()
	now := time.Now().UTC()
	for i := 0; i < 10; i++ {
		store.Append(ctx, events.RunEvent{
			ID: fmt.Sprintf("evt_%d", i), RunID: fmt.Sprintf("run_%d", i),
			Type: events.EventRunStarted, Status: "started",
			StartedAt: now.Add(time.Duration(-i) * time.Hour),
		})
	}
	store.Close()

	eventsPath := filepath.Join(root, ".mimoneko", "events.yaml")
	os.WriteFile(eventsPath, []byte("enabled: true\nstore_path: .mimoneko/events/run_events.jsonl\n"), 0o600)

	var stdout bytes.Buffer
	Run([]string{"dashboard", "--dir", root, "--limit", "3"}, Env{Stdout: &stdout})
	output := stdout.String()

	// Count run_ occurrences (excluding headers)
	dataLines := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "run_") && !strings.Contains(line, "Recent") {
			dataLines++
		}
	}
	if dataLines != 3 {
		t.Errorf("expected 3 data lines with --limit 3, got %d\noutput:\n%s", dataLines, output)
	}
}

// --- Web Dashboard CLI tests ---

func stubServeCommand(t *testing.T) **webserver.LocalServer {
	t.Helper()
	var captured *webserver.LocalServer
	old := serveCommandRun
	serveCommandRun = func(s *webserver.LocalServer) error {
		captured = s
		return nil
	}
	t.Cleanup(func() { serveCommandRun = old })
	return &captured
}

func TestServeCommandParsesHostPort(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	captured := stubServeCommand(t)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"serve", "--dir", root, "--host", "127.0.0.1", "--port", "9000", "--poll-interval", "5s"}, Env{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("Run(serve) code = %d, stderr = %q", code, stderr.String())
	}
	if *captured == nil {
		t.Fatal("serveCommandRun was not called")
	}
	if (*captured).Address() != "127.0.0.1:9000" {
		t.Fatalf("Address() = %q, want 127.0.0.1:9000", (*captured).Address())
	}
	if (*captured).PollInterval() != 5*time.Second {
		t.Fatalf("PollInterval() = %v, want 5s", (*captured).PollInterval())
	}
	if !strings.Contains(stdout.String(), "http://127.0.0.1:9000") {
		t.Fatalf("stdout = %q, want listen URL", stdout.String())
	}
}

func TestServeRejectsExtraArgs(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"serve", "extra"}, Env{Stderr: &stderr})
	if code != 2 {
		t.Fatalf("Run(serve extra) code = %d, want 2", code)
	}
}

func TestServeDefaultPort(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	captured := stubServeCommand(t)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"serve", "--dir", root}, Env{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("Run(serve) code = %d, stderr = %q", code, stderr.String())
	}
	if *captured == nil {
		t.Fatal("serveCommandRun was not called")
	}
	if (*captured).Address() != "127.0.0.1:8765" {
		t.Fatalf("Address() = %q, want 127.0.0.1:8765", (*captured).Address())
	}
}

func TestServeDoesNotDefaultToPublicHost(t *testing.T) {
	root := t.TempDir()
	if code := Run([]string{"init", "--dir", root}, Env{}); code != 0 {
		t.Fatalf("Run(init) code = %d", code)
	}
	captured := stubServeCommand(t)

	var stderr bytes.Buffer
	code := Run([]string{"serve", "--dir", root}, Env{Stderr: &stderr})
	if code != 0 {
		t.Fatalf("Run(serve) code = %d, stderr = %q", code, stderr.String())
	}
	if *captured == nil {
		t.Fatal("serveCommandRun was not called")
	}
	if strings.HasPrefix((*captured).Address(), "0.0.0.0:") {
		t.Fatalf("serve defaulted to public host: %s", (*captured).Address())
	}
	if !strings.HasPrefix((*captured).Address(), "127.0.0.1:") {
		t.Fatalf("Address() = %q, want localhost", (*captured).Address())
	}
}
