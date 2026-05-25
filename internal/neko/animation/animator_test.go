package animation

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/reasonforge/reasonforge/internal/neko/branding"
	"github.com/reasonforge/reasonforge/internal/neko/layout"
)

func TestMascotAnimationNoScreenFlicker(t *testing.T) {
	animator := testAnimator(false)
	seq := animator.HeaderRedrawSequence(testHeaderData(), 1)
	for _, forbidden := range []string{"\x1b[2J", "\x1b[3J", "\x1b[H", "\x1b[?1049h"} {
		if strings.Contains(seq, forbidden) {
			t.Fatalf("header redraw uses flicker-prone sequence %q in %q", forbidden, seq)
		}
	}
}

func TestMascotAnimationOnlyRedrawsHeader(t *testing.T) {
	animator := testAnimator(false)
	seq := animator.HeaderRedrawSequence(testHeaderData(), 2)
	if strings.Count(seq, "\x1b[2K") != branding.HeaderLineCount() {
		t.Fatalf("clear-line count = %d, want header height %d", strings.Count(seq, "\x1b[2K"), branding.HeaderLineCount())
	}
	if !strings.Contains(seq, "\x1b[1;1H") {
		t.Fatalf("redraw sequence = %q, want absolute header positioning", seq)
	}
	if !strings.Contains(seq, "Neko") || !strings.Contains(seq, "Forge") {
		t.Fatalf("redraw sequence = %q, want title header", seq)
	}
	if strings.Contains(seq, "Assistant:") || strings.Contains(seq, "User:") || strings.Contains(seq, "( o_o )") {
		t.Fatalf("redraw sequence touched message region: %q", seq)
	}
}

func TestAnimationDoesNotClearMessageRegion(t *testing.T) {
	animator := testAnimator(false)
	seq := animator.HeaderRedrawSequence(testHeaderData(), 0)
	if strings.Contains(seq, "\x1b[J") || strings.Contains(seq, "\x1b[0J") || strings.Contains(seq, "\x1b[1J") || strings.Contains(seq, "\x1b[2J") {
		t.Fatalf("redraw sequence clears message/screen region: %q", seq)
	}
}

func TestNoColorDisablesAnimation(t *testing.T) {
	var out bytes.Buffer
	testAnimator(true).RenderStartup(&out, testHeaderData())
	text := out.String()
	if strings.Contains(text, "\x1b[") {
		t.Fatalf("no-color animation leaked ANSI: %q", text)
	}
	if strings.Count(text, "NekoForge") != 1 {
		t.Fatalf("no-color startup should render one static header, got %q", text)
	}
}

func testAnimator(noColor bool) FrameAnimator {
	return NewFrameAnimator(
		branding.NewRenderer(noColor),
		layout.NewRegionLayout(branding.HeaderLineCount()),
		time.Nanosecond,
	)
}

func testHeaderData() branding.HeaderData {
	return branding.HeaderData{
		Mode:      "Multi-Agent",
		Model:     "mimo-v2.5-pro",
		Provider:  "mimo",
		Context:   "0 / 128k",
		Reasoning: "high",
		Tokens:    "input=0 cached=0 output=0 total=0",
		Cost:      "unavailable",
		Safety:    "dry-run=true worktree=true no auto-apply",
	}
}
