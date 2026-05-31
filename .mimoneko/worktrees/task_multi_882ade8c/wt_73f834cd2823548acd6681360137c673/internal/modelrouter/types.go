// Package modelrouter implements the Model Router layer for NekoMIMO.
//
// The Model Router is responsible for:
//   - Converting a ContextEngine Bundle into OpenAI-compatible messages.
//   - Selecting a provider and model via fallback chain.
//   - Calling the selected provider.
//   - Recording usage into the CacheRegistry.
//
// The Model Router does NOT:
//   - Read project files directly.
//   - Bypass ContextEngine.Bundle.
//   - Log or expose API keys.
package modelrouter

import (
	"context"

	"github.com/nekonomimo/nekonomimo/internal/contextengine"
)

// Provider is the interface that model providers must implement.
type Provider interface {
	// Complete sends a completion request to the model provider.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
	// Name returns the provider identifier (e.g. "deepseek", "local-openai-compatible").
	Name() string
	// Supports returns whether this provider can serve the given model.
	Supports(model string) bool
}

// ModelRouter selects a provider/model, calls it, and records usage.
type ModelRouter interface {
	// Complete resolves the provider/model, converts the Bundle, and calls the provider.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// CompletionRequest is the input to a model completion call.
// Bundle is the primary input; the ModelRouter must not reassemble context.
type CompletionRequest struct {
	Model           string
	Bundle          contextengine.Bundle
	MaxOutputTokens int
	Temperature     float64
	Metadata        map[string]string
}

// CompletionResponse is the output from a model completion call.
type CompletionResponse struct {
	Provider  string
	Model     string
	Text      string
	Usage     Usage
	RequestID string
}

// Usage tracks token consumption from a model call.
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CachedTokens int
	Estimated    bool
}
