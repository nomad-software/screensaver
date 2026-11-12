package ui

import (
	"fmt"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
)

// The tabs of the window, in order.
const (
	tabSavers = iota
	tabService
	tabInhibitors
	tabCount
)

// tabPadX and tabPadY are the room added to each tab beyond what the style
// gives it, across and down, as Prism has.
const (
	tabPadX = 20
	tabPadY = 10
)

// paneMargin is the room between each edge of a tab's pane and what is in it,
// the same all the way around.
const paneMargin = 20

// tabIconSize is the size the icons of the tabs are drawn at, in pixels at a
// ratio of one, and tabIconPen the width of their lines.
const (
	tabIconSize = 16
	tabIconPen  = 1.5
)

// tabWidget is a QTabWidget subclass, only so that it can be given a tab bar
// of its own, which is protected in Qt.
type tabWidget struct {
	qtwidgets.QTabWidgetBase
}

// paddedTabBar is a tab bar whose tabs have more room in them than the style
// gives them.
type paddedTabBar struct {
	qtwidgets.QTabBarBase
}

// TabSizeHint is the size the style gives the tab, with the padding added.
func (b *paddedTabBar) TabSizeHint(index int) qtcore.QSize {
	size := b.Super().TabSizeHint(index)
	return qtcore.QSize{Width: size.Width + tabPadX, Height: size.Height + tabPadY}
}

// makeTabs puts the panes in tabs, in the order of the tab constants, each
// with an icon, and a shortcut to each, Ctrl+1 to Ctrl+3. The tabs are as the
// style draws them with more room in them, and each pane has room around what
// is in it. The icons are drawn in the colour of text, and drawn again
// whenever the palette or style changes, as with the theme (see ChangeEvent).
func (w *Window) makeTabs(panes [tabCount]*qtwidgets.QWidget, titles [tabCount]string) {
	w.tabs = &tabWidget{}
	qtwidgets.NewQTabWidgetWith(w.tabs, nil)
	bar := &paddedTabBar{}
	qtwidgets.NewQTabBarWith(bar, nil)
	w.tabs.SetTabBar(bar)
	w.tabs.SetDocumentMode(true)
	for i, pane := range panes {
		pane.Layout().SetContentsMargins(margins(paneMargin, paneMargin, paneMargin, paneMargin))
		w.tabs.AddTab(pane, titles[i])
	}
	w.drawTabIcons()
	for i := range tabCount {
		key := qtgui.NewQKeySequenceWithKey(fmt.Sprintf("Ctrl+%d", i+1))
		shortcut := qtgui.NewQShortcutWithKeySequence(key, w)
		key.Delete()
		shortcut.OnActivated(func() { w.tabs.SetCurrentIndex(i) })
	}
}

// ChangeEvent draws the icons again when the palette or the style changes.
func (w *Window) ChangeEvent(event *qtcore.QEvent) {
	w.Super().ChangeEvent(event)
	// The window hears of changes before it has its tabs.
	if w.tabs == nil || w.inhibitors == nil {
		return
	}
	switch event.Type() {
	case qtcore.QEventPaletteChange, qtcore.QEventApplicationPaletteChange, qtcore.QEventStyleChange:
		w.drawTabIcons()
		w.inhibitors.drawBins()
	}
}

// drawTabIcons gives each tab its icon, in the colour of text: a monitor for
// the savers, a power symbol for the service, and a no entry sign for the
// programs that stop the saver.
func (w *Window) drawTabIcons() {
	// The application's palette is the theme's at once; when the window
	// hears of a change, the tabs can still have the palette before it.
	palette := qtgui.QGuiApplicationPalette()
	defer palette.Delete()
	text := palette.Color(qtgui.QPaletteWindowText)
	for i, draw := range [tabCount]func(*qtgui.QPainter){drawSaversIcon, drawServiceIcon, drawInhibitorsIcon} {
		icon := tabIcon(text, w.AsQPaintDevice().DevicePixelRatioF(), draw)
		w.tabs.SetTabIcon(i, icon)
		icon.Delete()
	}
}

