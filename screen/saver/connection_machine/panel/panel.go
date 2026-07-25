package panel

import (
	"math/rand"

	"github.com/nomad-software/screensaver/screen/saver/connection_machine/panel/block"
)

const (
	bitChance = 4 // Randomness of new blocks.
)

type Panel struct {
	width  int // With of the panel in blocks.
	height int // Height of the panel in blocks.

	rows []uint16 // Use bits to define the blocks.
}

// New creates a new panel.
func New(height int) *Panel {
	p := &Panel{
		width:  16,
		height: height,
		rows:   make([]uint16, height),
	}

	for i := 0; i < height; i++ {
		for bit := 0; bit < 16; bit++ {
			if rand.Intn(bitChance) == 0 {
				p.rows[i] |= 1 << bit
			}
		}
	}

	return p
}

// Width retuns the panel's width in blocks.
func (p *Panel) Width() int {
	return p.width
}

// Height retuns the panel's height in blocks.
func (p *Panel) Height() int {
	return p.height
}

// PixelWidth returns the panel's width in pixels.
func (p *Panel) PixelWidth() int {
	return p.width * block.Size()
}

// BlockOn returns true if a block is on.
func (p *Panel) BlockOn(width, height int) bool {
	if width > 15 {
		return false
	}
	return (p.rows[height] & (uint16(1) << (15 - width))) != 0
}

// Iterate takes a panel index to alternate the shifting pattern.
func (p *Panel) Iterate(panel int) {
	for i := range p.rows {
		bit := uint16(0)

		if rand.Intn(bitChance) == 0 {
			bit = 1
		}

		shiftLeft := (i/4)%2 == 0

		if panel%2 != 0 {
			shiftLeft = !shiftLeft
		}
		if shiftLeft {
			p.rows[i] = (p.rows[i] << 1) | bit
		} else {
			p.rows[i] = (p.rows[i] >> 1) | (bit << 15)
		}
	}
}
