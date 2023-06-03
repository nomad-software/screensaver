package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver"
	"github.com/nomad-software/screensaver/screen/saver/game_of_life/assets"
	"github.com/nomad-software/screensaver/screen/saver/game_of_life/colony"
)

var (
	opacityDropOff float32 = 0.2
	opacity        [][]float32
)

func main() {
	width, height := saver.CreateWindow("screensaver - game of life")
	defer saver.CloseWindow()

	rl.SetTargetFPS(15)

	cell := assets.NewCell()
	substrate := colony.New(width/cell.TextureWidth(), height/cell.TextureHeight())

	// Initialise the opacity slice.
	opacity = make([][]float32, substrate.Width())
	for x := 0; x < substrate.Width(); x++ {
		opacity[x] = make([]float32, substrate.Height())
	}

	for {
		if saver.InputDetected() {
			break
		}

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)

		view := substrate.View()

		for y := 0; y < substrate.Height(); y++ {
			for x := 0; x < substrate.Width(); x++ {
				if view[x][y] == colony.Alive {
					opacity[x][y] = 1.0
				} else {
					if opacity[x][y] > 0 {
						opacity[x][y] -= opacityDropOff
					}
				}
				rl.DrawTexture(cell.Texture(), int32(x*cell.TextureWidth()), int32(y*cell.TextureHeight()), rl.ColorAlpha(rl.White, opacity[x][y]))
			}
		}

		rl.EndDrawing()

		substrate.Incubate()
	}
}
