package glyph

import "math/rand"

const (
	numberOfGlyphs    = 64
	switchGlyphChance = 16
)

// Glyph is a type that holds an individual glyph of the matrix code.
type Glyph struct {
	iteration     int  // The number of interations for this glyph.
	isEmpty       bool // A flag that defines if this glyph is empty.
	index         int  // The index of the glyph. The index is an integer between 0 and maximum-1 glyphs.
	isHighlighted bool // A flag that defines if this glyph is highlighted.
}

// NewGlyph creates a new matrix code glyph.
func NewGlyph(isEmpty bool, index int, isHighlighted bool) *Glyph {
	return &Glyph{
		isEmpty:       isEmpty,
		index:         index,
		isHighlighted: isHighlighted,
	}
}

// IsEmpty returns true if the glyph is empty.
func (g *Glyph) IsEmpty() bool {
	return g.isEmpty
}

// Index returns the index of the glyph.
func (g *Glyph) Index() int {
	return g.index
}

// IsHighlighted returns true if the glyph is highlighted.
func (g *Glyph) IsHighlighted() bool {
	return g.isHighlighted
}

// RemoveHighlight removes the highlight from the glyph.
func (g *Glyph) RemoveHighlight() {
	g.isHighlighted = false
}

// NewRandomGlyph creates a new random glyph.
func NewRandomGlyph() *Glyph {
	return NewGlyph(false, rand.Intn(numberOfGlyphs), false)
}

// NewRandomHighlightedGlyph creates a new random highlighted glyph.
func NewRandomHighlightedGlyph() *Glyph {
	return NewGlyph(false, rand.Intn(numberOfGlyphs), true)
}

// NewEmptyGlyph creates a new empty glyph.
func NewEmptyGlyph() *Glyph {
	return NewGlyph(true, 0, false)
}

// Iterate moves the glyph to the next state.
func (g *Glyph) Iterate() {
	if !g.IsEmpty() {
		if g.iteration%3 == 0 {
			if rand.Intn(switchGlyphChance) == 0 {
				g.index = rand.Intn(numberOfGlyphs)
			}
		}
	}

	g.iteration++
}
