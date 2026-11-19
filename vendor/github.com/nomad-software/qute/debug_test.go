//go:build qute_debug

package qute_test

import (
	"strings"
	"testing"

	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// Debug builds check the thread on every call.
func TestDebugWrongThreadPanics(t *testing.T) {
	var lbl *qtwidgets.QLabel
	qutetest.Run(t, func() { lbl = qtwidgets.NewQLabelWithText("x", nil) })
	defer qutetest.Run(t, func() { lbl.Delete() })
	defer func() {
		p := recover()
		if p == nil || !strings.Contains(p.(string), "off the GUI thread") {
			t.Errorf("want thread panic, got %v", p)
		}
	}()
	lbl.SetText("from a test goroutine")
}

// Debug builds record where an object died.
func TestDebugDeathSite(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		lbl := qtwidgets.NewQLabelWithText("x", w)
		w.Delete()
		defer func() {
			msg, _ := recover().(string)
			if !strings.Contains(msg, "debug_test.go:") {
				t.Errorf("want death site in message, got %q", msg)
			}
		}()
		lbl.Text()
	})
}
