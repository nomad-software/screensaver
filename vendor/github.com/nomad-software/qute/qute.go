// Package qute runs Qt applications from Go and provides the tools that make
// them checkable without a human at the screen: headless screenshots and an
// object tree dump. Tests and scripted input are in package qutetest.
//
// A minimal program:
//
//	func main() {
//		qute.Main(func() {
//			w := qtwidgets.NewQWidget(nil)
//			w.SetWindowTitle("Hello")
//			w.Show()
//		})
//	}
//
// All Qt calls must run on the GUI thread, which is the main goroutine inside
// Main. From other goroutines use Post or PostSync.
package qute

import (
	"os"

	rt "github.com/nomad-software/qute/internal/rt"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtwidgets"
)

// Main creates the QApplication, runs setup on the GUI thread, then runs the
// event loop until the application quits. It returns the exit code. Call it
// from main(): the main goroutine is locked to the main OS thread when qute is
// imported, which Qt requires.
//
// Once the loop has exited, queued GC cleanups are dropped rather than run,
// since deleting objects after the application is gone crashes Qt.
func Main(setup func()) int {
	if !rt.IsGUIThread() {
		panic("qute.Main: must be called from the main goroutine (it owns the main OS thread)")
	}
	if !rt.AppExists() {
		qtwidgets.NewQApplication(os.Args)
	}
	rt.AppStarted()
	if setup != nil {
		setup()
	}
	code := qtwidgets.QApplicationExec()
	rt.AppExited()
	return code
}

// Post queues f to run on the GUI thread and returns immediately. It is safe
// to call from any goroutine, and before Main starts (f then runs once the
// event loop is up).
func Post(f func()) { rt.Post(f) }

// PostSync runs f on the GUI thread and waits for it to finish. On the GUI
// thread it calls f directly. A panic in f is re-raised in the caller.
//
// Do not call PostSync while holding a lock the GUI thread may need: it
// blocks until the GUI thread is free.
func PostSync(f func()) { rt.PostSync(f) }

// Quit asks the event loop to exit with code. Safe from any goroutine.
func Quit(code int) {
	Post(func() { qtcore.QCoreApplicationExit(code) })
}

// IsGUIThread reports whether the caller runs on the GUI thread.
func IsGUIThread() bool { return rt.IsGUIThread() }
