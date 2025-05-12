package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver"
	"github.com/nomad-software/screensaver/screen/saver/fish_pond/fish"
)

func main() {
	width, height := saver.CreateWindow("screensaver - fish pond")
	defer saver.CloseWindow()

	rl.SetTargetFPS(60)

	p := make([]*fish.Fish, 0)

	for range 20 {
		p = append(p, fish.NewFish(width, height))
	}

	for {
		// if saver.InputDetected() {
		// 	break
		// }
		if rl.GetKeyPressed() > 0 {
			break
		}

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)

		for _, fish := range p {
			fish.Update()
		}

		rl.EndDrawing()
	}
}
