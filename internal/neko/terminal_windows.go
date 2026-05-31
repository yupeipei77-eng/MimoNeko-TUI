//go:build windows

package neko

import (
	"syscall"
	"unsafe"
)

type coord struct {
	X int16
	Y int16
}

type smallRect struct {
	Left   int16
	Top    int16
	Right  int16
	Bottom int16
}

type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

func terminalSize() (int, int, bool) {
	handle, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return 0, 0, false
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetConsoleScreenBufferInfo")
	var info consoleScreenBufferInfo
	ret, _, _ := proc.Call(uintptr(handle), uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0, 0, false
	}
	cols := int(info.Window.Right - info.Window.Left + 1)
	rows := int(info.Window.Bottom - info.Window.Top + 1)
	if cols <= 0 || rows <= 0 {
		return 0, 0, false
	}
	return cols, rows, true
}
