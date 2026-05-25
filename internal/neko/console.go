package neko

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/reasonforge/reasonforge/internal/modelprofile"
	"github.com/reasonforge/reasonforge/internal/neko/animation"
	"github.com/reasonforge/reasonforge/internal/neko/branding"
	"github.com/reasonforge/reasonforge/internal/neko/layout"
)

type Options struct {
	Root      string
	Mode      string
	Model     string
	Reasoning string
	DryRun    bool
	DryRunSet bool
	NoColor   bool
	Animate   bool
	In        io.Reader
	Out       io.Writer
	Err       io.Writer

	Runner        RunHandler
	ModelTester   SimpleHandler
	ModelEnricher SimpleHandler
	RunsLister    SimpleHandler
	Chatter       ChatHandler
	Previewer     PatchHandler
	Reviewer      PatchHandler
	Discarder     PatchHandler
}

type ChatRequest struct {
	Root     string
	Message  string
	Model    string
	Provider string
}

type ChatResult struct {
	Response string
	Usage    Usage
}

type RunRequest struct {
	Root      string
	Goal      string
	Mode      string
	Model     string
	Reasoning string
	DryRun    bool
	Worktree  bool
}

type RunResult struct {
	RunID          string
	State          string
	WorktreeID     string
	Recommendation string
	Output         string
	Usage          Usage
	ExitCode       int
}

type RunHandler func(context.Context, RunRequest) (RunResult, error)
type ChatHandler func(context.Context, ChatRequest) (ChatResult, error)
type SimpleHandler func(context.Context, Session) (string, error)
type PatchHandler func(context.Context, Session, string) (string, error)

func Run(ctx context.Context, opt Options) int {
	if opt.In == nil {
		opt.In = os.Stdin
	}
	if opt.Out == nil {
		opt.Out = io.Discard
	}
	if opt.Err == nil {
		opt.Err = io.Discard
	}
	root := strings.TrimSpace(opt.Root)
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(opt.Err, "neko failed: %v\n", err)
			return 1
		}
		root = wd
	}
	absRoot, err := filepath.Abs(root)
	if err == nil {
		root = absRoot
	}
	models, err := modelprofile.Load(root)
	if err != nil {
		fmt.Fprintf(opt.Err, "neko failed: %s\n", modelprofile.SanitizeText(err.Error()))
		return 1
	}
	session := NewSession(root, models, opt)
	console := Console{Session: session, Options: opt}
	return console.Run(ctx)
}

type Console struct {
	Session  Session
	Options  Options
	Messages layout.MessageRenderer
	Input    layout.InputRenderer
}

func (c *Console) Run(ctx context.Context) int {
	c.Messages.SetNoColor(c.Session.NoColor)
	c.Input = layout.NewInputRenderer(c.Session.Model, c.Session.Provider, c.Session.ReasoningLabel())
	if c.Options.Animate && !c.Session.NoColor {
		regions := layout.NewRegionLayout(branding.HeaderLineCount())
		animator := animation.NewFrameAnimator(branding.NewRenderer(false), regions, 90*time.Millisecond)
		animator.RenderStartup(c.Options.Out, HeaderDataFromSession(c.Session))
	} else {
		RenderHeader(c.Options.Out, c.Session)
	}
	c.Input.RenderPrompt(c.Options.Out)
	scanner := bufio.NewScanner(c.Options.In)
	for scanner.Scan() {
		rawLine := scanner.Text()
		c.Input.RenderSubmittedPrompt(c.Options.Out, rawLine, shouldRewriteSubmittedPrompt(c.Options.In, c.Session.NoColor))
		c.Input.RenderPromptClose(c.Options.Out)
		line := strings.TrimSpace(rawLine)
		if line == "" {
			c.Input.RenderPrompt(c.Options.Out)
			continue
		}
		if strings.HasPrefix(line, "/") {
			if c.handleSlash(ctx, line) {
				return 0
			}
			c.Input.RenderPrompt(c.Options.Out)
			continue
		}
		c.chatMessage(ctx, line)
		c.Input.RenderPrompt(c.Options.Out)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(c.Options.Err, "neko input failed: %v\n", err)
		return 1
	}
	return 0
}

