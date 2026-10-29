package ui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"

	"github.com/nomad-software/screensaver/manager/service"
	"github.com/nomad-software/screensaver/manager/settings"
)

// warned keeps the warnings given, to be checked by the tests.
var warned struct {
	sync.Mutex
	list []string
}

func TestMain(m *testing.M) {
	os.Setenv("QT_QPA_PLATFORM", "offscreen")
	qt.NewQApplication(os.Args[:1])
	qt.QGuiApplication_SetQuitOnLastWindowClosed(false)
	warning = func(_ *qt.QWidget, title, text string) {
		warned.Lock()
		defer warned.Unlock()
		warned.list = append(warned.list, title+": "+text)
	}

	code := 0
	go func() {
		code = m.Run()
		mainthread.Start(qt.QCoreApplication_Quit)
	}()
	qt.QApplication_Exec()
	os.Exit(code)
}

// onMain runs f on the UI's thread and waits for it.
func onMain(f func()) {
	mainthread.Wait(f)
}

// wait waits, with a deadline, until cond, run on the UI's thread, is true.
func wait(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var ok bool
		onMain(func() { ok = cond() })
		if ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	warned.Lock()
	defer warned.Unlock()
	t.Fatalf("timed out waiting for %s; warnings: %q", what, warned.list)
}

// fakeSystemd stands for systemctl, with a service which is installed or not
// and running or not, and records the commands it is given.
type fakeSystemd struct {
	sync.Mutex
	running bool
	pid     int
	ran     []string
}

func (f *fakeSystemd) systemctl(args ...string) (string, error) {
	f.Lock()
	defer f.Unlock()
	if args[0] == "show" {
		state, pid := "inactive", 0
		if f.running {
			state, pid = "active", f.pid
		}
		return "ActiveState=" + state + "\nSubState=running\nUnitFileState=enabled\nMainPID=" + strconv.Itoa(pid) + "\n", nil
	}
	f.ran = append(f.ran, strings.Join(args, " "))
	switch args[0] {
	case "restart", "start":
		f.running = true
	case "stop", "disable":
		f.running = false
	}
	return "", nil
}

func (f *fakeSystemd) commands() []string {
	f.Lock()
	defer f.Unlock()
	return append([]string(nil), f.ran...)
}

// newTestWindow makes a window for a settings file and service directory of
// its own, with the settings given written first if they are not nil. It
// returns the path of the settings file.
func newTestWindow(t *testing.T, s *settings.Settings, sd *fakeSystemd) (*Window, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "screensaver-manager", "settings.json")
	if s != nil {
		if err := settings.Save(path, *s); err != nil {
			t.Fatal(err)
		}
	}
	m := &service.Manager{Dir: filepath.Join(dir, "systemd"), Systemctl: sd.systemctl}
	warned.Lock()
	warned.list = nil
	warned.Unlock()
	var w *Window
	onMain(func() { w = NewWindow(path, m) })
	t.Cleanup(func() { onMain(func() { w.DeleteLater() }) })
	return w, path
}

// withSaver returns settings with only a saver chosen.
func withSaver() *settings.Settings {
	return &settings.Settings{Saver: "screensaver-digital-rain"}
}

// load reads the settings file.
func load(t *testing.T, path string) settings.Settings {
	t.Helper()
	return settings.Load(path)
}

// withLauncher puts a launcher in a PATH of its own, and returns its path.
func withLauncher(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	path := filepath.Join(bin, launcher)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return path
}

