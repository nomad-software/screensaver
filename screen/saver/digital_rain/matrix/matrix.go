package matrix

import (
	"github.com/nomad-software/screensaver/screen/saver/digital_rain/matrix/column"
)

// Matrix is the type that controls all matrix logic.
type Matrix struct {
	width  int // The width in number of glyphs.
	height int // The height in number of glyphs.

	columns []*column.Column // Columns containing glyphs.
}

// New contructs a new matrix.
func New(width int, height int) *Matrix {
	m := &Matrix{
		width:   width,
		height:  height,
		columns: make([]*column.Column, width),
	}

	for i := 0; i < width; i++ {
		m.columns[i] = column.NewColumn(width, height)
	}

	return m
}

// Width returns the width of the matrix.
func (g *Matrix) Width() int {
	return g.width
}

// Height returns the height of the matrix.
func (g *Matrix) Height() int {
	return g.height
}

// ColumnAtIndex gets the column at the specified index.
func (m *Matrix) ColumnAtIndex(index int) *column.Column {
	return m.columns[index]
}

// Iterate moves the matrix along one step.
func (g *Matrix) Iterate() {
	for _, c := range g.columns {
		c.Iterate()
	}
}
