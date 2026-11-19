package qute_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// Subclassing is on for every class Qt lets Go subclass. These tests cover
// classes and shapes that needed a spec entry, or were impossible, before:
// views, validators, graphics items, models returning maps, protected enums,
// abstract classes with no override at all, and finding the Go value again.

type keyTree struct {
	qtwidgets.QTreeViewBase
	keys []qtcore.Key
}

func (v *keyTree) KeyPressEvent(e *qtgui.QKeyEvent) {
	v.keys = append(v.keys, qtcore.Key(e.Key()))
	v.Super().KeyPressEvent(e)
}

// A QTreeView subclass gets its events, and its Base reaches the protected
// methods and enums of QAbstractItemView.
func TestSubclassTreeView(t *testing.T) {
	qutetest.Run(t, func() {
		v := &keyTree{}
		qtwidgets.NewQTreeViewWith(v, nil)
		defer v.Delete()
		qutetest.Key(v, qtcore.Key_Down, 0)
		if len(v.keys) != 1 || v.keys[0] != qtcore.Key_Down {
			t.Errorf("KeyPressEvent saw %v, want [Key_Down]", v.keys)
		}
		v.SetState(qtwidgets.QAbstractItemViewEditingState)
		if got := v.State(); got != qtwidgets.QAbstractItemViewEditingState {
			t.Errorf("State() = %v after SetState(EditingState)", got)
		}
		v.SetState(qtwidgets.QAbstractItemViewNoState)
	})
}

type upperValidator struct {
	qtgui.QValidatorBase
	fixed int
}

// Validate upper-cases what is typed: the in-out string goes back to Qt.
func (u *upperValidator) Validate(input string, pos int) (qtgui.QValidatorState, string, int) {
	return qtgui.QValidatorAcceptable, strings.ToUpper(input), pos
}

func (u *upperValidator) Fixup(input string) string {
	u.fixed++
	return strings.TrimSpace(input)
}

// An override with in-out parameters (validate(QString &, int &)) changes
// what Qt sees, through a QLineEdit and through the binding.
func TestSubclassValidatorInOut(t *testing.T) {
	qutetest.Run(t, func() {
		edit := qtwidgets.NewQLineEdit(nil)
		defer edit.Delete()
		u := &upperValidator{}
		qtgui.NewQValidatorWith(u, edit)
		edit.SetValidator(u)
		qutetest.Type(edit, "abc")
		if got := edit.Text(); got != "ABC" {
			t.Errorf("text %q, want the validator's ABC", got)
		}
		// the binding dispatches virtually, so it reaches the override too
		state, s, pos := u.QValidator.Validate("xyz", 2)
		if state != qtgui.QValidatorAcceptable || s != "XYZ" || pos != 2 {
			t.Errorf("Validate = %v, %q, %d", state, s, pos)
		}
		if got := u.QValidator.Fixup("  a  "); got != "a" || u.fixed != 1 {
			t.Errorf("Fixup = %q (called %d times)", got, u.fixed)
		}
	})
}

type evenValidator struct{ qtgui.QIntValidatorBase }

// Validate accepts only even numbers, on top of QIntValidator's own check.
func (e *evenValidator) Validate(input string, pos int) (qtgui.QValidatorState, string, int) {
	state, input, pos := e.Super().Validate(input, pos)
	if state == qtgui.QValidatorAcceptable && (input[len(input)-1]-'0')%2 != 0 {
		state = qtgui.QValidatorIntermediate
	}
	return state, input, pos
}

// Super() of an in-out virtual calls the C++ implementation and returns its
// results.
func TestSubclassValidatorSuper(t *testing.T) {
	qutetest.Run(t, func() {
		e := &evenValidator{}
		qtgui.NewQIntValidatorWith(e, nil)
		defer e.Delete()
		for in, want := range map[string]qtgui.QValidatorState{
			"12": qtgui.QValidatorAcceptable, "13": qtgui.QValidatorIntermediate, "1x": qtgui.QValidatorInvalid,
		} {
			if got, _, _ := e.QValidator.Validate(in, 0); got != want {
				t.Errorf("Validate(%q) = %v, want %v", in, got, want)
			}
		}
	})
}

