package cli

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/config"
	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/pathutil"
	webserver "github.com/yupeipei77-eng/MimoNeko-TUI/internal/server"
)

var serveCommandRun = func(s *webserver.LocalServer) error {
	return s.ListenAndServe()
}

var openBrowserURL = func(url string) error {
	switch {
	case pathutil.IsWindows():
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case pathutil.IsDarwin():
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

type ServeCommand struct{}

func (c *ServeCommand) Name() string { return "serve" }

func (c *ServeCommand) Run(args []string, env Env) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dir := fs.String("dir", "", "project root")
	host := fs.String("host", webserver.DefaultHost, "host to bind (default 127.0.0.1)")
	port := fs.Int("port", webserver.DefaultPort, "port to bind")
	open := fs.Bool("open", false, "open the dashboard in a browser")
	pollInterval := fs.Duration("poll-interval", webserver.DefaultPollInterval, "browser polling interval")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rejectExtraArgs(fs, env) {
		return 2
	}

	root, err := resolveRoot(*dir, env)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(env.Stderr, "serve failed: %v\n", err)
		return 1
	}

	if *host == "0.0.0.0" {
		fmt.Fprintln(env.Stderr, "warning: --host 0.0.0.0 exposes the local dashboard on all interfaces")
	}

	srv := webserver.NewLocalServer(root, cfg, webserver.Options{
		Host:         *host,
		Port:         *port,
		PollInterval: *pollInterval,
	})

	fmt.Fprintf(env.Stdout, "MimoNeko Web Dashboard\n")
	fmt.Fprintf(env.Stdout, "listening on %s\n", srv.URL())

	if *open {
		if err := openBrowserURL(srv.URL()); err != nil {
			fmt.Fprintf(env.Stderr, "warning: could not open browser: %v\n", err)
		}
	}

	if err := serveCommandRun(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(env.Stderr, "serve failed: %v\n", err)
		return 1
	}
	return 0
}

func init() {
	commands.Register(&ServeCommand{})
}
