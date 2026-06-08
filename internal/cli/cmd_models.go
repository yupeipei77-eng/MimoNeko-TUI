package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
)

type ModelsCommand struct{}

func (c *ModelsCommand) Name() string { return "models" }

func (c *ModelsCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("models", flag.ContinueOnError)
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

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "models failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(env.Stdout, "MimoNeko Models")
	fmt.Fprintf(env.Stdout, "default_model=%s\n", cfg.Models.Routing.DefaultModel)
	fmt.Fprintln(env.Stdout)

	for _, provider := range cfg.Models.Providers {
		fmt.Fprintf(env.Stdout, "provider=%s\n", provider.Name)
		fmt.Fprintf(env.Stdout, "type=%s\n", provider.Type)
		fmt.Fprintf(env.Stdout, "base_url=%s\n", provider.BaseURL)
		fmt.Fprintf(env.Stdout, "api_key_env=%s\n", provider.APIKeyEnv)
		fmt.Fprintf(env.Stdout, "api_key_status=%s\n", apiKeyStatus(provider.APIKeyEnv))

		modelNames := make([]string, 0, len(provider.Models))
		for _, m := range provider.Models {
			modelNames = append(modelNames, m.Name)
		}
		fmt.Fprintf(env.Stdout, "models=%s\n", strings.Join(modelNames, ","))
		fmt.Fprintln(env.Stdout)
	}

	if len(cfg.Models.Routing.FallbackChain) > 0 {
		fmt.Fprintln(env.Stdout, "fallback_chain:")
		for i, entry := range cfg.Models.Routing.FallbackChain {
			fmt.Fprintf(env.Stdout, "%d. %s/%s\n", i+1, entry.Provider, entry.Model)
		}
	} else {
		fmt.Fprintln(env.Stdout, "fallback_chain:")
		fmt.Fprintf(env.Stdout, "1. %s/%s\n", findProviderForDefaultModel(cfg), cfg.Models.Routing.DefaultModel)
	}

	return 0
}

func init() {
	commands.Register(&ModelsCommand{})
}