type roleModel struct{ qtgui.QStandardItemModelBase }

func (m *roleModel) RoleNames() map[int][]byte {
	return map[int][]byte{int(qtcore.DisplayRole): []byte("title"), int(qtcore.UserRole): []byte("sender")}
}

func (m *roleModel) ItemData(index *qtcore.QModelIndex) map[int]*qtcore.QVariant {
	return map[int]*qtcore.QVariant{int(qtcore.UserRole): qtcore.NewQVariantWithInt(index.Row() + 40)}
}

// Maps come back from an override (QHash<int, QByteArray>, QMap<int, QVariant>).
func TestSubclassMapResults(t *testing.T) {
	qutetest.Run(t, func() {
		m := &roleModel{}
		qtgui.NewQStandardItemModelWith(m, nil)
		defer m.Delete()
		m.AppendRow(qtgui.NewQStandardItemWithText("a"))
		m.AppendRow(qtgui.NewQStandardItemWithText("b"))
		names := m.QAbstractItemModel.RoleNames()
		if string(names[int(qtcore.UserRole)]) != "sender" || string(names[int(qtcore.DisplayRole)]) != "title" || len(names) != 2 {
			t.Errorf("RoleNames = %v", names)
		}
		data := m.QAbstractItemModel.ItemData(m.Index(1, 0))
		if v := data[int(qtcore.UserRole)]; len(data) != 1 || v == nil || v.ToInt() != 41 {
			t.Errorf("ItemData = %v", data)
		}
	})
}

type redItem struct {
	qtwidgets.QGraphicsItemBase
	painted int
}

func (r *redItem) BoundingRect() qtcore.QRectF { return qtcore.QRectF{Width: 20, Height: 20} }

func (r *redItem) Paint(p *qtgui.QPainter, _ *qtwidgets.QStyleOptionGraphicsItem, _ *qtwidgets.QWidget) {
	r.painted++
	p.FillRectF(r.BoundingRect(), qtgui.RGB(220, 0, 0))
}

// A Go QGraphicsItem (abstract in C++, so only built through its shim)
// paints into a scene, comes back from the scene as itself, and dies with it.
func TestSubclassGraphicsItem(t *testing.T) {
	qutetest.Run(t, func() {
		scene := qtwidgets.NewQGraphicsScene(nil)
		item := &redItem{}
		qtwidgets.NewQGraphicsItemWith(item, nil)
		scene.AddItem(item)
		if got := item.Ownership(); got != qtcore.Parented {
			t.Errorf("item after AddItem: %v, want Parented", got)
		}
		img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 20, Height: 20}, qtgui.QImageFormat_ARGB32)
		img.FillColor(qtgui.RGB(255, 255, 255))
		p := qtgui.NewQPainterOn(img)
		scene.Render(p)
		p.End()
		p.Delete()
		if c := img.PixelColor(10, 10); item.painted == 0 || c.R < 200 || c.G > 20 {
			t.Errorf("painted %d times, centre pixel %v, want red", item.painted, c)
		}
		items := scene.Items()
		if len(items) != 1 {
			t.Fatalf("scene has %d items", len(items))
		}
		if got, ok := items[0].GoValue().(*redItem); !ok || got != item {
			t.Errorf("GoValue of the scene's item = %v, want the Go item", items[0].GoValue())
		}
		scene.Delete()
		if item.Alive() {
			t.Error("the Go item is alive after its scene was deleted")
		}
	})
}

type plainDelegate struct {
	qtwidgets.QStyledItemDelegateBase
}

type blockState struct {
	qtgui.QTextBlockUserDataBase
	words int
}

