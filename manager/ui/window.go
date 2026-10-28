// Package ui is the Qt interface: the savers to choose from, the launcher's
// service and the programs which keep the saver from being shown.
package ui

import (
	"os/exec"
	"slices"
	"time"

	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"

	"github.com/nomad-software/screensaver/manager/appicon"
	"github.com/nomad-software/screensaver/manager/service"
	"github.com/nomad-software/screensaver/manager/settings"
	"github.com/nomad-software/screensaver/process"
	"github.com/nomad-software/screensaver/screen"
)

// launcher is the name of the launcher's executable.
const launcher = "screensaver-launcher"

// refreshInterval is how often the service and the programs running are
// looked at again, so that what is shown follows them.
const refreshInterval = 2 * time.Second

// Window is the main window.
type Window struct {
	*qt.QMainWindow

	// settingsPath is where the manager's settings are kept, and settings
	// what they are: the theme, and what the launcher is run with.
	settingsPath string
	settings     settings.Settings

	manager *service.Manager
	status  service.Status

	tabs           *qt.QTabWidget
	settingsAction *qt.QAction
	quit           *qt.QAction
	about          *qt.QAction
	savers         *saverList
	service        *serviceBox
	inhibitors     *inhibitorBox

	// refreshing is whether the state is being looked at, so that a slow
	// systemctl does not have the looks pile up.
	refreshing bool

	// gone is whether the window has been destroyed, with its widgets, which
	// what finishes in the background must then leave alone.
	gone bool
}

// state is what is looked at away from the UI's thread at each refresh.
type state struct {
	status    service.Status
	statusErr error
	running   map[string][]int
}

// NewWindow makes the main window, for the manager's settings at
// settingsPath and the service the manager controls.
func NewWindow(settingsPath string, manager *service.Manager) *Window {
	w := &Window{QMainWindow: qt.NewQMainWindow2(), manager: manager,
		settingsPath: settingsPath, settings: settings.Load(settingsPath)}
	rememberSystemStyle()
	if w.settings.Theme != "" {
		applyTheme(w.settings.Theme)
	}
	w.SetWindowTitle("Screensaver Manager")
	w.OnDestroyed(func() { w.gone = true })
	icon := appIcon()
	defer icon.Delete()
	w.SetWindowIcon(icon)
	// Dialogs take their icon from the application.
	qt.QGuiApplication_SetWindowIcon(icon)
	w.makeMenus()

	w.savers = newSaverList(w, screen.Savers())
	w.service = newServiceBox(w)
	w.inhibitors = newInhibitorBox(w)
	w.makeTabs(
		[tabCount]*qt.QWidget{w.savers.QWidget, w.service.QWidget, w.inhibitors.QWidget},
		[tabCount]string{"Screensavers", "Start Up", "Inhibit"},
	)
	w.SetCentralWidget(w.tabs.QWidget)
	// A status bar along the bottom, with a handle in its corner to drag
	// the window bigger or smaller by.
	w.StatusBar().SetSizeGripEnabled(true)
	w.Resize(1000, 640)

	timer := qt.NewQTimer2(w.QObject)
	timer.OnTimeout(w.refresh)
	timer.Start(int(refreshInterval / time.Millisecond))
	w.refresh()
	return w
}

// makeMenus makes the menu bar: File, with the settings and Quit, and Help,
// with the about box.
func (w *Window) makeMenus() {
	file := w.MenuBar().AddMenuWithTitle("&File")
	w.settingsAction = file.AddActionWithText("&Settings…")
	w.settingsAction.SetMenuRole(qt.QAction__PreferencesRole)
	shortcut := qt.NewQKeySequence2("Ctrl+,")
	w.settingsAction.SetShortcut(shortcut)
	shortcut.Delete()
	w.settingsAction.OnTriggered(func() { w.editSettings() })
	file.AddSeparator()
	w.quit = file.AddActionWithText("&Quit")
	shortcut = qt.NewQKeySequence2("Ctrl+Q")
	w.quit.SetShortcut(shortcut)
	shortcut.Delete()
	w.quit.SetMenuRole(qt.QAction__QuitRole)
	w.quit.OnTriggered(func() { w.Close() })

	help := w.MenuBar().AddMenuWithTitle("&Help")
	w.about = help.AddActionWithText("&About")
	w.about.SetMenuRole(qt.QAction__AboutRole)
	shortcut = qt.NewQKeySequence2("Ctrl+I")
	w.about.SetShortcut(shortcut)
	shortcut.Delete()
	w.about.OnTriggered(func() { showAbout(w.QWidget) })
}

