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

	"github.com/mimoneko/mimoneko/internal/modelprofile"
	"github.com/mimoneko/mimoneko/internal/neko/animation"
	"github.com/mimoneko/mimoneko/internal/neko/branding"
	"github.com/mimoneko/mimoneko/internal/neko/layout"
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

	Runner           RunHandler
	ModelTester      SimpleHandler
	ModelEnricher    SimpleHandler
	RunsLister       SimpleHandler
	Chatter          ChatHandler
	StreamingChatter StreamingChatHandler
	Previewer        PatchHandler
	Reviewer         PatchHandler
	Discarder        PatchHandler
}

type ChatMessage struct {
	Role    string
	Content string
}

type ChatRequest struct {
	Root     string
	Message  string
	Model    string
	Provider string
	Messages []ChatMessage
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
type StreamingChatHandler func(ctx context.Context, req ChatRequest, onChunk func(chunk StreamingChatChunk)) (ChatResult, error)
type SimpleHandler func(context.Context, Session) (string, error)
type PatchHandler func(context.Context, Session, string) (string, error)

// StreamingChatChunk represents a single streamed token.
type StreamingChatChunk struct {
	Text          string
	ReasoningText string
	Done          bool
}

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
	Session         Session
	Options         Options
	Messages        layout.MessageRenderer
	Input           layout.InputRenderer
	Runtime         layout.RuntimeRenderer
	Thinking        *layout.ThinkingRenderer
	Mascot          *branding.MascotAnimator
	screenActive    bool
	screenEntered   bool
	screenCols      int
	screenRows      int
	screenLog       []screenLine
	draft           string
	introActive     bool
	paletteOpen     bool
	paletteFilter   string
	paletteSelected int
	panelMode       string
	panelTitle      string
	panelContent    string
	chatHistory     []ChatMessage
}

func (c *Console) Run(ctx context.Context) int {
	c.Messages.SetNoColor(c.Session.NoColor)
	if shouldStreamOutput(c.Options.Out, c.Session.NoColor) {
		c.Messages.Delay = 2 * time.Millisecond
	}
	c.Runtime.SetNoColor(c.Session.NoColor)
	c.Thinking = layout.NewThinkingRenderer(c.Session.NoColor)
	c.Mascot = branding.NewMascotAnimator(c.Session.NoColor)
	c.setupScreen()
	if c.screenActive {
		c.introActive = true
		defer c.teardownScreen()
		if c.runRawInput(ctx) {
			return 0
		}
	}
	if c.screenActive {
		c.repaintScreen()
	} else if c.Options.Animate && !c.Session.NoColor {
		regions := layout.NewRegionLayout(branding.HeaderLineCount())
		animator := animation.NewFrameAnimator(branding.NewRenderer(false), regions, 90*time.Millisecond)
		animator.RenderStartup(c.Options.Out, HeaderDataFromSession(c.Session))
	} else {
		RenderHeader(c.Options.Out, c.Session)
	}
	if !c.screenActive {
		c.renderPrompt()
	}
	scanner := bufio.NewScanner(c.Options.In)
	for scanner.Scan() {
		if c.handleInputLine(ctx, scanner.Text()) {
			return 0
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(c.Options.Err, "neko input failed: %v\n", err)
		return 1
	}
	return 0
}

func (c *Console) enterWorkspace() {
	if c.Session.NoColor {
		return
	}
	file, ok := c.Options.Out.(*os.File)
	if !ok {
		return
	}
	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return
	}
	if c.screenActive && !c.screenEntered {
		fmt.Fprint(c.Options.Out, "\x1b[?1049h")
		c.screenEntered = true
	}
	fmt.Fprintf(c.Options.Out, "\x1b]0;MIMO | %s\x07\x1b[?25h\x1b[2J\x1b[H", c.Session.Model)
}

func (c *Console) teardownScreen() {
	if !c.screenActive || !c.screenEntered {
		return
	}
	fmt.Fprint(c.Options.Out, "\x1b[?25h\x1b[?1049l")
	c.screenEntered = false
}

