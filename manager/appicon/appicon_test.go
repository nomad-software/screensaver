package appicon

import (
	"bytes"
	"image/png"
	"testing"
)

// TestPNG checks the icon built in is a PNG of the size the desktop is told
// it is.
func TestPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(PNG))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != Size || b.Dy() != Size {
		t.Fatalf("the icon is %v, want %d square", b, Size)
	}
}
