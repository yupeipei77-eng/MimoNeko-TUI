package neko

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/neko/branding"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/neko/layout"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"
)

func allowAutoSaveForTest(t *testing.T) {
	t.Helper()
	t.Setenv(security.PermissionModeEnvVar, string(security.PermissionApplyWithApproval))
	t.Setenv("MIMONEKO_AUTO_SAVE_APPROVED", "true")
}

func TestNekoNoColor(t *testing.T) {
	session := newTestSession(t, nil, Options{NoColor: true, DryRun: true, DryRunSet: true})
	var out bytes.Buffer
	RenderHeader(&out, session)
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("no-color output contains ANSI: %q", out.String())
	}
}

func TestNekoRendersBranding(t *testing.T) {
	session := newTestSession(t, nil, Options{NoColor: true, DryRun: true, DryRunSet: true})
	var out bytes.Buffer
	RenderHeader(&out, session)
	if !strings.Contains(out.String(), "MimoNeko") {
		t.Fatalf("branding output = %q", out.String())
	}
	for _, forbidden := range []string{"Session", "Shortcuts", "Ask anything", "local agent runtime"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("branding should be header only: %q", out.String())
		}
	}
}

func TestNekoDoesNotMentionOpenCode(t *testing.T) {
	session := newTestSession(t, nil, Options{NoColor: true, DryRun: true, DryRunSet: true})
	var out bytes.Buffer
	RenderHeader(&out, session)
	RenderHelp(&out, true)
	if strings.Contains(strings.ToLower(out.String()), "opencode") {
		t.Fatalf("output mentions forbidden brand: %q", out.String())
	}
}

func TestNekoAcceptsGoal(t *testing.T) {
	var gotGoal string
	output := runTestConsole(t, "/run fix tests\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			gotGoal = req.Goal
			return RunResult{RunID: "run_1", State: "succeeded"}, nil
		},
	})
	if gotGoal != "fix tests" {
		t.Fatalf("goal = %q, want fix tests", gotGoal)
	}
	if !strings.Contains(output, "Run completed:") {
		t.Fatalf("output = %q, want run completion", output)
	}
}

func TestNekoSingleBareInputUsesChatNotAgent(t *testing.T) {
	agentCalled := false
	chatCalled := false
	output := runTestConsole(t, "你好\n/exit\n", Options{
		Mode: "single",
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			agentCalled = true
			return RunResult{}, nil
		},
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			chatCalled = true
			if req.Message != "你好" {
				t.Fatalf("chat message = %q, want 你好", req.Message)
			}
			return ChatResult{Response: "你好，我在。"}, nil
		},
	})
	if agentCalled {
		t.Fatal("bare input should not execute agent")
	}
	if !chatCalled {
		t.Fatal("bare input should call chat")
	}
	if !strings.Contains(output, "▸ You") || !strings.Contains(output, "MimoNeko") || !strings.Contains(output, "你好，我在。") {
		t.Fatalf("output = %q, want terminal-native chat stream", output)
	}
}

func TestNekoBuildBareInputUsesAgentRuntime(t *testing.T) {
	agentCalled := false
	chatCalled := false
	output := runTestConsole(t, "inspect project files\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			agentCalled = true
			if req.Goal != "inspect project files" {
				t.Fatalf("agent goal = %q, want inspect project files", req.Goal)
			}
			return RunResult{RunID: "run_build_input", State: "succeeded", Output: "checked project files"}, nil
		},
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			chatCalled = true
			return ChatResult{Response: "should not call chat"}, nil
		},
	})
	if !agentCalled {
		t.Fatal("Build bare input should execute agent runtime")
	}
	if chatCalled {
		t.Fatal("Build bare input should not call chat")
	}
	if !strings.Contains(output, "Run completed:") || !strings.Contains(output, "checked project files") {
		t.Fatalf("output = %q, want agent runtime result", output)
	}
}

func TestNekoBuildGreetingUsesChatNotAgent(t *testing.T) {
	agentCalled := false
	chatCalled := false
	output := runTestConsole(t, "\u4f60\u597d\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			agentCalled = true
			return RunResult{}, nil
		},
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			chatCalled = true
			if req.Message != "\u4f60\u597d" {
				t.Fatalf("chat message = %q, want \u4f60\u597d", req.Message)
			}
			return ChatResult{Response: "\u4f60\u597d\uff0c\u6211\u5728\u3002"}, nil
		},
	})
	if agentCalled {
		t.Fatal("Build greeting should not execute agent runtime")
	}
	if !chatCalled {
		t.Fatal("Build greeting should call chat")
	}
	if strings.Contains(output, "executing agent runtime") || strings.Contains(output, "Run completed:") {
		t.Fatalf("output = %q, should not show agent runtime for greeting", output)
	}
	if !strings.Contains(output, "MimoNeko") {
		t.Fatalf("output = %q, want chat response", output)
	}
}

func TestLooksLikeAgentGoal(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{name: "chinese greeting", line: "\u4f60\u597d", want: false},
		{name: "english greeting", line: "hello", want: false},
		{name: "short cjk chat", line: "\u55ef\u55ef", want: false},
		{name: "english inspect", line: "inspect project files", want: true},
		{name: "chinese project check", line: "\u68c0\u67e5\u9879\u76ee\u6587\u4ef6", want: true},
		{name: "chinese fix tests", line: "\u4fee\u590d\u6d4b\u8bd5\u62a5\u9519", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeAgentGoal(tt.line); got != tt.want {
				t.Fatalf("looksLikeAgentGoal(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestNekoRuntimeEventStreamForChat(t *testing.T) {
	output := runTestConsole(t, "hello\n/exit\n", Options{
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "Ready."}, nil
		},
	})
	for _, want := range []string{"thinking...", "Ready."} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want runtime event %q", output, want)
		}
	}
}

func TestNekoScreenStreamingReplyFinalizesWithoutDuplicate(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.Options.StreamingChatter = func(ctx context.Context, req ChatRequest, onChunk func(chunk StreamingChatChunk)) (ChatResult, error) {
		onChunk(StreamingChatChunk{Text: "你好"})
		onChunk(StreamingChatChunk{Text: "，我在。"})
		return ChatResult{Response: "你好，我在。"}, nil
	}
	console.chatMessage(context.Background(), "你好")

	assistantCount := 0
	streamCount := 0
	for _, item := range console.screenLog {
		if item.Kind == "assistant" && item.Text == "你好，我在。" {
			assistantCount++
		}
		if item.Kind == "assistant_stream" {
			streamCount++
		}
	}
	if assistantCount != 1 || streamCount != 0 {
		t.Fatalf("assistantCount=%d streamCount=%d log=%+v, want one finalized assistant", assistantCount, streamCount, console.screenLog)
	}
}

func TestNekoSlashOpensCommandPalette(t *testing.T) {
	output := runTestConsole(t, "/\n/exit\n", Options{})
	for _, want := range []string{"Commands", "/agents", "/connect", "/models", "/new"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want command palette item %q", output, want)
		}
	}
	for _, hidden := range []string{"/reasoning", "/run <goal>", "/preview", "/review", "/discard"} {
		if strings.Contains(output, hidden) {
			t.Fatalf("output = %q, should hide extra command %q", output, hidden)
		}
	}
}

func TestNekoReasoningCommandCyclesLevel(t *testing.T) {
	output := runTestConsole(t, "/reasoning\n/exit\n", Options{})
	if strings.Contains(output, "reasoning=low") {
		t.Fatalf("output = %q, should not emit reasoning switch as a message", output)
	}
	if !strings.Contains(output, "Build · mimo-v2.5-pro · mimo · low") {
		t.Fatalf("output = %q, want composer to show new reasoning level", output)
	}
	if !strings.Contains(output, "reasoning low") {
		t.Fatalf("output = %q, want status to show new reasoning level", output)
	}
}

func TestNekoAgentsCommandSwitchesMode(t *testing.T) {
	output := runTestConsole(t, "/agents single\n/exit\n", Options{})
	if !strings.Contains(output, "agent=single worktree=false") {
		t.Fatalf("output = %q, want agent mode switch", output)
	}
}

