package assets

import (
	"embed"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/assets"
)

var (
	//go:embed images/cell.png
	fs embed.FS

	store = assets.New(fs)
)

// Cell models the loaded sheet and hold dimensions of glyphs.
type Cell struct {
	texture       rl.Texture2D // The loaded glyph texture.
	textureWidth  int          // The overall width of the glyph texture.
	textureHeight int          // The overall height of the glyph texture.
}

// NewCell creates a new glyph sheet.
func NewCell() *Cell {
	g := &Cell{
		texture:       store.LoadPngTexture("images/cell.png"),
		textureWidth:  32,
		textureHeight: 32,
	}

	return g
}

// Texture returns the texture.
func (g *Cell) Texture() rl.Texture2D {
	return g.texture
}

// TextureWidth returns the width of the Texture.
func (g *Cell) TextureWidth() int {
	return g.textureWidth
}

// TextureHeight returns the height of the Texture.
func (g *Cell) TextureHeight() int {
	return g.textureHeight
}
