package qute_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	rt "github.com/nomad-software/qute/internal/rt"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// Object lifetimes qute used to get wrong: objects Qt frees without telling
// a plain wrapper, a QObject seen through a second base class, a visible
// QWindow, windows kept alive by their own handlers, and widgets left in a
// layout that never got a widget.

// mustPanic runs f and reports whether it panicked with a message containing want.
func mustPanic(t *testing.T, what, want string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		p := recover()
		if p == nil {
			t.Errorf("%s: no panic", what)
			return
		}
		if msg, _ := p.(string); !strings.Contains(msg, want) {
			t.Errorf("%s: panic %v, want it to mention %q", what, p, want)
		}
	}()
	f()
}

// A layout item Qt made dies with its layout; the wrapper used to stay Alive
// and segfault on the next call.
func TestLayoutItemDiesWithLayout(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		lay := qtwidgets.NewQVBoxLayout(w)
		lay.AddWidget(qtwidgets.NewQLabelWithText("hi", nil))
		lay.AddStretch()
		label, stretch := lay.ItemAt(0), lay.ItemAt(1)
		if !label.Alive() || label.IsEmpty() || !stretch.Alive() {
			t.Error("items unusable while the layout lives")
		}
		w.Delete()
		if label.Alive() || stretch.Alive() {
			t.Errorf("items alive after their layout died: %v %v", label.Alive(), stretch.Alive())
		}
		mustPanic(t, "IsEmpty", "this QLayoutItem belongs to was destroyed", func() { label.IsEmpty() })
		mustPanic(t, "Geometry", "this QLayoutItem belongs to was destroyed", func() { stretch.Geometry() })
	})
}

// Deleting a widget makes its layout free the item that held it.
func TestLayoutItemDiesWithWidget(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		defer w.Delete()
		lay := qtwidgets.NewQVBoxLayout(w)
		lbl := qtwidgets.NewQLabelWithText("hi", nil)
		lay.AddWidget(lbl)
		it := lay.ItemAt(0)
		lbl.Delete()
		if it.Alive() {
			t.Error("item alive after its widget was deleted")
		}
		mustPanic(t, "IsEmpty", "this QLayoutItem belongs to was destroyed", func() { it.IsEmpty() })
	})
}

// A spacer built in Go reports its death when its layout frees it.
func TestSpacerItemReportsDeath(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		lay := qtwidgets.NewQVBoxLayout(w)
		s := qtwidgets.NewQSpacerItemWithW(10, 10)
		lay.AddItem(s)
		if s.Ownership() != qtcore.Parented {
			t.Errorf("spacer in a layout: %v, want Parented", s.Ownership())
		}
		w.Delete()
		if s.Alive() {
			t.Errorf("spacer %v after its layout died, want Dead", s.Ownership())
		}
	})
}

// A nested layout seen as a QLayoutItem is the layout's own handle.
func TestNestedLayoutItemSharesHandle(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		outer := qtwidgets.NewQVBoxLayout(w)
		inner := qtwidgets.NewQHBoxLayout(nil)
		outer.AddLayout(inner)
		it := outer.ItemAt(0)
		if it.Ownership() != inner.Ownership() || it.Ptr() == nil {
			t.Errorf("item %v, layout %v: want one shared state", it.Ownership(), inner.Ownership())
		}
		inner.Delete()
		if it.Alive() {
			t.Error("the item view of a deleted layout is alive")
		}
		w.Delete()
	})
}

// A QObject reached through its second base (a text item listed by the
// scene) shares the QObject's handle, so it learns the object died.
func TestSecondBaseSharesHandle(t *testing.T) {
	qutetest.Run(t, func() {
		sc := qtwidgets.NewQGraphicsScene(nil)
		defer sc.Delete()
		txt := sc.AddText("hi")
		items := sc.Items()
		if len(items) != 1 {
			t.Errorf("scene has %d items, want 1", len(items))
			return
		}
		it := items[0]
		if it.Ownership() != txt.Ownership() {
			t.Errorf("item %v, text %v: want the same state", it.Ownership(), txt.Ownership())
		}
		if rt.RefOf(unsafe.Pointer(it)).H != rt.RefOf(unsafe.Pointer(txt)).H {
			t.Error("the item and the text item have different handles")
		}
		txt.Delete()
		if it.Alive() {
			t.Error("item view alive after the text item was deleted")
		}
		mustPanic(t, "Pos", "object destroyed", func() { it.Pos() })
	})
}