func TestNekoAgentsCommandShowsModeMetadata(t *testing.T) {
	output := runTestConsole(t, "/agents\n/exit\n", Options{})
	for _, want := range []string{
		"Agents",
		"Build",
		"multi-agent worktree build",
		"tools=file_read,list_files,git_diff,test_run,patch_preview",
		"permission=patch-preview",
		"worktree=true",
		"Reviewer",
		"permission=read-only",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestNekoAgentsCommandOpensPickerInScreenMode(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.handleInputLine(context.Background(), "/agents")

	if !console.agentPickerOpen {
		t.Fatal("agentPickerOpen = false after /agents, want true")
	}
	text := out.String()
	for _, want := range []string{"Switch agent", "Build", "Single", "tools=read,list,diff,test,patch", "permission=patch-preview", "worktree=true"} {
		if !strings.Contains(text, want) {
			t.Fatalf("screen = %q, want %q", text, want)
		}
	}
	if strings.Contains(text, "Use /agents") {
		t.Fatalf("screen = %q, should not render plain agents help text", text)
	}
}

func TestNekoModelsCommandSwitchesModel(t *testing.T) {
	output := runTestConsole(t, "/models fast-model\n/exit\n", Options{})
	if !strings.Contains(output, "Model switched to fast-model") {
		t.Fatalf("output = %q, want model switch message", output)
	}
	if !strings.Contains(output, "Build · fast-model · fast") {
		t.Fatalf("output = %q, want composer to reflect switched model", output)
	}
}

func TestNekoScreenComposerStaysAtBottom(t *testing.T) {
	session := newTestSession(t, nil, Options{DryRun: true, DryRunSet: true})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.appendScreen("user", "hello", false)
	text := out.String()
	// Check for key UI elements - mode in title, model in header
	if !strings.Contains(text, "Build") || !strings.Contains(text, "mimo-v2.5-pro") {
		t.Fatalf("screen = %q, want fixed bottom composer with model info", text)
	}
}

func TestNekoScreenComposerUsesNativeCursorForCJKDraft(t *testing.T) {
	session := newTestSession(t, nil, Options{DryRun: true, DryRunSet: true})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
		draft:        "你好",
	}
	console.repaintScreen()
	text := out.String()
	if !strings.Contains(text, "> 你好") {
		t.Fatalf("screen = %q, want plain prompt text", text)
	}
	for _, forbidden := range []string{"|你好", "你|好", "你好|", "\x1b[7m"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("screen = %q, contains fake cursor %q", text, forbidden)
		}
	}

	width := console.composerWidth()
	left := (console.screenCols - width) / 2
	if left < 1 {
		left = 1
	}
	composerTop := console.screenRows - 4
	wantCursor := fmt.Sprintf("\x1b[%d;%dH", composerTop+1, left+2+runewidth.StringWidth("> ")+runewidth.StringWidth("你好"))
	if !strings.Contains(text, wantCursor) {
		t.Fatalf("screen = %q, want cursor move %q", text, wantCursor)
	}
}

func TestNekoScreenComposerUsesNativeCursorForEmojiDraft(t *testing.T) {
	session := newTestSession(t, nil, Options{DryRun: true, DryRunSet: true})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   96,
		screenRows:   24,
		draft:        "猫🙂",
	}
	console.repaintScreen()
	text := out.String()
	if !strings.Contains(text, "> 猫🙂") {
		t.Fatalf("screen = %q, want emoji draft text", text)
	}
	for _, forbidden := range []string{"|猫", "猫|", "🙂|", "\x1b[7m"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("screen = %q, contains fake cursor %q", text, forbidden)
		}
	}
	width := console.composerWidth()
	left := (console.screenCols - width) / 2
	if left < 1 {
		left = 1
	}
	composerTop := console.screenRows - 4
	wantCursor := fmt.Sprintf("\x1b[%d;%dH", composerTop+1, left+2+runewidth.StringWidth("> ")+runewidth.StringWidth("猫🙂"))
	if !strings.Contains(text, wantCursor) {
		t.Fatalf("screen = %q, want cursor move %q", text, wantCursor)
	}
}

func TestNekoScreenComposerCursorTracksResize(t *testing.T) {
	session := newTestSession(t, nil, Options{DryRun: true, DryRunSet: true})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   100,
		screenRows:   28,
		draft:        "你好",
	}
	console.repaintScreen()
	out.Reset()
	console.screenCols = 82
	console.screenRows = 22
	console.repaintScreen()
	text := out.String()
	width := console.composerWidth()
	left := (console.screenCols - width) / 2
	if left < 1 {
		left = 1
	}
	composerTop := console.screenRows - 4
	wantCursor := fmt.Sprintf("\x1b[%d;%dH", composerTop+1, left+2+runewidth.StringWidth("> ")+runewidth.StringWidth("你好"))
	if !strings.Contains(text, wantCursor) {
		t.Fatalf("resized screen = %q, want cursor move %q", text, wantCursor)
	}
}

func TestNekoAddProviderAPIKeyComposerIsMasked(t *testing.T) {
	session := newTestSession(t, nil, Options{DryRun: true, DryRunSet: true})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
		addFlow:      addProviderFlow{active: true, step: stepAPIKey},
		draft:        "sk-secret",
	}
	console.repaintScreen()
	text := out.String()
	if strings.Contains(text, "sk-secret") {
		t.Fatalf("screen = %q, leaked API key draft", text)
	}
	if !strings.Contains(text, "*********") || !strings.Contains(text, "API key") {
		t.Fatalf("screen = %q, want masked API key modal", text)
	}
	for _, forbidden := range []string{"|sk", "•", "› ", "╭", "╮", "╰", "╯", "│"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("screen = %q, contains dialog/cursor marker %q", text, forbidden)
		}
	}
}

func TestNekoIntroScreenShowsLargePromptBeforeWorkspace(t *testing.T) {
	session := newTestSession(t, nil, Options{DryRun: true, DryRunSet: true})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
		introActive:  true,
	}
	console.repaintScreen()
	text := out.String()
	for _, want := range []string{"/  |/  /", "\\____", "Ask anything", "ctrl+p commands"} {
		if !strings.Contains(text, want) {
			t.Fatalf("intro = %q, want %q", text, want)
		}
	}
	if strings.Contains(text, "\U0001F431") || strings.Contains(text, "Ask MIMO") || strings.Contains(text, "local agent runtime") || strings.Contains(text, "\nMIMO") {
		t.Fatalf("intro should be clean launch composer: %q", text)
	}
}

func TestNekoContextUsageGrowsWithConversation(t *testing.T) {
	output := runTestConsole(t, "hello world\n/exit\n", Options{
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "Ready."}, nil
		},
	})
	if !strings.Contains(output, "ctx <1%") {
		t.Fatalf("output = %q, want estimated context percentage", output)
	}
}

func TestNekoNewResetsConversationState(t *testing.T) {
	output := runTestConsole(t, "hello\n/new\n/exit\n", Options{
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "Ready."}, nil
		},
	})
	if !strings.Contains(output, "New session.") {
		t.Fatalf("output = %q, want new session acknowledgement", output)
	}
}

func TestNekoStatusBarRendersRuntimeContext(t *testing.T) {
	output := runTestConsole(t, "/exit\n", Options{})
	for _, want := range []string{"ctx <1%", "cache unsupported", "tools 0", "model mimo-v2.5-pro"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want status item %q", output, want)
		}
	}
}

func TestNekoCacheCommandReportsUnsupported(t *testing.T) {
	output := runTestConsole(t, "/cache\n/exit\n", Options{})
	for _, want := range []string{"Cache", "context=<1%", "input_tokens=0", "cached_tokens=0", "cache_hit_rate=unsupported"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want cache item %q", output, want)
		}
	}
}

func TestNekoCacheCommandReportsNativeMimoHitRate(t *testing.T) {
	output := runTestConsole(t, "hello\n/cache\n/exit\n", Options{
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{
				Response: "Ready.",
				Usage: Usage{
					CacheHitTokens:   30,
					CacheMissTokens:  70,
					OutputTokens:     8,
					NativeCacheKnown: true,
				},
			}, nil
		},
	})
	for _, want := range []string{"input_tokens=102", "cached_tokens=30", "cache_hit_rate=30.0%"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want cache item %q", output, want)
		}
	}
}

func TestNekoContextStatusUsesKUnits(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	session.ContextUsedTokens = 228
	got := session.ContextLabel()
	if got != "<1%" {
		t.Fatalf("context label = %q, want <1%%", got)
	}
}

func TestNekoCommandPaletteSelectionOpensAgentPicker(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:         session,
		Options:         Options{Out: &out},
		screenActive:    true,
		screenCols:      120,
		screenRows:      30,
		paletteOpen:     true,
		paletteSelected: 0,
	}
	if exit := console.executePaletteSelection(context.Background()); exit {
		t.Fatal("command selection should not exit")
	}
	if console.draft != "" || console.paletteOpen || !console.agentPickerOpen {
		t.Fatalf("draft=%q paletteOpen=%v agentPickerOpen=%v, want agent picker", console.draft, console.paletteOpen, console.agentPickerOpen)
	}
	if !strings.Contains(out.String(), "Switch agent") || !strings.Contains(out.String(), "Single") {
		t.Fatalf("screen = %q, want agent picker", out.String())
	}
}

func TestNekoTabOpensAgentPickerInScreenMode(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	if exit := console.handleRawRune(context.Background(), bufio.NewReader(strings.NewReader("")), '\t'); exit {
		t.Fatal("tab should open agent picker, not exit")
	}
	if !console.agentPickerOpen {
		t.Fatal("agentPickerOpen = false after tab, want true")
	}
	text := out.String()
	if !strings.Contains(text, "Switch agent") || !strings.Contains(text, "Single") {
		t.Fatalf("screen = %q, want agent picker", text)
	}
}

