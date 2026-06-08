package version

const Name = "MimoNeko"

// Version can be overridden at build time with:
// go build -ldflags "-X github.com/yupeipei77-eng/MimoNeko-TUI/internal/version.Version=vX.Y.Z"
var Version = "0.1.4-beta"

func String() string {
	return Name + " " + Version
}
