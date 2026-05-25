package branding

type MascotFrame struct {
	Lines []string
}

var miniCatFrames = []MascotFrame{
	{Lines: []string{
		` /\_/\  `,
		`( o_o )~`,
		` /|_|\  `,
	}},
	{Lines: []string{
		` /\_/\  `,
		`( -_o )~`,
		` /|_|\  `,
	}},
	{Lines: []string{
		` /\_/\  `,
		`~( o_o ) `,
		` /|_|\  `,
	}},
	{Lines: []string{
		`  /\_/\ `,
		` ( o_o )~`,
		`  /|_|\ `,
	}},
}

func FrameCount() int {
	// The current premium shell uses a static centered title and dialog.
	// Keeping zero frames avoids startup redraw flicker in real terminals.
	return 0
}

func CatFrame(frame int) MascotFrame {
	if len(miniCatFrames) == 0 {
		return MascotFrame{}
	}
	if frame < 0 {
		frame = -frame
	}
	return miniCatFrames[frame%len(miniCatFrames)]
}

func CatFrameLines(frame int) []string {
	source := CatFrame(frame).Lines
	lines := make([]string, len(source))
	copy(lines, source)
	return lines
}
