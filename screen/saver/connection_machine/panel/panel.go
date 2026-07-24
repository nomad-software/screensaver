package panel

import (
	"math/rand"

	"github.com/nomad-software/screensaver/screen/saver/connection_machine/panel/block"
)

const (
	bitChance = 4
)

type Panel struct {
	width  int // With of the panel in blocks.
	height int // Height of the panel in blocks.

	rows []uint16
}

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

func (p *Panel) Width() int {
	return p.width
}

func (p *Panel) Height() int {
	return p.height
}

func (p *Panel) PixelWidth() int {
	return p.width * block.Size()
}

func (p *Panel) BlockOn(width, height int) bool {
	return isBitSet(p.rows[height], width)
}

func (p *Panel) Iterate() {
	for i := range p.rows {
		bit := uint16(0)

		if rand.Intn(bitChance) == 0 {
			bit = 1
		}

		if (i/4)%2 == 0 {
			p.rows[i] = (p.rows[i] << 1) | bit
		} else {
			p.rows[i] = (p.rows[i] >> 1) | (bit << 15)
		}
	}
}

func isBitSet(v uint16, bit int) bool {
	if bit > 15 {
		return false
	}
	return (v & (uint16(1) << (15 - bit))) != 0
}
