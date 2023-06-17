package colony

import (
	"math/rand"
	"time"
)

var (
	neighbourhood = [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}
)

const (
	Alive = '#'
	Dead  = 0
)

// Colony is the main game.
type Colony struct {
	width     int
	height    int
	substrate [][]rune
	output    [][]rune
}

// New contructs a new game.
func New(width int, height int) *Colony {
	g := &Colony{
		width:     width,
		height:    height,
		substrate: make([][]rune, width),
		output:    make([][]rune, width),
	}

	for i := 0; i < width; i++ {
		g.substrate[i] = make([]rune, height)
		g.output[i] = make([]rune, height)
	}

	g.Seed()

	return g
}

// Width returns the width of the colony.
func (g *Colony) Width() int {
	return g.width
}

// Height returns the height of the colony.
func (g *Colony) Height() int {
	return g.height
}

// Incubate creates the next generation.
func (g *Colony) Incubate() {
	for y := 0; y < g.height; y++ {
		for x := 0; x < g.width; x++ {
			neighbours := 0

			// Check the neighbourhood for alive cells.
			for _, pos := range neighbourhood {
				x2 := x + pos[0]
				y2 := y + pos[1]

				if x2 < 0 {
					x2 = g.width - 1
				}

				if x2 >= g.width {
					x2 = 0
				}

				if y2 < 0 {
					y2 = g.height - 1
				}

				if y2 >= g.height {
					y2 = 0
				}

				if g.output[x2][y2] == Alive {
					neighbours++
				}
			}

			// The three standard rules of survival of Conway's game of life.
			// https://en.wikipedia.org/wiki/Conway%27s_Game_of_Life
			if neighbours == 3 {
				g.substrate[x][y] = Alive
			} else if g.output[x][y] == Alive && neighbours == 2 {
				g.substrate[x][y] = Alive
			} else {
				g.substrate[x][y] = Dead
			}

			// This is not a standard rule but we need it to prevent the colony
			// from dying because this is a screensaver and we have to keep
			// things moving on-screen.
			if neighbours == 1 && rand.Intn(1000) == 500 {
				g.substrate[x][y] = Alive
			}
		}
	}

	// Swap the next generation to the output buffer.
	g.output, g.substrate = g.substrate, g.output
}

// View returns the current game view.
func (g *Colony) View() [][]rune {
	return g.output
}

// Seed randomises the game cells.
func (g *Colony) Seed() {
	rand.New(rand.NewSource(time.Now().Unix()))

	for i := 0; i < (g.width * g.height / 4); i++ {
		g.output[rand.Intn(g.width)][rand.Intn(g.height)] = Alive
	}
}