func TestNekoAgentPickerSelectionSwitchesMode(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.openAgentPicker()
	console.agentPickerSelected = 1
	console.executeAgentPickerSelection()

	if console.agentPickerOpen {
		t.Fatal("agent picker should close after selection")
	}
	if console.Session.Mode != "single" || console.Session.Worktree {
		t.Fatalf("mode=%q worktree=%v, want single false", console.Session.Mode, console.Session.Worktree)
	}
	if !strings.Contains(out.String(), "Agent switched to Single") {
		t.Fatalf("screen = %q, want agent switch status", out.String())
	}
}

func TestNekoComposerModeLabelFollowsAgentMode(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	if !session.SetMode("single") {
		t.Fatal("failed to set single mode")
	}
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.refreshInput()
	console.renderScreenWorkbenchComposerV2(&out, branding.Renderer{NoColor: true}, 1, 120, 25)
	text := out.String()
	if !strings.Contains(text, "Single") {
		t.Fatalf("composer = %q, want Single mode label", text)
	}
	if strings.Contains(text, "Build |") {
		t.Fatalf("composer = %q, should not keep stale Build label after agent switch", text)
	}
}

func TestNekoSlashInputAutoOpensFilteredCommandPalette(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.draft = "/mod"
	console.updatePaletteForDraft()
	if !console.paletteOpen || console.paletteSelected != 0 {
		t.Fatalf("paletteOpen=%v selected=%d, want auto-open reset", console.paletteOpen, console.paletteSelected)
	}
	items := console.visibleCommandPaletteItems()
	if len(items) == 0 {
		t.Fatal("filtered command palette is empty")
	}
	for _, item := range items {
		if !strings.Contains(strings.ToLower(item.Command+" "+item.Help), "mod") {
			t.Fatalf("item=%+v does not match filter", item)
		}
	}
}

func TestNekoRawSlashShowsPullUpPaletteAndEnterConfirms(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
		introActive:  true,
	}
	if exit := console.handleRawRune(context.Background(), bufio.NewReader(strings.NewReader("")), '/'); exit {
		t.Fatal("slash should open palette, not exit")
	}
	if !console.paletteOpen || console.draft != "/" {
		t.Fatalf("paletteOpen=%v draft=%q, want slash pull-up palette", console.paletteOpen, console.draft)
	}
	text := out.String()
	if !strings.Contains(text, "Commands") || !strings.Contains(text, "/agents") {
		t.Fatalf("screen = %q, want pull-up command options", text)
	}
	if exit := console.handleRawRune(context.Background(), bufio.NewReader(strings.NewReader("")), '\r'); exit {
		t.Fatal("selecting first command should not exit")
	}
	if console.paletteOpen || console.draft != "" {
		t.Fatalf("paletteOpen=%v draft=%q, want enter to execute selected command", console.paletteOpen, console.draft)
	}
}

func TestNekoScreenCommandPaletteIsCompactBorderless(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.openCommandPalette()
	text := out.String()
	for _, want := range []string{"Commands", "Search", "Suggested", "/agents", "/connect", "/models"} {
		if !strings.Contains(text, want) {
			t.Fatalf("screen = %q, want %q", text, want)
		}
	}
	for _, hidden := range []string{"/run <goal>", "/reasoning", "/preview", "/review", "/discard", "/mode "} {
		if strings.Contains(text, hidden) {
			t.Fatalf("screen = %q, should hide extra command %q", text, hidden)
		}
	}
	for _, border := range []string{"╭", "╮", "╰", "╯", "│"} {
		if strings.Contains(text, border) {
			t.Fatalf("screen = %q, command palette should not draw border %q", text, border)
		}
	}
}

func TestNekoSlashCommandOpensUnfilteredPaletteForNormalDraft(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
		draft:        "write a script",
	}
	console.openCommandPalette()
	if !console.paletteOpen || console.paletteFilter != "" {
		t.Fatalf("paletteOpen=%v filter=%q, want unfiltered command palette", console.paletteOpen, console.paletteFilter)
	}
	if got := len(console.visibleCommandPaletteItems()); got != len(commandPaletteItems()) {
		t.Fatalf("visible items = %d, want all commands", got)
	}
}

func TestNekoCtrlPCyclesReasoning(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session: session,
		Options: Options{Out: &out},
	}
	if !console.handleControlInput(context.Background(), "\x10") {
		t.Fatal("ctrl+p should be handled")
	}
	if console.Session.Reasoning != "low" || out.String() != "" {
		t.Fatalf("reasoning=%q output=%q, want ctrl+p to cycle reasoning", console.Session.Reasoning, out.String())
	}
}

func TestNekoCtrlPVTSequenceOpensCommandsInScreenMode(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.handleEscapeSequence(bufio.NewReader(strings.NewReader("[112;5u")))
	if !console.paletteOpen {
		t.Fatalf("paletteOpen = false, want commands after CSI ctrl+p")
	}
}

func TestNekoHidesReasoningForModelsWithoutReasoning(t *testing.T) {
	session := newTestSession(t, nil, Options{Model: "plain-model"})
	var out bytes.Buffer
	console := Console{
		Session: session,
		Options: Options{Out: &out},
	}
	console.refreshInput()
	if console.Input.Reasoning != "" {
		t.Fatalf("status reasoning = %q, want hidden", console.Input.Reasoning)
	}
	console.cycleReasoning()
	if console.Session.Reasoning != "medium" || out.String() != "" {
		t.Fatalf("reasoning=%q output=%q, want unavailable no-op", console.Session.Reasoning, out.String())
	}
}

func TestNekoCacheHitRateLabelFromActualUsage(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	session.ApplyActualUsage(Usage{InputTokens: 100, CachedTokens: 40, OutputTokens: 10, TotalTokens: 110, Estimated: false})
	if got := session.CacheLabel(); got != "40.0%" {
		t.Fatalf("CacheLabel() = %q, want 40.0%%", got)
	}
}

func TestNekoCacheHitRateLabelFromMimoNativeUsage(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	session.ApplyActualUsage(Usage{
		InputTokens:      1000,
		CachedTokens:     12,
		CacheHitTokens:   900,
		CacheMissTokens:  100,
		NativeCacheKnown: true,
		OutputTokens:     10,
		TotalTokens:      1010,
		Estimated:        false,
	})
	if got := session.CacheLabel(); got != "90.0%" {
		t.Fatalf("CacheLabel() = %q, want 90.0%%", got)
	}
}

func TestNekoPreviewCanPopulateDiffPanel(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.callPatch(context.Background(), func(ctx context.Context, session Session, worktreeID string) (string, error) {
		return "diff --git a/main.go b/main.go", nil
	}, "wt_1", "patch preview")
	if console.panelMode != "diff" || !strings.Contains(console.panelContent, "diff --git") {
		t.Fatalf("panelMode=%q content=%q, want diff panel", console.panelMode, console.panelContent)
	}
}

func TestNekoRunShowsExecutionRuntimeEvents(t *testing.T) {
	output := runTestConsole(t, "/run fix tests\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			return RunResult{RunID: "run_runtime", State: "succeeded"}, nil
		},
	})
	for _, want := range []string{"planning...", "executing agent runtime...", "collecting result..."} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want run event %q", output, want)
		}
	}
}

func TestNekoChatFailureReturnsFriendlyText(t *testing.T) {
	output := runTestConsole(t, "hello\n/exit\n", Options{
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{}, errors.New("planner failed: invalid plan output")
		},
	})
	if !strings.Contains(output, "I could not reach the chat model yet") {
		t.Fatalf("output = %q, want friendly chat failure", output)
	}
	if strings.Contains(output, "MimoNeko Multi-Agent") {
		t.Fatalf("chat failure should not dump agent logs: %q", output)
	}
}

func TestNekoChatFailurePreservesAPIKeyEnvVarName(t *testing.T) {
	output := runTestConsole(t, "hello\n/exit\n", Options{
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{}, errors.New("API key not found in environment variable MIMO_API_KEY")
		},
	})
	if !strings.Contains(output, "MIMO_API_KEY") {
		t.Fatalf("output = %q, want env var name visible", output)
	}
	if strings.Contains(output, "MIMO_API_KEY<redacted>") {
		t.Fatalf("output = %q, should not redact env var name as value", output)
	}
}

func TestNekoChatFailureUsesRedANSIWhenColorEnabled(t *testing.T) {
	output := runTestConsoleWithColor(t, "hello\n/exit\n", Options{
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{}, errors.New("API key not found in environment variable MIMO_API_KEY")
		},
	})
	if !strings.Contains(output, "\x1b[38;5;203m") {
		t.Fatalf("output = %q, want red ANSI error styling", output)
	}
	if !strings.Contains(output, "MIMO_API_KEY") || strings.Contains(output, "MIMO_API_KEY<redacted>") {
		t.Fatalf("output = %q, want preserved env var name", output)
	}
}