func shouldRewriteSubmittedPrompt(r io.Reader, noColor bool) bool {
	if noColor {
		return false
	}
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (c *Console) handleSlash(ctx context.Context, line string) bool {
	fields := strings.Fields(line)
	cmd := strings.ToLower(fields[0])
	arg := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
	switch cmd {
	case "/exit", "/quit":
		fmt.Fprintln(c.Options.Out, "Goodbye from NekoForge.")
		return true
	case "/help":
		RenderHelp(c.Options.Out, c.Session.NoColor)
	case "/mode":
		if len(fields) != 2 || !c.Session.SetMode(fields[1]) {
			fmt.Fprintln(c.Options.Out, "usage: /mode single|multi")
			return false
		}
		fmt.Fprintf(c.Options.Out, "mode=%s worktree=%v\n", c.Session.Mode, c.Session.Worktree)
	case "/reasoning":
		if len(fields) != 2 || !c.Session.SetReasoning(fields[1]) {
			fmt.Fprintln(c.Options.Out, "usage: /reasoning low|medium|high")
			return false
		}
		fmt.Fprintf(c.Options.Out, "reasoning=%s\n", c.Session.Reasoning)
	case "/model":
		c.handleModel(ctx, arg)
	case "/runs":
		c.callSimple(ctx, c.Options.RunsLister, "recent runs unavailable")
	case "/preview":
		c.callPatch(ctx, c.Options.Previewer, arg, "patch preview")
	case "/review":
		c.callPatch(ctx, c.Options.Reviewer, arg, "patch review")
	case "/discard":
		c.callPatch(ctx, c.Options.Discarder, arg, "patch discard")
	case "/run":
		c.runGoal(ctx, arg)
	default:
		fmt.Fprintf(c.Options.Out, "unknown command %s\n", fields[0])
		RenderHelp(c.Options.Out, c.Session.NoColor)
	}
	return false
}

func (c *Console) handleModel(ctx context.Context, arg string) {
	switch strings.TrimSpace(arg) {
	case "":
		fmt.Fprintf(c.Options.Out, "provider=%s\n", emptyAsUnknown(c.Session.Provider))
		fmt.Fprintf(c.Options.Out, "model=%s\n", emptyAsUnknown(c.Session.Model))
		fmt.Fprintf(c.Options.Out, "base_url_host=%s\n", emptyAsUnknown(c.Session.BaseURLHost))
		fmt.Fprintf(c.Options.Out, "api_key_status=%s\n", emptyAsUnknown(c.Session.APIKeyStatus))
		fmt.Fprintf(c.Options.Out, "max_context_tokens=%s\n", c.Session.ContextLabel())
		fmt.Fprintf(c.Options.Out, "reasoning_level=%s\n", c.Session.ReasoningLabel())
		fmt.Fprintf(c.Options.Out, "pricing=%s\n", FormatCost(ComputeCost(c.Session.Usage, c.Session.Pricing)))
	case "test":
		c.callSimple(ctx, c.Options.ModelTester, "model test unavailable")
	case "enrich":
		c.callSimple(ctx, c.Options.ModelEnricher, "model enrich unavailable")
	default:
		fmt.Fprintln(c.Options.Out, "usage: /model, /model test, or /model enrich")
	}
}

func (c *Console) runGoal(ctx context.Context, goal string) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return
	}
	c.Messages.Add("User", goal)
	c.Messages.RenderLast(c.Options.Out)
	if c.Options.Runner == nil {
		c.Messages.Add("Assistant", "run unavailable in this console")
		c.Messages.RenderLast(c.Options.Out)
		return
	}
	result, err := c.Options.Runner(ctx, RunRequest{
		Root:      c.Session.Root,
		Goal:      goal,
		Mode:      c.Session.Mode,
		Model:     c.Session.Model,
		Reasoning: c.Session.Reasoning,
		DryRun:    c.Session.DryRun,
		Worktree:  c.Session.Worktree,
	})
	if result.Usage.TotalTokens != 0 || result.Usage.InputTokens != 0 || result.Usage.OutputTokens != 0 || result.Usage.CachedTokens != 0 {
		c.Session.Usage = NormalizeUsage(result.Usage)
	}
	if err != nil {
		var msg strings.Builder
		fmt.Fprintf(&msg, "Run completed:\nstate=failed\nerror=%s", SanitizeOutput(err.Error()))
		if result.Output != "" {
			fmt.Fprintf(&msg, "\n%s", SanitizeOutput(result.Output))
		}
		c.Messages.AddError("Assistant", msg.String())
		c.Messages.RenderLast(c.Options.Out)
		return
	}
	state := result.State
	if state == "" {
		state = "succeeded"
	}
	var msg strings.Builder
	fmt.Fprintln(&msg, "Run completed:")
	if result.RunID != "" {
		fmt.Fprintf(&msg, "run_id=%s\n", result.RunID)
	}
	fmt.Fprintf(&msg, "state=%s\n", state)
	if result.WorktreeID != "" {
		fmt.Fprintf(&msg, "worktree_id=%s\n", result.WorktreeID)
	}
	if result.Recommendation != "" {
		fmt.Fprintf(&msg, "recommendation=%s\n", result.Recommendation)
	}
	fmt.Fprintf(&msg, "tokens=%s\n", FormatTokens(c.Session.Usage))
	fmt.Fprintf(&msg, "cost=%s\n", FormatCost(ComputeCost(c.Session.Usage, c.Session.Pricing)))
	if result.Output != "" {
		fmt.Fprintln(&msg, SanitizeOutput(result.Output))
	}
	if result.WorktreeID != "" {
		fmt.Fprintln(&msg, "Next:")
		fmt.Fprintf(&msg, "/preview %s\n", result.WorktreeID)
		fmt.Fprintf(&msg, "/review %s\n", result.WorktreeID)
		fmt.Fprintf(&msg, "/discard %s\n", result.WorktreeID)
		fmt.Fprintln(&msg, "CLI apply:")
		fmt.Fprintf(&msg, "reasonforge patch apply %s\n", result.WorktreeID)
	}
	c.Messages.Add("Assistant", msg.String())
	c.Messages.RenderLast(c.Options.Out)
}