func shouldStreamOutput(w io.Writer, noColor bool) bool {
	if noColor {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (c *Console) refreshInput() {
	c.Input = layout.NewInputRenderer(c.Session.Model, c.Session.Provider, c.Session.ReasoningStatusLabel())
	c.Input.Context = statusContextLabel(c.Session.ContextLabel())
	c.Input.Cost = FormatCost(ComputeCost(c.Session.Usage, c.Session.Pricing))
	c.Input.Tools = c.Session.ToolsUsed
	c.Input.Memory = c.Session.MemoryLabel()
	c.Input.Cache = c.Session.CacheLabel()
	c.Input.Latency = c.Session.LatencyLabel()
	c.Input.Session = c.Session.SessionLabel()
	c.Input.CommandUI = c.Session.CommandHint()
	c.Input.NoColor = c.Session.NoColor

	// Add thinking toggle hint
	if c.Thinking != nil {
		c.Input.ThoughtToggleHint = layout.ThoughtToggleHint(c.Thinking.ShowThoughts(), c.Session.NoColor)
	}
}

func (c *Console) renderPrompt() {
	c.refreshInput()
	if c.screenActive {
		c.repaintScreen()
		return
	}
	c.Input.RenderPrompt(c.Options.Out)
}

func (c *Console) handleInputLine(ctx context.Context, rawLine string) bool {
	if c.handleControlInput(ctx, rawLine) {
		c.renderPrompt()
		return false
	}
	if !c.screenActive {
		c.refreshInput()
		c.Input.RenderSubmittedPrompt(c.Options.Out, rawLine, shouldRewriteSubmittedPrompt(c.Options.In, c.Session.NoColor))
		c.Input.RenderPromptClose(c.Options.Out)
	}
	line := strings.TrimSpace(rawLine)
	if line == "" {
		c.renderPrompt()
		return false
	}
	if strings.HasPrefix(line, "/") {
		if c.handleSlash(ctx, line) {
			return true
		}
		c.renderPrompt()
		return false
	}
	c.chatMessage(ctx, line)
	c.renderPrompt()
	return false
}

func (c *Console) handleControlInput(ctx context.Context, rawLine string) bool {
	switch rawLine {
	case "\x10":
		c.cycleReasoning()
		return true
	case "\x12":
		c.cycleReasoning()
		return true
	case "\x14": // Ctrl+Shift+T
		c.toggleThinking()
		return true
	default:
		return false
	}
}

// toggleThinking switches the thinking display state.
func (c *Console) toggleThinking() {
	if c.Thinking != nil {
		c.Thinking.Toggle()
		if c.Thinking.ShowThoughts() {
			c.emitInfo("Thought shown · Ctrl+Shift+T 隐藏")
		} else {
			c.emitInfo("Thought hidden · Ctrl+Shift+T 显示")
		}
	}
}

func statusContextLabel(label string) string {
	label = strings.TrimSpace(label)
	label = strings.TrimSuffix(label, " tokens")
	parts := strings.Split(label, " / ")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]) + " / " + compactDetailSuffix(parts[1])
	}
	return strings.TrimSpace(label)
}

