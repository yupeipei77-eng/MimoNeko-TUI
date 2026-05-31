package layout

import "fmt"

type Region struct {
	Name      string
	StartLine int
	Height    int
}

type RegionLayout struct {
	Header  Region
	Message Region
	Input   Region
}

func NewRegionLayout(headerHeight int) RegionLayout {
	if headerHeight < 1 {
		headerHeight = 1
	}
	return RegionLayout{
		Header:  Region{Name: "header", StartLine: 1, Height: headerHeight},
		Message: Region{Name: "message", StartLine: headerHeight + 1, Height: 0},
		Input:   Region{Name: "input", StartLine: headerHeight + 1, Height: 1},
	}
}

func (l RegionLayout) Validate() error {
	if l.Header.StartLine < 1 || l.Header.Height < 1 {
		return fmt.Errorf("invalid header region")
	}
	if l.Message.StartLine <= l.Header.StartLine {
		return fmt.Errorf("message region must start after header")
	}
	if l.Input.StartLine < l.Message.StartLine {
		return fmt.Errorf("input region must not overlap message region")
	}
	return nil
}

func (l RegionLayout) HeaderEndLine() int {
	return l.Header.StartLine + l.Header.Height - 1
}

func (l RegionLayout) HeaderContains(line int) bool {
	return line >= l.Header.StartLine && line <= l.HeaderEndLine()
}
