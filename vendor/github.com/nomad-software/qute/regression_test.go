package qute_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unsafe"
	"weak"

	"github.com/nomad-software/qute"
	rt "github.com/nomad-software/qute/internal/rt"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// Regression tests for bindings whose signature was wrong: out-parameters
// guessed from the type alone, pointers Qt keeps without owning, and an
// override that saw a copy of Qt's object.

// copyDataTo writes 16 floats; the old binding pointed it at one stack float.
func TestMatrixCopyDataTo(t *testing.T) {
	qutetest.Run(t, func() {
		m := qtgui.NewQMatrix4x4()
		m.TranslateFloat32Float32(1, 2, 3)
		got := m.CopyDataTo()
		want := [16]float32{
			1, 0, 0, 1,
			0, 1, 0, 2,
			0, 0, 1, 3,
			0, 0, 0, 1,
		}
		if got != want {
			t.Errorf("CopyDataTo = %v, want row-major %v", got, want)
		}
	})
}

// The image-over-data constructor used to build the image over a dead stack
// byte and dropped the data argument. It now takes the caller's buffer.
func TestImageOverCallerBuffer(t *testing.T) {
	qutetest.Run(t, func() {
		const w, h = 4, 2
		buf := make([]uint32, w*h)
		for i := range buf {
			buf[i] = 0xff112233
		}
		// Qt keeps the pointer for the image's life, so the Go memory must be
		// pinned until the image is gone.
		var pin runtime.Pinner
		pin.Pin(&buf[0])
		defer pin.Unpin()
		img := qtgui.NewQImageOverMemoryBytesPerLine(unsafe.Pointer(&buf[0]), w, h, w*4, qtgui.QImageFormat_ARGB32)
		if got := img.Pixel(3, 1); got != 0xff112233 {
			t.Errorf("Pixel = %#x, want 0xff112233", got)
		}
		img.Delete()
	})
}

// The byte-array stream constructor kept a pointer to a temporary; it is
// skipped now, and the read-only one has its proper name.
func TestDataStreamOverBytes(t *testing.T) {
	qutetest.Run(t, func() {
		ds := qtcore.NewQDataStreamWithBytes([]byte("hello"))
		buf := make([]byte, 5)
		if n := ds.ReadRawData(buf); n != 5 || string(buf) != "hello" {
			t.Errorf("ReadRawData = %d %q, want 5 \"hello\"", n, buf)
		}
		if !ds.AtEnd() {
			t.Error("AtEnd = false after reading everything")
		}
		ds.Delete()
	})
}

// Unmap takes the address Map returned (it used to pass a stack byte).
func TestFileUnmap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "map.bin")
	if err := os.WriteFile(path, []byte("mapped bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	qutetest.Run(t, func() {
		f := qtcore.NewQFileWithName(path, nil)
		defer f.Delete()
		if !f.Open(qtcore.QIODeviceBaseReadOnly) {
			t.Error("Open failed")
			return
		}
		p := f.Map(0, 6)
		if p == nil {
			t.Error("Map returned nil")
			return
		}
		if got := string(unsafe.Slice((*byte)(p), 6)); got != "mapped" {
			t.Errorf("mapped %q, want \"mapped\"", got)
		}
		if !f.Unmap(p) {
			t.Error("Unmap of the mapped address failed")
		}
	})
}

// polish(QPalette &) reads the palette it adjusts; it was bound as out-only,
// so the caller's palette never reached Qt.
func TestStylePolishPaletteReadsInput(t *testing.T) {
	qutetest.Run(t, func() {
		red := qtgui.RGB(255, 0, 0)
		pal := qtgui.NewQPalette()
		pal.SetColor(qtgui.QPaletteWindow, red)
		got := qtwidgets.QApplicationStyle().PolishPalette(pal)
		if c := got.Color(qtgui.QPaletteWindow); c != red {
			t.Errorf("polished Window colour = %v, want the input's %v", c, red)
		}
	})
}

type longLabelButton struct{ qtwidgets.QPushButtonBase }

const longLabel = "a label much longer than the button's own text"

