package cli

import (
	"os"
	"os/exec"

	"github.com/nomad-software/screensaver/output"
	"github.com/nomad-software/screensaver/process"
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

// Inhibited returns the first of the processes named that is running, if any
// is, in which case the saver should not be launched.
func Inhibited(names []string) (string, bool) {
	running, err := process.Running(names)
	if err != nil {
		output.LaunchErr("cannot list processes: %s", err)
		return "", false
	}
	for _, name := range names {
		if len(running[name]) > 0 {
			return name, true
		}
	}
	return "", false
}
