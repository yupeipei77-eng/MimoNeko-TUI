package branding

import (
	"fmt"
	"io"
	"time"
)

// MascotState represents the current state of the mascot.
type MascotState string

const (
	MascotIdle      MascotState = "idle"
	MascotThinking  MascotState = "thinking"
	MascotAnswering MascotState = "answering"
	MascotDone      MascotState = "done"
)

// MascotAnimator manages the mascot animation.
type MascotAnimator struct {
	state    MascotState
	frame    int
	running  bool
	interval time.Duration
	noColor  bool
}

// NewMascotAnimator creates a new MascotAnimator.
func NewMascotAnimator(noColor bool) *MascotAnimator {
	return &MascotAnimator{
		state:    MascotIdle,
		interval: 300 * time.Millisecond,
		noColor:  noColor,
	}
}

// SetState changes the mascot state.
func (a *MascotAnimator) SetState(state MascotState) {
	a.state = state
	a.frame = 0
	if state == MascotThinking || state == MascotAnswering {
		a.running = true
	} else {
		a.running = false
	}
}

// Update advances the animation by one frame.
func (a *MascotAnimator) Update() {
	if !a.running {
		return
	}
	a.frame++
}

// Render renders the current mascot frame.
func (a *MascotAnimator) Render(w io.Writer) {
	frame := a.getCurrentFrame()
	for _, line := range frame {
		fmt.Fprintln(w, line)
	}
}

// getCurrentFrame returns the current frame lines based on state.
func (a *MascotAnimator) getCurrentFrame() []string {
	switch a.state {
	case MascotIdle:
		return a.getIdleFrame()
	case MascotThinking:
		return a.getThinkingFrame()
	case MascotAnswering:
		return a.getAnsweringFrame()
	case MascotDone:
		return a.getDoneFrame()
	default:
		return a.getIdleFrame()
	}
}

// getIdleFrame returns the idle frame (cat sleeping).
func (a *MascotAnimator) getIdleFrame() []string {
	return []string{
		` /\_/\  `,
		`( -.- ) `,
		` > ^ <  `,
	}
}

// getThinkingFrame returns the thinking frame (eyes blinking).
func (a *MascotAnimator) getThinkingFrame() []string {
	frames := [][]string{
		{
			` /\_/\  `,
			`( o.o ) `,
			` > ^ <  `,
		},
		{
			` /\_/\  `,
			`( *.* ) `,
			` > ^ <  `,
		},
		{
			` /\_/\  `,
			`( o.o ) `,
			` < ^ >  `,
		},
		{
			` /\_/\  `,
			`( *.* ) `,
			` < ^ >  `,
		},
	}
	return frames[a.frame%len(frames)]
}

// getAnsweringFrame returns the answering frame (eyes sparkling, hands moving).
func (a *MascotAnimator) getAnsweringFrame() []string {
	frames := [][]string{
		{
			` /\_/\  `,
			`( ✦.✦ ) `,
			` > ^ <  `,
		},
		{
			` /\_/\  `,
			`( ★.★ ) `,
			` < ^ >  `,
		},
		{
			` /\_/\  `,
			`( ✦.✦ ) `,
			` < ^ >  `,
		},
		{
			` /\_/\  `,
			`( ★.★ ) `,
			` > ^ <  `,
		},
	}
	return frames[a.frame%len(frames)]
}

// getDoneFrame returns the done frame (cat relaxed).
func (a *MascotAnimator) getDoneFrame() []string {
	return []string{
		` /\_/\  `,
		`( ^.^ ) `,
		` > ^ <  `,
	}
}

// RenderWithLabel renders the mascot with a label.
func (a *MascotAnimator) RenderWithLabel(w io.Writer, label string) {
	frame := a.getCurrentFrame()
	for i, line := range frame {
		if i == 1 {
			fmt.Fprintf(w, "%s  %s\n", line, label)
		} else {
			fmt.Fprintln(w, line)
		}
	}
}

// IsRunning returns whether the animation is running.
func (a *MascotAnimator) IsRunning() bool {
	return a.running
}

// State returns the current mascot state.
func (a *MascotAnimator) State() MascotState {
	return a.state
}

// RenderStatic renders a static mascot with a label.
func RenderStatic(w io.Writer, label string, noColor bool) {
	frame := []string{
		` /\_/\  `,
		`( -.- ) `,
		` > ^ <  `,
	}
	for i, line := range frame {
		if i == 1 && label != "" {
			fmt.Fprintf(w, "%s  %s\n", line, label)
		} else {
			fmt.Fprintln(w, line)
		}
	}
}

// RenderThinking renders the mascot in thinking state.
func RenderThinking(w io.Writer, noColor bool) {
	frame := []string{
		` /\_/\  `,
		`( o.o ) `,
		` > ^ <  `,
	}
	for _, line := range frame {
		fmt.Fprintln(w, line)
	}
}

// RenderAnswering renders the mascot in answering state.
func RenderAnswering(w io.Writer, noColor bool) {
	frame := []string{
		` /\_/\  `,
		`( ✦.✦ ) `,
		` > ^ <  `,
	}
	for _, line := range frame {
		fmt.Fprintln(w, line)
	}
}

// RenderDone renders the mascot in done state.
func RenderDone(w io.Writer, noColor bool) {
	frame := []string{
		` /\_/\  `,
		`( ^.^ ) `,
		` > ^ <  `,
	}
	for _, line := range frame {
		fmt.Fprintln(w, line)
	}
}

// FormatMascotState returns a formatted mascot state string.
func FormatMascotState(state MascotState, noColor bool) string {
	switch state {
	case MascotIdle:
		return "idle"
	case MascotThinking:
		return "thinking..."
	case MascotAnswering:
		return "answering..."
	case MascotDone:
		return "done"
	default:
		return string(state)
	}
}
