package modelprofile

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nekonomimo/nekonomimo/internal/config"
)

const (
	CapabilitySourcePreset  = "preset"
	CapabilitySourceUnknown = "unknown"
	PricingSourceUnknown    = "unknown"
)

// ModelCapability describes optional model metadata that can be safely inferred.
type ModelCapability struct {
	Provider            string
	Model               string
	MaxContextTokens    int
	ReasoningLevel      string
	SupportsPrefixCache bool
	Pricing             *config.ModelPricingConfig
	Source              string
}

// EnrichOptions selects configured models to enrich with known capabilities.
type EnrichOptions struct {
	Provider     string
	Model        string
	All          bool
	AllowPricing bool
}

// EnrichResult reports which model profiles changed or were left untouched.
type EnrichResult struct {
	Updated []string
	Skipped []string
}

var capabilityPresets = []ModelCapability{
	{
		Provider:            "mimo",
		Model:               "mimo-v2.5-pro",
		MaxContextTokens:    131072,
		ReasoningLevel:      "high",
		SupportsPrefixCache: false,
		Source:              CapabilitySourcePreset,
	},
}

// CapabilityFor returns a known capability preset without guessing unknown models.
func CapabilityFor(providerName, modelName string) (ModelCapability, bool) {
	providerName = strings.TrimSpace(providerName)
	modelName = strings.TrimSpace(modelName)
	for _, cap := range capabilityPresets {
		if cap.Provider == providerName && cap.Model == modelName {
			return cap, true
		}
	}
	return ModelCapability{}, false
}

// ApplyCapability fills only missing optional fields. It never overwrites user values.
func ApplyCapability(model *config.ModelConfig, cap ModelCapability, allowPricing bool) bool {
	if model == nil {
		return false
	}
	changed := false
	if model.MaxContextTokens == 0 && cap.MaxContextTokens > 0 {
		model.MaxContextTokens = cap.MaxContextTokens
		changed = true
	}
	if strings.TrimSpace(model.ReasoningLevel) == "" && strings.TrimSpace(cap.ReasoningLevel) != "" {
		model.ReasoningLevel = strings.TrimSpace(cap.ReasoningLevel)
		changed = true
	}
	if strings.TrimSpace(model.CapabilitySource) == "" && changed {
		source := strings.TrimSpace(cap.Source)
		if source == "" {
			source = CapabilitySourcePreset
		}
		model.CapabilitySource = source
	}
	if allowPricing && model.Pricing == nil && cap.Pricing != nil {
		pricing := *cap.Pricing
		model.Pricing = &pricing
		if model.Pricing.Source == "" {
			model.Pricing.Source = cap.Source
		}
		changed = true
	}
	return changed
}

// Enrich updates existing model profiles with known capabilities.
func Enrich(root string, opt EnrichOptions) (EnrichResult, error) {
	if !opt.All && strings.TrimSpace(opt.Provider) == "" && strings.TrimSpace(opt.Model) == "" {
		return EnrichResult{}, errors.New("model enrich requires --provider, --model, or --all")
	}
	models, err := Load(root)
	if err != nil {
		return EnrichResult{}, err
	}
	result := enrichModels(&models, opt, nil)
	if len(result.Updated) > 0 {
		if err := Save(root, models); err != nil {
			return EnrichResult{}, err
		}
	}
	return result, nil
}

// EnrichDiscovered updates configured models that also appeared in /models output.
func EnrichDiscovered(root string, providerName string, discovered []string) (EnrichResult, error) {
	models, err := Load(root)
	if err != nil {
		return EnrichResult{}, err
	}
	allowed := make(map[string]bool, len(discovered))
	for _, name := range discovered {
		name = strings.TrimSpace(name)
		if name != "" {
			allowed[name] = true
		}
	}
	result := enrichModels(&models, EnrichOptions{Provider: providerName}, allowed)
	if len(result.Updated) > 0 {
		if err := Save(root, models); err != nil {
			return EnrichResult{}, err
		}
	}
	return result, nil
}

func enrichModels(models *config.ModelsConfig, opt EnrichOptions, allowed map[string]bool) EnrichResult {
	result := EnrichResult{}
	for providerIndex := range models.Providers {
		provider := &models.Providers[providerIndex]
		if strings.TrimSpace(opt.Provider) != "" && provider.Name != strings.TrimSpace(opt.Provider) {
			continue
		}
		for modelIndex := range provider.Models {
			model := &provider.Models[modelIndex]
			if strings.TrimSpace(opt.Model) != "" && model.Name != strings.TrimSpace(opt.Model) {
				continue
			}
			if allowed != nil && !allowed[model.Name] {
				continue
			}
			label := fmt.Sprintf("%s/%s", provider.Name, model.Name)
			capability, ok := CapabilityFor(provider.Name, model.Name)
			if !ok {
				result.Skipped = append(result.Skipped, label)
				continue
			}
			if ApplyCapability(model, capability, opt.AllowPricing) {
				result.Updated = append(result.Updated, label)
			} else {
				result.Skipped = append(result.Skipped, label)
			}
		}
	}
	return result
}