func (b *longLabelButton) InitStyleOption(opt *qtwidgets.QStyleOptionButton) {
	b.Super().InitStyleOption(opt)
	opt.SetText(longLabel)
}

// An InitStyleOption override used to fill in a copy Qt never saw. The paint
// path draws from the option, so the override's label must show on screen.
func TestInitStyleOptionOverrideReachesQt(t *testing.T) {
	qutetest.Run(t, func() {
		shot := func(b *qtwidgets.QPushButton) []byte {
			b.Resize(360, 40)
			return qute.Screenshot(b).Pix
		}
		over := qtwidgets.NewQPushButtonWith(&longLabelButton{}, nil)
		defer over.Delete()
		over.SetText("x")
		plainShort := qtwidgets.NewQPushButtonWithText("x", nil)
		defer plainShort.Delete()
		plainLong := qtwidgets.NewQPushButtonWithText(longLabel, nil)
		defer plainLong.Delete()
		got, short, long := shot(over), shot(plainShort), shot(plainLong)
		if bytes.Equal(got, short) {
			t.Error("the override's label never reached Qt: the button paints its own text")
		}
		if !bytes.Equal(got, long) {
			t.Error("the button does not paint the label the override set")
		}
	})
}

// Scoped style option: the override's argument is Qt's object, so it must
// expire when the override returns.
type keepOptionButton struct {
	qtwidgets.QPushButtonBase
	opt *qtwidgets.QStyleOptionButton
}

func (b *keepOptionButton) InitStyleOption(opt *qtwidgets.QStyleOptionButton) {
	b.Super().InitStyleOption(opt)
	b.opt = opt
}

func TestInitStyleOptionArgumentIsScoped(t *testing.T) {
	qutetest.Run(t, func() {
		v := &keepOptionButton{}
		btn := qtwidgets.NewQPushButtonWith(v, nil)
		defer btn.Delete()
		btn.SizeHint()
		if v.opt == nil {
			t.Error("InitStyleOption was not called")
			return
		}
		if v.opt.Alive() {
			t.Error("option still alive after the override returned; it is Qt's stack object")
		}
	})
}

// passStyle overrides the style virtuals that take a const QStyleOption *
// and hands each call straight to Super.
type passStyle struct {
	qtwidgets.QProxyStyleBase
	calls int
}

func (s *passStyle) DrawPrimitive(e qtwidgets.QStylePrimitiveElement, opt *qtwidgets.QStyleOption, p *qtgui.QPainter, w *qtwidgets.QWidget) {
	s.calls++
	s.Super().DrawPrimitive(e, opt, p, w)
}

func (s *passStyle) DrawControl(e qtwidgets.QStyleControlElement, opt *qtwidgets.QStyleOption, p *qtgui.QPainter, w *qtwidgets.QWidget) {
	s.calls++
	s.Super().DrawControl(e, opt, p, w)
}

// A style override's const QStyleOption * used to be copied as a plain
// QStyleOption, slicing off the QStyleOptionViewItem or QStyleOptionHeader
// Qt passed: Super() then drew a selected row blank and the header without
// its sort arrow. A pass-through override must paint exactly as the style.
func TestStyleOverrideOptionNotSliced(t *testing.T) {
	qutetest.Run(t, func() {
		shot := func(style qtwidgets.QStyleLike) []byte {
			tw := qtwidgets.NewQTreeWidget(nil)
			defer tw.Delete()
			tw.SetStyle(style)
			tw.Header().SetStyle(style) // setStyle does not reach child widgets
			tw.SetHeaderLabels([]string{"Subject", "From"})
			tw.Header().SetSortIndicatorShown(true)
			tw.Header().SetSortIndicator(0, qtcore.AscendingOrder)
			var sel *qtwidgets.QTreeWidgetItem
			for _, s := range []string{"one", "two", "three"} {
				it := qtwidgets.NewQTreeWidgetItemInWithStrings(tw, []string{s, "someone"})
				if s == "two" {
					sel = it
				}
			}
			tw.SetCurrentItem(sel)
			tw.Resize(300, 140)
			return qute.Screenshot(tw).Pix
		}
		fusion := qtwidgets.QStyleFactoryCreate("Fusion")
		defer fusion.Delete()
		v := &passStyle{}
		proxy := qtwidgets.NewQProxyStyleWithKeyWith(v, "Fusion")
		defer proxy.Delete()
		want, got := shot(fusion), shot(proxy)
		if v.calls == 0 {
			t.Error("the overrides were never called")
		}
		if !bytes.Equal(got, want) {
			t.Error("a pass-through style override paints differently from the style it forwards to")
		}
	})
}