// A widget seen as a QPaintDevice is the widget's handle; a device that is
// not a QObject (an image) depends on the painter that handed it out.
func TestPaintDeviceLifetime(t *testing.T) {
	qutetest.Run(t, func() {
		img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 8, Height: 8}, qtgui.QImageFormat_ARGB32)
		p := qtgui.NewQPainterOn(img)
		d := p.Device()
		if d.Width() != 8 {
			t.Errorf("device width %d, want 8", d.Width())
		}
		p.End()
		p.Delete()
		img.Delete()
		if d.Alive() {
			t.Error("device alive after its painter and image were deleted")
		}
		mustPanic(t, "Width", "QPainter this QPaintDevice belongs to was destroyed", func() { d.Width() })

		w := qtwidgets.NewQWidget(nil)
		wd := w.AsQPaintDevice()
		if !wd.Alive() || rt.RefOf(unsafe.Pointer(wd)).H != rt.RefOf(unsafe.Pointer(w)).H {
			t.Error("a widget's paint device view does not share its handle")
		}
		w.Delete()
		if wd.Alive() {
			t.Error("paint device view alive after its widget was deleted")
		}
	})
}

// A visible QWindow is pinned like a visible widget: GC must not delete it
// while it is on screen, and frees it once it is hidden.
func TestVisibleQWindowPinned(t *testing.T) {
	count := func() (n int) {
		qutetest.Run(t, func() {
			for _, w := range qtgui.QGuiApplicationTopLevelWindows() {
				if w.ObjectName() == "pinned-window" {
					n++
				}
			}
		})
		return n
	}
	qutetest.Run(t, func() {
		w := qtgui.NewQWindow(nil)
		w.SetObjectName("pinned-window")
		w.Show()
	})
	qutetest.CollectGarbage()
	if n := count(); n != 1 {
		t.Fatalf("visible QWindow count after GC = %d, want 1", n)
	}
	qutetest.Run(t, func() {
		for _, w := range qtgui.QGuiApplicationTopLevelWindows() {
			if w.ObjectName() == "pinned-window" {
				w.Hide()
			}
		}
	})
	qutetest.CollectGarbage()
	if n := count(); n != 0 {
		t.Errorf("hidden, unreferenced QWindow count after GC = %d, want 0", n)
	}
}

// A dialog whose button handler refers to the dialog used to live forever:
// the closure was a GC root. It is now held by the dialog's own handle.
func TestWindowWithHandlersCollected(t *testing.T) {
	const n = 20
	before := qutetest.Freed()
	qutetest.Run(t, func() {
		for i := 0; i < n; i++ {
			d := qtwidgets.NewQDialog(nil)
			b := qtwidgets.NewQPushButtonWithText("ok", d)
			b.OnClicked(func(bool) { d.Accept() })
			d.Show()
			d.Hide()
		}
	})
	qutetest.CollectGarbage()
	if got := qutetest.Freed() - before; got < n {
		t.Errorf("freed %d of %d dialogs whose handlers refer to them", got, n)
	}
}

type selfRef struct {
	qtwidgets.QWidgetBase
	win *qtwidgets.QWidget
}

func (s *selfRef) PaintEvent(*qtgui.QPaintEvent) {}

// A Go subclass inside a window, whose value refers to the window, is held by
// the window too, so the window can still be collected.
func TestWindowWithSubclassChildCollected(t *testing.T) {
	const n = 20
	before := qutetest.Freed()
	qutetest.Run(t, func() {
		for i := 0; i < n; i++ {
			w := qtwidgets.NewQWidget(nil)
			qtwidgets.NewQWidgetWith(&selfRef{win: w}, w)
		}
	})
	qutetest.CollectGarbage()
	if got := qutetest.Freed() - before; got < n {
		t.Errorf("freed %d of %d windows with a subclass child that refers back", got, n)
	}
}

