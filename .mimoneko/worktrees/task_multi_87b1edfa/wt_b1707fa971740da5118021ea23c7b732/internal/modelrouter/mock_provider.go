package modelrouter

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MockProvider is a test-only Provider implementation.
// It never contacts real networks and supports configurable responses,
// errors, delays, and model support filtering.
type MockProvider struct {
	name         string
	supported    map[string]bool
	text         string
	usage        Usage
	err          error
	delay        time.Duration
	callCount    int
	lastRequest  *CompletionRequest
	requestIDGen func() string
}

// NewMockProvider creates a MockProvider with the given name.
// By default, it supports all models.
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name:      name,
		supported: nil, // nil means all models supported
		text:      "mock response",
		usage: Usage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
			CachedTokens: 0,
			Estimated:    true,
		},
		requestIDGen: func() string {
			return fmt.Sprintf("mock-%d", time.Now().UnixNano())
		},
	}
}

// WithText sets the response text.
func (m *MockProvider) WithText(text string) *MockProvider {
	m.text = text
	return m
}

// WithUsage sets the response usage.
func (m *MockProvider) WithUsage(usage Usage) *MockProvider {
	m.usage = usage
	return m
}

// WithError sets an error to return instead of a response.
func (m *MockProvider) WithError(err error) *MockProvider {
	m.err = err
	return m
}

// WithDelay sets a simulated delay before responding.
func (m *MockProvider) WithDelay(d time.Duration) *MockProvider {
	m.delay = d
	return m
}

// WithSupportedModels restricts which models this provider supports.
// If never called, all models are supported.
func (m *MockProvider) WithSupportedModels(models []string) *MockProvider {
	m.supported = make(map[string]bool, len(models))
	for _, model := range models {
		m.supported[model] = true
	}
	return m
}

// WithRequestIDGen sets a custom request ID generator for testing.
func (m *MockProvider) WithRequestIDGen(fn func() string) *MockProvider {
	m.requestIDGen = fn
	return m
}

// Complete returns the configured response or error.
func (m *MockProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	m.callCount++
	m.lastRequest = &req

	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return CompletionResponse{}, ctx.Err()
		case <-time.After(m.delay):
		}
	}

	if err := ctx.Err(); err != nil {
		return CompletionResponse{}, err
	}

	if m.err != nil {
		return CompletionResponse{}, m.err
	}

	return CompletionResponse{
		Provider:  m.name,
		Model:     req.Model,
		Text:      m.text,
		Usage:     m.usage,
		RequestID: m.requestIDGen(),
	}, nil
}

// Name returns the provider name.
func (m *MockProvider) Name() string {
	return m.name
}

// Supports returns whether this provider can serve the given model.
// If no model filter is set, all models are supported.
func (m *MockProvider) Supports(model string) bool {
	if m.supported == nil {
		return true
	}
	return m.supported[model]
}

// CallCount returns how many times Complete has been called.
func (m *MockProvider) CallCount() int {
	return m.callCount
}

// LastRequest returns the most recent request, or nil if none.
func (m *MockProvider) LastRequest() *CompletionRequest {
	return m.lastRequest
}

// FallbackError is an aggregated error from a failed fallback chain.
// It contains information about which provider/model pairs were tried
// and why each failed, but never includes API keys or request bodies.
type FallbackError struct {
	Attempts []FallbackAttempt
}

// FallbackAttempt records a single attempt in the fallback chain.
type FallbackAttempt struct {
	Provider string
	Model    string
	Err      error
}

func (e *FallbackError) Error() string {
	var b strings.Builder
	b.WriteString("all providers failed in fallback chain:")
	for _, a := range e.Attempts {
		b.WriteString(fmt.Sprintf(" %s/%s: %v;", a.Provider, a.Model, a.Err))
	}
	return b.String()
}
