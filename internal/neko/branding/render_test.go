package branding

import (
	"bytes"
	"strings"
	"testing"
)

func TestBrandRendererMinimalCenteredLayout(t *testing.T) {
	var out bytes.Buffer
	NewRenderer(true).RenderStaticHeader(&out, sampleHeaderData())
	text := out.String()
	for _, want := range []string{"Mimo", "Neko"} {
		if !strings.Contains(text, want) {
			t.Fatalf("header = %q, want %q", text, want)
		}
	}
	// New design includes cat mascot, so we don't check for forbidden strings
	// The header should contain the brand name and model info
	if !strings.Contains(text, "mimo-v2.5-pro") {
		t.Fatalf("header should contain model info, got %q", text)
	}
}

func TestBrandRendererCentersTitle(t *testing.T) {
	var out bytes.Buffer
	NewRenderer(true).RenderStaticHeader(&out, sampleHeaderData())
	lines := strings.Split(out.String(), "\n")
	if len(lines) < 5 {
		t.Fatalf("header lines = %q", lines)
	}
	titleLine := ""
	for _, line := range lines {
		if strings.Contains(line, "MimoNeko") {
			titleLine = line
		}
	}
	if leadingSpaces(titleLine) < 40 {
		t.Fatalf("title line = %q, want centered title", titleLine)
	}
}

func TestBrandRendererNoColorHeaderOmitsANSI(t *testing.T) {
	var out bytes.Buffer
	NewRenderer(false).RenderNoColorHeader(&out, sampleHeaderData())
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("no-color header contains ANSI: %q", out.String())
	}
}

func TestPremiumThemeUsesWarmPalette(t *testing.T) {
	var out bytes.Buffer
	NewRenderer(false).RenderStaticHeader(&out, sampleHeaderData())
	text := out.String()
	if !strings.Contains(text, WarmAccent) {
		t.Fatalf("header = %q, want warm accent palette", text)
	}
}

func TestNoANSILeakInNoColorMode(t *testing.T) {
	var out bytes.Buffer
	NewRenderer(true).RenderStaticHeader(&out, sampleHeaderData())
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("no-color mode leaked ANSI: %q", out.String())
	}
}

func leadingSpaces(line string) int {
	count := 0
	for _, r := range line {
		if r != ' ' {
			return count
		}
		count++
	}
	return count
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