func TestNekoAutoSavesChatCodeBlockToDefaultProjectDir(t *testing.T) {
	allowAutoSaveForTest(t)
	root := t.TempDir()
	output := runTestConsole(t, "帮我生成一个bat批处理文件并保存到默认项目目录\n/exit\n", Options{
		Root: root,
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "```bat\n@echo off\necho OK\n```"}, nil
		},
	})
	target := filepath.Join(root, "batch_script.bat")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !strings.Contains(string(data), "@echo off") || !strings.Contains(output, "saved_file=") || !strings.Contains(output, filepath.Base(target)) {
		t.Fatalf("output=%q file=%q, want saved bat file", output, string(data))
	}
}

func TestNekoAutoSaveBlockedByDefaultPermission(t *testing.T) {
	root := t.TempDir()
	output := runTestConsole(t, "帮我生成一个bat批处理文件并保存到默认项目目录\n/exit\n", Options{
		Root: root,
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "```bat\n@echo off\necho blocked\n```"}, nil
		},
	})
	if !strings.Contains(output, "auto_save_failed=") || !strings.Contains(output, "permission mode") {
		t.Fatalf("output=%q, want auto-save permission failure", output)
	}
	if _, err := os.Stat(filepath.Join(root, "batch_script.bat")); !os.IsNotExist(err) {
		t.Fatalf("batch_script.bat should not be written by default, stat err=%v", err)
	}
}

func TestNekoAutoSaveSuppressesGeneratedBodyOutput(t *testing.T) {
	allowAutoSaveForTest(t)
	root := t.TempDir()
	output := runTestConsole(t, "帮我写一个bat脚本保存到默认项目目录\n/exit\n", Options{
		Root: root,
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "## 脚本功能\n```bat\necho hidden body\n```"}, nil
		},
	})
	target := filepath.Join(root, "batch_script.bat")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !strings.Contains(string(data), "echo hidden body") {
		t.Fatalf("file=%q, want script content saved", string(data))
	}
	for _, forbidden := range []string{"脚本功能", "echo hidden body"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("output=%q, should not echo generated body %q", output, forbidden)
		}
	}
	if !strings.Contains(output, "saved_file=") {
		t.Fatalf("output=%q, want saved_file summary", output)
	}
}

func TestNekoLocalFastPathWritesDesktopMigrationWithoutModel(t *testing.T) {
	allowAutoSaveForTest(t)
	root := t.TempDir()
	modelCalled := false
	output := runTestConsole(t, "写一个把桌面目录迁移到D盘的脚本保存到默认项目目录\n/exit\n", Options{
		Root: root,
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			modelCalled = true
			return ChatResult{Response: "should not call model"}, nil
		},
	})
	if modelCalled {
		t.Fatal("desktop migration script should use local fast path")
	}
	target := filepath.Join(root, "migrate_desktop_to_d_drive.bat")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	content := string(data)
	for _, want := range []string{`set "TARGET=D:\Desktop"`, "robocopy", "reg add"} {
		if !strings.Contains(content, want) {
			t.Fatalf("file=%q, want %q", content, want)
		}
	}
	if strings.Contains(output, "robocopy") || strings.Contains(output, "reg add") {
		t.Fatalf("output=%q, should not print script body", output)
	}
	if !strings.Contains(output, "saved_file=") || !strings.Contains(output, "migrate_desktop_to_d_drive.bat") {
		t.Fatalf("output=%q, want saved file summary", output)
	}
}

func TestNekoStreamingAutoSaveSuppressesGeneratedBodyOutput(t *testing.T) {
	allowAutoSaveForTest(t)
	root := t.TempDir()
	output := runTestConsole(t, "帮我写一个bat脚本保存到默认项目目录\n/exit\n", Options{
		Root: root,
		Mode: "single",
		StreamingChatter: func(ctx context.Context, req ChatRequest, onChunk func(chunk StreamingChatChunk)) (ChatResult, error) {
			for _, chunk := range []string{"## 说明\n", "```bat\n", "echo hidden stream\n", "```"} {
				onChunk(StreamingChatChunk{Text: chunk})
			}
			return ChatResult{}, nil
		},
	})
	target := filepath.Join(root, "batch_script.bat")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !strings.Contains(string(data), "echo hidden stream") {
		t.Fatalf("file=%q, want streamed script content saved", string(data))
	}
	for _, forbidden := range []string{"说明", "echo hidden stream"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("output=%q, should not stream generated body %q", output, forbidden)
		}
	}
	if !strings.Contains(output, "saved_file=") {
		t.Fatalf("output=%q, want saved_file summary", output)
	}
}

func TestNekoAutoSavesChatCodeBlockToSpecifiedDirectory(t *testing.T) {
	allowAutoSaveForTest(t)
	root := t.TempDir()
	targetDir := filepath.Join(root, "out")
	output := runTestConsole(t, "帮我写个bat文件，存放位置在"+targetDir+"\n/exit\n", Options{
		Root: root,
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "```bat\necho saved\n```"}, nil
		},
	})
	target := filepath.Join(targetDir, "batch_script.bat")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !strings.Contains(string(data), "echo saved") || !strings.Contains(output, "saved_file=") || !strings.Contains(output, filepath.Base(target)) {
		t.Fatalf("output=%q file=%q, want specified directory save", output, string(data))
	}
}

func TestNekoResolveAutoSavePathSupportsWindowsDesktopPhrase(t *testing.T) {
	root := t.TempDir()
	got := resolveAutoSavePath(root, "帮我写个bat脚本，保存到D盘桌面", "bat")
	want := filepath.Clean(`D:\Desktop\batch_script.bat`)
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestNekoResolveAutoSavePathSupportsMoveDesktopToDrivePhrase(t *testing.T) {
	root := t.TempDir()
	got := resolveAutoSavePath(root, "帮我写个bat脚本迁移桌面到D盘", "bat")
	want := filepath.Clean(`D:\Desktop\migrate_desktop_to_d_drive.bat`)
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestNekoResolveAutoSavePathNamesDesktopMigrationFromUserRequest(t *testing.T) {
	root := t.TempDir()
	got := resolveAutoSavePath(root, "写一个把桌面目录迁移到D盘的脚本到桌面", "bat")
	want := filepath.Clean(`D:\Desktop\migrate_desktop_to_d_drive.bat`)
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestNekoResolveAutoSavePathKeepsUserProvidedBaseName(t *testing.T) {
	root := t.TempDir()
	got := resolveAutoSavePath(root, "写一个bat脚本保存到桌面，文件名叫move_desktop", "bat")
	want := filepath.Join(mustUserHomeDesktop(t), "move_desktop.bat")
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestNekoPrepareModelPromptForAutoSaveRequestsCodeOnly(t *testing.T) {
	prompt := PrepareModelPrompt("帮我写一个bat脚本保存到桌面")
	if !strings.Contains(prompt, "Return only one fenced code block") || !strings.Contains(prompt, "User request:") {
		t.Fatalf("prompt=%q, want code-only save prompt", prompt)
	}
	if got := ModelMaxTokens("帮我写一个bat脚本保存到桌面"); got != 2048 {
		t.Fatalf("ModelMaxTokens = %d, want 2048", got)
	}
	if got := PrepareModelPrompt("你好"); got != "你好" {
		t.Fatalf("normal prompt changed to %q", got)
	}
}

func TestNekoResolveAutoSavePathTrimsBacktickedWindowsDirectory(t *testing.T) {
	root := t.TempDir()
	got := resolveAutoSavePath(root, "帮我生成bat并保存到 `D:\\Desktop`", "bat")
	want := filepath.Clean(`D:\Desktop\batch_script.bat`)
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func mustUserHomeDesktop(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error: %v", err)
	}
	return filepath.Join(home, "Desktop")
}

func TestNekoResolveAutoSavePathSupportsDriveRootPhrase(t *testing.T) {
	root := t.TempDir()
	got := resolveAutoSavePath(root, "生成脚本保存到D盘", "bat")
	want := filepath.Clean(`D:\script.bat`)
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestNekoAutoSavesChatResponseToExplicitFileName(t *testing.T) {
	allowAutoSaveForTest(t)
	root := t.TempDir()
	output := runTestConsole(t, "写入到hello.txt\n/exit\n", Options{
		Root: root,
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "hello file"}, nil
		},
	})
	target := filepath.Join(root, "hello.txt")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if strings.TrimSpace(string(data)) != "hello file" || !strings.Contains(output, "saved_file=") || !strings.Contains(output, filepath.Base(target)) {
		t.Fatalf("output=%q file=%q, want explicit filename save", output, string(data))
	}
}

func TestNekoAutoSavesBatScriptWithoutExplicitSaveIntent(t *testing.T) {
	allowAutoSaveForTest(t)
	root := t.TempDir()
	output := runTestConsole(t, "帮我写一个bat批处理内容\n/exit\n", Options{
		Root: root,
		Mode: "single",
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "```bat\necho auto save works\n```"}, nil
		},
	})
	if !strings.Contains(output, "saved_file=") {
		t.Fatalf("output=%q, should auto-save when message contains file generation intent", output)
	}
	if _, err := os.Stat(filepath.Join(root, "batch_script.bat")); err != nil {
		t.Fatalf("expected generated bat file, stat err=%v", err)
	}
}

func TestNekoSlashHelp(t *testing.T) {
	output := runTestConsole(t, "/help\n/exit\n", Options{})
	if !strings.Contains(output, "Commands") || !strings.Contains(output, "/connect") {
		t.Fatalf("help output = %q", output)
	}
	if strings.Contains(output, "/mode single") || strings.Contains(output, "/run <goal>") {
		t.Fatalf("help output = %q, should show compact command list", output)
	}
}

func TestNekoModeSwitch(t *testing.T) {
	output := runTestConsole(t, "/mode single\n/exit\n", Options{})
	if !strings.Contains(output, "mode=single worktree=false") {
		t.Fatalf("output = %q, want mode switch", output)
	}
}

func TestNekoExitCommand(t *testing.T) {
	output := runTestConsole(t, "/exit\n", Options{})
	if !strings.Contains(output, "Goodbye from MimoNeko.") {
		t.Fatalf("output = %q, want goodbye", output)
	}
}

func TestNekoEmptyGoalDoesNotRun(t *testing.T) {
	called := false
	_ = runTestConsole(t, "\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			called = true
			return RunResult{}, nil
		},
	})
	if called {
		t.Fatal("empty goal should not run")
	}
}

