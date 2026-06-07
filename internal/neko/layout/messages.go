package layout

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

const (
	CanvasWidth   = 132
	DialogWidth   = 112
	DialogPadding = (CanvasWidth - DialogWidth) / 2
)

var layoutANSIEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

type Message struct {
	Role  string
	Text  string
	Error bool
}

type MessageRenderer struct {
	history []Message
	NoColor bool
	Delay   time.Duration
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

func (r *MessageRenderer) Reset() {
	r.history = nil
}

func (r *MessageRenderer) RenderLast(w io.Writer) {
	if len(r.history) == 0 {
		return
	}
	msg := r.history[len(r.history)-1]
	RenderMessageStyled(w, msg.Role, msg.Text, msg.Error, r.NoColor, r.Delay)
}

func RenderMessage(w io.Writer, role, text string) {
	RenderMessageStyled(w, role, text, false, true, 0)
}

func RenderErrorMessage(w io.Writer, role, text string, noColor bool) {
	RenderMessageStyled(w, role, text, true, noColor, 0)
}

func RenderMessageStyled(w io.Writer, role, text string, isError, noColor bool, delay time.Duration) {
	if role == "" {
		role = "Message"
	}
	if isUserRole(role) {
		renderUserBlock(w, text, noColor)
		return
	}
	renderAssistantBlock(w, role, text, isError, noColor, delay)
}

func renderUserBlock(w io.Writer, text string, noColor bool) {
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 6

	// User label with accent icon
	fmt.Fprintf(w, "%s%s %s\n", indent, paintUserAccent("▸", noColor), paintBold("You", noColor))

	// Message content with subtle cyan-tinted background
	for _, line := range splitLines(text) {
		for _, wrapped := range wrapLine(line, innerWidth) {
			display := truncateToWidth(wrapped, innerWidth)
			panel := "  " + padRight(display, innerWidth+2)
			fmt.Fprintf(w, "%s%s\n", indent, paintUserPanel(panel, noColor))
		}
	}
	fmt.Fprintln(w)
}

func renderAssistantBlock(w io.Writer, role, text string, isError, noColor bool, delay time.Duration) {
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 2
	label := emptyAs(role, "MimoNeko")

	// Assistant label with icon
	if isError {
		fmt.Fprintf(w, "%s%s %s\n", indent, paintError("✗", noColor), paintBold(label, noColor))
	} else {
		fmt.Fprintf(w, "%s%s %s\n", indent, paintAccent("●", noColor), paintMuted(label, noColor))
	}

	// Message content
	for _, line := range splitLines(text) {
		wrappedLines := wrapLine(line, innerWidth)
		for _, wrapped := range wrappedLines {
			display := truncateToWidth(wrapped, innerWidth)
			fmt.Fprint(w, indent)
			writeStreamingText(w, display, isError, noColor, delay)
			fmt.Fprintln(w)
		}
	}
	fmt.Fprintln(w)
}

func isUserRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	return role == "user" || role == "you"
}

func writeStyledLine(w io.Writer, line string, isError, noColor bool) {
	if isError && !noColor {
		fmt.Fprint(w, paintError(line, noColor))
		return
	}
	fmt.Fprint(w, line)
}

func writeStreamingText(w io.Writer, text string, isError, noColor bool, delay time.Duration) {
	if delay <= 0 {
		writeStyledLine(w, text, isError, noColor)
		return
	}
	for _, r := range text {
		writeStyledLine(w, string(r), isError, noColor)
		time.Sleep(delay)
	}
}

type InputRenderer struct {
	Model             string
	Provider          string
	Reasoning         string
	Context           string
	Cost              string
	Tools             int
	Memory            string
	Cache             string
	Latency           string
	Session           string
	CommandUI         string
	ThoughtToggleHint string
	NoColor           bool
}

func NewInputRenderer(model, provider, reasoning string) InputRenderer {
	return InputRenderer{
		Model:     emptyAs(model, "model"),
		Provider:  emptyAs(provider, "provider"),
		Reasoning: strings.TrimSpace(reasoning),
	}
}

