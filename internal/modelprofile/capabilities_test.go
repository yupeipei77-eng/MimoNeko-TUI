package modelprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reasonforge/reasonforge/internal/config"
)

func TestModelCapabilityPresetMatchesKnownModel(t *testing.T) {
	capability, ok := CapabilityFor("mimo", "mimo-v2.5-pro")
	if !ok {
		t.Fatal("expected known mimo capability")
	}
	if capability.MaxContextTokens != 131072 || capability.ReasoningLevel != "high" {
		t.Fatalf("capability = %+v, want context and reasoning", capability)
	}
}

func TestModelCapabilityDoesNotGuessUnknownModel(t *testing.T) {
	if _, ok := CapabilityFor("mimo", "unknown-model"); ok {
		t.Fatal("unknown model should not have guessed capability")
	}
}

func TestModelCapabilityDoesNotOverrideUserConfig(t *testing.T) {
	model := config.ModelConfig{
		Name:             "mimo-v2.5-pro",
		MaxContextTokens: 32000,
		ReasoningLevel:   "low",
		CapabilitySource: "user",
	}
	capability, ok := CapabilityFor("mimo", "mimo-v2.5-pro")
	if !ok {
		t.Fatal("expected capability")
	}
	if ApplyCapability(&model, capability, false) {
		t.Fatal("capability should not change user-configured fields")
	}
	if model.MaxContextTokens != 32000 || model.ReasoningLevel != "low" || model.CapabilitySource != "user" {
		t.Fatalf("model was overridden: %+v", model)
	}
}

func TestModelEnrichWritesMissingContextLength(t *testing.T) {
	root := setupCapabilityRoot(t)
	result, err := Enrich(root, EnrichOptions{Provider: "mimo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 1 {
		t.Fatalf("updated = %+v, want one update", result.Updated)
	}
	model := loadCapabilityModel(t, root)
	if model.MaxContextTokens != 131072 {
		t.Fatalf("max_context_tokens = %d, want 131072", model.MaxContextTokens)
	}
}

func TestModelEnrichWritesReasoningLevel(t *testing.T) {
	root := setupCapabilityRoot(t)
	if _, err := Enrich(root, EnrichOptions{Model: "mimo-v2.5-pro"}); err != nil {
		t.Fatal(err)
	}
	model := loadCapabilityModel(t, root)
	if model.ReasoningLevel != "high" {
		t.Fatalf("reasoning_level = %q, want high", model.ReasoningLevel)
	}
}

func TestModelEnrichDoesNotWritePricingWhenUnknown(t *testing.T) {
	root := setupCapabilityRoot(t)
	if _, err := Enrich(root, EnrichOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	model := loadCapabilityModel(t, root)
	if model.Pricing != nil {
		t.Fatalf("pricing = %+v, want nil when preset has no known price", model.Pricing)
	}
}

func TestModelDiscoverWriteCapabilities(t *testing.T) {
	root := setupCapabilityRoot(t)
	result, err := EnrichDiscovered(root, "mimo", []string{"mimo-v2.5-pro"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 1 || result.Updated[0] != "mimo/mimo-v2.5-pro" {
		t.Fatalf("result = %+v, want discovered model update", result)
	}
	model := loadCapabilityModel(t, root)
	if model.MaxContextTokens == 0 || model.ReasoningLevel == "" {
		t.Fatalf("model was not enriched: %+v", model)
	}
}

func TestCapabilitySourceRecorded(t *testing.T) {
	root := setupCapabilityRoot(t)
	if _, err := Enrich(root, EnrichOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	model := loadCapabilityModel(t, root)
	if model.CapabilitySource != CapabilitySourcePreset {
		t.Fatalf("capability_source = %q, want preset", model.CapabilitySource)
	}
}

func TestPricingSourceRecorded(t *testing.T) {
	model := config.ModelConfig{Name: "priced-model"}
	changed := ApplyCapability(&model, ModelCapability{
		Provider: "test",
		Model:    "priced-model",
		Source:   CapabilitySourcePreset,
		Pricing:  &config.ModelPricingConfig{Currency: "CNY"},
	}, true)
	if !changed || model.Pricing == nil || model.Pricing.Source != CapabilitySourcePreset {
		t.Fatalf("model pricing = %+v, want source recorded", model.Pricing)
	}
}

func TestCapabilityDoesNotLeakAPIKey(t *testing.T) {
	root := setupCapabilityRoot(t)
	secret := "sk-capability-secret"
	t.Setenv("MIMO_API_KEY", secret)
	if _, err := Enrich(root, EnrichOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, ".reasonforge", "models.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), secret) {
		t.Fatalf("models.yaml leaked API key: %s", content)
	}
}

func setupCapabilityRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := config.InitDetailed(root); err != nil {
		t.Fatal(err)
	}
	models := config.ModelsConfig{
		Providers: []config.ProviderConfig{
			{
				Name:      "mimo",
				Type:      "openai-compatible",
				BaseURL:   "https://token-plan-cn.xiaomimimo.com/v1",
				APIKeyEnv: "MIMO_API_KEY",
				Models: []config.ModelConfig{
					{
						Name:                "mimo-v2.5-pro",
						Purpose:             "coding",
						MaxOutputTokens:     4096,
						SupportsPrefixCache: false,
					},
				},
			},
		},
		Routing: config.RoutingConfig{DefaultModel: "mimo-v2.5-pro"},
	}
	if err := Save(root, models); err != nil {
		t.Fatal(err)
	}
	return root
}

func loadCapabilityModel(t *testing.T, root string) config.ModelConfig {
	t.Helper()
	models, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range models.Providers {
		for _, model := range provider.Models {
			if model.Name == "mimo-v2.5-pro" {
				return model
			}
		}
	}
	t.Fatal("mimo-v2.5-pro not found")
	return config.ModelConfig{}
}
