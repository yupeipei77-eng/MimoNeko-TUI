package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/modelprofile"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/neko"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/pathutil"
)

type NekoCommand struct{}

func (c *NekoCommand) Name() string { return "neko" }

func (c *NekoCommand) Run(args []string, env Env) int {
	if len(args) > 0 {
		switch args[0] {
		case "status":
			return runNekoWorkflowStatus(args[1:], env)
		case "diff":
			return runNekoWorkflowDiff(args[1:], env)
		case "plan":
			return runNekoWorkflowPlan(args[1:], env)
		case "cache":
			return runNekoWorkflowCache(args[1:], env)
		case "tools":
			return runNekoWorkflowTools(args[1:], env)
		case "events":
			return runNekoWorkflowEvents(args[1:], env)
		}
	}
	if hasHelpFlag(args) {
		printNekoUsage(env.Stdout)
		return 0
	}
	fs := flag.NewFlagSet("neko", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	mode := fs.String("mode", "multi", "execution mode: single or multi")
	model := fs.String("model", "", "model name")
	reasoning := fs.String("reasoning", "", "reasoning effort: low, medium, high")
	dryRun := fs.Bool("dry-run", true, "dry run mode (no side effects)")
	noColor := fs.Bool("no-color", false, "disable color output")
	newWindow := fs.Bool("new-window", false, "launch in a new terminal window")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}

	root, err := resolveNekoRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	if *newWindow {
		if err := launchNekoNewWindow(root, *mode, *model, *reasoning, *dryRun, *noColor); err != nil {
			fmt.Fprintln(env.Stderr, err)
			return 1
		}
		return 0
	}

	options := neko.Options{
		Root:      root,
		Mode:      *mode,
		Model:     *model,
		Reasoning: *reasoning,
		DryRun:    *dryRun,
		DryRunSet: true,
		NoColor:   *noColor,
		Animate:   shouldAnimateNeko(env.Stdout, *noColor),
		In:        env.Stdin,
		Out:       env.Stdout,
		Err:       env.Stderr,
	}
	options.Runner = func(ctx context.Context, req neko.RunRequest) (neko.RunResult, error) {
		return runNekoGoal(req)
	}
	options.ModelTester = func(ctx context.Context, session neko.Session) (string, error) {
		result, err := modelprofile.Test(ctx, session.Root, modelprofile.TestOptions{
			Provider: session.Provider,
			Model:    session.Model,
			Prompt:   "Reply with OK only.",
		})
		if err != nil {
			return "", err
		}
		var out strings.Builder
		fmt.Fprintf(&out, "model=%s\nprovider=%s\nstatus=%s\nlatency_ms=%d\n", result.Model, result.Provider, result.Status, result.LatencyMs)
		if result.Status == "ok" {
			fmt.Fprintf(&out, "response=%s\n", result.Response)
		} else {
			fmt.Fprintf(&out, "error=%s\n", result.Error)
		}
		return out.String(), nil
	}
	options.ModelEnricher = func(ctx context.Context, session neko.Session) (string, error) {
		result, err := modelprofile.Enrich(session.Root, modelprofile.EnrichOptions{Provider: session.Provider})
		if err != nil {
			return "", err
		}
		var out strings.Builder
		printEnrichResult(&out, result)
		return out.String(), nil
	}
	options.RunsLister = func(ctx context.Context, session neko.Session) (string, error) {
		return captureCLI(func(stdout, stderr io.Writer) int {
			return runRuns([]string{"--dir", session.Root}, Env{Stdout: stdout, Stderr: stderr, Stdin: strings.NewReader("")})
		})
	}
	options.Chatter = func(ctx context.Context, req neko.ChatRequest) (neko.ChatResult, error) {
		messages := make([]modelprofile.ChatMessage, len(req.Messages))
		for i, msg := range req.Messages {
			messages[i] = modelprofile.ChatMessage{Role: msg.Role, Content: msg.Content}
		}
		result, err := modelprofile.Chat(ctx, req.Root, modelprofile.ChatOptions{
			Provider:  req.Provider,
			Model:     req.Model,
			Prompt:    neko.PrepareModelPrompt(req.Message),
			MaxTokens: neko.ModelMaxTokens(req.Message),
			Messages:  messages,
		})
		if err != nil {
			return neko.ChatResult{}, err
		}
		return neko.ChatResult{
			Response:  result.Response,
			Reasoning: result.Reasoning,
			Usage: neko.Usage{
				InputTokens:      result.PromptTokens,
				CachedTokens:     result.CachedTokens,
				CacheHitTokens:   result.CacheHitTokens,
				CacheMissTokens:  result.CacheMissTokens,
				NativeCacheKnown: result.NativeCacheKnown,
				OutputTokens:     result.CompletionTokens,
				TotalTokens:      result.TotalTokens,
				Estimated:        result.TotalTokens == 0 || !result.CachedTokensKnown,
			},
		}, nil
	}
	options.StreamingChatter = func(ctx context.Context, req neko.ChatRequest, onChunk func(neko.StreamingChatChunk)) (neko.ChatResult, error) {
		var fullReasoning strings.Builder
		messages := make([]modelprofile.ChatMessage, len(req.Messages))
		for i, msg := range req.Messages {
			messages[i] = modelprofile.ChatMessage{Role: msg.Role, Content: msg.Content}
		}
		streamFunc, err := modelprofile.ChatStream(req.Root, modelprofile.ChatOptions{
			Provider:  req.Provider,
			Model:     req.Model,
			Prompt:    neko.PrepareModelPrompt(req.Message),
			MaxTokens: neko.ModelMaxTokens(req.Message),
			Messages:  messages,
		})
		if err != nil {
			return options.Chatter(ctx, req)
		}

		result, err := streamFunc(ctx, func(chunk modelprofile.ChatStreamChunk) {
			if chunk.ReasoningText != "" {
				fullReasoning.WriteString(chunk.ReasoningText)
			}
			if chunk.Text != "" || chunk.ReasoningText != "" {
				onChunk(neko.StreamingChatChunk{
					Text:          chunk.Text,
					ReasoningText: chunk.ReasoningText,
				})
			}
			if chunk.Done {
				onChunk(neko.StreamingChatChunk{Done: true})
			}
		})

		if err != nil {
			return neko.ChatResult{}, err
		}

		return neko.ChatResult{
			Response:  result.Response,
			Reasoning: result.Reasoning,
			Usage: neko.Usage{
				InputTokens:      result.PromptTokens,
				CachedTokens:     result.CachedTokens,
				CacheHitTokens:   result.CacheHitTokens,
				CacheMissTokens:  result.CacheMissTokens,
				NativeCacheKnown: result.NativeCacheKnown,
				OutputTokens:     result.CompletionTokens,
				TotalTokens:      result.TotalTokens,
				Estimated:        result.TotalTokens == 0 || !result.CachedTokensKnown,
			},
		}, nil
	}
	options.Previewer = func(ctx context.Context, session neko.Session, worktreeID string) (string, error) {
		return captureCLI(func(stdout, stderr io.Writer) int {
			return runPatchPreview([]string{"--dir", session.Root, worktreeID}, Env{Stdout: stdout, Stderr: stderr, Stdin: strings.NewReader("")})
		})
	}
	options.Reviewer = func(ctx context.Context, session neko.Session, worktreeID string) (string, error) {
		return captureCLI(func(stdout, stderr io.Writer) int {
			return runPatchReview([]string{"--dir", session.Root, "--no-tests", worktreeID}, Env{Stdout: stdout, Stderr: stderr, Stdin: strings.NewReader("")})
		})
	}
	options.Discarder = func(ctx context.Context, session neko.Session, worktreeID string) (string, error) {
		return captureCLI(func(stdout, stderr io.Writer) int {
			return runPatchDiscard([]string{"--dir", session.Root, worktreeID}, Env{Stdout: stdout, Stderr: stderr, Stdin: strings.NewReader("")})
		})
	}
	return neko.Run(context.Background(), options)
}

