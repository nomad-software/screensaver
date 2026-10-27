package input

import (
	"os"
	"strings"

	"github.com/nomad-software/screensaver/cmd/launcher/input/mutter"
	"github.com/nomad-software/screensaver/cmd/launcher/input/wayland"
	"github.com/nomad-software/screensaver/output"
)

// GetInput checks for keyboard and mouse events using the backend that suits
// the current desktop and when detected sends a signal on the returned channel.
func GetInput() chan struct{} {
	if isGnome() {
		output.LaunchInfo("using mutter for input detection")
		return mutter.GetInput()
	}

	output.LaunchInfo("using wayland for input detection")
	return wayland.GetInput()
}

// isGnome returns true if the current desktop is Gnome. XDG_CURRENT_DESKTOP is
// a colon separated list of desktop names, e.g. 'ubuntu:GNOME'.
func isGnome() bool {
	for desktop := range strings.SplitSeq(os.Getenv("XDG_CURRENT_DESKTOP"), ":") {
		if strings.EqualFold(desktop, "GNOME") {
			return true
		}
	}
	return false
}