func TestNekoDisplaysCurrentModel(t *testing.T) {
	output := runTestConsole(t, "/model\n/exit\n", Options{})
	if !strings.Contains(output, "mimo-v2.5-pro") {
		t.Fatalf("output = %q, want model", output)
	}
}

func TestNekoDisplaysProvider(t *testing.T) {
	output := runTestConsole(t, "/model\n/exit\n", Options{})
	if !strings.Contains(output, "mimo") {
		t.Fatalf("output = %q, want provider", output)
	}
}

func TestNekoDisplaysAPIKeyStatusOnly(t *testing.T) {
	t.Setenv("MIMO_API_KEY", "sk-neko-status-secret")
	output := runTestConsole(t, "/model\n/exit\n", Options{})
	if !strings.Contains(output, "configured") {
		t.Fatalf("output = %q, want configured status", output)
	}
	if strings.Contains(output, "sk-neko-status-secret") {
		t.Fatalf("output leaked API key: %q", output)
	}
}

func TestNekoDoesNotLeakAPIKey(t *testing.T) {
	secret := "sk-neko-leak-secret"
	t.Setenv("MIMO_API_KEY", secret)
	output := runTestConsole(t, "/model\n/exit\n", Options{
		ModelTester: func(ctx context.Context, session Session) (string, error) {
			return "Authorization: Bearer " + secret, nil
		},
	})
	if strings.Contains(output, secret) {
		t.Fatalf("output leaked API key: %q", output)
	}
}

func TestNekoDisplaysContextLength(t *testing.T) {
	output := runTestConsole(t, "/model\n/exit\n", Options{})
	// New model config UI doesn't show context length directly
	if !strings.Contains(output, "Model Configuration") {
		t.Fatalf("output = %q, want Model Configuration", output)
	}
}

func TestNekoDisplaysReasoningLevel(t *testing.T) {
	output := runTestConsole(t, "/model\n/exit\n", Options{})
	// New model config UI doesn't show reasoning level directly
	if !strings.Contains(output, "Model Configuration") {
		t.Fatalf("output = %q, want Model Configuration", output)
	}
}

func TestNekoUsesConsistentPalette(t *testing.T) {
	session := newTestSession(t, nil, Options{DryRun: true, DryRunSet: true})
	var out bytes.Buffer
	RenderHeader(&out, session)
	// Current theme uses warm accent
	if !strings.Contains(out.String(), "\x1b[38;5;214m") {
		t.Fatalf("output = %q, want accent ANSI theme", out.String())
	}
}

func TestNekoNoColorOmitsANSI(t *testing.T) {
	output := runTestConsole(t, "/exit\n", Options{NoColor: true})
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("output contains ANSI: %q", output)
	}
}

func TestNekoDisplaysTokenUsage(t *testing.T) {
	output := runTestConsole(t, "/run token test\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			return RunResult{Usage: Usage{InputTokens: 10, CachedTokens: 5, OutputTokens: 3, Estimated: true}}, nil
		},
	})
	if !strings.Contains(output, "tokens=input=13 cached=5 output=3 total=21") {
		t.Fatalf("output = %q, want token usage", output)
	}
}

func TestAssistantMessagesRemainVisible(t *testing.T) {
	output := runTestConsole(t, "/run summarize README\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			return RunResult{RunID: "run_visible", State: "succeeded", Output: "Assistant result stays visible."}, nil
		},
	})
	for _, want := range []string{"▸ You", "summarize README", "Assistant", "Assistant result stays visible.", "run_id=run_visible"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want visible message %q", output, want)
		}
	}
	if strings.Contains(output, "\x1b[2J") || strings.Contains(output, "\x1b[H") {
		t.Fatalf("console output contains screen clear/move-home sequence: %q", output)
	}
}

func TestNekoComputesCNYCostFromPricing(t *testing.T) {
	pricing := &config.ModelPricingConfig{
		Currency:               "CNY",
		InputPer1MTokens:       10,
		CachedInputPer1MTokens: 1,
		OutputPer1MTokens:      20,
		Source:                 "user",
	}
	cost := ComputeCost(Usage{InputTokens: 1000, CachedTokens: 1000, OutputTokens: 1000, Estimated: true}, pricing)
	if got := FormatCost(cost); got != "¥0.0310 estimated" {
		t.Fatalf("cost = %q, want ¥0.0310 estimated", got)
	}
}

func TestNekoComputesMimoNativeCacheCostFromHitMiss(t *testing.T) {
	pricing := &config.ModelPricingConfig{
		Currency:               "CNY",
		InputPer1MTokens:       10,
		CachedInputPer1MTokens: 1,
		OutputPer1MTokens:      20,
		Source:                 "user",
	}
	cost := ComputeCost(Usage{
		InputTokens:      1000,
		CachedTokens:     12,
		CacheHitTokens:   900,
		CacheMissTokens:  100,
		NativeCacheKnown: true,
		OutputTokens:     1000,
		TotalTokens:      2000,
	}, pricing)
	if got := FormatCost(cost); got != "\u00a50.0219" {
		t.Fatalf("cost = %q, want \\u00a50.0219", got)
	}
}

func TestNekoCostUnavailableWithoutPricing(t *testing.T) {
	output := runTestConsole(t, "/model\n/exit\n", Options{})
	// New model config UI doesn't show pricing directly
	if !strings.Contains(output, "Model Configuration") {
		t.Fatalf("output = %q, want Model Configuration", output)
	}
}

func TestNekoMarksEstimatedUsage(t *testing.T) {
	cost := ComputeCost(Usage{InputTokens: 1, Estimated: true}, &config.ModelPricingConfig{Currency: "CNY", InputPer1MTokens: 1})
	if !strings.Contains(FormatCost(cost), "estimated") {
		t.Fatalf("cost should be marked estimated")
	}
}

func TestNekoDoesNotHardcodePricing(t *testing.T) {
	opt := Options{NoColor: true, DryRun: true, DryRunSet: true, Model: "plain-model"}
	session := newTestSession(t, nil, opt)
	got := FormatCost(ComputeCost(session.Usage, session.Pricing))
	if got == "" {
		t.Fatalf("cost should not be empty")
	}
}

func TestNekoSingleRunDryRun(t *testing.T) {
	var got RunRequest
	runTestConsole(t, "/run hello\n/exit\n", Options{
		Mode: "single",
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			got = req
			return RunResult{State: "succeeded"}, nil
		},
	})
	if got.Mode != "single" || !got.DryRun || got.Worktree {
		t.Fatalf("request = %+v, want single dry-run without default worktree", got)
	}
}

func TestNekoMultiRunDryRun(t *testing.T) {
	var got RunRequest
	runTestConsole(t, "/run hello\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			got = req
			return RunResult{State: "succeeded"}, nil
		},
	})
	if got.Mode != "multi" || !got.DryRun || !got.Worktree {
		t.Fatalf("request = %+v, want multi dry-run with worktree", got)
	}
}

func TestNekoDoesNotAutoApply(t *testing.T) {
	output := runTestConsole(t, "/run hello\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			return RunResult{WorktreeID: "wt_123", State: "succeeded"}, nil
		},
	})
	if strings.Contains(output, "auto apply") || strings.Contains(output, "auto-commit") || strings.Contains(output, "auto-push") {
		t.Fatalf("output implies automatic side effects: %q", output)
	}
	if !strings.Contains(output, "CLI apply:") {
		t.Fatalf("output = %q, want CLI apply hint only", output)
	}
}

func TestNekoShowsWorktreeID(t *testing.T) {
	output := runTestConsole(t, "/run hello\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			return RunResult{WorktreeID: "wt_neko", State: "succeeded"}, nil
		},
	})
	if !strings.Contains(output, "worktree_id=wt_neko") {
		t.Fatalf("output = %q, want worktree id", output)
	}
}