// readStyle reads the derived options Qt passes a style override through
// the qstyleoption_cast bindings.
type readStyle struct {
	qtwidgets.QProxyStyleBase
	rows     []string
	selected []string
	sort     qtwidgets.QStyleOptionHeaderSortIndicator
	wrong    bool                            // a cast to the wrong option type was not nil
	kept     *qtwidgets.QStyleOptionViewItem // a cast kept past the override
}

func (s *readStyle) DrawControl(e qtwidgets.QStyleControlElement, opt *qtwidgets.QStyleOption, p *qtgui.QPainter, w *qtwidgets.QWidget) {
	switch e {
	case qtwidgets.QStyleCE_ItemViewItem:
		if vi := qtwidgets.QStyleOptionViewItemCast(opt); vi != nil {
			s.rows = append(s.rows, vi.Text())
			if vi.State()&qtwidgets.QStyleState_Selected != 0 {
				s.selected = append(s.selected, vi.Text())
			}
			s.kept = vi
		}
		if qtwidgets.QStyleOptionHeaderCast(opt) != nil || qtwidgets.QStyleOptionButtonCast(opt) != nil {
			s.wrong = true
		}
	case qtwidgets.QStyleCE_Header:
		if h := qtwidgets.QStyleOptionHeaderCast(opt); h != nil && h.Section() == 0 {
			s.sort = h.SortIndicator()
		}
	}
	s.Super().DrawControl(e, opt, p, w)
}

// An override reads the derived option Qt passed (a row's text and
// selection, a header's sort arrow) through <Option>Cast, which is nil for
// the wrong type and dies with the option when the override returns.
func TestStyleOptionCast(t *testing.T) {
	qutetest.Run(t, func() {
		v := &readStyle{}
		style := qtwidgets.NewQProxyStyleWithKeyWith(v, "Fusion")
		defer style.Delete()
		tw := qtwidgets.NewQTreeWidget(nil)
		defer tw.Delete()
		tw.SetStyle(style)
		tw.Header().SetStyle(style) // setStyle does not reach child widgets
		tw.SetHeaderLabels([]string{"Subject"})
		tw.Header().SetSortIndicatorShown(true)
		tw.Header().SetSortIndicator(0, qtcore.AscendingOrder)
		var sel *qtwidgets.QTreeWidgetItem
		for _, s := range []string{"one", "two"} {
			it := qtwidgets.NewQTreeWidgetItemInWithStrings(tw, []string{s})
			if s == "two" {
				sel = it
			}
		}
		tw.SetCurrentItem(sel)
		tw.Resize(200, 100)
		qute.Screenshot(tw)
		if !slices.Contains(v.rows, "one") || !slices.Contains(v.rows, "two") {
			t.Errorf("rows read through the cast = %q, want one and two", v.rows)
		}
		if !slices.Equal(slices.Compact(v.selected), []string{"two"}) {
			t.Errorf("selected rows = %q, want [two]", v.selected)
		}
		if v.sort == qtwidgets.QStyleOptionHeaderNone {
			t.Error("the header's sort indicator was not seen through the cast")
		}
		if v.wrong {
			t.Error("a cast to another option type returned non-nil")
		}
		if v.kept == nil || v.kept.Alive() {
			t.Error("a cast of a scoped option is still alive after the override returned")
		}
	})
}

