package branding

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestMascotStateMachineTransitions(t *testing.T) {
	a := NewMascotAnimator(true)
	if got := a.State(); got != MascotIdle {
		t.Fatalf("initial state = %q, want idle", got)
	}
	if a.IsRunning() {
		t.Fatalf("idle should not be running")
	}

	a.SetState(MascotThinking)
	if got := a.State(); got != MascotThinking {
		t.Fatalf("after SetState(thinking) = %q", got)
	}
	if !a.IsRunning() {
		t.Fatalf("thinking should be running")
	}

	a.SetState(MascotError)
	if got := a.State(); got != MascotError {
		t.Fatalf("after SetState(error) = %q", got)
	}
	if a.IsRunning() {
		t.Fatalf("error should not be running")
	}

	now := time.Now()
	a.Tick(now.Add(2 * SuccessWindow))
	if got := a.State(); got != MascotError {
		t.Fatalf("error should not auto-revert; state = %q", got)
	}
}

func TestMascotSuccessAutoRevertsAfter1Second(t *testing.T) {
	a := NewMascotAnimator(true)
	a.SetState(MascotSuccess)
	if got := a.State(); got != MascotSuccess {
		t.Fatalf("after SetState(success) = %q", got)
	}
	now := time.Now()
	a.Tick(now.Add(500 * time.Millisecond))
	if got := a.State(); got != MascotSuccess {
		t.Fatalf("success should not revert at 500ms; state = %q", got)
	}
	a.Tick(now.Add(1500 * time.Millisecond))
	if got := a.State(); got != MascotIdle {
		t.Fatalf("success should revert to idle at 1.5s; state = %q", got)
	}
}

func TestMascotSuccessTimerRestarts(t *testing.T) {
	a := NewMascotAnimator(true)
	now := time.Now()
	a.SetState(MascotSuccess)
	a.successUntil = now.Add(500 * time.Millisecond)
	a.Tick(now.Add(600 * time.Millisecond))
	if got := a.State(); got != MascotIdle {
		t.Fatalf("expected revert at 600ms; state = %q", got)
	}
	a.SetState(MascotSuccess)
	if got := a.State(); got != MascotSuccess {
		t.Fatalf("re-set should re-arm; state = %q", got)
	}
	a.Tick(now.Add(2 * SuccessWindow))
	if got := a.State(); got != MascotIdle {
		t.Fatalf("re-armed timer should fire; state = %q", got)
	}
}

func TestMascotSuccessDoesNotBlockNextInput(t *testing.T) {
	a := NewMascotAnimator(true)
	a.SetState(MascotSuccess)
	a.SetState(MascotThinking)
	if got := a.State(); got != MascotThinking {
		t.Fatalf("thinking over success; state = %q", got)
	}
	a.Tick(time.Now().Add(2 * time.Second))
	if got := a.State(); got != MascotThinking {
		t.Fatalf("Tick must not revert thinking; state = %q", got)
	}
}

func TestMascotLegacyAliasesResolve(t *testing.T) {
	if got := MascotAnswering.ResolvedState(); got != MascotThinking {
		t.Fatalf("Answering resolves to %q, want thinking", got)
	}
	if got := MascotDone.ResolvedState(); got != MascotSuccess {
		t.Fatalf("Done resolves to %q, want success", got)
	}
	if got := MascotIdle.ResolvedState(); got != MascotIdle {
		t.Fatalf("Idle resolves to %q, want idle", got)
	}
	if got := MascotError.ResolvedState(); got != MascotError {
		t.Fatalf("Error resolves to %q, want error", got)
	}
}

func TestMascotFrameLinesEqualWidth(t *testing.T) {
	for _, st := range []MascotState{MascotIdle, MascotThinking, MascotSuccess, MascotError} {
		a := NewMascotAnimator(true)
		a.SetState(st)
		lines, _, block := a.FrameLines()
		for i, line := range lines {
			if cjkWidth(line) != block {
				t.Fatalf("state %q line %d width = %d, want %d (line=%q)",
					st, i, cjkWidth(line), block, line)
			}
		}
	}
}

func TestMascotNoFullwidthParens(t *testing.T) {
	for _, st := range []MascotState{MascotIdle, MascotThinking, MascotSuccess, MascotError} {
		a := NewMascotAnimator(true)
		a.SetState(st)
		lines, right, _ := a.FrameLines()
		combined := strings.Join([]string{
			lines[0], lines[1], lines[2], right,
		}, "")
		for _, r := range combined {
			if r == '（' || r == '）' || r == '【' || r == '】' {
				t.Fatalf("state %q contains fullwidth paren %q", st, r)
			}
		}
	}
}

func TestMascotEmojiOnlyOnLineOne(t *testing.T) {
	for _, st := range []MascotState{MascotIdle, MascotThinking, MascotSuccess, MascotError} {
		a := NewMascotAnimator(true)
		a.SetState(st)
		lines, _, _ := a.FrameLines()
		if !lineHasEmoji(lines[0]) {
			continue
		}
		if lineHasEmoji(lines[1]) || lineHasEmoji(lines[2]) {
			t.Fatalf("state %q has emoji on non-line-1: L1=%q L2=%q L3=%q",
				st, lines[0], lines[1], lines[2])
		}
	}
}