func (c *Console) chatMessage(ctx context.Context, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	c.Messages.Add("You", message)
	c.Messages.RenderLast(c.Options.Out)
	if c.Options.Chatter == nil {
		c.Messages.Add("NekoForge", defaultLocalChatReply(message))
		c.Messages.RenderLast(c.Options.Out)
		return
	}
	result, err := c.Options.Chatter(ctx, ChatRequest{
		Root:     c.Session.Root,
		Message:  message,
		Model:    c.Session.Model,
		Provider: c.Session.Provider,
	})
	if result.Usage.TotalTokens != 0 || result.Usage.InputTokens != 0 || result.Usage.OutputTokens != 0 || result.Usage.CachedTokens != 0 {
		c.Session.Usage = NormalizeUsage(result.Usage)
	}
	if err != nil {
		reply := fmt.Sprintf("I could not reach the chat model yet: %s\nUse /run <goal> for agent work, or /model to inspect provider status.", SanitizeOutput(err.Error()))
		c.Messages.AddError("NekoForge", reply)
		c.Messages.RenderLast(c.Options.Out)
		return
	}
	reply := SanitizeOutput(result.Response)
	replyIsError := false
	if save, triggered, saveErr := maybeAutoSaveChatResponse(c.Session.Root, message, result.Response); triggered {
		if saveErr != nil {
			reply += fmt.Sprintf("\n\nauto_save_failed=%s", SanitizeOutput(saveErr.Error()))
			replyIsError = true
		} else {
			reply += fmt.Sprintf("\n\nsaved_file=%s", save.Path)
		}
	}
	if replyIsError {
		c.Messages.AddError("NekoForge", reply)
	} else {
		c.Messages.Add("NekoForge", reply)
	}
	c.Messages.RenderLast(c.Options.Out)
}

func defaultLocalChatReply(message string) string {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "hi" || lower == "hello" || lower == "你好" || lower == "您好" {
		return "你好，我是 NekoForge。可以直接和我聊天；需要执行代码任务时，用 /run <goal>。"
	}
	return "我在。直接输入是聊天；执行代码任务请用 /run <goal>。"
}

func (c *Console) callSimple(ctx context.Context, handler SimpleHandler, unavailable string) {
	if handler == nil {
		fmt.Fprintln(c.Options.Out, unavailable)
		return
	}
	output, err := handler(ctx, c.Session)
	if err != nil {
		layout.RenderErrorMessage(c.Options.Out, "Error", fmt.Sprintf("error=%s", SanitizeOutput(err.Error())), c.Session.NoColor)
		return
	}
	fmt.Fprint(c.Options.Out, SanitizeOutput(output))
	if !strings.HasSuffix(output, "\n") {
		fmt.Fprintln(c.Options.Out)
	}
}

func (c *Console) callPatch(ctx context.Context, handler PatchHandler, worktreeID, label string) {
	worktreeID = strings.TrimSpace(worktreeID)
	if worktreeID == "" {
		fmt.Fprintf(c.Options.Out, "usage: /%s <worktree_id>\n", strings.Fields(label)[1])
		return
	}
	if handler == nil {
		fmt.Fprintf(c.Options.Out, "%s requested worktree_id=%s\n", label, worktreeID)
		return
	}
	output, err := handler(ctx, c.Session, worktreeID)
	if err != nil {
		layout.RenderErrorMessage(c.Options.Out, "Error", fmt.Sprintf("%s failed: %s", label, SanitizeOutput(err.Error())), c.Session.NoColor)
		return
	}
	fmt.Fprint(c.Options.Out, SanitizeOutput(output))
	if !strings.HasSuffix(output, "\n") {
		fmt.Fprintln(c.Options.Out)
	}
}

var reasoningPattern = regexp.MustCompile(`(?i)(chain-of-thought|hidden reasoning|private reasoning)`)

func SanitizeOutput(text string, secrets ...string) string {
	safe := modelprofile.SanitizeText(text, secrets...)
	return reasoningPattern.ReplaceAllString(safe, "<redacted reasoning>")
}
