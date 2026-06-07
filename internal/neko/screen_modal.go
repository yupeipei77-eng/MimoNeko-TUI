package neko

import (
	"fmt"
	"io"
	"strings"
)

func screenModalLine(content string, width int) string {
	return screenPaintBackground(content, width, modalRowBg)
}

func screenModalWidth(available, preferred, minimum int) int {
	width := preferred
	if width > available {
		width = available
	}
	if width < minimum {
		width = minimum
	}
	return width
}

func fillScreenModal(w io.Writer, left, width, top, bottom int) {
	for row := top; row <= bottom; row++ {
		fmt.Fprintf(w, "\x1b[%d;%dH%s", row, left, screenModalLine("", width))
	}
}

const (
	modalRowBg     = "48;5;236"
	modalAccentBg  = "48;5;216"
	modalAccentFg  = "38;5;16"
	modalFgTitle   = "38;5;230"
	modalFgMuted   = "38;5;244"
	modalFgAccent  = "38;5;214"
	modalFgSuccess = "38;5;114"
	modalFgFailure = "38;5;203"
	modalFgDivider = "48;5;240"
)

type modalPart struct {
	fg  string
	txt string
}

func renderModalRow(width int, parts ...modalPart) string {
	var b strings.Builder
	b.WriteString("\x1b[")
	b.WriteString(modalRowBg)
	b.WriteString("m")
	visible := 0
	for _, p := range parts {
		if p.fg != "" {
			b.WriteString("\x1b[")
			b.WriteString(p.fg)
			b.WriteString("m")
		}
		b.WriteString(p.txt)
		visible += screenWidth(p.txt)
	}
	if pad := width - visible; pad > 0 {
		b.WriteString(strings.Repeat(" ", pad))
	}
	b.WriteString("\x1b[0m")
	return b.String()
}

func renderModalAccentRow(width int, parts ...modalPart) string {
	var b strings.Builder
	b.WriteString("\x1b[")
	b.WriteString(modalAccentBg)
	b.WriteString("m")
	visible := 0
	for _, p := range parts {
		b.WriteString("\x1b[")
		b.WriteString(modalAccentFg)
		b.WriteString("m")
		b.WriteString(p.txt)
		visible += screenWidth(p.txt)
	}
	if pad := width - visible; pad > 0 {
		b.WriteString(strings.Repeat(" ", pad))
	}
	b.WriteString("\x1b[0m")
	return b.String()
}

func renderModalBlank(width int) string {
	if width <= 0 {
		return ""
	}
	return "\x1b[" + modalRowBg + "m" + strings.Repeat(" ", width) + "\x1b[0m"
}

func renderModalTwoCol(width int, left, right modalPart) string {
	lw := screenWidth(left.txt)
	rw := screenWidth(right.txt)
	gap := width - lw - rw
	if gap < 1 {
		gap = 1
	}
	var b strings.Builder
	b.WriteString("\x1b[")
	b.WriteString(modalRowBg)
	b.WriteString("m")
	b.WriteString("\x1b[")
	b.WriteString(left.fg)
	b.WriteString("m")
	b.WriteString(left.txt)
	b.WriteString(strings.Repeat(" ", gap))
	b.WriteString("\x1b[")
	b.WriteString(right.fg)
	b.WriteString("m")
	b.WriteString(right.txt)
	b.WriteString("\x1b[0m")
	return b.String()
}
