package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirNameUsesCanonicalEnv(t *testing.T) {
	t.Setenv("MIMONEKO_CONFIG_DIR", ".mimo-canonical")
	if got := DirName(); got != ".mimo-canonical" {
		t.Fatalf("DirName() = %q, want canonical env value", got)
	}
}

func TestDirNameAcceptsLegacyAlias(t *testing.T) {
	t.Setenv("MIMONEKO_CONFIG_DIR", "")
	t.Setenv("MimoNeko_CONFIG_DIR", ".mimo-legacy")
	if got := DirName(); got != ".mimo-legacy" {
		t.Fatalf("DirName() = %q, want legacy alias value", got)
	}
}

func TestInitAndLoadDefaultConfig(t *testing.T) {
	root := t.TempDir()

	written, err := Init(root)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	wantWritten := len(getDefaultConfigFiles()) + len(defaultScaffoldFiles)
	if len(written) != wantWritten {
		t.Fatalf("Init() wrote %d files, want %d", len(written), wantWritten)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Models.Routing.DefaultModel != "mimo-v2.5-pro" {
		t.Fatalf("default model = %q", cfg.Models.Routing.DefaultModel)
	}
	if len(cfg.Prefix.ImmutableSources) != 3 {
		t.Fatalf("immutable source count = %d", len(cfg.Prefix.ImmutableSources))
	}
}

func TestInitCreatesPromptAndSchemaFiles(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	for _, rel := range []string{"prompts/system.md", "prompts/coding_rules.md", "schemas/tools.json"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected %s to exist after init: %v", rel, err)
		}
	}
}

func TestInitDoesNotOverwriteExistingPrompts(t *testing.T) {
	root := t.TempDir()
	customSystem := filepath.Join(root, "prompts", "system.md")
	if err := os.MkdirAll(filepath.Dir(customSystem), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(customSystem, []byte("custom system prompt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	got, err := os.ReadFile(customSystem)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "custom system prompt\n" {
		t.Fatalf("Init() overwrote existing system prompt: %q", string(got))
	}
}

func TestInitCreatesSystemPrompt(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "prompts", "system.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "You are MimoNeko") {
		t.Fatalf("system prompt = %q, want MimoNeko default", string(content))
	}
}

func TestInitCreatesCodingRules(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "prompts", "coding_rules.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Coding rules:") {
		t.Fatalf("coding rules = %q, want default heading", string(content))
	}
}

func TestInitCreatesToolsSchema(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "schemas", "tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"list_files", "file_read", "git_diff", "test_run"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("tools schema = %q, want %q", string(content), want)
		}
	}
}

