package neko

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/neko/branding"
)

const (
	defaultScreenCols = 132
	defaultScreenRows = 36
	minComposerWidth  = 72
	maxComposerWidth  = 118
)

var screenANSIEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

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
		fmt.Fprint(&out, "\x1b[H\x1b[2J")
		row, col := c.renderIntroScreen(&out, renderer)
		fmt.Fprintf(&out, "\x1b[%d;%dH", row, col)
		fmt.Fprint(c.Options.Out, out.String())
		return
	}
	composerTop := c.screenRows - 4
	if composerTop < 12 {
		composerTop = c.screenRows - 4
	}
	messageTop := 2
	if c.Mascot != nil {
		messageTop = 6
	}
	messageBottom := composerTop - 2
	panelTop, panelBottom := c.screenPanelBounds(composerTop)
	paletteTop, paletteBottom := c.screenPaletteBounds(composerTop, panelTop)
	agentTop, agentBottom := c.agentPickerBounds(composerTop)
	providerTop, providerBottom := c.providerPickerBounds(composerTop)
	pickerTop, pickerBottom := c.modelPickerBounds(composerTop)
	addTop, addBottom := c.addFlowModalBounds(composerTop)
	if panelTop > 0 {
		messageBottom = panelTop - 2
	}
	if paletteTop > 0 && paletteTop-2 < messageBottom {
		messageBottom = paletteTop - 2
	}
	if agentTop > 0 && agentBottom+2 < messageBottom {
		messageBottom = agentTop - 2
	}
	if pickerTop > 0 && pickerBottom+2 < messageBottom {
		messageBottom = pickerTop - 2
	}
	if providerTop > 0 && providerBottom+2 < messageBottom {
		messageBottom = providerTop - 2
	}
	if addTop > 0 && addBottom+2 < messageBottom {
		messageBottom = addTop - 2
	}
	if messageBottom < messageTop {
		messageBottom = messageTop
	}

	fmt.Fprint(&out, "\x1b[H\x1b[2J")
	if c.Mascot != nil {
		c.Mascot.Tick(time.Now())
	}
	c.renderScreenHeader(&out, renderer, left, width)
	c.renderScreenMessages(&out, renderer, left, width, messageTop, messageBottom)
	if panelTop > 0 {
		c.renderScreenPanel(&out, renderer, left, width, panelTop, panelBottom)
	}
	c.renderScreenComposer(&out, renderer, left, width, composerTop)
	if paletteTop > 0 {
		c.renderScreenPalette(&out, renderer, left, width, paletteTop, paletteBottom)
	}
	if agentTop > 0 {
		c.renderAgentPicker(&out, renderer, left, width, agentTop, agentBottom)
	}
	if providerTop > 0 {
		c.renderProviderPicker(&out, renderer, left, width, providerTop, providerBottom)
	}
	if pickerTop > 0 {
		c.renderModelPickerV2(&out, renderer, left, width, pickerTop, pickerBottom)
	}
	promptLabel := "> "
	inputWidth := width - 2 - runewidth.StringWidth(promptLabel)
	visibleDraft := c.composerDraftDisplay(inputWidth)
	promptWidth := runewidth.StringWidth(promptLabel) + runewidth.StringWidth(visibleDraft)
	cursorCol := left + 2 + promptWidth
	cursorRow := composerTop + 1
	if addTop > 0 {
		cursorRow, cursorCol = c.renderAddProviderModal(&out, renderer, left, width, addTop, addBottom)
	} else if agentTop > 0 {
		cursorRow, cursorCol = agentTop+2, left+(width-screenModalWidth(width, 92, 58))/2+4
	} else if providerTop > 0 {
		cursorRow, cursorCol = providerTop+2, left+(width-screenModalWidth(width, 76, 58))/2+4
	} else if pickerTop > 0 {
		cursorRow, cursorCol = pickerTop+2, left+(width-screenModalWidth(width, 82, 56))/2+4
	} else if paletteTop > 0 {
		cursorRow, cursorCol = paletteTop+2, left+(width-screenModalWidth(width, 88, 54))/2+4
	}
	if cursorCol < 1 {
		cursorCol = 1
	}
	fmt.Fprintf(&out, "\x1b[%d;%dH", cursorRow, cursorCol)
	fmt.Fprint(c.Options.Out, out.String())
}

func (c *Console) composerWidth() int {
	width := c.screenCols - 36
	if width < minComposerWidth {
		width = minComposerWidth
	}
	if width > maxComposerWidth {
		width = maxComposerWidth
	}
	return width
}

