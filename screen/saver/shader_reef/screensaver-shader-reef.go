package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver"
	"github.com/nomad-software/screensaver/screen/saver/shader_reef/assets"
)

func main() {
	width, height := saver.CreateWindow("screensaver - shader reef")
	defer saver.CloseWindow()

	rl.SetTargetFPS(60)

	store := assets.NewShaderCollection()
	shader := store.Shader(width, height)

	timeLoc := rl.GetShaderLocation(shader, "u_time")

	for {
		if saver.InputDetected() {
			break
		}

		time := []float32{float32(rl.GetTime())}
		rl.SetShaderValue(shader, timeLoc, time, rl.ShaderUniformFloat)

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)

		rl.BeginShaderMode(shader)
		rl.DrawRectangle(0, 0, int32(width), int32(height), rl.White)
		rl.EndShaderMode()

		rl.EndDrawing()
	}
}
