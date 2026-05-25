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
	Dir        string
	Models     ModelsConfig
	Tools      ToolsConfig
	Security   SecurityConfig
	Prefix     PrefixConfig
	Worktree   WorktreeConfig
	Patch      PatchConfig
	Review     ReviewConfig
	Validation ValidationConfig
	MultiAgent MultiAgentConfig
	Events     EventsConfig
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
	DefaultModel  string          `yaml:"default_model"`
	FallbackChain []FallbackEntry `yaml:"fallback_chain,omitempty"`
}

// FallbackEntry is a single entry in the routing fallback chain.
type FallbackEntry struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type ToolsConfig struct {
	Tools        []ToolConfig        `yaml:"tools"`
	TestCommands []TestCommandConfig `yaml:"test_commands,omitempty"`
	Policy       ToolPolicyConfig    `yaml:"policy,omitempty"`
}

type ToolConfig struct {
	Name      string   `yaml:"name"`
	Kind      string   `yaml:"kind"`
	Enabled   bool     `yaml:"enabled"`
	RiskLevel string   `yaml:"risk_level,omitempty"`
	Command   []string `yaml:"command,omitempty"`
}

type TestCommandConfig struct {
	Name           string   `yaml:"name"`
	Command        []string `yaml:"command"`
	TimeoutSeconds int      `yaml:"timeout_seconds,omitempty"`
}

type ToolPolicyConfig struct {
	MaxOutputBytes        int      `yaml:"max_output_bytes,omitempty"`
	DefaultTimeoutSeconds int      `yaml:"default_timeout_seconds,omitempty"`
	DenyWritePaths        []string `yaml:"deny_write_paths,omitempty"`
	DenyReadPaths         []string `yaml:"deny_read_paths,omitempty"`
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
	ByteStable       ByteStableConfig     `yaml:"byte_stable"`
	Cache            PrefixCacheConfig    `yaml:"cache"`
	Budget           BudgetConfig         `yaml:"budget"`
}

type PrefixSourceConfig struct {
	Name     string `yaml:"name"`
	Kind     string `yaml:"kind"`
	Path     string `yaml:"path"`
	Required bool   `yaml:"required"`
}

type ByteStableConfig struct {
	NormalizeLineEndings   string `yaml:"normalize_line_endings"`
	SortToolSchemas        bool   `yaml:"sort_tool_schemas"`
	DisallowDynamicContent bool   `yaml:"disallow_dynamic_content"`
}

type PrefixCacheConfig struct {
	RegistryPath string `yaml:"registry_path"`
	EstimatedTTL string `yaml:"estimated_ttl"`
}

type BudgetConfig struct {
	WarnRatio  float64 `yaml:"warn_ratio"`
	BlockRatio float64 `yaml:"block_ratio"`
}

// WorktreeConfig configures git worktree isolation behavior.
type WorktreeConfig struct {
	// Enabled controls whether worktree isolation is available.
	Enabled bool `yaml:"enabled"`

	// Root is the directory under the repo where worktrees are created.
	Root string `yaml:"root"`

	// BranchPrefix is the prefix for worktree branch names.
	BranchPrefix string `yaml:"branch_prefix"`

	// KeepFailed determines whether to keep failed worktrees for debugging.
	KeepFailed bool `yaml:"keep_failed"`

	// KeepCancelled determines whether to keep cancelled worktrees for debugging.
	KeepCancelled bool `yaml:"keep_cancelled"`

	// MaxActive is the maximum number of active worktrees.
	MaxActive int `yaml:"max_active"`
}

// PatchConfig configures patch preview and apply behavior.
type PatchConfig struct {
	// MaxDiffBytes is the maximum diff output size in bytes.
	MaxDiffBytes int `yaml:"max_diff_bytes"`

	// RequireCleanMain requires the main workspace to be clean before applying.
	RequireCleanMain bool `yaml:"require_clean_main"`

	// AllowBinary controls whether binary file changes are allowed in patches.
	AllowBinary bool `yaml:"allow_binary"`
}