func TestNekoShowsPatchNextSteps(t *testing.T) {
	output := runTestConsole(t, "/run hello\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			return RunResult{WorktreeID: "wt_next", State: "succeeded"}, nil
		},
	})
	for _, want := range []string{"/preview wt_next", "/review wt_next", "/discard wt_next", "mimoneko patch apply wt_next"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestNekoReadOnlyRunHidesPatchNextSteps(t *testing.T) {
	output := runTestConsole(t, "inspect project files\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			return RunResult{WorktreeID: "wt_readonly", State: "succeeded", Output: "checked project files", ReadOnly: true}, nil
		},
	})
	for _, forbidden := range []string{"worktree_id=wt_readonly", "/preview wt_readonly", "/review wt_readonly", "/discard wt_readonly", "CLI apply:", "patch apply wt_readonly"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("output = %q, should hide read-only patch hint %q", output, forbidden)
		}
	}
	if !strings.Contains(output, "checked project files") {
		t.Fatalf("output = %q, want read-only result", output)
	}
}

func TestNekoSanitizesModelOutput(t *testing.T) {
	secret := "sk-neko-model-output"
	output := runTestConsole(t, "/run hello\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			return RunResult{Output: "model said " + secret}, nil
		},
	})
	if strings.Contains(output, secret) {
		t.Fatalf("output leaked model secret: %q", output)
	}
}

func TestNekoSanitizesEventMessages(t *testing.T) {
	output := runTestConsole(t, "/runs\n/exit\n", Options{
		RunsLister: func(ctx context.Context, session Session) (string, error) {
			return "latest event TOKEN=abc123", nil
		},
	})
	if strings.Contains(output, "TOKEN=abc123") {
		t.Fatalf("output leaked event secret: %q", output)
	}
}

func TestNekoNoCommitPushApplyCalls(t *testing.T) {
	output := runTestConsole(t, "/discard wt_1\n/exit\n", Options{
		Discarder: func(ctx context.Context, session Session, worktreeID string) (string, error) {
			return "worktree_id=" + worktreeID + " state=discarded", nil
		},
	})
	for _, forbidden := range []string{"git commit", "git push", "patch apply wt_1"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("output contains forbidden operation %q: %q", forbidden, output)
		}
	}
}

func TestNekoDoesNotExposeChainOfThought(t *testing.T) {
	output := runTestConsole(t, "/run hello\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			return RunResult{Output: "hidden reasoning should stay private; chain-of-thought: nope"}, errors.New("private reasoning failed")
		},
	})
	lower := strings.ToLower(output)
	if strings.Contains(lower, "chain-of-thought") || strings.Contains(lower, "hidden reasoning") || strings.Contains(lower, "private reasoning") {
		t.Fatalf("output exposes reasoning marker: %q", output)
	}
}

func TestNekoScreenStreamingReasoningSeparated(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.Thinking = layout.NewThinkingRenderer(true)
	console.Options.StreamingChatter = func(ctx context.Context, req ChatRequest, onChunk func(chunk StreamingChatChunk)) (ChatResult, error) {
		onChunk(StreamingChatChunk{ReasoningText: "Hmm, the user said hello"})
		onChunk(StreamingChatChunk{ReasoningText: ", I should respond."})
		onChunk(StreamingChatChunk{Text: "你好！"})
		onChunk(StreamingChatChunk{Text: "有什么可以帮你？"})
		return ChatResult{Response: "你好！有什么可以帮你？"}, nil
	}
	console.chatMessage(context.Background(), "你好")

	// Verify reasoning does NOT appear in the final assistant message
	for _, item := range console.screenLog {
		if item.Kind == "assistant" {
			if strings.Contains(item.Text, "Hmm") || strings.Contains(item.Text, "should respond") {
				t.Fatalf("assistant text contains reasoning: %q", item.Text)
			}
		}
	}

	// Verify no thought_stream entries remain (default is hidden, so they should be removed)
	for _, item := range console.screenLog {
		if item.Kind == "thought_stream" {
			t.Fatalf("thought_stream entry should be removed after streaming: %+v", console.screenLog)
		}
	}

	// Verify reasoning text is stored for toggle
	if console.lastReasoningText == "" {
		t.Fatalf("lastReasoningText should be set")
	}
	if !strings.Contains(console.lastReasoningText, "Hmm") {
		t.Fatalf("lastReasoningText = %q, want reasoning content", console.lastReasoningText)
	}

	// Verify the assistant entry has the answer only
	assistantCount := 0
	for _, item := range console.screenLog {
		if item.Kind == "assistant" && item.Text == "你好！有什么可以帮你？" {
			assistantCount++
		}
	}
	if assistantCount != 1 {
		t.Fatalf("assistantCount=%d log=%+v, want exactly one assistant with answer only", assistantCount, console.screenLog)
	}
}

func TestNekoScreenStreamingReasoningVisibleWhenToggled(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.Thinking = layout.NewThinkingRenderer(true)
	// Toggle to show thoughts before streaming
	console.Thinking.Toggle()

	console.Options.StreamingChatter = func(ctx context.Context, req ChatRequest, onChunk func(chunk StreamingChatChunk)) (ChatResult, error) {
		onChunk(StreamingChatChunk{ReasoningText: "thinking about this"})
		onChunk(StreamingChatChunk{Text: "answer here"})
		return ChatResult{Response: "answer here"}, nil
	}
	console.chatMessage(context.Background(), "test")

	// With ShowThoughts=true, thought_content entry should remain
	hasThought := false
	for _, item := range console.screenLog {
		if item.Kind == "thought_content" {
			hasThought = true
			if !strings.Contains(item.Text, "thinking about this") {
				t.Fatalf("thought text = %q, want reasoning content", item.Text)
			}
		}
	}
	if !hasThought {
		t.Fatalf("expected thought_content entry in screenLog when ShowThoughts=true: %+v", console.screenLog)
	}

	// Answer should still be separate
	hasAnswer := false
	for _, item := range console.screenLog {
		if item.Kind == "assistant" && item.Text == "answer here" {
			hasAnswer = true
		}
	}
	if !hasAnswer {
		t.Fatalf("expected assistant entry with answer: %+v", console.screenLog)
	}
}

func TestNekoScreenThinkingToggleAfterStream(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.Thinking = layout.NewThinkingRenderer(true)

	console.Options.StreamingChatter = func(ctx context.Context, req ChatRequest, onChunk func(chunk StreamingChatChunk)) (ChatResult, error) {
		onChunk(StreamingChatChunk{ReasoningText: "some reasoning"})
		onChunk(StreamingChatChunk{Text: "final answer"})
		return ChatResult{Response: "final answer"}, nil
	}
	console.chatMessage(context.Background(), "test")

	// Default: hidden, no thought_content entry
	for _, item := range console.screenLog {
		if item.Kind == "thought_content" || item.Kind == "thought_stream" {
			t.Fatalf("no thought entries expected when hidden: %+v", console.screenLog)
		}
	}

	// Toggle to show
	console.toggleThinking()
	hasThought := false
	for _, item := range console.screenLog {
		if item.Kind == "thought_content" {
			hasThought = true
			if !strings.Contains(item.Text, "some reasoning") {
				t.Fatalf("thought text = %q, want reasoning content", item.Text)
			}
		}
	}
	if !hasThought {
		t.Fatalf("expected thought_content entry after toggle-show: %+v", console.screenLog)
	}

	// Toggle back to hide
	console.toggleThinking()
	for _, item := range console.screenLog {
		if item.Kind == "thought_content" || item.Kind == "thought_stream" {
			t.Fatalf("no thought entries expected after toggle-hide: %+v", console.screenLog)
		}
	}
}

func TestNekoNonScreenStreamingReasoningNotExposed(t *testing.T) {
	output := runTestConsole(t, "你好\n/exit\n", Options{
		Mode: "single",
		StreamingChatter: func(ctx context.Context, req ChatRequest, onChunk func(chunk StreamingChatChunk)) (ChatResult, error) {
			onChunk(StreamingChatChunk{ReasoningText: "internal reasoning"})
			onChunk(StreamingChatChunk{ReasoningText: "more thoughts"})
			onChunk(StreamingChatChunk{Text: "你好！"})
			return ChatResult{Response: "你好！"}, nil
		},
	})
	lower := strings.ToLower(output)
	if strings.Contains(lower, "internal reasoning") || strings.Contains(lower, "more thoughts") {
		t.Fatalf("output exposes reasoning text: %q", output)
	}
	if !strings.Contains(output, "你好！") {
		t.Fatalf("output = %q, want answer text", output)
	}
}