func TestLoadAllowsEventsDisabled(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	eventsPath := filepath.Join(ConfigDir(root), "events.yaml")
	if err := os.WriteFile(eventsPath, []byte("enabled: false\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Events.Enabled {
		t.Fatal("Events.Enabled = true, want false")
	}
	if cfg.Events.StorePath == "" {
		t.Fatal("Events.StorePath should keep its default when omitted")
	}
	if !cfg.Events.EmitToolEvents {
		t.Fatal("Events.EmitToolEvents should keep its default when omitted")
	}
}

func TestLoadRejectsUnsupportedImmutablePrefixSourceKind(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	prefixPath := filepath.Join(ConfigDir(root), "prefix.yaml")
	content, err := os.ReadFile(prefixPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	updated := strings.Replace(string(content), "kind: static_file", "kind: memory", 1)
	if err := os.WriteFile(prefixPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want error")
	}
	if !strings.Contains(err.Error(), "unsupported kind") {
		t.Fatalf("Load() error = %v, want unsupported kind failure", err)
	}
}

func TestLoadAllowsStaticMemoryManagementRulesPath(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	prefixPath := filepath.Join(ConfigDir(root), "prefix.yaml")
	content, err := os.ReadFile(prefixPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	updated := strings.Replace(string(content), "path: prompts/system.md", "path: prompts/memory_management_rules.md", 1)
	if err := os.WriteFile(prefixPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Load(root); err != nil {
		t.Fatalf("Load() error = %v, want memory_management_rules.md to be allowed as a static file", err)
	}
}

func TestInitDoesNotOverwriteExistingFiles(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	modelsPath := filepath.Join(ConfigDir(root), "models.yaml")
	custom := []byte(`providers:
  - name: custom
    type: openai-compatible
    base_url: http://127.0.0.1:9999/v1
    api_key_env: ""
    models:
      - name: custom-model
        purpose: coding
        max_output_tokens: 2048
        supports_prefix_cache: true
routing:
  default_model: custom-model
`)
	if err := os.WriteFile(modelsPath, custom, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	written, err := Init(root)
	if err != nil {
		t.Fatalf("Init() second call error = %v", err)
	}
	if len(written) != 0 {
		t.Fatalf("Init() second call wrote %d files, want 0", len(written))
	}

	got, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(custom) {
		t.Fatalf("Init() overwrote existing models.yaml")
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load() with empty api_key_env error = %v", err)
	}
	if cfg.Models.Providers[0].APIKeyEnv != "" {
		t.Fatalf("api_key_env = %q, want empty string to remain allowed", cfg.Models.Providers[0].APIKeyEnv)
	}
}

func TestLoadRejectsMalformedYAML(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	modelsPath := filepath.Join(ConfigDir(root), "models.yaml")
	if err := os.WriteFile(modelsPath, []byte("providers:\n  - name: ["), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Load(root); err == nil {
		t.Fatal("Load() succeeded, want malformed YAML error")
	}
}

func TestLoadRejectsMissingDefaultModel(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	modelsPath := filepath.Join(ConfigDir(root), "models.yaml")
	content, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	updated := strings.Replace(string(content), "default_model: mimo-v2.5-pro", "default_model: missing-model", 1)
	if err := os.WriteFile(modelsPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want missing default model error")
	}
	if !strings.Contains(err.Error(), "is not defined by any provider") {
		t.Fatalf("Load() error = %v, want missing default model failure", err)
	}
}

func TestValidateRejectsInvalidByteStableSettings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Root)
		want   string
	}{
		{
			name: "line endings",
			mutate: func(cfg *Root) {
				cfg.Prefix.ByteStable.NormalizeLineEndings = "crlf"
			},
			want: "normalize_line_endings must be lf",
		},
		{
			name: "tool schema sorting",
			mutate: func(cfg *Root) {
				cfg.Prefix.ByteStable.SortToolSchemas = false
			},
			want: "sort_tool_schemas must be true",
		},
		{
			name: "dynamic content flag",
			mutate: func(cfg *Root) {
				cfg.Prefix.ByteStable.DisallowDynamicContent = false
			},
			want: "disallow_dynamic_content must be true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validRootConfig()
			tt.mutate(cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateRejectsEmptyImmutableSourceFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Root)
		want   string
	}{
		{
			name: "name",
			mutate: func(cfg *Root) {
				cfg.Prefix.ImmutableSources[0].Name = ""
			},
			want: "source name is required",
		},
		{
			name: "path",
			mutate: func(cfg *Root) {
				cfg.Prefix.ImmutableSources[0].Path = ""
			},
			want: "path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validRootConfig()
			tt.mutate(cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func validRootConfig() *Root {
	return &Root{
		Models: ModelsConfig{
			Providers: []ProviderConfig{
				{
					Name:    "local-openai-compatible",
					Type:    "openai-compatible",
					BaseURL: "http://127.0.0.1:11434/v1",
					Models: []ModelConfig{
						{Name: "local-coder"},
					},
				},
			},
			Routing: RoutingConfig{DefaultModel: "local-coder"},
		},
		Security: SecurityConfig{
			Sandbox: SandboxConfig{DefaultMode: "workspace-write"},
		},
		Prefix: PrefixConfig{
			Version: 1,
			ImmutableSources: []PrefixSourceConfig{
				{
					Name: "system_prompt",
					Kind: "static_file",
					Path: "prompts/system.md",
				},
			},
			ByteStable: ByteStableConfig{
				NormalizeLineEndings:   "lf",
				SortToolSchemas:        true,
				DisallowDynamicContent: true,
			},
			Cache: PrefixCacheConfig{
				RegistryPath: ".mimoneko/cache/prefixes.jsonl",
			},
			Budget: BudgetConfig{
				WarnRatio:  0.8,
				BlockRatio: 1.0,
			},
		},
	}
}

func TestLoadFallbackChain(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	modelsPath := filepath.Join(ConfigDir(root), "models.yaml")
	content := []byte(`providers:
  - name: deepseek
    type: openai-compatible
    base_url: https://api.deepseek.com/v1
    api_key_env: DEEPSEEK_API_KEY
    models:
      - name: deepseek-chat
        purpose: coding
        max_output_tokens: 4096
        supports_prefix_cache: true
  - name: local-openai-compatible
    type: openai-compatible
    base_url: http://127.0.0.1:11434/v1
    api_key_env: MimoNeko_API_KEY
    models:
      - name: local-coder
        purpose: coding
        max_output_tokens: 4096
        supports_prefix_cache: false
routing:
  default_model: deepseek-chat
  fallback_chain:
    - provider: deepseek
      model: deepseek-chat
    - provider: local-openai-compatible
      model: local-coder
`)
	if err := os.WriteFile(modelsPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Models.Routing.FallbackChain) != 2 {
		t.Fatalf("FallbackChain length = %d, want 2", len(cfg.Models.Routing.FallbackChain))
	}
	if cfg.Models.Routing.FallbackChain[0].Provider != "deepseek" {
		t.Errorf("FallbackChain[0].Provider = %q, want deepseek", cfg.Models.Routing.FallbackChain[0].Provider)
	}
	if cfg.Models.Routing.FallbackChain[0].Model != "deepseek-chat" {
		t.Errorf("FallbackChain[0].Model = %q, want deepseek-chat", cfg.Models.Routing.FallbackChain[0].Model)
	}
	if cfg.Models.Routing.FallbackChain[1].Provider != "local-openai-compatible" {
		t.Errorf("FallbackChain[1].Provider = %q, want local-openai-compatible", cfg.Models.Routing.FallbackChain[1].Provider)
	}
	if cfg.Models.Routing.FallbackChain[1].Model != "local-coder" {
		t.Errorf("FallbackChain[1].Model = %q, want local-coder", cfg.Models.Routing.FallbackChain[1].Model)
	}
}

func TestLoadRejectsInvalidFallbackChainProvider(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	modelsPath := filepath.Join(ConfigDir(root), "models.yaml")
	content := []byte(`providers:
  - name: local-openai-compatible
    type: openai-compatible
    base_url: http://127.0.0.1:11434/v1
    api_key_env: MimoNeko_API_KEY
    models:
      - name: local-coder
        purpose: coding
        max_output_tokens: 4096
        supports_prefix_cache: false
routing:
  default_model: local-coder
  fallback_chain:
    - provider: nonexistent
      model: local-coder
`)
	if err := os.WriteFile(modelsPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want error for invalid fallback_chain provider")
	}
	if !strings.Contains(err.Error(), "provider") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Load() error = %v, want provider not found failure", err)
	}
}

func TestLoadRejectsInvalidFallbackChainModel(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	modelsPath := filepath.Join(ConfigDir(root), "models.yaml")
	content := []byte(`providers:
  - name: local-openai-compatible
    type: openai-compatible
    base_url: http://127.0.0.1:11434/v1
    api_key_env: MimoNeko_API_KEY
    models:
      - name: local-coder
        purpose: coding
        max_output_tokens: 4096
        supports_prefix_cache: false
routing:
  default_model: local-coder
  fallback_chain:
    - provider: local-openai-compatible
      model: nonexistent-model
`)
	if err := os.WriteFile(modelsPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want error for invalid fallback_chain model")
	}
	if !strings.Contains(err.Error(), "not found in provider") {
		t.Fatalf("Load() error = %v, want model not found failure", err)
	}
}

func TestLoadDefaultConfigUsesMimoFallbackChain(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Models.Routing.FallbackChain) != 1 {
		t.Fatalf("FallbackChain length = %d, want 1", len(cfg.Models.Routing.FallbackChain))
	}
	if got := cfg.Models.Routing.FallbackChain[0]; got.Provider != "mimo" || got.Model != "mimo-v2.5-pro" {
		t.Fatalf("FallbackChain[0] = %+v, want mimo/mimo-v2.5-pro", got)
	}
}
