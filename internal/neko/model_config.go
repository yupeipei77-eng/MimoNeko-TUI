package neko

import (
	"fmt"
	"io"
	"strings"

	"github.com/mimoneko/mimoneko/internal/neko/branding"
)

// ModelConfig represents the model configuration display.
type ModelConfig struct {
	Provider        string
	Model           string
	BaseURL         string
	APIKeyStatus    string
	AvailableModels []string
	CurrentModel    string
	NoColor         bool
}

// NewModelConfig creates a new ModelConfig from session.
func NewModelConfig(session Session) ModelConfig {
	return ModelConfig{
		Provider:     session.Provider,
		Model:        session.Model,
		BaseURL:      session.BaseURLHost,
		APIKeyStatus: session.APIKeyStatus,
		CurrentModel: session.Model,
		NoColor:      session.NoColor,
	}
}

// RenderModelConfig renders the model configuration UI.
func (m ModelConfig) RenderModelConfig(w io.Writer) {
	renderer := branding.NewRenderer(m.NoColor)

	// Header
	fmt.Fprintln(w, renderer.Title("Model Configuration"))
	fmt.Fprintln(w)

	// Current configuration
	fmt.Fprintf(w, "  %s\n", renderer.Label("Current Configuration"))
	fmt.Fprintf(w, "    %-12s %s\n", "Provider:", renderer.Value(emptyAsUnknown(m.Provider)))
	fmt.Fprintf(w, "    %-12s %s\n", "Model:", renderer.Value(emptyAsUnknown(m.Model)))
	fmt.Fprintf(w, "    %-12s %s\n", "Base URL:", renderer.Value(emptyAsUnknown(m.BaseURL)))
	fmt.Fprintf(w, "    %-12s %s\n", "API Key:", renderer.Value(m.APIKeyStatus))
	fmt.Fprintln(w)

	// Available models
	if len(m.AvailableModels) > 0 {
		fmt.Fprintf(w, "  %s\n", renderer.Label("Available Models"))
		for i, model := range m.AvailableModels {
			prefix := "  "
			if model == m.CurrentModel {
				prefix = renderer.Accent("> ")
			}
			fmt.Fprintf(w, "    %s%s\n", prefix, model)
			if i > 10 {
				fmt.Fprintf(w, "    ... and %d more\n", len(m.AvailableModels)-10)
				break
			}
		}
		fmt.Fprintln(w)
	}

	// Commands
	fmt.Fprintf(w, "  %s\n", renderer.Label("Commands"))
	fmt.Fprintf(w, "    %-20s %s\n", "/model use <name>", "Switch model")
	fmt.Fprintf(w, "    %-20s %s\n", "/model provider <name>", "Switch provider")
	fmt.Fprintf(w, "    %-20s %s\n", "/model url <url>", "Set base URL")
	fmt.Fprintf(w, "    %-20s %s\n", "/model key", "Set API key")
	fmt.Fprintf(w, "    %-20s %s\n", "/model test", "Test connection")
	fmt.Fprintf(w, "    %-20s %s\n", "/model enrich", "Enrich capabilities")
	fmt.Fprintf(w, "    %-20s %s\n", "q / Esc", "Return to chat")
	fmt.Fprintln(w)
}

// RenderModelSwitch renders a model switch confirmation.
func RenderModelSwitch(w io.Writer, model, provider string, noColor bool) {
	renderer := branding.NewRenderer(noColor)
	fmt.Fprintf(w, "%s\n", renderer.Success(fmt.Sprintf("✓ Switched to %s (provider: %s)", model, provider)))
}

// RenderModelSwitchError renders a model switch error.
func RenderModelSwitchError(w io.Writer, model string, noColor bool) {
	renderer := branding.NewRenderer(noColor)
	fmt.Fprintf(w, "%s\n", renderer.Error(fmt.Sprintf("✗ Model %q not found; run /model to see available models", model)))
}

// RenderModelTestResult renders a model test result.
func RenderModelTestResult(w io.Writer, result string, noColor bool) {
	renderer := branding.NewRenderer(noColor)
	if strings.Contains(result, "status=ok") {
		fmt.Fprintf(w, "%s\n", renderer.Success("✓ Model test successful"))
	} else {
		fmt.Fprintf(w, "%s\n", renderer.Error("✗ Model test failed"))
	}
	fmt.Fprintln(w, result)
}