func compactDetailSuffix(value string) string {
	value = strings.TrimSpace(value)
	open := strings.LastIndex(value, "(")
	close := strings.LastIndex(value, ")")
	if open >= 0 && close > open {
		return strings.TrimSpace(value[open+1 : close])
	}
	return value
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
	case "/", "/commands", "/cmd", "/palette":
		if c.screenActive {
			c.openCommandPalette()
			return false
		}
		c.showRendered(func(w io.Writer) { RenderCommandPalette(w, c.Session) })
	case "/panel":
		c.handlePanel(arg)
	case "/exit", "/quit":
		c.emitInfo("Goodbye from MIMO.")
		return true
	case "/help":
		c.showRendered(func(w io.Writer) { RenderHelp(w, c.Session.NoColor) })
	case "/new":
		c.Messages.Reset()
		c.screenLog = nil
		c.Session.ResetConversation()
		c.chatHistory = nil
		c.introActive = c.screenActive
		c.enterWorkspace()
		c.emitInfo("New session.")
	case "/agents":
		if arg == "" {
			c.showRendered(func(w io.Writer) { RenderAgents(w, c.Session) })
			return false
		}
		if !c.Session.SetMode(arg) {
			c.emitInfo("usage: /agents single|multi")
			return false
		}
		c.emitInfo(fmt.Sprintf("agent=%s worktree=%v", c.Session.Mode, c.Session.Worktree))
	case "/mode":
		if len(fields) != 2 || !c.Session.SetMode(fields[1]) {
			c.emitInfo("usage: /mode single|multi")
			return false
		}
		c.emitInfo(fmt.Sprintf("mode=%s worktree=%v", c.Session.Mode, c.Session.Worktree))
	case "/reasoning", "/r":
		if len(fields) == 1 {
			c.cycleReasoning()
			return false
		}
		if len(fields) != 2 || !c.Session.SetReasoning(fields[1]) {
			c.emitInfo("usage: /reasoning [low|medium|high]")
			return false
		}
	case "/model":
		c.handleModel(ctx, arg)
	case "/models":
		if arg == "" {
			c.showRendered(func(w io.Writer) { RenderModels(w, c.Session) })
			return false
		}
		if !c.Session.SelectModel(arg) {
			c.emitInfo(fmt.Sprintf("model %q not found; run /models to inspect configured models", arg))
			return false
		}
		c.emitInfo(fmt.Sprintf("model=%s provider=%s", c.Session.Model, c.Session.Provider))
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
		c.emitInfo(fmt.Sprintf("unknown command %s", fields[0]))
		c.showRendered(func(w io.Writer) { RenderHelp(w, c.Session.NoColor) })
	}
	return false
}

func (c *Console) showRendered(render func(io.Writer)) {
	if c.screenActive {
		var buf strings.Builder
		render(&buf)
		c.appendScreen("info", strings.TrimRight(buf.String(), "\n"), false)
		return
	}
	render(c.Options.Out)
}

func (c *Console) handlePanel(arg string) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "diff":
		content := c.panelContent
		if strings.TrimSpace(content) == "" {
			content = "No diff preview loaded. Use /preview <worktree_id> to populate this panel."
		}
		c.setPanel("diff", "Diff", content)
	case "editor":
		c.setPanel("editor", "Editor", "Draft buffer")
	case "off", "hide", "none", "":
		c.clearPanel()
	default:
		c.emitInfo("usage: /panel diff|editor|off")
	}
}

func (c *Console) cycleReasoning() {
	c.Session.CycleReasoning()
}

func (c *Console) emitInfo(text string) {
	if c.screenActive {
		c.appendScreen("info", text, false)
		return
	}
	fmt.Fprintln(c.Options.Out, text)
}

func (c *Console) emitOutput(text string) {
	text = strings.TrimRight(text, "\n")
	if c.screenActive {
		c.appendScreen("info", text, false)
		return
	}
	fmt.Fprint(c.Options.Out, text)
	if !strings.HasSuffix(text, "\n") {
		fmt.Fprintln(c.Options.Out)
	}
}

func (c *Console) emitError(text string) {
	if c.screenActive {
		c.appendScreen("error", text, true)
		return
	}
	layout.RenderErrorMessage(c.Options.Out, "Error", text, c.Session.NoColor)
}

func (c *Console) emitUser(role, text string) {
	if c.screenActive {
		c.appendScreen("user", text, false)
		return
	}
	c.Messages.Add(role, text)
	c.Messages.RenderLast(c.Options.Out)
}

func (c *Console) emitAssistant(role, text string, isError bool) {
	if c.screenActive {
		c.appendScreen("assistant", text, isError)
		return
	}
	if isError {
		c.Messages.AddError(role, text)
	} else {
		c.Messages.Add(role, text)
	}
	c.Messages.RenderLast(c.Options.Out)
}

