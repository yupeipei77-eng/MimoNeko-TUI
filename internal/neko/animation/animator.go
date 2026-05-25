package animation

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/reasonforge/reasonforge/internal/neko/branding"
	"github.com/reasonforge/reasonforge/internal/neko/layout"
)

type FrameAnimator struct {
	Renderer branding.Renderer
	Layout   layout.RegionLayout
	Delay    time.Duration
}

func NewFrameAnimator(renderer branding.Renderer, regions layout.RegionLayout, delay time.Duration) FrameAnimator {
	return FrameAnimator{Renderer: renderer, Layout: regions, Delay: delay}
}

func (a FrameAnimator) RenderStartup(w io.Writer, data branding.HeaderData) {
	if a.Renderer.NoColor || a.Delay <= 0 {
		a.Renderer.RenderStaticHeader(w, data)
		return
	}
	frames := branding.FrameCount()
	if frames == 0 {
		a.Renderer.RenderStaticHeader(w, data)
		return
	}
	for frame := 0; frame < frames; frame++ {
		if frame > 0 {
			fmt.Fprintf(w, "\x1b[%dA", a.Layout.Header.Height)
			a.clearHeader(w)
			fmt.Fprintf(w, "\x1b[%dA", a.Layout.Header.Height-1)
		}
		a.Renderer.RenderAnimatedHeader(w, data, frame)
		time.Sleep(a.Delay)
	}
}

func (a FrameAnimator) HeaderRedrawSequence(data branding.HeaderData, frame int) string {
	var buf bytes.Buffer
	buf.WriteString("\x1b[s")
	fmt.Fprintf(&buf, "\x1b[%d;1H", a.Layout.Header.StartLine)
	a.clearHeader(&buf)
	fmt.Fprintf(&buf, "\x1b[%d;1H", a.Layout.Header.StartLine)
	a.Renderer.RenderAnimatedHeader(&buf, data, frame)
	buf.WriteString("\x1b[u")
	return buf.String()
}

func (a FrameAnimator) clearHeader(w io.Writer) {
	for line := 0; line < a.Layout.Header.Height; line++ {
		fmt.Fprint(w, "\r\x1b[2K")
		if line < a.Layout.Header.Height-1 {
			fmt.Fprint(w, "\x1b[1B")
		}
	}
}
