package cli

import (
	"fmt"
	"io"

	"github.com/mimoneko/mimoneko/internal/config"
)

func printUsage(w io.Writer) {
	ui := newCLIUI()
	ui.PrintHeader(w, "MioNeko CLI")
	fmt.Fprintln(w, "Usage: mimoneko <command>")
	fmt.Fprintln(w, "       mimoneko \"your goal\"")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	for _, name := range commands.Names() {
		switch name {
		case "version":
			fmt.Fprintln(w, "  version      Print version information")
		case "init":
			fmt.Fprintf(w, "  init         Create local %s config files\n", config.DirName())
		case "doctor":
			fmt.Fprintln(w, "  doctor       Validate local MimoNeko configuration")
		case "cache-report":
			fmt.Fprintln(w, "  cache-report Show prefix cache statistics")
		case "models":
			fmt.Fprintln(w, "  models       Show model provider configuration")
		case "model":
			fmt.Fprintln(w, "  model        Manage model providers and profiles")
		case "tools":
			fmt.Fprintln(w, "  tools        List available tools and their status")
		case "tool-run":
			fmt.Fprintln(w, "  tool-run     Execute a tool with arguments")
		case "run":
			fmt.Fprintln(w, "  run          Run an agent task")
		case "multi-run":
			fmt.Fprintln(w, "  multi-run    Run multi-agent task (Planner->Coder->Reviewer)")
		case "patch":
			fmt.Fprintln(w, "  patch        Manage patches (list, preview, validate, review, apply, discard)")
		case "runs":
			fmt.Fprintln(w, "  runs         List recent runs with state and progress")
		case "run-status":
			fmt.Fprintln(w, "  run-status   Show detailed status for a specific run")
		case "run-events":
			fmt.Fprintln(w, "  run-events   Show events for a specific run")
		case "dashboard":
			fmt.Fprintln(w, "  dashboard    Local TUI dashboard (list runs, view details, watch)")
		case "serve":
			fmt.Fprintln(w, "  serve        Start local Web Dashboard")
		case "neko":
			fmt.Fprintln(w, "  neko         Start MimoNeko terminal console")
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s Run a task:\n", ui.Icon("cat"))
	fmt.Fprintln(w, "  mimoneko \"修改 README\"")
	fmt.Fprintln(w, "  mimoneko run \"Reply OK\"")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s Setup:\n", ui.Icon("secret"))
	fmt.Fprintln(w, "  mimoneko auth login")
	fmt.Fprintln(w, "  mimoneko model test")
}

func printNekoUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: neko [--dir <project_root>] [--mode single|multi] [--model name] [--reasoning low|medium|high] [--dry-run] [--no-color] [--new-window]")
	fmt.Fprintln(w, "       mimoneko neko [same flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "MimoNeko is a local terminal console powered by MimoNeko.")
	fmt.Fprintln(w, "Defaults: mode=multi dry-run=true worktree=true for multi-agent runs.")
	fmt.Fprintln(w, "Inside MimoNeko: type / for commands, /reasoning to cycle levels, /new for a fresh session.")
}

func printInitNextSteps(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintln(w, "1. Set up a model:")
	fmt.Fprintln(w, "   mimoneko model setup")
	fmt.Fprintln(w, "2. Test the model:")
	fmt.Fprintln(w, "   mimoneko model test")
	fmt.Fprintln(w, "3. Run a safe task:")
	fmt.Fprintln(w, "   mimoneko run --goal \"Reply OK\" --dry-run")
	fmt.Fprintln(w, "4. Start dashboard:")
	fmt.Fprintln(w, "   mimoneko serve")
	fmt.Fprintln(w, "Windows API key example:")
	fmt.Fprintln(w, "   setx MIMO_API_KEY \"your-key\"")
}
