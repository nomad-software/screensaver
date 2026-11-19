package qtgui_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// recorder is an event filter that hands every event to on.
type recorder struct {
	qtcore.QObjectBase
	on func(e *qtcore.QEvent)
}

func (r *recorder) EventFilter(watched *qtcore.QObject, e *qtcore.QEvent) bool {
	r.on(e)
	return false
}

func watch(o qtcore.QObjectLike, on func(e *qtcore.QEvent)) *qtcore.QObject {
	f := qtcore.NewQObjectWith(&recorder{on: on}, nil)
	o.AsQObject().InstallEventFilter(f)
	return f
}

func waitFor(done func() bool) bool {
	for range 300 {
		if done() {
			return true
		}
		qutetest.Wait(10)
	}
	return done()
}

func TestDrag(t *testing.T) {
	qutetest.Run(t, func() {
		src := qtcore.NewQObject(nil)
		d := qtgui.NewQDrag(src)
		if d.Ownership() != qtcore.Parented || d.Source() != src {
			t.Errorf("drag: %v, source %v", d.Ownership(), d.Source())
		}
		md := qtcore.NewQMimeData()
		md.SetText("payload")
		d.SetMimeData(md)
		if d.MimeData() != md || md.Ownership() != qtcore.Parented {
			t.Errorf("mime data: %v", md.Ownership())
		}
		md2 := qtcore.NewQMimeData()
		d.SetMimeData(md2)
		if md.Alive() {
			t.Error("replaced mime data was not deleted")
		}
		d.SetHotSpot(qtcore.QPoint{X: 3, Y: 4})
		d.SetPixmap(qtgui.NewQPixmapWithW(8, 8))
		if d.HotSpot() != (qtcore.QPoint{X: 3, Y: 4}) || d.Pixmap().Width() != 8 {
			t.Error("hot spot or pixmap")
		}
		d.SetDragCursor(qtgui.NewQPixmapWithW(4, 4), qtcore.CopyAction)
		if d.DragCursor(qtcore.CopyAction).Width() != 4 {
			t.Error("drag cursor")
		}
		// cancel the drag from inside its loop, should the platform start one
		cancel := qtcore.NewQTimer(nil)
		cancel.SetSingleShot(true)
		cancel.SetInterval(50)
		cancel.OnTimeout(qtgui.QDragCancel)
		cancel.Start()
		if got := d.Exec(); got != qtcore.IgnoreAction {
			t.Errorf("cancelled drag: %v, want IgnoreAction", got)
		}
		cancel.Delete()
		src.Delete()
		if d.Alive() || md2.Alive() {
			t.Error("drag and its data outlived the source")
		}
	})
}

func TestFileSystemModel(t *testing.T) {
	qutetest.Run(t, func() {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644)
		m := qtgui.NewQFileSystemModel(nil)
		loaded := ""
		m.OnDirectoryLoaded(func(p string) { loaded = p })
		provider := qtgui.NewQAbstractFileIconProvider()
		m.SetIconProvider(provider)
		if m.IconProvider() != provider {
			t.Error("icon provider")
		}
		root := m.SetRootPath(dir)
		if !waitFor(func() bool { return loaded == dir && m.RowCountFull(root) == 1 }) {
			t.Fatalf("directory not loaded: %q, %d rows", loaded, m.RowCountFull(root))
		}
		idx := m.IndexOfPath(filepath.Join(dir, "a.txt"))
		if !idx.IsValid() || m.FilePath(idx) != filepath.Join(dir, "a.txt") || m.IsDir(idx) || m.Size(idx) != 5 {
			t.Errorf("file index: %q, size %d", m.FilePath(idx), m.Size(idx))
		}
		sub := m.Mkdir(root, "sub")
		if !sub.IsValid() || !m.IsDir(sub) {
			t.Error("Mkdir")
		}
		if st, err := os.Stat(filepath.Join(dir, "sub")); err != nil || !st.IsDir() {
			t.Error("Mkdir did not create the directory")
		}
		m.Delete()
		provider.Delete()
	})
}

func TestFontMetricsF(t *testing.T) {
	qutetest.Run(t, func() {
		f := qtgui.NewQFont()
		m := qtgui.NewQFontMetricsFWithFont(f)
		if m.Height() <= 0 || m.HorizontalAdvance("WW") <= m.HorizontalAdvance("W") {
			t.Errorf("height %v, advance %v", m.Height(), m.HorizontalAdvance("WW"))
		}
		img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 4, Height: 4}, qtgui.QImageFormat_ARGB32)
		md := qtgui.NewQFontMetricsFWithFontPaintDevice(f, img)
		if md.Height() <= 0 {
			t.Error("metrics for a device")
		}
		if qtgui.NewQFontMetricsFWithFontMetrics(qtgui.NewQFontMetricsWithFont(f)).Ascent() <= 0 {
			t.Error("metrics from integer metrics")
		}
	})
}