// A handler connected before its widget joined a window moves to the window
// with it; one on a widget moved to another window moves again. Neither is
// lost while its window lives.
func TestHandlersFollowReparenting(t *testing.T) {
	var clicks int
	var b *qtwidgets.QPushButton
	var keep *qtwidgets.QWidget
	qutetest.Run(t, func() {
		b = qtwidgets.NewQPushButtonWithText("x", nil)
		b.OnClicked(func(bool) { clicks++ })
		if got := rt.RefOf(unsafe.Pointer(b)).H.Anchored(); got != "global" {
			t.Errorf("parentless sender anchored %q, want global", got)
		}
		first := qtwidgets.NewQWidget(nil)
		qtwidgets.NewQVBoxLayout(first).AddWidget(b)
		if got := rt.RefOf(unsafe.Pointer(b)).H.Anchored(); got != "root" {
			t.Errorf("sender in a window anchored %q, want root", got)
		}
		keep = qtwidgets.NewQWidget(nil)
		b.SetParent(keep) // first is now unreachable and empty of b
	})
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		if !b.Alive() {
			t.Error("button died with the window it left")
			return
		}
		b.Click()
		if clicks != 1 {
			t.Errorf("clicks = %d after GC, want 1: the handler was lost", clicks)
		}
		keep.Delete()
	})
}

// Handlers of a window that is still referenced survive any number of GCs.
func TestHandlersOfLiveWindowSurviveGC(t *testing.T) {
	var w *qtwidgets.QWidget
	var b *qtwidgets.QPushButton
	clicks := 0
	qutetest.Run(t, func() {
		w = qtwidgets.NewQWidget(nil)
		b = qtwidgets.NewQPushButtonWithText("x", w)
		b.OnClicked(func(bool) { clicks++ })
	})
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		b.Click()
		w.Delete()
	})
	if clicks != 1 {
		t.Errorf("clicks = %d, want 1", clicks)
	}
}

// A view that keeps a model whose Go value refers to the window used to
// pin the window through the kept reference.
type winModel struct {
	qtcore.QAbstractListModelBase
	win *qtwidgets.QWidget
}

func (m *winModel) RowCount(*qtcore.QModelIndex) int               { return 0 }
func (m *winModel) Data(*qtcore.QModelIndex, int) *qtcore.QVariant { return nil }

func TestWindowWithKeptModelCollected(t *testing.T) {
	const n = 10
	before := qutetest.Freed()
	qutetest.Run(t, func() {
		for i := 0; i < n; i++ {
			w := qtwidgets.NewQWidget(nil)
			v := qtwidgets.NewQListView(w)
			m := &winModel{win: w}
			qtcore.NewQAbstractListModelWith(m, w)
			v.SetModel(m)
		}
	})
	qutetest.CollectGarbage()
	if got := qutetest.Freed() - before; got < n {
		t.Errorf("freed %d of %d windows whose view keeps a model that refers back", got, n)
	}
}

// Widgets added to a layout that never gets a widget used to leak: Parented
// for Go, parentless for Qt. When the layout dies they are Go's again.
func TestOrphanLayoutWidgetsFreed(t *testing.T) {
	alive := func() (n int) {
		qutetest.Run(t, func() {
			for _, w := range qtwidgets.QApplicationAllWidgets() {
				if w.ObjectName() == "orphan-label" {
					n++
				}
			}
		})
		return n
	}
	qutetest.Run(t, func() {
		for i := 0; i < 20; i++ {
			lay := qtwidgets.NewQVBoxLayout(nil)
			l := qtwidgets.NewQLabelWithText("x", nil)
			l.SetObjectName("orphan-label")
			lay.AddWidget(l)
		}
	})
	qutetest.CollectGarbage()
	qutetest.CollectGarbage() // the labels are freed once their layouts are
	if n := alive(); n != 0 {
		t.Errorf("%d of 20 labels in dropped parentless layouts still alive", n)
	}
	// a layout installed later still owns its widgets through Qt
	qutetest.Run(t, func() {
		lay := qtwidgets.NewQVBoxLayout(nil)
		l := qtwidgets.NewQLabelWithText("x", nil)
		lay.AddWidget(l)
		w := qtwidgets.NewQWidget(nil)
		w.SetLayout(lay)
		if l.Ownership() != qtcore.Parented {
			t.Errorf("label in an installed layout: %v, want Parented", l.Ownership())
		}
		lay.Delete() // the label stays, a child of w
		if !l.Alive() || l.Ownership() != qtcore.Parented {
			t.Errorf("label after its layout was deleted: alive %v, %v", l.Alive(), l.Ownership())
		}
		w.Delete()
		if l.Alive() {
			t.Error("label alive after its window was deleted")
		}
	})
}

