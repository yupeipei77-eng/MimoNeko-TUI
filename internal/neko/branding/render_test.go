package branding

import (
	"bytes"
	"strings"
	"testing"
)

func TestBrandRendererStaticHeaderPremiumLayout(t *testing.T) {
	var out bytes.Buffer
	NewRenderer(true).RenderStaticHeader(&out, sampleHeaderData())
	text := out.String()
	for _, want := range []string{` /\_/\`, "( o_o )~", " /|_|\\", "NekoForge", "local AI coding workspace", "Session", "Shortcuts"} {
		if !strings.Contains(text, want) {
			t.Fatalf("header = %q, want %q", text, want)
		}
	}
	if strings.Contains(text, "( o.o )") || strings.Contains(text, "=^.^=") {
		t.Fatalf("header still contains old large ASCII mark: %q", text)
	}
}

func TestMascotAnimationFrames(t *testing.T) {
	var frame0 bytes.Buffer
	var frame1 bytes.Buffer
	NewRenderer(true).RenderAnimatedHeader(&frame0, sampleHeaderData(), 0)
	NewRenderer(true).RenderAnimatedHeader(&frame1, sampleHeaderData(), 1)
	if frame0.String() == frame1.String() {
		t.Fatal("animated frames should differ")
	}
	if !strings.Contains(frame1.String(), "( -_o )~") {
		t.Fatalf("frame 1 = %q, want wink frame", frame1.String())
	}
}

func TestBrandRendererNoColorHeaderOmitsANSI(t *testing.T) {
	var out bytes.Buffer
	NewRenderer(false).RenderNoColorHeader(&out, sampleHeaderData())
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("no-color header contains ANSI: %q", out.String())
	}
}

func TestPremiumThemeUsesColdPalette(t *testing.T) {
	var out bytes.Buffer
	NewRenderer(false).RenderStaticHeader(&out, sampleHeaderData())
	text := out.String()
	if !strings.Contains(text, BrightCyan) || !strings.Contains(text, Cyan) || !strings.Contains(text, SoftWhite) {
		t.Fatalf("header = %q, want cold cyan and soft white palette", text)
	}
	if strings.Contains(text, "\x1b[33m") || strings.Contains(text, "\x1b[93m") {
		t.Fatalf("header should not use amber/yellow as primary color: %q", text)
	}
}

func TestMiniCatHasBody(t *testing.T) {
	for i := 0; i < FrameCount(); i++ {
		lines := CatFrameLines(i)
		if len(lines) != 3 {
			t.Fatalf("frame %d has %d lines, want head/body/tail body sprite", i, len(lines))
		}
		joined := strings.Join(lines, "\n")
		for _, want := range []string{`/\_/\`, "(", ")", "|_|", "~"} {
			if !strings.Contains(joined, want) {
				t.Fatalf("frame %d = %q, want body element %q", i, joined, want)
			}
		}
	}
}

func TestNoANSILeakInNoColorMode(t *testing.T) {
	var out bytes.Buffer
	NewRenderer(true).RenderStaticHeader(&out, sampleHeaderData())
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("no-color mode leaked ANSI: %q", out.String())
	}
}

func TestMiniCatFramesAreCompact(t *testing.T) {
	if FrameCount() < 2 || FrameCount() > 6 {
		t.Fatalf("frame count = %d, want 2..6", FrameCount())
	}
	for i := 0; i < FrameCount(); i++ {
		lines := CatFrameLines(i)
		if len(lines) > 3 {
			t.Fatalf("frame %d has too many lines: %q", i, lines)
		}
		for _, line := range lines {
			if len(line) > 10 {
				t.Fatalf("frame %d line = %q, want compact mascot", i, line)
			}
		}
	}
}

func sampleHeaderData() HeaderData {
	return HeaderData{
		Mode:      "Multi-Agent",
		Model:     "mimo-v2.5-pro",
		Provider:  "mimo",
		Context:   "0 / 128k tokens",
		Reasoning: "high",
		Tokens:    "input=0 cached=0 output=0 total=0",
		Cost:      "unavailable (pricing not configured)",
		Safety:    "dry-run=true worktree=true no auto-apply",
	}
}
