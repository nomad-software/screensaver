package ui

import (
	"fmt"
	"runtime"

	qt "github.com/mappu/miqt/qt6"
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

// makeTabs puts the panes in tabs, in the order of the tab constants, each
// with an icon, and a shortcut to each, Ctrl+1 to Ctrl+3. The tabs are as the
// style draws them with more room in them, and each pane has room around what
// is in it. The icons are drawn in the colour of text, and drawn again
// whenever the palette or style changes, as with the theme.
func (w *Window) makeTabs(panes [tabCount]*qt.QWidget, titles [tabCount]string) {
	w.tabs = qt.NewQTabWidget2()
	bar := qt.NewQTabBar2()
	// What is returned is copied once it is returned, so the size of each
	// tab is kept here, where the collector cannot free it before then.
	sizes := map[int]*qt.QSize{}
	bar.OnTabSizeHint(func(super func(int) *qt.QSize, index int) *qt.QSize {
		size := super(index)
		if sizes[index] == nil {
			sizes[index] = qt.NewQSize()
		}
		sizes[index].SetWidth(size.Width() + tabPadX)
		sizes[index].SetHeight(size.Height() + tabPadY)
		return sizes[index]
	})
	w.tabs.SetTabBar(bar)
	w.tabs.SetDocumentMode(true)
	for i, pane := range panes {
		pane.Layout().SetContentsMargins(paneMargin, paneMargin, paneMargin, paneMargin)
		w.tabs.AddTab(pane, titles[i])
	}
	w.drawTabIcons()
	for i := range tabCount {
		key := qt.NewQKeySequence2(fmt.Sprintf("Ctrl+%d", i+1))
		shortcut := qt.NewQShortcut2(key, w.QObject)
		key.Delete()
		shortcut.OnActivated(func() { w.tabs.SetCurrentIndex(i) })
	}
	w.OnChangeEvent(func(super func(*qt.QEvent), event *qt.QEvent) {
		super(event)
		switch event.Type() {
		case qt.QEvent__PaletteChange, qt.QEvent__ApplicationPaletteChange, qt.QEvent__StyleChange:
			w.drawTabIcons()
			w.inhibitors.drawBins()
		}
	})
}

// drawTabIcons gives each tab its icon, in the colour of text: a monitor for
// the savers, a power symbol for the service, and a no entry sign for the
// programs that stop the saver.
func (w *Window) drawTabIcons() {
	// The application's palette is the theme's at once; when the window
	// hears of a change, the tabs can still have the palette before it.
	palette := qt.QGuiApplication_Palette()
	defer runtime.KeepAlive(palette)
	text := palette.ColorWithCr(qt.QPalette__WindowText)
	for i, draw := range [tabCount]func(*qt.QPainter){drawSaversIcon, drawServiceIcon, drawInhibitorsIcon} {
		icon := tabIcon(text, w.DevicePixelRatioF(), draw)
		w.tabs.SetTabIcon(i, icon)
		icon.Delete()
	}
}

// tabIcon returns an icon drawn by draw, on a clear square of tabIconSize, in
// the colour, at the ratio of pixels the window has.
func tabIcon(colour *qt.QColor, ratio float64, draw func(*qt.QPainter)) *qt.QIcon {
	ratio = max(ratio, 1)
	side := int(tabIconSize * ratio)
	pixmap := qt.NewQPixmap2(side, side)
	defer pixmap.Delete()
	pixmap.SetDevicePixelRatio(ratio)
	clear := qt.NewQColor2(qt.Transparent)
	pixmap.FillWithFillColor(clear)
	clear.Delete()

	p := qt.NewQPainter2(pixmap.QPaintDevice)
	p.SetRenderHint(qt.QPainter__Antialiasing)
	pen := qt.NewQPen3(colour)
	pen.SetWidthF(tabIconPen)
	pen.SetCapStyle(qt.RoundCap)
	pen.SetJoinStyle(qt.RoundJoin)
	p.SetPenWithPen(pen)
	pen.Delete()
	p.SetBrushWithStyle(qt.NoBrush)
	draw(p)
	p.End()
	p.Delete()
	// The icon keeps a copy of the picture.
	return qt.NewQIcon2(pixmap)
}

// drawSaversIcon draws a monitor on a stand.
func drawSaversIcon(p *qt.QPainter) {
	screen := qt.NewQRectF4(1.5, 2, 13, 9)
	p.DrawRoundedRect(screen, 1.5, 1.5)
	screen.Delete()
	drawLine(p, 8, 11, 8, 13.5)
	drawLine(p, 5, 13.5, 11, 13.5)
}

// drawServiceIcon draws a power symbol: a ring open at the top, with a line
// down into the gap.
func drawServiceIcon(p *qt.QPainter) {
	ring := qt.NewQRectF4(2.5, 3, 11, 11)
	// Angles are in sixteenths of a degree, from three o'clock
	// anticlockwise; the gap is the 70 degrees about twelve o'clock.
	p.DrawArc(ring, 125*16, 290*16)
	ring.Delete()
	drawLine(p, 8, 1.5, 8, 7.5)
}

// drawInhibitorsIcon draws a no entry sign: a circle crossed by a line.
func drawInhibitorsIcon(p *qt.QPainter) {
	circle := qt.NewQRectF4(2, 2, 12, 12)
	p.DrawEllipse(circle)
	circle.Delete()
	drawLine(p, 3.8, 12.2, 12.2, 3.8)
}

// drawBinIcon draws a bin: a lid with a handle, and a body narrowing to its
// foot, with two lines down it.
func drawBinIcon(p *qt.QPainter) {
	drawLine(p, 2.5, 4, 13.5, 4)
	drawLine(p, 6.5, 4, 6.5, 2)
	drawLine(p, 6.5, 2, 9.5, 2)
	drawLine(p, 9.5, 2, 9.5, 4)
	body := qt.NewQPainterPath()
	defer body.Delete()
	for i, pt := range [][2]float64{{4, 4}, {5, 14}, {11, 14}, {12, 4}} {
		point := qt.NewQPointF3(pt[0], pt[1])
		if i == 0 {
			body.MoveTo(point)
		} else {
			body.LineTo(point)
		}
		point.Delete()
	}
	p.DrawPath(body)
	drawLine(p, 6.8, 6.5, 7.1, 11.5)
	drawLine(p, 9.2, 6.5, 8.9, 11.5)
}

// drawLine draws a line from one point to another.
func drawLine(p *qt.QPainter, x1, y1, x2, y2 float64) {
	from, to := qt.NewQPointF3(x1, y1), qt.NewQPointF3(x2, y2)
	p.DrawLine4(from, to)
	from.Delete()
	to.Delete()
}
