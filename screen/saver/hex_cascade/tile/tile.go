package tile

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver/hex_cascade/hive/cell"
)

const (
	inset    = 0.92 // The size a tile is drawn at, leaving a dark gap around it.
	flashMix = 0.3  // How white a tile goes when it has just landed.
)

var (
	// The light on each of the three faces of a tile in full relief, which
	// makes it look like a cube. Each face is a rhombus of two of the six
	// triangles, and they turn with the tile, so a tile that turns a sixth
	// looks like a cube turned inside out.
	faceLight = [3]float32{0.45, 0.7, 1.0}
)

// Draw draws a hexagonal tile as the view of a cell shows it.
func Draw(v cell.View, radius float32) {
	size := radius * inset * v.Size

	var corners [6]rl.Vector2
	for i := range corners {
		angle := float64(v.Rotation) + float64(i)*math.Pi/3
		p := rl.NewVector2(size*float32(math.Cos(angle)), size*float32(math.Sin(angle)))

		// A flipping tile is squashed along the direction it tips over.
		if v.Squash < 1 {
			along := rl.Vector2DotProduct(p, v.Axis)
			p = rl.Vector2Subtract(p, rl.Vector2Scale(v.Axis, along*(1-v.Squash)))
		}

		corners[i] = rl.Vector2Add(v.Centre, p)
	}

	for i := range corners {
		light := 1 - v.Relief*(1-faceLight[i/2])
		colour := shade(v.Colour, light*v.Brightness, v.Flash*flashMix)
		triangle(v.Centre, corners[i], corners[(i+1)%6], colour)
	}
}

// shade darkens a colour by the given light, then whitens it by the given
// flash.
func shade(c rl.Color, light, flash float32) rl.Color {
	mix := func(v uint8) uint8 {
		lit := float32(v) * light
		return uint8(min(255, lit+(255-lit)*flash))
	}
	return rl.NewColor(mix(c.R), mix(c.G), mix(c.B), 255)
}

// triangle draws a triangle whichever way round its corners are given, as
// raylib only draws those given anticlockwise.
func triangle(a, b, c rl.Vector2, colour rl.Color) {
	cross := (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
	if cross > 0 {
		b, c = c, b
	}
	rl.DrawTriangle(a, b, c, colour)
}
