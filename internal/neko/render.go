package neko

import (
	"fmt"
	"io"
	"strings"

	"github.com/mimoneko/mimoneko/internal/neko/branding"
)

func RenderHeader(w io.Writer, session Session) {
	renderer := branding.NewRenderer(session.NoColor)
	renderer.RenderStaticHeader(w, HeaderDataFromSession(session))
}

func HeaderDataFromSession(session Session) branding.HeaderData {
	return branding.HeaderData{
		Mode:      displayMode(session.Mode),
		Model:     emptyAsUnknown(session.Model),
		Provider:  emptyAsUnknown(session.Provider),
		Context:   session.ContextLabel(),
		Reasoning: session.ReasoningStatusLabel(),
		Tokens:    FormatTokens(session.Usage),
		Cost:      FormatCost(ComputeCost(session.Usage, session.Pricing)),
		Safety:    fmt.Sprintf("dry-run=%v worktree=%v no auto-apply", session.DryRun, session.Worktree),
	}
}

func RenderHelp(w io.Writer, noColor bool) {
	renderer := branding.NewRenderer(noColor)
	fmt.Fprintln(w, renderer.Title("Commands"))
	fmt.Fprintln(w, "plain text")
	fmt.Fprintln(w, "  chat with the configured model")
	fmt.Fprintln(w, "/")
	fmt.Fprintln(w, "  command palette")
	for _, item := range commandPaletteItems() {
		fmt.Fprintf(w, "%-10s %s\n", item.Command, item.Help)
	}
}

func RenderCommandPalette(w io.Writer, session Session) {
	renderer := branding.NewRenderer(session.NoColor)
	items := commandPaletteItems()
	fmt.Fprintln(w, renderer.Title("Commands"))
	for index, item := range items {
		prefix := "  "
		line := fmt.Sprintf("%-24s %s", item.Command, item.Help)
		if index == 0 {
			fmt.Fprintln(w, renderer.Accent("> ")+renderer.Value(line))
			continue
		}
		fmt.Fprintln(w, prefix+renderer.Muted(line))
	}
	fmt.Fprintln(w)
}

type commandPaletteItem struct {
	Command string
	Help    string
	Section string
}

func commandPaletteItems() []commandPaletteItem {
	return []commandPaletteItem{
		{Command: "/agents", Help: "Switch agent", Section: "Suggested"},
		{Command: "/models", Help: "Switch model", Section: "Suggested"},
		{Command: "/cache", Help: "Show cache stats", Section: "Suggested"},
		{Command: "/connect", Help: "Connect provider", Section: "Suggested"},
		{Command: "/diff", Help: "Open diff viewer", Section: "Workspace"},
		{Command: "/editor", Help: "Open editor", Section: "Workspace"},
		{Command: "/new", Help: "New session", Section: "Session"},
		{Command: "/help", Help: "Help", Section: "Session"},
		{Command: "/exit", Help: "Exit the app", Section: "Session"},
	}
}

func filterCommandPaletteItems(filter string) []commandPaletteItem {
	filter = strings.ToLower(strings.TrimSpace(filter))
	filter = strings.TrimPrefix(filter, "/")
	if filter == "" {
		return commandPaletteItems()
	}
	var out []commandPaletteItem
	for _, item := range commandPaletteItems() {
		command := strings.ToLower(strings.TrimPrefix(item.Command, "/"))
		help := strings.ToLower(item.Help)
		if strings.Contains(command, filter) || strings.Contains(help, filter) {
			out = append(out, item)
		}
	}
	return out
}

func RenderAgents(w io.Writer, session Session) {
	renderer := branding.NewRenderer(session.NoColor)
	fmt.Fprintln(w, renderer.Title("Agents"))
	for _, agent := range defaultAgentModes() {
		selected := ""
		if agent.ID() == session.Mode {
			selected = "* "
		} else {
			selected = "  "
		}
		fmt.Fprintf(w, "%s%-10s %s\n", renderer.Accent(selected), agent.Name(), renderer.Muted(agent.Description()))
		fmt.Fprintf(w, "  %-10s tools=%s permission=%s worktree=%v\n", "", renderer.Muted(strings.Join(agent.AllowedTools(), ",")), renderer.Muted(string(agent.WritePermission())), agent.UseWorktree())
	}
	fmt.Fprintln(w)
}

func RenderCache(w io.Writer, session Session) {
	renderer := branding.NewRenderer(session.NoColor)
	fmt.Fprintln(w, renderer.Title("Cache"))
	for _, line := range session.CacheReport() {
		fmt.Fprintln(w, line)
	}
	fmt.Fprintln(w)
}

func RenderModels(w io.Writer, session Session) {
	renderer := branding.NewRenderer(session.NoColor)
	fmt.Fprintln(w, renderer.Title("Models"))
	fmt.Fprintf(w, "provider=%s\n", emptyAsUnknown(session.Provider))
	fmt.Fprintf(w, "model=%s\n", emptyAsUnknown(session.Model))
	fmt.Fprintf(w, "api_key_status=%s\n", emptyAsUnknown(session.APIKeyStatus))
	fmt.Fprintf(w, "context=%s\n", session.ContextLabel())
	if session.ReasoningStatusLabel() != "" {
		fmt.Fprintf(w, "reasoning=%s\n", session.ReasoningStatusLabel())
	}
	fmt.Fprintf(w, "pricing=%s\n", FormatCost(ComputeCost(session.Usage, session.Pricing)))
	fmt.Fprintln(w, "available:")
	for _, model := range session.AvailableModels() {
		marker := "  "
		if strings.HasSuffix(model, "/"+session.Model) {
			marker = "* "
		}
		fmt.Fprintf(w, "%s%s\n", marker, model)
	}
	fmt.Fprintln(w, "Use /models <model-name> to switch.")
	fmt.Fprintln(w)
}

func displayMode(mode string) string {
	if selectedMode, ok := agentModeByID(mode); ok {
		return selectedMode.Name()
	}
	return "Build"
}

func emptyAsUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}
