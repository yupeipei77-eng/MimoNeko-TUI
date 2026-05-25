package layout

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

const (
	CanvasWidth   = 104
	DialogWidth   = 78
	DialogPadding = (CanvasWidth - DialogWidth) / 2
	ansiReset     = "\x1b[0m"
	ansiErrorRed  = "\x1b[31m"
)

type Message struct {
	Role  string
	Text  string
	Error bool
}

type MessageRenderer struct {
	history []Message
	NoColor bool
}

func NewMessageRenderer(noColor bool) MessageRenderer {
	return MessageRenderer{NoColor: noColor}
}

func (r *MessageRenderer) SetNoColor(noColor bool) {
	r.NoColor = noColor
}

func (r *MessageRenderer) Add(role, text string) {
	r.add(role, text, false)
}

func (r *MessageRenderer) AddError(role, text string) {
	r.add(role, text, true)
}

func (r *MessageRenderer) add(role, text string, isError bool) {
	if text == "" {
		return
	}
	r.history = append(r.history, Message{Role: role, Text: text, Error: isError})
}

func (r *MessageRenderer) History() []Message {
	out := make([]Message, len(r.history))
	copy(out, r.history)
	return out
}

func (r *MessageRenderer) RenderLast(w io.Writer) {
	if len(r.history) == 0 {
		return
	}
	msg := r.history[len(r.history)-1]
	RenderMessageStyled(w, msg.Role, msg.Text, msg.Error, r.NoColor)
}

func RenderMessage(w io.Writer, role, text string) {
	RenderMessageStyled(w, role, text, false, true)
}

func RenderErrorMessage(w io.Writer, role, text string, noColor bool) {
	RenderMessageStyled(w, role, text, true, noColor)
}

func RenderMessageStyled(w io.Writer, role, text string, isError, noColor bool) {
	if role == "" {
		role = "Message"
	}
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 4
	title := " " + role + " "
	ruleWidth := DialogWidth - 2 - visibleLen(title)
	if ruleWidth < 1 {
		ruleWidth = 1
	}
	writeStyledLine(w, fmt.Sprintf("%s╭%s%s╮\n", indent, title, strings.Repeat("─", ruleWidth)), isError, noColor)
	for _, line := range splitLines(text) {
		for _, wrapped := range wrapLine(line, innerWidth) {
			writeStyledLine(w, fmt.Sprintf("%s│ %s │\n", indent, padRight(wrapped, innerWidth)), isError, noColor)
		}
	}
	writeStyledLine(w, fmt.Sprintf("%s╰%s╯\n", indent, strings.Repeat("─", DialogWidth-2)), isError, noColor)
	fmt.Fprintln(w)
}

func writeStyledLine(w io.Writer, line string, isError, noColor bool) {
	if isError && !noColor {
		fmt.Fprint(w, ansiErrorRed, line, ansiReset)
		return
	}
	fmt.Fprint(w, line)
}

type InputRenderer struct {
	Model     string
	Provider  string
	Reasoning string
}

func NewInputRenderer(model, provider, reasoning string) InputRenderer {
	return InputRenderer{
		Model:     emptyAs(model, "model"),
		Provider:  emptyAs(provider, "provider"),
		Reasoning: emptyAs(reasoning, "reasoning"),
	}
}

func (r InputRenderer) RenderPrompt(w io.Writer) {
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 4
	fmt.Fprintf(w, "%s╭%s╮\n", indent, strings.Repeat("─", DialogWidth-2))
	fmt.Fprintf(w, "%s│ %s │\n", indent, padRight(`Ask anything...  "Read README and summarize it"`, innerWidth))
	r.RenderPromptInput(w)
}

func (r InputRenderer) RenderPromptInput(w io.Writer) {
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 4
	meta := fmt.Sprintf("Chat  ·  %s  ·  %s  ·  %s", r.Model, r.Provider, r.Reasoning)
	fmt.Fprintf(w, "%s│ %s │\n", indent, padRight(meta, innerWidth))
	fmt.Fprintf(w, "%s│ > ", indent)
}

func (InputRenderer) RenderPromptClose(w io.Writer) {
	indent := strings.Repeat(" ", DialogPadding)
	fmt.Fprintf(w, "%s╰%s╯\n\n", indent, strings.Repeat("─", DialogWidth-2))
}

func (InputRenderer) RenderSubmittedPrompt(w io.Writer, input string, rewrite bool) {
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 4
	line := "> " + input
	if rewrite {
		fmt.Fprint(w, "\x1b[1A\r\x1b[2K")
		fmt.Fprintf(w, "%s│ %s │\n", indent, padRight(line, innerWidth))
		return
	}
	remainingWidth := innerWidth - terminalWidth("> ")
	fmt.Fprintf(w, "%s │\n", padRight(input, remainingWidth))
}

func splitLines(text string) []string {
	if text == "" {
		return []string{""}
	}
	var lines []string
	start := 0
	for i, r := range text {
		if r == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	lines = append(lines, text[start:])
	return lines
}

func wrapLine(line string, width int) []string {
	if line == "" {
		return []string{""}
	}
	var out []string
	var current strings.Builder
	currentWidth := 0
	for _, r := range line {
		rw := runeWidth(r)
		if currentWidth > 0 && currentWidth+rw > width {
			out = append(out, current.String())
			current.Reset()
			currentWidth = 0
		}
		if rw > width {
			continue
		}
		current.WriteRune(r)
		currentWidth += rw
	}
	out = append(out, current.String())
	return out
}

func padRight(value string, width int) string {
	value = truncateToWidth(value, width)
	visible := terminalWidth(value)
	if visible >= width {
		return value
	}
	return value + strings.Repeat(" ", width-visible)
}

func visibleLen(value string) int {
	return terminalWidth(value)
}

func truncateToWidth(value string, width int) string {
	var out strings.Builder
	current := 0
	for _, r := range value {
		rw := runeWidth(r)
		if current+rw > width {
			break
		}
		out.WriteRune(r)
		current += rw
	}
	return out.String()
}

func terminalWidth(value string) int {
	width := 0
	for _, r := range value {
		width += runeWidth(r)
	}
	return width
}

func runeWidth(r rune) int {
	if r == '\t' {
		return 4
	}
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	if isWideRune(r) {
		return 2
	}
	return 1
}

func isWideRune(r rune) bool {
	if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
		return true
	}
	return (r >= 0x1100 && r <= 0x115f) ||
		(r >= 0x2329 && r <= 0x232a) ||
		(r >= 0x2e80 && r <= 0xa4cf) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6)
}

func emptyAs(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
