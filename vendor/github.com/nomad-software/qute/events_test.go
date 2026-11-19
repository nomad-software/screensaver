package qute_test

import (
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// Events can be made in Go and sent: sendEvent does not take them, so they
// stay Owned and alive after it, and Qt acts on them as on real input.
func TestSendGoEvents(t *testing.T) {
	qutetest.Run(t, func() {
		edit := qtwidgets.NewQLineEdit(nil)
		defer edit.Delete()
		key := qtgui.NewQKeyEventFull(qtcore.QEventKeyPress, int(qtcore.Key_A), qtcore.NoModifier, "a", false, 1)
		qtcore.QCoreApplicationSendEvent(edit, key)
		if got := edit.Text(); got != "a" {
			t.Errorf("text after a key event = %q, want a", got)
		}
		if !key.Alive() || key.Ownership() != qtcore.Owned {
			t.Errorf("sent event: alive %v, %v, want alive and Owned", key.Alive(), key.Ownership())
		}
		if qtgui.QKeyEventCast(key) != key {
			t.Error("the cast of a Go event is not its own wrapper")
		}
		key.Delete()

		button := qtwidgets.NewQPushButtonWithText("press", nil)
		defer button.Delete()
		button.Resize(100, 30)
		clicked := 0
		button.OnClicked(func(bool) { clicked++ })
		at := qtcore.QPointF{X: 50, Y: 15}
		for _, typ := range []qtcore.QEventType{qtcore.QEventMouseButtonPress, qtcore.QEventMouseButtonRelease} {
			buttons := qtcore.LeftButton
			if typ == qtcore.QEventMouseButtonRelease {
				buttons = 0
			}
			e := qtgui.NewQMouseEvent(typ, at, at, qtcore.LeftButton, buttons, qtcore.NoModifier)
			qtcore.QCoreApplicationSendEvent(button, e)
		}
		if clicked != 1 {
			t.Errorf("clicked %d times by a press and release, want 1", clicked)
		}
	})
}

// keyCatcher keeps the event its override is handed.
type keyCatcher struct {
	qtwidgets.QWidgetBase
	got *qtgui.QKeyEvent
	key int
}

func (k *keyCatcher) KeyPressEvent(e *qtgui.QKeyEvent) {
	k.got, k.key = e, e.Key()
}

// An override handed a Go event sees it as any event: valid only inside the
// handler (rule 6). The Go event itself lives on.
func TestGoEventInOverride(t *testing.T) {
	qutetest.Run(t, func() {
		k := &keyCatcher{}
		w := qtwidgets.NewQWidgetWith(k, nil)
		defer w.Delete()
		e := qtgui.NewQKeyEvent(qtcore.QEventKeyPress, int(qtcore.Key_B), qtcore.NoModifier)
		qtcore.QCoreApplicationSendEvent(w, e)
		if k.key != int(qtcore.Key_B) {
			t.Errorf("the override saw key %d, want B", k.key)
		}
		if k.got == nil || k.got.Alive() {
			t.Error("the handler's event is alive after the handler returned")
		}
		if !e.Alive() {
			t.Error("the Go event died with the handler's view of it")
		}
		e.Delete()
	})
}

// eventRecorder records the types of the events it is handed.
type eventRecorder struct {
	qtcore.QObjectBase
	types []qtcore.QEventType
}

func (r *eventRecorder) Event(e *qtcore.QEvent) bool {
	r.types = append(r.types, e.Type())
	return r.Super().Event(e)
}

// postEvent takes the event (transfer-to-cpp) and Qt deletes it once it is
// delivered: the event's shim reports that, so its handle is Dead rather than
// dangling.
func TestPostGoEvent(t *testing.T) {
	qutetest.Run(t, func() {
		r := &eventRecorder{}
		o := qtcore.NewQObjectWith(r, nil)
		defer o.Delete()
		typ := qtcore.QEventType(qtcore.QEventRegisterEventType())
		e := qtcore.NewQEvent(typ)
		qtcore.QCoreApplicationPostEvent(o, e)
		if e.Ownership() != qtcore.Parented {
			t.Errorf("posted event: %v, want Parented", e.Ownership())
		}
		qutetest.Wait(10)
		if e.Alive() {
			t.Error("the posted event is alive after Qt delivered and deleted it")
		}
		if len(r.types) != 1 || r.types[0] != typ {
			t.Errorf("events delivered %v, want one of type %d", r.types, typ)
		}
	})
}

// An event Go made and dropped is freed by the GC.
func TestGoEventsCollected(t *testing.T) {
	const n = 20
	before := qutetest.Freed()
	qutetest.Run(t, func() {
		for range n {
			qtgui.NewQKeyEvent(qtcore.QEventKeyPress, int(qtcore.Key_C), qtcore.NoModifier)
		}
	})
	qutetest.CollectGarbage()
	if got := qutetest.Freed() - before; got < n {
		t.Errorf("freed %d of %d dropped events", got, n)
	}
}