func (c *Console) emitRuntimeStage(stage string) {
	if c.screenActive {
		c.appendScreen("runtime", stage+"...", false)
		return
	}
	c.Runtime.RenderStage(c.Options.Out, stage)
}

func (c *Console) emitRuntimeDone(elapsed time.Duration) {
	label := "done · " + formatDuration(elapsed)
	if c.screenActive {
		c.appendScreen("done", label, false)
		return
	}
	c.Runtime.RenderDone(c.Options.Out, elapsed)
}

func (c *Console) emitThought(elapsed time.Duration) {
	if c.screenActive {
		c.appendScreen("thought", "Thought: "+formatDuration(elapsed), false)
		return
	}
	c.Runtime.RenderThoughtSummary(c.Options.Out)
}

func (c *Console) emitBuildBadge(elapsed time.Duration) {
	if c.screenActive {
		c.appendScreen("build", c.Session.Model+" · "+formatDuration(elapsed), false)
		return
	}
	layout.RenderBuildBadge(c.Options.Out, c.Session.Model, elapsed, c.Session.NoColor)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func (c *Console) handleModel(ctx context.Context, arg string) {
	switch strings.TrimSpace(arg) {
	case "":
		// Show interactive model configuration UI
		c.showModelConfig()
	case "test":
		c.callSimple(ctx, c.Options.ModelTester, "model test unavailable")
	case "enrich":
		c.callSimple(ctx, c.Options.ModelEnricher, "model enrich unavailable")
	default:
		// Handle subcommands
		fields := strings.Fields(arg)
		if len(fields) == 0 {
			c.showModelConfig()
			return
		}
		switch fields[0] {
		case "use":
			if len(fields) < 2 {
				c.emitInfo("usage: /model use <model_name>")
				return
			}
			modelName := fields[1]
			if c.Session.SelectModel(modelName) {
				c.emitOutput(fmt.Sprintf("✓ Switched to %s (provider: %s)", c.Session.Model, c.Session.Provider))
			} else {
				c.emitOutput(fmt.Sprintf("✗ Model %q not found; run /model to see available models", modelName))
			}
		case "list":
			c.showModelConfig()
		default:
			c.emitInfo("usage: /model, /model test, /model enrich, /model use <name>, /model list")
		}
	}
}

// showModelConfig displays the model configuration UI.
func (c *Console) showModelConfig() {
	config := NewModelConfig(c.Session)
	// Get available models from session
	config.AvailableModels = c.Session.AvailableModels()

	if c.screenActive {
		var buf strings.Builder
		config.RenderModelConfig(&buf)
		c.appendScreen("info", strings.TrimRight(buf.String(), "\n"), false)
		return
	}
	config.RenderModelConfig(c.Options.Out)
}

func (c *Console) runGoal(ctx context.Context, goal string) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return
	}
	c.Session.AddUserMemory(goal)
	c.emitUser("User", goal)
	c.Runtime.Reset()
	start := time.Now()
	c.emitRuntimeStage("planning")
	c.emitRuntimeStage("executing agent runtime")
	if c.Options.Runner == nil {
		c.Session.LastLatency = time.Since(start)
		c.emitRuntimeDone(c.Session.LastLatency)
		c.emitThought(c.Session.LastLatency)
		c.emitAssistant("Assistant", "run unavailable in this console", false)
		c.emitBuildBadge(c.Session.LastLatency)
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
	c.Session.ApplyActualUsage(result.Usage)
	c.Session.LastLatency = time.Since(start)
	c.Session.ToolsUsed++
	c.emitRuntimeStage("collecting result")
	c.emitRuntimeDone(c.Session.LastLatency)
	c.emitThought(c.Session.LastLatency)
	if err != nil {
		var msg strings.Builder
		fmt.Fprintf(&msg, "Run completed:\nstate=failed\nerror=%s", SanitizeOutput(err.Error()))
		if result.Output != "" {
			fmt.Fprintf(&msg, "\n%s", SanitizeOutput(result.Output))
		}
		c.Session.AddAssistantMemory(msg.String())
		c.emitAssistant("Assistant", msg.String(), true)
		c.emitBuildBadge(c.Session.LastLatency)
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
		fmt.Fprintf(&msg, "mimoneko patch apply %s\n", result.WorktreeID)
	}
	c.Session.AddAssistantMemory(msg.String())
	c.emitAssistant("Assistant", msg.String(), false)
	c.emitBuildBadge(c.Session.LastLatency)
}

