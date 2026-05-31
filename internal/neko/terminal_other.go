//go:build !windows

package neko

import (
	"os"
	"strconv"
)

func terminalSize() (int, int, bool) {
	cols, colsErr := strconv.Atoi(os.Getenv("COLUMNS"))
	rows, rowsErr := strconv.Atoi(os.Getenv("LINES"))
	if colsErr != nil || rowsErr != nil || cols <= 0 || rows <= 0 {
		return 0, 0, false
	}
	return cols, rows, true
}
