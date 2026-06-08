package neko

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
)

// Usage captures token usage shown in the terminal console.
type Usage struct {
	InputTokens      int
	CachedTokens     int
	CacheHitTokens   int
	CacheMissTokens  int
	NativeCacheKnown bool
	OutputTokens     int
	TotalTokens      int
	Estimated        bool
}

// Cost is an estimated CNY cost derived from configured model pricing.
type Cost struct {
	Amount      float64
	Currency    string
	Unavailable bool
	Estimated   bool
}

func NormalizeUsage(usage Usage) Usage {
	if usage.NativeCacheKnown {
		if usage.CachedTokens == 0 {
			usage.CachedTokens = usage.CacheHitTokens
		}
		if usage.InputTokens == 0 {
			usage.InputTokens = usage.CacheHitTokens + usage.CacheMissTokens
		}
	}
	if usage.TotalTokens == 0 {
		if usage.NativeCacheKnown {
			usage.TotalTokens = usage.CacheHitTokens + usage.CacheMissTokens + usage.OutputTokens
		} else {
			usage.TotalTokens = usage.InputTokens + usage.CachedTokens + usage.OutputTokens
		}
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
	inputTokens := usage.InputTokens
	cachedTokens := usage.CachedTokens
	if usage.NativeCacheKnown {
		inputTokens = usage.CacheMissTokens
		cachedTokens = usage.CacheHitTokens
	}
	amount := float64(inputTokens)/1_000_000*pricing.InputPer1MTokens +
		float64(cachedTokens)/1_000_000*pricing.CachedInputPer1MTokens +
		float64(usage.OutputTokens)/1_000_000*pricing.OutputPer1MTokens
	return Cost{Amount: amount, Currency: currency, Estimated: usage.Estimated}
}

func FormatTokens(usage Usage) string {
	usage = NormalizeUsage(usage)
	if usage.NativeCacheKnown {
		return fmt.Sprintf("input=%d cache_hit=%d cache_miss=%d output=%d total=%d", usage.InputTokens, usage.CacheHitTokens, usage.CacheMissTokens, usage.OutputTokens, usage.TotalTokens)
	}
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

func EstimateTextTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	asciiRunes := 0
	wideRunes := 0
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		if r >= 0x2e80 {
			wideRunes++
			continue
		}
		asciiRunes++
	}
	estimated := wideRunes + (asciiRunes+3)/4
	if estimated <= 0 {
		return 1
	}
	return estimated
}

func MergeUsage(current Usage, next Usage) Usage {
	current = NormalizeUsage(current)
	next = NormalizeUsage(next)
	return NormalizeUsage(Usage{
		InputTokens:      current.InputTokens + next.InputTokens,
		CachedTokens:     current.CachedTokens + next.CachedTokens,
		CacheHitTokens:   current.CacheHitTokens + next.CacheHitTokens,
		CacheMissTokens:  current.CacheMissTokens + next.CacheMissTokens,
		NativeCacheKnown: current.NativeCacheKnown || next.NativeCacheKnown,
		OutputTokens:     current.OutputTokens + next.OutputTokens,
		Estimated:        current.Estimated || next.Estimated,
	})
}
