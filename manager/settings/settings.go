// Package settings reads and writes the manager's settings: how it looks, and
// what the launcher's service is run with. The launcher itself has no config:
// what is chosen here is given to it as its command line options.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// The themes that can be chosen, besides the system's, which is "".
const (
	ThemeLight = "light"
	ThemeDark  = "dark"
)

// Settings are the manager's settings. Nothing is set until the user sets it.
type Settings struct {
	// Theme is how the program looks: ThemeLight, ThemeDark, or, where it
	// is empty, as the desktop's theme says.
	Theme string `json:"theme,omitempty"`

	// Saver is the name of the saver the launcher shows, empty until one is
	// chosen.
	Saver string `json:"saver,omitempty"`

	// Timer is how long the launcher waits for input before showing the
	// saver, as time.ParseDuration reads it, or empty for the launcher's
	// own.
	Timer string `json:"timer,omitempty"`

	// Inhibitors are the names of the processes which, while any of them
	// runs, keep the launcher from showing the saver.
	Inhibitors []string `json:"inhibitors,omitempty"`
}

// LauncherArgs returns the command line options the launcher is run with:
// the saver, and the timer and processes to inhibit it where they are set.
func (s Settings) LauncherArgs() []string {
	args := []string{"-saver", s.Saver}
	if s.Timer != "" {
		args = append(args, "-timer", s.Timer)
	}
	if len(s.Inhibitors) > 0 {
		args = append(args, "-inhibit", strings.Join(s.Inhibitors, ","))
	}
	return args
}

// Path returns where the settings file is kept:
// screensaver-manager/settings.json in $XDG_CONFIG_HOME, or in ~/.config
// without it.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "screensaver-manager", "settings.json"), nil
}

// Load reads the settings file at path. A file that does not exist or cannot
// be read gives the defaults, and a value that is unusable its default.
func Load(path string) Settings {
	var s Settings
	if data, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(data, &s) != nil {
			s = Settings{}
		}
	}
	if s.Theme != ThemeLight && s.Theme != ThemeDark {
		s.Theme = ""
	}
	return s
}

// Save writes the settings file at path, through a temporary file beside it,
// so that a reader never sees it half written.
func Save(path string, s Settings) error {
	data, err := json.MarshalIndent(s, "", "\t")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
