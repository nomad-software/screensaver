package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver"
	"github.com/nomad-software/screensaver/screen/saver/night_sky/assets"
)

func main() {
	width, height := saver.CreateWindow("screensaver - night sky")
	defer saver.CloseWindow()

	rl.SetTargetFPS(60)
	// rl.SetConfigFlags(rl.FlagMsaa4xHint)
	// rl.SetConfigFlags(rl.FlagVsyncHint)

	json := assets.NewJsonCollection()
	json.Prepare()

	texture := assets.NewTextureCollection()
	shader := assets.NewShaderCollection()

	cx := width / 2
	cy := height / 2
	println(cx)
	println(cy)

	camPos := rl.NewVector3(0.0, 0.0, 10.0)
	camTarget := rl.NewVector3(0.0, 0.0, 0.0)
	camUp := rl.NewVector3(0.0, 1.0, 0.0)
	cam := rl.NewCamera3D(camPos, camTarget, camUp, 45.0, rl.CameraPerspective)

	// var yAngle float32 = 0
	// var xRot float32 = 0
	// var yRot float32 = 0
	// var zRot float32 = 0

	for !rl.WindowShouldClose() {

		// if saver.InputDetected() {
		// 	break
		// }

		// m := rl.GetMouseDelta()

		// if rl.IsKeyDown(rl.KeyW) {
		// xRot += m.X
		// }

		// if xRot > 0 {
		// xRot *= 0.1
		// }

		// yAngle += 0.001
		// cam.Target.Y = float32(math.Sin(float64(yAngle)) * 100)
		// cam.Target.Z = float32(math.Cos(float64(yAngle)) * 100)

		// rl.UpdateCamera(&cam, rl.CameraOrbital)

		// m := rl.GetMouseDelta()
		// pos := rl.NewVector3(0, 0, 0)
		// rot := rl.NewVector3(xRot, yRot, zRot)
		// rl.UpdateCameraPro(&cam, pos, rot, 0.0)

		// Polaris
		// cam.Target.X = 1.0126985208311208
		// cam.Target.Y = 0.7899131402574538
		// cam.Target.Z = 99.99175205454077

		// Betelgeuse
		// cam.Target.X = 2.0889856993660962
		// cam.Target.Y = 99.14352253171715
		// cam.Target.Z = 12.8917833813145

		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(0, 0, 8, 255))

		// rl.BeginBlendMode(rl.BlendAdditive)
		rl.BeginShaderMode(shader.Highlight())
		rl.BeginMode3D(cam)

		for _, s := range json.Stars() {
			// if s.Magnitude > 9.0 {
			// 	continue
			// }

			// pos := rl.GetWorldToScreen(s.CoordsV, cam)
			// rl.DrawTextureEx(texture.NormalStar(), pos, 0.0, s.Size, s.Color)

			// rl.DrawBillboardPro()

			// rl.DrawBillboard(cam, texture.BrightStar(), s.CoordsV, s.Size, s.Color)
			rl.DrawBillboard(cam, texture.NormalStar(), s.CoordsV, s.Size, s.Color)
			// rl.DrawBillboard(cam, texture.Circle(), s.CoordsV, s.Size, s.Color)
		}

		rl.EndMode3D()

		// for _, s := range json.Stars() {
		// 	if s.ProperName != "" {
		// 		tp := rl.GetWorldToScreen(s.CoordsV, cam)
		// 		rl.DrawText(s.ProperName, int32(tp.X)+10, int32(tp.Y), 10, rl.DarkGray)
		// 	}
		// }

		rl.EndShaderMode()
		// rl.EndBlendMode()

		rl.DrawFPS(20, 20)
		rl.EndDrawing()
	}
}
