package glyph

import (
	"math/rand"
)

const (
	numberOfGlyphs    = 64
	switchGlyphChance = 8
)

// Glyph is a type that holds an individual glyph of the matrix code.
type Glyph struct {
	isEmpty       bool // A flag that defines if this glyph is empty.
	index         int  // The index of the glyph. The index is an integer between 0 and maximum-1 glyphs.
	isHighlighted bool // A flag that defines if this glyph is highlighted.
	isSwitcher    bool // A flag that defines if this glyph switches.
}

// NewGlyph creates a new matrix code glyph.
func NewGlyph(isEmpty bool, index int, isHighlighted bool, isSwitcher bool) *Glyph {
	return &Glyph{
		isEmpty:       isEmpty,
		index:         index,
		isHighlighted: isHighlighted,
		isSwitcher:    isSwitcher,
	}
}

// IsEmpty returns true if the glyph is empty.
func (g *Glyph) IsEmpty() bool {
	return g.isEmpty
}

// IsNotEmpty returns true if the glyph is not empty.
func (g *Glyph) IsNotEmpty() bool {
	return !g.isEmpty
}

// Index returns the index of the glyph.
func (g *Glyph) Index() int {
	return g.index
}

// IsHighlighted returns true if the glyph is highlighted.
func (g *Glyph) IsHighlighted() bool {
	return g.isHighlighted
}

// IsSwitcher returns true if the glyph is a switcher.
func (g *Glyph) IsSwitcher() bool {
	return g.isSwitcher
}

// Switch switches the glyph.
func (g *Glyph) Switch() {
	g.index = rand.Intn(numberOfGlyphs)
}

// RemoveHighlight removes the highlight from the glyph.
func (g *Glyph) RemoveHighlight() {
	g.isHighlighted = false
}

// NewRandomGlyph creates a new random glyph.
func NewRandomGlyph() *Glyph {
	return NewGlyph(false, rand.Intn(numberOfGlyphs), false, rand.Intn(switchGlyphChance) == 0)
}

// NewRandomHighlightedGlyph creates a new random highlighted glyph.
func NewRandomHighlightedGlyph() *Glyph {
	return NewGlyph(false, rand.Intn(numberOfGlyphs), true, rand.Intn(switchGlyphChance) == 0)
}

// NewEmptyGlyph creates a new empty glyph.
func NewEmptyGlyph() *Glyph {
	return NewGlyph(true, 0, false, false)
}
