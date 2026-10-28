package main

import (
	"fmt"
	"os"

	qt "github.com/mappu/miqt/qt6"

	"github.com/nomad-software/screensaver/manager/appicon"
	"github.com/nomad-software/screensaver/manager/desktop"
	"github.com/nomad-software/screensaver/manager/service"
	"github.com/nomad-software/screensaver/manager/settings"
	"github.com/nomad-software/screensaver/manager/ui"
)

func main() {
	// The settings are kept where the system keeps the configuration of
	// programs, or in the current directory if it has no such place.
	settingsPath, err := settings.Path()
	if err != nil {
		fmt.Fprintln(os.Stderr, "screensaver-manager: the settings are kept in the current directory:", err)
		settingsPath = "settings.json"
	}
	manager, err := service.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "screensaver-manager: there is no place for the service:", err)
		os.Exit(1)
	}

	qt.NewQApplication(os.Args)
	qt.QCoreApplication_SetApplicationName("screensaver-manager")
	// Under Wayland this is the name a window goes by, which a desktop
	// matches to the desktop entry of that name for its icon and its place
	// in the task bar.
	qt.QGuiApplication_SetDesktopFileName(desktop.Name)

	// The desktop is given an entry for the program, which starts the copy
	// that is running now, wherever it is and whatever it is called, so that
	// it can be started from the desktop's own menus and kept in a dock or
	// sidebar.
	if err := desktop.Install(appicon.PNG); err != nil {
		fmt.Fprintln(os.Stderr, "screensaver-manager: the desktop entry could not be written:", err)
	}

	window := ui.NewWindow(settingsPath, manager)
	window.Show()

	qt.QApplication_Exec()
}
