package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver"
	rain "github.com/nomad-software/screensaver/screen/saver/digital_rain/assets"
	"github.com/nomad-software/screensaver/screen/saver/digital_rain/matrix"
	"github.com/nomad-software/screensaver/screen/saver/digital_rain_glitched/assets"
	"github.com/nomad-software/screensaver/screen/saver/digital_rain_glitched/crt"
)

const (
	rainFPS   = 15 // How many times a second the rain moves.
	screenFPS = 60 // How many times a second the screen is drawn.
)

func main() {
	width, height := saver.CreateWindow("screensaver - digital rain glitched")
	defer saver.CloseWindow()

	// The glitches need to move far more smoothly than the rain, so the rain
	// is drawn onto a texture buffer each time it moves, and that buffer is
	// drawn to the screen through the glitch shader on every frame.
	rl.SetTargetFPS(screenFPS)

	sheet := rain.NewGlyphSheet()
	shader := rain.NewShaderCollection()
	matrix := matrix.New(width/sheet.GlyphWidth(), (height/sheet.GlyphHeight())+1)
	monitor := crt.New(assets.NewGlitchShader(), width, height)

	buffer := rl.LoadRenderTexture(int32(width), int32(height))
	defer rl.UnloadRenderTexture(buffer)
	rl.SetTextureFilter(buffer.Texture, rl.FilterBilinear)

	for frame := 0; !saver.InputDetected(); frame++ {
		if frame%(screenFPS/rainFPS) == 0 {
			rl.BeginTextureMode(buffer)
			rl.ClearBackground(rl.Black)
			drawMatrix(matrix, sheet, shader)
			rl.EndTextureMode()

			matrix.Iterate()
		}

		monitor.Update(rl.GetTime())

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)

		rl.BeginShaderMode(monitor.Shader())

		// The negative texture height is important because raylib's render
		// textures are vertically flipped.
		rl.DrawTextureRec(
			buffer.Texture,
			rl.NewRectangle(0, 0, float32(buffer.Texture.Width), -float32(buffer.Texture.Height)),
			rl.NewVector2(0, 0),
			rl.White,
		)

		rl.EndShaderMode()

		rl.EndDrawing()
	}
}

// drawMatrix draws every glyph of the matrix.
func drawMatrix(matrix *matrix.Matrix, sheet *rain.GlyphSheet, shader *rain.ShaderCollection) {
	offset := rl.NewVector2(
		float32((sheet.MaskWidth()-sheet.GlyphWidth())/2),
		float32((sheet.MaskHeight()-sheet.GlyphHeight())/2),
	)

	for x := 0; x < matrix.Width(); x++ {
		for y := 0; y < matrix.Height(); y++ {
			glyph := matrix.ColumnAtIndex(x).GlyphAtIndex(y)

			if glyph.IsEmpty() {
				continue
			}

			pos := rl.NewVector2(
				float32(x*sheet.GlyphWidth()),
				float32(y*sheet.GlyphHeight()),
			)

			pos = rl.Vector2Subtract(pos, offset)

			if glyph.IsHighlighted() {
				rl.BeginShaderMode(shader.Highlight())
				rl.DrawTextureRec(sheet.Texture(), sheet.Masks[glyph.Index()], pos, rl.White)
				rl.EndShaderMode()

			} else if glyph.IsHighlightFading() {
				rl.BeginShaderMode(shader.HighlightFading())
				rl.DrawTextureRec(sheet.Texture(), sheet.Masks[glyph.Index()], pos, rl.White)
				rl.EndShaderMode()

			} else {
				rl.DrawTextureRec(sheet.Texture(), sheet.Masks[glyph.Index()], pos, rl.White)
			}
		}
	}
}
