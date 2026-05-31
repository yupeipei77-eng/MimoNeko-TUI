package neko

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/mimoneko/mimoneko/internal/neko/branding"
)

const (
	defaultScreenCols = 132
	defaultScreenRows = 36
	minComposerWidth  = 74
	maxComposerWidth  = 150
)

type screenLine struct {
	Kind  string
	Text  string
	Error bool
}

func (c *Console) setupScreen() {
	if c.Session.NoColor {
		return
	}
	file, ok := c.Options.Out.(*os.File)
	if !ok {
		return
	}
	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return
	}
	input, ok := c.Options.In.(*os.File)
	if !ok || !isTerminalFile(input) {
		return
	}
	c.screenActive = true
	c.screenCols, c.screenRows = screenSize()
	c.enterWorkspace()
}

func screenSize() (int, int) {
	cols, rows, ok := terminalSize()
	if !ok {
		return defaultScreenCols, defaultScreenRows
	}
	if cols < 80 {
		cols = 80
	}
	if rows < 20 {
		rows = 20
	}
	return cols, rows
}

func (c *Console) repaintScreen() {
	if !c.screenActive {
		return
	}
	c.screenCols, c.screenRows = screenSize()
	var out bytes.Buffer
	renderer := branding.NewRenderer(false)
	width := c.composerWidth()
	left := (c.screenCols - width) / 2
	if left < 1 {
		left = 1
	}
	if c.introActive {
		row, col := c.renderIntroScreen(&out, renderer)
		fmt.Fprintf(&out, "\x1b[%d;%dH", row, col)
		fmt.Fprint(c.Options.Out, out.String())
		return
	}
	composerTop := c.screenRows - 4
	if composerTop < 12 {
		composerTop = c.screenRows - 4
	}
	messageTop := 5
	messageBottom := composerTop - 2
	panelTop, panelBottom := c.screenPanelBounds(composerTop)
	paletteTop, paletteBottom := c.screenPaletteBounds(composerTop, panelTop)
	if panelTop > 0 {
		messageBottom = panelTop - 2
	}
	if paletteTop > 0 && paletteTop-2 < messageBottom {
		messageBottom = paletteTop - 2
	}
	if messageBottom < messageTop {
		messageBottom = messageTop
	}

	fmt.Fprint(&out, "\x1b[H\x1b[2J")
	c.renderScreenHeader(&out, renderer, left, width)
	c.renderScreenMessages(&out, renderer, left, width, messageTop, messageBottom)
	if panelTop > 0 {
		c.renderScreenPanel(&out, renderer, left, width, panelTop, panelBottom)
	}
	if paletteTop > 0 {
		c.renderScreenPalette(&out, renderer, left, width, paletteTop, paletteBottom)
	}
	c.renderScreenComposer(&out, renderer, left, width, composerTop)
	cursorCol := left + screenWidth("╰> "+c.visibleDraft(width-6))
	if cursorCol < 1 {
		cursorCol = 1
	}
	fmt.Fprintf(&out, "\x1b[%d;%dH", composerTop+2, cursorCol)
	fmt.Fprint(c.Options.Out, out.String())
}

func (c *Console) composerWidth() int {
	width := c.screenCols - 8
	if width < minComposerWidth {
		width = minComposerWidth
	}
	if width > maxComposerWidth {
		width = maxComposerWidth
	}
	return width
}

