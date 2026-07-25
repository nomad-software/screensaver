package block

import (
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	size = 20 // Size of a block in pixles.
)

func Size() int {
	return size
}

func SizeV() rl.Vector2 {
	return rl.Vector2{X: size, Y: size}
}

func Colour() color.RGBA {
	return color.RGBA{0xC8, 0x0, 0x0, 0xFF}
}
