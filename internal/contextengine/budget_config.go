package contextengine

// BudgetConfig defines configurable token budgets for context assembly.
// This replaces hardcoded budget values in agent and multi-agent runtimes.
type BudgetConfig struct {
	// ImmutablePrefix is the token budget for the immutable prefix.
	ImmutablePrefix int `yaml:"immutable_prefix"`

	// Conversation is the token budget for conversation history.
	Conversation int `yaml:"conversation"`

	// Scratchpad is the token budget for volatile scratchpad content.
	Scratchpad int `yaml:"scratchpad"`

	// Output is the token budget for model output.
	Output int `yaml:"output"`
}

// DefaultBudgetConfig returns the default budget configuration.
func DefaultBudgetConfig() BudgetConfig {
	return BudgetConfig{
		ImmutablePrefix: 100000,
		Conversation:    50000,
		Scratchpad:      30000,
		Output:          4096,
	}
}

// ToTokenBudget converts BudgetConfig to TokenBudget.
func (c BudgetConfig) ToTokenBudget() TokenBudget {
	return TokenBudget{
		ImmutablePrefix: c.ImmutablePrefix,
		Conversation:    c.Conversation,
		Scratchpad:      c.Scratchpad,
		Output:          c.Output,
	}
}

// BudgetForModel returns a BudgetConfig adjusted for the given model's context window.
// If maxContextTokens is 0, returns the default budget.
func BudgetForModel(maxContextTokens int, defaults BudgetConfig) BudgetConfig {
	if maxContextTokens <= 0 {
		return defaults
	}

	// Reserve 25% of context for output
	available := maxContextTokens * 3 / 4

	// Scale budgets proportionally
	totalDefault := defaults.ImmutablePrefix + defaults.Conversation + defaults.Scratchpad
	if totalDefault <= 0 {
		return defaults
	}

	scale := float64(available) / float64(totalDefault)

	result := BudgetConfig{
		ImmutablePrefix: int(float64(defaults.ImmutablePrefix) * scale),
		Conversation:    int(float64(defaults.Conversation) * scale),
		Scratchpad:      int(float64(defaults.Scratchpad) * scale),
		Output:          maxContextTokens / 4,
	}

	// Ensure minimum budgets
	if result.ImmutablePrefix < 1000 {
		result.ImmutablePrefix = 1000
	}
	if result.Conversation < 1000 {
		result.Conversation = 1000
	}
	if result.Scratchpad < 500 {
		result.Scratchpad = 500
	}

	return result
}
