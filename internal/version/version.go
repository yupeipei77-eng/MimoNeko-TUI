package version

const Name = "reasonforge"

// Version can be overridden at build time with:
// go build -ldflags "-X github.com/reasonforge/reasonforge/internal/version.Version=vX.Y.Z"
var Version = "0.1.0-dev"

func String() string {
	return Name + " " + Version
}
