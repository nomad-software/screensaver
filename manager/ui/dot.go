package ui

import (
	qt "github.com/mappu/miqt/qt6"
)

// The colours of the dots which show whether something is running. They are
// seen on light and dark themes alike.
const (
	colourOn      = "#2e9d4a"
	colourOff     = "#8a8a8a"
	colourWaiting = "#d99a1e"
	colourFailed  = "#d0453b"
)

// dot returns a dot of the colour, as rich text for a label.
func dot(colour string) string {
	return `<span style="color: ` + colour + `">●</span>`
}

// dotIcon returns a dot of the colour as an icon, for a list.
func dotIcon(colour string) *qt.QIcon {
	const size = 12
	pixmap := qt.NewQPixmap2(size, size)
	defer pixmap.Delete()
	transparent := qt.NewQColor2(qt.Transparent)
	defer transparent.Delete()
	pixmap.FillWithFillColor(transparent)
	painter := qt.NewQPainter2(pixmap.QPaintDevice)
	painter.SetRenderHint(qt.QPainter__Antialiasing)
	painter.SetPenWithStyle(qt.NoPen)
	c := qt.NewQColor6(colour)
	defer c.Delete()
	brush := qt.NewQBrush3(c)
	defer brush.Delete()
	painter.SetBrush(brush)
	painter.DrawEllipse2(1, 1, size-2, size-2)
	painter.End()
	painter.Delete()
	return qt.NewQIcon2(pixmap)
}
