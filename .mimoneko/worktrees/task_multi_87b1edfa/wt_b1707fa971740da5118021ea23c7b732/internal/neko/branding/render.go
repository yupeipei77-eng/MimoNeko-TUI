package branding

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	HeaderHeight  = 4
	CanvasWidth   = 104
	DialogWidth   = 78
	DialogPadding = (CanvasWidth - DialogWidth) / 2
)

type HeaderData struct {
	Mode      string
	Model     string
	Provider  string
	Context   string
	Reasoning string
	Tokens    string
	Cost      string
	Safety    string
}

func (r Renderer) RenderStaticHeader(w io.Writer, data HeaderData) {
	r.RenderAnimatedHeader(w, data, 0)
}

func (r Renderer) RenderAnimatedHeader(w io.Writer, data HeaderData, frame int) {
	_ = frame
	r.renderMinimalHero(w, data)
}

func (r Renderer) RenderNoColorHeader(w io.Writer, data HeaderData) {
	Renderer{NoColor: true}.RenderStaticHeader(w, data)
}

func (r Renderer) RenderHeaderToString(data HeaderData, frame int) string {
	var buf bytes.Buffer
	r.RenderAnimatedHeader(&buf, data, frame)
	return buf.String()
}

func HeaderLineCount() int {
	return HeaderHeight
}

func (r Renderer) renderMinimalHero(w io.Writer, data HeaderData) {
	fmt.Fprintln(w)
	r.centerLine(w, r.Accent("Neko")+r.Title("Forge"), len("NekoForge"))
	fmt.Fprintln(w)
	_ = data
	fmt.Fprintln(w)
}

func (r Renderer) renderDialog(w io.Writer, data HeaderData) {
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 4
	fmt.Fprintf(w, "%s%s\n", indent, r.Label("╭"+strings.Repeat("─", DialogWidth-2)+"╮"))
	fmt.Fprintf(w, "%s%s %s %s\n", indent, r.Label("│"), r.Value(padRight(`Ask anything...  "Read README and summarize it"`, innerWidth)), r.Label("│"))
	meta := fmt.Sprintf("Chat  ·  %s  ·  %s  ·  %s", data.Model, data.Provider, data.Reasoning)
	fmt.Fprintf(w, "%s%s %s %s\n", indent, r.Label("│"), r.Muted(padRight(meta, innerWidth)), r.Label("│"))
	actions := "/run agent task   /model provider   /help commands   /exit quit"
	fmt.Fprintf(w, "%s%s %s %s\n", indent, r.Label("│"), r.Muted(padRight(actions, innerWidth)), r.Label("│"))
	fmt.Fprintf(w, "%s%s\n", indent, r.Label("╰"+strings.Repeat("─", DialogWidth-2)+"╯"))
}

func (r Renderer) centerLine(w io.Writer, styled string, visibleLen int) {
	padding := (CanvasWidth - visibleLen) / 2
	if padding < 0 {
		padding = 0
	}
	fmt.Fprintf(w, "%s%s\n", strings.Repeat(" ", padding), styled)
}

func padRight(value string, width int) string {
	visible := utf8.RuneCountInString(value)
	if visible >= width {
		runes := []rune(value)
		return string(runes[:width])
	}
	return value + strings.Repeat(" ", width-visible)
}
