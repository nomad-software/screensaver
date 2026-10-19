package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver"
	"github.com/nomad-software/screensaver/screen/saver/the_thing/assets"
	"github.com/nomad-software/screensaver/screen/saver/the_thing/display"
	"github.com/nomad-software/screensaver/screen/saver/the_thing/report"
	"github.com/nomad-software/screensaver/screen/saver/the_thing/simulation"
	"github.com/nomad-software/screensaver/screen/saver/the_thing/sound"
)

// scene is one part of the looping sequence.
type scene interface {
	Update(dt float32)
	Draw()
	Done() bool
}

func main() {
	width, height := saver.CreateWindow("screensaver - the thing")
	defer saver.CloseWindow()

	sound.Init()
	defer sound.Close()

	rl.SetTargetFPS(30)

	// Everything is drawn at a low resolution then scaled up to look like an
	// old monitor.
	scale := display.Scale(width)
	virtualWidth := width / scale
	virtualHeight := height / scale

	shaders := assets.NewShaderCollection()
	crt := shaders.CRT(width, height, scale)

	buffer := rl.LoadRenderTexture(int32(virtualWidth), int32(virtualHeight))
	defer rl.UnloadRenderTexture(buffer)
	rl.SetTextureFilter(buffer.Texture, rl.FilterPoint)

	scenes := []func() scene{
		func() scene { return simulation.New(virtualWidth, virtualHeight) },
		func() scene { return report.Probability(virtualWidth, virtualHeight) },
		func() scene { return report.Projection(virtualWidth, virtualHeight) },
	}

	index := 0
	current := scenes[index]()

	for {
		if saver.InputDetected() {
			break
		}

		current.Update(rl.GetFrameTime())
		if current.Done() {
			index = (index + 1) % len(scenes)
			current = scenes[index]()
		}

		// Draw the entire scene onto the texure buffer.
		// This is to enable shader manipulation later.
		rl.BeginTextureMode(buffer)
		rl.ClearBackground(display.Background)
		current.Draw()
		rl.EndTextureMode()

		shaders.SetTime(float32(rl.GetTime()))

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)

		rl.BeginShaderMode(crt)

		// Draw the texture buffer to the screen.
		// The negative texture height is important because raylib's render
		// textures are vertically flipped.
		rl.DrawTexturePro(
			buffer.Texture,
			rl.NewRectangle(0, 0, float32(buffer.Texture.Width), -float32(buffer.Texture.Height)),
			rl.NewRectangle(0, 0, float32(virtualWidth*scale), float32(virtualHeight*scale)),
			rl.NewVector2(0, 0),
			0,
			rl.White,
		)

		rl.EndShaderMode()

		rl.EndDrawing()
	}
}