func (c *Console) renderScreenHeader(w io.Writer, renderer branding.Renderer, left, width int) {
	// Cat mascot with better styling
	catLine1 := renderer.Accent(`  /\_/\  `)
	catLine2 := renderer.Accent(` ( `) + renderer.TitleBold("o.o") + renderer.Accent(` ) `)
	catLine3 := renderer.Accent(`  > `) + renderer.AccentBold("^") + renderer.Accent(` <  `)

	// Brand name
	brand := renderer.TitleBold("M") + renderer.Accent("IMO") + renderer.Muted("Neko")

	catWidth := 9
	brandWidth := 8
	catCol := left + (width-catWidth)/2
	brandCol := left + (width-brandWidth)/2

	if catCol < 1 {
		catCol = 1
	}
	if brandCol < 1 {
		brandCol = 1
	}

	// Render cat mascot
	fmt.Fprintf(w, "\x1b[1;%dH%s", catCol, catLine1)
	fmt.Fprintf(w, "\x1b[2;%dH%s", catCol, catLine2)
	fmt.Fprintf(w, "\x1b[3;%dH%s", catCol, catLine3)

	// Render brand name
	fmt.Fprintf(w, "\x1b[4;%dH%s", brandCol, brand)

	// Render model info if available
	if c.Session.Model != "" {
		modelInfo := renderer.Muted(c.Session.Model + " · " + c.Session.Provider)
		infoWidth := len(c.Session.Model) + len(c.Session.Provider) + 3
		infoCol := left + (width-infoWidth)/2
		if infoCol < 1 {
			infoCol = 1
		}
		fmt.Fprintf(w, "\x1b[5;%dH%s", infoCol, modelInfo)
	}
}

func (c *Console) renderIntroScreen(w io.Writer, renderer branding.Renderer) (int, int) {
	width := c.screenCols - 18
	if width < 64 {
		width = 64
	}
	if width > 104 {
		width = 104
	}
	left := (c.screenCols - width) / 2
	if left < 2 {
		left = 2
	}
	inner := width - 4
	top := c.screenRows/2 - 4
	if top < 6 {
		top = 6
	}

	brand := renderer.Accent("( MIMO )~")
	brandCol := left + (width-9)/2
	if brandCol < 1 {
		brandCol = 1
	}
	fmt.Fprintf(w, "\x1b[%d;%dH%s", max(2, top-4), brandCol, brand)
	meta := c.introMetaLine()
	metaCol := left + (width-screenWidth(meta))/2
	if metaCol < 1 {
		metaCol = 1
	}
	fmt.Fprintf(w, "\x1b[%d;%dH%s", max(3, top-3), metaCol, renderer.Muted(meta))

	if c.paletteOpen {
		paletteTop, paletteBottom := c.screenPaletteBounds(top, 0)
		if paletteTop > 1 {
			c.renderScreenPalette(w, renderer, left, width, paletteTop, paletteBottom)
		}
	}

	title := "Ask MIMO"
	prompt := "> " + c.visibleDraft(inner-screenWidth("> "))
	footer := c.introFooter()
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top, left, screenBoxTop(renderer, title, inner))
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top+1, left, screenBoxLine(renderer, "", inner))
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top+2, left, screenBoxLine(renderer, prompt, inner))
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top+3, left, screenBoxLine(renderer, "", inner))
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top+4, left, screenBoxLine(renderer, footer, inner))
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top+5, left, screenBoxBottom(renderer, inner))
	cursorRow := top + 2
	cursorCol := left + screenWidth("╰> "+c.visibleDraft(inner-screenWidth("> ")))
	return cursorRow, cursorCol
}

func (c *Console) introMetaLine() string {
	parts := []string{c.Session.Model, c.Session.Provider}
	if reasoning := c.Session.ReasoningStatusLabel(); reasoning != "" {
		parts = append(parts, "reasoning "+reasoning)
	}
	return strings.Join(parts, " · ")
}

func (c *Console) introFooter() string {
	if c.Session.ReasoningAvailable {
		return "/ commands  ·  ctrl+p reasoning"
	}
	return "/ commands"
}

func (c *Console) renderScreenMessages(w io.Writer, renderer branding.Renderer, left, width, top, bottom int) {
	lines := c.screenVisualLines(renderer, width)
	maxLines := bottom - top + 1
	if maxLines < 1 {
		return
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	row := top
	for _, line := range lines {
		if row > bottom {
			break
		}
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, left, line)
		row++
	}
}

