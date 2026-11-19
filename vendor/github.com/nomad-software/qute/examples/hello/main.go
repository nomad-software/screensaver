// Command hello is a small qute application: a form, a list, a custom-painted
// widget and a menu. Run it with -screenshot out.png to render it offscreen
// and exit (QT_QPA_PLATFORM=offscreen needs no display).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
)

// Swatch is a Go subclass of QWidget that paints itself.
type Swatch struct {
	qtwidgets.QWidgetBase
	color qtgui.QColor
}

func (s *Swatch) PaintEvent(e *qtgui.QPaintEvent) {
	// the painter ends, and is freed, when the function returns
	qtgui.PaintOn(s, func(p *qtgui.QPainter) {
		p.SetRenderHint(qtgui.QPainterAntialiasing)
		r := s.Rect()
		p.FillRect(r, qtgui.RGB(250, 250, 250))
		p.SetBrushColor(s.color)
		p.SetPenColor(qtgui.RGB(40, 40, 40))
		p.DrawEllipse(r.Adjusted(8, 8, -8, -8))
		p.DrawText(qtcore.QPoint{X: 12, Y: r.Height - 12}, s.color.Name())
	})
}

func (s *Swatch) SizeHint() qtcore.QSize { return qtcore.QSize{Width: 140, Height: 140} }

func (s *Swatch) MousePressEvent(e *qtgui.QMouseEvent) {
	s.color = qtgui.RGB(s.color.G, s.color.B, s.color.R) // rotate channels
	s.Update()
}

func build() *qtwidgets.QMainWindow {
	win := qtwidgets.NewQMainWindow(nil)
	win.SetWindowTitle("qute hello")

	file := win.MenuBar().AddMenuTitle("&File")
	quit := file.AddActionText("&Quit")
	quit.OnTriggered(func(bool) { win.Close() })

	central := qtwidgets.NewQWidget(nil)
	row := qtwidgets.NewQHBoxLayout(central)

	form := qtwidgets.NewQFormLayout(nil)
	name := qtwidgets.NewQLineEdit(nil)
	name.SetPlaceholderText("your name")
	greeting := qtwidgets.NewQLabelWithText("Hello, stranger", nil)
	name.OnTextChanged(func(s string) {
		if s == "" {
			s = "stranger"
		}
		greeting.SetText(fmt.Sprintf("Hello, %s", s))
	})
	form.AddRowText("Name:", name)
	form.AddRowWidget(greeting)

	list := qtwidgets.NewQListWidget(nil)
	for _, s := range []string{"Owned", "Parented", "Borrowed", "Dead"} {
		list.AddItemText(s)
	}
	list.SetCurrentRow(1)
	form.AddRowText("States:", list)

	swatch := &Swatch{color: qtgui.RGB(220, 80, 60)}
	qtwidgets.NewQWidgetWith(swatch, nil)

	row.AddLayout(form)
	row.AddWidget(swatch)
	win.SetCentralWidget(central)
	win.StatusBar().ShowMessage("Click the circle")
	win.Resize(480, 260)
	return win
}

func main() {
	shot := flag.String("screenshot", "", "render the window to this PNG and exit")
	tree := flag.Bool("tree", false, "with -screenshot, also print the object tree")
	flag.Parse()
	if *shot != "" && os.Getenv("QT_QPA_PLATFORM") == "" {
		os.Setenv("QT_QPA_PLATFORM", "offscreen")
	}
	os.Exit(qute.Main(func() {
		win := build()
		win.Show()
		if *shot != "" {
			qute.Post(func() { // after the first layout pass
				if err := qute.SaveScreenshot(win, *shot); err != nil {
					log.Fatal(err)
				}
				if *tree {
					fmt.Println(qute.DumpTree(win))
				}
				qute.Quit(0)
			})
		}
	}))
}
