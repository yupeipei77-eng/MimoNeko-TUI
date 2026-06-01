package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	if got := MaskSecret("tp-cn-real-key-5wq8"); got != "tp-c****5wq8" {
		t.Fatalf("MaskSecret() = %q", got)
	}
	if got := MaskSecret(""); got != "missing" {
		t.Fatalf("MaskSecret(empty) = %q", got)
	}
}

func TestNoEmojiMode(t *testing.T) {
	t.Setenv("MIMONEKO_NO_EMOJI", "1")
	var out bytes.Buffer
	PrintSuccess(&out, "ready")
	if strings.Contains(out.String(), "✅") {
		t.Fatalf("output = %q, should not include emoji", out.String())
	}
	if !strings.Contains(out.String(), "[OK] ready") {
		t.Fatalf("output = %q, want ASCII success marker", out.String())
	}
}

func TestNoColorAlsoDisablesEmoji(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out bytes.Buffer
	PrintHeader(&out, "Model Test")
	if strings.Contains(out.String(), "🤖") {
		t.Fatalf("output = %q, should not include emoji", out.String())
	}
	if !strings.Contains(out.String(), "[model] Model Test") {
		t.Fatalf("output = %q, want ASCII model marker", out.String())
	}
}
