package main

import (
	"embed"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/nomad-software/screensaver/assets"
	"github.com/nomad-software/screensaver/screen/saver"
	"github.com/nomad-software/screensaver/screen/saver/game_of_life/colony"
)

var (
	//go:embed assets/*
	fs embed.FS

	store      = assets.New(fs)
	cell       = store.LoadImage("assets/cell.png")
	cellWidth  = cell.Bounds().Dx()
	cellHeight = cell.Bounds().Dy()

	backgroundColour = color.NRGBA{0, 0, 0, 255}
	opacityDropOff   = 0.05

	substrate *colony.Colony
	opacity   [][]float64

	tick uint
)

type GameOfLife struct {
	saver.Saver
}

func (g *GameOfLife) Update() error {
	if substrate == nil {
		substrate = colony.New(g.ScreenWidth/cellWidth, g.ScreenHeight/cellHeight)
	}

	if opacity == nil {
		opacity = make([][]float64, substrate.Width())
		for x := 0; x < substrate.Width(); x++ {
			opacity[x] = make([]float64, substrate.Height())
			for y := 0; y < substrate.Height(); y++ {
				opacity[x][y] = 0.0
			}
		}
	}

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
		}
	}

	tick++

	if tick%4 == 0 {
		substrate.Incubate()
	}

	return g.Saver.Update()
}

func (g *GameOfLife) Draw(screen *ebiten.Image) {
	buffer := g.Blit(screen)
	buffer.Fill(backgroundColour)

	for y := 0; y < substrate.Height(); y++ {
		for x := 0; x < substrate.Width(); x++ {
			if opacity[x][y] > 0 {
				opt := &ebiten.DrawImageOptions{}
				opt.GeoM.Translate(float64(x*cellWidth), float64(y*cellHeight))
				opt.ColorM.Scale(1.0, 1.0, 1.0, opacity[x][y])
				buffer.DrawImage(cell, opt)
			}
		}
	}
}

func main() {
	ebiten.SetFullscreen(true)
	ebiten.SetCursorMode(ebiten.CursorModeHidden)

	saver.Run(&GameOfLife{})
}
