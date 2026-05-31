package neko

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

func (c *Console) runRawInput(ctx context.Context) bool {
	input, ok := c.Options.In.(*os.File)
	if !ok || !isTerminalFile(input) {
		return false
	}
	restore, err := enableRawInput(input)
	if err != nil {
		return false
	}
	defer restore()

	c.repaintScreen()
	reader := bufio.NewReader(input)
	for {
		if err := ctx.Err(); err != nil {
			c.emitError(fmt.Sprintf("input cancelled: %v", err))
			return true
		}
		r, _, err := reader.ReadRune()
		if err != nil {
			if err != io.EOF {
				c.emitError(fmt.Sprintf("input failed: %v", err))
			}
			return true
		}
		if c.handleRawRune(ctx, reader, r) {
			return true
		}
	}
}

func (c *Console) handleRawRune(ctx context.Context, reader *bufio.Reader, r rune) bool {
	switch r {
	case '\x03':
		c.emitInfo("Goodbye from MIMO.")
		return true
	case '\r', '\n':
		if c.paletteOpen {
			return c.executePaletteSelection(ctx)
		}
		line := c.draft
		c.draft = ""
		return c.submitRawLine(ctx, line)
	case '\x08', '\x7f':
		c.deleteDraftRune()
		c.updatePaletteForDraft()
		c.repaintScreen()
	case '\x10':
		c.cycleReasoning()
		c.repaintScreen()
	case '\x12':
		c.cycleReasoning()
		c.repaintScreen()
	case '\x04':
		c.setPanel("diff", "Diff", emptyAsPanel(c.panelContent, "No diff preview loaded."))
	case '\x05':
		c.setPanel("editor", "Editor", "Draft buffer")
	case '\x1b':
		c.handleEscapeSequence(reader)
	default:
		if isPrintableInputRune(r) {
			c.draft += string(r)
			c.updatePaletteForDraft()
			c.repaintScreen()
		}
	}
	return false
}

func (c *Console) handleEscapeSequence(reader *bufio.Reader) {
	first, err := reader.ReadByte()
	if err != nil {
		return
	}
	if first == '\x10' {
		c.cycleReasoning()
		c.repaintScreen()
		return
	}
	if first != '[' {
		return
	}
	var seq []byte
	for len(seq) < 24 {
		next, err := reader.ReadByte()
		if err != nil {
			return
		}
		seq = append(seq, next)
		if next >= 0x40 && next <= 0x7e {
			break
		}
	}
	switch string(seq) {
	case "A":
		c.moveCommandPalette(-1)
	case "B":
		c.moveCommandPalette(1)
	case "112;5u", "80;5u":
		c.cycleReasoning()
		c.repaintScreen()
	}
}

func (c *Console) executePaletteSelection(ctx context.Context) bool {
	command := c.selectedPaletteCommand()
	c.paletteOpen = false
	c.paletteFilter = ""
	if command == "" {
		line := strings.TrimSpace(c.draft)
		c.draft = ""
		if line == "" {
			c.repaintScreen()
			return false
		}
		return c.submitRawLine(ctx, line)
	}
	if strings.Contains(command, "<") {
		c.draft = strings.TrimSpace(command[:strings.Index(command, "<")])
		if c.draft != "" {
			c.draft += " "
		}
		c.repaintScreen()
		return false
	}
	c.draft = ""
	return c.submitRawLine(ctx, command)
}

func (c *Console) submitRawLine(ctx context.Context, line string) bool {
	if c.introActive {
		if strings.TrimSpace(line) == "" {
			c.repaintScreen()
			return false
		}
		c.introActive = false
	}
	return c.handleInputLine(ctx, line)
}

func (c *Console) deleteDraftRune() {
	if c.draft == "" {
		return
	}
	runes := []rune(c.draft)
	c.draft = string(runes[:len(runes)-1])
}

func (c *Console) updatePaletteForDraft() {
	filter := strings.TrimSpace(c.draft)
	if strings.HasPrefix(filter, "/") {
		if c.paletteFilter != filter {
			c.paletteSelected = 0
			c.paletteFilter = filter
		}
		c.paletteOpen = true
		return
	}
	c.paletteFilter = ""
	c.paletteOpen = false
}

func isPrintableInputRune(r rune) bool {
	return r >= 0x20 && r != 0x7f && !unicode.IsControl(r)
}

func isTerminalFile(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func emptyAsPanel(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