func (c *Console) renderScreenComposer(w io.Writer, renderer branding.Renderer, left, width, top int) {
	inner := width - 4
	if inner < 20 {
		inner = 20
	}
	metaParts := []string{"Build", c.Session.Model, c.Session.Provider}
	if reasoning := c.Session.ReasoningStatusLabel(); reasoning != "" {
		metaParts = append(metaParts, reasoning)
	}
	meta := strings.Join(metaParts, " · ")
	status := c.screenStatusLine(width)
	prompt := "> " + c.visibleDraft(inner-screenWidth("> "))
	title := `Ask anything...  "Fix broken tests"`
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top, left, screenBoxTop(renderer, title, inner))
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top+1, left, screenBoxLine(renderer, meta, inner))
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top+2, left, screenBoxLine(renderer, prompt, inner))
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top+3, left, screenStatusBox(renderer, status, inner))
}

func (c *Console) visibleDraft(width int) string {
	if width <= 0 {
		return ""
	}
	return screenTail(c.draft, width)
}

func (c *Console) screenStatusLine(width int) string {
	c.refreshInput()
	renderer := branding.NewRenderer(false)

	// Build status items with colors
	statusItems := []string{
		renderer.Muted("ctx") + " " + renderer.Value(c.Input.Context),
		renderer.Muted("cache") + " " + renderer.Value(c.Input.Cache),
		renderer.Muted("tools") + " " + renderer.Value(fmt.Sprintf("%d", c.Input.Tools)),
		renderer.Muted("memory") + " " + renderer.Value(c.Input.Memory),
		renderer.Muted("model") + " " + renderer.Accent(c.Input.Model),
		renderer.Muted("provider") + " " + renderer.Label(c.Input.Provider),
	}

	if strings.TrimSpace(c.Input.Reasoning) != "" {
		statusItems = append(statusItems, renderer.Muted("reasoning")+" "+renderer.Value(strings.TrimSpace(c.Input.Reasoning)))
	}
	if c.Input.Latency != "" {
		statusItems = append(statusItems, renderer.Muted("latency")+" "+renderer.Paint(branding.SuccessGreen, c.Input.Latency))
	}
	if c.Input.Session != "" {
		statusItems = append(statusItems, renderer.Muted("session")+" "+renderer.Muted(c.Input.Session))
	}
	if c.Input.Cost != "" {
		statusItems = append(statusItems, renderer.Muted("cost")+" "+renderer.Value(c.Input.Cost))
	}

	status := strings.Join(statusItems, "  ")
	right := c.Input.CommandUI
	if strings.TrimSpace(right) == "" {
		right = renderer.Muted("/ commands")
	}
	inner := width - 4
	if inner < 0 {
		inner = 0
	}
	available := inner - screenWidth(right) - 2
	if available < 0 {
		available = 0
	}
	if screenWidth(status) > available {
		return screenPadRight(screenTruncate(status, inner), inner)
	}
	status = screenTruncate(status, available)
	return status + strings.Repeat(" ", max(1, inner-screenWidth(status)-screenWidth(right))) + right
}

func (c *Console) screenVisualLines(renderer branding.Renderer, width int) []string {
	contentWidth := width - 5
	if contentWidth < 20 {
		contentWidth = 20
	}
	var lines []string
	for _, item := range c.screenLog {
		switch item.Kind {
		case "user":
			// User message with cyan accent
			lines = append(lines, renderer.Cyan("▸")+renderer.Bold("You"))
			for _, line := range wrapScreenText(item.Text, contentWidth) {
				lines = append(lines, renderer.Cyan("╰")+screenUserPanel(" "+screenPadRight(line, contentWidth), false))
			}
			lines = append(lines, "")
		case "assistant":
			// AI message with warm accent
			label := renderer.Muted("MIMO")
			if item.Error {
				label = renderer.Paint(branding.FailureRed, "! MIMO")
			} else {
				label = renderer.Accent("●")+renderer.Muted(" MIMO")
			}
			lines = append(lines, label)
			lines = append(lines, renderAssistantScreenText(renderer, item.Text, item.Error, contentWidth)...)
			lines = append(lines, "")
		case "assistant_stream":
			lines = append(lines, renderer.Accent("●")+renderer.Muted(" MIMO"))
			lines = append(lines, renderAssistantScreenText(renderer, item.Text, false, contentWidth)...)
		case "runtime":
			lines = append(lines, renderer.Accent("●")+renderer.Muted(" ")+renderer.Muted(item.Text))
		case "done":
			lines = append(lines, renderer.Paint(branding.SuccessGreen, "✓")+renderer.Muted(" ")+renderer.Muted(item.Text))
		case "thought":
			lines = append(lines, renderer.Accent("+")+renderer.Muted(" ")+renderer.Muted(item.Text))
		case "build":
			lines = append(lines, renderer.Label("◆")+renderer.Muted(" ")+renderer.Title("Build")+renderer.Muted(" · ")+renderer.Muted(item.Text))
			lines = append(lines, "")
		case "error":
			lines = append(lines, renderer.Paint(branding.FailureRed, "! ")+renderer.Paint(branding.FailureRed, screenTruncate(item.Text, contentWidth)))
			lines = append(lines, "")
		default:
			for _, line := range wrapScreenText(item.Text, contentWidth) {
				lines = append(lines, line)
			}
			lines = append(lines, "")
		}
	}
	return lines
}

