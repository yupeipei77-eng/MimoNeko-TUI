package neko

import (
	"fmt"

	"github.com/nekonomimo/nekonomimo/internal/config"
)

// Usage captures token usage shown in the terminal console.
type Usage struct {
	InputTokens  int
	CachedTokens int
	OutputTokens int
	TotalTokens  int
	Estimated    bool
}

// Cost is an estimated CNY cost derived from configured model pricing.
type Cost struct {
	Amount      float64
	Currency    string
	Unavailable bool
	Estimated   bool
}

func NormalizeUsage(usage Usage) Usage {
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.CachedTokens + usage.OutputTokens
	}
	return usage
}

func ComputeCost(usage Usage, pricing *config.ModelPricingConfig) Cost {
	usage = NormalizeUsage(usage)
	if pricing == nil {
		return Cost{Unavailable: true, Estimated: usage.Estimated}
	}
	currency := pricing.Currency
	if currency == "" {
		currency = "CNY"
	}
	amount := float64(usage.InputTokens)/1_000_000*pricing.InputPer1MTokens +
		float64(usage.CachedTokens)/1_000_000*pricing.CachedInputPer1MTokens +
		float64(usage.OutputTokens)/1_000_000*pricing.OutputPer1MTokens
	return Cost{Amount: amount, Currency: currency, Estimated: usage.Estimated}
}

func FormatTokens(usage Usage) string {
	usage = NormalizeUsage(usage)
	return fmt.Sprintf("input=%d cached=%d output=%d total=%d", usage.InputTokens, usage.CachedTokens, usage.OutputTokens, usage.TotalTokens)
}

func FormatCost(cost Cost) string {
	if cost.Unavailable {
		return "unavailable (pricing not configured)"
	}
	suffix := ""
	if cost.Estimated {
		suffix = " estimated"
	}
	if cost.Currency == "CNY" || cost.Currency == "" {
		return fmt.Sprintf("¥%.4f%s", cost.Amount, suffix)
	}
	return fmt.Sprintf("%.4f %s%s", cost.Amount, cost.Currency, suffix)
}
