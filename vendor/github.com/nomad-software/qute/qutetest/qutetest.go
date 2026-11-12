// Package qutetest runs tests of qute programs on the GUI thread, headlessly,
// and drives widgets as a user would (clicks, typing, key presses) through
// Qt's QtTest module. It is a separate package so that programs, which import
// only qute, do not link QtTest.
//
//	func TestMain(m *testing.M) { qutetest.TestMain(m) }
//
//	func TestButton(t *testing.T) {
//		qutetest.Run(t, func() {
//			btn := qtwidgets.NewQPushButtonWithText("OK", nil)
//			clicked := false
//			btn.OnClicked(func(bool) { clicked = true })
//			qutetest.Click(btn)
//			if !clicked {
//				t.Error("not clicked")
//			}
//		})
//	}
package qutetest

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	"github.com/nomad-software/qute"
	rt "github.com/nomad-software/qute/internal/rt"
	"github.com/nomad-software/qute/qtgui"
)

// TestMain runs a package's tests with a QApplication on the main thread.
// Use it from the package's TestMain:
//
//	func TestMain(m *testing.M) { qutetest.TestMain(m) }
//
// Test functions run on other goroutines, so wrap Qt calls in Run(t, ...).
// QT_QPA_PLATFORM defaults to offscreen, so tests need no display. Closing the
// last window does not quit the application here (as it does under qute.Main),
// so a test may close or delete its windows and later tests still run.
func TestMain(m *testing.M) {
	if os.Getenv("QT_QPA_PLATFORM") == "" {
		os.Setenv("QT_QPA_PLATFORM", "offscreen")
	}
	done := make(chan int, 1)
	// A t.Fatal inside Run ends the GUI goroutine; the runtime keeps serving
	// Qt from a nested event loop, and once that quits the binary exits here.
	rt.AfterRescue = func() { os.Exit(<-done) }
	qute.Main(func() {
		// A test that closes or deletes its last visible window must not quit
		// the application: every later test would find no event loop.
		qtgui.QGuiApplicationSetQuitOnLastWindowClosed(false)
		go func() {
			code := m.Run()
			done <- code
			qute.Quit(code)
		}()
	})
	os.Exit(<-done)
}

// Run executes f on the GUI thread and waits. A panic in f fails t with the
// panic message instead of crashing the test binary. A panic in a signal or
// event handler that f triggers is recovered where Qt called the handler, so
// Qt's own frames return normally; it fails t too, and the input helpers
// (Click, Type, Key, Wait and qute.ProcessEvents) re-raise it as soon as they return.
//
// t.Fatal, t.FailNow and t.Skip work inside f: they end f and then the test,
// as they would on the test's own goroutine. (They end the GUI goroutine
// too, which the runtime replaces with a nested event loop, so prefer
// t.Error and return in code that fails often.)
func Run(t testing.TB, f func()) {
	t.Helper()
	var failure string
	exited := rt.PostSyncExit(func() {
		stop := rt.CapturePanics()
		defer func() {
			cp := stop()
			if p := recover(); p != nil {
				failure = fmt.Sprint(p)
				if _, ok := p.(runtime.Error); ok {
					failure += "\n" + string(debug.Stack())
				}
			}
			if cp != nil {
				if failure != "" {
					failure += "\n"
				}
				failure += cp.String()
			}
		}()
		f()
	})
	if failure != "" {
		t.Fatal(failure)
	}
	if exited {
		// f called t.FailNow or t.SkipNow on the GUI goroutine; end the test
		// on its own goroutine the same way
		if t.Skipped() && !t.Failed() {
			t.SkipNow()
		}
		if !t.Failed() {
			t.Error("qutetest.Run: the function called runtime.Goexit")
		}
		t.FailNow()
	}
}

// CollectGarbage runs the GC until queued cleanups have been handed to the
// GUI thread, then lets the GUI thread free them. Use it in tests to check
// that unreachable objects really are freed. Call it off the GUI thread.
func CollectGarbage() {
	if rt.IsGUIThread() {
		panic("qutetest.CollectGarbage: call it outside Run, so the GUI thread can free objects")
	}
	for i := 0; i < 4; i++ {
		runtime.GC()
		time.Sleep(5 * time.Millisecond) // cleanups run on their own goroutine
		qute.PostSync(func() { rt.Drain() })
	}
}

// Freed returns how many objects GC cleanups have freed so far.
func Freed() int64 { return rt.Drained.Load() }
