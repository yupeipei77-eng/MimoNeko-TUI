package version

const Name = "MimoNeko"

// Version can be overridden at build time with:
// go build -ldflags "-X github.com/mimoneko/mimoneko/internal/version.Version=vX.Y.Z"
var Version = "0.1.0-beta"

func String() string {
	return Name + " " + Version
}
