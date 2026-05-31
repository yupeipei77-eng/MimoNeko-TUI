package prefix

import (
	"context"
	"fmt"

	"github.com/nekonomimo/nekonomimo/internal/config"
)

// ImmutablePrefixBuilder builds byte-stable immutable prefixes from
// system prompts, coding rules, and tool schemas. The output is
// deterministic: identical inputs always produce identical bytes and hashes.
type ImmutablePrefixBuilder struct {
	byteStable config.ByteStableConfig
}

// NewImmutablePrefixBuilder creates a builder with the given byte-stable configuration.
func NewImmutablePrefixBuilder(bs config.ByteStableConfig) *ImmutablePrefixBuilder {
	return &ImmutablePrefixBuilder{byteStable: bs}
}

// Build assembles an immutable prefix document from the request.
// The assembly order is: system_prompt â†?coding_rules â†?tool_schemas.
// All content is canonicalized for byte-stability.
func (b *ImmutablePrefixBuilder) Build(ctx context.Context, req BuildRequest) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}

	// Validate source kinds when dynamic content is disallowed
	if b.byteStable.DisallowDynamicContent {
		for _, src := range req.Sources {
			if !isAllowedSourceKind(src.Kind) {
				return Document{}, fmt.Errorf("immutable prefix source %q has disallowed kind %q", src.Name, src.Kind)
			}
		}
	}

	// Canonicalize each section
	systemPrompt := CanonicalText(req.SystemPrompt)
	codingRules := CanonicalText(req.CodingRules)
	toolSchemas := CanonicalTools(req.ToolSchemas)

	// Assemble in deterministic order: system_prompt â†?coding_rules â†?tool_schemas
	var assembled []byte
	assembled = append(assembled, systemPrompt...)
	if len(codingRules) > 0 {
		assembled = append(assembled, '\n')
		assembled = append(assembled, codingRules...)
	}
	if len(toolSchemas) > 0 {
		assembled = append(assembled, '\n')
		assembled = append(assembled, toolSchemas...)
	}

	return Document{
		Version:       req.Version,
		Bytes:         assembled,
		SHA256:        StableHash(assembled),
		Sources:       req.Sources,
		TokenEstimate: EstimateTokens(assembled),
	}, nil
}

// Fingerprint returns the SHA-256 fingerprint and version for the given request
// without carrying the full byte payload.
func (b *ImmutablePrefixBuilder) Fingerprint(ctx context.Context, req BuildRequest) (Fingerprint, error) {
	doc, err := b.Build(ctx, req)
	if err != nil {
		return Fingerprint{}, err
	}
	return Fingerprint{SHA256: doc.SHA256, Version: doc.Version}, nil
}

func isAllowedSourceKind(kind SourceKind) bool {
	switch kind {
	case SourceKindStaticFile, SourceKindGeneratedSchema:
		return true
	default:
		return false
	}
}
