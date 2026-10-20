package display

import (
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	virtualWidth = 320 // Rough width of the low resolution display in pixels.
	fontSize     = 10  // Size of the default raylib font in pixels.
	fontSpacing  = 1   // Spacing between characters in pixels.
	lineHeight   = 12  // Height of a line of text in pixels.
)

var (
	Background   = color.RGBA{0x06, 0x0C, 0x38, 0xFF}
	Text         = color.RGBA{0xC8, 0xE0, 0xFF, 0xFF}
	Border       = color.RGBA{0x90, 0xC8, 0xFF, 0xFF}
	Cell         = color.RGBA{0x3C, 0xA8, 0xFF, 0xFF}
	CellCore     = color.RGBA{0x5C, 0xB8, 0xFF, 0xFF}
	Intruder     = color.RGBA{0xD8, 0x50, 0x30, 0xFF}
	IntruderCore = color.RGBA{0xF0, 0x80, 0x50, 0xFF}
	Infection    = color.RGBA{0xD8, 0x4C, 0x34, 0xFF}
	Spear        = color.RGBA{0xA8, 0xDC, 0xFF, 0x90}
)

// Scale returns how many screen pixels are used to draw one virtual pixel.
// Keeping this an integer stops the pixels being drawn unevenly.
func Scale(width int) int {
	return max(1, (width+virtualWidth/2)/virtualWidth)
}

// LineHeight returns the height of a line of text.
func LineHeight() int {
	return lineHeight
}

// MeasureText returns the width of the passed text in pixels.
func MeasureText(text string) int {
	return int(rl.MeasureTextEx(rl.GetFontDefault(), text, fontSize, fontSpacing).X)
}

// DrawText draws a line of text with its top left corner at the passed position.
func DrawText(text string, x, y int) {
	pos := rl.NewVector2(float32(x), float32(y))
	rl.DrawTextEx(rl.GetFontDefault(), text, pos, fontSize, fontSpacing, Text)
}