// A stream keeps a raw pointer to its device: the device must survive a GC
// while only the stream refers to it.
func TestStreamKeepsDevice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.bin")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	var ds *qtcore.QDataStream
	qutetest.Run(t, func() {
		f := qtcore.NewQFileWithName(path, nil)
		f.Open(qtcore.QIODeviceBaseReadOnly)
		ds = qtcore.NewQDataStreamWithIODevice(f)
	})
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		if ds.AtEnd() {
			t.Error("AtEnd = true on an unread 11-byte file")
		}
		if ds.Device() == nil {
			t.Error("Device() = nil")
		}
		// replacing the device drops the old reference
		ds.SetDevice(nil)
		ds.Delete()
	})
}

// A model index keeps its model alive, and panics (not segfaults) once the
// model is deleted.
func TestIndexKeepsModel(t *testing.T) {
	var idx *qtcore.QModelIndex
	qutetest.Run(t, func() {
		m := qtcore.NewQStringListModelWithStrings([]string{"a"}, nil)
		idx = m.Index(0, 0)
	})
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		if got := idx.Data().ToString(); got != "a" {
			t.Errorf("Data = %q, want \"a\"", got)
		}
		idx.Model().AsQObject().Delete()
		defer func() {
			p := recover()
			msg, _ := p.(string)
			if !strings.Contains(msg, "QModelIndex.Data") || !strings.Contains(msg, "QStringListModel") {
				t.Errorf("after deleting the model: panic %v, want a QModelIndex.Data message naming the model", p)
			}
		}()
		idx.Data()
	})
}

// A view keeps a model nobody else references (it used to empty silently).
func TestViewKeepsModel(t *testing.T) {
	var v *qtwidgets.QListView
	qutetest.Run(t, func() {
		v = qtwidgets.NewQListView(nil)
		v.SetModel(qtcore.NewQStringListModelWithStrings([]string{"a", "b"}, nil))
	})
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		m := v.Model()
		if m == nil {
			t.Error("model was garbage collected while the view used it")
			return
		}
		if n := m.RowCount(); n != 2 {
			t.Errorf("RowCount = %d, want 2", n)
		}
		v.Delete()
	})
}

// Replacing a kept argument releases the old one, so it can be collected.
func TestKeptArgumentReplaced(t *testing.T) {
	var v *qtwidgets.QListView
	var old weak.Pointer[qtcore.QStringListModel]
	qutetest.Run(t, func() {
		v = qtwidgets.NewQListView(nil)
		m := qtcore.NewQStringListModelWithStrings([]string{"a"}, nil)
		old = weak.Make(m)
		v.SetModel(m)
		v.SetModel(nil)
	})
	qutetest.CollectGarbage()
	if old.Value() != nil {
		t.Error("the replaced model is still referenced; SetModel(nil) should drop the view's reference")
	}
	qutetest.Run(t, func() { v.Delete() })
}

// keep-each and release: a menu keeps a parentless action until it is removed.
func TestWidgetKeepsActions(t *testing.T) {
	qutetest.Run(t, func() {
		menu := qtwidgets.NewQMenu(nil)
		defer menu.Delete()
		a := qtgui.NewQActionWithText("one", nil)
		defer a.Delete()
		menu.AddAction(a)
		if !rt.KeepsArg(unsafe.Pointer(menu), unsafe.Pointer(a)) {
			t.Error("AddAction: action not kept")
		}
		menu.RemoveAction(a)
		if rt.KeepsArg(unsafe.Pointer(menu), unsafe.Pointer(a)) {
			t.Error("RemoveAction: action still kept")
		}
	})
}

// A widget's kept references go when the widget dies, even though no Go
// variable refers to it any more (its handle is anchored while it keeps them).
func TestKeptReferencesDropWithOwner(t *testing.T) {
	qutetest.Run(t, func() {
		root := qtwidgets.NewQWidget(nil)
		view := qtwidgets.NewQListView(root)
		m := qtcore.NewQStringListModelWithStrings([]string{"a"}, nil)
		defer m.Delete()
		view.SetModel(m)
		if !rt.KeepsArg(unsafe.Pointer(view), unsafe.Pointer(m)) {
			t.Error("model not kept")
		}
		root.Delete()
		if rt.KeepsArg(unsafe.Pointer(view), unsafe.Pointer(m)) {
			t.Error("model still kept after the view died")
		}
	})
}