// Images dropped without Delete() hold memory the Go heap cannot see; the
// bytes are counted, and enough of them runs the GC, even inside one handler.
func TestImageMemoryTriggersGC(t *testing.T) {
	runtime.GC()
	qutetest.CollectGarbage()
	live0, gcs0 := rt.MemoryLive(), rt.MemoryCollections.Load()
	var peak int64
	qutetest.Run(t, func() {
		for i := 0; i < 60; i++ { // 60 x 16 MB
			img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 2000, Height: 2000}, qtgui.QImageFormat_ARGB32)
			img.Fill(0)
			peak = max(peak, rt.MemoryLive()-live0)
		}
	})
	if rt.MemoryCollections.Load() == gcs0 {
		t.Error("960 MB of images never made qute run the GC")
	}
	if peak > 400<<20 {
		t.Errorf("tracked image memory peaked at %d MB in one handler, want it bounded", peak>>20)
	}
	qutetest.CollectGarbage()
	if live := rt.MemoryLive() - live0; live > 64<<20 {
		t.Errorf("%d MB of images still counted after GC", live>>20)
	}
	// a size change is measured again, and Delete gives the bytes back
	qutetest.Run(t, func() {
		img := qtgui.NewQImage()
		before := rt.MemoryLive()
		img2 := qtgui.NewQImageWithSize(qtcore.QSize{Width: 100, Height: 100}, qtgui.QImageFormat_ARGB32)
		if got := rt.MemoryLive() - before; got != 100*100*4 {
			t.Errorf("a 100x100 image counts %d bytes, want %d", got, 100*100*4)
		}
		img2.Delete()
		if rt.MemoryLive() != before {
			t.Error("Delete did not give the bytes back")
		}
		img.Delete()
	})
}

type popupWindow struct {
	qtwidgets.QMainWindowBase
	menu *qtwidgets.QMenu
}

func (w *popupWindow) CreatePopupMenu() *qtwidgets.QMenu {
	w.menu = qtwidgets.NewQMenu(nil)
	return w.menu
}

// An object a Go override returns to a C++ caller that owns it (a menu from
// createPopupMenu, mime data from mimeData) is Parented from then on, so GC
// cannot free it while Qt still uses it.
func TestOverrideResultTransferredToCpp(t *testing.T) {
	qutetest.Run(t, func() {
		v := &popupWindow{}
		w := qtwidgets.NewQMainWindowWith(v, nil)
		defer w.Delete()
		h := rt.RefOf(unsafe.Pointer(w)).H
		var ret unsafe.Pointer
		self, vtable := h.Subclass()
		for _, vt := range vtable {
			if vt.Name == "CreatePopupMenu" {
				vt.Call(self, nil, unsafe.Pointer(&ret)) // as the shim does for Qt
			}
		}
		if ret == nil {
			t.Error("the override was not called")
			return
		}
		m := v.menu
		if m.Ptr() != ret {
			t.Error("the shim got a different pointer than the override returned")
		}
		if m.Ownership() != qtcore.Parented {
			t.Errorf("menu returned to Qt: %v, want Parented", m.Ownership())
		}
		m.Delete()
	})
}

// The tests below cover misuse that used to crash, or left a handle in the
// wrong state.

// panicText runs f and returns the text of its panic, or "".
func panicText(f func()) (msg string) {
	defer func() {
		if p := recover(); p != nil {
			msg, _ = p.(string)
			if msg == "" {
				msg = "non-string panic"
			}
		}
	}()
	f()
	return ""
}

// A painter and the large image it draws on, dropped together inside one
// handler, are freed by the memory-pressure path. It used to free the image
// first, and ~QPainter then ran on freed memory.
func TestMemoryPathFreesPainterBeforeImage(t *testing.T) {
	qutetest.Run(t, func() {
		for i := 0; i < 100; i++ {
			img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 1024, Height: 1024}, qtgui.QImageFormat_ARGB32)
			p := qtgui.NewQPainterOn(img)
			p.FillRect(qtcore.QRect{Width: 10, Height: 10}, qtgui.RGB(1, 2, 3))
			time.Sleep(time.Millisecond) // lets GC cleanups queue the pair
		}
	})
	qutetest.CollectGarbage()
}