func (c *Console) renderScreenHeader(w io.Writer, renderer branding.Renderer, left, width int) {
	_, _ = w, renderer
	_ = left
	if c.Mascot == nil || width <= 0 {
		return
	}
	top := 2
	rightCol := left + width - 2
	c.Mascot.RenderScreenHeader(w, top, rightCol, width)
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
	top := c.screenRows/2 + 4
	if top < 10 {
		top = 10
	}
	if maxTop := c.screenRows - 5; top > maxTop {
		top = maxTop
	}

	c.renderIntroWordmark(w, renderer, left, width, top)

	cursorRow, cursorCol := c.renderScreenWorkbenchComposerV2(w, renderer, left, width, top)
	if c.paletteOpen {
		paletteTop, paletteBottom := c.screenPaletteBounds(c.screenRows-4, 0)
		if paletteTop > 1 {
			c.renderScreenPalette(w, renderer, left, width, paletteTop, paletteBottom)
		}
	}
	return cursorRow, cursorCol
}

func (c *Console) renderIntroWordmark(w io.Writer, renderer branding.Renderer, left, width, composerTop int) {
	lines := []string{
		"    __  ___ _                 _   __     __        ",
		"   /  |/  /(_)___ ___  ____  / | / /__  / /______  ",
		"  / /|_/ // / __ `__ \\/ __ \\/  |/ / _ \\/ //_/ __ \\ ",
		" / /  / // / / / / / / /_/ / /|  /  __/ ,< / /_/ / ",
		"/_/  /_//_/_/ /_/ /_/\\____/_/ |_/\\___/_/|_|\\____/  ",
	}
	start := composerTop - len(lines) - 3
	if start < 2 {
		start = 2
	}
	for i, line := range lines {
		col := left + (width-screenWidth(line))/2
		if col < 1 {
			col = 1
		}
		styled := renderer.AccentBold(line)
		fmt.Fprintf(w, "\x1b[%d;%dH%s", start+i, col, styled)
	}

	meta := c.introMetaLine()
	metaCol := left + (width-screenWidth(meta))/2
	if metaCol < 1 {
		metaCol = 1
	}
	fmt.Fprintf(w, "\x1b[%d;%dH%s", start+len(lines)+1, metaCol, renderer.Muted(meta))
}

func (c *Console) introMetaLine() string {
	return c.Session.Model
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
	c.renderScreenWorkbenchComposerV2(w, renderer, left, width, top)
}

func (c *Console) renderScreenWorkbenchComposerV2(w io.Writer, renderer branding.Renderer, left, width, top int) (int, int) {
	contentWidth := width - 2
	if contentWidth < 20 {
		contentWidth = 20
	}
	modeLabel := renderer.Paint(branding.BrightCyan, displayMode(c.Session.Mode))
	statusParts := []string{modeLabel}
	if strings.TrimSpace(c.Session.Model) != "" {
		statusParts = append(statusParts, renderer.Title(c.Session.Model))
	}
	if strings.TrimSpace(c.Session.Provider) != "" {
		statusParts = append(statusParts, renderer.Muted(c.Session.Provider))
	}
	if reasoning := strings.TrimSpace(c.Session.ReasoningStatusLabel()); reasoning != "" {
		statusParts = append(statusParts, renderer.Accent(reasoning))
	}
	status := screenJoinStyled(statusParts, renderer.Muted(" | "))

	promptLabel := "> "
	inputWidth := contentWidth - screenWidth(promptLabel)
	input := c.composerDraftDisplay(inputWidth)
	if c.addFlow.active {
		input = ""
	}
	displayInput := input
	if displayInput == "" && !c.addFlow.active {
		displayInput = renderer.Muted(`Ask anything... "Fix broken tests"`)
	}

	ctxLabel := c.Input.Context
	if ctxLabel == "" {
		ctxLabel = "n/a"
	}
	footer := screenTwoColumn(
		renderer.Muted("ctx ")+renderer.Value(ctxLabel),
		renderer.Muted(screenFooterHintsV2()),
		width-2,
	)

	fmt.Fprintf(w, "\x1b[%d;%dH%s", top, left, screenWorkbenchBarLine(renderer, "", width))
	if c.addFlow.active {
		flowTitle := c.addFlowModalTitle()
		strip := renderModalTwoCol(
			width,
			modalPart{fg: modalFgMuted, txt: "    " + flowTitle},
			modalPart{fg: modalFgMuted, txt: "esc  "},
		)
		fmt.Fprintf(w, "\x1b[%d;%dH%s", top+1, left, strip)
	} else {
		prompt := promptLabel + displayInput
		fmt.Fprintf(w, "\x1b[%d;%dH%s", top+1, left, screenWorkbenchBarLine(renderer, prompt, width))
	}
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top+2, left, screenWorkbenchBarLine(renderer, status, width))
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top+3, left, "  "+screenPadRight(footer, width-2))

	cursorRow := top + 1
	cursorCol := left + 2 + runewidth.StringWidth(promptLabel) + runewidth.StringWidth(input)
	// When addFlow is active, the modal owns the cursor. repaintScreen replaces
	// these composer coordinates with the modal cursor before drawing.
	return cursorRow, cursorCol
}

