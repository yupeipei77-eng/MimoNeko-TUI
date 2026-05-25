package neko

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reasonforge/reasonforge/internal/config"
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
	if !strings.Contains(out.String(), "NekoForge") {
		t.Fatalf("branding output = %q", out.String())
	}
	for _, forbidden := range []string{"( o.o )", `/\_/\`, "Session", "Shortcuts", "Ask anything"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("branding should be title only: %q", out.String())
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
	if !strings.Contains(output, "╭ You") || !strings.Contains(output, "╭ NekoForge") || !strings.Contains(output, "你好，我在。") {
		t.Fatalf("output = %q, want chat bubbles", output)
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
	if strings.Contains(output, "ReasonForge Multi-Agent") {
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
	if !strings.Contains(output, "\x1b[31m") {
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
	target := filepath.Join(root, "neko_generated.bat")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !strings.Contains(string(data), "@echo off") || !strings.Contains(output, "saved_file=") || !strings.Contains(output, filepath.Base(target)) {
		t.Fatalf("output=%q file=%q, want saved bat file", output, string(data))
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
	target := filepath.Join(targetDir, "neko_generated.bat")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !strings.Contains(string(data), "echo saved") || !strings.Contains(output, "saved_file=") || !strings.Contains(output, filepath.Base(target)) {
		t.Fatalf("output=%q file=%q, want specified directory save", output, string(data))
	}
}

func TestNekoAutoSavesChatResponseToExplicitFileName(t *testing.T) {
	root := t.TempDir()
	output := runTestConsole(t, "写入到 hello.txt\n/exit\n", Options{
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

func TestNekoDoesNotAutoSaveWithoutExplicitIntent(t *testing.T) {
	root := t.TempDir()
	output := runTestConsole(t, "帮我写一个bat批处理内容\n/exit\n", Options{
		Root: root,
		Chatter: func(ctx context.Context, req ChatRequest) (ChatResult, error) {
			return ChatResult{Response: "```bat\necho no save\n```"}, nil
		},
	})
	if strings.Contains(output, "saved_file=") {
		t.Fatalf("output=%q, should not auto-save without explicit save/write intent", output)
	}
	if _, err := os.Stat(filepath.Join(root, "neko_generated.bat")); !os.IsNotExist(err) {
		t.Fatalf("expected no generated file, stat err=%v", err)
	}
}

func TestNekoSlashHelp(t *testing.T) {
	output := runTestConsole(t, "/help\n/exit\n", Options{})
	if !strings.Contains(output, "NekoForge commands") || !strings.Contains(output, "/mode single") {
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
	if !strings.Contains(output, "Goodbye from NekoForge.") {
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
	if !strings.Contains(output, "model=mimo-v2.5-pro") {
		t.Fatalf("output = %q, want model", output)
	}
}

func TestNekoDisplaysProvider(t *testing.T) {
	output := runTestConsole(t, "/model\n/exit\n", Options{})
	if !strings.Contains(output, "provider=mimo") {
		t.Fatalf("output = %q, want provider", output)
	}
}

func TestNekoDisplaysAPIKeyStatusOnly(t *testing.T) {
	t.Setenv("MIMO_API_KEY", "sk-neko-status-secret")
	output := runTestConsole(t, "/model\n/exit\n", Options{})
	if !strings.Contains(output, "api_key_status=configured") {
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
	if !strings.Contains(output, "max_context_tokens=0 / 128k tokens") {
		t.Fatalf("output = %q, want context length", output)
	}
}

func TestNekoDisplaysReasoningLevel(t *testing.T) {
	output := runTestConsole(t, "/model\n/exit\n", Options{})
	if !strings.Contains(output, "reasoning_level=high") {
		t.Fatalf("output = %q, want high reasoning", output)
	}
}

func TestNekoUsesColdPalette(t *testing.T) {
	session := newTestSession(t, nil, Options{DryRun: true, DryRunSet: true})
	var out bytes.Buffer
	RenderHeader(&out, session)
	if !strings.Contains(out.String(), "\x1b[96m") || !strings.Contains(out.String(), "\x1b[97m") {
		t.Fatalf("output = %q, want cold cyan/silver ANSI theme", out.String())
	}
	if strings.Contains(out.String(), "\x1b[33m") || strings.Contains(out.String(), "\x1b[93m") {
		t.Fatalf("output = %q, should not use yellow/amber primary theme", out.String())
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
	if !strings.Contains(output, "tokens=input=10 cached=5 output=3 total=18") {
		t.Fatalf("output = %q, want token usage", output)
	}
}

func TestAssistantMessagesRemainVisible(t *testing.T) {
	output := runTestConsole(t, "/run summarize README\n/exit\n", Options{
		Runner: func(ctx context.Context, req RunRequest) (RunResult, error) {
			return RunResult{RunID: "run_visible", State: "succeeded", Output: "Assistant result stays visible."}, nil
		},
	})
	for _, want := range []string{"╭ User", "summarize README", "╭ Assistant", "Assistant result stays visible.", "run_id=run_visible"} {
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
	if !strings.Contains(output, "pricing=unavailable (pricing not configured)") {
		t.Fatalf("output = %q, want unavailable pricing", output)
	}
}

func TestNekoMarksEstimatedUsage(t *testing.T) {
	cost := ComputeCost(Usage{InputTokens: 1, Estimated: true}, &config.ModelPricingConfig{Currency: "CNY", InputPer1MTokens: 1})
	if !strings.Contains(FormatCost(cost), "estimated") {
		t.Fatalf("cost should be marked estimated")
	}
}

func TestNekoDoesNotHardcodePricing(t *testing.T) {
	session := newTestSession(t, nil, Options{NoColor: true, DryRun: true, DryRunSet: true})
	if got := FormatCost(ComputeCost(session.Usage, session.Pricing)); got != "unavailable (pricing not configured)" {
		t.Fatalf("cost = %q, want pricing unavailable", got)
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
	for _, want := range []string{"/preview wt_next", "/review wt_next", "/discard wt_next", "reasonforge patch apply wt_next"} {
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
		},
		Routing: config.RoutingConfig{DefaultModel: "mimo-v2.5-pro"},
	}
	if opt.Mode == "" {
		opt.Mode = "multi"
	}
	return NewSession(root, models, opt)
}
