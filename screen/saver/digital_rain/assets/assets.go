package assets

import (
	"embed"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/nomad-software/screensaver/assets"
)

var (
	//go:embed *.png
	fs embed.FS

	store = assets.New(fs)
	sheet = store.LoadImage("glyphs.png")
)

// Collection is the parsed collection of matrix glyphs.
type Collection struct {
	tileWidth  int // The overall width of the glyph tile.
	tileHeight int // The overall height of the glyph tile.

	glyphWidth  int // The actual width of the glyph.
	glyphHeight int // The actual height of the glyph.

	Images []*ebiten.Image // A collection of glyphs taken from the overall sprite sheet.
}

// NewCollection creates a new glyph collection.
func NewCollection() *Collection {
	col := &Collection{
		tileWidth:   60,
		tileHeight:  72,
		glyphWidth:  20,
		glyphHeight: 32,

		Images: make([]*ebiten.Image, 0),
	}

	for y := 0; y < sheet.Bounds().Dy(); y = y + col.tileHeight {
		for x := 0; x < sheet.Bounds().Dx(); x = x + col.tileWidth {
			col.Images = append(col.Images, sheet.SubImage(image.Rect(x, y, x+col.tileWidth, y+col.tileHeight)).(*ebiten.Image))
		}
	}

	return col
}

// TileWidth returns the tile width of the glyph.
func (c *Collection) TileWidth() int {
	return c.tileWidth
}

// TileHeight returns the tile height of the glyph.
func (c *Collection) TileHeight() int {
	return c.tileHeight
}

// GlyphWidth returns the actual width of the glyph.
func (c *Collection) GlyphWidth() int {
	return c.glyphWidth
}

// GlyphHeight returns the actual height of the glyph.
func (c *Collection) GlyphHeight() int {
	return c.glyphHeight
}
