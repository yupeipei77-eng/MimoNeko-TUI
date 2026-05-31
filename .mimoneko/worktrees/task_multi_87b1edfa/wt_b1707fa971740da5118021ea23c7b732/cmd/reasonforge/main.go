package main

import (
	"os"

	"github.com/nekonomimo/nekonomimo/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.Env{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Getwd:  os.Getwd,
	}))
}