func TestWriters(t *testing.T) {
	qutetest.Run(t, func() {
		img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 3, Height: 2}, qtgui.QImageFormat_ARGB32)
		img.FillColor(qtgui.RGB(10, 200, 30))
		buf := qtcore.NewQBuffer(nil)
		buf.Open(qtcore.QIODeviceBaseReadWrite)
		w := qtgui.NewQImageWriterWithIODevice(buf, []byte("png"))
		if !w.CanWrite() || !w.Write(img) {
			t.Fatalf("write: %s", w.ErrorString())
		}
		if string(w.Format()) != "png" || w.Device() != buf.AsQIODevice() {
			t.Error("writer state")
		}
		w.Delete()
		back := qtgui.NewQImage()
		if !back.LoadFromData(buf.Data()) || back.Width() != 3 || back.PixelColor(1, 1) != qtgui.RGB(10, 200, 30) {
			t.Errorf("read back %dx%d", back.Width(), back.Height())
		}
		buf.Delete()
		if !slices.ContainsFunc(qtgui.QImageWriterSupportedImageFormats(), func(f []byte) bool { return string(f) == "png" }) {
			t.Error("png is not a supported format")
		}

		doc := qtgui.NewQTextDocumentWithText("hello writer", nil)
		out := qtcore.NewQBuffer(nil)
		out.Open(qtcore.QIODeviceBaseReadWrite)
		tw := qtgui.NewQTextDocumentWriterWithIODevice(out, []byte("plaintext"))
		if !tw.Write(doc) {
			t.Fatal("text document write failed")
		}
		tw.Delete()
		if strings.TrimSpace(string(out.Data())) != "hello writer" {
			t.Errorf("plain text %q", out.Data())
		}
		out.Delete()
		path := filepath.Join(t.TempDir(), "doc.html")
		fw := qtgui.NewQTextDocumentWriterWithFileName(path)
		if !fw.Write(doc) || !slices.ContainsFunc(qtgui.QTextDocumentWriterSupportedDocumentFormats(), func(f []byte) bool { return string(f) == "HTML" }) {
			t.Error("HTML file write")
		}
		fw.Delete()
		if b, _ := os.ReadFile(path); !strings.Contains(string(b), "hello writer") {
			t.Error("HTML file lacks the text")
		}
		doc.Delete()
	})
}

func TestPainterHelpers(t *testing.T) {
	qutetest.Run(t, func() {
		path := qtgui.NewQPainterPath()
		path.MoveTo(qtcore.QPointF{X: 0, Y: 10})
		path.LineTo(qtcore.QPointF{X: 100, Y: 10})
		s := qtgui.NewQPainterPathStroker()
		s.SetWidth(4)
		s.SetCapStyle(qtcore.FlatCap)
		if s.Width() != 4 || s.CapStyle() != qtcore.FlatCap {
			t.Error("stroker settings")
		}
		r := s.CreateStroke(path).BoundingRect()
		if r.Height < 3.9 || r.Height > 4.1 || r.Width < 99 {
			t.Errorf("stroke bounds %v, want 100x4", r)
		}
		if qtgui.NewQPainterPathStrokerWithPen(qtgui.NewQPenWithColor(qtgui.RGB(0, 0, 0))).Width() != 1 {
			t.Error("stroker from a default pen")
		}

		img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 4, Height: 4}, qtgui.QImageFormat_ARGB32)
		p := qtgui.NewQPainterOn(img)
		red := qtgui.RGB(255, 0, 0)
		p.SetPen(qtgui.NewQPenWithColor(red))
		g := qtgui.NewQPainterStateGuardWithPainter(p)
		p.SetPen(qtgui.NewQPenWithColor(qtgui.RGB(0, 0, 255)))
		g.Restore()
		if p.Pen().Color() != red {
			t.Error("Restore did not restore the pen")
		}
		g.Save()
		p.SetPen(qtgui.NewQPenWithColor(qtgui.RGB(0, 0, 255)))
		g.Delete() // restores what it saved
		if p.Pen().Color() != red {
			t.Error("deleting the guard did not restore the pen")
		}
		p.End()
		p.Delete()
	})
}