// wantDepPanic runs f and checks it panics naming method and the dead owner.
func wantDepPanic(t *testing.T, method, owner string, f func()) {
	t.Helper()
	defer func() {
		p := recover()
		msg, _ := p.(string)
		if !strings.Contains(msg, method) || !strings.Contains(msg, "the "+owner+" this") {
			t.Errorf("%s after its %s died: panic %v, want a message naming both", method, owner, p)
		}
	}()
	f()
}

// A text block holds a raw pointer into its document: the block keeps the
// document alive, and panics once it is deleted (it used to read freed memory
// and return "").
func TestTextBlockKeepsDocument(t *testing.T) {
	var b *qtgui.QTextBlock
	qutetest.Run(t, func() {
		b = qtgui.NewQTextDocumentWithText("hello\nworld", nil).Begin()
	})
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		if got := b.Text(); got != "hello" {
			t.Errorf("Text after GC = %q, want \"hello\"", got)
		}
		b.Document().Delete()
		wantDepPanic(t, "QTextBlock.Text", "QTextDocument", func() { b.Text() })
	})
}

// A table cell holds a raw pointer to its table: it keeps the document alive
// (it used to segfault in QTextTableCell::format) and panics once the table
// or the document is gone.
func TestTextTableCellKeepsTable(t *testing.T) {
	var c *qtgui.QTextTableCell
	qutetest.Run(t, func() {
		d := qtgui.NewQTextDocument(nil)
		c = qtgui.NewQTextCursorWithTextDocument(d).InsertTable(2, 2).CellAtWithCol(1, 1)
	})
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		if c.Row() != 1 || c.RowSpan() != 1 {
			t.Errorf("Row, RowSpan after GC = %d, %d, want 1, 1", c.Row(), c.RowSpan())
		}
		d := c.FirstCursorPosition().Document()
		d.Clear() // destroys the table, not the document
		wantDepPanic(t, "QTextTableCell.RowSpan", "QTextTable", func() { c.RowSpan() })
		d.Delete()
	})
	qutetest.Run(t, func() {
		d := qtgui.NewQTextDocument(nil)
		c := qtgui.NewQTextCursorWithTextDocument(d).InsertTable(1, 1).CellAtWithCol(0, 0)
		d.Delete()
		wantDepPanic(t, "QTextTableCell.Row", "QTextTable", func() { c.Row() })
	})
}

// A cell of a nested table depends on the inner table, not the outer one.
func TestTextTableCellFindsInnermostTable(t *testing.T) {
	qutetest.Run(t, func() {
		d := qtgui.NewQTextDocument(nil)
		defer d.Delete()
		outer := qtgui.NewQTextCursorWithTextDocument(d).InsertTable(2, 2)
		oc := outer.CellAtWithCol(0, 0)
		inner := oc.FirstCursorPosition().InsertTable(1, 1)
		ic := inner.CellAtWithCol(0, 0)
		oc = outer.CellAtWithCol(0, 0)
		depOf := func(x unsafe.Pointer) *rt.Handle {
			ds := rt.RefOf(x).H.Dependencies()
			if len(ds) != 2 {
				t.Errorf("%d dependencies, want the table and the document", len(ds))
				return nil
			}
			return ds[0]
		}
		if got := depOf(unsafe.Pointer(ic)); got != rt.RefOf(unsafe.Pointer(inner)).H {
			t.Error("inner cell does not depend on the inner table")
		}
		if got := depOf(unsafe.Pointer(oc)); got != rt.RefOf(unsafe.Pointer(outer)).H {
			t.Error("outer cell does not depend on the outer table")
		}
	})
}

