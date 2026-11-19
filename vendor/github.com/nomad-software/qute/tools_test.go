package qute_test

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

func TestProcessEventsRunsPostedWork(t *testing.T) {
	qutetest.Run(t, func() {
		ran := false
		qute.Post(func() { ran = true })
		qute.ProcessEvents()
		if !ran {
			t.Error("work posted to the GUI thread did not run in ProcessEvents")
		}
		if !qute.IsGUIThread() {
			t.Error("IsGUIThread() = false inside qutetest.Run")
		}
	})
}

// SaveScreenshot writes the image Screenshot returns as a PNG, and reports a
// path it cannot write instead of panicking.
func TestSaveScreenshot(t *testing.T) {
	dir := t.TempDir()
	qutetest.Run(t, func() {
		w := qtwidgets.NewQLabelWithText("saved", nil)
		defer w.Delete()
		w.Resize(120, 40)
		path := filepath.Join(dir, "shot.png")
		if err := qute.SaveScreenshot(w, path); err != nil {
			t.Error(err)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			t.Errorf("the saved file is no PNG: %v", err)
			return
		}
		if got, want := img.Bounds(), qute.Screenshot(w).Bounds(); got != want || got.Dx() != 120 || got.Dy() != 40 {
			t.Errorf("saved image is %v, Screenshot gives %v, want 120x40", got, want)
		}
		if err := qute.SaveScreenshot(w, filepath.Join(dir, "missing", "shot.png")); err == nil {
			t.Error("SaveScreenshot into a missing directory returned no error")
		}
	})
}

// Tree reports objects Go never wrapped as untracked, and nil for nil.
func TestTreeUntrackedAndNil(t *testing.T) {
	qutetest.Run(t, func() {
		if qute.Tree(nil) != nil {
			t.Error("Tree(nil) is not nil")
		}
		tabs := qtwidgets.NewQTabWidget(nil) // builds a tab bar and a stack of its own
		defer tabs.Delete()
		n := qute.Tree(tabs)
		if n.Class != "QTabWidget" || n.Ownership != "Owned" || n.Geometry == nil || !n.Window {
			t.Errorf("root node: %+v", n)
		}
		if len(n.Children) == 0 {
			t.Error("a tab widget has no children in its tree")
		}
		for _, c := range n.Children {
			if c.Ownership != "untracked" {
				t.Errorf("%s that Go never wrapped has ownership %q, want untracked", c.Class, c.Ownership)
			}
		}
	})
}

// Keep pins an object until as many Unkeeps; one Unkeep too many does nothing.
func TestKeepUnkeepPins(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		defer w.Delete()
		pins := func() int { return qute.Tree(w).Pins }
		w.Keep()
		w.Keep()
		if pins() != 2 {
			t.Errorf("after two Keeps: %d pins, want 2", pins())
		}
		w.Unkeep()
		w.Unkeep()
		w.Unkeep()
		if pins() != 0 {
			t.Errorf("after three Unkeeps: %d pins, want 0", pins())
		}
		w.Keep()
		if pins() != 1 {
			t.Errorf("Keep after an extra Unkeep: %d pins, want 1", pins())
		}
	})
}

// A kept object survives the GC with no Go reference, and is freed once
// Unkeep lets it go.
func TestKeepHoldsThroughGC(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		w.SetObjectName("kept-through-gc")
		w.Keep()
	})
	find := func() (found *qtwidgets.QWidget) {
		for _, w := range qtwidgets.QApplicationTopLevelWidgets() {
			if w.ObjectName() == "kept-through-gc" {
				found = w
			}
		}
		return found
	}
	qutetest.CollectGarbage()
	before := qutetest.Freed()
	ok := false
	qutetest.Run(t, func() {
		if w := find(); w != nil {
			ok = true
			w.Unkeep()
		}
	})
	if !ok {
		t.Fatal("a kept widget nothing referred to was freed by GC")
	}
	qutetest.CollectGarbage()
	if qutetest.Freed() == before {
		t.Error("the widget was not freed after Unkeep")
	}
	qutetest.Run(t, func() {
		if find() != nil {
			t.Error("the widget is still a top-level widget after Unkeep and GC")
		}
	})
}