func TestPixmapCache(t *testing.T) {
	qutetest.Run(t, func() {
		pm := qtgui.NewQPixmapWithW(5, 5)
		if !qtgui.QPixmapCacheInsertKey("qute-test", pm) {
			t.Fatal("insert failed")
		}
		found := qtgui.NewQPixmap()
		if !qtgui.QPixmapCacheFind("qute-test", found) || found.Width() != 5 {
			t.Error("find by name")
		}
		key := qtgui.QPixmapCacheInsert(qtgui.NewQPixmapWithW(7, 7))
		if !key.IsValid() || !qtgui.QPixmapCacheFindPixmapCacheKey(key, found) || found.Width() != 7 {
			t.Error("find by key")
		}
		qtgui.QPixmapCacheRemove("qute-test")
		qtgui.QPixmapCacheRemovePixmapCacheKey(key)
		if qtgui.QPixmapCacheFind("qute-test", found) || key.IsValid() {
			t.Error("removed pixmaps are still found")
		}
		if qtgui.NewQPixmapCacheKey().IsValid() || qtgui.QPixmapCacheCacheLimit() <= 0 {
			t.Error("default key or cache limit")
		}
		qtgui.QPixmapCacheClear()
	})
}

func TestRegularExpressionValidator(t *testing.T) {
	qutetest.Run(t, func() {
		v := qtgui.NewQRegularExpressionValidatorWithRegularExpression(qtcore.NewQRegularExpressionWithPattern(`\d{3}`), nil)
		for in, want := range map[string]qtgui.QValidatorState{"12": qtgui.QValidatorIntermediate, "123": qtgui.QValidatorAcceptable, "12a": qtgui.QValidatorInvalid} {
			if got, _, _ := v.Validate(in, 0); got != want {
				t.Errorf("%q: %v, want %v", in, got, want)
			}
		}
		changed := ""
		v.OnRegularExpressionChanged(func(re *qtcore.QRegularExpression) { changed = re.Pattern() })
		v.SetRegularExpression(qtcore.NewQRegularExpressionWithPattern("[a-z]+"))
		if changed != "[a-z]+" || v.RegularExpression().Pattern() != "[a-z]+" {
			t.Errorf("changed %q", changed)
		}
		v.Delete()
	})
}

// raster paints its window red.
type raster struct {
	qtgui.QRasterWindowBase
	painted bool
}

func (r *raster) PaintEvent(e *qtgui.QPaintEvent) {
	p := qtgui.NewQPainterOn(r)
	p.FillRect(e.Rect(), qtgui.RGB(255, 0, 0))
	p.End()
	p.Delete()
	r.painted = true
}

func TestRasterWindowAndSurfaceEvents(t *testing.T) {
	qutetest.Run(t, func() {
		r := &raster{}
		w := qtgui.NewQRasterWindowWith(r, nil)
		var surface []qtgui.QPlatformSurfaceEventSurfaceEventType
		var children []*qtgui.QWindow
		f := watch(w, func(e *qtcore.QEvent) {
			if s := qtgui.QPlatformSurfaceEventCast(e); s != nil {
				surface = append(surface, s.SurfaceEventType())
			}
			if c := qtgui.QChildWindowEventCast(e); c != nil && e.Type() == qtcore.QEventChildWindowAdded {
				children = append(children, c.Child())
			}
		})
		w.Resize(qtcore.QSize{Width: 20, Height: 20})
		w.Show()
		if !waitFor(func() bool { return r.painted }) {
			t.Error("the raster window was not painted")
		}
		if !slices.Contains(surface, qtgui.QPlatformSurfaceEventSurfaceCreated) {
			t.Errorf("surface events %v", surface)
		}
		child := qtgui.NewQWindow(w)
		if len(children) != 1 || children[0] != child {
			t.Errorf("child window events %v", children)
		}
		w.Delete()
		f.Delete()
	})
}

