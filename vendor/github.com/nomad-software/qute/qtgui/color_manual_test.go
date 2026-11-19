// Hand-written; qutegen never overwrites *_manual files.

package qtgui_test

import (
	"image/color"
	"testing"

	"github.com/nomad-software/qute/qtgui"
)

func TestParseColor(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want qtgui.QColor
	}{
		{"#f80", qtgui.QColor{R: 0xff, G: 0x88, B: 0x00, A: 0xff}},
		{"#102030", qtgui.QColor{R: 0x10, G: 0x20, B: 0x30, A: 0xff}},
		{"#80102030", qtgui.QColor{R: 0x10, G: 0x20, B: 0x30, A: 0x80}}, // Qt's #aarrggbb
		{"102030", qtgui.QColor{R: 0x10, G: 0x20, B: 0x30, A: 0xff}},
	} {
		got, err := qtgui.ParseColor(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseColor(%q) = %v, %v; want %v", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"", "#12", "#12345", "#gggggg", "#123456789"} {
		if _, err := qtgui.ParseColor(bad); err == nil {
			t.Errorf("ParseColor(%q) succeeded, want an error", bad)
		}
	}
}

func TestColorConversions(t *testing.T) {
	c := qtgui.RGBAColor(10, 20, 30, 40)
	if got := qtgui.ColorFromGo(c); got != c {
		t.Errorf("ColorFromGo(QColor) = %v, want %v", got, c)
	}
	if got := qtgui.ColorFromGo(color.Gray{Y: 0x40}); got != qtgui.RGB(0x40, 0x40, 0x40) {
		t.Errorf("ColorFromGo(Gray) = %v", got)
	}
	if got := qtgui.RGB(1, 2, 255).Name(); got != "#0102ff" {
		t.Errorf("Name() = %q", got)
	}
	if (qtgui.QColor{}).IsValid() || !qtgui.RGB(0, 0, 0).IsValid() {
		t.Error("IsValid: the zero value is invalid, opaque black is valid")
	}
}
