// Hand-written; qutegen never overwrites *_manual.go files.

package qtgui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// QColor is Qt's QColor as a plain 8-bit RGBA value. An invalid QColor
// crosses as the zero value. It implements image/color.Color.
type QColor struct{ R, G, B, A uint8 }

// RGBA implements color.Color (alpha-premultiplied, 16-bit).
func (c QColor) RGBA() (r, g, b, a uint32) {
	return color.NRGBA{c.R, c.G, c.B, c.A}.RGBA()
}

// RGB returns an opaque colour.
func RGB(r, g, b uint8) QColor { return QColor{r, g, b, 255} }

// RGBAColor returns a colour with alpha.
func RGBAColor(r, g, b, a uint8) QColor { return QColor{r, g, b, a} }

// ColorFromGo converts any image/color.Color.
func ColorFromGo(c color.Color) QColor {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return QColor{n.R, n.G, n.B, n.A}
}

// ParseColor parses "#rgb", "#rrggbb" or "#aarrggbb" (Qt's order).
func ParseColor(s string) (QColor, error) {
	h := strings.TrimPrefix(s, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil || (len(h) != 6 && len(h) != 8) {
		return QColor{}, fmt.Errorf("qtgui.ParseColor: %q is not #rgb, #rrggbb or #aarrggbb", s)
	}
	if len(h) == 6 {
		return QColor{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
	}
	return QColor{uint8(v >> 16), uint8(v >> 8), uint8(v), uint8(v >> 24)}, nil
}

// Name returns "#rrggbb".
func (c QColor) Name() string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// IsValid reports whether the colour is not the zero (invalid) value.
func (c QColor) IsValid() bool { return c != QColor{} }
