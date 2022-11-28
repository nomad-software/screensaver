package cli

import (
	"os"
	"os/exec"

	"github.com/nomad-software/screensaver/output"
)

// Launch launches a screensaver by its command name.
func Launch(saver string, args ...string) error {
	output.LaunchInfo("launching screensaver: %s", saver)

	cmd := exec.Command(saver, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
