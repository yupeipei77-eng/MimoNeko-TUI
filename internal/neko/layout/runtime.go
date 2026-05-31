package layout

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type RuntimeRenderer struct {
	NoColor     bool
	events      []string
	lastElapsed time.Duration
}

func NewRuntimeRenderer(noColor bool) RuntimeRenderer {
	return RuntimeRenderer{NoColor: noColor}
}

func (r *RuntimeRenderer) SetNoColor(noColor bool) {
	r.NoColor = noColor
}

func (r *RuntimeRenderer) RenderStage(w io.Writer, stage string) {
	stage = strings.TrimSpace(stage)
	if stage == "" {
		return
	}
	r.events = append(r.events, stage)
	indent := strings.Repeat(" ", DialogPadding)

	// Use spinner character for active stages
	spinner := "⠋"
	if !r.NoColor {
		fmt.Fprintf(w, "%s%s %s\n", indent, paintAccent(spinner, r.NoColor), paintMuted(stage+"...", r.NoColor))
	} else {
		fmt.Fprintf(w, "%s%s %s\n", indent, paintAccent("*", r.NoColor), paintDim(stage+"...", r.NoColor))
	}
}

func (r *RuntimeRenderer) RenderDone(w io.Writer, elapsed time.Duration) {
	label := fmt.Sprintf("done %s %s", "·", formatLatency(elapsed))
	r.events = append(r.events, label)
	r.lastElapsed = elapsed
	indent := strings.Repeat(" ", DialogPadding)
	fmt.Fprintf(w, "%s%s %s\n", indent, paintSuccess("✓", r.NoColor), paintMuted(label, r.NoColor))
}

func (r *RuntimeRenderer) RenderThoughtSummary(w io.Writer) {
	if len(r.events) == 0 {
		return
	}
	indent := strings.Repeat(" ", DialogPadding)
	fmt.Fprintf(w, "%s%s %s\n", indent, paintAccent("+", r.NoColor), paintMuted("Thought: "+formatLatency(r.lastElapsed), r.NoColor))
}

func (r *RuntimeRenderer) Reset() {
	r.events = nil
	r.lastElapsed = 0
}

func RenderBuildBadge(w io.Writer, model string, elapsed time.Duration, noColor bool) {
	indent := strings.Repeat(" ", DialogPadding)
	model = emptyAs(model, "model")

	// Enhanced build badge with icon
	icon := "◆"
	if !noColor {
		fmt.Fprintf(w, "%s%s %s %s %s\n", indent,
			paintAccent(icon, noColor),
			paintValue("Build", noColor),
			paintMuted(model, noColor),
			paintDim(formatLatency(elapsed), noColor))
	} else {
		fmt.Fprintf(w, "%s%s %s %s %s\n", indent,
			paintLabel(icon, noColor),
			paintValue("Build", noColor),
			paintDim(model, noColor),
			paintDim(formatLatency(elapsed), noColor))
	}
}

func formatLatency(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
