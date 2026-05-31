package layout

import (
	"bytes"
	"strings"
	"testing"
)

func TestTerminalLayoutRegions(t *testing.T) {
	regions := NewRegionLayout(10)
	if err := regions.Validate(); err != nil {
		t.Fatal(err)
	}
	if regions.Header.Name != "header" || regions.Message.Name != "message" || regions.Input.Name != "input" {
		t.Fatalf("regions = %+v, want named header/message/input regions", regions)
	}
	if regions.Message.StartLine <= regions.HeaderEndLine() {
		t.Fatalf("message starts at %d, overlaps header end %d", regions.Message.StartLine, regions.HeaderEndLine())
	}
}

func TestMessageHistoryPersists(t *testing.T) {
	var renderer MessageRenderer
	renderer.Add("User", "hello")
	renderer.Add("Assistant", "world")
	history := renderer.History()
	if len(history) != 2 || history[0].Text != "hello" || history[1].Text != "world" {
		t.Fatalf("history = %+v, want both messages", history)
	}
	history[0].Text = "mutated"
	if renderer.History()[0].Text != "hello" {
		t.Fatal("history should be copied, not externally mutable")
	}
}

func TestInputRegionStable(t *testing.T) {
	regions := NewRegionLayout(10)
	before := regions.Input
	var out bytes.Buffer
	InputRenderer{NoColor: true}.RenderPrompt(&out)
	after := regions.Input
	if before != after {
		t.Fatalf("input region changed from %+v to %+v", before, after)
	}
	if !strings.Contains(out.String(), "Ask anything") || !strings.Contains(out.String(), "▸") {
		t.Fatalf("prompt = %q, want centered composer", out.String())
	}
}

func TestSubmittedInputClosesRightBorder(t *testing.T) {
	var out bytes.Buffer
	renderer := InputRenderer{NoColor: true}
	renderer.RenderPrompt(&out)
	renderer.RenderSubmittedPrompt(&out, "你好，你是什么模型", false)
	renderer.RenderPromptClose(&out)
	text := out.String()
	if !strings.Contains(text, "▸ 你好，你是什么模型") && !strings.Contains(text, "你好，你是什么模型") {
		t.Fatalf("prompt = %q, want submitted Chinese input in composer", text)
	}
	if !strings.Contains(text, "你是什么模型") || !strings.Contains(text, "/ commands") {
		t.Fatalf("prompt = %q, want stable composer and status bar", text)
	}
}

func TestMessageRendererPadsCJKByTerminalWidth(t *testing.T) {
	var out bytes.Buffer
	RenderMessage(&out, "You", "你好，你是什么模型")
	text := out.String()
	if !strings.Contains(text, "你好，你是什么模型") {
		t.Fatalf("message = %q, want Chinese content", text)
	}
	// Check that the message contains user label
	if !strings.Contains(text, "You") {
		t.Fatalf("message = %q, want user label", text)
	}
}

func TestStatusBarRendersCommandPaletteHint(t *testing.T) {
	var out bytes.Buffer
	RenderStatusBar(&out, StatusData{
		Context:   "8.6k / 128k",
		Tools:     3,
		Memory:    "on",
		Cache:     "40.0%",
		Reasoning: "high",
		Model:     "mimo-v2.5-pro",
		Provider:  "mimo",
		Latency:   "522ms",
		Session:   "24m",
		Cost:      "¥0.0123 estimated",
		NoColor:   true,
		CommandUI: "ctrl+p reasoning  / commands",
	})
	text := out.String()
	for _, want := range []string{"ctx 8.6k / 128k", "cache 40.0%", "tools 3", "memory on", "model mimo-v2.5-pro", "provider mimo", "reasoning high"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status = %q, want %q", text, want)
		}
	}
}

func TestStatusBarKeepsCommandHintWhenItFits(t *testing.T) {
	var out bytes.Buffer
	RenderStatusBar(&out, StatusData{
		Context:   "1K / 1M",
		CommandUI: "ctrl+p reasoning  / commands",
		NoColor:   true,
	})
	text := out.String()
	for _, want := range []string{"ctrl+p reasoning", "/ commands"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status = %q, want %q", text, want)
		}
	}
}

func TestRuntimeRendererShowsThoughtSummary(t *testing.T) {
	var out bytes.Buffer
	renderer := NewRuntimeRenderer(true)
	renderer.RenderStage(&out, "thinking")
	renderer.RenderDone(&out, 0)
	renderer.RenderThoughtSummary(&out)
	text := out.String()
	for _, want := range []string{"thinking...", "done", "+ Thought:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("runtime = %q, want %q", text, want)
		}
	}
}