// installed writes an empty unit file, as if the service were installed.
func installed(t *testing.T, w *Window) {
	t.Helper()
	if err := os.MkdirAll(w.manager.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(w.manager.Path(), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// unitExec waits for the unit file to have an ExecStart, and returns it.
func unitExec(t *testing.T, w *Window, want string) {
	t.Helper()
	wait(t, "the unit file to run "+want, func() bool {
		unit, _ := os.ReadFile(w.manager.Path())
		return strings.Contains(string(unit), "ExecStart="+want+"\n")
	})
}

func TestSaversShown(t *testing.T) {
	var c settings.Settings
	c.Saver = "screensaver-the-thing"
	w, _ := newTestWindow(t, &c, &fakeSystemd{})
	onMain(func() {
		g := w.savers
		if g.list.Count() == 0 {
			t.Fatal("no savers")
		}
		if g.selected < 0 || g.savers[g.selected].Name != "screensaver-the-thing" {
			t.Fatalf("selected %d, want the saver configured", g.selected)
		}
		if !strings.HasPrefix(g.list.Item(g.selected).Text(), "✔ ") {
			t.Errorf("the saver configured is not marked: %q", g.list.Item(g.selected).Text())
		}
		// The saver in use cannot be used again.
		if g.use.IsEnabled() || !g.preview.IsEnabled() {
			t.Error("Use is enabled, or Preview disabled, for the saver in use")
		}
		if g.movie == nil || !g.movie.IsValid() {
			t.Fatal("the preview of the saver selected cannot be played")
		}
		// Selecting another plays its preview instead.
		before := g.movie
		g.list.SetCurrentRow(0)
		if g.movie == nil || g.movie == before || !g.movie.IsValid() {
			t.Fatal("the preview of the saver selected next is not played")
		}
	})
	wait(t, "a preview to have played", func() bool {
		return w.savers.movie.CurrentFrameNumber() > 0
	})
}

func TestNoSaverConfigured(t *testing.T) {
	var c settings.Settings
	c.Saver = "screensaver-not-in-the-list"
	w, _ := newTestWindow(t, &c, &fakeSystemd{})
	onMain(func() {
		g := w.savers
		if g.selected >= 0 || g.movie != nil || g.preview.IsEnabled() || g.use.IsEnabled() {
			t.Fatalf("selected %d with nothing configured in the list", g.selected)
		}
	})
}

func TestUseSaverRestartsLauncher(t *testing.T) {
	sd := &fakeSystemd{running: true, pid: 1}
	exe := withLauncher(t)
	w, path := newTestWindow(t, nil, sd)
	installed(t, w)
	wait(t, "the service to be seen installed and running", func() bool { return w.status.Installed && w.status.Running() })
	var name string
	onMain(func() {
		g := w.savers
		for i, s := range g.savers {
			if s.Name != w.settings.Saver {
				g.list.SetCurrentRow(i)
				name = s.Name
				break
			}
		}
		if !g.use.IsEnabled() {
			t.Fatal("Use is not enabled for another saver")
		}
		g.use.Click()
	})
	if got := load(t, path).Saver; got != name {
		t.Fatalf("the settings have saver %q, want %q", got, name)
	}
	// The service is given the saver as an option, and restarted with it.
	unitExec(t, w, exe+" -saver "+name)
	wait(t, "the launcher to be restarted", func() bool {
		return reflect.DeepEqual(sd.commands(), []string{"daemon-reload", "try-restart " + service.Name})
	})
}

func TestUseSaverNotRunning(t *testing.T) {
	sd := &fakeSystemd{}
	w, path := newTestWindow(t, nil, sd)
	wait(t, "a refresh", func() bool { return !w.refreshing })
	onMain(func() {
		w.savers.list.SetCurrentRow(0)
		w.savers.useSelected()
	})
	if got := load(t, path).Saver; got != w.savers.savers[0].Name {
		t.Fatalf("the settings have saver %q", got)
	}
	time.Sleep(100 * time.Millisecond)
	if got := sd.commands(); len(got) != 0 {
		t.Fatalf("ran %q for a service not installed", got)
	}
}

func TestPreview(t *testing.T) {
	var started []string
	startSaver = func(name string) error {
		started = append(started, name)
		return nil
	}
	w, _ := newTestWindow(t, nil, &fakeSystemd{})
	onMain(func() {
		w.savers.list.SetCurrentRow(1)
		w.savers.preview.Click()
	})
	want := []string{w.savers.savers[1].Name}
	if !reflect.DeepEqual(started, want) {
		t.Fatalf("started %q, want %q", started, want)
	}

	startSaver = func(string) error { return errors.New("no display") }
	onMain(func() { w.savers.preview.Click() })
	warned.Lock()
	defer warned.Unlock()
	if len(warned.list) != 1 || !strings.Contains(warned.list[0], "no display") {
		t.Fatalf("warned %q", warned.list)
	}
}

func TestInhibitors(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	me := filepath.Base(self)
	var c settings.Settings
	c.Inhibitors = []string{"no-such-program"}
	w, path := newTestWindow(t, &c, &fakeSystemd{})
	b := w.inhibitors
	onMain(func() {
		if b.add.IsEnabled() {
			t.Error("Add is enabled with no name")
		}
		b.name.SetText("  " + me + " ")
		b.add.Click()
	})
	if got := load(t, path).Inhibitors; !reflect.DeepEqual(got, []string{"no-such-program", me}) {
		t.Fatalf("the settings have %q", got)
	}
	// This test's own process is running, so it is shown so.
	wait(t, "the test to be seen running", func() bool {
		return strings.Contains(b.list.Item(1).Text(), "running") &&
			!strings.Contains(b.list.Item(0).Text(), "running") &&
			strings.Contains(b.summary.Text(), "will not be shown")
	})

	onMain(func() {
		// A name listed already is not added again.
		b.name.SetText(me)
		if b.add.IsEnabled() {
			t.Error("Add is enabled for a name listed")
		}
		if len(b.bins) != 2 || b.bins[1].ToolTip() != "Remove "+me {
			t.Fatalf("the rows have no buttons to remove them")
		}
		b.bins[1].Click()
	})
	if got := load(t, path).Inhibitors; !reflect.DeepEqual(got, []string{"no-such-program"}) {
		t.Fatalf("the settings have %q", got)
	}
	wait(t, "nothing to be seen running", func() bool {
		return b.list.Count() == 1 && strings.Contains(b.summary.Text(), "will be shown")
	})
}

func TestInstall(t *testing.T) {
	// The launcher is found in the PATH.
	exe := withLauncher(t)
	sd := &fakeSystemd{pid: 7}
	s := withSaver()
	s.Timer = "5m"
	s.Inhibitors = []string{"vlc", "mpv"}
	w, _ := newTestWindow(t, s, sd)
	onMain(func() {
		if w.service.uninstall.IsEnabled() || w.service.stop.IsEnabled() {
			t.Error("Uninstall or Stop is enabled with no service")
		}
		w.service.install.Click()
	})
	// The launcher is given what has been chosen as its options, away from
	// the UI's thread.
	unitExec(t, w, exe+" -saver screensaver-digital-rain -timer 5m -inhibit vlc,mpv")
	wait(t, "the service to be shown running", func() bool {
		return strings.Contains(w.service.state.Text(), "Running (process 7)") && w.service.stop.IsEnabled()
	})
}

func TestInstallNoSaver(t *testing.T) {
	withLauncher(t)
	w, path := newTestWindow(t, nil, &fakeSystemd{})
	onMain(func() { w.service.install.Click() })
	if _, err := os.Stat(w.manager.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a service was written with no saver chosen: %v", err)
	}
	// Nothing is written for the user until they choose a saver.
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the settings file was written: %v", err)
	}
	warned.Lock()
	defer warned.Unlock()
	if len(warned.list) != 1 || !strings.Contains(warned.list[0], "Choose a saver") {
		t.Fatalf("warned %q", warned.list)
	}
}

func TestNoDefaultSaver(t *testing.T) {
	w, _ := newTestWindow(t, nil, &fakeSystemd{})
	onMain(func() {
		g := w.savers
		if w.settings.Saver != "" || g.selected >= 0 || g.movie != nil {
			t.Fatalf("saver %q selected %d with none chosen", w.settings.Saver, g.selected)
		}
		for i := range g.list.Count() {
			if strings.HasPrefix(g.list.Item(i).Text(), "✔") {
				t.Errorf("%q is marked with none chosen", g.list.Item(i).Text())
			}
		}
	})
}

func TestInstallNoLauncher(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	w, _ := newTestWindow(t, withSaver(), &fakeSystemd{})
	onMain(func() { w.service.install.Click() })
	if _, err := os.Stat(w.manager.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a service was written with no launcher: %v", err)
	}
	warned.Lock()
	defer warned.Unlock()
	if len(warned.list) != 1 || !strings.Contains(warned.list[0], "make install") {
		t.Fatalf("warned %q", warned.list)
	}
}

func TestUninstallDeclined(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, launcher), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	sd := &fakeSystemd{}
	w, _ := newTestWindow(t, withSaver(), sd)
	onMain(func() { w.service.install.Click() })
	wait(t, "the service to be installed", func() bool { return w.service.uninstall.IsEnabled() })
	asked := false
	confirm = func(*qt.QWidget, string, string) bool { asked = true; return false }
	onMain(func() { w.service.uninstall.Click() })
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(w.manager.Path()); !asked || err != nil {
		t.Fatalf("asked %v; unit file: %v", asked, err)
	}
	confirm = func(*qt.QWidget, string, string) bool { return true }
	onMain(func() { w.service.uninstall.Click() })
	wait(t, "the service to be removed", func() bool {
		_, err := os.Stat(w.manager.Path())
		return errors.Is(err, os.ErrNotExist) && strings.Contains(w.service.state.Text(), "Not installed")
	})
}

func TestStrayLauncher(t *testing.T) {
	w, _ := newTestWindow(t, nil, &fakeSystemd{running: true, pid: 5})
	onMain(func() {
		w.service.show(state{
			status:  service.Status{Installed: true, State: "active", PID: 5},
			running: map[string][]int{launcher: {9, 5}},
		})
		if w.service.stray.IsHidden() || !strings.Contains(w.service.stray.Text(), "(process 9)") {
			t.Fatalf("stray launcher not shown: %q", w.service.stray.Text())
		}
		w.service.show(state{
			status:  service.Status{Installed: true, State: "active", PID: 5},
			running: map[string][]int{launcher: {5}},
		})
		if !w.service.stray.IsHidden() {
			t.Fatal("the service's own launcher is shown as a stray")
		}
	})
}

func TestTimer(t *testing.T) {
	w, path := newTestWindow(t, nil, &fakeSystemd{})
	onMain(func() {
		if w.service.timer.Value() != 15 {
			t.Fatalf("timer shows %d", w.service.timer.Value())
		}
		w.service.timer.SetValue(16)
		w.service.timer.SetValue(20)
	})
	deadline := time.Now().Add(5 * time.Second)
	for load(t, path).Timer != "20m" {
		if time.Now().After(deadline) {
			t.Fatalf("the settings have timer %q", load(t, path).Timer)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestMinutes(t *testing.T) {
	for timer, want := range map[string]int{"15m": 15, "1h": 60, "90s": 2, "1s": 1, "soon": 15} {
		if got := minutes(timer); got != want {
			t.Errorf("minutes(%q) = %d, want %d", timer, got, want)
		}
	}
}

func TestTabsPadded(t *testing.T) {
	w, _ := newTestWindow(t, nil, &fakeSystemd{})
	onMain(func() {
		tabs := w.tabs
		plain := qt.NewQTabWidget2()
		defer plain.Delete()
		for _, title := range []string{"Screensavers", "Start Up", "Inhibit"} {
			plain.AddTab(qt.NewQWidget2(), title)
		}
		plain.SetDocumentMode(true)
		// The tabs are laid out once shown.
		w.Show()
		plain.Resize(tabs.Width(), tabs.Height())
		plain.Show()
		bar := tabs.TabBar()
		for i := range tabs.Count() {
			got, natural := bar.TabRect(i), plain.TabBar().TabRect(i)
			if got.Width() < natural.Width()+tabPadX || got.Height() < natural.Height()+tabPadY {
				t.Errorf("tab %d is %dx%d, want at least %dx%d", i, got.Width(), got.Height(), natural.Width()+tabPadX, natural.Height()+tabPadY)
			}
			margins := tabs.Widget(i).Layout().ContentsMargins()
			if margins.Left() != paneMargin || margins.Top() != paneMargin || margins.Right() != paneMargin || margins.Bottom() != paneMargin {
				t.Errorf("pane %d has margins %d, %d, %d, %d, want %d all round", i, margins.Left(), margins.Top(), margins.Right(), margins.Bottom(), paneMargin)
			}
		}
	})
}

func TestMenus(t *testing.T) {
	w, _ := newTestWindow(t, nil, &fakeSystemd{})
	onMain(func() {
		var titles []string
		for _, menu := range w.MenuBar().Actions() {
			titles = append(titles, menu.Text())
		}
		if !reflect.DeepEqual(titles, []string{"&File", "&Help"}) {
			t.Errorf("the menus are %q", titles)
		}
		for action, want := range map[*qt.QAction]string{w.quit: "Ctrl+Q", w.about: "Ctrl+I"} {
			key := action.Shortcut()
			if got := key.ToString(); got != want {
				t.Errorf("%s has shortcut %q, want %q", action.Text(), got, want)
			}
		}
		icon := w.WindowIcon()
		if icon.IsNull() {
			t.Error("the window has no icon")
		}
	})
}

func TestAbout(t *testing.T) {
	w, _ := newTestWindow(t, nil, &fakeSystemd{})
	onMain(func() {
		d := showAbout(w.QWidget)
		if !d.IsVisible() || d.WindowTitle() != "About Screensaver Manager" {
			t.Errorf("the about box is not shown: %q", d.WindowTitle())
		}
		if !strings.Contains(licenceText(), qtVersion()) {
			t.Error("the licences do not say which Qt is used")
		}
		d.Accept()
	})
}

func TestTabIcons(t *testing.T) {
	w, _ := newTestWindow(t, nil, &fakeSystemd{})
	onMain(func() {
		for i := range tabCount {
			icon := w.tabs.TabIcon(i)
			if icon.IsNull() {
				t.Errorf("tab %d has no icon", i)
			}
		}
		// The shortcuts choose the tabs.
		w.tabs.SetCurrentIndex(tabInhibitors)
		w.tabs.SetCurrentIndex(tabSavers)
		if w.tabs.CurrentIndex() != tabSavers {
			t.Error("the tab was not chosen")
		}
	})
}

func TestStatusBar(t *testing.T) {
	w, _ := newTestWindow(t, nil, &fakeSystemd{})
	onMain(func() {
		if !w.StatusBar().IsSizeGripEnabled() {
			t.Error("the status bar has no handle to drag")
		}
	})
}

func TestPreviewKeepsSize(t *testing.T) {
	w, _ := newTestWindow(t, withSaver(), &fakeSystemd{})
	onMain(func() {
		g := w.savers
		w.Show()
		// The smallest is smaller than the window can be made.
		for _, size := range [][2]int{{1400, 900}, {1000, 640}, {200, 200}} {
			w.Resize(size[0], size[1])
			qt.QCoreApplication_ProcessEvents()
			scaled := g.movie.ScaledSize()
			inside := g.screen.ContentsRect()
			if inside.Width() != previewWidth || inside.Height() != previewHeight ||
				scaled.Width() != previewWidth || scaled.Height() != previewHeight {
				t.Errorf("in a window of %v the preview is %dx%d, playing at %dx%d", size,
					inside.Width(), inside.Height(), scaled.Width(), scaled.Height())
			}
			if g.screen.FrameWidth() == 0 {
				t.Errorf("in a window of %v the preview has no frame", size)
			}
			if g.list.Width() < listMinWidth {
				t.Errorf("in a window of %v the list is %d wide, want at least %d", size, g.list.Width(), listMinWidth)
			}
			// The buttons are at the bottom, level with the bottom of the
			// list.
			if bottom, listBottom := g.preview.Y()+g.preview.Height(), g.list.Y()+g.list.Height(); bottom != listBottom {
				t.Errorf("in a window of %v the buttons end at %d, the list at %d", size, bottom, listBottom)
			}
		}
	})
}

func TestApplyTheme(t *testing.T) {
	t.Cleanup(func() { onMain(func() { applyTheme("") }) })
	onMain(func() {
		window := func() string {
			// The palette is a copy, which its finalizer frees.
			p := qt.QGuiApplication_Palette()
			defer runtime.KeepAlive(p)
			return p.ColorWithCr(qt.QPalette__Window).NameWithFormat(qt.QColor__HexRgb)
		}
		rememberSystemStyle()
		applyTheme(settings.ThemeLight)
		if c := window(); c != "#efefef" {
			t.Errorf("light window = %s", c)
		}
		if s := qt.QApplication_Style().Name(); s != "fusion" {
			t.Errorf("light style = %s", s)
		}
		applyTheme(settings.ThemeDark)
		if c := window(); c != "#2d2d2d" {
			t.Errorf("dark window = %s", c)
		}
		applyTheme("")
		if s := qt.QApplication_Style().Name(); s != systemStyle {
			t.Errorf("system style = %s, want %s", s, systemStyle)
		}
	})
}

func TestSettingsTheme(t *testing.T) {
	t.Cleanup(func() { onMain(func() { applyTheme("") }) })
	w, _ := newTestWindow(t, nil, &fakeSystemd{})
	onMain(func() {
		key := w.settingsAction.Shortcut()
		if got := key.ToString(); got != "Ctrl+," {
			t.Errorf("Settings has shortcut %q", got)
		}
		lightness := func() int {
			icon := w.tabs.TabIcon(tabSavers)
			image := icon.Pixmap2(tabIconSize, tabIconSize)
			defer runtime.KeepAlive(image)
			img := image.ToImage()
			defer runtime.KeepAlive(img)
			most := 0
			for y := range img.Height() {
				for x := range img.Width() {
					c := img.PixelColor(x, y)
					if c.Alpha() == 0xff {
						most = max(most, c.Lightness())
					}
					runtime.KeepAlive(c)
				}
			}
			return most
		}

		// Choosing the dark theme puts it into effect, with the tabs'
		// icons drawn light, and saves it.
		d := w.editSettings()
		if d.Width() < settingsWidth || d.Height() < settingsHeight {
			t.Errorf("the settings open at %dx%d, want at least %dx%d", d.Width(), d.Height(), settingsWidth, settingsHeight)
		}
		if d.tabs.Count() != 1 || d.tabs.TabText(0) != "User interface" {
			t.Errorf("the settings have tabs %d, the first %q", d.tabs.Count(), d.tabs.TabText(0))
		}
		selectData(d.theme, settings.ThemeDark)
		d.box.Button(qt.QDialogButtonBox__Ok).Click()
		if s := qt.QApplication_Style().Name(); s != "fusion" {
			t.Errorf("the style is %s after choosing the dark theme", s)
		}
		if l := lightness(); l < 128 {
			t.Errorf("the tab icons are %d light in the dark theme", l)
		}
		if s := settings.Load(w.settingsPath); s.Theme != settings.ThemeDark {
			t.Errorf("the settings file has theme %q", s.Theme)
		}

		// Cancelling changes nothing.
		d = w.editSettings()
		if currentData(d.theme) != settings.ThemeDark {
			t.Error("the dialog does not show the theme chosen")
		}
		selectData(d.theme, settings.ThemeLight)
		d.box.Button(qt.QDialogButtonBox__Cancel).Click()
		if w.settings.Theme != settings.ThemeDark {
			t.Error("cancelling changed the theme")
		}

		d = w.editSettings()
		selectData(d.theme, settings.ThemeLight)
		d.box.Button(qt.QDialogButtonBox__Ok).Click()
		if l := lightness(); l > 128 {
			t.Errorf("the tab icons are %d light in the light theme", l)
		}
	})
}

func TestStartThemed(t *testing.T) {
	t.Cleanup(func() { onMain(func() { applyTheme("") }) })
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := settings.Save(path, settings.Settings{Theme: settings.ThemeDark}); err != nil {
		t.Fatal(err)
	}
	m := &service.Manager{Dir: dir, Systemctl: (&fakeSystemd{}).systemctl}
	var w *Window
	onMain(func() { w = NewWindow(path, m) })
	t.Cleanup(func() { onMain(func() { w.DeleteLater() }) })
	onMain(func() {
		if s := qt.QApplication_Style().Name(); s != "fusion" {
			t.Errorf("the style is %s, want the dark theme's", s)
		}
	})
}

func TestInhibitorsReconfigure(t *testing.T) {
	exe := withLauncher(t)
	sd := &fakeSystemd{running: true, pid: 3}
	w, _ := newTestWindow(t, withSaver(), sd)
	installed(t, w)
	wait(t, "the service to be seen installed", func() bool { return w.status.Installed })
	onMain(func() {
		// A name with a comma or space cannot be given to the launcher.
		for _, name := range []string{"a,b", "a b"} {
			w.inhibitors.name.SetText(name)
			if w.inhibitors.add.IsEnabled() {
				t.Errorf("Add is enabled for %q", name)
			}
		}
		w.inhibitors.name.SetText("kodi")
		w.inhibitors.add.Click()
	})
	unitExec(t, w, exe+" -saver screensaver-digital-rain -inhibit kodi")
	onMain(func() { w.inhibitors.bins[0].Click() })
	unitExec(t, w, exe+" -saver screensaver-digital-rain")
}
