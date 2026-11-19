package qutetest_test

import (
	"testing"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// Closing the last visible window quits a Qt application by default. Under
// TestMain it must not: the event loop would stop, and every later test
// would panic in Run with "the application is not running".
func TestCloseLastWindowKeepsApplication(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		w.Show()
		qute.ProcessEvents()
		w.Close()
		qute.ProcessEvents()
		w.Delete()
	})
	// the quit, if any, is posted: let the event loop see it
	qutetest.Run(t, func() { qute.ProcessEvents() })
	ran := false
	qute.PostSync(func() { ran = true })
	if !ran {
		t.Error("the event loop stopped after the last window closed")
	}
}
