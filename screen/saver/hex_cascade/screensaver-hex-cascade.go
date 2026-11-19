package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver"
	"github.com/nomad-software/screensaver/screen/saver/hex_cascade/assets"
	"github.com/nomad-software/screensaver/screen/saver/hex_cascade/hive"
	"github.com/nomad-software/screensaver/screen/saver/hex_cascade/tile"
)

func main() {
	width, height := saver.CreateWindow("screensaver - hex cascade")
	defer saver.CloseWindow()

	rl.SetTargetFPS(60)

	hive := hive.New(width, height)

	shader := assets.NewShaderCollection()
	glow := shader.Glow(width, height, hive.Radius()*0.7)

	buffer := rl.LoadRenderTexture(int32(width), int32(height))
	defer rl.UnloadRenderTexture(buffer)

	// The glow samples around each pixel, so stop it wrapping round from one
	// edge of the screen to the other.
	rl.SetTextureWrap(buffer.Texture, rl.WrapClamp)

	for {
		if saver.InputDetected() {
			break
		}

		hive.Iterate(rl.GetFrameTime())

		// Draw the entire scene onto the texure buffer.
		// This is to enable shader manipulation later.
		rl.BeginTextureMode(buffer)
		rl.ClearBackground(rl.Black)

		// Settled tiles are drawn first, so that moving ones, which can
		// reach over their neighbours, are drawn on top of them.
		for _, c := range hive.Cells() {
			if !c.IsMoving() {
				tile.Draw(c.View(hive.Time()), hive.Radius())
			}
		}
		for _, c := range hive.Cells() {
			if c.IsMoving() {
				tile.Draw(c.View(hive.Time()), hive.Radius())
			}
		}

		rl.EndTextureMode()

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)

		rl.BeginShaderMode(glow)

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
