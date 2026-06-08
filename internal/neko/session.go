package neko

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/modelprofile"
)

type Session struct {
	Root               string
	Models             config.ModelsConfig
	Mode               string
	Model              string
	Provider           string
	BaseURLHost        string
	APIKeyStatus       string
	MaxContextTokens   int
	ContextUnknown     bool
	Reasoning          string
	ReasoningDefault   bool
	ReasoningAvailable bool
	DryRun             bool
	Worktree           bool
	NoColor            bool
	Usage              Usage
	ContextUsedTokens  int
	MemoryMessages     int
	Pricing            *config.ModelPricingConfig
	StartedAt          time.Time
	ToolsUsed          int
	LastLatency        time.Duration
	CacheHitKnown      bool
	CacheHitRate       float64
}

func NewSession(root string, models config.ModelsConfig, opt Options) Session {
	mode := strings.ToLower(strings.TrimSpace(opt.Mode))
	if mode == "" {
		mode = "multi"
	}
	selectedMode, ok := agentModeByID(mode)
	if !ok {
		mode = "multi"
		selectedMode, _ = agentModeByID(mode)
	}
	dryRun := true
	if opt.DryRunSet {
		dryRun = opt.DryRun
	}
	session := Session{
		Root:      root,
		Models:    models,
		Mode:      selectedMode.ID(),
		DryRun:    dryRun,
		Worktree:  selectedMode.UseWorktree(),
		NoColor:   opt.NoColor,
		Usage:     Usage{Estimated: true},
		StartedAt: time.Now(),
	}

	provider, model := selectModel(models, opt.Model)
	session.Provider = provider.Name
	session.Model = model.Name
	session.BaseURLHost = hostOnly(provider.BaseURL)
	session.APIKeyStatus = modelprofile.APIKeyStatus(provider.APIKeyEnv)
	session.MaxContextTokens = model.MaxContextTokens
	session.Reasoning = strings.TrimSpace(model.ReasoningLevel)
	session.ReasoningAvailable = session.Reasoning != ""
	session.Pricing = model.Pricing

	if capability, ok := modelprofile.CapabilityFor(provider.Name, model.Name); ok {
		if session.MaxContextTokens == 0 {
			session.MaxContextTokens = capability.MaxContextTokens
		}
		if session.Reasoning == "" {
			session.Reasoning = capability.ReasoningLevel
		}
		if strings.TrimSpace(capability.ReasoningLevel) != "" {
			session.ReasoningAvailable = true
		}
		if session.Pricing == nil {
			session.Pricing = capability.Pricing
		}
	}

	if strings.TrimSpace(opt.Reasoning) != "" {
		session.Reasoning = strings.TrimSpace(opt.Reasoning)
		session.ReasoningDefault = false
		session.ReasoningAvailable = true
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

func (s *Session) SelectModel(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, provider := range s.Models.Providers {
		for _, model := range provider.Models {
			if model.Name == name {
				s.applyModel(provider, model)
				return true
			}
		}
	}
	return false
}

func (s *Session) AvailableModels() []string {
	var out []string
	for _, provider := range s.Models.Providers {
		for _, model := range provider.Models {
			if model.Name != "" {
				out = append(out, provider.Name+"/"+model.Name)
			}
		}
	}
	return out
}

func (s *Session) applyModel(provider config.ProviderConfig, model config.ModelConfig) {
	s.Provider = provider.Name
	s.Model = model.Name
	s.BaseURLHost = hostOnly(provider.BaseURL)
	s.APIKeyStatus = modelprofile.APIKeyStatus(provider.APIKeyEnv)
	s.MaxContextTokens = model.MaxContextTokens
	s.ContextUnknown = s.MaxContextTokens == 0
	s.Reasoning = strings.TrimSpace(model.ReasoningLevel)
	s.ReasoningDefault = false
	s.ReasoningAvailable = s.Reasoning != ""
	s.Pricing = model.Pricing
	if capability, ok := modelprofile.CapabilityFor(provider.Name, model.Name); ok {
		if s.MaxContextTokens == 0 {
			s.MaxContextTokens = capability.MaxContextTokens
			s.ContextUnknown = false
		}
		if s.Reasoning == "" {
			s.Reasoning = capability.ReasoningLevel
		}
		if strings.TrimSpace(capability.ReasoningLevel) != "" {
			s.ReasoningAvailable = true
		}
		if s.Pricing == nil {
			s.Pricing = capability.Pricing
		}
	}
	if s.Reasoning == "" {
		s.Reasoning = "medium"
		s.ReasoningDefault = true
	}
}

func (s Session) SessionLabel() string {
	if s.StartedAt.IsZero() {
		return "0m"
	}
	elapsed := time.Since(s.StartedAt)
	if elapsed < time.Minute {
		return fmt.Sprintf("%ds", int(elapsed.Seconds()))
	}
	if elapsed < time.Hour {
		return fmt.Sprintf("%dm", int(elapsed.Minutes()))
	}
	return fmt.Sprintf("%.1fh", elapsed.Hours())
}

func (s Session) LatencyLabel() string {
	if s.LastLatency <= 0 {
		return ""
	}
	if s.LastLatency < time.Second {
		return fmt.Sprintf("%dms", s.LastLatency.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", s.LastLatency.Seconds())
}

func (s Session) ContextLabel() string {
	if s.ContextUnknown || s.MaxContextTokens <= 0 {
		return "n/a"
	}
	pct := 0
	if s.MaxContextTokens > 0 {
		pct = (s.ContextUsedTokens * 100) / s.MaxContextTokens
	}
	if pct < 1 {
		return "<1%"
	}
	return fmt.Sprintf("%d%%", pct)
}

func (s Session) ReasoningLabel() string {
	if !s.ReasoningAvailable {
		return ""
	}
	if s.ReasoningDefault {
		return s.Reasoning + " (default)"
	}
	return s.Reasoning
}

func (s Session) ReasoningStatusLabel() string {
	if !s.ReasoningAvailable {
		return ""
	}
	return strings.TrimSpace(s.Reasoning)
}

func (s Session) CommandHint() string {
	return "ctrl+p commands"
}

func (s *Session) SetMode(mode string) bool {
	selectedMode, ok := agentModeByID(mode)
	if !ok {
		return false
	}
	s.Mode = selectedMode.ID()
	s.Worktree = selectedMode.UseWorktree()
	return true
}

func (s *Session) SetReasoning(level string) bool {
	if !s.ReasoningAvailable {
		return false
	}
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

func (s *Session) CycleReasoning() (string, bool) {
	if !s.ReasoningAvailable {
		return "", false
	}
	switch strings.ToLower(strings.TrimSpace(s.Reasoning)) {
	case "low":
		s.Reasoning = "medium"
	case "medium":
		s.Reasoning = "high"
	default:
		s.Reasoning = "low"
	}
	s.ReasoningDefault = false
	return s.Reasoning, true
}

func (s *Session) AddUserMemory(text string) {
	tokens := EstimateTextTokens(text)
	if tokens <= 0 {
		return
	}
	s.ContextUsedTokens += tokens
	s.Usage = MergeUsage(s.Usage, Usage{InputTokens: tokens, Estimated: true})
	s.MemoryMessages++
}

func (s *Session) AddAssistantMemory(text string) {
	tokens := EstimateTextTokens(text)
	if tokens <= 0 {
		return
	}
	s.ContextUsedTokens += tokens
	s.Usage = MergeUsage(s.Usage, Usage{OutputTokens: tokens, Estimated: true})
	s.MemoryMessages++
}

func (s *Session) ApplyActualUsage(usage Usage) {
	usage = NormalizeUsage(usage)
	if usage.TotalTokens == 0 && usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.CachedTokens == 0 {
		return
	}
	if !usage.Estimated {
		if usage.NativeCacheKnown {
			denom := usage.CacheHitTokens + usage.CacheMissTokens
			if denom > 0 {
				s.CacheHitKnown = true
				s.CacheHitRate = float64(usage.CacheHitTokens) / float64(denom) * 100
			}
		} else if usage.InputTokens > 0 {
			s.CacheHitKnown = true
			s.CacheHitRate = float64(usage.CachedTokens) / float64(usage.InputTokens) * 100
		}
	}
	s.Usage = MergeUsage(s.Usage, usage)
	if s.Usage.TotalTokens > s.ContextUsedTokens {
		s.ContextUsedTokens = s.Usage.TotalTokens
	}
}

func (s *Session) MemoryLabel() string {
	if s.MemoryMessages <= 0 {
		return "0 msgs"
	}
	return fmt.Sprintf("%d msgs", s.MemoryMessages)
}

func (s *Session) ResetConversation() {
	s.Usage = Usage{Estimated: true}
	s.ContextUsedTokens = 0
	s.MemoryMessages = 0
	s.ToolsUsed = 0
	s.LastLatency = 0
	s.CacheHitKnown = false
	s.CacheHitRate = 0
	s.StartedAt = time.Now()
}

func (s Session) CacheLabel() string {
	if !s.CacheHitKnown {
		return "unsupported"
	}
	return fmt.Sprintf("%.1f%%", s.CacheHitRate)
}

func (s Session) CacheReport() []string {
	usage := NormalizeUsage(s.Usage)
	cachedTokens := usage.CachedTokens
	if usage.NativeCacheKnown {
		cachedTokens = usage.CacheHitTokens
	}
	return []string{
		fmt.Sprintf("context=%s", s.ContextLabel()),
		fmt.Sprintf("input_tokens=%d", usage.InputTokens),
		fmt.Sprintf("cached_tokens=%d", cachedTokens),
		fmt.Sprintf("cache_hit_rate=%s", s.CacheLabel()),
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
	if tokens <= 0 {
		return "0K"
	}
	if tokens < 100 {
		return trimCompactFloat(float64(tokens)/1000, 3) + "K"
	}
	if tokens < 1_000_000 {
		k := float64(tokens) / 1000
		return trimCompactFloat(k, 3) + "K"
	}
	m := float64(tokens) / 1_000_000
	return trimCompactFloat(m, 3) + "M"
}

func detailedTokens(tokens int) string {
	return fmt.Sprintf("%s tok (%s)", formatTokenInteger(tokens), compactTokens(tokens))
}

func formatTokenInteger(tokens int) string {
	raw := fmt.Sprintf("%d", tokens)
	if len(raw) <= 3 {
		return raw
	}
	var out []byte
	first := len(raw) % 3
	if first == 0 {
		first = 3
	}
	out = append(out, raw[:first]...)
	for i := first; i < len(raw); i += 3 {
		out = append(out, ',')
		out = append(out, raw[i:i+3]...)
	}
	return string(out)
}

func trimCompactFloat(value float64, precision int) string {
	formatted := fmt.Sprintf("%.*f", precision, value)
	formatted = strings.TrimRight(formatted, "0")
	return strings.TrimSuffix(formatted, ".")
}
