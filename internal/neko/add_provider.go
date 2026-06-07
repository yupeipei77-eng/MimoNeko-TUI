package neko

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mimoneko/mimoneko/internal/auth"
	"github.com/mimoneko/mimoneko/internal/config"
	"github.com/mimoneko/mimoneko/internal/modelprofile"
)

const (
	stepIdle = iota
	stepProviderName
	stepBaseURL
	stepAPIKey
	stepHeaderType
	stepDiscovering
	stepManualModelFallback
	stepDone
)

type addProviderFlow struct {
	active       bool
	step         int
	name         string
	baseURL      string
	apiKey       string
	headerType   string
	selectedHead int
	manualModel  string
	status       string
}

func (c *Console) startAddProviderFlow() {
	c.startAddProviderFlowWithProvider("", "")
}

func (c *Console) startAddProviderFlowWithProvider(name, baseURL string) {
	name = strings.TrimSpace(name)
	baseURL = strings.TrimSpace(baseURL)
	step := stepProviderName
	if name != "" {
		step = stepBaseURL
	}
	if baseURL != "" {
		step = stepAPIKey
	}
	c.addFlow = addProviderFlow{
		active:       true,
		step:         step,
		name:         name,
		baseURL:      baseURL,
		selectedHead: 0,
	}
	c.providerPickerOpen = false
	c.modelPickerOpen = false
	c.paletteOpen = false
	c.draft = ""
	if !c.screenActive {
		c.emitInfo("Add API Provider\n" + c.addFlowPrompt())
	}
	c.repaintScreen()
}

func (c *Console) cancelAddProviderFlow() {
	c.addFlow = addProviderFlow{}
	c.draft = ""
	c.emitAssistant("system", "Cancelled.", false)
	c.repaintScreen()
}

func (c *Console) addFlowPrompt() string {
	switch c.addFlow.step {
	case stepProviderName:
		return "Provider name"
	case stepBaseURL:
		return "Base URL"
	case stepAPIKey:
		return "API Key"
	case stepHeaderType:
		return "Header type"
	case stepManualModelFallback:
		return "Model name"
	default:
		return ""
	}
}

func (c *Console) addFlowIsPassword() bool {
	return c.addFlow.step == stepAPIKey
}

func (f addProviderFlow) headerTypeLabel() string {
	if f.selectedHead == 1 || strings.EqualFold(f.headerType, "bearer") {
		return "Bearer"
	}
	return "API_KEY"
}

func (c *Console) moveAddFlowHeader(delta int) {
	if !c.addFlow.active || c.addFlow.step != stepHeaderType {
		return
	}
	if delta != 0 {
		c.addFlow.selectedHead = (c.addFlow.selectedHead + delta + 2) % 2
	}
	c.repaintScreen()
}

func (c *Console) handleAddFlowInput(ctx context.Context) bool {
	input := strings.TrimSpace(c.draft)
	c.draft = ""

	switch c.addFlow.step {
	case stepProviderName:
		if input == "" {
			return false
		}
		c.addFlow.name = input
		if !c.screenActive {
			c.emitInfo("> " + input + "\n\nBase URL")
		}
		c.addFlow.step = stepBaseURL

	case stepBaseURL:
		if input == "" {
			return false
		}
		c.addFlow.baseURL = input
		if !c.screenActive {
			c.emitInfo("> " + input + "\n\nAPI Key")
		}
		c.addFlow.step = stepAPIKey

	case stepAPIKey:
		if input == "" {
			return false
		}
		c.addFlow.apiKey = input
		c.addFlow.selectedHead = 0
		c.emitInfo("> " + strings.Repeat("*", len([]rune(input))) + "\n\nHeader type\n1. API_KEY\n2. Bearer\nUse Up/Down or type 1/2")
		c.addFlow.step = stepHeaderType

	case stepHeaderType:
		if input == "" {
			input = c.addFlow.headerTypeLabel()
		}
		switch input {
		case "1", "api_key", "API_KEY", "api-key":
			c.addFlow.headerType = "api_key"
			c.addFlow.selectedHead = 0
		case "2", "bearer", "Bearer":
			c.addFlow.headerType = "bearer"
			c.addFlow.selectedHead = 1
		default:
			c.emitInfo("Please enter 1 or 2.")
			return false
		}
		c.emitInfo("> " + c.addFlow.headerTypeLabel() + "\n\nDiscovering models...")
		c.addFlow.step = stepDiscovering
		go c.runAddProviderDiscovery()

	case stepManualModelFallback:
		if input == "" {
			return false
		}
		c.addFlow.manualModel = input
		c.finishAddProviderFlow([]string{input})

	case stepDone:
		c.addFlow = addProviderFlow{}
	}
	c.repaintScreen()
	return false
}

