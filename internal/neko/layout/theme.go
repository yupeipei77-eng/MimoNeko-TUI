package layout

const (
	ansiReset    = "\x1b[0m"
	ansiBold     = "\x1b[1m"
	ansiDim      = "\x1b[2m"
	ansiItalic   = "\x1b[3m"
	ansiWarm     = "\x1b[38;5;214m" // Bright orange-gold - primary accent
	ansiBeige    = "\x1b[38;5;180m" // Soft gold - labels
	ansiCream    = "\x1b[38;5;230m" // Cream white - titles
	ansiWarmDim  = "\x1b[38;5;138m" // Muted gold - values
	ansiGreen    = "\x1b[38;5;114m" // Soft green - success
	ansiErrorRed = "\x1b[38;5;203m" // Soft red - errors
	ansiWarning  = "\x1b[38;5;221m" // Warm yellow - warnings
	ansiInfo     = "\x1b[38;5;111m" // Soft blue - info
	ansiCyan     = "\x1b[38;5;116m" // Soft cyan - user accent
	ansiPurple   = "\x1b[38;5;141m" // Soft purple - highlights
	ansiPanel    = "\x1b[48;5;236m" // Dark panel background
	ansiPanelLight = "\x1b[48;5;238m" // Lighter panel for user messages
	ansiBorder   = "\x1b[38;5;240m" // Dark gray for borders
	ansiMuted    = "\x1b[38;5;245m" // Medium gray text
)

func paint(code, text string, noColor bool) string {
	if noColor {
		return text
	}
	return code + text + ansiReset
}

func paintBold(text string, noColor bool) string {
	if noColor {
		return text
	}
	return ansiBold + text + ansiReset
}

func paintDim(text string, noColor bool) string {
	return paint(ansiDim, text, noColor)
}

func paintMuted(text string, noColor bool) string {
	return paint(ansiMuted, text, noColor)
}

func paintAccent(text string, noColor bool) string {
	return paint(ansiWarm, text, noColor)
}

func paintLabel(text string, noColor bool) string {
	return paint(ansiBeige, text, noColor)
}

func paintValue(text string, noColor bool) string {
	return paint(ansiWarmDim, text, noColor)
}

func paintSuccess(text string, noColor bool) string {
	return paint(ansiGreen, text, noColor)
}

func paintError(text string, noColor bool) string {
	return paint(ansiErrorRed, text, noColor)
}

func paintWarning(text string, noColor bool) string {
	return paint(ansiWarning, text, noColor)
}

func paintInfo(text string, noColor bool) string {
	return paint(ansiInfo, text, noColor)
}

func paintPanel(text string, noColor bool) string {
	if noColor {
		return text
	}
	return ansiPanel + text + ansiReset
}

func paintBorder(text string, noColor bool) string {
	return paint(ansiBorder, text, noColor)
}

func paintUserAccent(text string, noColor bool) string {
	return paint(ansiCyan, text, noColor)
}

func paintUserPanel(text string, noColor bool) string {
	if noColor {
		return text
	}
	return ansiPanelLight + text + ansiReset
}

func paintHighlight(text string, noColor bool) string {
	return paint(ansiPurple, text, noColor)
}
