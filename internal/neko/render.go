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
	fmt.Fprintln(w, renderer.Title("MIMO commands"))
	fmt.Fprintln(w, "plain text")
	fmt.Fprintln(w, "  chat with the configured model")
	fmt.Fprintln(w, "/help")
	fmt.Fprintln(w, "/mode single")
	fmt.Fprintln(w, "/mode multi")
	fmt.Fprintln(w, "/model")
	fmt.Fprintln(w, "/models")
	fmt.Fprintln(w, "/model test")
	fmt.Fprintln(w, "/model enrich")
	fmt.Fprintln(w, "/reasoning")
	fmt.Fprintln(w, "/reasoning low|medium|high")
	fmt.Fprintln(w, "/agents")
	fmt.Fprintln(w, "/new")
	fmt.Fprintln(w, "/runs")
	fmt.Fprintln(w, "/run <goal>")
	fmt.Fprintln(w, "  execute an agent task")
	fmt.Fprintln(w, "/preview <worktree_id>")
	fmt.Fprintln(w, "/review <worktree_id>")
	fmt.Fprintln(w, "/discard <worktree_id>")
	fmt.Fprintln(w, "ctrl+p")
	fmt.Fprintln(w, "  cycle reasoning when the current model supports it")
	fmt.Fprintln(w, "/")
	fmt.Fprintln(w, "  command palette")
	fmt.Fprintln(w, "/exit")
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
}

func commandPaletteItems() []commandPaletteItem {
	return []commandPaletteItem{
		{"/run <goal>", "run agent task"},
		{"/agents", "switch agent mode"},
		{"/agents single", "single-agent dry run"},
		{"/agents multi", "multi-agent worktree build"},
		{"/models", "inspect model/provider"},
		{"/models <name>", "switch model"},
		{"/reasoning", "cycle reasoning level"},
		{"/panel diff", "show diff panel"},
		{"/panel editor", "show editor panel"},
		{"/panel off", "hide panel"},
		{"/new", "new session"},
		{"/preview <worktree_id>", "patch preview"},
		{"/review <worktree_id>", "patch review"},
		{"/discard <worktree_id>", "discard worktree"},
		{"/runs", "recent runs"},
		{"/help", "help"},
		{"/exit", "quit"},
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
	agents := []struct {
		Name string
		Mode string
		Help string
	}{
		{"Build", "multi", "multi-agent worktree build"},
		{"Single", "single", "single-agent dry run"},
		{"Review", "review", "patch review via /review"},
	}
	for _, agent := range agents {
		selected := ""
		if agent.Mode == session.Mode {
			selected = "* "
		} else {
			selected = "  "
		}
		fmt.Fprintf(w, "%s%-10s %s\n", renderer.Accent(selected), agent.Name, renderer.Muted(agent.Help))
	}
	fmt.Fprintln(w, renderer.Muted("Use /agents single or /agents multi."))
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
	if mode == "single" {
		return "Single Run"
	}
	return "Multi-Agent"
}

func emptyAsUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}
