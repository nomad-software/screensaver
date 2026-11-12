package ui

import (
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
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
func dotIcon(colour string) *qtgui.QIcon {
	const size = 12
	pixmap := qtgui.NewQPixmapWithW(size, size)
	defer pixmap.Delete()
	// The zero colour is transparent.
	pixmap.FillFull(qtgui.QColor{})
	qtgui.PaintOn(pixmap, func(painter *qtgui.QPainter) {
		painter.SetRenderHint(qtgui.QPainterAntialiasing)
		painter.SetPenStyle(qtcore.NoPen)
		painter.SetBrushColor(mustColour(colour))
		painter.DrawEllipse(qtcore.QRect{X: 1, Y: 1, Width: size - 2, Height: size - 2})
	})
	// The icon keeps a copy of the picture.
	return qtgui.NewQIconWithPixmap(pixmap)
}
