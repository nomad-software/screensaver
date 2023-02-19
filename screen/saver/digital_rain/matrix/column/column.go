package column

import (
	"math/rand"

	"github.com/nomad-software/screensaver/screen/saver/digital_rain/matrix/glyph"
)

// Column is a type that holds an individual column of the matrix code.
type Column struct {
	iteration int // The number of interations for this column.
	width     int // The number of columns used overall. This helps determine random chances.
	height    int // The height (number of glyphs) of this column.

	startColumnChance             int
	deleteColumnChance            int
	highlightedGlyphStutterChance int

	glyphs []*glyph.Glyph // The collection of glyphs in this column.
}

// NewColumn creates a new matrix code column.
func NewColumn(width, height int) *Column {
	col := &Column{
		iteration: 0,
		height:    height,
		width:     width,

		startColumnChance:             int(float64(width) / 2.5),
		deleteColumnChance:            int(float64(width) / 3.0),
		highlightedGlyphStutterChance: int(float64(width) / 12.5),

		glyphs: make([]*glyph.Glyph, height),
	}

	for i := 0; i < height; i++ {
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

// GlyphAfterIndex gets the glyph after the specified index from the top down.
func (c *Column) GlyphAfterIndex(index int) *glyph.Glyph {
	return c.glyphs[index+1]
}

// FadeGlyphs fades highlighted glyphs.
func (c *Column) FadeGlyphs() {
	for i := 0; i < c.height; i++ {
		// If there are any glyphs currently fading, remove the highlight.
		if c.GlyphAtIndex(i).IsHighlightFading() {
			c.GlyphAtIndex(i).RemoveHighlight()
		}

		if i < c.height-1 {
			// Fade any highlighted glyph that has another added after it.
			if c.GlyphAtIndex(i).IsNotEmpty() && c.GlyphAtIndex(i).IsHighlighted() && c.GlyphAfterIndex(i).IsNotEmpty() {
				c.GlyphAtIndex(i).FadeHighlight()
			}
		} else {
			// The bottom glyph is handled differently as you don't have to
			// check the one after.
			if c.GlyphAtIndex(i).IsNotEmpty() && c.GlyphAtIndex(i).IsHighlighted() {
				c.GlyphAtIndex(i).FadeHighlight()
			}
		}
	}
}

// AppendGlyphs add new glyphs to columns.
func (c *Column) AppendGlyphs() {
	for i := c.height - 1; i > -1; i-- {
		if i == 0 {
			// Always append one to the start of the column if chance favours it.
			if c.GlyphAtIndex(i).IsEmpty() && c.GlyphAfterIndex(i).IsEmpty() {
				if rand.Intn(c.startColumnChance) == 0 {
					c.SetGlyphAtIndex(i, glyph.NewRandomGlyph())
				}
			}
		} else {
			// Continue appending glyphs to the column.
			if c.GlyphAtIndex(i).IsEmpty() && c.GlyphBeforeIndex(i).IsNotEmpty() {
				if c.GlyphBeforeIndex(i).IsHighlighted() {
					// If the glyph is highlighted and if chance favours it,
					// add a new highlighted one.
					if rand.Intn(c.highlightedGlyphStutterChance) != 0 {
						c.SetGlyphAtIndex(i, glyph.NewRandomHighlightedGlyph(c.GlyphBeforeIndex(i).IsSwitcherSpreader()))
					}
				} else {
					c.SetGlyphAtIndex(i, glyph.NewRandomStandardGlyph(c.GlyphBeforeIndex(i).IsSwitcherSpreader()))
				}
			}
		}
	}
}

// SwitchGlyphs switches glyphs if glyph is a switcher.
func (c *Column) SwitchGlyphs() {
	for i := 0; i < c.height; i++ {
		if c.GlyphAtIndex(i).IsNotEmpty() {
			if c.GlyphAtIndex(i).IsSwitcher() && c.iteration%3 == 0 {
				c.GlyphAtIndex(i).Switch()
			}
		}
	}
}

// DeleteGlyphs deletes glyphs starting from the bottom of a column.
func (c *Column) DeleteGlyphs() {
	for i := c.height - 1; i >= 0; i-- {
		// Start deleting at the start of the column if chance favours it.
		if i == 0 {
			if c.GlyphAtIndex(i).IsNotEmpty() {
				if rand.Intn(c.deleteColumnChance) == 0 {
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
	c.AppendGlyphs()
	c.SwitchGlyphs()
	c.DeleteGlyphs()
	c.FadeGlyphs()

	c.iteration++
}