func (c *Console) screenPanelBounds(composerTop int) (int, int) {
	if c.panelMode == "" {
		return 0, 0
	}
	height := 7
	top := composerTop - height - 1
	if top < 5 {
		top = 5
	}
	bottom := top + height - 1
	if bottom >= composerTop {
		bottom = composerTop - 1
	}
	if bottom < top {
		return 0, 0
	}
	return top, bottom
}

func (c *Console) screenPaletteBounds(composerTop, panelTop int) (int, int) {
	if !c.paletteOpen {
		return 0, 0
	}
	items := c.visibleCommandPaletteItems()
	height := len(items) + 2
	if len(items) == 0 {
		height = 3
	}
	if height > 10 {
		height = 10
	}
	anchor := composerTop
	if panelTop > 0 {
		anchor = panelTop
	}
	top := anchor - height - 1
	if top < 5 {
		top = 5
	}
	bottom := top + height - 1
	if bottom >= anchor {
		bottom = anchor - 1
	}
	if bottom < top {
		return 0, 0
	}
	return top, bottom
}

func (c *Console) renderScreenPanel(w io.Writer, renderer branding.Renderer, left, width, top, bottom int) {
	title := c.panelTitle
	if title == "" {
		title = strings.Title(c.panelMode)
	}
	content := c.panelContent
	if c.panelMode == "editor" {
		if strings.TrimSpace(c.draft) != "" {
			content = c.draft
		} else if strings.TrimSpace(content) == "" {
			content = "Draft buffer"
		}
	}
	inner := width - 4
	if inner < 20 {
		inner = 20
	}
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top, left, screenBoxTop(renderer, title, inner))
	row := top + 1
	for _, line := range wrapScreenText(content, inner) {
		if row >= bottom {
			break
		}
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, left, screenBoxLine(renderer, line, inner))
		row++
	}
	for row < bottom {
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, left, screenBoxLine(renderer, "", inner))
		row++
	}
	fmt.Fprintf(w, "\x1b[%d;%dH%s", bottom, left, screenBoxBottom(renderer, inner))
}

func (c *Console) renderScreenPalette(w io.Writer, renderer branding.Renderer, left, width, top, bottom int) {
	items := c.visibleCommandPaletteItems()
	inner := width - 4
	if inner < 30 {
		inner = 30
	}
	title := "Commands"
	if strings.TrimSpace(c.paletteFilter) != "" {
		title += " " + c.paletteFilter
	}
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top, left, screenBoxTop(renderer, title, inner))
	row := top + 1
	if len(items) == 0 && row <= bottom {
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, left, screenBoxLine(renderer, "no matching commands", inner))
		if row+1 <= bottom {
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row+1, left, screenBoxBottom(renderer, inner))
		}
		return
	}
	for index, item := range items {
		if row >= bottom {
			break
		}
		marker := " "
		line := fmt.Sprintf("%-22s %s", item.Command, item.Help)
		if index == c.paletteSelected {
			marker = ">"
			selected := screenSelected(marker+" "+screenPadRight(line, inner-2), false)
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row, left, screenBoxLineRaw(renderer, selected, inner))
		} else {
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row, left, screenBoxLine(renderer, marker+" "+line, inner))
		}
		row++
	}
	for row < bottom {
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, left, screenBoxLine(renderer, "", inner))
		row++
	}
	fmt.Fprintf(w, "\x1b[%d;%dH%s", bottom, left, screenBoxBottom(renderer, inner))
}

