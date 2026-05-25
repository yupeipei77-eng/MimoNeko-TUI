package neko

import (
	"fmt"
	"io"
	"strings"

	"github.com/reasonforge/reasonforge/internal/neko/branding"
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
		Reasoning: session.ReasoningLabel(),
		Tokens:    FormatTokens(session.Usage),
		Cost:      FormatCost(ComputeCost(session.Usage, session.Pricing)),
		Safety:    fmt.Sprintf("dry-run=%v worktree=%v no auto-apply", session.DryRun, session.Worktree),
	}
}

func RenderHelp(w io.Writer, noColor bool) {
	renderer := branding.NewRenderer(noColor)
	fmt.Fprintln(w, renderer.Title("NekoForge commands"))
	fmt.Fprintln(w, "plain text")
	fmt.Fprintln(w, "  chat with the configured model")
	fmt.Fprintln(w, "/help")
	fmt.Fprintln(w, "/mode single")
	fmt.Fprintln(w, "/mode multi")
	fmt.Fprintln(w, "/model")
	fmt.Fprintln(w, "/model test")
	fmt.Fprintln(w, "/model enrich")
	fmt.Fprintln(w, "/reasoning low|medium|high")
	fmt.Fprintln(w, "/runs")
	fmt.Fprintln(w, "/run <goal>")
	fmt.Fprintln(w, "  execute an agent task")
	fmt.Fprintln(w, "/preview <worktree_id>")
	fmt.Fprintln(w, "/review <worktree_id>")
	fmt.Fprintln(w, "/discard <worktree_id>")
	fmt.Fprintln(w, "/exit")
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