func screenWorkbenchBarLine(renderer branding.Renderer, content string, width int) string {
	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	if renderer.NoColor {
		return "| " + screenPadRight(content, contentWidth)
	}
	return "\x1b[48;5;214m \x1b[0m" + screenPanel(" "+screenPadRight(content, contentWidth), false)
}

func screenFooterHintsV2() string {
	return "tab agents   ctrl+p commands"
}

func screenJoinStyled(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, sep)
}

func (c *Console) addFlowModalBounds(composerTop int) (int, int) {
	if !c.addFlow.active {
		return 0, 0
	}
	height := 7
	switch c.addFlow.step {
	case stepHeaderType:
		height = 9
	case stepDiscovering:
		height = 6
	}
	top := (c.screenRows - height) / 2
	if top < 5 {
		top = 5
	}
	bottom := top + height - 1
	if bottom >= composerTop-1 {
		bottom = composerTop - 2
	}
	if bottom < top {
		return 0, 0
	}
	return top, bottom
}

func (c *Console) renderAddProviderModal(w io.Writer, renderer branding.Renderer, left, width, top, bottom int) (int, int) {
	_ = renderer
	modalWidth := screenModalWidth(width, 68, 52)
	modalLeft := left + (width-modalWidth)/2
	if modalLeft < 1 {
		modalLeft = 1
	}
	title := c.addFlowModalTitle()
	headerRow := renderModalTwoCol(
		modalWidth,
		modalPart{fg: "1;" + modalFgTitle, txt: "    " + title},
		modalPart{fg: modalFgMuted, txt: "esc    "},
	)
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top, modalLeft, headerRow)
	row := top + 1
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, renderModalBlank(modalWidth))
	row++

	cursorRow := row
	cursorCol := modalLeft + 4
	switch c.addFlow.step {
	case stepHeaderType:
		for i, label := range []string{"API_KEY", "Bearer"} {
			rowText := "    " + label
			if i == c.addFlow.selectedHead {
				fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, renderModalAccentRow(modalWidth, modalPart{fg: modalFgAccent, txt: rowText}))
			} else {
				fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, renderModalRow(modalWidth, modalPart{fg: modalFgTitle, txt: rowText}))
			}
			row++
		}
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, renderModalBlank(modalWidth))
		row++
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, renderModalRow(modalWidth, modalPart{fg: modalFgMuted, txt: "    Use Up/Down or type 1/2"}))
		cursorRow = top + 2 + c.addFlow.selectedHead
		cursorCol = modalLeft + 4
	case stepDiscovering:
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, renderModalRow(modalWidth, modalPart{fg: modalFgMuted, txt: "    Discovering models..."}))
		cursorRow = row
		cursorCol = modalLeft + 4
	default:
		placeholder := c.addFlowPrompt()
		display := c.composerDraftDisplay(modalWidth - 8)
		if display == "" {
			display = placeholder
		}
		var rowParts []modalPart
		if display == placeholder {
			rowParts = []modalPart{{fg: modalFgMuted, txt: "    " + display}}
		} else {
			rowParts = []modalPart{{fg: modalFgTitle, txt: "    " + display}}
		}
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, renderModalRow(modalWidth, rowParts...))
		cursorRow = row
		cursorCol = modalLeft + 4 + runewidth.StringWidth(c.composerDraftDisplay(modalWidth-8))
		row++
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, renderModalBlank(modalWidth))
		row++
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, renderModalRow(modalWidth,
			modalPart{fg: modalFgAccent, txt: "    enter"},
			modalPart{fg: modalFgMuted, txt: "  submit"},
		))
	}
	for blank := row + 1; blank <= bottom; blank++ {
		fmt.Fprintf(w, "\x1b[%d;%dH%s", blank, modalLeft, renderModalBlank(modalWidth))
	}
	return cursorRow, cursorCol
}

func (c *Console) addFlowModalTitle() string {
	switch c.addFlow.step {
	case stepProviderName:
		return "Connect a provider"
	case stepBaseURL:
		return "Base URL"
	case stepAPIKey:
		return "API key"
	case stepHeaderType:
		return "Header type"
	case stepDiscovering:
		return "Discovering models"
	case stepManualModelFallback:
		return "Model name"
	default:
		return "Connect a provider"
	}
}

