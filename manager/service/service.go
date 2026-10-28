// Package service installs the launcher as a systemd user service, which
// starts with the desktop session, and controls it through systemctl.
package service

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Name is the name of the launcher's unit.
const Name = "screensaver-launcher.service"

// Unit returns the unit file which runs the launcher with the arguments given.
// It is part of the graphical session, so that it starts once the desktop has
// given the user's services its display, and stops with it. The launcher
// exits if it cannot reach the display or its port is taken, so it is started
// again after a pause.
func Unit(launcher string, args []string) string {
	exec := quote(launcher)
	for _, arg := range args {
		exec += " " + quote(arg)
	}
	var b strings.Builder
	b.WriteString("[Unit]\n")
	b.WriteString("Description=Screensaver launcher\n")
	b.WriteString("PartOf=graphical-session.target\n")
	b.WriteString("After=graphical-session.target\n")
	b.WriteString("\n[Service]\n")
	b.WriteString("ExecStart=" + exec + "\n")
	b.WriteString("Restart=on-failure\n")
	b.WriteString("RestartSec=5\n")
	b.WriteString("\n[Install]\n")
	b.WriteString("WantedBy=graphical-session.target\n")
	return b.String()
}

// quote makes a path or other argument one argument of ExecStart: % and $ are
// doubled, which would otherwise begin a specifier or a variable, and it is
// put in double quotes, with \ and " escaped, if it has a space or a quote.
func quote(arg string) string {
	arg = strings.NewReplacer("%", "%%", "$", "$$").Replace(arg)
	if !strings.ContainsAny(arg, " \t\"'\\") {
		return arg
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(arg) + `"`
}

// Status is the state of the service.
type Status struct {
	// Installed is whether the unit file is there.
	Installed bool

	// Enabled is whether the service starts with the session.
	Enabled bool

	// State is systemd's active state (active, inactive, failed,
	// activating...) and Sub its sub-state (running, auto-restart...).
	State, Sub string

	// PID is the id of the launcher's process, 0 when it is not running.
	PID int
}

// Running is whether the launcher is running.
func (s Status) Running() bool {
	return s.State == "active" && s.PID != 0
}

// Manager installs and controls the service.
type Manager struct {
	// Dir is the directory the unit file is written to.
	Dir string

	// Systemctl runs systemctl --user with the arguments given, and returns
	// what it printed. Tests replace it.
	Systemctl func(args ...string) (string, error)
}

// New returns a manager for the user's own services, whose unit files are
// kept in systemd/user in $XDG_CONFIG_HOME, or in ~/.config without it.
func New() (*Manager, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Manager{Dir: filepath.Join(dir, "systemd", "user"), Systemctl: systemctl}, nil
}

// systemctl runs systemctl --user. A command which fails returns an error with
// what systemctl said.
func systemctl(args ...string) (string, error) {
	out, err := exec.Command("systemctl", append([]string{"--user"}, args...)...).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return string(out), fmt.Errorf("systemctl %s: %s", strings.Join(args, " "), msg)
		}
		return string(out), fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// Path returns the unit file's path.
func (m *Manager) Path() string {
	return filepath.Join(m.Dir, Name)
}

// Install writes the unit file and enables the service, so that it starts with
// every session, and (re)starts it now, so that a changed unit takes effect.
func (m *Manager) Install(launcher string, args []string) error {
	if err := m.write(launcher, args); err != nil {
		return err
	}
	return m.run([]string{"daemon-reload"}, []string{"enable", Name}, []string{"restart", Name})
}

// Reconfigure writes the unit file again with the arguments given, and
// restarts the launcher with them if it is running. A service that is
// stopped is left stopped, to start with them when it is next started.
func (m *Manager) Reconfigure(launcher string, args []string) error {
	if err := m.write(launcher, args); err != nil {
		return err
	}
	return m.run([]string{"daemon-reload"}, []string{"try-restart", Name})
}

// write writes the unit file.
func (m *Manager) write(launcher string, args []string) error {
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.Path(), []byte(Unit(launcher, args)), 0o644)
}

// Uninstall stops and disables the service and removes its unit file.
func (m *Manager) Uninstall() error {
	if _, err := os.Stat(m.Path()); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err := m.run([]string{"disable", "--now", Name}); err != nil {
		return err
	}
	if err := os.Remove(m.Path()); err != nil {
		return err
	}
	return m.run([]string{"daemon-reload"})
}

// Start starts the service.
func (m *Manager) Start() error {
	return m.run([]string{"start", Name})
}

// Stop stops the service.
func (m *Manager) Stop() error {
	return m.run([]string{"stop", Name})
}

// run runs each systemctl command in turn, stopping at the first to fail.
func (m *Manager) run(commands ...[]string) error {
	for _, args := range commands {
		if _, err := m.Systemctl(args...); err != nil {
			return err
		}
	}
	return nil
}

// Status returns the state of the service.
func (m *Manager) Status() (Status, error) {
	var s Status
	if _, err := os.Stat(m.Path()); err == nil {
		s.Installed = true
	}
	out, err := m.Systemctl("show", Name, "--property=ActiveState,SubState,UnitFileState,MainPID")
	if err != nil {
		return s, err
	}
	scan := bufio.NewScanner(strings.NewReader(out))
	for scan.Scan() {
		key, value, _ := strings.Cut(scan.Text(), "=")
		switch key {
		case "ActiveState":
			s.State = value
		case "SubState":
			s.Sub = value
		case "UnitFileState":
			s.Enabled = value == "enabled"
		case "MainPID":
			s.PID, _ = strconv.Atoi(value)
		}
	}
	return s, nil
}
