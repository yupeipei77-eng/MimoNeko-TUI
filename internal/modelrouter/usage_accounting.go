package modelrouter

import (
	"time"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/cache"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/contextengine"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/prefix"
)

// UsageToObservation converts a CompletionResponse Usage and Bundle into a
// cache.Observation for recording into the CacheRegistry.
//
// Field mapping:
//   - Fingerprint         <- Bundle.CacheFingerprint
//   - Provider            <- provider name
//   - Model               <- model name
//   - RequestID           <- request ID from provider
//   - InputTokens         <- Usage.InputTokens (fallback: Bundle.Report.TotalTokens)
//   - CachedTokens        <- Usage.CachedTokens (0 if missing)
//   - CacheHitTokens      <- Usage.CacheHitTokens (MIMO native when available)
//   - CacheMissTokens     <- Usage.CacheMissTokens (MIMO native when available)
//   - ObservedAt          <- time.Now()
//   - Estimated           <- Usage.Estimated
//   - PrefixTokens        <- Bundle.Report.PrefixTokens
//   - ConversationTokens  <- Bundle.Report.ConversationTokens
//   - ScratchpadTokens    <- Bundle.Report.ScratchpadTokens
//   - CurrentInputTokens  <- Bundle.Report.CurrentInputTokens
func UsageToObservation(usage Usage, bundle contextengine.Bundle, provider, model, requestID string) cache.Observation {
	inputTokens := usage.InputTokens
	if inputTokens == 0 && bundle.Report.TotalTokens > 0 {
		inputTokens = bundle.Report.TotalTokens
	}

	return cache.Observation{
		Fingerprint: prefix.Fingerprint{
			SHA256:  bundle.CacheFingerprint.SHA256,
			Version: bundle.CacheFingerprint.Version,
		},
		Provider:           provider,
		Model:              model,
		RequestID:          requestID,
		InputTokens:        inputTokens,
		CachedTokens:       usage.CachedTokens,
		CacheHitTokens:     usage.CacheHitTokens,
		CacheMissTokens:    usage.CacheMissTokens,
		NativeCacheKnown:   usage.NativeCacheKnown,
		ObservedAt:         time.Now(),
		Estimated:          usage.Estimated,
		PrefixTokens:       bundle.Report.PrefixTokens,
		ConversationTokens: bundle.Report.ConversationTokens,
		ScratchpadTokens:   bundle.Report.ScratchpadTokens,
		CurrentInputTokens: bundle.Report.CurrentInputTokens,
	}
}
