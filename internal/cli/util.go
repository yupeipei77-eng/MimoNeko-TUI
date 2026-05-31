package cli

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mimoneko/mimoneko/internal/auth"
	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/events"
	"github.com/mimoneko/mimoneko/internal/modelprofile"
	"github.com/mimoneko/mimoneko/internal/pathutil"
	"github.com/mimoneko/mimoneko/internal/review"
)

func resolveRoot(dir string, env Env) (string, error) {
	if strings.TrimSpace(dir) != "" {
		return pathutil.AbsPath(dir), nil
	}
	root, err := env.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return pathutil.AbsPath(root), nil
}

func rejectExtraArgs(fs *flag.FlagSet, env Env) bool {
	if fs.NArg() == 0 {
		return false
	}
	fmt.Fprintf(env.Stderr, "%s accepts no positional arguments: %s\n", fs.Name(), strings.Join(fs.Args(), " "))
	return true
}

func ensureProjectConfigForRun(root string) error {
	modelsPath := filepath.Join(config.ConfigDir(root), "models.yaml")
	if _, err := os.Stat(modelsPath); err == nil {
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := config.InitDetailed(root); err != nil {
		return err
	}
	if models, ok := configuredModelsForRun(); ok {
		return modelprofile.Save(root, models)
	}
	return nil
}

func configuredModelsForRun() (config.ModelsConfig, bool) {
	if models, ok, err := auth.UserModelsConfig(); err == nil && ok {
		return models, true
	}
	for _, provider := range []string{"mimo", "openai", "local"} {
		envVar := auth.APIKeyEnv(provider)
		key := strings.TrimSpace(os.Getenv(envVar))
		if key == "" || pathutil.APIKeyLooksPlaceholder(key) {
			continue
		}
		model := auth.DefaultModel(provider)
		return config.ModelsConfig{
			Providers: []config.ProviderConfig{
				{
					Name:      provider,
					Type:      auth.ProviderType(provider),
					BaseURL:   auth.GetBaseURL(provider),
					APIKeyEnv: envVar,
					Models: []config.ModelConfig{
						{
							Name:                model,
							Purpose:             "coding",
							MaxOutputTokens:     4096,
							SupportsPrefixCache: false,
						},
					},
				},
			},
			Routing: config.RoutingConfig{
				DefaultModel: model,
				FallbackChain: []config.FallbackEntry{
					{Provider: provider, Model: model},
				},
			},
		}, true
	}
	return config.ModelsConfig{}, false
}

func generateShortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func captureCLI(run func(stdout, stderr io.Writer) int) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(&stdout, &stderr)
	output := stdout.String() + stderr.String()
	if code != 0 {
		return output, fmt.Errorf("command exited with code %d", code)
	}
	return output, nil
}

func extractCLIValue(output, key string) string {
	prefix := key + "="
	value := ""
	for _, field := range strings.Fields(output) {
		if strings.HasPrefix(field, prefix) {
			value = strings.Trim(strings.TrimPrefix(field, prefix), "\"")
		}
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "help" {
			return true
		}
	}
	return false
}

func apiKeyStatus(envVar string) string {
	return pathutil.APIKeyStatus(envVar)
}

func findProviderForDefaultModel(cfg *config.Root) string {
	for _, provider := range cfg.Models.Providers {
		for _, model := range provider.Models {
			if model.Name == cfg.Models.Routing.DefaultModel {
				return provider.Name
			}
		}
	}
	return "unknown"
}

func printEnrichResult(w io.Writer, result modelprofile.EnrichResult) {
	fmt.Fprintln(w, "capabilities:")
	for _, item := range result.Updated {
		fmt.Fprintf(w, "updated %s\n", item)
	}
	for _, item := range result.Skipped {
		fmt.Fprintf(w, "skipped %s\n", item)
	}
	if len(result.Updated) == 0 && len(result.Skipped) == 0 {
		fmt.Fprintln(w, "skipped none")
	}
}

func safePricingStatus(pricing *config.ModelPricingConfig) string {
	if pricing == nil {
		return "unavailable"
	}
	source := strings.TrimSpace(pricing.Source)
	if source == "" {
		source = modelprofile.PricingSourceUnknown
	}
	currency := strings.TrimSpace(pricing.Currency)
	if currency == "" {
		currency = "unknown"
	}
	return fmt.Sprintf("configured currency=%s source=%s", currency, source)
}

