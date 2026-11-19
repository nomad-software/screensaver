package qute_test

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
	"weak"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// t.Fatal must not be called inside qutetest.Run: it would end the GUI goroutine.

// These tests guard against the use-after-free in miqt issue #366: once a
// generated method has handed the raw pointer to C, nothing refers to the Go
// wrapper, so a call that runs a nested event loop can let the cleanup queue
// drain and free the receiver or an argument while Qt still uses it. Generated
// methods call runtime.KeepAlive after the C call to prevent that.

// collectInside runs the GC and lets the GUI thread drain the cleanup queue
// from inside a running call. It returns how many objects were freed.
func collectInside() int64 {
	before := qutetest.Freed()
	for range 4 {
		runtime.GC()
		time.Sleep(5 * time.Millisecond) // cleanups run on their own goroutine
		qute.ProcessEvents()
	}
	return qutetest.Freed() - before
}

// TestKeepAliveReceiver: an unreferenced QEventLoop must survive its own Exec.
func TestKeepAliveReceiver(t *testing.T) {
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		loop := qtcore.NewQEventLoop(nil)
		p := loop.Ptr()
		timer := qtcore.NewQTimer(nil)
		defer timer.Delete()
		timer.SetSingleShot(true)
		timer.OnTimeout(func() {
			if n := collectInside(); n > 0 {
				// The loop is gone, so returning through it would crash, and
				// t.Fatal cannot unwind the C frames of the nested loop.
				fmt.Fprintf(os.Stderr, "--- FAIL: TestKeepAliveReceiver\n\tQEventLoop.Exec: GC freed %d object(s) while the call was running; the receiver must be kept alive\n", n)
				os.Exit(1)
			}
			qtcore.QEventLoopFromPtr(p, qtcore.Borrowed).Quit()
		})
		timer.Start()
		loop.Exec() // last use of loop: only KeepAlive holds it now
	})
}

// fontChangeWidget runs the GC from inside QWidget::setFont, which sends
// QEvent::FontChange synchronously.
type fontChangeWidget struct {
	qtwidgets.QWidgetBase
	freed int64
	seen  bool
}

func (w *fontChangeWidget) ChangeEvent(e *qtcore.QEvent) {
	if e.Type() == qtcore.QEventFontChange && !w.seen {
		w.seen = true
		w.freed = collectInside()
	}
	w.Super().ChangeEvent(e)
}

// TestKeepAliveArgument: an unreferenced value-class argument must survive
// the call it was passed to.
func TestKeepAliveArgument(t *testing.T) {
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		v := &fontChangeWidget{}
		w := qtwidgets.NewQWidgetWith(v, nil)
		defer w.Delete()
		w.SetFont(qtgui.NewQFontWithFamily("Monospace")) // only KeepAlive holds the font
		if !v.seen {
			t.Error("QWidget.SetFont did not send QEvent::FontChange; the test no longer runs the GC inside the call")
		}
		if v.freed > 0 {
			t.Errorf("QWidget.SetFont: GC freed %d object(s) while the call was running; arguments must be kept alive", v.freed)
		}
	})
}

// overrideModel returns a fresh, otherwise unreferenced QVariant from Data.
type overrideModel struct {
	qtcore.QAbstractListModelBase
	returned []weak.Pointer[qtcore.QVariant]
}

func (m *overrideModel) RowCount(parent *qtcore.QModelIndex) int { return 1 }

func (m *overrideModel) Data(index *qtcore.QModelIndex, role int) *qtcore.QVariant {
	if role != int(qtcore.DisplayRole) {
		return nil
	}
	v := qtcore.NewQVariantOf("hello")
	m.returned = append(m.returned, weak.Make(v))
	return v
}

// TestOverrideReturnValue: a value returned from a Go override is copied by
// the C++ shim before Go lets go of it (miqt #154 and #366), and the Go copy
// is still freed by GC afterwards, so it neither crashes nor leaks.
func TestOverrideReturnValue(t *testing.T) {
	om := &overrideModel{}
	qutetest.Run(t, func() {
		m := qtcore.NewQAbstractListModelWith(om, nil)
		defer m.Delete()
		for range 50 {
			// The C++ caller goes through the shim, which copies the result.
			if got := m.Data(m.Index(0, 0)).ToString(); got != "hello" {
				t.Errorf("Data() = %q, want %q", got, "hello")
				return
			}
			collectInside()
		}
	})
	qutetest.CollectGarbage()
	if len(om.returned) != 50 {
		t.Fatalf("Data override ran %d times, want 50", len(om.returned))
	}
	for i, w := range om.returned {
		if w.Value() != nil {
			t.Fatalf("QVariant %d returned from the Data override is still reachable after GC; it leaks", i)
		}
	}
}
