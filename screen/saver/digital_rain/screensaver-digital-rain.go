package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/nomad-software/screensaver/screen"
	"github.com/nomad-software/screensaver/screen/saver/digital_rain/assets"
	"github.com/nomad-software/screensaver/screen/saver/digital_rain/matrix"
)

var (
	backgroundColour = color.NRGBA{0, 0, 0, 255}

	view  *matrix.Matrix
	asset *assets.Collection

	tick uint
)

type DigitalRain struct {
	screen.Saver
}

func (g *DigitalRain) Update() error {
	if view == nil {
		asset = assets.NewCollection()
		view = matrix.New(g.ScreenWidth/asset.GlyphWidth(), (g.ScreenHeight/asset.GlyphHeight())+1)
	}

	tick++

	if tick%4 == 0 {
		view.Iterate()
	}

	return g.Saver.Update()
}

func (g *DigitalRain) Draw(screen *ebiten.Image) {
	buffer := g.Blit(screen)
	buffer.Fill(backgroundColour)

	for x := 0; x < view.Width(); x++ {
		for y := 0; y < view.Height(); y++ {
			glyph := view.ColumnAtIndex(x).GlyphAtIndex(y)

			if !glyph.IsEmpty() {
				opt := &ebiten.DrawImageOptions{}
				// Adjust translation for the tile.
				opt.GeoM.Translate(-float64((asset.TileWidth()-asset.GlyphWidth())/2), -float64((asset.TileHeight()-asset.GlyphHeight())/2))

				// Translate for the actual glyph.
				opt.GeoM.Translate(float64(x*asset.GlyphWidth()), float64(y*asset.GlyphHeight()))

				// Alter the colour if the glyph is highlighted.
				if glyph.IsHighlighted() {
					opt.ColorM.Scale(2.5, 2.0, 2.5, 1.25)
				}

				buffer.DrawImage(asset.Images[glyph.Index()], opt)
			}
		}
	}
}

func (s *DigitalRain) Layout(width, height int) (int, int) {
	s.ScreenWidth = width
	s.ScreenHeight = height

	return s.ScreenWidth, s.ScreenHeight
}

func main() {
	ebiten.SetFullscreen(true)
	ebiten.SetCursorMode(ebiten.CursorModeHidden)

	saver := &DigitalRain{}
	screen.Run(saver)
}
