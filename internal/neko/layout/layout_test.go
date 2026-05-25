package layout

import (
	"bytes"
	"strings"
	"testing"
)

func TestTerminalLayoutRegions(t *testing.T) {
	regions := NewRegionLayout(23)
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
	regions := NewRegionLayout(23)
	before := regions.Input
	var out bytes.Buffer
	InputRenderer{}.RenderPrompt(&out)
	after := regions.Input
	if before != after {
		t.Fatalf("input region changed from %+v to %+v", before, after)
	}
	if strings.TrimSpace(out.String()) != ">" {
		t.Fatalf("prompt = %q, want >", out.String())
	}
}
