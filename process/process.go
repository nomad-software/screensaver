// Package process finds which of a list of programs are running, by their
// process names, as pgrep does.
package process

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
)

// Running returns those of the names given that have a process running, with
// the ids of their processes.
func Running(names []string) (map[string][]int, error) {
	return runningIn("/proc", names)
}

// runningIn looks for the processes in the proc file system mounted at root.
// A process matches a name if its name, which the kernel cuts to 15
// characters, is that name, or if the file name of the program it was started
// as is.
func runningIn(root string, names []string) (map[string][]int, error) {
	found := make(map[string][]int)
	if len(names) == 0 {
		return found, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || !e.IsDir() {
			continue
		}
		// A process can end while it is being looked at, which leaves its
		// files unreadable; it is then simply not running.
		dir := filepath.Join(root, e.Name())
		comm, _ := os.ReadFile(filepath.Join(dir, "comm"))
		comm = bytes.TrimRight(comm, "\n")
		cmdline, _ := os.ReadFile(filepath.Join(dir, "cmdline"))
		var program string
		if arg0, _, _ := bytes.Cut(cmdline, []byte{0}); len(arg0) > 0 {
			program = filepath.Base(string(arg0))
		}
		for _, name := range names {
			if name != "" && (name == string(comm) || name == program) {
				found[name] = append(found[name], pid)
			}
		}
	}
	return found, nil
}
