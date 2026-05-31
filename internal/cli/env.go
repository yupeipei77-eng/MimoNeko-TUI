package cli

import "io"

type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
	Getwd  func() (string, error)
}