// A widget item keeps a raw pointer to a widget it does not own.
func TestWidgetItemKeepsWidget(t *testing.T) {
	var it *qtwidgets.QWidgetItem
	var w weak.Pointer[qtwidgets.QWidget]
	qutetest.Run(t, func() {
		x := qtwidgets.NewQWidget(nil)
		x.SetMinimumSize(30, 20)
		w = weak.Make(x)
		it = qtwidgets.NewQWidgetItemWithWidget(x)
	})
	qutetest.CollectGarbage()
	if w.Value() == nil {
		t.Fatal("the widget was collected while the item pointed at it")
	}
	qutetest.Run(t, func() {
		// a parentless widget is a window, which an item reports as empty,
		// so read the widget back through the item
		if got := it.Widget().MinimumSize(); got.Width != 30 || got.Height != 20 {
			t.Errorf("Widget().MinimumSize after GC = %v, want 30x20", got)
		}
		it.Widget().Delete()
		wantDepPanic(t, "MinimumSize", "QWidget", func() { it.MinimumSize() })
		it.Delete()
	})
}

// QBuffer gives QDataStream an in-memory device to write to.
func TestDataStreamOverBuffer(t *testing.T) {
	qutetest.Run(t, func() {
		buf := qtcore.NewQBuffer(nil)
		defer buf.Delete()
		if !buf.Open(qtcore.QIODeviceBaseReadWrite) {
			t.Error("Open failed")
			return
		}
		ds := qtcore.NewQDataStreamWithIODevice(buf)
		defer ds.Delete()
		if n := ds.WriteRawData([]byte("hello")); n != 5 {
			t.Errorf("WriteRawData = %d, want 5", n)
		}
		if got := string(buf.Data()); got != "hello" {
			t.Errorf("Data = %q, want \"hello\"", got)
		}
		buf.SeekTo(0)
		got := make([]byte, 5)
		if n := ds.ReadRawData(got); n != 5 || string(got) != "hello" {
			t.Errorf("ReadRawData = %d %q, want 5 \"hello\"", n, got)
		}
	})
}

// fakeTB records the failure qutetest.Run reports instead of failing the test.
type fakeTB struct {
	testing.TB
	failed string
}

func (f *fakeTB) Helper()                      {}
func (f *fakeTB) Fatal(args ...any)            { f.failed = fmt.Sprint(args...) }
func (f *fakeTB) Fatalf(s string, args ...any) { f.failed = fmt.Sprintf(s, args...) }

// A panic in a signal handler is recovered where Qt called it, so Qt's frames
// return normally; Click re-raises it and Run reports it.
func TestHandlerPanicIsReported(t *testing.T) {
	ft := &fakeTB{TB: t}
	reached := false
	qutetest.Run(ft, func() {
		btn := qtwidgets.NewQPushButtonWithText("boom", nil)
		defer btn.Delete()
		btn.OnClicked(func(bool) { panic("boom from a handler") })
		qutetest.Click(btn)
		reached = true
	})
	if !strings.Contains(ft.failed, "boom from a handler") || !strings.Contains(ft.failed, "panic in a Qt callback") {
		t.Errorf("Run reported %q, want the handler's panic", ft.failed)
	}
	if reached {
		t.Error("code after Click ran; Click should re-raise the handler's panic")
	}
	// Qt is still usable afterwards.
	qutetest.Run(t, func() {
		clicks := 0
		btn := qtwidgets.NewQPushButtonWithText("ok", nil)
		defer btn.Delete()
		btn.OnClicked(func(bool) { clicks++ })
		qutetest.Click(btn)
		if clicks != 1 {
			t.Errorf("clicks = %d after a recovered panic, want 1", clicks)
		}
	})
}

