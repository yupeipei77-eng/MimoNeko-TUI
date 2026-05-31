package modelrouter

import (
	"context"
	"fmt"

	"github.com/nekonomimo/nekonomimo/internal/cache"
	"github.com/nekonomimo/nekonomimo/internal/config"
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
// Model selection semantics:
//
//	If req.Model is non-empty:
//	  - The requested model is used exactly; it is never silently replaced.
//	  - Only providers that Supports(req.Model) are attempted.
//	  - If fallback_chain contains providers that support req.Model, they are
//	    tried in fallback_chain order (but always with req.Model, not entry.Model).
//	  - If no fallback_chain entry supports req.Model, the providers map is
//	    scanned for a provider that Supports(req.Model).
//	  - If no provider supports req.Model, a clear error is returned.
//
//	If req.Model is empty:
//	  - The defaultModel is used.
//	  - If fallback_chain is configured, it is used as-is (entry.Model applies).
//	  - If fallback_chain is empty, it is derived from defaultModel.
func (r *DefaultModelRouter) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Track whether the caller explicitly specified a model.
	explicitModel := req.Model != ""

	// Resolve the effective model.
	model := req.Model
	if model == "" {
		model = r.defaultModel
	}
	req.Model = model

	// Build the attempt chain based on model selection semantics.
	var chain []FallbackEntry
	if explicitModel {
		chain = r.buildExplicitModelChain(model)
	} else {
		chain = r.buildDefaultModelChain(model)
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

// buildExplicitModelChain builds the attempt chain when the caller explicitly
// specified a model. The model must not be silently replaced.
//
// Strategy:
//  1. Scan fallback_chain for providers that Supports(model), keeping their
//     provider order but always using the requested model (not entry.Model).
//  2. If fallback_chain has no supporting provider, scan the providers map
//     for any provider that Supports(model).
func (r *DefaultModelRouter) buildExplicitModelChain(model string) []FallbackEntry {
	var chain []FallbackEntry

	// First: try fallback_chain providers that support this model
	for _, entry := range r.fallbackChain {
		provider, ok := r.providers[entry.Provider]
		if !ok {
			continue
		}
		if provider.Supports(model) {
			chain = append(chain, FallbackEntry{Provider: entry.Provider, Model: model})
		}
	}

	// If fallback_chain yielded supporting providers, use them
	if len(chain) > 0 {
		return chain
	}

	// Fallback: scan all providers for one that supports this model
	for name, provider := range r.providers {
		if provider.Supports(model) {
			chain = append(chain, FallbackEntry{Provider: name, Model: model})
		}
	}

	return chain
}

// buildDefaultModelChain builds the attempt chain when no model was explicitly
// specified. The defaultModel and fallback_chain entry models apply.
func (r *DefaultModelRouter) buildDefaultModelChain(model string) []FallbackEntry {
	if len(r.fallbackChain) > 0 {
		return r.fallbackChain
	}

	// Derive from providers map
	for name, p := range r.providers {
		if p.Supports(model) {
			return []FallbackEntry{{Provider: name, Model: model}}
		}
	}

	return nil
}
