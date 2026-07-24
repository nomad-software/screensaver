package main

import (
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver"
	"github.com/nomad-software/screensaver/screen/saver/connection_machine/panel"
	"github.com/nomad-software/screensaver/screen/saver/connection_machine/panel/block"
)

func main() {
	width, height := saver.CreateWindow("screensaver - connection machine")
	defer saver.CloseWindow()

	rl.SetTargetFPS(10)

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

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)

		gap := ((width - (panels[0].PixelWidth() * len(panels))) / (len(panels) + 1))
		origin := gap

		for _, panel := range panels {
			for x := 0; x < panel.Width(); x++ {
				for y := 0; y < panel.Height(); y++ {

					if !panel.BlockOn(x, y) {
						continue
					}

					pos := rl.NewVector2(
						float32(origin+(x*block.Size())),
						float32(y*block.Size()),
					)

					rl.DrawRectangleV(pos, block.SizeV(), color.RGBA{0xC8, 0x0, 0x0, 0xFF})
				}
			}
			panel.Iterate()

			origin += panel.PixelWidth() + gap
		}

		rl.EndDrawing()
	}
}