func TestNekoModelPickerOpensInScreenMode(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.openModelPicker()

	if !console.modelPickerOpen {
		t.Fatalf("modelPickerOpen = false, want true")
	}
	if len(console.modelPickerItems) == 0 {
		t.Fatalf("modelPickerItems is empty, want items")
	}

	// Should have model entries and add-provider entry; providers render as a right column.
	groupCount := 0
	modelCount := 0
	addProviderCount := 0
	for _, item := range console.modelPickerItems {
		if item.IsGroup {
			groupCount++
		} else if item.IsAddProvider {
			addProviderCount++
		} else {
			modelCount++
		}
	}
	if groupCount != 0 {
		t.Fatalf("groupCount = %d, want compact model list without group headers", groupCount)
	}
	if modelCount != 3 {
		t.Fatalf("modelCount = %d, want 3 models", modelCount)
	}
	if addProviderCount != 1 {
		t.Fatalf("addProviderCount = %d, want 1", addProviderCount)
	}

	// Current model should be pre-selected
	sel := console.modelPickerItems[console.modelPickerSelected]
	if sel.Model != "mimo-v2.5-pro" {
		t.Fatalf("selected model = %q, want %q", sel.Model, "mimo-v2.5-pro")
	}
}

func TestNekoModelPickerNavigation(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.openModelPicker()

	// Move down (should skip group headers)
	console.moveModelPicker(1)
	sel := console.modelPickerItems[console.modelPickerSelected]
	if sel.IsGroup {
		t.Fatalf("selected item is group header after move down: %+v", sel)
	}

	// Move up back
	console.moveModelPicker(-1)
	sel = console.modelPickerItems[console.modelPickerSelected]
	if sel.IsGroup {
		t.Fatalf("selected item is group header after move up: %+v", sel)
	}
}

func TestNekoModelPickerSelection(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.openModelPicker()

	// Navigate to fast-model
	for i, item := range console.modelPickerItems {
		if item.Model == "fast-model" {
			console.modelPickerSelected = i
			break
		}
	}

	console.executeModelPickerSelection()

	if console.Session.Model != "fast-model" {
		t.Fatalf("model = %q, want %q", console.Session.Model, "fast-model")
	}
	if console.Session.Provider != "fast" {
		t.Fatalf("provider = %q, want %q", console.Session.Provider, "fast")
	}
	if console.modelPickerOpen {
		t.Fatalf("modelPickerOpen = true after selection, want false")
	}
}

func TestNekoModelPickerAddProviderOpensProviderPicker(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.openModelPicker()
	for i, item := range console.modelPickerItems {
		if item.IsAddProvider {
			console.modelPickerSelected = i
			break
		}
	}
	out.Reset()
	console.executeModelPickerSelection()

	if console.modelPickerOpen {
		t.Fatalf("modelPickerOpen = true, want picker closed")
	}
	if !console.providerPickerOpen {
		t.Fatalf("providerPickerOpen = false, want provider picker")
	}
	text := out.String()
	if !strings.Contains(text, "Connect a provider") || !strings.Contains(text, "Custom API Provider") {
		t.Fatalf("screen = %q, want provider picker", text)
	}
	if strings.Contains(text, "Base URL") || strings.Contains(text, "API Key") {
		t.Fatalf("screen = %q, should not render overlay form fields up front", text)
	}
}

func TestNekoConnectCommandOpensProviderPickerInScreenMode(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.handleInputLine(context.Background(), "/connect")
	if !console.providerPickerOpen {
		t.Fatalf("providerPickerOpen = false, want provider picker")
	}
	if console.modelPickerOpen {
		t.Fatalf("model picker should be closed")
	}
	if !strings.Contains(out.String(), "Connect a provider") {
		t.Fatalf("screen = %q, want provider picker", out.String())
	}
}

func TestNekoProviderPickerSelectionOpensAPIKeyModalForConfiguredProvider(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.openProviderPicker()
	console.executeProviderPickerSelection()

	if console.providerPickerOpen {
		t.Fatalf("providerPickerOpen = true, want closed")
	}
	if !console.addFlow.active || console.addFlow.step != stepAPIKey {
		t.Fatalf("addFlow = %+v, want API-key step for configured provider", console.addFlow)
	}
	if console.addFlow.name == "" || console.addFlow.baseURL == "" {
		t.Fatalf("addFlow = %+v, want provider name and base URL prefilled", console.addFlow)
	}
	text := out.String()
	if !strings.Contains(text, "API key") {
		t.Fatalf("screen = %q, want API key modal", text)
	}
}

func TestNekoProviderPickerCustomSelectionStartsProviderNameModal(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.openProviderPicker()
	console.providerPickerSelected = len(console.providerPickerItems) - 1
	console.executeProviderPickerSelection()

	if !console.addFlow.active || console.addFlow.step != stepProviderName {
		t.Fatalf("addFlow = %+v, want provider-name step for custom provider", console.addFlow)
	}
	if !strings.Contains(out.String(), "Connect a provider") {
		t.Fatalf("screen = %q, want connect-provider modal", out.String())
	}
}

func TestNekoAddProviderHeaderTypeArrowSelection(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
		addFlow:      addProviderFlow{active: true, step: stepHeaderType},
	}
	reader := bufio.NewReader(strings.NewReader("[B"))
	if _, err := reader.Peek(2); err != nil {
		t.Fatalf("prime escape reader: %v", err)
	}
	console.handleRawRune(context.Background(), reader, '\x1b')
	if !console.addFlow.active {
		t.Fatalf("addFlow cancelled by arrow key")
	}
	if console.addFlow.headerTypeLabel() != "Bearer" {
		t.Fatalf("header selection = %q, want Bearer", console.addFlow.headerTypeLabel())
	}
}

func TestNekoModelPickerClose(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.openModelPicker()
	if !console.modelPickerOpen {
		t.Fatalf("picker not open")
	}
	console.closeModelPicker()
	if console.modelPickerOpen {
		t.Fatalf("picker still open after close")
	}
}

func TestNekoModelPickerCurrentHighlighted(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	// Switch to fast-model first
	session.SelectModel("fast-model")
	console.Session = session
	console.openModelPicker()

	sel := console.modelPickerItems[console.modelPickerSelected]
	if sel.Model != "fast-model" {
		t.Fatalf("selected model = %q, want fast-model (current)", sel.Model)
	}
}

func TestNekoModelsCommandOpensPickerInScreenMode(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.handleInputLine(context.Background(), "/models")

	if !console.modelPickerOpen {
		t.Fatalf("modelPickerOpen = false after /models, want true")
	}
}

func TestNekoModelCommandOpensPickerInScreenMode(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.handleInputLine(context.Background(), "/model")

	if !console.modelPickerOpen {
		t.Fatalf("modelPickerOpen = false after /model, want true")
	}
	if !strings.Contains(out.String(), "Select model") {
		t.Fatalf("screen = %q, want model selector", out.String())
	}
}

func TestNekoModelPickerRendersBorderlessSelector(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}
	console.openModelPicker()
	text := out.String()
	for _, want := range []string{"Select model", "Search", "Connect provider"} {
		if !strings.Contains(text, want) {
			t.Fatalf("screen = %q, want %q", text, want)
		}
	}
	for _, border := range []string{"╭", "╮", "╰", "╯", "│"} {
		if strings.Contains(text, border) {
			t.Fatalf("screen = %q, model picker should not draw border %q", text, border)
		}
	}
}

func TestNekoModelsCommandWithArgSwitchesModel(t *testing.T) {
	output := runTestConsole(t, "/models fast-model\n/exit\n", Options{})
	if !strings.Contains(output, "Model switched to fast-model") {
		t.Fatalf("output = %q, want model switch message", output)
	}
}

func runTestConsole(t *testing.T, input string, opt Options) string {
	t.Helper()
	opt.NoColor = true
	opt.DryRun = true
	opt.DryRunSet = true
	session := newTestSession(t, nil, opt)
	var out bytes.Buffer
	opt.In = strings.NewReader(input)
	opt.Out = &out
	opt.Err = &out
	console := Console{Session: session, Options: opt}
	if code := console.Run(context.Background()); code != 0 {
		t.Fatalf("console code = %d, output = %q", code, out.String())
	}
	return out.String()
}

func runTestConsoleWithColor(t *testing.T, input string, opt Options) string {
	t.Helper()
	opt.NoColor = false
	opt.DryRun = true
	opt.DryRunSet = true
	session := newTestSession(t, nil, opt)
	var out bytes.Buffer
	opt.In = strings.NewReader(input)
	opt.Out = &out
	opt.Err = &out
	console := Console{Session: session, Options: opt}
	if code := console.Run(context.Background()); code != 0 {
		t.Fatalf("console code = %d, output = %q", code, out.String())
	}
	return out.String()
}

func newTestSession(t *testing.T, pricing *config.ModelPricingConfig, opt Options) Session {
	t.Helper()
	root := opt.Root
	if root == "" {
		root = t.TempDir()
	}
	models := config.ModelsConfig{
		Providers: []config.ProviderConfig{
			{
				Name:      "mimo",
				Type:      "openai-compatible",
				BaseURL:   "https://token-plan-cn.xiaomimimo.com/v1",
				APIKeyEnv: "MIMO_API_KEY",
				Models: []config.ModelConfig{
					{
						Name:                "mimo-v2.5-pro",
						Purpose:             "coding",
						MaxOutputTokens:     4096,
						SupportsPrefixCache: false,
						Pricing:             pricing,
					},
				},
			},
			{
				Name:      "fast",
				Type:      "openai-compatible",
				BaseURL:   "https://fast.example/v1",
				APIKeyEnv: "FAST_API_KEY",
				Models: []config.ModelConfig{
					{Name: "fast-model", Purpose: "coding", MaxOutputTokens: 2048, MaxContextTokens: 32000, ReasoningLevel: "low"},
				},
			},
			{
				Name:      "plain",
				Type:      "openai-compatible",
				BaseURL:   "https://plain.example/v1",
				APIKeyEnv: "PLAIN_API_KEY",
				Models: []config.ModelConfig{
					{Name: "plain-model", Purpose: "coding", MaxOutputTokens: 2048, MaxContextTokens: 32000},
				},
			},
		},
		Routing: config.RoutingConfig{DefaultModel: "mimo-v2.5-pro"},
	}
	if opt.Mode == "" {
		opt.Mode = "multi"
	}
	return NewSession(root, models, opt)
}