// ReviewConfig configures patch review behavior (Phase 6).
type ReviewConfig struct {
	// MaxDiffBytes caps the diff output size for review.
	MaxDiffBytes int `yaml:"max_diff_bytes"`

	// HighRiskFileCount is the file count threshold for high risk.
	HighRiskFileCount int `yaml:"high_risk_file_count"`

	// MediumRiskFileCount is the file count threshold for medium risk.
	MediumRiskFileCount int `yaml:"medium_risk_file_count"`

	// HighRiskLineCount is the total additions+deletions threshold for high risk.
	HighRiskLineCount int `yaml:"high_risk_line_count"`

	// MediumRiskLineCount is the total additions+deletions threshold for medium risk.
	MediumRiskLineCount int `yaml:"medium_risk_line_count"`

	// RequireTestsForCodeChanges produces a warning when source code is modified
	// without corresponding test changes.
	RequireTestsForCodeChanges bool `yaml:"require_tests_for_code_changes"`

	// StrictModelReview causes the entire review to fail if the model review fails.
	StrictModelReview bool `yaml:"strict_model_review"`
}

// ValidationConfig configures test validation behavior (Phase 6).
type ValidationConfig struct {
	// DefaultTestCommands lists the default test command names.
	DefaultTestCommands []string `yaml:"default_test_commands"`

	// MaxOutputBytes caps the output per command.
	MaxOutputBytes int `yaml:"max_output_bytes"`

	// TimeoutSeconds caps the total validation duration.
	TimeoutSeconds int `yaml:"timeout_seconds"`
}

// MultiAgentConfig configures the multi-agent runtime behavior (Phase 7).
type MultiAgentConfig struct {
	// MaxIterations is the default maximum number of Planner->Coder->Reviewer iterations.
	MaxIterations int `yaml:"max_iterations"`

	// MaxAllowedIterations is the hard upper bound for iterations (max 5).
	MaxAllowedIterations int `yaml:"max_allowed_iterations"`

	// DefaultWorktree controls whether worktree isolation is used by default.
	DefaultWorktree bool `yaml:"default_worktree"`

	// DefaultDryRun controls whether dry-run mode is on by default.
	DefaultDryRun bool `yaml:"default_dry_run"`

	// PlannerModel specifies an optional model override for the planner agent.
	PlannerModel string `yaml:"planner_model"`

	// CoderModel specifies an optional model override for the coder agent.
	CoderModel string `yaml:"coder_model"`

	// ReviewerModel specifies an optional model override for the reviewer agent.
	ReviewerModel string `yaml:"reviewer_model"`

	// ReviewerUseModelReview controls whether the reviewer uses AI model review.
	ReviewerUseModelReview bool `yaml:"reviewer_use_model_review"`
}

// EventsConfig configures the event system behavior (Phase 8).
type EventsConfig struct {
	// Enabled controls whether the event system is active.
	Enabled bool `yaml:"enabled"`

	// StorePath is the path to the JSONL event store file.
	StorePath string `yaml:"store_path"`

	// MaxMessageBytes caps the size of event Message fields.
	MaxMessageBytes int `yaml:"max_message_bytes"`

	// MaxMetadataValueBytes caps the size of each metadata value.
	MaxMetadataValueBytes int `yaml:"max_metadata_value_bytes"`

	// EmitToolEvents controls whether tool.started/finished events are emitted.
	EmitToolEvents bool `yaml:"emit_tool_events"`

	// EmitModelEvents controls whether model call events are emitted.
	EmitModelEvents bool `yaml:"emit_model_events"`

	// EmitPatchEvents controls whether patch.preview.started/finished events are emitted.
	EmitPatchEvents bool `yaml:"emit_patch_events"`

	// EmitValidationEvents controls whether validation.started/finished events are emitted.
	EmitValidationEvents bool `yaml:"emit_validation_events"`
}

type InitResult struct {
	Created []string
	Skipped []string
}

type scaffoldFile struct {
	Path string
	Body string
	Mode os.FileMode
}

func ConfigDir(root string) string {
	return filepath.Join(root, DirName)
}

func Init(root string) ([]string, error) {
	result, err := InitDetailed(root)
	if err != nil {
		return result.Created, err
	}
	return result.Created, nil
}

