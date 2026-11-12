package ui

import (
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"github.com/nomad-software/screensaver/screen"
)

// The size the previews are played at, which is the size of their GIFs, so
// that a preview keeps its size however the window is resized.
const (
	previewWidth  = 640
	previewHeight = 480
)

// listMinWidth is the narrowest the list of savers can be made, by shrinking
// the window, so that their names can still be read.
const listMinWidth = 240

// saverList lists the savers by name, which suits however many there are.
// The one selected plays its preview, with buttons to run the saver itself and
// to use it.
type saverList struct {
	*qtwidgets.QWidget
	w      *Window
	savers []screen.Saver
	list   *qtwidgets.QListWidget

	// selected is the index of the saver selected, -1 for none.
	selected int

	// movie plays the preview of the saver selected, nil while none is.
	movie *qtgui.QMovie

	screen  *qtwidgets.QLabel
	preview *qtwidgets.QPushButton
	use     *qtwidgets.QPushButton
}

// newSaverList lists the savers, with the one chosen marked and selected, so
// that its preview plays.
func newSaverList(w *Window, savers []screen.Saver) *saverList {
	g := &saverList{QWidget: qtwidgets.NewQWidget(nil), w: w, savers: savers, selected: -1}
	layout := qtwidgets.NewQHBoxLayout(g)

	g.list = qtwidgets.NewQListWidget(nil)
	for _, s := range savers {
		g.list.AddItemText(s.Title)
	}
	g.list.SetCurrentRow(-1)
	g.list.OnCurrentRowChanged(g.choose)
	g.list.SetMinimumWidth(listMinWidth)
	layout.AddWidgetFull(g.list, 1, 0)

	// The preview is at the top, and the buttons below it at the bottom.
	side := qtwidgets.NewQVBoxLayout(nil)
	g.screen = qtwidgets.NewQLabel(nil)
	// The frame matches the list's, and is drawn around the preview rather
	// than over it, so that none of it is hidden.
	g.screen.SetFrameShape(qtwidgets.QFrameStyledPanel)
	g.screen.SetFrameShadow(qtwidgets.QFrameSunken)
	frame := 2 * g.screen.FrameWidth()
	g.screen.SetFixedSize(previewWidth+frame, previewHeight+frame)
	g.screen.SetAlignment(qtcore.AlignCenter)
	g.screen.SetStyleSheet("background: black")
	side.AddWidget(g.screen)
	side.AddStretch()
	buttons := qtwidgets.NewQHBoxLayout(nil)
	buttons.AddStretch()
	g.preview = qtwidgets.NewQPushButtonWithText("Preview", nil)
	g.preview.SetToolTip("Runs the saver now, full screen. Move the mouse or press a key to end it.")
	g.preview.OnClicked(func(bool) { g.previewSelected() })
	buttons.AddWidget(g.preview)
	g.use = qtwidgets.NewQPushButtonWithText("Use This Saver", nil)
	g.use.SetToolTip("Makes it the saver the launcher shows, restarting the launcher if it is running.")
	g.use.OnClicked(func(bool) { g.useSelected() })
	buttons.AddWidget(g.use)
	side.AddLayout(buttons)
	layout.AddLayout(side)

	g.markCurrent()
	g.choose(-1)
	for i, s := range savers {
		if s.Name == w.settings.Saver {
			g.list.SetCurrentRow(i)
		}
	}
	return g
}

// choose makes the saver at index i the one chosen, and plays its preview, or
// none for -1, and enables what can be done with it: the one in use cannot be
// used again.
func (g *saverList) choose(i int) {
	g.selected = i
	g.play(i)
	if i < 0 {
		g.preview.SetEnabled(false)
		g.use.SetEnabled(false)
		return
	}
	g.preview.SetEnabled(true)
	g.use.SetEnabled(g.savers[i].Name != g.w.settings.Saver)
}

// play plays the preview of the saver at index i, or none for -1. The movie
// of the one before, and its buffer, are deleted, so that only one is kept
// however many are selected in turn.
func (g *saverList) play(i int) {
	if g.movie != nil {
		g.screen.SetMovie(nil)
		g.movie.Stop()
		g.movie.DeleteLater()
		g.movie = nil
	}
	if i < 0 {
		return
	}
	movie := qtgui.NewQMovie(g.screen)
	buffer := qtcore.NewQBuffer(movie)
	buffer.SetData(g.savers[i].Preview)
	buffer.Open(qtcore.QIODeviceBaseReadOnly)
	movie.SetDevice(buffer)
	movie.SetFormat([]byte("gif"))
	movie.SetScaledSize(qtcore.QSize{Width: previewWidth, Height: previewHeight})
	g.movie = movie
	g.screen.SetMovie(movie)
	movie.Start()
}

// markCurrent marks the saver configured in the list.
func (g *saverList) markCurrent() {
	for i, s := range g.savers {
		text := s.Title
		if s.Name == g.w.settings.Saver {
			text = "✔ " + text
		}
		g.list.Item(i).SetText(text)
	}
}

// previewSelected runs the saver chosen.
func (g *saverList) previewSelected() {
	if g.selected < 0 {
		return
	}
	s := g.savers[g.selected]
	if err := startSaver(s.Name); err != nil {
		warning(&g.w.QWidget, "Preview", s.Title+" could not be started: "+err.Error())
	}
}

// useSelected makes the saver chosen the one the launcher shows.
func (g *saverList) useSelected() {
	if g.selected < 0 {
		return
	}
	g.w.settings.Saver = g.savers[g.selected].Name
	g.w.saveAndReconfigure()
	g.markCurrent()
	g.choose(g.selected)
}
