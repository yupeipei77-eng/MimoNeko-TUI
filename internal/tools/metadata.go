package tools

import (
	"fmt"
	"slices"
	"time"
)

// RiskLevel classifies the safety risk of a registered tool.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// ToolMetadata is read-only descriptive metadata for tool review surfaces.
// It is intentionally not enforced by ToolRuntime in Phase 4.1.
type ToolMetadata struct {
	Name             string
	Description      string
	RiskLevel        RiskLevel
	Timeout          time.Duration
	RequiresApproval bool
	AllowedPaths     []string
}

// ToolMetadataRegistry exposes metadata registration and lookup without
// changing the execution-oriented ToolRegistry interface.
type ToolMetadataRegistry interface {
	RegisterMetadata(metadata ToolMetadata) error
	Metadata(name string) (ToolMetadata, bool)
	ListMetadata() []ToolMetadata
}

// RegisterToolMetadata registers metadata on registries that support it.
func RegisterToolMetadata(registry ToolRegistry, metadata ToolMetadata) error {
	metaRegistry, ok := registry.(ToolMetadataRegistry)
	if !ok {
		return fmt.Errorf("tools: registry does not support metadata")
	}
	return metaRegistry.RegisterMetadata(metadata)
}

// LookupToolMetadata returns metadata from registries that support it.
func LookupToolMetadata(registry ToolRegistry, name string) (ToolMetadata, bool) {
	metaRegistry, ok := registry.(ToolMetadataRegistry)
	if !ok {
		return ToolMetadata{}, false
	}
	return metaRegistry.Metadata(name)
}

// ListToolMetadata returns sorted metadata from registries that support it.
func ListToolMetadata(registry ToolRegistry) []ToolMetadata {
	metaRegistry, ok := registry.(ToolMetadataRegistry)
	if !ok {
		return nil
	}
	return metaRegistry.ListMetadata()
}

func metadataFromTool(tool Tool) ToolMetadata {
	risk := RiskLevel(tool.RiskLevel())
	if !validRiskLevel(risk) {
		risk = RiskLevelMedium
	}
	return ToolMetadata{
		Name:             tool.Name(),
		Description:      tool.Description(),
		RiskLevel:        risk,
		Timeout:          DefaultTimeoutSeconds * time.Second,
		RequiresApproval: defaultRequiresApproval(risk),
		AllowedPaths:     []string{"repo_root"},
	}
}

func normalizeMetadata(metadata ToolMetadata) (ToolMetadata, error) {
	if metadata.Name == "" {
		return ToolMetadata{}, fmt.Errorf("tools: metadata name must not be empty")
	}
	if metadata.RiskLevel == "" {
		metadata.RiskLevel = RiskLevelMedium
	}
	if !validRiskLevel(metadata.RiskLevel) {
		return ToolMetadata{}, fmt.Errorf("tools: invalid risk level %q", metadata.RiskLevel)
	}
	if metadata.Timeout <= 0 {
		metadata.Timeout = DefaultTimeoutSeconds * time.Second
	}
	metadata.AllowedPaths = slices.Clone(metadata.AllowedPaths)
	slices.Sort(metadata.AllowedPaths)
	return metadata, nil
}

func copyMetadata(metadata ToolMetadata) ToolMetadata {
	metadata.AllowedPaths = slices.Clone(metadata.AllowedPaths)
	return metadata
}

func validRiskLevel(risk RiskLevel) bool {
	switch risk {
	case RiskLevelLow, RiskLevelMedium, RiskLevelHigh, RiskLevelCritical:
		return true
	default:
		return false
	}
}

func defaultRequiresApproval(risk RiskLevel) bool {
	switch risk {
	case RiskLevelMedium, RiskLevelHigh, RiskLevelCritical:
		return true
	default:
		return false
	}
}
