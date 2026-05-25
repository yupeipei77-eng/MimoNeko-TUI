package neko

import (
	"fmt"
	"io"
	"strings"
)

const (
	ansiReset        = "\x1b[0m"
	ansiDim          = "\x1b[2m"
	ansiYellow       = "\x1b[33m"
	ansiBrightYellow = "\x1b[93m"
	ansiGreen        = "\x1b[32m"
	ansiRed          = "\x1b[31m"
)

type palette struct {
	enabled bool
}

func (p palette) paint(code, text string) string {
	if !p.enabled {
		return text
	}
	return code + text + ansiReset
}

func (p palette) label(text string) string {
	return p.paint(ansiYellow, text)
}

func (p palette) title(text string) string {
	return p.paint(ansiBrightYellow, text)
}

func (p palette) dim(text string) string {
	return p.paint(ansiDim, text)
}

func (p palette) state(state string) string {
	switch strings.ToLower(state) {
	case "succeeded", "approve", "ok":
		return p.paint(ansiGreen, state)
	case "failed", "rejected", "request_changes":
		return p.paint(ansiRed, state)
	default:
		return state
	}
}

func RenderHeader(w io.Writer, session Session) {
	p := palette{enabled: !session.NoColor}
	fmt.Fprintf(w, "%s\n", p.title(`/\_/\`))
	fmt.Fprintf(w, "%s   %s\n", p.title("( o.o )"), p.title("NekoForge"))
	fmt.Fprintf(w, "%s    %s\n", p.title("> ^ <"), p.dim("powered by ReasonForge"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s %s\n", p.label("Ask your coding cat..."), p.dim(`"Fix broken tests"`))
	fmt.Fprintln(w)
	renderStatusLine(w, p, "Mode", displayMode(session.Mode))
	renderStatusLine(w, p, "Model", emptyAsUnknown(session.Model))
	renderStatusLine(w, p, "Provider", emptyAsUnknown(session.Provider))
	renderStatusLine(w, p, "Context", session.ContextLabel())
	renderStatusLine(w, p, "Reasoning", session.ReasoningLabel())
	renderStatusLine(w, p, "Tokens", FormatTokens(session.Usage))
	renderStatusLine(w, p, "Cost", FormatCost(ComputeCost(session.Usage, session.Pricing)))
	renderStatusLine(w, p, "Safety", fmt.Sprintf("dry-run=%v worktree=%v no auto-apply", session.DryRun, session.Worktree))
	fmt.Fprintln(w)
	fmt.Fprintln(w, p.label("Shortcuts:"))
	fmt.Fprintln(w, "enter    run")
	fmt.Fprintln(w, "/mode    switch mode")
	fmt.Fprintln(w, "/model   show model")
	fmt.Fprintln(w, "/runs    recent runs")
	fmt.Fprintln(w, "/exit    quit")
	fmt.Fprintln(w)
}

func RenderHelp(w io.Writer, noColor bool) {
	p := palette{enabled: !noColor}
	fmt.Fprintln(w, p.title("NekoForge commands"))
	fmt.Fprintln(w, "/help")
	fmt.Fprintln(w, "/mode single")
	fmt.Fprintln(w, "/mode multi")
	fmt.Fprintln(w, "/model")
	fmt.Fprintln(w, "/model test")
	fmt.Fprintln(w, "/model enrich")
	fmt.Fprintln(w, "/reasoning low|medium|high")
	fmt.Fprintln(w, "/runs")
	fmt.Fprintln(w, "/run <goal>")
	fmt.Fprintln(w, "/preview <worktree_id>")
	fmt.Fprintln(w, "/review <worktree_id>")
	fmt.Fprintln(w, "/discard <worktree_id>")
	fmt.Fprintln(w, "/exit")
}

func renderStatusLine(w io.Writer, p palette, label, value string) {
	fmt.Fprintf(w, "%-10s %s\n", p.label(label), value)
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
