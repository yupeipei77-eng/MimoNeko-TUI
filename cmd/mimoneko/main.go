package main

import (
	"os"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.Env{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
		Getwd:  os.Getwd,
	}))
}