func (c *Console) runAddProviderDiscovery() {
	models, err := c.discoverModels(context.Background())
	c.addFlow.status = "done"
	if err != nil {
		c.addFlow.status = "failed"
		c.emitInfo("Model discovery failed.\nEnter a model name manually:")
		c.addFlow.step = stepManualModelFallback
	} else if len(models) == 0 {
		c.addFlow.status = "empty"
		c.emitInfo("No models found.\nEnter a model name manually:")
		c.addFlow.step = stepManualModelFallback
	} else {
		c.finishAddProviderFlow(models)
	}
	c.repaintScreen()
}

func (c *Console) finishAddProviderFlow(models []string) {
	if err := c.saveAddProviderConfig(models); err != nil {
		c.setStatus(fmt.Sprintf("Provider save failed: %s", SanitizeOutput(err.Error(), c.addFlow.apiKey)), true)
		c.addFlow = addProviderFlow{active: false, step: stepDone}
		c.draft = ""
		return
	}
	c.setStatus(fmt.Sprintf("Provider saved · %d models discovered", len(models)), false)
	c.addFlow = addProviderFlow{active: false, step: stepDone}
	c.draft = ""
}

func (c *Console) saveAddProviderConfig(models []string) error {
	name := strings.ToLower(strings.TrimSpace(c.addFlow.name))
	apiKeyEnv := auth.APIKeyEnv(name)
	headerType := c.addFlow.headerType
	if headerType == "" {
		headerType = "bearer"
	}

	providerType := "openai-compatible"
	if headerType == "api_key" {
		providerType = "mimo"
	}

	userConfig, err := auth.LoadUserConfig()
	if err == nil {
		if userConfig.Auth.Providers == nil {
			userConfig.Auth.Providers = make(map[string]auth.ProviderConfig)
		}
		userConfig.Auth.Providers[name] = auth.ProviderConfig{
			APIKey:  c.addFlow.apiKey,
			BaseURL: c.addFlow.baseURL,
		}
		if err := auth.SaveUserConfig(userConfig); err != nil {
			return err
		}
	}

	if err := os.Setenv(apiKeyEnv, c.addFlow.apiKey); err != nil {
		return err
	}

	modelsConfig, err := modelprofile.Load(c.Session.Root)
	if err != nil {
		modelsConfig = config.ModelsConfig{}
	}

	var modelConfigs []config.ModelConfig
	for _, m := range models {
		modelConfigs = append(modelConfigs, config.ModelConfig{
			Name:            m,
			Purpose:         "general",
			MaxOutputTokens: 4096,
		})
	}
	if len(modelConfigs) == 0 && c.addFlow.manualModel != "" {
		modelConfigs = append(modelConfigs, config.ModelConfig{
			Name:            c.addFlow.manualModel,
			Purpose:         "general",
			MaxOutputTokens: 4096,
		})
	}

	provider := config.ProviderConfig{
		Name:      name,
		Type:      providerType,
		BaseURL:   strings.TrimRight(c.addFlow.baseURL, "/"),
		APIKeyEnv: apiKeyEnv,
		Models:    modelConfigs,
	}

	found := false
	for i, p := range modelsConfig.Providers {
		if p.Name == name {
			modelsConfig.Providers[i] = provider
			found = true
			break
		}
	}
	if !found {
		modelsConfig.Providers = append(modelsConfig.Providers, provider)
	}

	if len(modelConfigs) > 0 && strings.TrimSpace(modelsConfig.Routing.DefaultModel) == "" {
		modelsConfig.Routing.DefaultModel = modelConfigs[0].Name
		modelsConfig.Routing.FallbackChain = []config.FallbackEntry{
			{Provider: name, Model: modelConfigs[0].Name},
		}
	}

	if err := modelprofile.Save(c.Session.Root, modelsConfig); err != nil {
		return err
	}
	c.Session.Models = modelsConfig
	return nil
}

func (c *Console) discoverModels(ctx context.Context) ([]string, error) {
	baseURL := strings.TrimRight(c.addFlow.baseURL, "/")
	if !strings.HasSuffix(baseURL, "/models") {
		baseURL = baseURL + "/models"
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.addFlow.headerType == "api_key" {
		req.Header.Set("api-key", c.addFlow.apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.addFlow.apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("provider did not expose /models (status %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse /models response: %w", err)
	}

	var names []string
	for _, item := range parsed.Data {
		if strings.TrimSpace(item.ID) != "" {
			names = append(names, item.ID)
		}
	}
	return names, nil
}