func renderAssistantScreenText(renderer branding.Renderer, text string, isError bool, width int) []string {
	var out []string
	inCode := false
	codeLine := 1
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCode = !inCode
			if inCode {
				out = append(out, screenPanel(" # code"+strings.Repeat(" ", max(0, width-7)), false))
				codeLine = 1
			}
			continue
		}
		if inCode {
			prefix := fmt.Sprintf("%3d ", codeLine)
			codeLine++
			for _, wrapped := range wrapScreenText(line, width-len(prefix)-1) {
				out = append(out, screenPanel(" "+prefix+screenPadRight(wrapped, width-len(prefix)-1), false))
			}
			continue
		}
		for _, wrapped := range wrapScreenText(line, width) {
			if isError {
				out = append(out, renderer.Paint(branding.FailureRed, wrapped))
			} else {
				out = append(out, renderer.Title(wrapped))
			}
		}
	}
	return out
}

func screenPanel(text string, noColor bool) string {
	if noColor {
		return text
	}
	return "\x1b[48;5;235m" + text + branding.Reset
}

func screenUserPanel(text string, noColor bool) string {
	if noColor {
		return text
	}
	return "\x1b[48;5;237m" + text + branding.Reset
}

func screenSelected(text string, noColor bool) string {
	if noColor {
		return text
	}
	return "\x1b[38;5;231m\x1b[48;5;24m" + text + branding.Reset
}

func screenBoxTop(renderer branding.Renderer, title string, inner int) string {
	title = " " + strings.TrimSpace(title) + " "
	fill := inner - screenWidth(title)
	if fill < 0 {
		title = screenTruncate(title, inner)
		fill = 0
	}
	return renderer.BorderAccent(branding.BoxTopLeft+branding.BoxHorizontal) +
		renderer.TitleBold(title) +
		renderer.Border(strings.Repeat(branding.BoxHorizontal, fill)) +
		renderer.BorderAccent(branding.BoxHorizontal+branding.BoxTopRight)
}

func screenBoxBottom(renderer branding.Renderer, inner int) string {
	return renderer.BorderAccent(branding.BoxBottomLeft) +
		renderer.Border(strings.Repeat(branding.BoxHorizontal, inner+2)) +
		renderer.BorderAccent(branding.BoxBottomRight)
}

func screenBoxLine(renderer branding.Renderer, content string, inner int) string {
	return renderer.BorderAccent(branding.BoxVertical) + " " +
		screenPanel(screenPadRight(content, inner), false) + " " +
		renderer.BorderAccent(branding.BoxVertical)
}

func screenBoxLineRaw(renderer branding.Renderer, content string, inner int) string {
	return renderer.BorderAccent(branding.BoxVertical) + " " + content + " " +
		renderer.BorderAccent(branding.BoxVertical)
}

func screenStatusBox(renderer branding.Renderer, status string, inner int) string {
	status = screenPadRight(status, inner)
	return renderer.BorderAccent(branding.BoxTopLeft) +
		renderer.Muted(status) +
		renderer.BorderAccent(" " + branding.BoxTopRight)
}