// Deleting the device a painter still draws on used to leave the painter
// with a dangling pointer. It panics now; after End it is allowed.
func TestDeleteDeviceUnderPainterPanics(t *testing.T) {
	qutetest.Run(t, func() {
		img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 8, Height: 8}, qtgui.QImageFormat_ARGB32)
		p := qtgui.NewQPainterOn(img)
		msg := panicText(img.Delete)
		if !strings.Contains(msg, "QPainter still uses this QImage") {
			t.Errorf("Delete of a painted-on image: panic %q, want the QPainter message", msg)
		}
		p.FillRect(qtcore.QRect{Width: 4, Height: 4}, qtgui.RGB(9, 9, 9))
		p.End()
		if msg := panicText(img.Delete); msg != "" {
			t.Errorf("Delete after End: panic %q", msg)
		}
		p.Delete()
	})
}

// A stream does not notice its device being deleted either.
func TestDeleteStreamDevicePanics(t *testing.T) {
	qutetest.Run(t, func() {
		buf := qtcore.NewQBuffer(nil)
		buf.Open(qtcore.QIODeviceBaseWriteOnly)
		ds := qtcore.NewQDataStreamWithIODevice(buf)
		if msg := panicText(buf.Delete); !strings.Contains(msg, "QDataStream still uses this QBuffer") {
			t.Errorf("Delete of a stream's device: panic %q", msg)
		}
		ds.SetDevice(nil)
		buf.Delete()
		ds.Delete()
	})
}

// A view notices its model being deleted, so that stays allowed.
func TestDeleteViewModelAllowed(t *testing.T) {
	qutetest.Run(t, func() {
		view := qtwidgets.NewQListView(nil)
		m := qtcore.NewQStringListModelWithStrings([]string{"a"}, nil)
		view.SetModel(m)
		if msg := panicText(m.Delete); msg != "" {
			t.Errorf("Delete of a view's model: panic %q", msg)
		}
		view.Delete()
	})
}

// A layout keeps a pointer to a spacer item and does not notice its
// destructor; Delete of a spacer in a layout used to crash the next layout
// pass. It panics now and names TakeAt.
func TestDeleteSpacerInLayoutPanics(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		lay := qtwidgets.NewQVBoxLayout(w)
		sp := qtwidgets.NewQSpacerItemWithW(10, 10)
		lay.AddSpacerItem(sp)
		if msg := panicText(sp.Delete); !strings.Contains(msg, "TakeAt") {
			t.Errorf("Delete of a spacer in a layout: panic %q, want the TakeAt hint", msg)
		}
		if n := lay.Count(); n != 1 {
			t.Errorf("layout has %d items, want 1", n)
		}
		it := lay.TakeAt(0)
		if it.Ownership() != qtcore.Owned {
			t.Errorf("taken item: %v, want Owned", it.Ownership())
		}
		it.Delete()
		qutetest.Wait(1)
		w.Delete()
	})
}

// A sub-layout is a QObject: its destructor takes it out of its parent
// layout, so deleting it stays allowed.
func TestDeleteSubLayoutAllowed(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		lay := qtwidgets.NewQVBoxLayout(w)
		sub := qtwidgets.NewQHBoxLayout(nil)
		lay.AddLayout(sub)
		if msg := panicText(sub.Delete); msg != "" {
			t.Errorf("Delete of a sub-layout: panic %q", msg)
		}
		if n := lay.Count(); n != 0 {
			t.Errorf("layout has %d items after its sub-layout was deleted, want 0", n)
		}
		w.Delete()
	})
}

// Rule 5 holds in every build, not only with -tags qute_debug.
func TestDeleteInHandlerPanics(t *testing.T) {
	qutetest.Run(t, func() {
		btn := qtwidgets.NewQPushButtonWithText("x", nil)
		var msg string
		btn.OnClicked(func(bool) { msg = panicText(btn.Delete) })
		qutetest.Click(btn)
		if !strings.Contains(msg, "DeleteLater") {
			t.Errorf("want the DeleteLater hint, got %q", msg)
		}
		if !btn.Alive() {
			t.Error("the button was deleted inside its handler")
		}
		btn.Delete()
	})
}

