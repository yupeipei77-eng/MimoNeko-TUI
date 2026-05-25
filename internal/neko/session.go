package neko

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/reasonforge/reasonforge/internal/config"
	"github.com/reasonforge/reasonforge/internal/modelprofile"
)

type Session struct {
	Root             string
	Mode             string
	Model            string
	Provider         string
	BaseURLHost      string
	APIKeyStatus     string
	MaxContextTokens int
	ContextUnknown   bool
	Reasoning        string
	ReasoningDefault bool
	DryRun           bool
	Worktree         bool
	NoColor          bool
	Usage            Usage
	Pricing          *config.ModelPricingConfig
}

func NewSession(root string, models config.ModelsConfig, opt Options) Session {
	mode := strings.ToLower(strings.TrimSpace(opt.Mode))
	if mode == "" {
		mode = "multi"
	}
	if mode != "single" && mode != "multi" {
		mode = "multi"
	}
	dryRun := true
	if opt.DryRunSet {
		dryRun = opt.DryRun
	}
	session := Session{
		Root:     root,
		Mode:     mode,
		DryRun:   dryRun,
		Worktree: mode == "multi",
		NoColor:  opt.NoColor,
		Usage:    Usage{Estimated: true},
	}

	provider, model := selectModel(models, opt.Model)
	session.Provider = provider.Name
	session.Model = model.Name
	session.BaseURLHost = hostOnly(provider.BaseURL)
	session.APIKeyStatus = modelprofile.APIKeyStatus(provider.APIKeyEnv)
	session.MaxContextTokens = model.MaxContextTokens
	session.Reasoning = strings.TrimSpace(model.ReasoningLevel)
	session.Pricing = model.Pricing

	if capability, ok := modelprofile.CapabilityFor(provider.Name, model.Name); ok {
		if session.MaxContextTokens == 0 {
			session.MaxContextTokens = capability.MaxContextTokens
		}
		if session.Reasoning == "" {
			session.Reasoning = capability.ReasoningLevel
		}
		if session.Pricing == nil {
			session.Pricing = capability.Pricing
		}
	}

	if strings.TrimSpace(opt.Reasoning) != "" {
		session.Reasoning = strings.TrimSpace(opt.Reasoning)
		session.ReasoningDefault = false
	}
	if session.Reasoning == "" {
		session.Reasoning = "medium"
		session.ReasoningDefault = true
	}
	if session.MaxContextTokens == 0 {
		session.ContextUnknown = true
	}
	return session
}

func (s Session) ContextLabel() string {
	if s.ContextUnknown || s.MaxContextTokens <= 0 {
		return "unknown"
	}
	return fmt.Sprintf("0 / %s tokens", compactTokens(s.MaxContextTokens))
}

func (s Session) ReasoningLabel() string {
	if s.ReasoningDefault {
		return s.Reasoning + " (default)"
	}
	return s.Reasoning
}

func (s *Session) SetMode(mode string) bool {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "single" && mode != "multi" {
		return false
	}
	s.Mode = mode
	s.Worktree = mode == "multi"
	return true
}

func (s *Session) SetReasoning(level string) bool {
	level = strings.ToLower(strings.TrimSpace(level))
	switch level {
	case "low", "medium", "high":
		s.Reasoning = level
		s.ReasoningDefault = false
		return true
	default:
		return false
	}
}

func selectModel(models config.ModelsConfig, override string) (config.ProviderConfig, config.ModelConfig) {
	target := strings.TrimSpace(override)
	if target == "" {
		target = strings.TrimSpace(models.Routing.DefaultModel)
	}
	for _, provider := range models.Providers {
		for _, model := range provider.Models {
			if target == "" || model.Name == target {
				return provider, model
			}
		}
	}
	if len(models.Providers) > 0 {
		provider := models.Providers[0]
		if len(provider.Models) > 0 {
			return provider, provider.Models[0]
		}
		return provider, config.ModelConfig{}
	}
	return config.ProviderConfig{}, config.ModelConfig{}
}

func hostOnly(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}

func compactTokens(tokens int) string {
	if tokens >= 1024 && tokens%1024 == 0 {
		return fmt.Sprintf("%dk", tokens/1024)
	}
	return fmt.Sprintf("%d", tokens)
}