func (c *Console) composerDraftDisplay(width int) string {
	if width <= 0 {
		return ""
	}
	display := c.draft
	if c.addFlow.active {
		switch c.addFlow.step {
		case stepAPIKey:
			display = strings.Repeat("*", len([]rune(c.draft)))
		case stepHeaderType:
			if strings.TrimSpace(c.draft) == "" {
				display = c.addFlow.headerTypeLabel()
			}
		}
	}
	return screenTail(display, width)
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

	// Minimal status items
	statusItems := []string{
		renderer.Muted("ctx") + " " + renderer.Value(c.Input.Context),
		renderer.Muted("tools") + " " + renderer.Value(fmt.Sprintf("%d", c.Input.Tools)),
	}

	inner := width - 4
	if inner < 0 {
		inner = 0
	}

	status := strings.Join(statusItems, "  ")
	right := c.Input.CommandUI
	if strings.TrimSpace(right) == "" {
		right = renderer.Muted("/ commands")
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
			label := renderer.Muted("MimoNeko")
			if item.Error {
				label = renderer.Paint(branding.FailureRed, "! MimoNeko")
			} else {
				label = renderer.Accent("●") + renderer.Muted(" MimoNeko")
			}
			lines = append(lines, label)
			lines = append(lines, renderAssistantScreenText(renderer, item.Text, item.Error, contentWidth)...)
			lines = append(lines, "")
		case "assistant_stream":
			lines = append(lines, renderer.Accent("●")+renderer.Muted(" MimoNeko"))
			lines = append(lines, renderAssistantScreenText(renderer, item.Text, false, contentWidth)...)
		case "runtime":
			lines = append(lines, renderer.Accent("●")+renderer.Muted(" ")+renderer.Muted(item.Text))
		case "done":
			lines = append(lines, renderer.Paint(branding.SuccessGreen, "✓")+renderer.Muted(" ")+renderer.Muted(item.Text))
		case "thought":
			lines = append(lines, renderer.Accent("+")+renderer.Muted(" ")+renderer.Muted(item.Text))
		case "thought_content":
			lines = append(lines, renderer.Muted("› Thinking:"))
			for _, line := range wrapScreenText(item.Text, contentWidth) {
				lines = append(lines, renderer.Muted("  › ")+renderer.Muted(line))
			}
			lines = append(lines, "")
		case "thought_stream":
			if c.Thinking != nil && c.Thinking.ShowThoughts() {
				// Show reasoning text with dim styling
				lines = append(lines, renderer.Muted("› Thinking..."))
				for _, line := range wrapScreenText(item.Text, contentWidth) {
					lines = append(lines, renderer.Muted("  › ")+renderer.Muted(line))
				}
			}
			// When hidden, don't render — the runtime entry already shows ● thinking...
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
	height := len(items) + countCommandSections(items) + 5
	if len(items) == 0 {
		height = 6
	}
	if height > 16 {
		height = 16
	}
	anchor := composerTop
	if panelTop > 0 {
		anchor = panelTop
	}
	top := (c.screenRows - height) / 2
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
	modalWidth := screenModalWidth(width, 88, 54)
	modalLeft := left + (width-modalWidth)/2
	if modalLeft < 1 {
		modalLeft = 1
	}
	title := screenTwoColumn("    "+renderer.TitleBold("Commands"), renderer.Muted("esc")+"    ", modalWidth)
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top, modalLeft, screenModalLine(title, modalWidth))
	row := top + 1
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenModalLine("", modalWidth))
	row++
	search := strings.TrimPrefix(strings.TrimSpace(c.paletteFilter), "/")
	searchLine := "    " + renderer.Muted("Search")
	if search != "" {
		searchLine = "    /" + search
	}
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenModalLine(searchLine, modalWidth))
	row++
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenModalLine("", modalWidth))
	row++
	if len(items) == 0 && row <= bottom {
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenModalLine("    "+renderer.Muted("no matching commands"), modalWidth))
		fillScreenModal(w, modalLeft, modalWidth, row+1, bottom)
		return
	}
	section := ""
	for index, item := range items {
		if row > bottom {
			break
		}
		if item.Section != section {
			section = item.Section
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenModalLine("    "+renderer.Highlight(section), modalWidth))
			row++
			if row > bottom {
				break
			}
		}
		line := fmt.Sprintf("    %-18s %s", item.Command, renderer.Muted(item.Help))
		if index == c.paletteSelected {
			selectedLine := fmt.Sprintf("    %-18s %s", item.Command, item.Help)
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenSelected(screenPadRight(selectedLine, modalWidth), false))
		} else {
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenModalLine(line, modalWidth))
		}
		row++
	}
	fillScreenModal(w, modalLeft, modalWidth, row, bottom)
}

// --- Agent Picker ---

func (c *Console) openAgentPicker() {
	c.agentPickerItems = c.buildAgentPickerItems()
	c.agentPickerOpen = true
	c.providerPickerOpen = false
	c.modelPickerOpen = false
	c.paletteOpen = false
	c.agentPickerSelected = 0
	for i, item := range c.agentPickerItems {
		if item.Mode == c.Session.Mode {
			c.agentPickerSelected = i
			break
		}
	}
	c.repaintScreen()
}

func (c *Console) closeAgentPicker() {
	c.agentPickerOpen = false
	c.refreshInput()
	c.repaintScreen()
}

func (c *Console) moveAgentPicker(delta int) {
	if !c.agentPickerOpen || len(c.agentPickerItems) == 0 {
		return
	}
	c.agentPickerSelected = (c.agentPickerSelected + delta + len(c.agentPickerItems)) % len(c.agentPickerItems)
	c.repaintScreen()
}

