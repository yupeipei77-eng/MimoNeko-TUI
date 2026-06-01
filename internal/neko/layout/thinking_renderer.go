package layout

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// ThinkingRenderer handles rendering of the thinking area.
type ThinkingRenderer struct {
	state   *ThinkingState
	NoColor bool
	Delay   time.Duration
}

// NewThinkingRenderer creates a new ThinkingRenderer.
func NewThinkingRenderer(noColor bool) *ThinkingRenderer {
	return &ThinkingRenderer{
		state:   NewThinkingState(noColor),
		NoColor: noColor,
	}
}

// Toggle switches the thinking display state.
func (r *ThinkingRenderer) Toggle() {
	r.state.Toggle()
}

// ShowThoughts returns whether thoughts are shown.
func (r *ThinkingRenderer) ShowThoughts() bool {
	return r.state.ShowThoughts
}

// StartThinking begins the thinking animation.
func (r *ThinkingRenderer) StartThinking() {
	r.state.StartThinking()
}

// StopThinking stops the thinking animation.
func (r *ThinkingRenderer) StopThinking() {
	r.state.StopThinking()
}

// AddThought adds a thought line.
func (r *ThinkingRenderer) AddThought(text string) {
	r.state.AddThought(text)
}

// IsThinking returns whether currently thinking.
func (r *ThinkingRenderer) IsThinking() bool {
	return r.state.IsThinking
}

// RenderThinking renders the thinking area to the writer.
func (r *ThinkingRenderer) RenderThinking(w io.Writer) {
	if !r.state.IsThinking {
		return
	}

	indent := strings.Repeat(" ", DialogPadding)

	if r.state.ShowThoughts {
		r.renderThoughtsShown(w, indent)
	} else {
		r.renderThoughtsHidden(w, indent)
	}
}

// renderThoughtsShown renders the thinking area with full text.
func (r *ThinkingRenderer) renderThoughtsShown(w io.Writer, indent string) {
	// Header with toggle hint
	header := paintMuted("(思考中... 按 Ctrl+Shift+T 隐藏思考过程)", r.NoColor)
	fmt.Fprintf(w, "%s%s\n", indent, header)

	// Thought lines
	if len(r.state.ThoughtText) == 0 {
		fmt.Fprintf(w, "%s%s\n", indent, paintDim("› 等待思考内容...", r.NoColor))
	} else {
		for _, line := range r.state.ThoughtText {
			fmt.Fprintf(w, "%s%s\n", indent, paintDim("› "+line, r.NoColor))
		}
	}

	// Empty line separator before answer
	fmt.Fprintln(w)
}

// renderThoughtsHidden renders the thinking area with dot animation.
func (r *ThinkingRenderer) renderThoughtsHidden(w io.Writer, indent string) {
	dots := r.state.Animator.Dots()
	status := paintMuted(fmt.Sprintf("(思考中...) %s", dots), r.NoColor)
	fmt.Fprintf(w, "%s%s\r", indent, status)
}

// RenderToggleHint renders the toggle hint in the status area.
func (r *ThinkingRenderer) RenderToggleHint(w io.Writer) {
	hint := ThoughtToggleHint(r.state.ShowThoughts, r.NoColor)
	fmt.Fprint(w, hint)
}

// ClearThinkingLine clears the thinking animation line.
func (r *ThinkingRenderer) ClearThinkingLine(w io.Writer) {
	indent := strings.Repeat(" ", DialogPadding)
	fmt.Fprintf(w, "%s%s\r", indent, strings.Repeat(" ", 50))
}

// RenderThinkingSeparator renders a separator between thinking and answer.
func (r *ThinkingRenderer) RenderThinkingSeparator(w io.Writer) {
	fmt.Fprintln(w)
}