// GoValue finds the Go value behind a pointer Qt hands back: a view's
// delegate, a block's user data. An object built without one has none.
func TestGoValue(t *testing.T) {
	qutetest.Run(t, func() {
		view := qtwidgets.NewQListView(nil)
		defer view.Delete()
		d := &plainDelegate{}
		qtwidgets.NewQStyledItemDelegateWith(d, view)
		view.SetItemDelegate(d)
		if got, ok := view.ItemDelegate().GoValue().(*plainDelegate); !ok || got != d {
			t.Errorf("ItemDelegate().GoValue() = %v, want the Go delegate", view.ItemDelegate().GoValue())
		}
		if v := qtwidgets.NewQStyledItemDelegate(view).GoValue(); v != nil {
			t.Errorf("a plain delegate has Go value %v", v)
		}
		doc := qtgui.NewQTextDocument(nil)
		defer doc.Delete()
		doc.SetPlainText("one two three")
		bs := &blockState{words: 3}
		qtgui.NewQTextBlockUserDataWith(bs)
		doc.FirstBlock().SetUserData(bs)
		if got, ok := doc.FirstBlock().UserData().GoValue().(*blockState); !ok || got.words != 3 {
			t.Errorf("UserData().GoValue() = %v, want the block state", doc.FirstBlock().UserData().GoValue())
		}
	})
}

type wordHighlighter struct {
	qtgui.QSyntaxHighlighterBase
	blocks []string
}

// HighlightBlock bolds the first word and records the block's word count as
// Go state on the block.
func (h *wordHighlighter) HighlightBlock(text string) {
	h.blocks = append(h.blocks, text)
	f := qtgui.NewQTextCharFormat()
	f.SetFontWeight(int(qtgui.QFontBold))
	h.SetFormat(0, strings.IndexByte(text+" ", ' '), f)
	st := &blockState{words: len(strings.Fields(text))}
	qtgui.NewQTextBlockUserDataWith(st)
	h.SetCurrentBlockUserData(st)
}

// A Go QSyntaxHighlighter (abstract: highlightBlock is pure) runs for each
// block and hands its per-block state to the document, which frees it.
func TestSubclassSyntaxHighlighter(t *testing.T) {
	qutetest.Run(t, func() {
		doc := qtgui.NewQTextDocument(nil)
		h := &wordHighlighter{}
		qtgui.NewQSyntaxHighlighterWith(h, doc) // the QObject constructor: parented, but not attached
		h.SetDocument(doc)
		doc.SetPlainText("hello world\nsecond line here")
		h.blocks = nil
		h.Rehighlight() // Qt otherwise highlights the new text from a deferred call
		if len(h.blocks) != 2 {
			t.Fatalf("HighlightBlock ran for %q", h.blocks)
		}
		second := doc.FirstBlock().Next()
		st, ok := second.UserData().GoValue().(*blockState)
		if !ok || st.words != 3 {
			t.Fatalf("second block's state = %v", second.UserData().GoValue())
		}
		if got := st.Ownership(); got != qtcore.Parented {
			t.Errorf("block state: %v, want Parented (the document owns it)", got)
		}
		doc.Delete()
		if st.Alive() || h.Alive() {
			t.Errorf("after the document died: state alive %v, highlighter alive %v", st.Alive(), h.Alive())
		}
	})
}

type emptyLayout struct{ qtwidgets.QLayoutBase }

// An abstract class whose pure virtuals Go leaves alone returns their zero
// value (itemAt, takeAt: nullptr; count: 0), so Qt can use it.
func TestSubclassPureVirtualDefaults(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		defer w.Delete()
		l := &emptyLayout{}
		qtwidgets.NewQLayoutWith(l, w)
		w.Resize(100, 50)
		w.Show()
		qute.ProcessEvents()
		if l.Count() != 0 || l.ItemAt(0) != nil || l.TakeAt(0) != nil {
			t.Errorf("Count %d, ItemAt %v, TakeAt %v, want the zero values", l.Count(), l.ItemAt(0), l.TakeAt(0))
		}
		w.Hide()
	})
}

type hintCanvas struct {
	qtwidgets.QWidgetBase
	hints int
}

func (c *hintCanvas) SizeHint() qtcore.QSize {
	c.hints++
	return qtcore.QSize{Width: 123, Height: 45}
}

// fancyCanvas gets SizeHint from the user type it embeds.
type fancyCanvas struct{ hintCanvas }

