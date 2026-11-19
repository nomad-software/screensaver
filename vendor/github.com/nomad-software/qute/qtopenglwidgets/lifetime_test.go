package qtopenglwidgets_test

import (
	"testing"

	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtopenglwidgets"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

type sizedCanvas struct {
	qtopenglwidgets.QOpenGLWidgetBase
	paints int
}

func (c *sizedCanvas) PaintGL() {
	c.paints++
	gl := c.Context().Functions()
	gl.GlClearColor(1, 0, 0, 1)
	gl.GlClear(qtgui.GL_COLOR_BUFFER_BIT)
}

// The widget renders at its size into a framebuffer that GrabFramebuffer
// reads back.
func TestOpenGLWidgetGrab(t *testing.T) {
	qutetest.Run(t, func() {
		c := &sizedCanvas{}
		w := qtopenglwidgets.NewQOpenGLWidgetWith(c, nil)
		defer w.Delete()
		w.Resize(40, 24)
		img := w.GrabFramebuffer()
		if c.paints == 0 {
			t.Skip("no OpenGL context available on this platform")
		}
		if img.Width() != 40 || img.Height() != 24 {
			t.Errorf("framebuffer %dx%d, want 40x24", img.Width(), img.Height())
		}
		if col := img.PixelColor(20, 12); col.R != 255 || col.G != 0 || col.B != 0 {
			t.Errorf("pixel %+v, want the red clear colour", col)
		}
	})
}

// A GL widget subclass inside a window is collected with the window.
func TestOpenGLWidgetCollectedWithWindow(t *testing.T) {
	before := qutetest.Freed()
	qutetest.Run(t, func() {
		win := qtwidgets.NewQWidget(nil)
		lay := qtwidgets.NewQVBoxLayout(win)
		lay.AddWidget(qtopenglwidgets.NewQOpenGLWidgetWith(&sizedCanvas{}, nil))
	})
	qutetest.CollectGarbage()
	if qutetest.Freed() == before {
		t.Error("the window with a GL widget was not freed")
	}
}
