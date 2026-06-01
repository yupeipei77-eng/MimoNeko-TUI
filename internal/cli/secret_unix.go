//go:build !windows

package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func promptSecretLine(reader *bufio.Reader, env Env, prompt string) string {
	fmt.Fprintf(env.Stdout, "%s: ", prompt)
	restore, hidden := disableInputEcho(env.Stdin)
	input, _ := reader.ReadString('\n')
	if restore != nil {
		restore()
	}
	if hidden {
		fmt.Fprintln(env.Stdout)
	}
	return strings.TrimSpace(input)
}

func disableInputEcho(stdin any) (func(), bool) {
	file, ok := stdin.(*os.File)
	if !ok || file == nil {
		return nil, false
	}
	cmd := exec.Command("stty", "-echo")
	cmd.Stdin = file
	if err := cmd.Run(); err != nil {
		return nil, false
	}
	return func() {
		restore := exec.Command("stty", "echo")
		restore.Stdin = file
		_ = restore.Run()
	}, true
}