func wrapScreenText(text string, width int) []string {
	if text == "" {
		return []string{""}
	}
	var out []string
	var current strings.Builder
	currentWidth := 0
	for _, r := range text {
		rw := screenRuneWidth(r)
		if currentWidth > 0 && currentWidth+rw > width {
			out = append(out, current.String())
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += rw
	}
	out = append(out, current.String())
	return out
}

func screenPadRight(value string, width int) string {
	value = screenTruncate(value, width)
	visible := screenWidth(value)
	if visible >= width {
		return value
	}
	return value + strings.Repeat(" ", width-visible)
}

func screenTruncate(value string, width int) string {
	var out strings.Builder
	current := 0
	for _, r := range value {
		rw := screenRuneWidth(r)
		if current+rw > width {
			break
		}
		out.WriteRune(r)
		current += rw
	}
	return out.String()
}

func screenTail(value string, width int) string {
	if screenWidth(value) <= width {
		return value
	}
	input := []rune(value)
	var tail []rune
	current := 0
	for i := len(input) - 1; i >= 0; i-- {
		r := input[i]
		rw := screenRuneWidth(r)
		if current+rw > width {
			break
		}
		tail = append(tail, r)
		current += rw
	}
	for i, j := 0, len(tail)-1; i < j; i, j = i+1, j-1 {
		tail[i], tail[j] = tail[j], tail[i]
	}
	return string(tail)
}

func screenWidth(value string) int {
	width := 0
	for _, r := range value {
		width += screenRuneWidth(r)
	}
	return width
}

func screenRuneWidth(r rune) int {
	if r == '\t' {
		return 4
	}
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
		return 2
	}
	return 1
}

func (c *Console) appendScreen(kind, text string, isError bool) {
	if !c.screenActive {
		return
	}
	c.screenLog = append(c.screenLog, screenLine{Kind: kind, Text: text, Error: isError})
	c.repaintScreen()
}

func (c *Console) updateScreenAssistantStream(text string) {
	if !c.screenActive {
		return
	}
	if len(c.screenLog) > 0 && c.screenLog[len(c.screenLog)-1].Kind == "assistant_stream" {
		c.screenLog[len(c.screenLog)-1].Text = text
	} else {
		c.screenLog = append(c.screenLog, screenLine{Kind: "assistant_stream", Text: text})
	}
	c.repaintScreen()
}

func (c *Console) finalizeScreenAssistantStream(text string, isError bool) {
	if !c.screenActive {
		return
	}
	for index := len(c.screenLog) - 1; index >= 0; index-- {
		if c.screenLog[index].Kind == "assistant_stream" {
			c.screenLog[index] = screenLine{Kind: "assistant", Text: text, Error: isError}
			c.repaintScreen()
			return
		}
	}
	c.screenLog = append(c.screenLog, screenLine{Kind: "assistant", Text: text, Error: isError})
	c.repaintScreen()
}

func (c *Console) openCommandPalette() {
	c.paletteOpen = true
	filter := strings.TrimSpace(c.draft)
	if !strings.HasPrefix(filter, "/") {
		filter = ""
	}
	c.paletteFilter = filter
	if c.paletteSelected < 0 || c.paletteSelected >= len(c.visibleCommandPaletteItems()) {
		c.paletteSelected = 0
	}
	c.repaintScreen()
}

func (c *Console) toggleCommandPalette() {
	if c.paletteOpen {
		c.paletteOpen = false
	} else {
		c.openCommandPalette()
		return
	}
	c.repaintScreen()
}

func (c *Console) moveCommandPalette(delta int) {
	if !c.paletteOpen {
		return
	}
	items := c.visibleCommandPaletteItems()
	if len(items) == 0 {
		return
	}
	c.paletteSelected = (c.paletteSelected + delta + len(items)) % len(items)
	c.repaintScreen()
}

func (c *Console) selectedPaletteCommand() string {
	items := c.visibleCommandPaletteItems()
	if len(items) == 0 {
		return ""
	}
	if c.paletteSelected < 0 || c.paletteSelected >= len(items) {
		c.paletteSelected = 0
	}
	return items[c.paletteSelected].Command
}

func (c *Console) visibleCommandPaletteItems() []commandPaletteItem {
	return filterCommandPaletteItems(c.paletteFilter)
}

func (c *Console) setPanel(mode, title, content string) {
	c.panelMode = strings.ToLower(strings.TrimSpace(mode))
	c.panelTitle = strings.TrimSpace(title)
	c.panelContent = strings.TrimRight(content, "\n")
	c.repaintScreen()
}

func (c *Console) clearPanel() {
	c.panelMode = ""
	c.panelTitle = ""
	c.panelContent = ""
	c.repaintScreen()
}
