package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DirName = ".reasonforge"

type Root struct {
	Dir      string
	Models   ModelsConfig
	Tools    ToolsConfig
	Security SecurityConfig
	Prefix   PrefixConfig
}

type ModelsConfig struct {
	Providers []ProviderConfig `yaml:"providers"`
	Routing   RoutingConfig    `yaml:"routing"`
}

type ProviderConfig struct {
	Name      string        `yaml:"name"`
	Type      string        `yaml:"type"`
	BaseURL   string        `yaml:"base_url"`
	APIKeyEnv string        `yaml:"api_key_env"`
	Models    []ModelConfig `yaml:"models"`
}

type ModelConfig struct {
	Name                string `yaml:"name"`
	Purpose             string `yaml:"purpose"`
	MaxOutputTokens     int    `yaml:"max_output_tokens"`
	SupportsPrefixCache bool   `yaml:"supports_prefix_cache"`
}

type RoutingConfig struct {
	DefaultModel string `yaml:"default_model"`
}

type ToolsConfig struct {
	Tools []ToolConfig `yaml:"tools"`
}

type ToolConfig struct {
	Name    string   `yaml:"name"`
	Kind    string   `yaml:"kind"`
	Enabled bool     `yaml:"enabled"`
	Command []string `yaml:"command"`
}

type SecurityConfig struct {
	Sandbox SandboxConfig `yaml:"sandbox"`
	Network NetworkConfig `yaml:"network"`
	Secrets SecretsConfig `yaml:"secrets"`
}

type SandboxConfig struct {
	DefaultMode string `yaml:"default_mode"`
}

type NetworkConfig struct {
	EnabledByDefault bool `yaml:"enabled_by_default"`
}

type SecretsConfig struct {
	AllowEnvPrefixes []string `yaml:"allow_env_prefixes"`
}

type PrefixConfig struct {
	Version          int                  `yaml:"version"`
	ImmutableSources []PrefixSourceConfig `yaml:"immutable_sources"`
	ByteStable      ByteStableConfig     `yaml:"byte_stable"`
	Cache           PrefixCacheConfig    `yaml:"cache"`
	Budget          BudgetConfig         `yaml:"budget"`
}

type PrefixSourceConfig struct {
	Name     string `yaml:"name"`
	Kind     string `yaml:"kind"`
	Path     string `yaml:"path"`
	Required bool   `yaml:"required"`
}

type ByteStableConfig struct {
	NormalizeLineEndings    string `yaml:"normalize_line_endings"`
	SortToolSchemas         bool   `yaml:"sort_tool_schemas"`
	DisallowDynamicContent  bool   `yaml:"disallow_dynamic_content"`
}

type PrefixCacheConfig struct {
	RegistryPath string `yaml:"registry_path"`
}

type BudgetConfig struct {
	WarnRatio  float64 `yaml:"warn_ratio"`
	BlockRatio float64 `yaml:"block_ratio"`
}

func ConfigDir(root string) string {
	return filepath.Join(root, DirName)
}

func Init(root string) ([]string, error) {
	dir := ConfigDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	written := make([]string, 0, len(defaultConfigFiles))
	for _, file := range defaultConfigFiles {
		path := filepath.Join(dir, file.Name)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return written, fmt.Errorf("stat %s: %w", path, err)
		}

		if err := os.WriteFile(path, []byte(file.Body), 0o600); err != nil {
			return written, fmt.Errorf("write %s: %w", path, err)
		}
		written = append(written, path)
	}

	return written, nil
}