func TestDeliveredEvents(t *testing.T) {
	qutetest.Run(t, func() {
		// status tips
		target := qtcore.NewQObject(nil)
		tip := ""
		var kept *qtgui.QStatusTipEvent
		f := watch(target, func(e *qtcore.QEvent) {
			if s := qtgui.QStatusTipEventCast(e); s != nil {
				tip, kept = s.Tip(), s
			}
		})
		a := qtgui.NewQAction(nil)
		a.SetStatusTip("helpful")
		a.ShowStatusTextFull(target)
		if tip != "helpful" {
			t.Errorf("status tip %q", tip)
		}
		if kept == nil || kept.Alive() {
			t.Error("the cast status tip event outlived its handler")
		}
		a.Delete()
		f.Delete()
		target.Delete()

		// window state changes
		w := qtwidgets.NewQWidget(nil)
		var old qtcore.WindowStates = -1
		f = watch(w, func(e *qtcore.QEvent) {
			if s := qtgui.QWindowStateChangeEventCast(e); s != nil {
				old = s.OldState()
				if s.IsOverride() {
					t.Error("a plain state change is an override")
				}
			}
		})
		w.SetWindowState(qtcore.WindowMaximized)
		if old != qtcore.WindowNoState {
			t.Errorf("old state %v", old)
		}

		// shortcuts
		w.Show()
		w.ActivateWindow()
		qutetest.Wait(20)
		sc := qtgui.NewQShortcutWithKeySequence(qtgui.NewQKeySequenceWithKey("Ctrl+K"), w)
		var key string
		ambiguous := true
		g := watch(sc, func(e *qtcore.QEvent) {
			if s := qtgui.QShortcutEventCast(e); s != nil {
				key, ambiguous = s.Key().ToString(), s.IsAmbiguous()
			}
		})
		activated := false
		sc.OnActivated(func() { activated = true })
		qutetest.Key(w, qtcore.Key_K, qtcore.ControlModifier)
		if key != "Ctrl+K" || ambiguous || !activated {
			t.Errorf("shortcut event %q, ambiguous %v, activated %v", key, ambiguous, activated)
		}

		// a widget that takes focus is asked what input it accepts
		le := qtwidgets.NewQLineEdit(w)
		var queries qtcore.InputMethodQueries
		var answered int
		h := watch(le, func(e *qtcore.QEvent) {
			if q := qtgui.QInputMethodQueryEventCast(e); q != nil {
				queries |= q.Queries()
				q.SetValue(qtcore.ImHints, qtcore.NewQVariantOf(7))
				answered = q.Value(qtcore.ImHints).ToInt()
			}
		})
		le.Show()
		le.SetFocus()
		le.SetText("typed") // the input method state changes: Qt asks again
		if queries&qtcore.ImEnabled == 0 || answered != 7 {
			t.Errorf("queries %v, value round trip %d", queries, answered)
		}
		if got := qtgui.QInputMethodQueryFocusObject(qtcore.ImSurroundingText, qtcore.NewQVariant()); got.ToString() != "typed" {
			t.Errorf("focus object query %q", got.ToString())
		}
		w.Delete()
		for _, x := range []*qtcore.QObject{f, g, h} {
			x.Delete()
		}

		// events only some platforms send (macOS, touch pads, mobile): the
		// casts reject other events
		ev := qtcore.NewQObject(nil)
		checked := false
		f = watch(ev, func(e *qtcore.QEvent) {
			checked = true
			if qtgui.QApplicationStateChangeEventCast(e) != nil || qtgui.QFileOpenEventCast(e) != nil ||
				qtgui.QIconDragEventCast(e) != nil || qtgui.QNativeGestureEventCast(e) != nil ||
				qtgui.QScreenOrientationChangeEventCast(e) != nil || qtgui.QToolBarChangeEventCast(e) != nil ||
				qtgui.QWhatsThisClickedEventCast(e) != nil || qtgui.QScrollEventCast(e) != nil || qtgui.QScrollPrepareEventCast(e) != nil {
				t.Errorf("event %v cast to an unrelated class", e.Type())
			}
		})
		ev.SetProperty("x", qtcore.NewQVariantOf(1))
		if !checked {
			t.Error("no event reached the filter")
		}
		ev.Delete()
		f.Delete()
		if qtgui.QStatusTipEventCast(nil) != nil {
			t.Error("cast of nil")
		}
	})
}

func TestScaledPixmapArgument(t *testing.T) {
	qutetest.Run(t, func() {
		a := qtgui.NewQIconEngineScaledPixmapArgument()
		a.SetSize(qtcore.QSize{Width: 16, Height: 16})
		a.SetMode(qtgui.QIconDisabled)
		a.SetState(qtgui.QIconOn)
		a.SetScale(2)
		a.SetPixmap(qtgui.NewQPixmapWithW(3, 3))
		if a.Size() != (qtcore.QSize{Width: 16, Height: 16}) || a.Mode() != qtgui.QIconDisabled || a.State() != qtgui.QIconOn || a.Scale() != 2 || a.Pixmap().Width() != 3 {
			t.Error("scaled pixmap argument fields")
		}
	})
}

func TestDesktopServices(t *testing.T) {
	qutetest.Run(t, func() {
		// an invalid URL is refused before any program is started
		if qtgui.QDesktopServicesOpenUrl(qtcore.NewQUrl()) {
			t.Error("opened an invalid URL")
		}
	})
}
