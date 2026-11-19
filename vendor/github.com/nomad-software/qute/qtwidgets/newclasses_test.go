package qtwidgets_test

import (
	"slices"
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// recorder is an event filter that hands every event to on.
type recorder struct {
	qtcore.QObjectBase
	on func(e *qtcore.QEvent) bool
}

func (r *recorder) EventFilter(watched *qtcore.QObject, e *qtcore.QEvent) bool { return r.on(e) }

func watch(o qtcore.QObjectLike, on func(e *qtcore.QEvent) bool) *qtcore.QObject {
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

// namePage is a wizard page with a mandatory field.
type namePage struct {
	qtwidgets.QWizardPageBase
	edit *qtwidgets.QLineEdit
}

func TestWizard(t *testing.T) {
	qutetest.Run(t, func() {
		wiz := qtwidgets.NewQWizard(nil)
		np := &namePage{}
		first := qtwidgets.NewQWizardPageWith(np, nil)
		first.SetTitle("Name")
		np.edit = qtwidgets.NewQLineEdit(first)
		np.RegisterField("name*", np.edit) // * makes it mandatory
		second := qtwidgets.NewQWizardPage(nil)
		second.SetFinalPage(true)
		if wiz.AddPage(first) != 0 || wiz.AddPage(second) != 1 || first.Ownership() != qtcore.Parented {
			t.Fatalf("pages: %v, ids %v", first.Ownership(), wiz.PageIds())
		}
		var ids []int
		wiz.OnCurrentIdChanged(func(id int) { ids = append(ids, id) })
		wiz.Show()
		if wiz.CurrentPage() != first || np.Wizard() != wiz || first.IsComplete() {
			t.Errorf("start: current %v, complete %v", wiz.CurrentPage(), first.IsComplete())
		}
		np.edit.SetText("bob")
		if !first.IsComplete() || wiz.Field("name").ToString() != "bob" {
			t.Errorf("field %q, complete %v", wiz.Field("name").ToString(), first.IsComplete())
		}
		wiz.Next()
		if wiz.CurrentId() != 1 || wiz.Page(1) != second || !wiz.HasVisitedPage(0) || !slices.Equal(ids, []int{0, 1}) {
			t.Errorf("after Next: id %d, ids %v", wiz.CurrentId(), ids)
		}
		wiz.Back()
		if wiz.CurrentId() != 0 {
			t.Errorf("after Back: id %d", wiz.CurrentId())
		}
		next := wiz.Button(qtwidgets.QWizardNextButton)
		wiz.SetButtonText(qtwidgets.QWizardNextButton, "Onward")
		if next == nil || next.Text() != "Onward" {
			t.Error("button text")
		}
		custom := qtwidgets.NewQPushButtonWithText("custom", nil)
		wiz.SetButton(qtwidgets.QWizardCustomButton1, custom)
		if custom.Ownership() != qtcore.Parented || wiz.Button(qtwidgets.QWizardCustomButton1) != custom.AsQAbstractButton() {
			t.Errorf("custom button: %v", custom.Ownership())
		}
		side := qtwidgets.NewQLabelWithText("side", nil)
		wiz.SetSideWidget(side)
		if wiz.SideWidget() != side.AsQWidget() {
			t.Error("side widget")
		}
		wiz.Delete()
		if first.Alive() || custom.Alive() {
			t.Error("pages or buttons outlived the wizard")
		}
	})
}

// columns counts the column views a QColumnView asks Go for.
type columns struct {
	qtwidgets.QColumnViewBase
	made int
}

func (c *columns) CreateColumn(root *qtcore.QModelIndex) *qtwidgets.QAbstractItemView {
	c.made++
	return qtwidgets.NewQListView(nil).AsQAbstractItemView()
}

func TestColumnViewAndMapper(t *testing.T) {
	qutetest.Run(t, func() {
		m := qtcore.NewQStringListModelWithStrings([]string{"a", "b"}, nil)
		defer m.Delete()
		c := &columns{}
		cv := qtwidgets.NewQColumnViewWith(c, nil)
		cv.SetModel(m)
		preview := qtwidgets.NewQLabelWithText("preview", nil)
		cv.SetPreviewWidget(preview)
		if cv.PreviewWidget() != preview.AsQWidget() || preview.Ownership() != qtcore.Parented {
			t.Errorf("preview: %v", preview.Ownership())
		}
		cv.SetResizeGripsVisible(false)
		cv.Show()
		if c.made == 0 || cv.ResizeGripsVisible() {
			t.Errorf("the Go CreateColumn ran %d times", c.made)
		}
		cv.Delete()
		if preview.Alive() {
			t.Error("preview widget outlived its view")
		}

		mp := qtwidgets.NewQDataWidgetMapper(nil)
		mp.SetModel(m)
		le := qtwidgets.NewQLineEdit(nil)
		mp.AddMapping(le, 0)
		mp.ToFirst()
		if le.Text() != "a" || mp.MappedWidgetAt(0) != le.AsQWidget() || mp.MappedSection(le) != 0 || mp.Model() != m.AsQAbstractItemModel() {
			t.Errorf("first row: %q", le.Text())
		}
		mp.ToNext()
		le.SetText("B")
		if !mp.Submit() || m.Data(m.Index(1, 0)).ToString() != "B" || mp.CurrentIndex() != 1 {
			t.Errorf("submitted %q", m.Data(m.Index(1, 0)).ToString())
		}
		d := qtwidgets.NewQStyledItemDelegate(nil)
		mp.SetItemDelegate(d)
		if mp.ItemDelegate() != d.AsQAbstractItemDelegate() {
			t.Error("item delegate")
		}
		mp.RemoveMapping(le)
		if mp.MappedSection(le) != -1 {
			t.Error("mapping not removed")
		}
		mp.Delete()
		d.Delete()
		le.Delete()
	})
}

// paints counts the items a Go QItemDelegate draws.
type paints struct {
	qtwidgets.QItemDelegateBase
	n int
}

func (p *paints) Paint(painter *qtgui.QPainter, option *qtwidgets.QStyleOptionViewItem, index *qtcore.QModelIndex) {
	p.n++
	p.Super().Paint(painter, option, index)
}

func TestItemDelegate(t *testing.T) {
	qutetest.Run(t, func() {
		m := qtcore.NewQStringListModelWithStrings([]string{"a", "b", "c"}, nil)
		defer m.Delete()
		view := qtwidgets.NewQListView(nil)
		view.SetModel(m)
		p := &paints{}
		d := qtwidgets.NewQItemDelegateWith(p, view)
		view.SetItemDelegate(d)
		d.SetClipping(false)
		f := qtwidgets.NewQItemEditorFactory()
		d.SetItemEditorFactory(f)
		if d.HasClipping() || d.ItemEditorFactory() != f {
			t.Error("delegate settings")
		}
		view.Resize(100, 100)
		view.Grab()
		if p.n < 3 {
			t.Errorf("painted %d items, want 3", p.n)
		}
		view.Delete()
		f.Delete()
	})
}

func TestGraphicsAnchorsAndAnimation(t *testing.T) {
	qutetest.Run(t, func() {
		form := qtwidgets.NewQGraphicsWidget(nil)
		lay := qtwidgets.NewQGraphicsAnchorLayout(nil)
		form.SetLayout(lay)
		a, b := qtwidgets.NewQGraphicsWidget(nil), qtwidgets.NewQGraphicsWidget(nil)
		a.AsQGraphicsLayoutItem().SetMinimumSize(qtcore.QSizeF{Width: 20, Height: 20})
		b.AsQGraphicsLayoutItem().SetMinimumSize(qtcore.QSizeF{Width: 20, Height: 20})
		lay.AddCornerAnchors(lay, qtcore.TopLeftCorner, a, qtcore.TopLeftCorner)
		anchor := lay.AddAnchor(a, qtcore.AnchorRight, b, qtcore.AnchorLeft)
		lay.AddAnchor(a, qtcore.AnchorTop, b, qtcore.AnchorTop)
		lay.AddAnchorsFull(lay, b, qtcore.Vertical)
		anchor.SetSpacing(15)
		if anchor.Spacing() != 15 || lay.Anchor(a, qtcore.AnchorRight, b, qtcore.AnchorLeft) != anchor {
			t.Errorf("anchor spacing %v", anchor.Spacing())
		}
		if a.Ownership() != qtcore.Parented || b.Ownership() != qtcore.Parented {
			t.Errorf("items: %v %v, want Parented", a.Ownership(), b.Ownership())
		}
		form.SetGeometry(qtcore.QRectF{Width: 200, Height: 100})
		lay.Activate()
		ag, bg := a.AsQGraphicsLayoutItem().Geometry(), b.AsQGraphicsLayoutItem().Geometry()
		if got := bg.X - (ag.X + ag.Width); got != 15 {
			t.Errorf("gap between the items %v, want 15", got)
		}
		lay.SetHorizontalSpacing(3)
		if lay.HorizontalSpacing() != 3 {
			t.Error("horizontal spacing")
		}
		form.Delete()
		if a.Alive() || b.Alive() || anchor.Alive() {
			t.Error("items or anchors outlived the form")
		}

		item := qtwidgets.NewQGraphicsRectItem(nil)
		tl := qtcore.NewQTimeLineWithDuration(100, nil)
		anim := qtwidgets.NewQGraphicsItemAnimation(nil)
		anim.SetItem(item)
		anim.SetTimeLine(tl)
		anim.SetPosAt(0, qtcore.QPointF{})
		anim.SetPosAt(1, qtcore.QPointF{X: 100})
		anim.SetRotationAt(1, 90)
		anim.SetStep(0.5)
		if item.Pos().X != 50 || anim.PosAt(0.5).X != 50 || anim.RotationAt(0.5) != 45 || len(anim.PosList()) != 2 {
			t.Errorf("half way: pos %v, rotation %v", item.Pos(), anim.RotationAt(0.5))
		}
		if anim.Item() != item.AsQGraphicsItem() || anim.TimeLine() != tl {
			t.Error("item or time line")
		}
		anim.Delete()
		tl.Delete()
		item.Delete()
	})
}

func TestScroller(t *testing.T) {
	qutetest.Run(t, func() {
		target := qtwidgets.NewQWidget(nil)
		target.Resize(100, 100)
		target.Show()
		var positions []qtcore.QPointF
		var prepared bool
		f := watch(target, func(e *qtcore.QEvent) bool {
			if p := qtgui.QScrollPrepareEventCast(e); p != nil {
				prepared = true
				p.SetViewportSize(qtcore.QSizeF{Width: 100, Height: 100})
				p.SetContentPosRange(qtcore.QRectF{Width: 0, Height: 1000})
				p.SetContentPos(qtcore.QPointF{})
				if p.ViewportSize().Width != 100 || p.ContentPosRange().Height != 1000 {
					t.Error("prepare event setters")
				}
				p.Accept()
				return true
			}
			if s := qtgui.QScrollEventCast(e); s != nil {
				positions = append(positions, s.ContentPos())
				return true
			}
			return false
		})
		sc := qtwidgets.QScrollerScroller(target)
		if !qtwidgets.QScrollerHasScroller(target) || sc.Target() != target.AsQObject() {
			t.Fatal("no scroller for the target")
		}
		props := qtwidgets.NewQScrollerProperties()
		props.SetScrollMetric(qtwidgets.QScrollerPropertiesDecelerationFactor, qtcore.NewQVariantOf(0.5))
		sc.SetScrollerProperties(props)
		if sc.ScrollerProperties().ScrollMetric(qtwidgets.QScrollerPropertiesDecelerationFactor).ToDouble() != 0.5 {
			t.Error("scroller properties")
		}
		sc.ScrollToWithScrollTime(qtcore.QPointF{Y: 300}, 50)
		if !waitFor(func() bool { return sc.State() == qtwidgets.QScrollerInactive && len(positions) > 0 }) {
			t.Fatalf("scroll did not finish: state %v, %d positions", sc.State(), len(positions))
		}
		if !prepared || positions[len(positions)-1] != (qtcore.QPointF{Y: 300}) {
			t.Errorf("prepared %v, positions %v", prepared, positions)
		}
		target.Delete()
		f.Delete()
		if sc.Alive() {
			t.Error("the scroller outlived its target")
		}
	})
}

func TestSmallWidgets(t *testing.T) {
	qutetest.Run(t, func() {
		main := qtwidgets.NewQWidget(nil)
		main.Resize(100, 100)
		splash := qtwidgets.NewQSplashScreen()
		splash.SetPixmap(qtgui.NewQPixmapWithW(20, 20))
		msg := ""
		splash.OnMessageChanged(func(m string) { msg = m })
		splash.Show()
		splash.ShowMessage("loading")
		if msg != "loading" || splash.Message() != "loading" || splash.Pixmap().Width() != 20 {
			t.Errorf("splash message %q", splash.Message())
		}
		main.Show()
		splash.Finish(main)
		if splash.IsVisible() {
			t.Error("splash still visible after Finish")
		}
		splash.Delete()

		le := qtwidgets.NewQLineEdit(main)
		ff := qtwidgets.NewQFocusFrame(main)
		ff.SetWidget(le)
		if ff.Widget() != le.AsQWidget() {
			t.Error("focus frame widget")
		}

		em := qtwidgets.NewQErrorMessage(nil)
		em.ShowMessage("something failed")
		if !em.IsVisible() {
			t.Error("error message not shown")
		}
		em.Delete()

		le.Show() // a layout item of a hidden widget is empty
		item := qtwidgets.NewQWidgetItemV2WithWidget(le)
		if item.Widget() != le.AsQWidget() || item.SizeHint() != le.SizeHint() {
			t.Errorf("widget item: %v vs %v", item.SizeHint(), le.SizeHint())
		}
		le.Delete()
		if item.Alive() {
			t.Error("widget item outlived its widget")
		}

		qtwidgets.QWhatsThisEnterWhatsThisMode()
		if !qtwidgets.QWhatsThisInWhatsThisMode() {
			t.Error("not in what's this mode")
		}
		qtwidgets.QWhatsThisLeaveWhatsThisMode()
		qtwidgets.QWhatsThisShowText(qtcore.QPoint{X: 5, Y: 5}, "help")
		qtwidgets.QWhatsThisHideText()
		act := qtwidgets.QWhatsThisCreateAction()
		if act.Ownership() != qtcore.Owned {
			t.Errorf("what's this action: %v", act.Ownership())
		}
		act.Trigger()
		if !qtwidgets.QWhatsThisInWhatsThisMode() {
			t.Error("the action did not enter what's this mode")
		}
		qtwidgets.QWhatsThisLeaveWhatsThisMode()
		act.Delete()

		stack := qtgui.NewQUndoStack(nil)
		stack.Push(qtgui.NewQUndoCommandWithText("one", nil))
		view := qtwidgets.NewQUndoViewWithUndoStack(stack, nil)
		view.SetEmptyLabel("<start>")
		if view.Stack() != stack || view.Model().RowCount() != 2 || view.EmptyLabel() != "<start>" {
			t.Errorf("undo view: %d rows", view.Model().RowCount())
		}
		g := qtgui.NewQUndoGroup(nil)
		g.AddStack(stack)
		view.SetGroup(g)
		if view.Group() != g {
			t.Error("undo view group")
		}
		view.Delete()
		g.Delete()
		stack.Delete()
		main.Delete()
	})
}

// styled paints a red pixmap and a focus rectangle with a QStylePainter.
type styled struct {
	qtwidgets.QWidgetBase
	painted bool
}

func (s *styled) PaintEvent(e *qtgui.QPaintEvent) {
	p := qtwidgets.NewQStylePainterWithWidget(s)
	pm := qtgui.NewQPixmapWithW(10, 10)
	pm.FillFull(qtgui.RGB(255, 0, 0))
	p.DrawItemPixmap(qtcore.QRect{Width: 10, Height: 10}, int(qtcore.AlignLeft|qtcore.AlignTop), pm)
	opt := qtwidgets.NewQStyleOptionFocusRect()
	opt.SetRect(qtcore.QRect{X: 12, Width: 8, Height: 8})
	opt.SetBackgroundColor(qtgui.RGB(0, 0, 255))
	p.DrawPrimitive(qtwidgets.QStylePE_FrameFocusRect, opt.AsQStyleOption())
	s.painted = p.Style() != nil
	p.End()
	p.Delete()
}

func TestStylePainterAndOptions(t *testing.T) {
	qutetest.Run(t, func() {
		s := &styled{}
		w := qtwidgets.NewQWidgetWith(s, nil)
		w.Resize(30, 30)
		img := w.Grab().ToImage()
		if !s.painted || img.PixelColor(5, 5) != qtgui.RGB(255, 0, 0) {
			t.Errorf("painted %v, pixel %v", s.painted, img.PixelColor(5, 5))
		}
		w.Delete()

		fr := qtwidgets.NewQStyleOptionFocusRect()
		fr.SetBackgroundColor(qtgui.RGB(1, 2, 3))
		hv := qtwidgets.NewQStyleOptionHeaderV2()
		hv.SetTextElideMode(qtcore.ElideMiddle)
		hv.SetIsSectionDragTarget(true)
		sg := qtwidgets.NewQStyleOptionSizeGrip()
		sg.SetCorner(qtcore.TopRightCorner)
		tb := qtwidgets.NewQStyleOptionTabBarBase()
		tb.SetShape(qtwidgets.QTabBarTriangularNorth)
		tb.SetDocumentMode(true)
		tt := qtwidgets.NewQStyleOptionTitleBar()
		tt.SetText("title")
		tt.SetTitleBarFlags(qtcore.Window)
		tx := qtwidgets.NewQStyleOptionToolBox()
		tx.SetText("box")
		tx.SetPosition(qtwidgets.QStyleOptionToolBoxEnd)
		mi := qtwidgets.NewQStyleOptionMenuItemV2()
		mi.SetMouseDown(true)
		tr := qtwidgets.NewQTileRulesWithTileRule(qtcore.RepeatTile, qtcore.RoundTile)
		if fr.BackgroundColor() != qtgui.RGB(1, 2, 3) || hv.TextElideMode() != qtcore.ElideMiddle || !hv.IsSectionDragTarget() ||
			sg.Corner() != qtcore.TopRightCorner || tb.Shape() != qtwidgets.QTabBarTriangularNorth || !tb.DocumentMode() ||
			tt.Text() != "title" || tt.TitleBarFlags() != qtcore.Window || tx.Text() != "box" || tx.Position() != qtwidgets.QStyleOptionToolBoxEnd ||
			!mi.MouseDown() || tr.Horizontal() != qtcore.RepeatTile || tr.Vertical() != qtcore.RoundTile {
			t.Error("style option fields do not round-trip")
		}
		if qtwidgets.NewQTileRules().Horizontal() != qtcore.StretchTile {
			t.Error("default tile rules")
		}
	})
}

func TestGestureClasses(t *testing.T) {
	qutetest.Run(t, func() {
		owner := qtcore.NewQObject(nil)
		g := qtwidgets.NewQGesture(owner)
		g.SetHotSpot(qtcore.QPointF{X: 1, Y: 2})
		g.SetGestureCancelPolicy(qtwidgets.QGestureCancelAllInContext)
		if !g.HasHotSpot() || g.HotSpot() != (qtcore.QPointF{X: 1, Y: 2}) || g.GestureCancelPolicy() != qtwidgets.QGestureCancelAllInContext || g.State() != qtcore.NoGesture {
			t.Error("gesture fields")
		}
		g.UnsetHotSpot()
		if g.HasHotSpot() {
			t.Error("UnsetHotSpot")
		}
		pan := qtwidgets.NewQPanGesture(owner)
		pan.SetOffset(qtcore.QPointF{X: 5})
		pan.SetLastOffset(qtcore.QPointF{X: 2})
		pan.SetAcceleration(1.5)
		pinch := qtwidgets.NewQPinchGesture(owner)
		pinch.SetScaleFactor(2)
		pinch.SetRotationAngle(30)
		pinch.SetCenterPoint(qtcore.QPointF{X: 3, Y: 3})
		pinch.SetChangeFlags(qtwidgets.QPinchGestureScaleFactorChanged)
		swipe := qtwidgets.NewQSwipeGesture(owner)
		swipe.SetSwipeAngle(90)
		hold := qtwidgets.NewQTapAndHoldGesture(owner)
		hold.SetPosition(qtcore.QPointF{X: 7})
		tap := qtwidgets.NewQTapGesture(owner)
		tap.SetPosition(qtcore.QPointF{Y: 8})
		if pan.Delta() != (qtcore.QPointF{X: 3}) || pan.Acceleration() != 1.5 ||
			pinch.ScaleFactor() != 2 || pinch.RotationAngle() != 30 || pinch.CenterPoint() != (qtcore.QPointF{X: 3, Y: 3}) || pinch.ChangeFlags() != qtwidgets.QPinchGestureScaleFactorChanged ||
			swipe.SwipeAngle() != 90 || swipe.VerticalDirection() != qtwidgets.QSwipeGestureUp ||
			hold.Position().X != 7 || tap.Position().Y != 8 {
			t.Error("gesture subclass fields")
		}
		old := qtwidgets.QTapAndHoldGestureTimeout()
		qtwidgets.QTapAndHoldGestureSetTimeout(1234)
		if qtwidgets.QTapAndHoldGestureTimeout() != 1234 {
			t.Error("tap and hold timeout")
		}
		qtwidgets.QTapAndHoldGestureSetTimeout(old)
		owner.Delete()
		if g.Alive() || tap.Alive() {
			t.Error("gestures outlived their parent")
		}
	})
}
