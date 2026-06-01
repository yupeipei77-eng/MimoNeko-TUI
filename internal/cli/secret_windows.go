//go:build windows

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const windowsEnableEchoInput = 0x0004

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
	handle := syscall.Handle(file.Fd())
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	var original uint32
	ret, _, _ := getConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&original)))
	if ret == 0 {
		return nil, false
	}
	hiddenMode := original &^ windowsEnableEchoInput
	ret, _, _ = setConsoleMode.Call(uintptr(handle), uintptr(hiddenMode))
	if ret == 0 {
		return nil, false
	}
	return func() {
		setConsoleMode.Call(uintptr(handle), uintptr(original))
	}, true
}
