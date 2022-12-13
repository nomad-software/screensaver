package glyph

import (
	"math/rand"
)

const (
	numberOfGlyphs          = 64
	highlightedGlyphChance  = 4
	switchGlyphChance       = 8
	switchGlyphSpreadChance = 32
)

// Glyph is a type that holds an individual glyph of the matrix code.
type Glyph struct {
	isEmpty          bool // A flag that defines if this glyph is empty.
	index            int  // The index of the glyph. The index is an integer between 0 and maximum-1 glyphs.
	isHighlighted    bool // A flag that defines if this glyph is highlighted.
	isSwitcher       bool // A flag that defines if this glyph switches.
	isSwitchSpreader bool // A flag that defines if this glyph spreads its switching to others.
}

// NewRandomGlyph creates a new random glyph.
func NewRandomGlyph() *Glyph {
	return &Glyph{
		index:            rand.Intn(numberOfGlyphs),
		isHighlighted:    (rand.Intn(highlightedGlyphChance) == 0),
		isSwitcher:       (rand.Intn(switchGlyphChance) == 0),
		isSwitchSpreader: (rand.Intn(switchGlyphSpreadChance) == 0),
	}
}

// NewRandomStandardGlyph creates a new random normal glyph.
func NewRandomStandardGlyph(isSwitchSpreader bool) *Glyph {
	return &Glyph{
		index:            rand.Intn(numberOfGlyphs),
		isSwitcher:       isSwitchSpreader || (rand.Intn(switchGlyphChance) == 0),
		isSwitchSpreader: isSwitchSpreader,
	}
}

// NewRandomHighlightedGlyph creates a new random highlighted glyph.
func NewRandomHighlightedGlyph(isSwitchSpreader bool) *Glyph {
	return &Glyph{
		index:            rand.Intn(numberOfGlyphs),
		isHighlighted:    true,
		isSwitcher:       isSwitchSpreader || (rand.Intn(switchGlyphChance) == 0),
		isSwitchSpreader: isSwitchSpreader,
	}
}

// NewEmptyGlyph creates a new empty glyph.
func NewEmptyGlyph() *Glyph {
	return &Glyph{
		isEmpty: true,
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

// RemoveHighlight removes the highlight from the glyph.
func (g *Glyph) RemoveHighlight() {
	g.isHighlighted = false
}

// IsSwitcher returns true if the glyph is a switcher.
func (g *Glyph) IsSwitcher() bool {
	return g.isSwitcher
}

// IsSwitcherSpreader returns true if the glyph is a switch spreader.
func (g *Glyph) IsSwitcherSpreader() bool {
	return g.isSwitchSpreader
}

// Switch switches the glyph.
func (g *Glyph) Switch() {
	g.index = rand.Intn(numberOfGlyphs)
}