type deepCanvas struct {
	fancyCanvas
	extra int
}

type plainCanvas struct{ qtwidgets.QWidgetBase }

// An override declared on a user type that another user type embeds is still
// an override: it was once taken for a promoted generated method and ignored
// without a word. A type that defines nothing keeps the C++ implementation.
func TestSubclassOverrideThroughEmbedding(t *testing.T) {
	qutetest.Run(t, func() {
		f := &fancyCanvas{}
		w := qtwidgets.NewQWidgetWith(f, nil)
		defer w.Delete()
		if got := w.SizeHint(); got.Width != 123 || f.hints != 1 {
			t.Errorf("embedded override: SizeHint = %v after %d calls, want 123x45 after 1", got, f.hints)
		}
		d := &deepCanvas{}
		dw := qtwidgets.NewQWidgetWith(d, nil)
		defer dw.Delete()
		if got := dw.SizeHint(); got.Width != 123 || d.hints != 1 {
			t.Errorf("override two embeddings deep: SizeHint = %v after %d calls", got, d.hints)
		}
		p := &plainCanvas{}
		pw := qtwidgets.NewQWidgetWith(p, nil)
		defer pw.Delete()
		if got := pw.SizeHint(); got.Width == 123 {
			t.Errorf("no override: SizeHint = %v", got)
		}
	})
}

// swatch is a Go icon engine that paints one colour.
type swatch struct {
	qtgui.QIconEngineBase
	color  qtgui.QColor
	clones int
}

func (s *swatch) Paint(p *qtgui.QPainter, r qtcore.QRect, mode qtgui.QIconMode, state qtgui.QIconState) {
	p.FillRect(r, s.color)
}

func (s *swatch) Clone() *qtgui.QIconEngine {
	s.clones++
	return qtgui.NewQIconEngineWith(&swatch{color: s.color})
}

func (s *swatch) Key() string { return "swatch" }

// A QIcon draws through a Go engine, owns it, and frees it with its last
// copy; Clone's result belongs to whoever asked for it (transfer-to-go).
func TestGoIconEngine(t *testing.T) {
	qutetest.Run(t, func() {
		red := qtgui.RGB(220, 20, 20)
		sw := &swatch{color: red}
		e := qtgui.NewQIconEngineWith(sw)
		icon := qtgui.NewQIconWithIconEngine(e)
		if e.Ownership() != qtcore.Parented {
			t.Errorf("engine in an icon: %v, want Parented", e.Ownership())
		}
		img := icon.Pixmap(qtcore.QSize{Width: 8, Height: 8}).ToImage()
		if img.PixelColor(4, 4) != red {
			t.Errorf("icon pixel %v, want %v", img.PixelColor(4, 4), red)
		}
		if e.Key() != "swatch" || e.GoValue() != sw {
			t.Error("Key or GoValue does not reach the Go engine")
		}
		c := e.Clone()
		if c == nil || c.Ownership() != qtcore.Owned || sw.clones != 1 {
			t.Fatalf("clone: %v, %d clones", c, sw.clones)
		}
		if _, ok := c.GoValue().(*swatch); !ok {
			t.Error("the clone is not the Go engine Clone returned")
		}
		c.Delete()
		icon.Delete()
		if e.Alive() {
			t.Error("engine outlived its last icon")
		}
	})
}

type cloneless struct{ qtgui.QIconEngineBase }

func (c *cloneless) Paint(p *qtgui.QPainter, r qtcore.QRect, mode qtgui.QIconMode, state qtgui.QIconState) {
}

// Qt clones an icon's engine when a copy of the icon is changed and uses the
// result unchecked, so the zero value a missing pure virtual returns would
// crash there (it did, in QIcon::addPixmap). A Go engine must define Clone:
// its constructor says so instead.
func TestGoIconEngineNeedsClone(t *testing.T) {
	qutetest.Run(t, func() {
		defer func() {
			msg := fmt.Sprint(recover())
			for _, want := range []string{"NewQIconEngineWith", "cloneless does not define Clone", "define Clone on the type"} {
				if !strings.Contains(msg, want) {
					t.Errorf("panic %q, want it to contain %q", msg, want)
				}
			}
		}()
		qtgui.NewQIconEngineWith(&cloneless{})
	})
}