// A transfer-to-go result that Qt has given a parent is Parented, not Owned.
func TestTransferToGoResultWithParentIsParented(t *testing.T) {
	qutetest.Run(t, func() {
		parent := qtcore.NewQObject(nil)
		child := qtcore.NewQObject(parent)
		h := rt.Wrap(child.Ptr(), "QObject", rt.KindQObject, rt.TakeOwned, rt.DeleteQObject)
		if h.State() != rt.Parented {
			t.Errorf("transfer-to-go of a child: %v, want Parented", h.State())
		}
		child.SetParent(nil)
		if child.Ownership() != qtcore.Owned {
			t.Errorf("after SetParent(nil): %v, want Owned", child.Ownership())
		}
		child.Delete()
		parent.Delete()
	})
}

// A text cursor keeps its document from being collected, but Qt copes with
// the document being deleted, so that is allowed (kept-delete: allow).
func TestDeleteCursorDocumentAllowed(t *testing.T) {
	qutetest.Run(t, func() {
		doc := qtgui.NewQTextDocument(nil)
		c := qtgui.NewQTextCursorWithTextDocument(doc)
		if msg := panicText(doc.Delete); msg != "" {
			t.Errorf("Delete of a cursor's document: panic %q", msg)
		}
		if !c.IsNull() {
			t.Error("cursor not null after its document was deleted")
		}
	})
}

type guardedItem struct {
	qtwidgets.QGraphicsItemBase
	guard   *qtgui.QPainterStateGuard
	painter *qtgui.QPainter
}

func (g *guardedItem) BoundingRect() qtcore.QRectF { return qtcore.QRectF{Width: 20, Height: 20} }

func (g *guardedItem) Paint(p *qtgui.QPainter, _ *qtwidgets.QStyleOptionGraphicsItem, _ *qtwidgets.QWidget) {
	g.painter = p
	g.guard = qtgui.NewQPainterStateGuardWithPainter(p) // never deleted
	p.Translate(5, 5)
}

// A QPainterStateGuard left alive on a handler's scoped painter ends with
// the handler, as its C++ scope would: it restores the painter while Qt still
// has it. It used to live on until GC freed it, and its destructor then
// called restore() on a painter that was long gone.
func TestGuardOnScopedPainterEndsWithHandler(t *testing.T) {
	item := &guardedItem{}
	qutetest.Run(t, func() {
		scene := qtwidgets.NewQGraphicsScene(nil)
		qtwidgets.NewQGraphicsItemWith(item, nil)
		scene.AddItem(item)
		img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 30, Height: 30}, qtgui.QImageFormat_ARGB32)
		p := qtgui.NewQPainterOn(img)
		scene.Render(p)
		p.Delete()
		img.Delete()
		if item.guard == nil {
			t.Error("Paint did not run")
			return
		}
		if item.guard.Alive() {
			t.Error("guard alive after the handler that owned its painter returned")
		}
		if item.painter.Alive() {
			t.Error("scoped painter alive after its handler returned")
		}
		defer func() {
			if msg := fmt.Sprint(recover()); !strings.Contains(msg, "the QPainter argument it used expired") {
				t.Errorf("use of the ended guard: panic %q", msg)
			}
			scene.Delete()
		}()
		item.guard.Restore()
	})
	item.guard = nil
	qutetest.CollectGarbage() // nothing left for GC to free on a dead painter
}

// A QSignalMapper stores a mapped object as a plain pointer: the mapper
// keeps it from the GC, so Map never emits an object that was collected.
func TestSignalMapperKeepsMappedObject(t *testing.T) {
	var m *qtcore.QSignalMapper
	var s *qtcore.QObject
	qutetest.Run(t, func() {
		m = qtcore.NewQSignalMapper(nil)
		s = qtcore.NewQObject(nil)
		o := qtcore.NewQObject(nil)
		o.SetObjectName("mapped")
		m.SetMappingObject(s, o)
	})
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		name := ""
		m.OnMappedObject(func(o *qtcore.QObject) { name = o.ObjectName() })
		m.MapWithObject(s)
		if name != "mapped" {
			t.Errorf("mapped object after a GC: name %q, want %q", name, "mapped")
		}
		m.Delete()
		s.Delete()
	})
}
