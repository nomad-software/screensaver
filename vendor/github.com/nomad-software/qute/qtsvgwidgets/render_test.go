package qtsvgwidgets_test

import (
	"testing"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtsvg"
	"github.com/nomad-software/qute/qtsvgwidgets"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

const redSquare = `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20">` +
	`<rect width="20" height="20" fill="#ff0000"/></svg>`

// A QSvgWidget draws the document it loaded.
func TestSvgWidgetRenders(t *testing.T) {
	qutetest.Run(t, func() {
		w := qtsvgwidgets.NewQSvgWidget(nil)
		defer w.Delete()
		w.LoadBytes([]byte(redSquare))
		if !w.Renderer().IsValid() {
			t.Fatal("the widget's renderer rejected a valid SVG")
		}
		w.Resize(20, 20)
		img := qute.Screenshot(w)
		if c := img.NRGBAAt(10, 10); c.R != 255 || c.G != 0 || c.B != 0 {
			t.Errorf("pixel %v, want red", c)
		}
	})
}

// A graphics SVG item keeps a shared renderer that nothing else refers to
// through a collection, and draws with it.
func TestSvgItemKeepsSharedRenderer(t *testing.T) {
	var view *qtwidgets.QGraphicsView
	qutetest.Run(t, func() {
		scene := qtwidgets.NewQGraphicsScene(nil)
		item := qtsvgwidgets.NewQGraphicsSvgItem(nil)
		item.SetSharedRenderer(qtsvg.NewQSvgRendererWithBytes([]byte(redSquare), nil))
		scene.AddItem(item)
		view = qtwidgets.NewQGraphicsViewWithScene(scene, nil)
		view.Resize(60, 60)
	})
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {
		img := qute.Screenshot(view)
		red := 0
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if c := img.NRGBAAt(x, y); c.R == 255 && c.G == 0 && c.B == 0 {
					red++
				}
			}
		}
		if red < 100 {
			t.Errorf("%d red pixels in the view, want the 20x20 square", red)
		}
		scene := view.Scene()
		view.Delete()
		scene.Delete()
	})
}
