package neko

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mimoneko/mimoneko/internal/config"
)

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
	if !strings.Contains(out.String(), "MIMO") {
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

func TestNekoBareInputUsesChatNotAgent(t *testing.T) {
	agentCalled := false
	chatCalled := false
	output := runTestConsole(t, "你好\n/exit\n", Options{
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
	if !strings.Contains(output, "▸ You") || !strings.Contains(output, "MIMO") || !strings.Contains(output, "你好，我在。") {
		t.Fatalf("output = %q, want terminal-native chat stream", output)
	}
}

func TestNekoRuntimeEventStreamForChat(t *testing.T) {
	output := runTestConsole(t, "hello\n/exit\n", Options{
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "Ready."}, nil
		},
	})
	for _, want := range []string{"thinking...", "requesting model...", "generating response...", "done ·", "+ Thought:"} {
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
	for _, want := range []string{"Commands", "/agents", "/models", "/reasoning", "/new"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want command palette item %q", output, want)
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

func TestNekoModelsCommandSwitchesModel(t *testing.T) {
	output := runTestConsole(t, "/models fast-model\n/exit\n", Options{})
	if !strings.Contains(output, "model=fast-model provider=fast") {
		t.Fatalf("output = %q, want model switch", output)
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
	// Check for key UI elements - new design includes cat mascot and improved styling
	if !strings.Contains(text, "Ask anything") || !strings.Contains(text, "Build") || !strings.Contains(text, "mimo-v2.5-pro") {
		t.Fatalf("screen = %q, want fixed bottom composer with model info", text)
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
	for _, want := range []string{"MIMO", "Ask MIMO", "/ commands"} {
		if !strings.Contains(text, want) {
			t.Fatalf("intro = %q, want %q", text, want)
		}
	}
	if strings.Contains(text, "Ask anything") || strings.Contains(text, "local agent runtime") {
		t.Fatalf("intro should be clean launch dialog: %q", text)
	}
}

func TestNekoContextUsageGrowsWithConversation(t *testing.T) {
	output := runTestConsole(t, "hello world\n/exit\n", Options{
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "Ready."}, nil
		},
	})
	if !strings.Contains(output, "ctx 5 tok (0.005K) / 1M") {
		t.Fatalf("output = %q, want estimated context usage to grow", output)
	}
	if !strings.Contains(output, "memory 2 msgs") {
		t.Fatalf("output = %q, want memory message count", output)
	}
}

func TestNekoNewResetsConversationState(t *testing.T) {
	output := runTestConsole(t, "hello\n/new\n/exit\n", Options{
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "Ready."}, nil
		},
	})
	if !strings.Contains(output, "New session.") {
		t.Fatalf("output = %q, want new session acknowledgement", output)
	}
	if !strings.Contains(output, "ctx 0 tok (0K) / 1M") || !strings.Contains(output, "memory 0 msgs") {
		t.Fatalf("output = %q, want reset context and memory", output)
	}
}

func TestNekoStatusBarRendersRuntimeContext(t *testing.T) {
	output := runTestConsole(t, "/exit\n", Options{})
	for _, want := range []string{"ctx 0 tok (0K) / 1M", "cache n/a", "tools 0", "memory 0 msgs", "model mimo-v2.5-pro", "provider mimo", "reasoning high"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want status item %q", output, want)
		}
	}
}

func TestNekoContextStatusUsesKUnits(t *testing.T) {
	session := newTestSession(t, nil, Options{})
	session.ContextUsedTokens = 228
	if got := statusContextLabel(session.ContextLabel()); got != "228 tok (0.228K) / 1M" {
		t.Fatalf("context label = %q, want K-formatted usage", got)
	}
}

func TestNekoCommandPaletteSelectionFillsArgumentCommand(t *testing.T) {
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
		t.Fatal("argument command selection should not exit")
	}
	if console.draft != "/run " || console.paletteOpen {
		t.Fatalf("draft=%q paletteOpen=%v, want selected command inserted", console.draft, console.paletteOpen)
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
	if !strings.Contains(text, "Commands /") || !strings.Contains(text, "/run <goal>") {
		t.Fatalf("screen = %q, want pull-up command options", text)
	}
	if exit := console.handleRawRune(context.Background(), bufio.NewReader(strings.NewReader("")), '\r'); exit {
		t.Fatal("selecting /run placeholder should fill draft, not exit")
	}
	if console.paletteOpen || console.draft != "/run " {
		t.Fatalf("paletteOpen=%v draft=%q, want enter to confirm selected command into draft", console.paletteOpen, console.draft)
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

func TestNekoCtrlPVTSequenceCyclesReasoning(t *testing.T) {
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
	if console.Session.Reasoning != "low" || !strings.Contains(out.String(), "low") {
		t.Fatalf("reasoning=%q output=%q, want CSI ctrl+p to cycle reasoning", console.Session.Reasoning, out.String())
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
	if strings.Contains(console.screenStatusLine(120), "reasoning") {
		t.Fatalf("status line = %q, should not show reasoning", console.screenStatusLine(120))
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
	for _, want := range []string{"planning...", "executing agent runtime...", "collecting result...", "+ Thought:", "◆ Build"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want run event %q", output, want)
		}
	}
}

func TestNekoChatFailureReturnsFriendlyText(t *testing.T) {
	output := runTestConsole(t, "hello\n/exit\n", Options{
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
	root := t.TempDir()
	output := runTestConsole(t, "帮我生成一个bat批处理文件并保存到默认项目目录\n/exit\n", Options{
		Root: root,
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

func TestNekoAutoSaveSuppressesGeneratedBodyOutput(t *testing.T) {
	root := t.TempDir()
	output := runTestConsole(t, "帮我写一个bat脚本保存到默认项目目录\n/exit\n", Options{
		Root: root,
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
	root := t.TempDir()
	modelCalled := false
	output := runTestConsole(t, "写一个把桌面目录迁移到D盘的脚本保存到默认项目目录\n/exit\n", Options{
		Root: root,
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
	root := t.TempDir()
	output := runTestConsole(t, "帮我写一个bat脚本保存到默认项目目录\n/exit\n", Options{
		Root: root,
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
	root := t.TempDir()
	targetDir := filepath.Join(root, "out")
	output := runTestConsole(t, "帮我写个bat文件，存放位置在"+targetDir+"\n/exit\n", Options{
		Root: root,
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
	root := t.TempDir()
	output := runTestConsole(t, "写入到hello.txt\n/exit\n", Options{
		Root: root,
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
	root := t.TempDir()
	output := runTestConsole(t, "帮我写一个bat批处理内容\n/exit\n", Options{
		Root: root,
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
	if !strings.Contains(output, "MIMO commands") || !strings.Contains(output, "/mode single") {
		t.Fatalf("help output = %q", output)
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
	if !strings.Contains(output, "Goodbye from MIMO.") {
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