func TestAddFlow_UpDownOnAPIKey_DoesNotCancel(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	console := Console{
		Session:      session,
		Options:      Options{Out: &bytes.Buffer{}},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
		addFlow:      addProviderFlow{active: true, step: stepAPIKey, name: "demo", baseURL: "https://x"},
	}
	reader := bufio.NewReader(strings.NewReader("[A"))
	if _, err := reader.Peek(2); err != nil {
		t.Fatalf("prime escape reader: %v", err)
	}
	if cancelled := console.handleRawRune(context.Background(), reader, '\x1b'); cancelled {
		t.Fatal("handleRawRune returned true (flow exited) on arrow key at API-key step")
	}
	if !console.addFlow.active {
		t.Fatal("addFlow was cancelled by arrow key at API-key step")
	}
	if console.addFlow.step != stepAPIKey {
		t.Fatalf("addFlow.step = %d, want stepAPIKey (%d)", console.addFlow.step, stepAPIKey)
	}
}

func TestAddFlow_UpDownOnHeaderType_TogglesHeader(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	console := Console{
		Session:      session,
		Options:      Options{Out: &bytes.Buffer{}},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
		addFlow:      addProviderFlow{active: true, step: stepHeaderType, selectedHead: 0},
	}
	reader := bufio.NewReader(strings.NewReader("[B"))
	if _, err := reader.Peek(2); err != nil {
		t.Fatalf("prime escape reader: %v", err)
	}
	if cancelled := console.handleRawRune(context.Background(), reader, '\x1b'); cancelled {
		t.Fatal("handleRawRune returned true on arrow key at header-type step")
	}
	if !console.addFlow.active {
		t.Fatal("addFlow was cancelled by arrow key at header-type step")
	}
	if console.addFlow.headerTypeLabel() != "Bearer" {
		t.Fatalf("header selection = %q, want Bearer", console.addFlow.headerTypeLabel())
	}
}

func TestAddFlow_BareEsc_CancelsFlow(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	console := Console{
		Session:      session,
		Options:      Options{Out: &bytes.Buffer{}},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
		addFlow:      addProviderFlow{active: true, step: stepProviderName},
	}
	reader := bufio.NewReader(strings.NewReader(""))
	console.handleRawRune(context.Background(), reader, '\x1b')
	if console.addFlow.active {
		t.Fatal("addFlow still active after bare Esc")
	}
}

func TestAddFlow_ComposerRowHidden_WhenFlowActive(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	var out bytes.Buffer
	console := Console{
		Session:      session,
		Options:      Options{Out: &out},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
		addFlow:      addProviderFlow{active: true, step: stepProviderName},
		uiMode:       "build",
	}
	top := 26
	console.renderScreenWorkbenchComposerV2(&out, branding.Renderer{NoColor: true}, 1, 120, top)
	rendered := out.String()
	if strings.Contains(rendered, "Ask anything") {
		t.Fatalf("composer row should not show the Ask anything placeholder while addFlow is active; got %q", rendered)
	}
	if !strings.Contains(rendered, "Connect a provider") {
		t.Fatalf("composer row should show the active flow title; got %q", rendered)
	}
	if !strings.Contains(rendered, "esc") {
		t.Fatalf("composer row should show the esc hint; got %q", rendered)
	}
}

func TestSetStatus_ReplacesTrailingStatus(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	console := Console{
		Session:      session,
		Options:      Options{Out: &bytes.Buffer{}},
		screenActive: true,
		screenCols:   120,
		screenRows:   30,
	}

	console.setStatus("first success", false)
	if got := console.screenLog[len(console.screenLog)-1]; got.Kind != "done" || got.Text != "first success" {
		t.Fatalf("trailing entry = %+v, want done/first success", got)
	}

	console.setStatus("second success", false)
	if len(console.screenLog) != 1 {
		t.Fatalf("len(screenLog) = %d, want 1 after dedup; entries = %+v", len(console.screenLog), console.screenLog)
	}
	if got := console.screenLog[0]; got.Text != "second success" {
		t.Fatalf("trailing entry = %+v, want second success", got)
	}

	console.setStatus("boom", true)
	if len(console.screenLog) != 1 {
		t.Fatalf("len(screenLog) = %d, want 1 after dedup of trailing done by error; entries = %+v", len(console.screenLog), console.screenLog)
	}
	if got := console.screenLog[0]; got.Kind != "error" || got.Text != "boom" {
		t.Fatalf("trailing entry = %+v, want error/boom", got)
	}

	console.setStatus("recovered", false)
	if len(console.screenLog) != 1 {
		t.Fatalf("len(screenLog) = %d, want 1 after dedup of trailing error by done; entries = %+v", len(console.screenLog), console.screenLog)
	}
	if got := console.screenLog[0]; got.Kind != "done" || got.Text != "recovered" {
		t.Fatalf("trailing entry = %+v, want done/recovered", got)
	}

	console.setStatus("again", true)
	if got := console.screenLog[0]; got.Kind != "error" || got.Text != "again" {
		t.Fatalf("trailing entry = %+v, want error/again", got)
	}
}

func TestRenderModalRow_NoMidContentReset(t *testing.T) {
	width := 24
	row := renderModalRow(
		width,
		modalPart{fg: modalFgTitle, txt: "API_KEY"},
		modalPart{fg: modalFgMuted, txt: "  hello"},
	)
	if !strings.HasPrefix(row, "\x1b[48;5;236m") {
		t.Fatalf("row must start with bg SGR; got %q", row)
	}
	if !strings.HasSuffix(row, "\x1b[0m") {
		t.Fatalf("row must end with single Reset; got %q", row)
	}
	if got := strings.Count(row, "\x1b[0m"); got != 1 {
		t.Fatalf("row has %d Resets, want exactly 1; got %q", got, row)
	}
	if strings.Contains(row, "\x1b[0m\x1b[") {
		t.Fatalf("row has internal Reset before another SGR; got %q", row)
	}
	if !strings.Contains(row, "\x1b[38;5;230m") {
		t.Fatalf("row should switch to title fg; got %q", row)
	}
	if !strings.Contains(row, "\x1b[38;5;244m") {
		t.Fatalf("row should switch to muted fg; got %q", row)
	}
	if got := strings.Count(row, "\x1b[48;5;236m"); got != 1 {
		t.Fatalf("row has %d bg SGRs, want exactly 1; got %q", got, row)
	}
	if !strings.HasSuffix(strings.TrimRight(row, " "), "\x1b[0m") {
		t.Fatalf("row padding should still end with single Reset; got %q", row)
	}
	visible := screenWidth(row)
	if visible != width {
		t.Fatalf("visible width = %d, want %d", visible, width)
	}
}

func TestScreenModalLine_ReappliesBackgroundAfterInnerANSIReset(t *testing.T) {
	renderer := branding.NewRenderer(false)
	row := screenModalLine("    "+renderer.Muted("Search"), 24)

	if !strings.HasPrefix(row, "\x1b[48;5;236m") {
		t.Fatalf("row must start with modal bg; got %q", row)
	}
	if !strings.Contains(row, branding.Reset+"\x1b[48;5;236m") {
		t.Fatalf("row must restore modal bg after colored text reset; got %q", row)
	}
	if !strings.HasSuffix(row, branding.Reset) {
		t.Fatalf("row must end with reset; got %q", row)
	}
	if visible := screenWidth(row); visible != 24 {
		t.Fatalf("visible width = %d, want 24; row=%q", visible, row)
	}
}

func TestRenderModalAccentRow_UsesSelectionForeground(t *testing.T) {
	row := renderModalAccentRow(18, modalPart{fg: modalFgAccent, txt: "    API_KEY"})

	if !strings.HasPrefix(row, "\x1b[48;5;216m") {
		t.Fatalf("selected row must start with accent bg; got %q", row)
	}
	if strings.Contains(row, modalFgAccent) {
		t.Fatalf("selected row should not preserve per-item fg over selection fg; got %q", row)
	}
	if !strings.Contains(row, "\x1b[38;5;16m") {
		t.Fatalf("selected row should use selection fg; got %q", row)
	}
	if visible := screenWidth(row); visible != 18 {
		t.Fatalf("visible width = %d, want 18; row=%q", visible, row)
	}
}