func (c *Console) chatMessage(ctx context.Context, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	c.Session.AddUserMemory(message)
	c.emitUser("You", message)
	c.Runtime.Reset()
	start := time.Now()
	c.emitRuntimeStage("thinking")
	if c.tryLocalAutoSave(message, start) {
		return
	}
	if c.Options.StreamingChatter == nil && c.Options.Chatter == nil {
		c.Session.LastLatency = time.Since(start)
		c.emitRuntimeStage("generating response")
		c.emitRuntimeDone(c.Session.LastLatency)
		c.emitThought(c.Session.LastLatency)
		reply := defaultLocalChatReply(message)
		c.Session.AddAssistantMemory(reply)
		c.emitAssistant("MIMO", reply, false)
		c.emitBuildBadge(c.Session.LastLatency)
		c.addToChatHistory("user", message)
		c.addToChatHistory("assistant", reply)
		return
	}

	chatReq := ChatRequest{
		Root:     c.Session.Root,
		Message:  message,
		Model:    c.Session.Model,
		Provider: c.Session.Provider,
		Messages: c.chatHistory,
	}

	// Try streaming first
	if c.Options.StreamingChatter != nil {
		c.chatMessageStream(ctx, chatReq, start)
		return
	}

	// Fall back to non-streaming
	c.chatMessageSync(ctx, chatReq, start)
}

func (c *Console) addToChatHistory(role, content string) {
	c.chatHistory = append(c.chatHistory, ChatMessage{Role: role, Content: content})
	if len(c.chatHistory) > 50 {
		c.chatHistory = c.chatHistory[len(c.chatHistory)-50:]
	}
}

func (c *Console) tryLocalAutoSave(message string, start time.Time) bool {
	response, ok := localAutoSaveResponse(message)
	if !ok {
		return false
	}
	c.emitRuntimeStage("generating file")
	save, triggered, err := maybeAutoSaveChatResponse(c.Session.Root, message, response)
	if !triggered {
		return false
	}
	c.Session.LastLatency = time.Since(start)
	c.emitRuntimeDone(c.Session.LastLatency)
	c.emitThought(c.Session.LastLatency)
	replyIsError := false
	reply := ""
	if err != nil {
		reply = fmt.Sprintf("auto_save_failed=%s", SanitizeOutput(err.Error()))
		replyIsError = true
	} else {
		reply = formatAutoSaveResult(save)
	}
	c.Session.AddAssistantMemory(reply)
	c.emitAssistant("MIMO", reply, replyIsError)
	c.emitBuildBadge(c.Session.LastLatency)
	return true
}

