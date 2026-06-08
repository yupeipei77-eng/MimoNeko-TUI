package main

import (
	"os"

	"github.com/yupeipei77-eng/MimoNeko-TUI/internal/cli"
)

func main() {
	args := append([]string{"neko"}, os.Args[1:]...)
	os.Exit(cli.Run(args, cli.Env{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
		Getwd:  os.Getwd,
	}))
}