func (c *Console) executeAgentPickerSelection() {
	if !c.agentPickerOpen || len(c.agentPickerItems) == 0 {
		return
	}
	item := c.agentPickerItems[c.agentPickerSelected]
	if c.Session.SetMode(item.Mode) {
		c.refreshInput()
		c.agentPickerOpen = false
		c.setStatus(fmt.Sprintf("Agent switched to %s | mode %s", item.Name, c.Session.Mode), false)
		return
	}
	c.closeAgentPicker()
}

func (c *Console) buildAgentPickerItems() []agentPickerItem {
	modes := defaultAgentModes()
	items := make([]agentPickerItem, 0, len(modes))
	for _, mode := range modes {
		items = append(items, agentPickerItem{
			Name:        mode.Name(),
			Mode:        mode.ID(),
			Description: mode.Description(),
			Tools:       mode.AllowedTools(),
			Permission:  string(mode.WritePermission()),
			Worktree:    mode.UseWorktree(),
		})
	}
	return items
}

func (c *Console) agentPickerBounds(composerTop int) (int, int) {
	if !c.agentPickerOpen {
		return 0, 0
	}
	height := len(c.agentPickerItems)*2 + 5
	if height < 7 {
		height = 7
	}
	top := (c.screenRows - height) / 2
	if top < 5 {
		top = 5
	}
	bottom := top + height - 1
	if bottom >= composerTop-1 {
		bottom = composerTop - 2
	}
	if bottom < top {
		return 0, 0
	}
	return top, bottom
}

func (c *Console) renderAgentPicker(w io.Writer, renderer branding.Renderer, left, width, top, bottom int) {
	pickerWidth := screenModalWidth(width, 92, 58)
	pickerLeft := left + (width-pickerWidth)/2
	if pickerLeft < 1 {
		pickerLeft = 1
	}
	title := screenTwoColumn("    "+renderer.TitleBold("Switch agent"), renderer.Muted("esc")+"    ", pickerWidth)
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top, pickerLeft, screenModalLine(title, pickerWidth))
	row := top + 1
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenModalLine("", pickerWidth))
	row++
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenModalLine("    "+renderer.Muted("Use Up/Down then enter"), pickerWidth))
	row++
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenModalLine("", pickerWidth))
	row++

	for idx, item := range c.agentPickerItems {
		if row > bottom {
			break
		}
		leftText := "    " + item.Name
		if item.Mode == c.Session.Mode {
			leftText = "  * " + item.Name
		}
		line := screenTwoColumn(leftText, renderer.Muted(item.Description)+"    ", pickerWidth)
		if idx == c.agentPickerSelected {
			selectedLine := screenTwoColumn("    "+item.Name, item.Description+"    ", pickerWidth)
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenSelected(screenPadRight(selectedLine, pickerWidth), false))
		} else {
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenModalLine(line, pickerWidth))
		}
		row++
		if row > bottom {
			break
		}
		meta := fmt.Sprintf("tools=%s  permission=%s  worktree=%v", strings.Join(agentPickerToolLabels(item.Tools), ","), item.Permission, item.Worktree)
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenModalLine("      "+renderer.Muted(meta), pickerWidth))
		row++
	}
	fillScreenModal(w, pickerLeft, pickerWidth, row, bottom)
}

func agentPickerToolLabels(tools []string) []string {
	labels := make([]string, 0, len(tools))
	for _, tool := range tools {
		switch tool {
		case "file_read":
			labels = append(labels, "read")
		case "list_files":
			labels = append(labels, "list")
		case "git_diff":
			labels = append(labels, "diff")
		case "test_run":
			labels = append(labels, "test")
		case "patch_preview":
			labels = append(labels, "patch")
		default:
			labels = append(labels, tool)
		}
	}
	return labels
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
	return screenPaintBackground(text, screenWidth(text), "48;5;235")
}

