package branding

const (
	Reset          = "\x1b[0m"
	Dim            = "\x1b[2m"
	Italic         = "\x1b[3m"
	Cyan           = "\x1b[36m"
	BrightCyan     = "\x1b[38;5;215m" // Kept for API compatibility; now warm peach.
	WarmAccent     = "\x1b[38;5;214m" // Orange-gold - primary accent
	WarmLabel      = "\x1b[38;5;180m" // Softer gold
	WarmTitle      = "\x1b[38;5;230m" // Cream white
	WarmMuted      = "\x1b[38;5;244m" // Warm gray
	SuccessGreen   = "\x1b[38;5;114m" // Soft green
	FailureRed     = "\x1b[38;5;203m" // Soft red
	Bold           = "\x1b[1m"
	White          = "\x1b[97m"
	BorderDim      = "\x1b[38;5;240m" // Dark gray for borders
	UserAccent     = "\x1b[38;5;215m" // Warm peach for user
	AIAccent       = "\x1b[38;5;214m" // Orange for AI
	SystemAccent   = "\x1b[38;5;138m" // Muted gold for system
	BoxTopLeft     = "╭"
	BoxTopRight    = "╮"
	BoxBottomLeft  = "╰"
	BoxBottomRight = "╯"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
	BoxDiamond     = "◆"
	BoxArrow       = "→"
	BoxDot         = "●"
	BoxCheck       = "✓"
	BoxCross       = "✗"
	BoxStar        = "★"
)

// Renderer owns the terminal color policy for MimoNeko branding.
type Renderer struct {
	NoColor bool
}

func NewRenderer(noColor bool) Renderer {
	return Renderer{NoColor: noColor}
}

func (r Renderer) Paint(code, text string) string {
	if r.NoColor {
		return text
	}
	return code + text + Reset
}

func (r Renderer) Accent(text string) string {
	return r.Paint(WarmAccent, text)
}

func (r Renderer) Label(text string) string {
	return r.Paint(WarmLabel, text)
}

func (r Renderer) Title(text string) string {
	return r.Paint(WarmTitle, text)
}

func (r Renderer) Value(text string) string {
	return r.Paint(Dim, text)
}

func (r Renderer) Muted(text string) string {
	return r.Paint(WarmMuted, text)
}

func (r Renderer) State(state string) string {
	switch state {
	case "succeeded", "approve", "ok":
		return r.Paint(SuccessGreen, state)
	case "failed", "rejected", "request_changes":
		return r.Paint(FailureRed, state)
	default:
		return state
	}
}

func (r Renderer) BorderAccent(text string) string {
	return r.Paint(BorderDim, text)
}

func (r Renderer) TitleBold(text string) string {
	return r.Paint(Bold+White, text)
}

func (r Renderer) Border(text string) string {
	return r.Paint(BorderDim, text)
}

func (r Renderer) Bright(text string) string {
	return r.Paint(WarmAccent, text)
}

func (r Renderer) Cyan(text string) string {
	return r.Paint(UserAccent, text)
}

func (r Renderer) AccentBold(text string) string {
	return r.Paint(Bold+WarmAccent, text)
}

func (r Renderer) Bold(text string) string {
	return r.Paint(Bold, text)
}

func (r Renderer) Highlight(text string) string {
	return r.Paint("\x1b[38;5;216m", text) // Warm peach highlight
}

func (r Renderer) Success(text string) string {
	return r.Paint(SuccessGreen, text)
}

func (r Renderer) Error(text string) string {
	return r.Paint(FailureRed, text)
}
