package panel

import (
	"math/rand"

	"github.com/nomad-software/screensaver/screen/saver/connection_machine/panel/block"
)

const (
	bitChance = 4 // Randomness of new blocks.
)

type Panel struct {
	columns int // With of the panel in blocks.
	rows    int // Height of the panel in blocks.

	panel []uint16 // Use bits to define the blocks.
}

// New creates a new panel.
func New(rows int) *Panel {
	p := &Panel{
		columns: 16,
		rows:    rows,
		panel:   make([]uint16, rows),
	}

	for row := 0; row < rows; row++ {
		for shift := 0; shift < p.columns; shift++ {
			if rand.Intn(bitChance) == 0 {
				p.panel[row] |= 1 << shift
			}
		}
	}

	return p
}

// Columns retuns the amount of columns in the panel.
func (p *Panel) Columns() int {
	return p.columns
}

// Rows retuns the amount of rows in the panel.
func (p *Panel) Rows() int {
	return p.rows
}

// Width returns the overall panel's width in pixels.
func (p *Panel) Width() int {
	return p.columns * block.Size()
}

// IsBlockLit returns true if a block is lit.
func (p *Panel) IsBlockLit(column, row int) bool {
	if column > 15 {
		return false
	}
	return (p.panel[row] & (uint16(1) << (15 - column))) != 0
}

// Iterate moves the pattern on one frame and takes a panel index to alternate
// the shifting pattern.
func (p *Panel) Iterate(panel int) {
	for i := range p.panel {
		bit := uint16(0)

		if rand.Intn(bitChance) == 0 {
			bit = 1
		}

		shiftLeft := (i/4)%2 == 0

		if panel%2 != 0 {
			shiftLeft = !shiftLeft
		}

		if shiftLeft {
			p.panel[i] = (p.panel[i] << 1) | bit
		} else {
			p.panel[i] = (p.panel[i] >> 1) | (bit << 15)
		}
	}
}