// A copy of an icon with a Go engine that is then changed gets its own
// engine from Clone, owned by the copy.
func TestGoIconEngineClonedOnDetach(t *testing.T) {
	qutetest.Run(t, func() {
		sw := &swatch{color: qtgui.RGB(1, 2, 3)}
		icon := qtgui.NewQIconWithIconEngine(qtgui.NewQIconEngineWith(sw))
		a := qtgui.NewQAction(nil)
		a.SetIcon(icon)
		cp := a.Icon() // shares the engine
		cp.AddPixmap(qtgui.NewQPixmapWithW(4, 4))
		if sw.clones != 1 {
			t.Errorf("%d clones after changing a copy, want 1", sw.clones)
		}
		cp.Delete()
		a.Delete()
		icon.Delete()
	})
}

// clickRecognizer turns a mouse press into a finished gesture.
type clickRecognizer struct {
	qtwidgets.QGestureRecognizerBase
	created int
}

func (r *clickRecognizer) Create(target *qtcore.QObject) *qtwidgets.QGesture {
	r.created++
	return qtwidgets.NewQGesture(nil)
}

func (r *clickRecognizer) Recognize(g *qtwidgets.QGesture, watched *qtcore.QObject, e *qtcore.QEvent) qtwidgets.QGestureRecognizerResult {
	if e.Type() == qtcore.QEventMouseButtonPress {
		return qtwidgets.QGestureRecognizerFinishGesture
	}
	return qtwidgets.QGestureRecognizerIgnore
}

// gestureWidget records the gesture events it receives.
type gestureWidget struct {
	qtwidgets.QWidgetBase
	typ   qtcore.GestureType
	got   []qtcore.GestureState
	event *qtwidgets.QGestureEvent
}

func (w *gestureWidget) Event(e *qtcore.QEvent) bool {
	if ge := qtwidgets.QGestureEventCast(e); ge != nil {
		w.event = ge
		if g := ge.Gesture(w.typ); g != nil {
			w.got = append(w.got, g.State())
			ge.SetWidget(w.QWidget) // Qt fills it in only for graphics views
			if g.GestureType() != w.typ || len(ge.Gestures()) != 1 || ge.Widget() != w.QWidget {
				panic(fmt.Sprintf("gesture event: type %v (want %v), %d gestures, widget %v", g.GestureType(), w.typ, len(ge.Gestures()), ge.Widget()))
			}
			ge.Accept(g)
		}
		return true
	}
	return w.Super().Event(e)
}

// A Go recognizer, registered with Qt, turns clicks into gesture events for
// widgets that grab its gesture type.
func TestGoGestureRecognizer(t *testing.T) {
	qutetest.Run(t, func() {
		rec := &clickRecognizer{}
		r := qtwidgets.NewQGestureRecognizerWith(rec)
		typ := qtwidgets.QGestureRecognizerRegisterRecognizer(r)
		if r.Ownership() != qtcore.Parented {
			t.Errorf("registered recognizer: %v, want Parented", r.Ownership())
		}
		gw := &gestureWidget{typ: typ}
		w := qtwidgets.NewQWidgetWith(gw, nil)
		w.Resize(40, 40)
		w.GrabGesture(typ)
		w.Show()
		qutetest.Click(w)
		if rec.created == 0 || len(gw.got) == 0 || gw.got[len(gw.got)-1] != qtcore.GestureFinished {
			t.Errorf("created %d gestures, states %v", rec.created, gw.got)
		}
		if gw.event == nil || gw.event.Alive() {
			t.Error("the cast gesture event outlived its handler")
		}
		g := r.Create(nil) // from Go: the caller owns the result
		if g == nil || g.Ownership() != qtcore.Owned {
			t.Errorf("Create from Go: %v", g)
		}
		g.Delete()
		w.Delete()
		qtwidgets.QGestureRecognizerUnregisterRecognizer(typ)
	})
}
