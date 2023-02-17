package cli

import (
	"os"
	"os/exec"

	"github.com/nomad-software/screensaver/output"
)

// Launch launches a screensaver by its command line name.
// This function will block until the launched screensaver exits.
func Launch(saver string, args ...string) {
	output.LaunchInfo("launching screensaver: %s", saver)

	cmd := exec.Command(saver, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	if err != nil {
		output.LaunchErr("screensaver launch error: %s", err)
	}
}