func InitDetailed(root string) (InitResult, error) {
	dir := ConfigDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return InitResult{}, fmt.Errorf("create config dir: %w", err)
	}

	result := InitResult{
		Created: make([]string, 0, len(defaultConfigFiles)+len(defaultScaffoldFiles)),
		Skipped: make([]string, 0, len(defaultConfigFiles)+len(defaultScaffoldFiles)),
	}
	for _, file := range defaultConfigFiles {
		path := filepath.Join(dir, file.Name)
		if _, err := os.Stat(path); err == nil {
			result.Skipped = append(result.Skipped, path)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return result, fmt.Errorf("stat %s: %w", path, err)
		}

		if err := os.WriteFile(path, []byte(file.Body), 0o600); err != nil {
			return result, fmt.Errorf("write %s: %w", path, err)
		}
		result.Created = append(result.Created, path)
	}

	for _, file := range defaultScaffoldFiles {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if _, err := os.Stat(path); err == nil {
			result.Skipped = append(result.Skipped, path)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return result, fmt.Errorf("stat %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return result, fmt.Errorf("create scaffold dir: %w", err)
		}
		if err := os.WriteFile(path, []byte(file.Body), file.Mode); err != nil {
			return result, fmt.Errorf("write %s: %w", path, err)
		}
		result.Created = append(result.Created, path)
	}

	return result, nil
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

	// worktree.yaml and patch.yaml are optional (Phase 5 addition).
	// If missing, apply safe defaults.
	if err := loadYAMLOptional(filepath.Join(dir, "worktree.yaml"), &cfg.Worktree); err != nil {
		return nil, err
	}
	if err := loadYAMLOptional(filepath.Join(dir, "patch.yaml"), &cfg.Patch); err != nil {
		return nil, err
	}
	// review.yaml and validation.yaml are optional (Phase 6 addition).
	if err := loadYAMLOptional(filepath.Join(dir, "review.yaml"), &cfg.Review); err != nil {
		return nil, err
	}
	if err := loadYAMLOptional(filepath.Join(dir, "validation.yaml"), &cfg.Validation); err != nil {
		return nil, err
	}
	// multiagent.yaml is optional (Phase 7 addition).
	if err := loadYAMLOptional(filepath.Join(dir, "multiagent.yaml"), &cfg.MultiAgent); err != nil {
		return nil, err
	}
	// events.yaml is optional (Phase 8 addition).
	cfg.Events = defaultEventsConfig()
	if err := loadYAMLOptional(filepath.Join(dir, "events.yaml"), &cfg.Events); err != nil {
		return nil, err
	}
	cfg.applyWorktreeDefaults()
	cfg.applyPatchDefaults()
	cfg.applyReviewDefaults()
	cfg.applyValidationDefaults()
	cfg.applyMultiAgentDefaults()
	cfg.applyEventsDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func MissingRequiredPrefixSources(root string, prefix PrefixConfig) ([]string, error) {
	var missing []string
	for _, source := range prefix.ImmutableSources {
		if !source.Required {
			continue
		}
		if source.Kind != "static_file" && source.Kind != "generated_schema" {
			continue
		}
		path, err := safePrefixSourcePath(root, source.Path)
		if err != nil {
			return missing, fmt.Errorf("prefix source %q: %w", source.Path, err)
		}
		if _, err := os.Stat(path); err == nil {
			continue
		} else if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, filepath.ToSlash(source.Path))
			continue
		} else {
			return missing, fmt.Errorf("stat prefix source %q: %w", source.Path, err)
		}
	}
	return missing, nil
}

