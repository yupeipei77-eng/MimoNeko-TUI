package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitAndLoadDefaultConfig(t *testing.T) {
	root := t.TempDir()

	written, err := Init(root)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if len(written) != len(defaultConfigFiles) {
		t.Fatalf("Init() wrote %d files, want %d", len(written), len(defaultConfigFiles))
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Models.Routing.DefaultModel != "local-coder" {
		t.Fatalf("default model = %q", cfg.Models.Routing.DefaultModel)
	}
	if len(cfg.Prefix.ImmutableSources) != 3 {
		t.Fatalf("immutable source count = %d", len(cfg.Prefix.ImmutableSources))
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

	updated := strings.Replace(string(content), "default_model: local-coder", "default_model: missing-model", 1)
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
		name string
		mutate func(*Root)
		want string
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
		name string
		mutate func(*Root)
		want string
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
				RegistryPath: ".reasonforge/cache/prefixes.jsonl",
			},
			Budget: BudgetConfig{
				WarnRatio:  0.8,
				BlockRatio: 1.0,
			},
		},
	}
}
