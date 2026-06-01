package layout

import (
	"fmt"
	"io"
	"strings"
)

type StatusData struct {
	Context           string
	Tools             int
	Memory            string
	Cache             string
	Reasoning         string
	Model             string
	Provider          string
	Latency           string
	Session           string
	Cost              string
	NoColor           bool
	Compact           bool
	CommandUI         string
	ThoughtToggleHint string
}

func RenderStatusBar(w io.Writer, data StatusData) {
	indent := strings.Repeat(" ", DialogPadding)
	innerWidth := DialogWidth - 4
	memory := emptyAs(data.Memory, "on")
	cache := emptyAs(data.Cache, "n/a")
	commandUI := emptyAs(data.CommandUI, "/ commands")

	// Build status items with icons and colors
	items := []string{
		fmt.Sprintf("ctx %s", paintValue(emptyAs(data.Context, "unknown"), data.NoColor)),
		fmt.Sprintf("cache %s", paintValue(cache, data.NoColor)),
		fmt.Sprintf("tools %s", paintValue(fmt.Sprintf("%d", data.Tools), data.NoColor)),
		fmt.Sprintf("memory %s", paintValue(memory, data.NoColor)),
		fmt.Sprintf("model %s", paintAccent(emptyAs(data.Model, "unknown"), data.NoColor)),
		fmt.Sprintf("provider %s", paintLabel(emptyAs(data.Provider, "unknown"), data.NoColor)),
	}

	if strings.TrimSpace(data.Reasoning) != "" {
		items = append(items, "reasoning "+paintHighlight(strings.TrimSpace(data.Reasoning), data.NoColor))
	}
	if data.Latency != "" {
		items = append(items, "latency "+paintSuccess(data.Latency, data.NoColor))
	}
	if data.Session != "" {
		items = append(items, "session "+paintMuted(data.Session, data.NoColor))
	}
	if data.Cost != "" {
		items = append(items, "cost "+paintValue(data.Cost, data.NoColor))
	}
	if data.ThoughtToggleHint != "" {
		items = append(items, data.ThoughtToggleHint)
	}

	left := strings.Join(items, "  ")
	line := fitStatus(left, commandUI, innerWidth)

	// Render with subtle styling
	fmt.Fprintf(w, "%s%s\n", indent, paintMuted(" "+line, data.NoColor))
}

func fitStatus(left, right string, width int) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if right == "" {
		return padRight(left, width)
	}
	space := width - terminalWidth(left) - terminalWidth(right)
	if space < 2 {
		return padRight(truncateToWidth(left, width), width)
	}
	if space < 1 {
		space = 1
	}
	return left + strings.Repeat(" ", space) + right
}