func safePrefixSourcePath(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is absolute, must be relative", rel)
	}
	if len(rel) > 0 && rel[0] == '/' {
		return "", fmt.Errorf("path %q starts with /, must be relative", rel)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	joined := filepath.Join(absRoot, rel)
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if !strings.HasPrefix(absJoined, absRoot+string(os.PathSeparator)) && absJoined != absRoot {
		return "", fmt.Errorf("path %q escapes root %q", rel, absRoot)
	}
	return absJoined, nil
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

	// Validate fallback chain entries reference existing providers and models
	for i, entry := range cfg.Models.Routing.FallbackChain {
		providerFound := false
		for _, provider := range cfg.Models.Providers {
			if provider.Name == entry.Provider {
				providerFound = true
				modelFound := false
				for _, model := range provider.Models {
					if model.Name == entry.Model {
						modelFound = true
						break
					}
				}
				if !modelFound {
					issues = append(issues, fmt.Sprintf("models.yaml fallback_chain[%d]: model %q not found in provider %q", i, entry.Model, entry.Provider))
				}
				break
			}
		}
		if !providerFound {
			issues = append(issues, fmt.Sprintf("models.yaml fallback_chain[%d]: provider %q not found", i, entry.Provider))
		}
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

// loadYAMLOptional loads a YAML file if it exists. Missing files are not an error.
func loadYAMLOptional(path string, out any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // missing file is OK, defaults will be used
		}
		return fmt.Errorf("read %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	return nil
}

// applyWorktreeDefaults fills in safe defaults for WorktreeConfig.
func (cfg *Root) applyWorktreeDefaults() {
	// Default to enabled for Phase 5+
	if !cfg.Worktree.Enabled && cfg.Worktree.Root == "" && cfg.Worktree.BranchPrefix == "" {
		// Config was not set at all (empty struct from missing file), apply defaults
		cfg.Worktree.Enabled = true
	}
	if cfg.Worktree.Root == "" {
		cfg.Worktree.Root = ".reasonforge/worktrees"
	}
	if cfg.Worktree.BranchPrefix == "" {
		cfg.Worktree.BranchPrefix = "reasonforge"
	}
	if cfg.Worktree.MaxActive == 0 {
		cfg.Worktree.MaxActive = 10
	}
	// KeepFailed and KeepCancelled default to true (safe: preserve for debugging)
	// Only override if they are explicitly false AND the file existed
}

// applyPatchDefaults fills in safe defaults for PatchConfig.
func (cfg *Root) applyPatchDefaults() {
	if cfg.Patch.MaxDiffBytes == 0 {
		cfg.Patch.MaxDiffBytes = 131072 // 128KB
	}
	// RequireCleanMain defaults to true (safe: don't overwrite user changes)
	if !cfg.Patch.RequireCleanMain {
		cfg.Patch.RequireCleanMain = true
	}
	// AllowBinary defaults to false (safe: binary patches are risky)
}

// applyReviewDefaults fills in safe defaults for ReviewConfig.
func (cfg *Root) applyReviewDefaults() {
	if cfg.Review.MaxDiffBytes == 0 {
		cfg.Review.MaxDiffBytes = 131072
	}
	if cfg.Review.HighRiskFileCount == 0 {
		cfg.Review.HighRiskFileCount = 20
	}
	if cfg.Review.MediumRiskFileCount == 0 {
		cfg.Review.MediumRiskFileCount = 5
	}
	if cfg.Review.HighRiskLineCount == 0 {
		cfg.Review.HighRiskLineCount = 500
	}
	if cfg.Review.MediumRiskLineCount == 0 {
		cfg.Review.MediumRiskLineCount = 100
	}
	// RequireTestsForCodeChanges defaults to false
	// StrictModelReview defaults to false
}

// applyValidationDefaults fills in safe defaults for ValidationConfig.
func (cfg *Root) applyValidationDefaults() {
	if len(cfg.Validation.DefaultTestCommands) == 0 {
		cfg.Validation.DefaultTestCommands = []string{"go-test"}
	}
	if cfg.Validation.MaxOutputBytes == 0 {
		cfg.Validation.MaxOutputBytes = 65536
	}
	if cfg.Validation.TimeoutSeconds == 0 {
		cfg.Validation.TimeoutSeconds = 120
	}
}

// applyMultiAgentDefaults fills in safe defaults for MultiAgentConfig.
func (cfg *Root) applyMultiAgentDefaults() {
	if cfg.MultiAgent.MaxIterations == 0 {
		cfg.MultiAgent.MaxIterations = 2
	}
	if cfg.MultiAgent.MaxAllowedIterations == 0 {
		cfg.MultiAgent.MaxAllowedIterations = 5
	}
	// DefaultWorktree defaults to true (safe: always isolate)
	cfg.MultiAgent.DefaultWorktree = true
	// DefaultDryRun defaults to true (safe: no side effects by default)
	cfg.MultiAgent.DefaultDryRun = true
	// PlannerModel, CoderModel, ReviewerModel default to empty (use default model)
	// ReviewerUseModelReview defaults to false
}

// applyEventsDefaults fills in safe defaults for EventsConfig.
func (cfg *Root) applyEventsDefaults() {
	if cfg.Events.StorePath == "" {
		cfg.Events.StorePath = ".reasonforge/events/run_events.jsonl"
	}
	if cfg.Events.MaxMessageBytes == 0 {
		cfg.Events.MaxMessageBytes = 2048
	}
	if cfg.Events.MaxMetadataValueBytes == 0 {
		cfg.Events.MaxMetadataValueBytes = 512
	}
}

func defaultEventsConfig() EventsConfig {
	return EventsConfig{
		Enabled:               true,
		StorePath:             ".reasonforge/events/run_events.jsonl",
		MaxMessageBytes:       2048,
		MaxMetadataValueBytes: 512,
		EmitToolEvents:        true,
		EmitModelEvents:       true,
		EmitPatchEvents:       true,
		EmitValidationEvents:  true,
	}
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

var defaultScaffoldFiles = []scaffoldFile{
	{
		Path: "prompts/system.md",
		Body: `You are ReasonForge, a safe local coding agent.
Follow the user goal, keep changes minimal, and never expose secrets.
Do not automatically commit, push, or apply patches.
`,
		Mode: 0o644,
	},
	{
		Path: "prompts/coding_rules.md",
		Body: `Coding rules:

* Prefer minimal, focused changes.
* Respect dry-run and worktree isolation.
* Do not edit secrets or credential files.
* Do not auto-commit or auto-push.
* Explain results clearly.
`,
		Mode: 0o644,
	},
	{
		Path: "schemas/tools.json",
		Body: "[]\n",
		Mode: 0o644,
	},
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
  - name: file_read
    kind: builtin
    enabled: true
    risk_level: low
  - name: file_write
    kind: builtin
    enabled: true
    risk_level: medium
  - name: file_patch
    kind: builtin
    enabled: true
    risk_level: medium
  - name: git_diff
    kind: builtin
    enabled: true
    risk_level: low
  - name: test_run
    kind: builtin
    enabled: true
    risk_level: medium
test_commands:
  - name: go-test
    command: ["go", "test", "./..."]
    timeout_seconds: 120
policy:
  max_output_bytes: 65536
  default_timeout_seconds: 30
  deny_write_paths:
    - ".git"
    - ".reasonforge"
    - ".env"
    - "*.pem"
    - "*.key"
    - "id_rsa"
    - "id_ed25519"
  deny_read_paths:
    - ".git"
    - ".reasonforge"
    - ".env"
    - "*.pem"
    - "*.key"
    - "id_rsa"
    - "id_ed25519"
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
  estimated_ttl: 1h
budget:
  warn_ratio: 0.8
  block_ratio: 1.0
`,
	},
	{
		Name: "worktree.yaml",
		Body: `# Worktree isolation configuration
# Git worktrees provide isolated working directories for agent tasks.
enabled: true
root: .reasonforge/worktrees
branch_prefix: reasonforge
keep_failed: true
keep_cancelled: true
max_active: 10
`,
	},
	{
		Name: "patch.yaml",
		Body: `# Patch manager configuration
# Controls how diffs from worktrees are previewed and applied.
max_diff_bytes: 131072
require_clean_main: true
allow_binary: false
`,
	},
	{
		Name: "review.yaml",
		Body: `# Patch review configuration (Phase 6)
# Controls risk scoring and rule-based review thresholds.
max_diff_bytes: 131072
high_risk_file_count: 20
medium_risk_file_count: 5
high_risk_line_count: 500
medium_risk_line_count: 100
require_tests_for_code_changes: false
strict_model_review: false
`,
	},
	{
		Name: "validation.yaml",
		Body: `# Test validation configuration (Phase 6)
# Controls how test validation is executed during patch review.
default_test_commands:
  - go-test
max_output_bytes: 65536
timeout_seconds: 120
`,
	},
	{
		Name: "multiagent.yaml",
		Body: `# Multi-agent runtime configuration (Phase 7)
# Controls how Planner->Coder->Reviewer iterations work.
max_iterations: 2
max_allowed_iterations: 5
default_worktree: true
default_dry_run: true
planner_model: ""
coder_model: ""
reviewer_model: ""
reviewer_use_model_review: false
`,
	},
	{
		Name: "events.yaml",
		Body: `# Event system configuration (Phase 8)
# Controls structured event recording for run progress tracking.
enabled: true
store_path: .reasonforge/events/run_events.jsonl
max_message_bytes: 2048
max_metadata_value_bytes: 512
emit_tool_events: true
emit_model_events: true
emit_patch_events: true
emit_validation_events: true
`,
	},
}