func printReviewReport(env Env, report review.PatchReviewReport, cfg *config.Root) {
	fmt.Fprintf(env.Stdout, "=== Patch Review Report ===\n")
	fmt.Fprintf(env.Stdout, "worktree_id=%s\n", report.WorktreeID)
	fmt.Fprintf(env.Stdout, "recommendation=%s\n", report.Recommendation)
	fmt.Fprintf(env.Stdout, "risk_level=%s risk_score=%d\n", report.RiskScore.Level, report.RiskScore.Score)
	if len(report.RiskScore.Reasons) > 0 {
		fmt.Fprintf(env.Stdout, "risk_reasons:\n")
		for _, r := range report.RiskScore.Reasons {
			fmt.Fprintf(env.Stdout, "  - %s\n", r)
		}
	}
	fmt.Fprintf(env.Stdout, "files_changed=%d additions=%d deletions=%d\n",
		report.Preview.Summary.FilesChanged,
		report.Preview.Summary.Additions,
		report.Preview.Summary.Deletions)
	fmt.Fprintf(env.Stdout, "has_binary=%v\n", report.Preview.Summary.HasBinary)
	if len(report.Preview.Violations) > 0 {
		fmt.Fprintf(env.Stdout, "violations=%d\n", len(report.Preview.Violations))
		for _, v := range report.Preview.Violations {
			fmt.Fprintf(env.Stdout, "  violation: path=%s reason=%s\n", v.Path, v.Reason)
		}
	}
	if len(report.Findings) > 0 {
		fmt.Fprintf(env.Stdout, "findings=%d\n", len(report.Findings))
		for _, f := range report.Findings {
			fmt.Fprintf(env.Stdout, "  finding: severity=%s category=%s path=%s message=%s\n",
				f.Severity, f.Category, f.Path, f.Message)
		}
	}
	if report.Validation != nil {
		fmt.Fprintf(env.Stdout, "validation_success=%v\n", report.Validation.Success)
		fmt.Fprintf(env.Stdout, "validation_summary=%s\n", report.Validation.Summary)
		for _, cmd := range report.Validation.Commands {
			fmt.Fprintf(env.Stdout, "  command: name=%s success=%v exit_code=%d duration_ms=%d\n",
				cmd.CommandName, cmd.Success, cmd.ExitCode, cmd.DurationMs)
		}
	} else if report.ValidationSkipped {
		fmt.Fprintln(env.Stdout, "validation_skipped=true")
		fmt.Fprintf(env.Stdout, "reason=%s\n", report.ValidationSkipReason)
	}
	if report.ModelReview != nil {
		fmt.Fprintf(env.Stdout, "model_review:\n")
		fmt.Fprintf(env.Stdout, "  provider=%s model=%s\n", report.ModelReview.Provider, report.ModelReview.Model)
		fmt.Fprintf(env.Stdout, "  summary=%s\n", report.ModelReview.Summary)
		fmt.Fprintf(env.Stdout, "  recommendation=%s\n", report.ModelReview.Recommendation)
		for _, f := range report.ModelReview.Findings {
			fmt.Fprintf(env.Stdout, "  finding: severity=%s category=%s message=%s\n",
				f.Severity, f.Category, f.Message)
		}
	}
	if report.Preview.Diff != "" && len(report.Preview.Violations) == 0 {
		fmt.Fprintln(env.Stdout, "--- diff ---")
		maxBytes := cfg.Patch.MaxDiffBytes
		if maxBytes <= 0 {
			maxBytes = 131072
		}
		diff := report.Preview.Diff
		if len(diff) > maxBytes {
			diff = diff[:maxBytes] + "\n... (truncated)"
		}
		fmt.Fprint(env.Stdout, diff)
	}
}

func hasModelSetupFields(args []string) bool {
	for _, arg := range args {
		name := strings.TrimLeft(arg, "-")
		if i := strings.IndexByte(name, '='); i >= 0 {
			name = name[:i]
		}
		switch name {
		case "preset", "provider", "base-url", "api-key-env", "model", "purpose", "max-output-tokens", "supports-prefix-cache", "set-default":
			return true
		}
	}
	return false
}