func (c *Console) chatMessageStream(ctx context.Context, chatReq ChatRequest, start time.Time) {
	c.emitRuntimeStage("requesting model")

	var answerChunks []string
	var reasoningChunks []string
	var lastUsage Usage
	suppressBody := hasAutoSaveIntent(chatReq.Message)

	// Start thinking animation and mascot
	if c.Thinking != nil {
		c.Thinking.StartThinking()
		defer c.Thinking.StopThinking()
	}
	if c.Mascot != nil {
		c.Mascot.SetState(branding.MascotThinking)
		defer c.Mascot.SetState(branding.MascotDone)
	}

	var reasoningDone bool
	var answerStarted bool
	result, err := c.Options.StreamingChatter(ctx, chatReq, func(chunk StreamingChatChunk) {
		if chunk.ReasoningText != "" {
			reasoningChunks = append(reasoningChunks, chunk.ReasoningText)
			if suppressBody {
				return
			}
			if !reasoningDone {
				reasoningDone = true
				// Render thinking area
				if c.Thinking != nil {
					c.Thinking.AddThought(chunk.ReasoningText)
					if !c.screenActive {
						c.Thinking.RenderThinking(c.Options.Out)
					}
				}
			}
			if c.screenActive {
				c.updateScreenAssistantStream(strings.Join(append(reasoningChunks, answerChunks...), ""))
			}
		}
		if chunk.Text != "" {
			answerChunks = append(answerChunks, chunk.Text)
			if suppressBody {
				return
			}
			// If this is the first answer chunk, render separator
			if !answerStarted && len(reasoningChunks) > 0 {
				answerStarted = true
				if c.Thinking != nil && !c.screenActive {
					c.Thinking.StopThinking()
					c.Thinking.ClearThinkingLine(c.Options.Out)
					c.Thinking.RenderThinkingSeparator(c.Options.Out)
				}
				// Switch mascot to answering state
				if c.Mascot != nil {
					c.Mascot.SetState(branding.MascotAnswering)
				}
			}
			if c.screenActive {
				c.updateScreenAssistantStream(strings.Join(append(reasoningChunks, answerChunks...), ""))
			} else {
				fmt.Fprint(c.Options.Out, chunk.Text)
			}
		}
	})

	if err == nil {
		lastUsage = result.Usage
	}

	c.Session.ApplyActualUsage(lastUsage)
	c.Session.LastLatency = time.Since(start)
	allChunks := append(reasoningChunks, answerChunks...)
	if !suppressBody && !c.screenActive && len(allChunks) > 0 {
		fmt.Fprintln(c.Options.Out)
	}
	c.emitRuntimeDone(c.Session.LastLatency)
	c.emitThought(c.Session.LastLatency)

	if err != nil {
		reply := fmt.Sprintf("I could not reach the chat model yet: %s\nUse /run <goal> for agent work, or /model to inspect provider status.", SanitizeOutput(err.Error()))
		c.Session.AddAssistantMemory(reply)
		if c.screenActive {
			c.finalizeScreenAssistantStream(reply, true)
		} else {
			c.emitAssistant("MIMO", reply, true)
		}
		c.emitBuildBadge(c.Session.LastLatency)
		return
	}

	streamedReply := ""
	if len(allChunks) > 0 {
		streamedReply = SanitizeOutput(strings.Join(allChunks, ""))
	}
	saveSource := result.Response
	if strings.TrimSpace(saveSource) == "" && len(allChunks) > 0 {
		saveSource = strings.Join(allChunks, "")
	}
	reply := SanitizeOutput(result.Response)
	if reply == "" && streamedReply != "" {
		reply = streamedReply
	}
	replyIsError := false
	if save, triggered, saveErr := maybeAutoSaveChatResponse(c.Session.Root, chatReq.Message, saveSource); triggered {
		if saveErr != nil {
			reply = fmt.Sprintf("auto_save_failed=%s", SanitizeOutput(saveErr.Error()))
			replyIsError = true
		} else {
			reply = formatAutoSaveResult(save)
		}
	}
	if replyIsError {
		c.Session.AddAssistantMemory(reply)
		if c.screenActive {
			c.finalizeScreenAssistantStream(reply, true)
		} else {
			c.emitAssistant("MIMO", reply, true)
		}
	} else {
		c.Session.AddAssistantMemory(reply)
		c.addToChatHistory("user", chatReq.Message)
		c.addToChatHistory("assistant", reply)
		if c.screenActive {
			c.finalizeScreenAssistantStream(reply, false)
		} else if suppressBody || len(allChunks) == 0 {
			c.emitAssistant("MIMO", reply, false)
		} else if reply != streamedReply {
			c.emitOutput(strings.TrimSpace(strings.TrimPrefix(reply, streamedReply)))
		}
	}
	c.emitBuildBadge(c.Session.LastLatency)
}

