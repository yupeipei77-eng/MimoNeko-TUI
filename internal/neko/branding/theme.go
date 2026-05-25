package branding

const (
	Reset        = "\x1b[0m"
	Dim          = "\x1b[2m"
	Cyan         = "\x1b[36m"
	BrightCyan   = "\x1b[96m"
	SoftWhite    = "\x1b[97m"
	SoftGray     = "\x1b[37m"
	SuccessGreen = "\x1b[32m"
	FailureRed   = "\x1b[31m"
)

// Renderer owns the terminal color policy for NekoForge branding.
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
	return r.Paint(BrightCyan, text)
}

func (r Renderer) Label(text string) string {
	return r.Paint(Cyan, text)
}

func (r Renderer) Title(text string) string {
	return r.Paint(SoftWhite, text)
}

func (r Renderer) Value(text string) string {
	return r.Paint(SoftGray, text)
}

func (r Renderer) Muted(text string) string {
	return r.Paint(Dim, text)
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