// editSettings shows the settings, which are put into effect and saved if
// they are accepted.
func (w *Window) editSettings() *settingsDialog {
	return newSettingsDialog(w.QWidget, w.settings, w.applySettings)
}

// applySettings puts the settings into effect, and saves them.
func (w *Window) applySettings(s settings.Settings) {
	if s.Theme != w.settings.Theme {
		applyTheme(s.Theme)
		// The icons are drawn in the theme's colour of text at once, rather
		// than when the window hears of the change.
		w.drawTabIcons()
		w.inhibitors.drawBins()
	}
	w.settings = s
	w.save()
}

// appIcon returns the program's icon.
func appIcon() *qt.QIcon {
	pixmap := qt.NewQPixmap()
	// The icon keeps a copy of the picture.
	defer pixmap.Delete()
	pixmap.LoadFromDataWithData(appicon.PNG)
	return qt.NewQIcon2(pixmap)
}

// save writes the settings file, and says so if it cannot.
func (w *Window) save() bool {
	if err := settings.Save(w.settingsPath, w.settings); err != nil {
		warning(w.QWidget, "Settings", "The settings file "+w.settingsPath+" could not be written: "+err.Error())
		return false
	}
	return true
}

// saveAndReconfigure writes the settings file and gives the service the
// launcher's options as they are now.
func (w *Window) saveAndReconfigure() {
	if w.save() {
		w.reconfigure()
	}
}

// reconfigure gives the service, if it is installed, the launcher's options
// as they are now, restarting the launcher with them if it is running.
func (w *Window) reconfigure() {
	if !w.status.Installed || w.settings.Saver == "" {
		return
	}
	path, ok := w.launcherPath()
	if !ok {
		return
	}
	args := w.settings.LauncherArgs()
	w.inBackground(func() error { return w.manager.Reconfigure(path, args) })
}

// launcherPath returns the path of the launcher, found in the PATH, as the
// service needs it: systemd runs only a program given by its path. It says
// so if the launcher is not installed.
func (w *Window) launcherPath() (string, bool) {
	path, err := exec.LookPath(launcher)
	if err != nil {
		warning(w.QWidget, "Start Up", "The launcher, "+launcher+", is not installed: run make install.")
		return "", false
	}
	return path, true
}

// inBackground runs work, which reaches systemctl, away from the UI's thread,
// then warns of its error, if any, and shows the state it leaves.
func (w *Window) inBackground(work func() error) {
	go func() {
		err := work()
		mainthread.Start(func() {
			if w.gone {
				return
			}
			if err != nil {
				warning(w.QWidget, "Start Up", err.Error())
			}
			w.refresh()
		})
	}()
}

// refresh looks at the service and at the programs running, away from the
// UI's thread, and shows them.
func (w *Window) refresh() {
	if w.refreshing {
		return
	}
	w.refreshing = true
	names := append(slices.Clone(w.settings.Inhibitors), launcher)
	go func() {
		var s state
		s.status, s.statusErr = w.manager.Status()
		s.running, _ = process.Running(names)
		mainthread.Start(func() {
			if w.gone {
				return
			}
			w.refreshing = false
			w.status = s.status
			w.service.show(s)
			w.inhibitors.show(s.running)
		})
	}()
}

// startSaver starts a saver by name, found in the PATH where it is
// installed, to preview it, without waiting for it to end. Tests replace it.
var startSaver = func(name string) error {
	cmd := exec.Command(name)
	if err := cmd.Start(); err != nil {
		return err
	}
	// It is waited for, so that it does not linger once it ends.
	go cmd.Wait()
	return nil
}

// warning shows a warning. Tests replace it rather than let the dialog open.
var warning = func(parent *qt.QWidget, title, text string) {
	qt.QMessageBox_Warning(parent, title, text)
}

// confirm asks the user a question, and reports if they said yes. Tests
// replace it rather than let the dialog open.
var confirm = func(parent *qt.QWidget, title, text string) bool {
	return qt.QMessageBox_Question5(parent, title, text, qt.QMessageBox__Yes|qt.QMessageBox__No) == qt.QMessageBox__Yes
}
