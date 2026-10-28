package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mappu/miqt/qt6/mainthread"

	"github.com/nomad-software/screensaver/manager/settings"
)

// screenshotEnv names the file the window is drawn into.
const screenshotEnv = "SCREENSAVER_MANAGER_SCREENSHOT"

// TestScreenshot draws the window, in the dark theme, with the service running
// and a program that stops the saver running, into the file screenshotEnv
// names.
func TestScreenshot(t *testing.T) {
	path := os.Getenv(screenshotEnv)
	if path == "" {
		t.Skip(screenshotEnv + " is not set")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { onMain(func() { applyTheme("") }) })
	var c settings.Settings
	c.Theme = settings.ThemeDark
	c.Saver = "screensaver-digital-rain"
	c.Inhibitors = append(c.Inhibitors, self[strings.LastIndex(self, "/")+1:])
	w, _ := newTestWindow(t, &c, &fakeSystemd{running: true, pid: 2817})
	if err := os.MkdirAll(w.manager.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(w.manager.Path(), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	onMain(func() { w.Show() })
	wait(t, "the service to be shown running", func() bool { return strings.Contains(w.service.state.Text(), "Running") })
	// The previews are a second in, rather than at their first frames.
	time.Sleep(time.Second)
	if !mainthread.Wait2(func() bool { return w.Grab().Save(path) }) {
		t.Fatal("the screenshot could not be saved")
	}
}
