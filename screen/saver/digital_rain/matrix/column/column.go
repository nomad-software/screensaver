package column

import (
	"math/rand"

	"github.com/nomad-software/screensaver/screen/saver/digital_rain/matrix/glyph"
)

const (
	startColumnChance  = 50
	deleteColumnChance = 40

	highlightedGlyphChance        = 5
	highlightedGlyphStutterChance = 10
)

// Column is a type that holds an individual column of the matrix code.
type Column struct {
	iteration int // The number of interations for this column.
	depth     int // The depth (number of glyphs) of this column.

	glyphs []*glyph.Glyph // The collection of glyphs in this column.
}

// NewColumn creates a new matrix code column.
func NewColumn(depth int) *Column {
	col := &Column{
		iteration: 0,
		depth:     depth,
		glyphs:    make([]*glyph.Glyph, depth),
	}

	for i := 0; i < depth; i++ {
		col.SetGlyphAtIndex(i, glyph.NewEmptyGlyph())
	}

	return col
}

// GlyphAtIndex gets the glyph at the specified index.
func (c *Column) GlyphAtIndex(index int) *glyph.Glyph {
	return c.glyphs[index]
}

// SetGlyphAtIndex sets the glyph at the specified index.
func (c *Column) SetGlyphAtIndex(index int, g *glyph.Glyph) {
	c.glyphs[index] = g
}

// GlyphBeforeIndex gets the glyph before the specified index from the top down.
func (c *Column) GlyphBeforeIndex(index int) *glyph.Glyph {
	return c.glyphs[index-1]
}

// AppendGlyphs add new glyphs to columns.
func (c *Column) AppendGlyphs() {
	for i := c.depth - 1; i > -1; i-- {
		if i == 0 {
			// Always append one to the start of the column if chance favours it.
			if c.GlyphAtIndex(i).IsEmpty() {
				if rand.Intn(startColumnChance) == 0 {
					if rand.Intn(highlightedGlyphChance) == 0 {
						c.SetGlyphAtIndex(i, glyph.NewRandomHighlightedGlyph())
					} else {
						c.SetGlyphAtIndex(i, glyph.NewRandomGlyph())
					}
				}
			}
		} else {
			// If we're at the bottom and the glyph is not empty and
			// highlighted, remove the highlight.
			if i == c.depth-1 {
				if c.GlyphAtIndex(i).IsNotEmpty() {
					if c.GlyphAtIndex(i).IsHighlighted() {
						c.GlyphAtIndex(i).RemoveHighlight()
					}
				}
			}

			// Continue appending glyphs to the column.
			if c.GlyphAtIndex(i).IsEmpty() && c.GlyphBeforeIndex(i).IsNotEmpty() {
				if c.GlyphBeforeIndex(i).IsHighlighted() {
					// If the glyph is highlighted and if chance favours it,
					// skip adding a new one.
					if rand.Intn(highlightedGlyphStutterChance) != 0 {
						c.GlyphBeforeIndex(i).RemoveHighlight()
						c.SetGlyphAtIndex(i, glyph.NewRandomHighlightedGlyph())
					}
				} else {
					c.SetGlyphAtIndex(i, glyph.NewRandomGlyph())
				}
			}
		}
	}
}

// DeleteGlyphs deletes glyphs starting from the top of a column.
func (c *Column) DeleteGlyphs() {
	for i := c.depth - 1; i >= 0; i-- {
		// Start deleting at the start of the column if chance favours it.
		if i == 0 {
			if c.GlyphAtIndex(i).IsNotEmpty() {
				if rand.Intn(deleteColumnChance) == 0 {
					c.SetGlyphAtIndex(i, glyph.NewEmptyGlyph())
				}
			}
		} else {
			// Once a column is deleting, we continue, deleting the glyph at the
			// end of the deleted run of glyphs.
			if c.GlyphAtIndex(i).IsNotEmpty() && c.GlyphBeforeIndex(i).IsEmpty() {
				c.SetGlyphAtIndex(i, glyph.NewEmptyGlyph())
			}
		}
	}
}

// Iterate moves the column to the next state.
func (c *Column) Iterate() {
	for _, g := range c.glyphs {
		g.Iterate()
	}

	c.AppendGlyphs()
	c.DeleteGlyphs()

	c.iteration++
}
