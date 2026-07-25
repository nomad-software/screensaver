package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver"
	"github.com/nomad-software/screensaver/screen/saver/connection_machine/assets"
	"github.com/nomad-software/screensaver/screen/saver/connection_machine/panel"
	"github.com/nomad-software/screensaver/screen/saver/connection_machine/panel/block"
)

func main() {
	width, height := saver.CreateWindow("screensaver - connection machine")
	defer saver.CloseWindow()

	rl.SetTargetFPS(10)

	shader := assets.NewShaderCollection()
	blur := shader.Blur(width, height)

	buffer := rl.LoadRenderTexture(int32(width), int32(height))
	defer rl.UnloadRenderTexture(buffer)

	panels := []*panel.Panel{
		panel.New((height / block.Size()) + 1),
		panel.New((height / block.Size()) + 1),
		panel.New((height / block.Size()) + 1),
		panel.New((height / block.Size()) + 1),
	}

	for {
		if saver.InputDetected() {
			break
		}

		// Draw the entire scene onto the texure buffer.
		// This is to enable shader manipulation later.
		rl.BeginTextureMode(buffer)
		rl.ClearBackground(rl.Black)

		gap := ((width - (panels[0].PixelWidth() * len(panels))) / (len(panels) + 1))
		origin := gap

		for i, panel := range panels {
			for x := 0; x < panel.Width(); x++ {
				for y := 0; y < panel.Height(); y++ {

					if !panel.BlockOn(x, y) {
						continue
					}

					pos := rl.NewVector2(
						float32(origin+(x*block.Size())),
						float32(y*block.Size()),
					)

					rl.DrawRectangleV(pos, block.SizeV(), block.Colour())
				}
			}

			panel.Iterate(i)
			origin += panel.PixelWidth() + gap
		}

		rl.EndTextureMode()

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)

		rl.BeginShaderMode(blur)

		// Draw the texture buffer to the screen.
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