func promptModelSetupOptions(env Env) (modelprofile.SetupOptions, error) {
	scanner := bufio.NewScanner(env.Stdin)
	presetOrder := []string{"openai", "deepseek", "glm", "mimo", "custom-openai-compatible"}
	fmt.Fprintln(env.Stdout, "Select provider preset:")
	for i, name := range presetOrder {
		fmt.Fprintf(env.Stdout, "%d. %s\n", i+1, name)
	}
	selected, err := promptLine(scanner, env.Stdout, "preset", "mimo")
	if err != nil {
		return modelprofile.SetupOptions{}, err
	}
	if n := parsePresetNumber(selected, len(presetOrder)); n > 0 {
		selected = presetOrder[n-1]
	}
	preset, ok := modelprofile.GetPreset(selected)
	if !ok {
		return modelprofile.SetupOptions{}, fmt.Errorf("unknown provider preset %q", selected)
	}
	provider, err := promptLine(scanner, env.Stdout, "provider name", preset.Name)
	if err != nil {
		return modelprofile.SetupOptions{}, err
	}
	baseURL, err := promptLine(scanner, env.Stdout, "base_url", preset.BaseURL)
	if err != nil {
		return modelprofile.SetupOptions{}, err
	}
	apiKeyEnv, err := promptLine(scanner, env.Stdout, "api_key_env", preset.APIKeyEnv)
	if err != nil {
		return modelprofile.SetupOptions{}, err
	}
	defaultModel := ""
	if len(preset.SuggestedModels) > 0 {
		defaultModel = preset.SuggestedModels[0]
	}
	model, err := promptLine(scanner, env.Stdout, "model name", defaultModel)
	if err != nil {
		return modelprofile.SetupOptions{}, err
	}
	purpose, err := promptLine(scanner, env.Stdout, "purpose", "coding")
	if err != nil {
		return modelprofile.SetupOptions{}, err
	}
	maxTokensText, err := promptLine(scanner, env.Stdout, "max_output_tokens", "4096")
	if err != nil {
		return modelprofile.SetupOptions{}, err
	}
	maxTokens := 4096
	if strings.TrimSpace(maxTokensText) != "" {
		if _, scanErr := fmt.Sscanf(maxTokensText, "%d", &maxTokens); scanErr != nil {
			return modelprofile.SetupOptions{}, fmt.Errorf("invalid max_output_tokens %q", maxTokensText)
		}
	}
	prefixCacheText, err := promptLine(scanner, env.Stdout, "supports_prefix_cache", "false")
	if err != nil {
		return modelprofile.SetupOptions{}, err
	}
	setDefaultText, err := promptLine(scanner, env.Stdout, "set as default?", "yes")
	if err != nil {
		return modelprofile.SetupOptions{}, err
	}
	return modelprofile.SetupOptions{
		Preset:              selected,
		Provider:            provider,
		BaseURL:             baseURL,
		APIKeyEnv:           apiKeyEnv,
		Model:               model,
		Purpose:             purpose,
		MaxOutputTokens:     maxTokens,
		SupportsPrefixCache: parseYes(prefixCacheText),
		SetDefault:          parseYes(setDefaultText),
	}, nil
}

func promptLine(scanner *bufio.Scanner, w io.Writer, label, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(w, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(w, "%s: ", label)
	}
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		if defaultValue != "" {
			return defaultValue, nil
		}
		return "", errors.New("input ended before setup completed")
	}
	value := strings.TrimSpace(scanner.Text())
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func parsePresetNumber(value string, max int) int {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &n); err != nil {
		return 0
	}
	if n < 1 || n > max {
		return 0
	}
	return n
}

func parseYes(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "y", "yes", "true", "1":
		return true
	default:
		return false
	}
}

func mustGenerateCLIEventID() string {
	id, err := events.GenerateEventID()
	if err != nil {
		return "evt_error"
	}
	return id
}

func shouldAnimateNeko(w io.Writer, noColor bool) bool {
	if noColor {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
