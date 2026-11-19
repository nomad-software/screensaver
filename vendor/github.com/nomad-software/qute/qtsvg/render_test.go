package qtsvg_test

import (
	"strings"
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtsvg"
	"github.com/nomad-software/qute/qutetest"
)

const redSquare = `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10">` +
	`<rect width="10" height="10" fill="#ff0000"/></svg>`

// The renderer draws into an image through a painter.
func TestRenderToImage(t *testing.T) {
	qutetest.Run(t, func() {
		r := qtsvg.NewQSvgRendererWithBytes([]byte(redSquare), nil)
		if !r.IsValid() {
			t.Fatal("renderer rejected a valid SVG")
		}
		img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 10, Height: 10}, qtgui.QImageFormat_ARGB32)
		img.Fill(0)
		p := qtgui.NewQPainterOn(img)
		r.Render(p)
		p.End()
		if c := img.PixelColor(5, 5); c.R != 255 || c.G != 0 || c.B != 0 {
			t.Errorf("pixel %v, want red", c)
		}
		r.Delete()
	})
}

// The generator writes SVG into the device passed to SetOutputDevice.
func TestGeneratorWritesToKeptDevice(t *testing.T) {
	qutetest.Run(t, func() {
		g := qtsvg.NewQSvgGenerator()
		buf := qtcore.NewQBuffer(nil)
		g.SetOutputDevice(buf)
		g.SetSize(qtcore.QSize{Width: 10, Height: 10})
		p := qtgui.NewQPainterOn(g)
		p.FillRect(qtcore.QRect{Width: 10, Height: 10}, qtgui.RGB(0, 0, 255))
		p.End()
		if out := string(buf.Data()); !strings.Contains(out, "<svg") {
			t.Errorf("generator output %q, want an SVG document", out)
		}
		g.Delete()
	})
}
