//go:build !windows

package neko

import (
	"fmt"
	"os"
)

func enableRawInput(file *os.File) (func(), error) {
	if file == nil {
		return nil, fmt.Errorf("nil input file")
	}
	return nil, fmt.Errorf("raw input is not available on this platform")
}
