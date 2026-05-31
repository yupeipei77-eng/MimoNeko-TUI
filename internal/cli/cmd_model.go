package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/mimoneko/mimoneko/internal/modelprofile"
)

type ModelCommand struct{}

func (c *ModelCommand) Name() string { return "model" }

func (c *ModelCommand) Run(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "model requires a subcommand")
		fmt.Fprintln(env.Stderr, "Usage: mimoneko model <setup|list|discover|enrich|test|use|remove>")
		return 2
	}

	switch args[0] {
	case "setup":
		return c.runSetup(args[1:], env)
	case "list":
		return c.runList(args[1:], env)
	case "discover":
		return c.runDiscover(args[1:], env)
	case "enrich":
		return c.runEnrich(args[1:], env)
	case "test":
		return c.runTest(args[1:], env)
	case "use":
		return c.runUse(args[1:], env)
	case "remove":
		return c.runRemove(args[1:], env)
	default:
		fmt.Fprintf(env.Stderr, "unknown model subcommand %q (use: setup, list, discover, enrich, test, use, remove)\n", args[0])
		return 2
	}
}

func (c *ModelCommand) runSetup(args []string, env Env) int {
	fs := flag.NewFlagSet("model setup", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	preset := fs.String("preset", "", "provider preset")
	provider := fs.String("provider", "", "provider name")
	baseURL := fs.String("base-url", "", "OpenAI-compatible base URL")
	apiKeyEnv := fs.String("api-key-env", "", "API key environment variable")
	model := fs.String("model", "", "model name")
	purpose := fs.String("purpose", "", "model purpose")
	maxOutputTokens := fs.Int("max-output-tokens", 0, "maximum output tokens")
	supportsPrefixCache := fs.Bool("supports-prefix-cache", false, "model supports provider-side prefix cache")
	setDefault := fs.Bool("set-default", false, "set model as routing.default_model")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	opt := modelprofile.SetupOptions{
		Preset:              *preset,
		Provider:            *provider,
		BaseURL:             *baseURL,
		APIKeyEnv:           *apiKeyEnv,
		Model:               *model,
		Purpose:             *purpose,
		MaxOutputTokens:     *maxOutputTokens,
		SupportsPrefixCache: *supportsPrefixCache,
		SetDefault:          *setDefault,
	}
	if !hasModelSetupFields(args) {
		opt, err = promptModelSetupOptions(env)
		if err != nil {
			fmt.Fprintf(env.Stderr, "model setup failed: %v\n", err)
			return 1
		}
	}

	result, err := modelprofile.Setup(root, opt)
	if err != nil {
		fmt.Fprintf(env.Stderr, "model setup failed: %s\n", modelprofile.SanitizeText(err.Error()))
		return 1
	}
	fmt.Fprintln(env.Stdout, "model provider configured")
	fmt.Fprintf(env.Stdout, "provider=%s\n", result.Provider)
	fmt.Fprintf(env.Stdout, "model=%s\n", result.Model)
	if opt.SetDefault {
		fmt.Fprintf(env.Stdout, "default_model=%s\n", result.Model)
	}
	for _, hint := range result.Hints {
		fmt.Fprintln(env.Stdout, hint)
	}
	return 0
}

func (c *ModelCommand) runList(args []string, env Env) int {
	fs := flag.NewFlagSet("model list", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}
	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	models, err := modelprofile.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "model list failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(env.Stdout, "MimoNeko Model Profiles")
	fmt.Fprintf(env.Stdout, "default_model=%s\n", models.Routing.DefaultModel)
	fmt.Fprintln(env.Stdout)
	for _, provider := range models.Providers {
		fmt.Fprintf(env.Stdout, "provider=%s\n", provider.Name)
		fmt.Fprintf(env.Stdout, "type=%s\n", provider.Type)
		fmt.Fprintf(env.Stdout, "base_url=%s\n", provider.BaseURL)
		fmt.Fprintf(env.Stdout, "api_key_env=%s\n", provider.APIKeyEnv)
		fmt.Fprintf(env.Stdout, "api_key_status=%s\n", modelprofile.APIKeyStatus(provider.APIKeyEnv))
		fmt.Fprintln(env.Stdout, "models:")
		for _, model := range provider.Models {
			fmt.Fprintf(env.Stdout, "- %s purpose=%s max_output_tokens=%d supports_prefix_cache=%v",
				model.Name, model.Purpose, model.MaxOutputTokens, model.SupportsPrefixCache)
			if model.MaxContextTokens > 0 {
				fmt.Fprintf(env.Stdout, " max_context_tokens=%d", model.MaxContextTokens)
			}
			if model.ReasoningLevel != "" {
				fmt.Fprintf(env.Stdout, " reasoning_level=%s", model.ReasoningLevel)
			}
			if model.CapabilitySource != "" {
				fmt.Fprintf(env.Stdout, " capability_source=%s", model.CapabilitySource)
			}
			if model.Pricing != nil {
				fmt.Fprintf(env.Stdout, " pricing=%s", safePricingStatus(model.Pricing))
			}
			fmt.Fprintln(env.Stdout)
		}
		fmt.Fprintln(env.Stdout)
	}
	return 0
}

func (c *ModelCommand) runDiscover(args []string, env Env) int {
	fs := flag.NewFlagSet("model discover", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	provider := fs.String("provider", "", "provider name")
	baseURL := fs.String("base-url", "", "OpenAI-compatible base URL")
	apiKeyEnv := fs.String("api-key-env", "", "API key environment variable")
	writeCapabilities := fs.Bool("write-capabilities", false, "write known capability metadata for configured discovered models")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}
	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	models, err := modelprofile.Discover(context.Background(), root, modelprofile.DiscoverOptions{
		Provider:  *provider,
		BaseURL:   *baseURL,
		APIKeyEnv: *apiKeyEnv,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "model discover failed: %s\n", modelprofile.SanitizeText(err.Error()))
		return 1
	}
	fmt.Fprintln(env.Stdout, "Available models:")
	for _, model := range models {
		fmt.Fprintf(env.Stdout, "* %s\n", model)
	}
	if *writeCapabilities {
		result, err := modelprofile.EnrichDiscovered(root, *provider, models)
		if err != nil {
			fmt.Fprintf(env.Stderr, "model discover capability write failed: %s\n", modelprofile.SanitizeText(err.Error()))
			return 1
		}
		printEnrichResult(env.Stdout, result)
	}
	return 0
}

func (c *ModelCommand) runEnrich(args []string, env Env) int {
	fs := flag.NewFlagSet("model enrich", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	provider := fs.String("provider", "", "provider name")
	model := fs.String("model", "", "model name")
	all := fs.Bool("all", false, "enrich all configured models")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}
	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	result, err := modelprofile.Enrich(root, modelprofile.EnrichOptions{
		Provider: *provider,
		Model:    *model,
		All:      *all,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "model enrich failed: %s\n", modelprofile.SanitizeText(err.Error()))
		return 1
	}
	printEnrichResult(env.Stdout, result)
	return 0
}

func (c *ModelCommand) runTest(args []string, env Env) int {
	fs := flag.NewFlagSet("model test", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	provider := fs.String("provider", "", "provider name")
	model := fs.String("model", "", "model name")
	baseURL := fs.String("base-url", "", "OpenAI-compatible base URL")
	apiKeyEnv := fs.String("api-key-env", "", "API key environment variable")
	prompt := fs.String("prompt", "", "prompt to send for the smoke test")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}
	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	result, err := modelprofile.Test(context.Background(), root, modelprofile.TestOptions{
		Provider:  *provider,
		Model:     *model,
		BaseURL:   *baseURL,
		APIKeyEnv: *apiKeyEnv,
		Prompt:    *prompt,
	})
	if err != nil {
		fmt.Fprintf(env.Stderr, "model test failed: %s\n", modelprofile.SanitizeText(err.Error()))
		return 1
	}
	fmt.Fprintf(env.Stdout, "model=%s\n", result.Model)
	fmt.Fprintf(env.Stdout, "provider=%s\n", result.Provider)
	fmt.Fprintf(env.Stdout, "base_url=%s\n", result.BaseURL)
	fmt.Fprintf(env.Stdout, "api_key_env=%s\n", result.APIKeyEnv)
	fmt.Fprintf(env.Stdout, "api_key_status=%s\n", modelprofile.APIKeyStatus(result.APIKeyEnv))
	fmt.Fprintf(env.Stdout, "status=%s\n", result.Status)
	fmt.Fprintf(env.Stdout, "latency_ms=%d\n", result.LatencyMs)
	if result.Status != "ok" {
		fmt.Fprintf(env.Stdout, "error=%s\n", modelprofile.SanitizeText(result.Error))
		return 1
	}
	fmt.Fprintf(env.Stdout, "response=%s\n", result.Response)
	return 0
}

func (c *ModelCommand) runUse(args []string, env Env) int {
	fs := flag.NewFlagSet("model use", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(env.Stderr, "model use requires exactly one model name")
		return 2
	}
	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	model := fs.Arg(0)
	provider, err := modelprofile.Use(root, model)
	if err != nil {
		fmt.Fprintf(env.Stderr, "model use failed: %s\n", modelprofile.SanitizeText(err.Error()))
		return 1
	}
	fmt.Fprintf(env.Stdout, "default_model=%s\n", model)
	fmt.Fprintf(env.Stdout, "provider=%s\n", provider)
	return 0
}

func (c *ModelCommand) runRemove(args []string, env Env) int {
	fs := flag.NewFlagSet("model remove", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	provider := fs.String("provider", "", "provider name")
	model := fs.String("model", "", "model name")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}
	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	err = modelprofile.Remove(root, modelprofile.RemoveOptions{Provider: *provider, Model: *model})
	if err != nil {
		fmt.Fprintf(env.Stderr, "model remove failed: %s\n", modelprofile.SanitizeText(err.Error()))
		return 1
	}
	if strings.TrimSpace(*provider) != "" {
		fmt.Fprintf(env.Stdout, "removed_provider=%s\n", *provider)
	} else {
		fmt.Fprintf(env.Stdout, "removed_model=%s\n", *model)
	}
	return 0
}

func init() {
	commands.Register(&ModelCommand{})
}
