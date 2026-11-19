package qute_test

import (
	"strings"
	"testing"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

func TestMain(m *testing.M) { qutetest.TestMain(m) }

func TestLifecycle(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		lay := qtwidgets.NewQVBoxLayout(w)
		lbl := qtwidgets.NewQLabelWithText("hi", nil)
		if got := w.Ownership(); got != qtcore.Owned {
			t.Errorf("w: %v, want Owned", got)
		}
		if got := lay.Ownership(); got != qtcore.Parented {
			t.Errorf("lay: %v, want Parented", got)
		}
		if got := lbl.Ownership(); got != qtcore.Owned {
			t.Errorf("lbl before AddWidget: %v, want Owned", got)
		}
		lay.AddWidget(lbl)
		if got := lbl.Ownership(); got != qtcore.Parented {
			t.Errorf("lbl after AddWidget: %v, want Parented", got)
		}
		if lbl.ParentWidget() != w {
			t.Errorf("lbl.ParentWidget() is not w (one object, one handle)")
		}
		w.Delete()
		if lbl.Alive() || lay.Alive() || w.Alive() {
			t.Errorf("children alive after parent Delete: lbl=%v lay=%v w=%v", lbl.Alive(), lay.Alive(), w.Alive())
		}
		w.Delete() // idempotent
	})
}

func TestDeadHandlePanics(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		lbl := qtwidgets.NewQLabelWithText("x", w)
		w.Delete()
		defer func() {
			p := recover()
			if p == nil {
				t.Fatal("no panic on dead handle")
			}
			if msg := p.(string); !strings.Contains(msg, "QLabel.SetText: object destroyed") {
				t.Errorf("unexpected panic: %s", msg)
			}
		}()
		lbl.SetText("boom")
	})
}

func TestBorrowedDeletePanics(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		defer w.Delete()
		qtwidgets.NewQLabelWithText("x", w)
		kids := w.Children()
		if len(kids) == 0 {
			t.Fatal("no children")
		}
		w.SetFocus()
		defer func() {
			if recover() == nil {
				t.Error("Delete on Borrowed did not panic")
			}
		}()
		app := qtcore.QCoreApplicationInstance()
		app.Delete()
	})
}

func TestSignal(t *testing.T) {
	qutetest.Run(t, func() {
		btn := qtwidgets.NewQPushButtonWithText("press", nil)
		defer btn.Delete()
		clicks := 0
		conn := btn.OnClicked(func(checked bool) { clicks++ })
		qutetest.Click(btn)
		qutetest.Click(btn)
		if clicks != 2 {
			t.Errorf("clicks = %d, want 2", clicks)
		}
		conn.Disconnect()
		qutetest.Click(btn)
		if clicks != 2 {
			t.Errorf("clicks after Disconnect = %d, want 2", clicks)
		}
		conn.Disconnect()
	})
}

func TestTypeAndText(t *testing.T) {
	qutetest.Run(t, func() {
		e := qtwidgets.NewQLineEdit(nil)
		defer e.Delete()
		var changes []string
		e.OnTextChanged(func(s string) { changes = append(changes, s) })
		qutetest.Type(e, "héllo")
		if e.Text() != "héllo" {
			t.Errorf("Text() = %q", e.Text())
		}
		if len(changes) != 5 {
			t.Errorf("textChanged fired %d times", len(changes))
		}
	})
}

type canvas struct {
	qtwidgets.QWidgetBase
	paints int
	sized  bool
}

func (c *canvas) PaintEvent(e *qtgui.QPaintEvent) {
	c.paints++
	p := qtgui.NewQPainterOn(c)
	p.FillRect(e.Rect(), qtgui.RGB(200, 30, 30))
	p.End()
	p.Delete()
}

func (c *canvas) SizeHint() qtcore.QSize {
	c.sized = true
	return qtcore.QSize{Width: 40, Height: 30}
}

func TestVirtualOverride(t *testing.T) {
	qutetest.Run(t, func() {
		c := &canvas{}
		w := qtwidgets.NewQWidgetWith(c, nil)
		defer w.Delete()
		if got := w.SizeHint(); got != (qtcore.QSize{Width: 40, Height: 30}) {
			t.Errorf("SizeHint() = %v", got)
		}
		w.Resize(40, 30)
		img := qute.Screenshot(w)
		if c.paints == 0 {
			t.Fatal("PaintEvent override not called")
		}
		r, g, b, _ := img.At(10, 10).RGBA()
		if r>>8 != 200 || g>>8 != 30 || b>>8 != 30 {
			t.Errorf("pixel = %d,%d,%d", r>>8, g>>8, b>>8)
		}
		if c.Super() == nil {
			t.Fatal("Super nil")
		}
	})
}

func TestScopedEventExpires(t *testing.T) {
	var saved *qtgui.QPaintEvent
	qutetest.Run(t, func() {
		sc := &savingCanvas{save: &saved}
		w := qtwidgets.NewQWidgetWith(sc, nil)
		defer w.Delete()
		w.Resize(10, 10)
		qute.Screenshot(w)
	})
	if saved == nil {
		t.Fatal("no paint event")
	}
	if saved.Alive() {
		t.Error("scoped event still alive after its handler returned")
	}
}

type savingCanvas struct {
	qtwidgets.QWidgetBase
	save **qtgui.QPaintEvent
}

func (c *savingCanvas) PaintEvent(e *qtgui.QPaintEvent) { *c.save = e }

func TestGCFreesUnreachable(t *testing.T) {
	before := qutetest.Freed()
	qutetest.Run(t, func() {
		for i := 0; i < 50; i++ {
			qtwidgets.NewQLabelWithText("garbage", nil)
		}
	})
	qutetest.CollectGarbage()
	if freed := qutetest.Freed() - before; freed < 40 {
		t.Errorf("GC freed %d of 50 unreachable labels", freed)
	}
}

func TestVisibleWindowPinned(t *testing.T) {
	var win *qtwidgets.QWidget
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		w.SetObjectName("pinned")
		w.Show()
		win = w
	})
	ptr := win.Ptr()
	win = nil
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		found := false
		for _, w := range qtwidgets.QApplicationTopLevelWidgets() {
			if w.Ptr() == ptr {
				found = true
				if w.Ownership() != qtcore.Owned {
					t.Errorf("visible window state %v", w.Ownership())
				}
				w.Hide()
				w.Delete()
			}
		}
		if !found {
			t.Error("visible window was freed by GC")
		}
	})
}

func TestDumpTree(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		defer w.Delete()
		w.SetObjectName("root")
		lay := qtwidgets.NewQHBoxLayout(w)
		b := qtwidgets.NewQPushButtonWithText("ok", nil)
		b.SetObjectName("ok")
		lay.AddWidget(b)
		b.OnClicked(func(bool) {}) // its window's handle holds the handler
		s := qute.DumpTree(w)
		for _, want := range []string{`"class": "QWidget"`, `"objectName": "root"`, `"class": "QPushButton"`, `"ownership": "Parented"`, `"anchored": "root"`} {
			if !strings.Contains(s, want) {
				t.Errorf("tree missing %s:\n%s", want, s)
			}
		}
	})
}

func TestPostFromGoroutine(t *testing.T) {
	done := make(chan string)
	qute.Post(func() {
		l := qtwidgets.NewQLabelWithText("from post", nil)
		done <- l.Text()
		l.Delete()
	})
	if got := <-done; got != "from post" {
		t.Errorf("got %q", got)
	}
}
