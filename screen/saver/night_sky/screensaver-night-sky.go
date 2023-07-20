package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver"
	"github.com/nomad-software/screensaver/screen/saver/night_sky/assets"
)

func main() {
	saver.CreateWindow("screensaver - night sky")
	defer saver.CloseWindow()

	rl.SetTargetFPS(60)

	json := assets.NewJsonCollection()
	json.Prepare()

	texture := assets.NewTextureCollection()
	shader := assets.NewShaderCollection()

	camPos := rl.NewVector3(0.0, 0.0, 10.0)
	camTarget := rl.NewVector3(0.0, 0.0, 0.0)
	camUp := rl.NewVector3(0.0, 1.0, 0.0)
	cam := rl.NewCamera3D(camPos, camTarget, camUp, 45.0, rl.CameraPerspective)

	for !rl.WindowShouldClose() {
		// if saver.InputDetected() {
		// 	break
		// }

		rl.UpdateCamera(&cam, rl.CameraFree)

		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(0, 0, 8, 255))

		rl.BeginBlendMode(rl.BlendAdditive)
		rl.BeginShaderMode(shader.AlphaDiscard())
		rl.BeginMode3D(cam)

		for _, s := range json.Stars() {
			// if s.Magnitude > 10.0 {
			// 	continue
			// }

			source := rl.NewRectangle(0, 0, float32(texture.Star().Width), float32(texture.Star().Height))
			size := rl.NewVector2(s.Size, s.Size)
			origin := rl.NewVector2(s.Size/2, s.Size/2)

			rl.DrawBillboardPro(cam, texture.Star(), source, s.CoordsV, cam.Up, size, origin, 0.0, s.Color)
		}

		rl.EndMode3D()

		rl.EndShaderMode()
		rl.EndBlendMode()

		rl.DrawFPS(20, 20)
		rl.EndDrawing()
	}
}
