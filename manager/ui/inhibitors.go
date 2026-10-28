package ui

import (
	"html"
	"runtime"
	"slices"
	"strings"

	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
)

// summaryPad is the room above and below the line saying which programs are
// running, as much as the tab's pane has at its top and bottom.
const summaryPad = paneMargin

// inhibitorBox lists the programs which, while any of them runs, keep the
// launcher from showing the saver, with which of them are running now.
type inhibitorBox struct {
	*qt.QWidget
	w       *Window
	summary *qt.QLabel
	list    *qt.QListWidget
	name    *qt.QLineEdit
	add     *qt.QPushButton

	// bins are the buttons at the right of each row which remove its
	// program, in the order of the rows.
	bins []*qt.QToolButton

	// running and stopped are the icons of programs running and not, and
	// bin that of the buttons which remove them.
	running, stopped, bin *qt.QIcon

	// found is what was running at the last refresh, by name.
	found map[string][]int
}

// newInhibitorBox lists the programs the settings name.
func newInhibitorBox(w *Window) *inhibitorBox {
	b := &inhibitorBox{QWidget: qt.NewQWidget2(), w: w,
		running: dotIcon(colourOn), stopped: dotIcon(colourOff)}
	layout := qt.NewQVBoxLayout(b.QWidget)
	note := qt.NewQLabel3("The saver is not shown while any of these programs is running. " +
		"Give the name of the process, as pgrep or top shows it, such as vlc, mpv or steam. Changes take effect at once.")
	note.SetWordWrap(true)
	layout.AddWidget(note.QWidget)
	b.summary = qt.NewQLabel2()
	b.summary.SetWordWrap(true)
	// The summary stands apart from the note above and the list below.
	b.summary.SetContentsMargins(0, summaryPad, 0, summaryPad)
	layout.AddWidget(b.summary.QWidget)

	b.list = qt.NewQListWidget2()
	layout.AddWidget2(b.list.QWidget, 1)

	row := qt.NewQHBoxLayout2()
	b.name = qt.NewQLineEdit2()
	b.name.SetPlaceholderText("Process name")
	b.name.OnReturnPressed(b.addName)
	b.name.OnTextChanged(func(string) { b.enable() })
	row.AddWidget2(b.name.QWidget, 1)
	b.add = qt.NewQPushButton3("Add")
	b.add.OnClicked(b.addName)
	row.AddWidget(b.add.QWidget)
	layout.AddLayout(row.QLayout)

	b.drawBins()
	b.fill()
	return b
}

// drawBins draws the icon of the buttons which remove programs, in the colour
// of text, and gives it to those there are. It is drawn again whenever the
// palette or style changes, as the tabs' icons are.
func (b *inhibitorBox) drawBins() {
	palette := qt.QGuiApplication_Palette()
	defer runtime.KeepAlive(palette)
	text := palette.ColorWithCr(qt.QPalette__WindowText)
	if b.bin != nil {
		b.bin.Delete()
	}
	b.bin = tabIcon(text, b.w.DevicePixelRatioF(), drawBinIcon)
	for _, bin := range b.bins {
		bin.SetIcon(b.bin)
	}
}

// fill lists the programs again, from the settings, each row with a button at
// its right which removes its program.
func (b *inhibitorBox) fill() {
	b.list.Clear()
	b.bins = nil
	for _, name := range b.w.settings.Inhibitors {
		item := qt.NewQListWidgetItem7("", b.list)
		bin := qt.NewQToolButton2()
		bin.SetAutoRaise(true)
		bin.SetIcon(b.bin)
		bin.SetToolTip("Remove " + name)
		bin.OnClicked(func() { b.removeName(name) })
		b.bins = append(b.bins, bin)
		// The button is on a clear widget over the row, at its right, so
		// that the row's own dot and text show beside it.
		cell := qt.NewQWidget2()
		cellLayout := qt.NewQHBoxLayout(cell)
		cellLayout.SetContentsMargins(0, 0, 4, 0)
		cellLayout.AddStretch()
		cellLayout.AddWidget(bin.QWidget)
		hint := cell.SizeHint()
		item.SetSizeHint(hint)
		runtime.KeepAlive(hint)
		b.list.SetItemWidget(item, cell)
	}
	b.show(b.found)
}

// show shows which of the programs are running, as the refresh found.
func (b *inhibitorBox) show(found map[string][]int) {
	b.found = found
	var running []string
	for i, name := range b.w.settings.Inhibitors {
		item := b.list.Item(i)
		if item == nil {
			continue
		}
		if pids := found[name]; len(pids) > 0 {
			sorted := slices.Sorted(slices.Values(pids))
			item.SetIcon(b.running)
			item.SetText(name + "  —  running (process " + pidList(sorted) + ")")
			running = append(running, name)
		} else {
			item.SetIcon(b.stopped)
			item.SetText(name)
		}
	}
	if len(running) > 0 {
		b.summary.SetText(dot(colourOn) + " <b>" + html.EscapeString(strings.Join(running, ", ")) +
			"</b> running: the saver will not be shown.")
	} else {
		b.summary.SetText(dot(colourOff) + " None running: the saver will be shown.")
	}
	b.enable()
}

// enable enables Add for a name that can be added.
func (b *inhibitorBox) enable() {
	b.add.SetEnabled(b.canAdd(strings.TrimSpace(b.name.Text())))
}

// canAdd says whether the name can be added: one not listed yet, with no
// comma or space, as the launcher is given the names joined by commas.
func (b *inhibitorBox) canAdd(name string) bool {
	return name != "" && !strings.ContainsAny(name, ", \t") && !slices.Contains(b.w.settings.Inhibitors, name)
}

// addName adds the program named in the field.
func (b *inhibitorBox) addName() {
	name := strings.TrimSpace(b.name.Text())
	if !b.canAdd(name) {
		return
	}
	b.w.settings.Inhibitors = append(b.w.settings.Inhibitors, name)
	if !b.w.save() {
		b.w.settings.Inhibitors = b.w.settings.Inhibitors[:len(b.w.settings.Inhibitors)-1]
		return
	}
	b.w.reconfigure()
	b.name.Clear()
	b.fill()
	// Whether it runs is shown at once, rather than at the next refresh.
	b.w.refresh()
}

// removeName removes the program named.
func (b *inhibitorBox) removeName(name string) {
	row := slices.Index(b.w.settings.Inhibitors, name)
	if row < 0 {
		return
	}
	before := b.w.settings.Inhibitors
	b.w.settings.Inhibitors = slices.Delete(slices.Clone(before), row, row+1)
	if !b.w.save() {
		b.w.settings.Inhibitors = before
		return
	}
	b.w.reconfigure()
	// The list is made again once the click is done with, as making it
	// again deletes the button that was clicked.
	mainthread.Start(b.fill)
}
