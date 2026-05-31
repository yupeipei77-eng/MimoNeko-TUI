package contextengine

import "testing"

func TestDefaultBudgetConfig(t *testing.T) {
	cfg := DefaultBudgetConfig()
	if cfg.ImmutablePrefix != 100000 {
		t.Errorf("ImmutablePrefix = %d, want 100000", cfg.ImmutablePrefix)
	}
	if cfg.Conversation != 50000 {
		t.Errorf("Conversation = %d, want 50000", cfg.Conversation)
	}
	if cfg.Scratchpad != 30000 {
		t.Errorf("Scratchpad = %d, want 30000", cfg.Scratchpad)
	}
	if cfg.Output != 4096 {
		t.Errorf("Output = %d, want 4096", cfg.Output)
	}
}

func TestBudgetConfigToTokenBudget(t *testing.T) {
	cfg := BudgetConfig{
		ImmutablePrefix: 100,
		Conversation:    200,
		Scratchpad:      300,
		Output:          400,
	}

	tb := cfg.ToTokenBudget()
	if tb.ImmutablePrefix != 100 {
		t.Errorf("ImmutablePrefix = %d, want 100", tb.ImmutablePrefix)
	}
	if tb.Conversation != 200 {
		t.Errorf("Conversation = %d, want 200", tb.Conversation)
	}
	if tb.Scratchpad != 300 {
		t.Errorf("Scratchpad = %d, want 300", tb.Scratchpad)
	}
	if tb.Output != 400 {
		t.Errorf("Output = %d, want 400", tb.Output)
	}
}

func TestBudgetForModelZeroContext(t *testing.T) {
	defaults := DefaultBudgetConfig()
	result := BudgetForModel(0, defaults)

	if result.ImmutablePrefix != defaults.ImmutablePrefix {
		t.Errorf("should return defaults when maxContextTokens=0")
	}
}

func TestBudgetForModelScalesDown(t *testing.T) {
	defaults := DefaultBudgetConfig()
	// Small context window should scale down
	result := BudgetForModel(8000, defaults)

	total := result.ImmutablePrefix + result.Conversation + result.Scratchpad
	// Should be scaled to fit within ~75% of 8000 = 6000
	if total > 6500 {
		t.Errorf("total budget = %d, should be <= ~6000 for 8k context", total)
	}
	if result.Output != 2000 {
		t.Errorf("Output = %d, want 2000 (8000/4)", result.Output)
	}
}

func TestBudgetForModelLargeContext(t *testing.T) {
	defaults := DefaultBudgetConfig()
	// Large context window (128k) should use proportional scaling
	result := BudgetForModel(128000, defaults)

	// 128k * 0.75 = 96k available, default total = 180k
	// So it scales DOWN, not up. But the total should still be larger than a small context.
	total := result.ImmutablePrefix + result.Conversation + result.Scratchpad
	if total <= 0 {
		t.Errorf("total budget = %d, want > 0", total)
	}
	if result.Output != 32000 {
		t.Errorf("Output = %d, want 32000 (128000/4)", result.Output)
	}
}

func TestBudgetForModelMinimums(t *testing.T) {
	defaults := DefaultBudgetConfig()
	// Very small context should enforce minimums
	result := BudgetForModel(1000, defaults)

	if result.ImmutablePrefix < 1000 {
		t.Errorf("ImmutablePrefix = %d, want >= 1000", result.ImmutablePrefix)
	}
	if result.Conversation < 1000 {
		t.Errorf("Conversation = %d, want >= 1000", result.Conversation)
	}
	if result.Scratchpad < 500 {
		t.Errorf("Scratchpad = %d, want >= 500", result.Scratchpad)
	}
}
