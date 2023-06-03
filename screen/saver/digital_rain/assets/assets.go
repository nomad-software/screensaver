package assets

import (
	"embed"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/assets"
)

var (
	//go:embed glyphs.png
	fs embed.FS

	store = assets.New(fs)
)

// GlyphSheet models the loaded sheet and hold dimensions of glyphs.
type GlyphSheet struct {
	texture       rl.Texture2D // The loaded glyph texture.
	textureWidth  int          // The overall width of the glyph texture.
	textureHeight int          // The overall height of the glyph texture.

	glyphWidth  int // The width of a glyph.
	glyphHeight int // The height of a glyph.

	maskWidth  int // The overall width of a glyph mask.
	maskHeight int // The overall height of a glyph mask.

	Masks []rl.Rectangle // A collection of glyphs taken from the overall sprite sheet.
}

// NewGlyphSheet creates a new glyph sheet.
func NewGlyphSheet() *GlyphSheet {
	g := &GlyphSheet{
		texture:       store.LoadPngTexture("glyphs.png"),
		textureWidth:  480,
		textureHeight: 576,
		maskWidth:     60,
		maskHeight:    72,
		glyphWidth:    20,
		glyphHeight:   32,

		Masks: make([]rl.Rectangle, 0),
	}

	for y := 0; y < g.textureHeight; y += g.maskHeight {
		for x := 0; x < g.textureWidth; x += g.maskWidth {
			mask := rl.NewRectangle(
				float32(x),
				float32(y),
				float32(g.maskWidth),
				float32(g.maskHeight),
			)
			g.Masks = append(g.Masks, mask)
		}
	}

	return g
}

// Texture returns the texture.
func (g *GlyphSheet) Texture() rl.Texture2D {
	return g.texture
}

// GlyphWidth returns the width of a glyph.
func (g *GlyphSheet) GlyphWidth() int {
	return g.glyphWidth
}

// GlyphHeight returns the height of a glyph.
func (g *GlyphSheet) GlyphHeight() int {
	return g.glyphHeight
}

// MaskWidth returns the mask width of a glyph.
func (g *GlyphSheet) MaskWidth() int {
	return g.maskWidth
}

// MaskHeight returns the mask height of a glyph.
func (g *GlyphSheet) MaskHeight() int {
	return g.maskHeight
}
