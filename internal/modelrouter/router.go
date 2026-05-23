package modelrouter

import (
	"context"
	"fmt"

	"github.com/reasonforge/reasonforge/internal/cache"
	"github.com/reasonforge/reasonforge/internal/config"
)

// FallbackEntry is a single entry in the fallback chain configuration.
type FallbackEntry struct {
	Provider string
	Model    string
}

// DefaultModelRouter implements ModelRouter with fallback chain support.
//
// It resolves the target provider/model, iterates through the fallback chain
// on error, records usage to CacheRegistry, and never exposes API keys.
type DefaultModelRouter struct {
	providers      map[string]Provider     // keyed by provider name
	fallbackChain  []FallbackEntry
	defaultModel   string
	cacheRegistry  cache.CacheRegistry
}

// NewDefaultModelRouter creates a new router with the given providers and config.
// If fallbackChain is empty, it derives the chain from defaultModel's provider.
func NewDefaultModelRouter(
	providers map[string]Provider,
	fallbackChain []FallbackEntry,
	defaultModel string,
	cacheRegistry cache.CacheRegistry,
) *DefaultModelRouter {
	return &DefaultModelRouter{
		providers:     providers,
		fallbackChain: fallbackChain,
		defaultModel:  defaultModel,
		cacheRegistry: cacheRegistry,
	}
}

// BuildFallbackChainFromConfig builds a FallbackEntry list from config.
// If routing.fallback_chain is configured, it uses that.
// Otherwise, it finds the provider for the default_model and creates a single entry.
func BuildFallbackChainFromConfig(cfg *config.Root) ([]FallbackEntry, error) {
	if len(cfg.Models.Routing.FallbackChain) > 0 {
		chain := make([]FallbackEntry, 0, len(cfg.Models.Routing.FallbackChain))
		for _, entry := range cfg.Models.Routing.FallbackChain {
			chain = append(chain, FallbackEntry{
				Provider: entry.Provider,
				Model:    entry.Model,
			})
		}
		return chain, nil
	}

	// Derive from default_model
	defaultModel := cfg.Models.Routing.DefaultModel
	for _, provider := range cfg.Models.Providers {
		for _, model := range provider.Models {
			if model.Name == defaultModel {
				return []FallbackEntry{{Provider: provider.Name, Model: defaultModel}}, nil
			}
		}
	}

	return nil, fmt.Errorf("default model %q not found in any provider", defaultModel)
}

// Complete resolves the provider/model, converts the Bundle, calls the provider,
// and records usage to the CacheRegistry.
//
// If req.Model is set, it uses that model. Otherwise it uses the default_model.
// It iterates through the fallback chain on provider errors.
// If all providers fail, it returns a FallbackError.
func (r *DefaultModelRouter) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Determine which model to use
	model := req.Model
	if model == "" {
		model = r.defaultModel
	}
	req.Model = model

	// Build the attempt chain
	chain := r.fallbackChain
	if len(chain) == 0 {
		// Try to find the provider for the requested model
		for name, p := range r.providers {
			if p.Supports(model) {
				chain = []FallbackEntry{{Provider: name, Model: model}}
				break
			}
		}
	}

	if len(chain) == 0 {
		return CompletionResponse{}, fmt.Errorf("no provider available for model %q", model)
	}

	var attempts []FallbackAttempt
	for _, entry := range chain {
		provider, ok := r.providers[entry.Provider]
		if !ok {
			attempts = append(attempts, FallbackAttempt{
				Provider: entry.Provider,
				Model:    entry.Model,
				Err:      fmt.Errorf("provider not found"),
			})
			continue
		}

		// Use the model from the fallback entry if the request doesn't specify one,
		// or if the request model matches this entry.
		attemptReq := req
		attemptReq.Model = entry.Model

		resp, err := provider.Complete(ctx, attemptReq)
		if err != nil {
			attempts = append(attempts, FallbackAttempt{
				Provider: entry.Provider,
				Model:    entry.Model,
				Err:      err,
			})
			continue
		}

		// Record usage to cache registry
		if r.cacheRegistry != nil {
			observation := UsageToObservation(resp.Usage, req.Bundle, resp.Provider, resp.Model, resp.RequestID)
			_ = r.cacheRegistry.Record(ctx, observation) // best-effort; don't fail the request
		}

		return resp, nil
	}

	return CompletionResponse{}, &FallbackError{Attempts: attempts}
}