func (c *Console) chatMessageSync(ctx context.Context, chatReq ChatRequest, start time.Time) {
	c.emitRuntimeStage("requesting model")
	result, err := c.Options.Chatter(ctx, chatReq)
	c.Session.ApplyActualUsage(result.Usage)
	c.Session.LastLatency = time.Since(start)
	c.emitRuntimeStage("generating response")
	c.emitRuntimeDone(c.Session.LastLatency)
	c.emitThought(c.Session.LastLatency)
	if err != nil {
		reply := fmt.Sprintf("I could not reach the chat model yet: %s\nUse /run <goal> for agent work, or /model to inspect provider status.", SanitizeOutput(err.Error()))
		c.Session.AddAssistantMemory(reply)
		c.emitAssistant("MIMO", reply, true)
		c.emitBuildBadge(c.Session.LastLatency)
		return
	}
	reply := SanitizeOutput(result.Response)
	replyIsError := false
	if save, triggered, saveErr := maybeAutoSaveChatResponse(c.Session.Root, chatReq.Message, result.Response); triggered {
		if saveErr != nil {
			reply = fmt.Sprintf("auto_save_failed=%s", SanitizeOutput(saveErr.Error()))
			replyIsError = true
		} else {
			reply = formatAutoSaveResult(save)
		}
	}
	if replyIsError {
		c.Session.AddAssistantMemory(reply)
		c.emitAssistant("MIMO", reply, true)
	} else {
		c.Session.AddAssistantMemory(reply)
		c.addToChatHistory("user", chatReq.Message)
		c.addToChatHistory("assistant", reply)
		c.emitAssistant("MIMO", reply, false)
	}
	c.emitBuildBadge(c.Session.LastLatency)
}

func formatAutoSaveResult(save autoSaveResult) string {
	return fmt.Sprintf("saved_file=%s\nsaved_file_name=%s", save.Path, filepath.Base(save.Path))
}

func defaultLocalChatReply(message string) string {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "hi" || lower == "hello" || lower == "你好" || lower == "您好" {
		return "Ready. I can chat here; use /run <goal> when you want agent work."
	}
	return "Ready. Plain text is chat; /run <goal> starts the local agent."
}

func (c *Console) callSimple(ctx context.Context, handler SimpleHandler, unavailable string) {
	if handler == nil {
		fmt.Fprintln(c.Options.Out, unavailable)
		return
	}
	start := time.Now()
	c.Runtime.Reset()
	c.emitRuntimeStage("executing command")
	output, err := handler(ctx, c.Session)
	c.Session.LastLatency = time.Since(start)
	c.Session.ToolsUsed++
	c.emitRuntimeDone(c.Session.LastLatency)
	c.emitThought(c.Session.LastLatency)
	if err != nil {
		c.emitError(fmt.Sprintf("error=%s", SanitizeOutput(err.Error())))
		return
	}
	c.emitOutput(SanitizeOutput(output))
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
	start := time.Now()
	c.Runtime.Reset()
	c.emitRuntimeStage("executing " + label)
	output, err := handler(ctx, c.Session, worktreeID)
	c.Session.LastLatency = time.Since(start)
	c.Session.ToolsUsed++
	c.emitRuntimeDone(c.Session.LastLatency)
	c.emitThought(c.Session.LastLatency)
	if err != nil {
		c.emitError(fmt.Sprintf("%s failed: %s", label, SanitizeOutput(err.Error())))
		return
	}
	safeOutput := SanitizeOutput(output)
	if c.screenActive && strings.Contains(label, "preview") {
		c.setPanel("diff", "Diff", safeOutput)
		c.emitInfo("Diff panel updated.")
		return
	}
	c.emitOutput(safeOutput)
}

var reasoningPattern = regexp.MustCompile(`(?i)(chain-of-thought|hidden reasoning|private reasoning)`)

func SanitizeOutput(text string, secrets ...string) string {
	safe := modelprofile.SanitizeText(text, secrets...)
	return reasoningPattern.ReplaceAllString(safe, "<redacted reasoning>")
}
