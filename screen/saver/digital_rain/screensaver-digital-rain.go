package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver"
	"github.com/nomad-software/screensaver/screen/saver/digital_rain/assets"
	"github.com/nomad-software/screensaver/screen/saver/digital_rain/matrix"
)

func main() {
	width, height := saver.CreateWindow("screensaver - digital rain")
	defer saver.CloseWindow()

	rl.SetTargetFPS(15)

	sheet := assets.NewGlyphSheet()
	matrix := matrix.New(width/sheet.GlyphWidth(), (height/sheet.GlyphHeight())+1)

	for {
		if saver.InputDetected() {
			break
		}

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)

		for x := 0; x < matrix.Width(); x++ {
			for y := 0; y < matrix.Height(); y++ {
				glyph := matrix.ColumnAtIndex(x).GlyphAtIndex(y)

				if glyph.IsEmpty() {
					continue
				}

				offset := rl.NewVector2(
					float32((sheet.MaskWidth()-sheet.GlyphWidth())/2),
					float32((sheet.MaskHeight()-sheet.GlyphHeight())/2),
				)

				pos := rl.NewVector2(
					float32(x*sheet.GlyphWidth()),
					float32(y*sheet.GlyphHeight()),
				)

				pos = rl.Vector2Subtract(pos, offset)

				rl.DrawTextureRec(sheet.Texture(), sheet.Masks[glyph.Index()], pos, rl.White)

				rl.BeginBlendMode(rl.BlendAdditive)

				if glyph.IsHighlightFading() {
					rl.DrawTextureRec(sheet.Texture(), sheet.Masks[glyph.Index()], pos, rl.White)
				}

				if glyph.IsHighlighted() {
					rl.DrawTextureRec(sheet.Texture(), sheet.Masks[glyph.Index()], pos, rl.White)
					rl.DrawTextureRec(sheet.Texture(), sheet.Masks[glyph.Index()], pos, rl.White)
				}

				rl.EndBlendMode()
			}
		}

		matrix.Iterate()

		rl.EndDrawing()
	}
}