// The destroyed signal hands its handler the dying object. That used to be
// wrapped as a new handle, registered while Qt destroyed the object, so it
// never learnt of the death: the next object Qt put at the address was given
// that stale handle, never watched, and used after it was freed (an
// application's window benchmark crashed). The handler now gets the handle Go
// had, Dead, and nothing is registered for the dead address.
func TestOnDestroyedArgument(t *testing.T) {
	qutetest.Run(t, func() {
		for range 50 {
			w := qtwidgets.NewQWidget(nil)
			child := qtcore.NewQObject(w)
			var gotChild, gotW *qtcore.QObject
			childAlive, wAlive := true, true
			child.OnDestroyed(func(o *qtcore.QObject) { gotChild, childAlive = o, o.Alive() })
			w.OnDestroyed(func(o *qtcore.QObject) { gotW, wAlive = o, o.Alive() })
			wp, cp := w.AsQObject().Ptr(), child.Ptr()
			w.Delete()
			if gotChild != child || gotW != w.AsQObject() {
				t.Fatalf("destroyed handlers got %p and %p, want the wrappers Go had (%p, %p)", gotChild, gotW, child, w.AsQObject())
			}
			if childAlive || wAlive {
				t.Fatalf("the dying object was alive in its destroyed handler (child %v, widget %v)", childAlive, wAlive)
			}
			if rt.Lookup(wp) != nil || rt.Lookup(cp) != nil {
				t.Fatal("a handle is registered for a destroyed object's address")
			}
			// objects Qt may put at the freed addresses are watched as usual
			for range 4 {
				o := qtcore.NewQObject(nil)
				c := qtcore.NewQObject(o)
				o.Delete()
				if c.Alive() {
					t.Fatal("a child of a deleted object is alive: its death notice was lost")
				}
			}
		}
	})
}

// An upcast to a secondary base (AsQPaintDevice, AsQSurface...) adds an
// offset worked out once in C++ instead of calling C++ each time. Calling a
// virtual through each upcast checks the offset lands on the base: a wrong
// one would read another vtable.
func TestSecondaryBaseUpcasts(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		defer w.Delete()
		w.Resize(30, 20)
		if pd := w.AsQPaintDevice(); pd.Width() != 30 || pd.Height() != 20 {
			t.Errorf("QWidget.AsQPaintDevice: %dx%d, want 30x20", pd.Width(), pd.Height())
		}
		lay := qtwidgets.NewQVBoxLayout(w)
		if got := lay.AsQLayoutItem().Layout(); got.Ptr() != lay.AsQLayout().Ptr() {
			t.Error("QLayout.AsQLayoutItem().Layout() is not the layout")
		}

		win := qtgui.NewQWindow(nil)
		defer win.Delete()
		if c := win.AsQSurface().SurfaceClass(); c != qtgui.QSurfaceWindow {
			t.Errorf("QWindow.AsQSurface: class %v, want Window", c)
		}
		rw := qtgui.NewQRasterWindow(nil)
		defer rw.Delete()
		rw.Resize(qtcore.QSize{Width: 12, Height: 7})
		if pd := rw.AsQPaintDevice(); pd.Width() != 12 {
			t.Errorf("QPaintDeviceWindow.AsQPaintDevice: width %d, want 12", pd.Width())
		}
		off := qtgui.NewQOffscreenSurfaceWithScreen(nil, nil)
		defer off.Delete()
		if c := off.AsQSurface().SurfaceClass(); c != qtgui.QSurfaceOffscreen {
			t.Errorf("QOffscreenSurface.AsQSurface: class %v, want Offscreen", c)
		}
		pdf := qtgui.NewQPdfWriterWithFilename(filepath.Join(t.TempDir(), "x.pdf"))
		defer pdf.Delete()
		if pd := pdf.AsQPagedPaintDevice(); pd.Width() <= 0 {
			t.Errorf("QPdfWriter.AsQPagedPaintDevice: width %d", pd.Width())
		}

		gw := qtwidgets.NewQGraphicsWidget(nil)
		defer gw.Delete()
		item := gw.AsQGraphicsItem()
		if o := item.ToGraphicsObject(); o == nil || o.Ptr() != gw.AsQGraphicsObject().Ptr() {
			t.Error("QGraphicsObject.AsQGraphicsItem().ToGraphicsObject() is not the object")
		}
		if gi := gw.AsQGraphicsLayoutItem().GraphicsItem(); gi == nil || gi.Ptr() != item.Ptr() {
			t.Error("QGraphicsWidget.AsQGraphicsLayoutItem().GraphicsItem() is not the item")
		}
	})
}
