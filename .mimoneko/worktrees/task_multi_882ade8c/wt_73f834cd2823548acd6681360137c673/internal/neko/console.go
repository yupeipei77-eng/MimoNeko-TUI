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

	"github.com/nekonomimo/nekonomimo/internal/modelprofile"
)

type Options struct {
	Root      string
	Mode      string
	Model     string
	Reasoning string
	DryRun    bool
	DryRunSet bool
	NoColor   bool
	In        io.Reader
	Out       io.Writer
	Err       io.Writer

	Runner        RunHandler
	ModelTester   SimpleHandler
	ModelEnricher SimpleHandler
	RunsLister    SimpleHandler
	Previewer     PatchHandler
	Reviewer      PatchHandler
	Discarder     PatchHandler
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
	Session Session
	Options Options
}

func (c *Console) Run(ctx context.Context) int {
	RenderHeader(c.Options.Out, c.Session)
	scanner := bufio.NewScanner(c.Options.In)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			if c.handleSlash(ctx, line) {
				return 0
			}
			continue
		}
		c.runGoal(ctx, line)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(c.Options.Err, "neko input failed: %v\n", err)
		return 1
	}
	return 0
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
	if c.Options.Runner == nil {
		fmt.Fprintln(c.Options.Out, "run unavailable in this console")
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
		fmt.Fprintf(c.Options.Out, "Run completed:\nstate=failed\nerror=%s\n", SanitizeOutput(err.Error()))
		if result.Output != "" {
			fmt.Fprintln(c.Options.Out, SanitizeOutput(result.Output))
		}
		return
	}
	state := result.State
	if state == "" {
		state = "succeeded"
	}
	fmt.Fprintln(c.Options.Out, "Run completed:")
	if result.RunID != "" {
		fmt.Fprintf(c.Options.Out, "run_id=%s\n", result.RunID)
	}
	fmt.Fprintf(c.Options.Out, "state=%s\n", state)
	if result.WorktreeID != "" {
		fmt.Fprintf(c.Options.Out, "worktree_id=%s\n", result.WorktreeID)
	}
	if result.Recommendation != "" {
		fmt.Fprintf(c.Options.Out, "recommendation=%s\n", result.Recommendation)
	}
	fmt.Fprintf(c.Options.Out, "tokens=%s\n", FormatTokens(c.Session.Usage))
	fmt.Fprintf(c.Options.Out, "cost=%s\n", FormatCost(ComputeCost(c.Session.Usage, c.Session.Pricing)))
	if result.Output != "" {
		fmt.Fprintln(c.Options.Out, SanitizeOutput(result.Output))
	}
	if result.WorktreeID != "" {
		fmt.Fprintln(c.Options.Out, "Next:")
		fmt.Fprintf(c.Options.Out, "/preview %s\n", result.WorktreeID)
		fmt.Fprintf(c.Options.Out, "/review %s\n", result.WorktreeID)
		fmt.Fprintf(c.Options.Out, "/discard %s\n", result.WorktreeID)
		fmt.Fprintln(c.Options.Out, "CLI apply:")
		fmt.Fprintf(c.Options.Out, "NekoMIMO patch apply %s\n", result.WorktreeID)
	}
}

func (c *Console) callSimple(ctx context.Context, handler SimpleHandler, unavailable string) {
	if handler == nil {
		fmt.Fprintln(c.Options.Out, unavailable)
		return
	}
	output, err := handler(ctx, c.Session)
	if err != nil {
		fmt.Fprintf(c.Options.Out, "error=%s\n", SanitizeOutput(err.Error()))
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
		fmt.Fprintf(c.Options.Out, "%s failed: %s\n", label, SanitizeOutput(err.Error()))
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
