package branding

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

const HeaderHeight = 23

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
	r.renderBrandBlock(w, frame)
	r.renderSessionBlock(w, data)
	r.renderShortcutBlock(w)
	fmt.Fprintln(w)
}

func (r Renderer) RenderNoColorHeader(w io.Writer, data HeaderData) {
	Renderer{NoColor: true}.RenderStaticHeader(w, data)
}

func (r Renderer) RenderHeaderToString(data HeaderData, frame int) string {
	var buf bytes.Buffer
	r.RenderAnimatedHeader(&buf, data, frame)
	return buf.String()
}

func (r Renderer) renderBrandBlock(w io.Writer, frame int) {
	lines := CatFrameLines(frame)
	brand := []string{
		r.Title("NekoForge"),
		r.Muted("local AI coding workspace  |  powered by ReasonForge"),
		r.Value("floating local coding familiar"),
	}
	for i := 0; i < len(lines); i++ {
		fmt.Fprintf(w, "  %s   %s\n", r.Accent(lines[i]), brand[i])
	}
	fmt.Fprintf(w, "  %s\n", r.Label(strings.Repeat("-", 58)))
	r.renderField(w, "Ask", "type a goal, or /help")
	fmt.Fprintln(w)
}

func (r Renderer) renderSessionBlock(w io.Writer, data HeaderData) {
	fmt.Fprintf(w, "  %s\n", r.Label("Session"))
	r.renderField(w, "Mode", data.Mode)
	r.renderField(w, "Model", data.Model)
	r.renderField(w, "Provider", data.Provider)
	r.renderField(w, "Context", data.Context)
	r.renderField(w, "Reasoning", data.Reasoning)
	r.renderField(w, "Tokens", data.Tokens)
	r.renderField(w, "Cost", data.Cost)
	r.renderField(w, "Safety", data.Safety)
	fmt.Fprintln(w)
}

func (r Renderer) renderShortcutBlock(w io.Writer) {
	fmt.Fprintf(w, "  %s\n", r.Label("Shortcuts"))
	r.renderField(w, "enter", "run")
	r.renderField(w, "/mode", "switch mode")
	r.renderField(w, "/model", "show model")
	r.renderField(w, "/runs", "recent runs")
	r.renderField(w, "/exit", "quit")
}

func (r Renderer) renderField(w io.Writer, label, value string) {
	padding := 10 - len(label)
	if padding < 1 {
		padding = 1
	}
	fmt.Fprintf(w, "  %s%s %s\n", r.Label(label), strings.Repeat(" ", padding), r.Value(value))
}

func HeaderLineCount() int {
	return HeaderHeight
}