func TestMascotRenderScreenHeaderPositionsRight(t *testing.T) {
	a := NewMascotAnimator(true)
	a.SetState(MascotIdle)
	var out bytes.Buffer
	a.RenderScreenHeader(&out, 1, 80, 80)
	text := out.String()
	if !strings.Contains(text, "\x1b[1;") {
		t.Fatalf("expected absolute row position 1, got %q", text)
	}
	if !strings.Contains(text, "\x1b[2;") || !strings.Contains(text, "\x1b[3;") {
		t.Fatalf("expected 3 rows, got %q", text)
	}
}

func TestMascotRenderScreenHeaderNarrowFallback(t *testing.T) {
	a := NewMascotAnimator(true)
	a.SetState(MascotThinking)
	var out bytes.Buffer
	a.RenderScreenHeader(&out, 1, 12, 12)
	text := out.String()
	if strings.Count(text, "\x1b[") != 1 {
		t.Fatalf("narrow terminal should emit a single escape; got %q", text)
	}
	if !strings.Contains(text, "MimoNeko") {
		t.Fatalf("single-line fallback should contain brand; got %q", text)
	}
	if !strings.Contains(text, "Thinking") {
		t.Fatalf("single-line fallback should reflect state; got %q", text)
	}
}

func TestMascotRenderScreenHeaderErrorLabel(t *testing.T) {
	a := NewMascotAnimator(true)
	a.SetState(MascotError)
	var out bytes.Buffer
	a.RenderScreenHeader(&out, 1, 12, 12)
	if !strings.Contains(out.String(), "Error") {
		t.Fatalf("error fallback should contain 'Error'; got %q", out.String())
	}
}

func TestMascotRenderScreenHeaderBlockTooWide(t *testing.T) {
	a := NewMascotAnimator(true)
	a.SetState(MascotThinking)
	var out bytes.Buffer
	a.RenderScreenHeader(&out, 1, 30, 10)
	if strings.Count(out.String(), "\x1b[") != 1 {
		t.Fatalf("when blockW > width, must fall back to single line; got %q", out.String())
	}
}

func TestMascotRenderScreenHeaderZeroGuards(t *testing.T) {
	a := NewMascotAnimator(true)
	var out bytes.Buffer
	a.RenderScreenHeader(&out, 0, 80, 80)
	if out.Len() != 0 {
		t.Fatalf("top=0 should be no-op; got %q", out.String())
	}
	out.Reset()
	a.RenderScreenHeader(&out, 1, 0, 80)
	if out.Len() != 0 {
		t.Fatalf("rightCol=0 should be no-op; got %q", out.String())
	}
	out.Reset()
	a.RenderScreenHeader(&out, 1, 80, 0)
	if out.Len() != 0 {
		t.Fatalf("width=0 should be no-op; got %q", out.String())
	}
}

func TestMascotRenderScreenHeaderNarrowThreshold(t *testing.T) {
	a := NewMascotAnimator(true)
	a.SetState(MascotIdle)
	var out bytes.Buffer
	a.RenderScreenHeader(&out, 1, 16, 15)
	if strings.Count(out.String(), "\x1b[") != 1 {
		t.Fatalf("width<16 should fall back; got %q", out.String())
	}
}

func TestMascotSingleLineLabelPerState(t *testing.T) {
	cases := map[MascotState]string{
		MascotIdle:     "MimoNeko",
		MascotThinking: "Thinking",
		MascotSuccess:  "Done",
		MascotError:    "Error",
	}
	for st, want := range cases {
		a := NewMascotAnimator(true)
		a.SetState(st)
		got := a.SingleLineLabel()
		if !strings.Contains(got, want) {
			t.Fatalf("state %q label = %q, want substring %q", st, got, want)
		}
	}
}

func TestMascotBlockWidthConsistentAcrossStates(t *testing.T) {
	widths := map[MascotState]int{}
	for _, st := range []MascotState{MascotIdle, MascotThinking, MascotSuccess, MascotError} {
		a := NewMascotAnimator(true)
		a.SetState(st)
		widths[st] = a.BlockWidth()
	}
	for k, v := range widths {
		if v <= 0 {
			t.Fatalf("state %q block width = %d", k, v)
		}
	}
}

func TestFormatMascotState(t *testing.T) {
	if got := FormatMascotState(MascotIdle, true); got != "idle" {
		t.Fatalf("idle label = %q", got)
	}
	if got := FormatMascotState(MascotThinking, true); !strings.Contains(got, "thinking") {
		t.Fatalf("thinking label = %q", got)
	}
	if got := FormatMascotState(MascotError, true); got != "error" {
		t.Fatalf("error label = %q", got)
	}
}

func TestPadRightMascotBehavior(t *testing.T) {
	if got := padRightMascot("ab", 5); got != "ab   " {
		t.Fatalf("pad to 5 = %q", got)
	}
	if got := padRightMascot("abcde", 5); got != "abcde" {
		t.Fatalf("already at target = %q", got)
	}
	if got := padRightMascot("abcdef", 5); got != "abcdef" {
		t.Fatalf("over target = %q", got)
	}
}

func lineHasEmoji(s string) bool {
	for _, r := range s {
		if r >= 0x1F300 && r <= 0x1FAFF {
			return true
		}
	}
	return false
}