// tabIcon returns an icon drawn by draw, on a clear square of tabIconSize, in
// the colour, at the ratio of pixels the window has.
func tabIcon(colour qtgui.QColor, ratio float64, draw func(*qtgui.QPainter)) *qtgui.QIcon {
	ratio = max(ratio, 1)
	side := int(tabIconSize * ratio)
	pixmap := qtgui.NewQPixmapWithW(side, side)
	defer pixmap.Delete()
	pixmap.SetDevicePixelRatio(ratio)
	// The zero colour is transparent.
	pixmap.FillFull(qtgui.QColor{})

	qtgui.PaintOn(pixmap, func(p *qtgui.QPainter) {
		p.SetRenderHint(qtgui.QPainterAntialiasing)
		pen := qtgui.NewQPenWithColor(colour)
		defer pen.Delete()
		pen.SetWidthF(tabIconPen)
		pen.SetCapStyle(qtcore.RoundCap)
		pen.SetJoinStyle(qtcore.RoundJoin)
		p.SetPen(pen)
		p.SetBrushStyle(qtcore.NoBrush)
		draw(p)
	})
	// The icon keeps a copy of the picture.
	return qtgui.NewQIconWithPixmap(pixmap)
}

// drawSaversIcon draws a monitor on a stand.
func drawSaversIcon(p *qtgui.QPainter) {
	p.DrawRoundedRectF(qtcore.QRectF{X: 1.5, Y: 2, Width: 13, Height: 9}, 1.5, 1.5)
	drawLine(p, 8, 11, 8, 13.5)
	drawLine(p, 5, 13.5, 11, 13.5)
}

// drawServiceIcon draws a power symbol: a ring open at the top, with a line
// down into the gap.
func drawServiceIcon(p *qtgui.QPainter) {
	// Angles are in sixteenths of a degree, from three o'clock
	// anticlockwise; the gap is the 70 degrees about twelve o'clock.
	p.DrawArcF(qtcore.QRectF{X: 2.5, Y: 3, Width: 11, Height: 11}, 125*16, 290*16)
	drawLine(p, 8, 1.5, 8, 7.5)
}

// drawInhibitorsIcon draws a no entry sign: a circle crossed by a line.
func drawInhibitorsIcon(p *qtgui.QPainter) {
	p.DrawEllipseF(qtcore.QRectF{X: 2, Y: 2, Width: 12, Height: 12})
	drawLine(p, 3.8, 12.2, 12.2, 3.8)
}

// drawBinIcon draws a bin: a lid with a handle, and a body narrowing to its
// foot, with two lines down it.
func drawBinIcon(p *qtgui.QPainter) {
	drawLine(p, 2.5, 4, 13.5, 4)
	drawLine(p, 6.5, 4, 6.5, 2)
	drawLine(p, 6.5, 2, 9.5, 2)
	drawLine(p, 9.5, 2, 9.5, 4)
	body := qtgui.NewQPainterPath()
	defer body.Delete()
	for i, pt := range []qtcore.QPointF{{X: 4, Y: 4}, {X: 5, Y: 14}, {X: 11, Y: 14}, {X: 12, Y: 4}} {
		if i == 0 {
			body.MoveTo(pt)
		} else {
			body.LineTo(pt)
		}
	}
	p.DrawPath(body)
	drawLine(p, 6.8, 6.5, 7.1, 11.5)
	drawLine(p, 9.2, 6.5, 8.9, 11.5)
}

// drawLine draws a line from one point to another.
func drawLine(p *qtgui.QPainter, x1, y1, x2, y2 float64) {
	p.DrawLineF(qtcore.QPointF{X: x1, Y: y1}, qtcore.QPointF{X: x2, Y: y2})
}
