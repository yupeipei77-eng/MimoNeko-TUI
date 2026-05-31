//go:build windows

package neko

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	enableProcessedInput       = 0x0001
	enableLineInput            = 0x0002
	enableEchoInput            = 0x0004
	enableVirtualTerminalInput = 0x0200
)

func enableRawInput(file *os.File) (func(), error) {
	if file == nil {
		return nil, fmt.Errorf("nil input file")
	}
	handle := syscall.Handle(file.Fd())
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	var original uint32
	ret, _, err := getConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&original)))
	if ret == 0 {
		return nil, err
	}
	raw := original
	raw &^= enableProcessedInput | enableLineInput | enableEchoInput
	raw |= enableVirtualTerminalInput
	ret, _, err = setConsoleMode.Call(uintptr(handle), uintptr(raw))
	if ret == 0 {
		return nil, err
	}
	return func() {
		setConsoleMode.Call(uintptr(handle), uintptr(original))
	}, nil
}
