package layout

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// StreamRegion represents a distinct region in the streaming output.
type StreamRegion string

const (
	RegionThought StreamRegion = "thought"
	RegionAnswer  StreamRegion = "answer"
	RegionStatus  StreamRegion = "status"
)

// ThinkingState manages the thinking display state.
type ThinkingState struct {
	ShowThoughts bool
	IsThinking   bool
	ThoughtText  []string
	Animator     *DotAnimator
	NoColor      bool
}

// NewThinkingState creates a new ThinkingState.
func NewThinkingState(noColor bool) *ThinkingState {
	return &ThinkingState{
		ShowThoughts: false, // Default: hidden
		NoColor:      noColor,
		Animator:     NewDotAnimator(300 * time.Millisecond),
	}
}

// Toggle switches the thinking display state.
func (ts *ThinkingState) Toggle() {
	ts.ShowThoughts = !ts.ShowThoughts
}

// StartThinking begins the thinking animation.
func (ts *ThinkingState) StartThinking() {
	ts.IsThinking = true
	ts.ThoughtText = nil
	ts.Animator.Start()
}

// StopThinking stops the thinking animation.
func (ts *ThinkingState) StopThinking() {
	ts.IsThinking = false
	ts.Animator.Stop()
}

// AddThought adds a thought line.
func (ts *ThinkingState) AddThought(text string) {
	if text != "" {
		ts.ThoughtText = append(ts.ThoughtText, text)
	}
}

// RenderThinking renders the thinking area.
func (ts *ThinkingState) RenderThinking(w io.Writer) {
	if !ts.IsThinking {
		return
	}

	indent := strings.Repeat(" ", DialogPadding)

	if ts.ShowThoughts {
		// Show full thought text
		ts.renderThoughtsShown(w, indent)
	} else {
		// Show dot animation only
		ts.renderThoughtsHidden(w, indent)
	}
}

// renderThoughtsShown renders the thinking area with full text.
func (ts *ThinkingState) renderThoughtsShown(w io.Writer, indent string) {
	// Thought lines
	if len(ts.ThoughtText) == 0 {
		fmt.Fprintf(w, "%s%s\n", indent, paintMuted("thinking...", ts.NoColor))
	} else {
		for _, line := range ts.ThoughtText {
			fmt.Fprintf(w, "%s%s\n", indent, paintDim("› "+line, ts.NoColor))
		}
	}

	// Empty line separator
	fmt.Fprintln(w)
}

// renderThoughtsHidden renders the thinking area with dot animation.
func (ts *ThinkingState) renderThoughtsHidden(w io.Writer, indent string) {
	dots := ts.Animator.Dots()
	status := paintMuted(fmt.Sprintf("thinking%s", dots), ts.NoColor)
	fmt.Fprintf(w, "%s%s\r", indent, status)
}

// DotAnimator manages the dot animation.
type DotAnimator struct {
	interval time.Duration
	dots     int
	maxDots  int
	running  bool
	ticker   *time.Ticker
}

// NewDotAnimator creates a new DotAnimator.
func NewDotAnimator(interval time.Duration) *DotAnimator {
	return &DotAnimator{
		interval: interval,
		maxDots:  5,
		dots:     1,
	}
}

// Start starts the animation.
func (da *DotAnimator) Start() {
	da.running = true
	da.dots = 1
}

// Stop stops the animation.
func (da *DotAnimator) Stop() {
	da.running = false
	da.dots = 1
}

// Update advances the animation by one frame.
func (da *DotAnimator) Update() {
	if !da.running {
		return
	}
	da.dots++
	if da.dots > da.maxDots {
		da.dots = 1
	}
}

// Dots returns the current dot string.
func (da *DotAnimator) Dots() string {
	if !da.running {
		return ""
	}
	return strings.Repeat(".", da.dots)
}

// IsRunning returns whether the animation is running.
func (da *DotAnimator) IsRunning() bool {
	return da.running
}

// ThoughtToggleHint returns the toggle hint text.
func ThoughtToggleHint(showThoughts bool, noColor bool) string {
	if showThoughts {
		return paintMuted("Thought shown · Ctrl+T to hide", noColor)
	}
	return paintMuted("Thought hidden · Ctrl+T to show", noColor)
}
