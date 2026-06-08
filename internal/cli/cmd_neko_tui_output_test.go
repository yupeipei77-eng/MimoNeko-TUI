package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/neko"
)

func TestRunNekoGoalReadOnlyUsesSingleAgentPath(t *testing.T) {
	oldRunAgent := runAgent
	oldRunMultiAgent := runMultiAgent
	defer func() {
		runAgent = oldRunAgent
		runMultiAgent = oldRunMultiAgent
	}()

	agentCalled := false
	multiCalled := false
	runAgent = func(args []string, env Env) int {
		agentCalled = true
		if hasArg(args, "--worktree") {
			t.Fatalf("read-only goal args = %v, should not request a worktree", args)
		}
		fmt.Fprintln(env.Stdout, "MimoNeko Run")
		fmt.Fprintln(env.Stdout, "Result:")
		fmt.Fprintln(env.Stdout, "project looks good")
		fmt.Fprintln(env.Stdout, "Run ID:")
		fmt.Fprintln(env.Stdout, "run_readonly")
		return 0
	}
	runMultiAgent = func(args []string, env Env) int {
		multiCalled = true
		return 0
	}

	result, err := runNekoGoal(neko.RunRequest{
		Root:     t.TempDir(),
		Goal:     "\u68c0\u67e5\u9879\u76ee\u6587\u4ef6",
		Mode:     "multi",
		DryRun:   true,
		Worktree: true,
	})
	if err != nil {
		t.Fatalf("runNekoGoal returned error: %v", err)
	}
	if !agentCalled || multiCalled {
		t.Fatalf("agentCalled=%v multiCalled=%v, want single-agent read-only route", agentCalled, multiCalled)
	}
	if !result.ReadOnly {
		t.Fatalf("result.ReadOnly = false, want true")
	}
	if result.Output != "project looks good" {
		t.Fatalf("result.Output = %q, want cleaned Result body", result.Output)
	}
	if result.RunID != "run_readonly" {
		t.Fatalf("result.RunID = %q, want run_readonly", result.RunID)
	}
}

func TestRunNekoGoalMultiOutputIsSummarizedForTUI(t *testing.T) {
	oldRunAgent := runAgent
	oldRunMultiAgent := runMultiAgent
	defer func() {
		runAgent = oldRunAgent
		runMultiAgent = oldRunMultiAgent
	}()

	runAgent = func(args []string, env Env) int {
		t.Fatalf("write goal should not use single-agent route")
		return 1
	}
	runMultiAgent = func(args []string, env Env) int {
		fmt.Fprintln(env.Stdout, "MimoNeko Multi-Agent")
		fmt.Fprintln(env.Stdout, "run_id=run_multi")
		fmt.Fprintln(env.Stdout, "state=succeeded")
		fmt.Fprintln(env.Stdout, "worktree_id=wt_123")
		fmt.Fprintln(env.Stdout, "plan_steps=2 risk_level=low")
		fmt.Fprintln(env.Stdout, "  step 1: Update README")
		fmt.Fprintln(env.Stdout, "  step 2: Run tests")
		fmt.Fprintln(env.Stdout, "iteration 0: recommendation=approve")
		fmt.Fprintln(env.Stdout, "final_recommendation=approve")
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, "To apply changes, run:")
		fmt.Fprintln(env.Stdout, "  mimoneko patch apply wt_123")
		return 0
	}

	result, err := runNekoGoal(neko.RunRequest{
		Root:   t.TempDir(),
		Goal:   "fix README",
		Mode:   "multi",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("runNekoGoal returned error: %v", err)
	}
	if result.ReadOnly {
		t.Fatalf("result.ReadOnly = true, want false")
	}
	if result.WorktreeID != "wt_123" || result.Recommendation != "approve" {
		t.Fatalf("result = %+v, want worktree and recommendation extracted", result)
	}
	for _, want := range []string{"Plan:", "1: Update README", "2: Run tests"} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("result.Output = %q, want %q", result.Output, want)
		}
	}
	for _, forbidden := range []string{"MimoNeko Multi-Agent", "To apply changes", "patch apply"} {
		if strings.Contains(result.Output, forbidden) {
			t.Fatalf("result.Output = %q, should not contain raw CLI text %q", result.Output, forbidden)
		}
	}
}

func TestIsReadOnlyNekoGoal(t *testing.T) {
	tests := []struct {
		name string
		goal string
		want bool
	}{
		{name: "english inspect", goal: "inspect project files", want: true},
		{name: "chinese inspect", goal: "\u68c0\u67e5\u9879\u76ee\u6587\u4ef6", want: true},
		{name: "fix overrides review", goal: "review and fix README", want: false},
		{name: "patch overrides review", goal: "review and patch README", want: false},
		{name: "chinese generate overrides analyze", goal: "\u5206\u6790\u5e76\u751f\u6210\u62a5\u544a", want: false},
		{name: "plain chat", goal: "hello there", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isReadOnlyNekoGoal(tt.goal); got != tt.want {
				t.Fatalf("isReadOnlyNekoGoal(%q) = %v, want %v", tt.goal, got, tt.want)
			}
		})
	}
}

func TestRunNekoGoalSingleExtractsWorktreeID(t *testing.T) {
	oldRunAgent := runAgent
	oldRunMultiAgent := runMultiAgent
	defer func() {
		runAgent = oldRunAgent
		runMultiAgent = oldRunMultiAgent
	}()

	runAgent = func(args []string, env Env) int {
		if !hasArg(args, "--worktree") {
			t.Fatalf("single worktree args = %v, want --worktree", args)
		}
		fmt.Fprintln(env.Stdout, "MimoNeko Run")
		fmt.Fprintln(env.Stdout, "\u2713 Completed")
		fmt.Fprintln(env.Stdout, "Result:")
		fmt.Fprintln(env.Stdout, "updated file")
		fmt.Fprintln(env.Stdout, "Run ID:")
		fmt.Fprintln(env.Stdout, "run_single")
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, "Worktree:")
		fmt.Fprintln(env.Stdout, "ID       wt_single")
		return 0
	}
	runMultiAgent = func(args []string, env Env) int {
		t.Fatalf("single mode should not call multi-agent")
		return 1
	}

	result, err := runNekoGoal(neko.RunRequest{
		Root:     t.TempDir(),
		Goal:     "fix README",
		Mode:     "single",
		DryRun:   true,
		Worktree: true,
	})
	if err != nil {
		t.Fatalf("runNekoGoal returned error: %v", err)
	}
	if result.WorktreeID != "wt_single" {
		t.Fatalf("result.WorktreeID = %q, want wt_single", result.WorktreeID)
	}
	if result.State != "succeeded" {
		t.Fatalf("result.State = %q, want succeeded", result.State)
	}
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