func screenTwoColumn(left, right string, width int) string {
	gap := width - screenWidth(left) - screenWidth(right)
	if gap < 1 {
		return screenTruncate(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

func countCommandSections(items []commandPaletteItem) int {
	seen := make(map[string]bool)
	count := 0
	for _, item := range items {
		if item.Section == "" || seen[item.Section] {
			continue
		}
		seen[item.Section] = true
		count++
	}
	return count
}

func screenUserPanel(text string, noColor bool) string {
	if noColor {
		return text
	}
	return screenPaintBackground(text, screenWidth(text), "48;5;237")
}

func screenSelected(text string, noColor bool) string {
	if noColor {
		return text
	}
	return screenPaintBackground("\x1b[38;5;16m"+text, screenWidth(text), modalAccentBg)
}

func screenPaintBackground(content string, width int, bgSGR string) string {
	if width <= 0 {
		return ""
	}
	padded := screenPadRight(content, width)
	return "\x1b[" + bgSGR + "m" +
		strings.ReplaceAll(padded, branding.Reset, branding.Reset+"\x1b["+bgSGR+"m") +
		branding.Reset
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
		renderer.BorderAccent(" "+branding.BoxTopRight)
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
	for i := 0; i < len(value); {
		if escapeLen := screenANSIEscapeLen(value, i); escapeLen > 0 {
			out.WriteString(value[i : i+escapeLen])
			i += escapeLen
			continue
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && size == 0 {
			break
		}
		rw := screenRuneWidth(r)
		if current+rw > width {
			break
		}
		out.WriteRune(r)
		current += rw
		i += size
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
	return runewidth.StringWidth(stripScreenANSI(value))
}

func screenRuneWidth(r rune) int {
	if r == '\t' {
		return 4
	}
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	return runewidth.RuneWidth(r)
}

func stripScreenANSI(value string) string {
	if value == "" {
		return ""
	}
	return screenANSIEscapePattern.ReplaceAllString(value, "")
}

func screenANSIEscapeLen(value string, start int) int {
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

func (c *Console) appendScreen(kind, text string, isError bool) {
	if !c.screenActive {
		return
	}
	c.screenLog = append(c.screenLog, screenLine{Kind: kind, Text: text, Error: isError})
	c.repaintScreen()
}

// removeTransientScreenEntries removes runtime, done, thought, and build entries
// that should not persist in the chat history after a response completes.
func (c *Console) removeTransientScreenEntries() {
	filtered := c.screenLog[:0]
	for _, item := range c.screenLog {
		switch item.Kind {
		case "runtime", "done", "thought", "build":
			// skip transient entries
		default:
			filtered = append(filtered, item)
		}
	}
	c.screenLog = filtered
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

// updateScreenThoughtStream creates or updates the thought_stream entry in screenLog.
// During streaming, reasoning text is kept in a separate entry from the answer.
func (c *Console) updateScreenThoughtStream(text string) {
	if !c.screenActive {
		return
	}
	// Look for existing thought_stream entry
	for i := len(c.screenLog) - 1; i >= 0; i-- {
		if c.screenLog[i].Kind == "thought_stream" {
			c.screenLog[i].Text = text
			c.repaintScreen()
			return
		}
	}
	// No existing entry; append new one
	c.screenLog = append(c.screenLog, screenLine{Kind: "thought_stream", Text: text})
	c.repaintScreen()
}

// finalizeScreenThoughtStream converts the thought_stream entry into its final form.
// If thoughts are visible, it becomes a permanent "thought_content" entry.
// If thoughts are hidden, the thought_stream entry is removed.
func (c *Console) finalizeScreenThoughtStream() {
	if !c.screenActive {
		return
	}
	for i := len(c.screenLog) - 1; i >= 0; i-- {
		if c.screenLog[i].Kind == "thought_stream" {
			if c.Thinking != nil && c.Thinking.ShowThoughts() {
				// Keep as permanent thought_content entry
				c.screenLog[i].Kind = "thought_content"
			} else {
				// Remove thought entry entirely
				c.screenLog = append(c.screenLog[:i], c.screenLog[i+1:]...)
			}
			c.repaintScreen()
			return
		}
	}
}

// refreshScreenThinking re-renders the thinking area in screen mode
// when the user toggles thought visibility (Ctrl+T).
func (c *Console) refreshScreenThinking() {
	if !c.screenActive {
		return
	}
	// Remove any existing thought_content or thought_stream entries
	filtered := c.screenLog[:0]
	for _, item := range c.screenLog {
		if item.Kind != "thought_content" && item.Kind != "thought_stream" {
			filtered = append(filtered, item)
		}
	}
	c.screenLog = filtered

	// If we have stored reasoning text and thoughts should be visible, re-add
	if c.lastReasoningText != "" && c.Thinking != nil && c.Thinking.ShowThoughts() {
		// Insert before the first assistant_stream or assistant entry
		insertIdx := len(c.screenLog)
		for i, item := range c.screenLog {
			if item.Kind == "assistant_stream" || item.Kind == "assistant" {
				insertIdx = i
				break
			}
		}
		newEntry := screenLine{Kind: "thought_content", Text: c.lastReasoningText}
		c.screenLog = append(c.screenLog, screenLine{}) // make room
		copy(c.screenLog[insertIdx+1:], c.screenLog[insertIdx:])
		c.screenLog[insertIdx] = newEntry
	}
	c.repaintScreen()
}

// screenThinkingDots returns a time-based dot animation string.
// Dots cycle: . → .. → ... → .... → ..... → . → ...
func screenThinkingDots() string {
	n := int(time.Now().UnixMilli()/300) % 5
	if n == 0 {
		n = 5
	}
	return strings.Repeat(".", n)
}

func (c *Console) openCommandPalette() {
	c.paletteOpen = true
	c.agentPickerOpen = false
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

// --- Provider Picker ---

func (c *Console) openProviderPicker() {
	c.providerPickerItems = c.buildProviderPickerItems()
	c.providerPickerOpen = true
	c.agentPickerOpen = false
	c.modelPickerOpen = false
	c.paletteOpen = false
	c.providerPickerSelected = 0
	for i, item := range c.providerPickerItems {
		if item.Current {
			c.providerPickerSelected = i
			break
		}
	}
	c.repaintScreen()
}

func (c *Console) closeProviderPicker() {
	c.providerPickerOpen = false
	c.repaintScreen()
}

func (c *Console) moveProviderPicker(delta int) {
	if !c.providerPickerOpen || len(c.providerPickerItems) == 0 {
		return
	}
	c.providerPickerSelected = (c.providerPickerSelected + delta + len(c.providerPickerItems)) % len(c.providerPickerItems)
	c.repaintScreen()
}

func (c *Console) executeProviderPickerSelection() {
	if !c.providerPickerOpen || len(c.providerPickerItems) == 0 {
		return
	}
	item := c.providerPickerItems[c.providerPickerSelected]
	c.providerPickerOpen = false
	if item.IsCustom {
		c.startAddProviderFlow()
		return
	}
	c.startAddProviderFlowWithProvider(item.Name, item.BaseURL)
}

func (c *Console) buildProviderPickerItems() []providerPickerItem {
	var items []providerPickerItem
	for _, provider := range c.Session.Models.Providers {
		name := strings.TrimSpace(provider.Name)
		if name == "" {
			continue
		}
		items = append(items, providerPickerItem{
			Name:       name,
			BaseURL:    strings.TrimSpace(provider.BaseURL),
			Configured: true,
			Current:    strings.EqualFold(name, c.Session.Provider),
		})
	}
	items = append(items, providerPickerItem{Name: "Custom API Provider", IsCustom: true})
	return items
}

func (c *Console) providerPickerBounds(composerTop int) (int, int) {
	if !c.providerPickerOpen {
		return 0, 0
	}
	height := len(c.providerPickerItems) + 5
	if height > 17 {
		height = 17
	}
	if height < 8 {
		height = 8
	}
	top := (c.screenRows - height) / 2
	if top < 5 {
		top = 5
	}
	bottom := top + height - 1
	if bottom >= composerTop-1 {
		bottom = composerTop - 2
	}
	if bottom < top {
		return 0, 0
	}
	return top, bottom
}

func (c *Console) renderProviderPicker(w io.Writer, renderer branding.Renderer, left, width, top, bottom int) {
	items := c.providerPickerItems
	modalWidth := screenModalWidth(width, 76, 58)
	modalLeft := left + (width-modalWidth)/2
	if modalLeft < 1 {
		modalLeft = 1
	}
	title := screenTwoColumn("    "+renderer.TitleBold("Connect a provider"), renderer.Muted("esc")+"    ", modalWidth)
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top, modalLeft, screenModalLine(title, modalWidth))
	row := top + 1
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenModalLine("", modalWidth))
	row++
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenModalLine("    "+renderer.Muted("Search"), modalWidth))
	row++
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenModalLine("", modalWidth))
	row++

	visibleRows := bottom - row + 1
	startIdx := 0
	if c.providerPickerSelected >= visibleRows {
		startIdx = c.providerPickerSelected - visibleRows + 1
	}
	endIdx := startIdx + visibleRows
	if endIdx > len(items) {
		endIdx = len(items)
	}
	for idx := startIdx; idx < endIdx; idx++ {
		item := items[idx]
		label := "    " + item.Name
		if item.Current {
			label = "  ✓ " + item.Name
		}
		right := ""
		if item.Configured {
			right = renderer.Muted("Configured") + "    "
		}
		if item.IsCustom {
			right = renderer.Muted("OpenAI-compatible") + "    "
		}
		line := screenTwoColumn(label, right, modalWidth)
		if idx == c.providerPickerSelected {
			selectedLine := screenTwoColumn("    "+item.Name, stripScreenANSI(right), modalWidth)
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenSelected(screenPadRight(selectedLine, modalWidth), false))
		} else {
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row, modalLeft, screenModalLine(line, modalWidth))
		}
		row++
	}
	fillScreenModal(w, modalLeft, modalWidth, row, bottom)
}

// --- Model Picker ---

func (c *Console) openModelPicker() {
	c.modelPickerItems = c.buildModelPickerItems()
	c.modelPickerOpen = true
	c.agentPickerOpen = false
	c.providerPickerOpen = false
	c.paletteOpen = false
	// Pre-select current model
	for i, item := range c.modelPickerItems {
		if !item.IsGroup && item.Model == c.Session.Model && item.Provider == c.Session.Provider {
			c.modelPickerSelected = i
			break
		}
	}
	c.repaintScreen()
}

func (c *Console) closeModelPicker() {
	c.modelPickerOpen = false
	c.refreshInput()
	c.repaintScreen()
}

func (c *Console) moveModelPicker(delta int) {
	if !c.modelPickerOpen || len(c.modelPickerItems) == 0 {
		return
	}
	// Skip group headers when navigating
	for {
		c.modelPickerSelected = (c.modelPickerSelected + delta + len(c.modelPickerItems)) % len(c.modelPickerItems)
		if !c.modelPickerItems[c.modelPickerSelected].IsGroup {
			break
		}
		// Safety: if all items are groups (shouldn't happen), break
		if c.modelPickerSelected == 0 && delta > 0 || c.modelPickerSelected == len(c.modelPickerItems)-1 && delta < 0 {
			break
		}
	}
	c.repaintScreen()
}

func (c *Console) executeModelPickerSelection() {
	if !c.modelPickerOpen || len(c.modelPickerItems) == 0 {
		return
	}
	item := c.modelPickerItems[c.modelPickerSelected]
	if item.IsGroup {
		return
	}
	if item.IsAddProvider {
		c.modelPickerOpen = false
		c.openProviderPicker()
		return
	}
	if c.Session.SelectModel(item.Model) {
		c.refreshInput()
		c.setStatus(fmt.Sprintf("Model switched to %s | provider %s", c.Session.Model, c.Session.Provider), false)
	}
	c.closeModelPicker()
}

func (c *Console) buildModelPickerItems() []modelPickerItem {
	var items []modelPickerItem
	for _, provider := range c.Session.Models.Providers {
		if len(provider.Models) == 0 {
			continue
		}
		for _, model := range provider.Models {
			if model.Name != "" {
				items = append(items, modelPickerItem{Provider: provider.Name, Model: model.Name})
			}
		}
	}
	items = append(items, modelPickerItem{IsAddProvider: true})
	return items
}

func (c *Console) modelPickerBounds(composerTop int) (int, int) {
	if !c.modelPickerOpen {
		return 0, 0
	}
	height := len(c.modelPickerItems) + 5
	if height > 18 {
		height = 18
	}
	if height < 7 {
		height = 7
	}
	top := (c.screenRows - height) / 2
	if top < 5 {
		top = 5
	}
	bottom := top + height - 1
	if bottom >= composerTop-1 {
		bottom = composerTop - 2
	}
	if bottom < top {
		return 0, 0
	}
	return top, bottom
}

func (c *Console) renderModelPickerV2(w io.Writer, renderer branding.Renderer, left, width, top, bottom int) {
	pickerWidth := screenModalWidth(width, 82, 56)
	pickerLeft := left + (width-pickerWidth)/2
	if pickerLeft < 1 {
		pickerLeft = 1
	}
	title := screenTwoColumn("    "+renderer.TitleBold("Select model"), renderer.Muted("esc")+"    ", pickerWidth)
	fmt.Fprintf(w, "\x1b[%d;%dH%s", top, pickerLeft, screenModalLine(title, pickerWidth))
	row := top + 1
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenModalLine("", pickerWidth))
	row++
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenModalLine("    "+renderer.Muted("Search"), pickerWidth))
	row++
	fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenModalLine("", pickerWidth))
	row++

	visibleRows := bottom - row + 1
	if visibleRows < 1 {
		return
	}
	startIdx := 0
	if c.modelPickerSelected >= visibleRows {
		startIdx = c.modelPickerSelected - visibleRows + 1
	}
	endIdx := startIdx + visibleRows
	if endIdx > len(c.modelPickerItems) {
		endIdx = len(c.modelPickerItems)
	}
	for idx := startIdx; idx < endIdx; idx++ {
		item := c.modelPickerItems[idx]
		if item.IsAddProvider {
			line := screenTwoColumn("    Connect provider", renderer.Muted("ctrl+a")+"    ", pickerWidth)
			if idx == c.modelPickerSelected {
				fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenSelected(screenPadRight("    Connect provider", pickerWidth), false))
			} else {
				fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenModalLine(line, pickerWidth))
			}
			row++
			continue
		}
		isCurrent := item.Model == c.Session.Model && item.Provider == c.Session.Provider
		label := "    " + item.Model
		if isCurrent {
			label = "  ✓ " + item.Model
		}
		line := screenTwoColumn(label, renderer.Muted(item.Provider)+"    ", pickerWidth)
		if idx == c.modelPickerSelected {
			selectedLine := screenTwoColumn("    "+item.Model, item.Provider+"    ", pickerWidth)
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenSelected(screenPadRight(selectedLine, pickerWidth), false))
		} else {
			fmt.Fprintf(w, "\x1b[%d;%dH%s", row, pickerLeft, screenModalLine(line, pickerWidth))
		}
		row++
	}
	fillScreenModal(w, pickerLeft, pickerWidth, row, bottom)
}
