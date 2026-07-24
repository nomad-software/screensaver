package block

import rl "github.com/gen2brain/raylib-go/raylib"

const (
	size = 30 // Size of a block in pixles.
)

func Size() int {
	return size
}

func SizeV() rl.Vector2 {
	return rl.Vector2{X: size, Y: size}
}
