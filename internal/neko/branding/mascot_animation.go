package branding

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

// MascotState represents the current state of the mascot.
//
// The user-visible state set is: MascotIdle / MascotThinking / MascotSuccess
// / MascotError. The legacy names MascotAnswering and MascotDone are kept
// as aliases so existing call sites continue to compile; Answering renders
// as Thinking and Done renders as Success.
type MascotState string

const (
	MascotIdle      MascotState = "idle"
	MascotThinking  MascotState = "thinking"
	MascotAnswering MascotState = "answering"
	MascotDone      MascotState = "done"
	MascotSuccess   MascotState = "success"
	MascotError     MascotState = "error"
)

// ResolvedState maps legacy aliases to the user-visible state they render as.
func (s MascotState) ResolvedState() MascotState {
	switch s {
	case MascotAnswering:
		return MascotThinking
	case MascotDone:
		return MascotSuccess
	default:
		return s
	}
}

// mascotFrame is one fixed 3-line cat pose with a right-side caption.
type mascotFrame struct {
	lines [3]string
	right string
}

// userFrames holds the user-specified MimoNeko art. Lines are half-width
// ASCII; only the first line carries the emoji caption so middle lines never
// shift under CJK fallback fonts.
var userFrames = map[MascotState]mascotFrame{
	MascotIdle: {
		lines: [3]string{
			`   /\_/\`,
			`  (=~ェ~=)`,
			`   { ฅ     ฅ   }0`,
		},
		right: "🐱喵～",
	},
	MascotThinking: {
		lines: [3]string{
			`   /\_/\`,
			`  (=>ェ<=)`,
			`   { ฅ     ฅ   }`,
		},
		right: "🔥 死脑快想！",
	},
	MascotSuccess: {
		lines: [3]string{
			`   /\_/\`,
			`   (=0ω0=)`,
			`   { ฅ     ฅ   }0`,
		},
		right: "✨",
	},
	MascotError: {
		lines: [3]string{
			`   /\_/\`,
			`  (=ΩДΩ=)`,
			`   { ฅ     ฅ   }0`,
		},
		right: "❗",
	},
}

func init() {
	userFrames = map[MascotState]mascotFrame{
		MascotIdle: {
			lines: [3]string{
				" /\\_/\\ ",
				"(=owo=)",
				" /   \\ ",
			},
			right: "✦",
		},
		MascotThinking: {
			lines: [3]string{
				" /\\_/\\ ",
				"(=>w<=)",
				" /   \\ ",
			},
			right: "🔥 死脑快想！",
		},
		MascotSuccess: {
			lines: [3]string{
				" /\\_/\\ ",
				"(=^w^=)",
				" /   \\ ",
			},
			right: "✓",
		},
		MascotError: {
			lines: [3]string{
				" /\\_/\\ ",
				"(=x_x=)",
				" /   \\ ",
			},
			right: "!",
		},
	}
}

func init() {
	userFrames = map[MascotState]mascotFrame{
		MascotIdle: {
			lines: [3]string{
				" /\\_/\\ ",
				"(=owo=)",
				" /   \\ ",
			},
			right: "*",
		},
		MascotThinking: {
			lines: [3]string{
				" /\\_/\\ ",
				"(=>w<=)",
				" /   \\ ",
			},
			right: "thinking!",
		},
		MascotSuccess: {
			lines: [3]string{
				" /\\_/\\ ",
				"(=^w^=)",
				" /   \\ ",
			},
			right: "ok",
		},
		MascotError: {
			lines: [3]string{
				" /\\_/\\ ",
				"(=x_x=)",
				" /   \\ ",
			},
			right: "!",
		},
	}
}

// MascotAnimator owns the mascot state and renders the user-specified cat.
//
// State semantics:
//
//   - MascotIdle      — no auto-revert.
//   - MascotThinking  — entered on user input; stays until the chat returns.
//   - MascotSuccess   — entered on a successful response; Tick() reverts to
//     idle after 1 second.
//   - MascotError     — entered on a failed response; stays until the next
//     user input or the next successful response.
type MascotAnimator struct {
	state        MascotState
	frame        int
	running      bool
	interval     time.Duration
	noColor      bool
	successUntil time.Time
}

// SuccessWindow is the duration a success frame stays on screen before
// reverting to idle.
const SuccessWindow = 1 * time.Second

// NewMascotAnimator creates a new MascotAnimator.
func NewMascotAnimator(noColor bool) *MascotAnimator {
	return &MascotAnimator{
		state:    MascotIdle,
		interval: 300 * time.Millisecond,
		noColor:  noColor,
	}
}

// SetState changes the mascot state.
//
// Repeated calls to the same state are no-ops (except for MascotSuccess,
// which always restarts the 1-second timer). Switching into a thinking-like
// state re-arms the running flag so animators that drive Update() can
// advance frames; other states clear it.
func (a *MascotAnimator) SetState(state MascotState) {
	resolved := state.ResolvedState()
	if resolved == a.state.ResolvedState() && resolved != MascotSuccess {
		return
	}
	a.state = state
	a.frame = 0
	if resolved == MascotThinking {
		a.running = true
	} else {
		a.running = false
	}
	switch resolved {
	case MascotSuccess:
		a.successUntil = time.Now().Add(SuccessWindow)
	case MascotError, MascotIdle, MascotThinking:
		a.successUntil = time.Time{}
	}
}

// Tick advances the success timer. Should be called before rendering.
//
// When the success window has elapsed, the state reverts to idle.
func (a *MascotAnimator) Tick(now time.Time) {
	if a.state.ResolvedState() != MascotSuccess {
		return
	}
	if a.successUntil.IsZero() {
		return
	}
	if !now.Before(a.successUntil) {
		a.state = MascotIdle
		a.running = false
		a.successUntil = time.Time{}
	}
}

// State returns the current mascot state (raw, not resolved).
func (a *MascotAnimator) State() MascotState {
	return a.state
}

// IsRunning reports whether the animator is in a "running" state (currently
// only thinking; legacy answering maps to thinking).
func (a *MascotAnimator) IsRunning() bool {
	return a.running
}

// Update advances the animation by one frame. Used by external animators
// that drive the cat blink cycle.
func (a *MascotAnimator) Update() {
	if !a.running {
		return
	}
	a.frame++
}

// frameFor returns the user-frame for the current state, falling back to
// idle when the state has no registered frame.
func (a *MascotAnimator) frameFor() mascotFrame {
	resolved := a.state.ResolvedState()
	if f, ok := userFrames[resolved]; ok {
		return f
	}
	return userFrames[MascotIdle]
}

// BlockWidth returns the display-cell width of the mascot block for the
// current state.
func (a *MascotAnimator) BlockWidth() int {
	f := a.frameFor()
	line1 := cjkWidth(f.lines[0])
	right := cjkWidth(f.right)
	if right > 0 {
		line1 += 2 + right
	}
	line2 := cjkWidth(f.lines[1])
	line3 := cjkWidth(f.lines[2])
	w := line1
	if line2 > w {
		w = line2
	}
	if line3 > w {
		w = line3
	}
	return w
}

// FrameLines returns the three padded lines for the current state, along
// with the right-side caption and the block width. The returned lines are
// padded to a common width so the three lines align on the right edge.
func (a *MascotAnimator) FrameLines() (lines [3]string, right string, blockWidth int) {
	f := a.frameFor()
	blockWidth = a.BlockWidth()
	lines[0] = f.lines[0]
	if rw := cjkWidth(f.right); rw > 0 {
		lines[0] = f.lines[0] + "  " + f.right
	}
	lines[0] = padRightMascotCJK(lines[0], blockWidth)
	lines[1] = padRightMascotCJK(f.lines[1], blockWidth)
	lines[2] = padRightMascotCJK(f.lines[2], blockWidth)
	return lines, f.right, blockWidth
}

// SingleLineLabel returns a one-line caption for the current state, used as
// a fallback when the terminal is too narrow to fit the 3-line block.
func (a *MascotAnimator) SingleLineLabel() string {
	switch a.state.ResolvedState() {
	case MascotThinking:
		return "MimoNeko · Thinking..."
	case MascotSuccess:
		return "MimoNeko · Done"
	case MascotError:
		return "MimoNeko · Error"
	default:
		return "MimoNeko"
	}
}

// Render writes the 3-line mascot to w with trailing newlines. Use
// RenderScreenHeader for absolute-positioned screen rendering.
func (a *MascotAnimator) Render(w io.Writer) {
	lines, _, _ := a.FrameLines()
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
}

func (a *MascotAnimator) paintScreenLine(line string, row int) string {
	if a.noColor {
		return line
	}
	switch row {
	case 0:
		return WarmAccent + line + Reset
	case 1:
		return WarmTitle + line + Reset
	default:
		return WarmMuted + line + Reset
	}
}

// RenderScreenHeader draws the mascot right-aligned starting at (top, rightCol).
//
// When the available width is too small for the 3-line block, the mascot
// collapses to a single labelled line on the top row.
func (a *MascotAnimator) RenderScreenHeader(w io.Writer, top, rightCol, width int) {
	if width <= 0 || rightCol <= 0 || top <= 0 {
		return
	}
	if width < 16 || a.BlockWidth() > width {
		single := a.SingleLineLabel()
		singleWidth := cjkWidth(single)
		col := rightCol - singleWidth + 1
		if col < 1 {
			col = 1
		}
		fmt.Fprintf(w, "\x1b[%d;%dH%s", top, col, a.paintScreenLine(single, 0))
		return
	}
	_, _, blockW := a.FrameLines()
	col := rightCol - blockW + 1
	if col < 1 {
		col = 1
	}
	lines, _, _ := a.FrameLines()
	for i := 0; i < 3; i++ {
		fmt.Fprintf(w, "\x1b[%d;%dH%s", top+i, col, a.paintScreenLine(lines[i], i))
	}
}

// RenderWithLabel renders the mascot with a label appended to the middle
// line. Kept for compatibility with previous callers.
func (a *MascotAnimator) RenderWithLabel(w io.Writer, label string) {
	lines, _, _ := a.FrameLines()
	for i, line := range lines {
		if i == 1 && label != "" {
			fmt.Fprintf(w, "%s  %s\n", line, label)
		} else {
			fmt.Fprintln(w, line)
		}
	}
}

// getCurrentFrame returns the current 3 lines as a slice. Kept for
// compatibility with previous callers.
func (a *MascotAnimator) getCurrentFrame() []string {
	lines, _, _ := a.FrameLines()
	return []string{lines[0], lines[1], lines[2]}
}

func (a *MascotAnimator) getIdleFrame() []string      { return a.getCurrentFrame() }
func (a *MascotAnimator) getThinkingFrame() []string  { return a.getCurrentFrame() }
func (a *MascotAnimator) getAnsweringFrame() []string { return a.getCurrentFrame() }
func (a *MascotAnimator) getDoneFrame() []string      { return a.getCurrentFrame() }

// RenderStatic renders a static idle mascot with a label on the middle line.
func RenderStatic(w io.Writer, label string, noColor bool) {
	a := NewMascotAnimator(noColor)
	a.RenderWithLabel(w, label)
}

// RenderThinking renders the mascot in thinking state.
func RenderThinking(w io.Writer, noColor bool) {
	a := NewMascotAnimator(noColor)
	a.SetState(MascotThinking)
	a.Render(w)
}

// RenderAnswering renders the mascot in answering state (legacy alias).
func RenderAnswering(w io.Writer, noColor bool) {
	RenderThinking(w, noColor)
}

// RenderDone renders the mascot in done state (legacy alias → success).
func RenderDone(w io.Writer, noColor bool) {
	a := NewMascotAnimator(noColor)
	a.SetState(MascotSuccess)
	a.Render(w)
}

// FormatMascotState returns a human-readable label for a state.
func FormatMascotState(state MascotState, noColor bool) string {
	switch state.ResolvedState() {
	case MascotIdle:
		return "idle"
	case MascotThinking:
		return "thinking..."
	case MascotSuccess:
		return "done"
	case MascotError:
		return "error"
	default:
		return string(state)
	}
}

// padRightMascot pads s with spaces to target display cells, using
// runewidth to count. If s is already at or beyond the target, it is
// returned unchanged.
func padRightMascot(s string, target int) string {
	cur := runewidth.StringWidth(s)
	if cur >= target {
		return s
	}
	return s + strings.Repeat(" ", target-cur)
}

// padRightMascotCJK is the CJK-aware variant of padRightMascot. It pads
// s with spaces so the resulting visual width (CJK + emoji = 2 cells)
// matches target.
func padRightMascotCJK(s string, target int) string {
	cur := cjkWidth(s)
	if cur >= target {
		return s
	}
	return s + strings.Repeat(" ", target-cur)
}

// cjkWidth returns the visual cell width of s assuming a CJK terminal
// (East Asian chars and most emoji count as 2 cells). The rest of the
// codebase uses runewidth.StringWidth with EastAsianWidth=false, but the
// mascot art explicitly contains CJK and emoji that must occupy 2 cells
// to align with what the user sees in their terminal.
func cjkWidth(s string) int {
	w := 0
	for _, r := range s {
		w += cjkRuneWidth(r)
	}
	return w
}

// cjkRuneWidth returns the visual cell width of a single rune under the
// mascot's CJK terminal assumption.
func cjkRuneWidth(r rune) int {
	switch {
	case r < 0x20:
		return 0
	case r == 0x7F:
		return 0
	case r == 0x20:
		return 1
	case isWideRune(r):
		return 2
	default:
		return 1
	}
}

// isWideRune mirrors the East Asian Width=Wide/Fullwidth plus the major
// emoji ranges that occupy 2 cells in CJK terminals.
func isWideRune(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F:
		return true
	case r >= 0x2E80 && r <= 0x303E:
		return true
	case r >= 0x3041 && r <= 0x33FF:
		return true
	case r >= 0x3400 && r <= 0x4DBF:
		return true
	case r >= 0x4E00 && r <= 0x9FFF:
		return true
	case r >= 0xA000 && r <= 0xA4CF:
		return true
	case r >= 0xAC00 && r <= 0xD7A3:
		return true
	case r >= 0xF900 && r <= 0xFAFF:
		return true
	case r >= 0xFE30 && r <= 0xFE4F:
		return true
	case r >= 0xFF00 && r <= 0xFF60:
		return true
	case r >= 0xFFE0 && r <= 0xFFE6:
		return true
	case r >= 0x1F300 && r <= 0x1FAFF:
		return true
	case r >= 0x20000 && r <= 0x2FFFD:
		return true
	case r >= 0x30000 && r <= 0x3FFFD:
		return true
	}
	return false
}