func Load(root string) (*Root, error) {
	dir := ConfigDir(root)
	cfg := &Root{Dir: dir}

	if err := loadYAML(filepath.Join(dir, "models.yaml"), &cfg.Models); err != nil {
		return nil, err
	}
	if err := loadYAML(filepath.Join(dir, "tools.yaml"), &cfg.Tools); err != nil {
		return nil, err
	}
	if err := loadYAML(filepath.Join(dir, "security.yaml"), &cfg.Security); err != nil {
		return nil, err
	}
	if err := loadYAML(filepath.Join(dir, "prefix.yaml"), &cfg.Prefix); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cfg *Root) Validate() error {
	var issues []string

	if len(cfg.Models.Providers) == 0 {
		issues = append(issues, "models.yaml must define at least one provider")
	}
	if cfg.Models.Routing.DefaultModel == "" {
		issues = append(issues, "models.yaml routing.default_model is required")
	}
	defaultModelFound := false
	for _, provider := range cfg.Models.Providers {
		if strings.TrimSpace(provider.Name) == "" {
			issues = append(issues, "models.yaml provider name is required")
		}
		if provider.Type != "openai-compatible" {
			issues = append(issues, fmt.Sprintf("models.yaml provider %q must be openai-compatible", provider.Name))
		}
		if strings.TrimSpace(provider.BaseURL) == "" {
			issues = append(issues, fmt.Sprintf("models.yaml provider %q base_url is required", provider.Name))
		}
		if len(provider.Models) == 0 {
			issues = append(issues, fmt.Sprintf("models.yaml provider %q must define at least one model", provider.Name))
		}
		for _, model := range provider.Models {
			if model.Name == cfg.Models.Routing.DefaultModel {
				defaultModelFound = true
			}
		}
	}
	if cfg.Models.Routing.DefaultModel != "" && len(cfg.Models.Providers) > 0 && !defaultModelFound {
		issues = append(issues, fmt.Sprintf("models.yaml default model %q is not defined by any provider", cfg.Models.Routing.DefaultModel))
	}
	if cfg.Security.Sandbox.DefaultMode == "" {
		issues = append(issues, "security.yaml sandbox.default_mode is required")
	}
	if cfg.Prefix.Version <= 0 {
		issues = append(issues, "prefix.yaml version must be positive")
	}
	if !cfg.Prefix.ByteStable.DisallowDynamicContent {
		issues = append(issues, "prefix.yaml byte_stable.disallow_dynamic_content must be true")
	}
	if cfg.Prefix.ByteStable.NormalizeLineEndings != "lf" {
		issues = append(issues, "prefix.yaml byte_stable.normalize_line_endings must be lf")
	}
	if !cfg.Prefix.ByteStable.SortToolSchemas {
		issues = append(issues, "prefix.yaml byte_stable.sort_tool_schemas must be true")
	}
	if strings.TrimSpace(cfg.Prefix.Cache.RegistryPath) == "" {
		issues = append(issues, "prefix.yaml cache.registry_path is required")
	}
	if cfg.Prefix.Budget.WarnRatio <= 0 {
		issues = append(issues, "prefix.yaml budget.warn_ratio must be positive")
	}
	if cfg.Prefix.Budget.BlockRatio <= cfg.Prefix.Budget.WarnRatio {
		issues = append(issues, "prefix.yaml budget.block_ratio must be greater than warn_ratio")
	}

	for _, source := range cfg.Prefix.ImmutableSources {
		if strings.TrimSpace(source.Name) == "" {
			issues = append(issues, "prefix.yaml immutable source name is required")
		}
		if strings.TrimSpace(source.Path) == "" {
			issues = append(issues, fmt.Sprintf("prefix.yaml immutable source %q path is required", source.Name))
		}
		if !isAllowedImmutableSourceKind(source.Kind) {
			issues = append(issues, fmt.Sprintf("prefix.yaml immutable source %q has unsupported kind %q", source.Name, source.Kind))
		}
	}

	if len(issues) > 0 {
		return errors.New(strings.Join(issues, "; "))
	}
	return nil
}

func loadYAML(path string, out any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	return nil
}

func isAllowedImmutableSourceKind(kind string) bool {
	switch kind {
	case "static_file", "generated_schema":
		return true
	default:
		return false
	}
}

type defaultConfigFile struct {
	Name string
	Body string
}

var defaultConfigFiles = []defaultConfigFile{
	{
		Name: "models.yaml",
		Body: `providers:
  - name: local-openai-compatible
    type: openai-compatible
    base_url: http://127.0.0.1:11434/v1
    api_key_env: REASONFORGE_API_KEY
    models:
      - name: local-coder
        purpose: coding
        max_output_tokens: 4096
        supports_prefix_cache: false
routing:
  default_model: local-coder
`,
	},
	{
		Name: "tools.yaml",
		Body: `tools:
  - name: shell
    kind: process
    enabled: false
    command: []
`,
	},
	{
		Name: "security.yaml",
		Body: `sandbox:
  default_mode: workspace-write
network:
  enabled_by_default: false
secrets:
  allow_env_prefixes:
    - REASONFORGE_
`,
	},
	{
		Name: "prefix.yaml",
		Body: `version: 1
immutable_sources:
  - name: system_prompt
    kind: static_file
    path: prompts/system.md
    required: true
  - name: tool_schema
    kind: generated_schema
    path: schemas/tools.json
    required: true
  - name: coding_rules
    kind: static_file
    path: prompts/coding_rules.md
    required: true
byte_stable:
  normalize_line_endings: lf
  sort_tool_schemas: true
  disallow_dynamic_content: true
cache:
  registry_path: .reasonforge/cache/prefixes.jsonl
budget:
  warn_ratio: 0.8
  block_ratio: 1.0
`,
	},
}
