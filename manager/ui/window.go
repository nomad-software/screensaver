// Package ui is the Qt interface: the savers to choose from, the launcher's
// service and the programs which keep the saver from being shown.
package ui

import (
	"os/exec"
	"slices"
	"time"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

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

// Window is the main window. It is a Go subclass of QMainWindow, so that it
// hears of changes to the palette and the style (see ChangeEvent).
type Window struct {
	qtwidgets.QMainWindowBase

	// settingsPath is where the manager's settings are kept, and settings
	// what they are: the theme, and what the launcher is run with.
	settingsPath string
	settings     settings.Settings

	manager *service.Manager
	status  service.Status

	tabs           *tabWidget
	settingsAction *qtgui.QAction
	quit           *qtgui.QAction
	about          *qtgui.QAction
	savers         *saverList
	service        *serviceBox
	inhibitors     *inhibitorBox

	// refreshing is whether the state is being looked at, so that a slow
	// systemctl does not have the looks pile up.
	refreshing bool
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
	w := &Window{manager: manager, settingsPath: settingsPath, settings: settings.Load(settingsPath)}
	qtwidgets.NewQMainWindowWith(w, nil)
	rememberSystemStyle()
	if w.settings.Theme != "" {
		applyTheme(w.settings.Theme)
	}
	w.SetWindowTitle("Screensaver Manager")
	icon := appIcon()
	defer icon.Delete()
	w.SetWindowIcon(icon)
	// Dialogs take their icon from the application.
	qtgui.QGuiApplicationSetWindowIcon(icon)
	w.makeMenus()

	w.savers = newSaverList(w, screen.Savers())
	w.service = newServiceBox(w)
	w.inhibitors = newInhibitorBox(w)
	w.makeTabs(
		[tabCount]*qtwidgets.QWidget{w.savers.QWidget, w.service.QWidget, w.inhibitors.QWidget},
		[tabCount]string{"Screensavers", "Start Up", "Inhibit"},
	)
	w.SetCentralWidget(w.tabs)
	// A status bar along the bottom, with a handle in its corner to drag
	// the window bigger or smaller by.
	w.StatusBar().SetSizeGripEnabled(true)
	w.Resize(1000, 640)

	timer := qtcore.NewQTimer(w)
	timer.OnTimeout(w.refresh)
	timer.StartWithDuration(refreshInterval)
	w.refresh()
	return w
}

// makeMenus makes the menu bar: File, with the settings and Quit, and Help,
// with the about box.
func (w *Window) makeMenus() {
	file := w.MenuBar().AddMenuTitle("&File")
	w.settingsAction = file.AddActionText("&Settings…")
	w.settingsAction.SetMenuRole(qtgui.QActionPreferencesRole)
	setShortcut(w.settingsAction, "Ctrl+,")
	w.settingsAction.OnTriggered(func(bool) { w.editSettings() })
	file.AddSeparator()
	w.quit = file.AddActionText("&Quit")
	setShortcut(w.quit, "Ctrl+Q")
	w.quit.SetMenuRole(qtgui.QActionQuitRole)
	w.quit.OnTriggered(func(bool) { w.Close() })

	help := w.MenuBar().AddMenuTitle("&Help")
	w.about = help.AddActionText("&About")
	w.about.SetMenuRole(qtgui.QActionAboutRole)
	setShortcut(w.about, "Ctrl+I")
	w.about.OnTriggered(func(bool) { showAbout(&w.QWidget) })
}

// setShortcut gives the action the shortcut of the keys, such as "Ctrl+Q".
func setShortcut(action *qtgui.QAction, keys string) {
	shortcut := qtgui.NewQKeySequenceWithKey(keys)
	// The action keeps a copy of the shortcut.
	defer shortcut.Delete()
	action.SetShortcut(shortcut)
}

// editSettings shows the settings, which are put into effect and saved if
// they are accepted.
func (w *Window) editSettings() *settingsDialog {
	return newSettingsDialog(&w.QWidget, w.settings, w.applySettings)
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
func appIcon() *qtgui.QIcon {
	pixmap := qtgui.NewQPixmap()
	// The icon keeps a copy of the picture.
	defer pixmap.Delete()
	pixmap.LoadFromData(appicon.PNG)
	return qtgui.NewQIconWithPixmap(pixmap)
}

// save writes the settings file, and says so if it cannot.
func (w *Window) save() bool {
	if err := settings.Save(w.settingsPath, w.settings); err != nil {
		warning(&w.QWidget, "Settings", "The settings file "+w.settingsPath+" could not be written: "+err.Error())
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
		warning(&w.QWidget, "Start Up", "The launcher, "+launcher+", is not installed: run make install.")
		return "", false
	}
	return path, true
}

// inBackground runs work, which reaches systemctl, away from the UI's thread,
// then warns of its error, if any, and shows the state it leaves.
func (w *Window) inBackground(work func() error) {
	go func() {
		err := work()
		qute.Post(func() {
			// The window may have been destroyed, with its widgets, while
			// the work was done.
			if !w.Alive() {
				return
			}
			if err != nil {
				warning(&w.QWidget, "Start Up", err.Error())
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
		qute.Post(func() {
			if !w.Alive() {
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
var warning = func(parent *qtwidgets.QWidget, title, text string) {
	qtwidgets.QMessageBoxWarning(parent, title, text)
}

// confirm asks the user a question, and reports if they said yes. Tests
// replace it rather than let the dialog open.
var confirm = func(parent *qtwidgets.QWidget, title, text string) bool {
	return qtwidgets.QMessageBoxQuestionFull(parent, title, text,
		qtwidgets.QMessageBoxYes|qtwidgets.QMessageBoxNo, qtwidgets.QMessageBoxNoButton) == qtwidgets.QMessageBoxYes
}
