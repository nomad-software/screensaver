package palette

import (
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Palette is a set of colours that look good together, brightest first.
type Palette [4]rl.Color

var (
	all = []Palette{
		// Neon.
		{hex(0x00e5ff), hex(0xff2bd6), hex(0x7b2cff), hex(0x1b1f6b)},
		// Ember.
		{hex(0xffb000), hex(0xff6b1a), hex(0xc2185b), hex(0x3a0a2e)},
		// Ocean.
		{hex(0x90e0ef), hex(0x00b4d8), hex(0x0077b6), hex(0x03045e)},
		// Jade.
		{hex(0x9fd8d2), hex(0x2ec4b6), hex(0x1b9e77), hex(0x0b3d3a)},
		// Aurora.
		{hex(0xe0ff4f), hex(0x3ddc97), hex(0x46b1c9), hex(0x7b5ea7)},
		// Honey.
		{hex(0xffd166), hex(0xffb627), hex(0xe2711d), hex(0x5c2e00)},
		// Orchid.
		{hex(0xf72585), hex(0xb5179e), hex(0x7209b7), hex(0x3a0ca3)},
		// Steel.
		{hex(0xb8c2cc), hex(0x778da9), hex(0x415a77), hex(0x1b263b)},
	}
)

// Cycle hands out every palette in a shuffled order before any is repeated.
type Cycle struct {
	order []int
	next  int
}

// NewCycle creates a new cycle of palettes.
func NewCycle() *Cycle {
	return &Cycle{
		order: rand.Perm(len(all)),
	}
}

// Next returns the next palette.
func (c *Cycle) Next() Palette {
	if c.next == len(c.order) {
		last := c.order[len(c.order)-1]
		c.order = rand.Perm(len(all))
		// Never show the same palette twice running across a reshuffle.
		if c.order[0] == last {
			c.order[0], c.order[len(c.order)-1] = c.order[len(c.order)-1], c.order[0]
		}
		c.next = 0
	}

	p := all[c.order[c.next]]
	c.next++
	return p
}

// hex creates a colour from a 0xrrggbb value.
func hex(v uint32) rl.Color {
	return rl.NewColor(uint8(v>>16), uint8(v>>8), uint8(v), 255)
}