func (r InputRenderer) RenderPrompt(w io.Writer) {
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 5

	// Top border with accent corners
	fmt.Fprintf(w, "%s%s%s%s\n", indent,
		paintAccent("╭", r.NoColor),
		paintMuted(strings.Repeat("─", innerWidth+2), r.NoColor),
		paintAccent("╮", r.NoColor))

	// Placeholder text with icon
	placeholderText := "Ask anything..."
	exampleText := `"Fix broken tests"`
	placeholder := padRight("  "+placeholderText+"  "+exampleText, innerWidth)
	fmt.Fprintf(w, "%s%s %s %s\n", indent,
		paintAccent("│", r.NoColor),
		paintDim(placeholder, r.NoColor),
		paintAccent("│", r.NoColor))

	r.RenderPromptInput(w)
}

func (r InputRenderer) RenderPromptInput(w io.Writer) {
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 5
	parts := []string{"Build", r.Model, r.Provider}
	if strings.TrimSpace(r.Reasoning) != "" {
		parts = append(parts, strings.TrimSpace(r.Reasoning))
	}
	meta := strings.Join(parts, " · ")

	// Meta line with subtle styling
	fmt.Fprintf(w, "%s%s %s %s\n", indent,
		paintAccent("│", r.NoColor),
		paintMuted(padRight("  "+meta, innerWidth), r.NoColor),
		paintAccent("│", r.NoColor))

	// Input line prompt.
	promptSymbol := paintAccent("▸", r.NoColor)
	fmt.Fprintf(w, "%s%s %s %s", indent,
		paintAccent("│", r.NoColor),
		paintPanel(padRight("  "+promptSymbol+" ", innerWidth), r.NoColor),
		paintAccent("│", r.NoColor))
}

func (r InputRenderer) RenderPromptClose(w io.Writer) {
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 5

	// Bottom border
	fmt.Fprintf(w, "%s%s%s%s\n", indent,
		paintAccent("╰", r.NoColor),
		paintMuted(strings.Repeat("─", innerWidth+2), r.NoColor),
		paintAccent("╯", r.NoColor))

	RenderStatusBar(w, StatusData{
		Context:           r.Context,
		Tools:             r.Tools,
		Memory:            r.Memory,
		Cache:             r.Cache,
		Reasoning:         r.Reasoning,
		Model:             r.Model,
		Provider:          r.Provider,
		Latency:           r.Latency,
		Session:           r.Session,
		Cost:              r.Cost,
		NoColor:           r.NoColor,
		CommandUI:         r.CommandUI,
		ThoughtToggleHint: r.ThoughtToggleHint,
	})
	fmt.Fprintln(w)
}

func (r InputRenderer) RenderSubmittedPrompt(w io.Writer, input string, rewrite bool) {
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 5
	line := "  > " + input
	if rewrite {
		fmt.Fprint(w, "\x1b[1A\r\x1b[2K")
		fmt.Fprintf(w, "%s%s%s\n", indent, paintLabel("▌", r.NoColor), paintPanel(padRight(line, innerWidth), r.NoColor))
		return
	}
	remainingWidth := innerWidth - terminalWidth("  > ")
	fmt.Fprintf(w, "%s\n", padRight(input, remainingWidth))
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
	for i := 0; i < len(value); {
		if escapeLen := layoutANSIEscapeLen(value, i); escapeLen > 0 {
			out.WriteString(value[i : i+escapeLen])
			i += escapeLen
			continue
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && size == 0 {
			break
		}
		rw := runeWidth(r)
		if current+rw > width {
			break
		}
		out.WriteRune(r)
		current += rw
		i += size
	}
	return out.String()
}

func terminalWidth(value string) int {
	return runewidth.StringWidth(stripLayoutANSI(value))
}

func runeWidth(r rune) int {
	if r == '\t' {
		return 4
	}
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	return runewidth.RuneWidth(r)
}

func stripLayoutANSI(value string) string {
	if value == "" {
		return ""
	}
	return layoutANSIEscapePattern.ReplaceAllString(value, "")
}

func layoutANSIEscapeLen(value string, start int) int {
	if start >= len(value) || value[start] != '\x1b' || start+1 >= len(value) || value[start+1] != '[' {
		return 0
	}
	for i := start + 2; i < len(value) && i-start <= 32; i++ {
		if value[i] >= 0x40 && value[i] <= 0x7e {
			return i - start + 1
		}
	}
	return 0
}

func emptyAs(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
