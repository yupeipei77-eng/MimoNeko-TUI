package cli

import (
	"io"
	"os"
)

func Run(args []string, env Env) int {
	if env.Stdout == nil {
		env.Stdout = io.Discard
	}
	if env.Stderr == nil {
		env.Stderr = io.Discard
	}
	if env.Stdin == nil {
		env.Stdin = os.Stdin
	}
	if env.Getwd == nil {
		env.Getwd = func() (string, error) { return ".", nil }
	}
	return commands.Dispatch(args, env)
}
