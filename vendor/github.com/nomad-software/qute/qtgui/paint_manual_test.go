// Hand-written; qutegen never overwrites *_manual files.

package qtgui_test

import (
	"strings"
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qutetest"
)

func newTestImage() *qtgui.QImage {
	img := qtgui.NewQImageWithSize(qtcore.QSize{Width: 4, Height: 4}, qtgui.QImageFormat_ARGB32)
	img.Fill(0)
	return img
}

// panicText runs f and returns what it panicked with, or "".
func panicText(f func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg, _ = r.(string)
			if msg == "" {
				msg = "non-string panic"
			}
		}
	}()
	f()
	return ""
}

// PaintOn draws, and its painter is ended, freed and Dead when it returns:
// the image can be deleted afterwards, which a live painter would forbid.
func TestPaintOnDraws(t *testing.T) {
	qutetest.Run(t, func() {
		img := newTestImage()
		var painter *qtgui.QPainter
		qtgui.PaintOn(img, func(p *qtgui.QPainter) {
			painter = p
			if !p.IsActive() {
				t.Error("the painter is not active")
			}
			p.FillRect(qtcore.QRect{Width: 4, Height: 4}, qtgui.RGB(255, 0, 0))
		})
		if painter.Alive() {
			t.Error("the painter is alive after PaintOn returned")
		}
		if px := img.Pixel(1, 1); px != 0xffff0000 {
			t.Errorf("pixel %#x, want red", px)
		}
		if msg := panicText(func() { painter.IsActive() }); !strings.Contains(msg, "rule 6") {
			t.Errorf("use after PaintOn: panic %q, want rule 6", msg)
		}
		img.Delete()
	})
}

// While the painter draws, its device cannot be deleted, and the painter
// itself is not Go's to delete.
func TestPaintOnGuards(t *testing.T) {
	qutetest.Run(t, func() {
		img := newTestImage()
		qtgui.PaintOn(img, func(p *qtgui.QPainter) {
			if msg := panicText(img.Delete); !strings.Contains(msg, "a QPainter still uses this QImage") {
				t.Errorf("Delete of the device inside PaintOn: panic %q", msg)
			}
			if msg := panicText(p.Delete); !strings.Contains(msg, "rule 4") {
				t.Errorf("Delete of the painter inside PaintOn: panic %q, want rule 4", msg)
			}
		})
		img.Delete()
		if img.Alive() {
			t.Error("the image survived Delete after PaintOn")
		}
	})
}

// A panic in f still ends and frees the painter, so the device is free again.
func TestPaintOnPanic(t *testing.T) {
	qutetest.Run(t, func() {
		img := newTestImage()
		var painter *qtgui.QPainter
		msg := panicText(func() {
			qtgui.PaintOn(img, func(p *qtgui.QPainter) {
				painter = p
				panic("boom")
			})
		})
		if msg != "boom" {
			t.Errorf("panic %q, want boom", msg)
		}
		if painter.Alive() {
			t.Error("the painter is alive after f panicked")
		}
		img.Delete()
	})
}

// An object built on the painter and left alive (a state guard) ends before
// the painter does, as its C++ scope would have.
func TestPaintOnEndsBoundObjects(t *testing.T) {
	qutetest.Run(t, func() {
		img := newTestImage()
		var guard *qtgui.QPainterStateGuard
		qtgui.PaintOn(img, func(p *qtgui.QPainter) {
			guard = qtgui.NewQPainterStateGuardWithPainter(p)
			p.SetOpacity(0.5)
		})
		if guard.Alive() {
			t.Error("the state guard outlived the PaintOn painter")
		}
		img.Delete()
	})
}

func TestPaintOnNilDevice(t *testing.T) {
	qutetest.Run(t, func() {
		if msg := panicText(func() { qtgui.PaintOn(nil, func(*qtgui.QPainter) {}) }); !strings.Contains(msg, "device is nil") {
			t.Errorf("nil device: panic %q", msg)
		}
	})
}