func launchNekoNewWindow(root, mode, model, reasoning string, dryRun, noColor bool) error {
	exe := pathutil.ExecutablePath()
	if exe == "" {
		return errors.New("cannot determine executable path")
	}
	args := []string{}
	if pathutil.IsMimoNekoExe() {
		args = append(args, "neko")
	}
	args = append(args, "--dir", root, "--mode", mode)
	if model != "" {
		args = append(args, "--model", model)
	}
	if reasoning != "" {
		args = append(args, "--reasoning", reasoning)
	}
	if dryRun {
		args = append(args, "--dry-run")
	}
	if noColor {
		args = append(args, "--no-color")
	}
	if pathutil.IsWindows() {
		startArgs := append([]string{"/c", "start", "MimoNeko", exe}, args...)
		return exec.Command("cmd", startArgs...).Start()
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func resolveNekoRoot(dir string, env Env) (string, error) {
	if strings.TrimSpace(dir) != "" {
		return pathutil.AbsPath(dir), nil
	}
	root, err := env.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	absRoot := pathutil.AbsPath(root)
	if found, ok := findNekoProjectRoot(absRoot); ok {
		return found, nil
	}
	if found, ok := findNekoDefaultRoot(); ok {
		return found, nil
	}
	return "", fmt.Errorf("MimoNeko could not find %s/models.yaml from %s\nRun:\n  cd <project_root>\n  neko\nor:\n  neko --dir <project_root>\nIf this is a new project, run:\n  mimoneko init\n  mimoneko model setup", config.DirName(), absRoot)
}

func findNekoProjectRoot(start string) (string, bool) {
	current := pathutil.CleanPath(start)
	for {
		if hasNekoModels(current) {
			return current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

func findNekoDefaultRoot() (string, bool) {
	for _, candidate := range nekoDefaultRootCandidates() {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "\ufeff")
		if candidate == "" {
			continue
		}
		abs := pathutil.AbsPath(candidate)
		if hasNekoModels(abs) {
			return abs, true
		}
		if found, ok := findNekoProjectRoot(abs); ok {
			return found, true
		}
	}
	return "", false
}

func nekoDefaultRootCandidates() []string {
	var candidates []string
	if value := pathutil.NekoRootFromEnv(); value != "" {
		candidates = append(candidates, value)
	}
	if path := nekoDefaultRootFile(); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			candidates = append(candidates, strings.TrimSpace(string(data)))
		}
	}
	return candidates
}

func nekoDefaultRootFile() string {
	return pathutil.NekoDefaultRootFilePath()
}

func hasNekoModels(root string) bool {
	if _, err := os.Stat(filepath.Join(root, config.DirName(), "models.yaml")); err == nil {
		return true
	}
	return false
}

func runNekoGoal(req neko.RunRequest) (neko.RunResult, error) {
	args := []string{"--dir", req.Root, "--goal", req.Goal}
	if req.DryRun {
		args = append(args, "--dry-run")
	} else {
		args = append(args, "--dry-run=false")
	}
	readOnly := isReadOnlyNekoGoal(req.Goal)
	if req.Mode == "single" || readOnly {
		if req.Worktree && !readOnly {
			args = append(args, "--worktree")
		}
		output, err := captureCLI(func(stdout, stderr io.Writer) int {
			return runAgent(args, Env{Stdout: stdout, Stderr: stderr, Stdin: strings.NewReader("")})
		})
		return neko.RunResult{
			RunID:      firstNonEmpty(extractCLIValue(output, "run_id"), extractRunCommandID(output)),
			State:      firstNonEmpty(extractCLIValue(output, "state"), extractRunCommandState(output)),
			WorktreeID: firstNonEmpty(extractCLIValue(output, "worktree_id"), extractRunCommandWorktreeID(output)),
			Output:     extractRunCommandResult(output),
			Usage:      neko.Usage{Estimated: true},
			ReadOnly:   readOnly,
		}, err
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	output, err := captureCLI(func(stdout, stderr io.Writer) int {
		return runMultiAgent(args, Env{Stdout: stdout, Stderr: stderr, Stdin: strings.NewReader("")})
	})
	return neko.RunResult{
		RunID:          extractCLIValue(output, "run_id"),
		State:          extractCLIValue(output, "state"),
		WorktreeID:     extractCLIValue(output, "worktree_id"),
		Recommendation: firstNonEmpty(extractCLIValue(output, "final_recommendation"), extractCLIValue(output, "recommendation")),
		Output:         summarizeMultiRunOutputForTUI(output),
		Usage:          neko.Usage{Estimated: true},
	}, err
}

// Exposed here for neko's callback references → directly calls PatchCommand methods.
var runPatchPreview = (&PatchCommand{}).runPreview
var runPatchReview = (&PatchCommand{}).runReview
var runPatchDiscard = (&PatchCommand{}).runDiscard
var runAgent = (&RunCommand{}).Run
var runMultiAgent = (&MultiRunCommand{}).Run
var runRuns = (&RunsCommand{}).Run

func init() {
	commands.Register(&NekoCommand{})
}
