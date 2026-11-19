package hive

import (
	"math"
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver/hex_cascade/hive/cell"
	"github.com/nomad-software/screensaver/screen/saver/hex_cascade/hive/palette"
	"github.com/nomad-software/screensaver/screen/saver/hex_cascade/hive/wave"
)

const (
	cellsHigh    = 18    // About how many cells fit the height of the screen.
	firstWave    = 0.4   // When the first wave begins, in seconds.
	minWaveGap   = 4.2   // The least time between waves, in seconds.
	maxWaveGap   = 6.0   // The most time between waves, in seconds.
	shuffleRate  = 0.006 // The share of cells that shuffle each second.
	shuffleTime  = 0.55  // How long a shuffle takes, in seconds.
	maxFrameTime = 0.1   // The longest step taken at once, in seconds.
)

var (
	// The colour of every cell before the first wave paints it.
	unlit = rl.NewColor(14, 14, 20, 255)

	// The steps to each of the six neighbours of a cell, in odd q offset
	// coordinates, for even and odd columns.
	neighbours = [2][6][2]int{
		{{1, -1}, {1, 0}, {0, 1}, {-1, 0}, {-1, -1}, {0, -1}},
		{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {0, -1}},
	}
)

// Hive is a screen covered in hexagonal cells, which waves of change sweep
// across, each painting a new pattern.
type Hive struct {
	width   float32 // The width of the screen, in pixels.
	height  float32 // The height of the screen, in pixels.
	radius  float32 // The distance from the centre of a cell to a corner.
	columns int     // The number of columns of cells.
	rows    int     // The number of rows of cells.

	cells    []*cell.Cell   // Every cell, column by column.
	palettes *palette.Cycle // The palettes waves paint with.
	time     float64        // The time since the hive began, in seconds.
	nextWave float64        // The time the next wave begins.
	waves    int            // The number of waves so far.
	shuffles float64        // Shuffles owed but not yet made.
}

// New creates a new hive to cover a screen of the given size.
func New(width, height int) *Hive {
	h := &Hive{
		width:    float32(width),
		height:   float32(height),
		radius:   float32(height) / cellsHigh / float32(math.Sqrt(3)),
		palettes: palette.NewCycle(),
		nextWave: firstWave,
	}

	// Flat topped hexagons sit in columns, one and a half radii apart, with
	// every other column dropped by half a cell. There is a spare one at each
	// edge, so the screen is covered right to its edges.
	cellHeight := h.radius * float32(math.Sqrt(3))
	h.columns = int(h.width/(h.radius*1.5)) + 2
	h.rows = int(h.height/cellHeight) + 2

	for column := range h.columns {
		for row := range h.rows {
			centre := rl.NewVector2(
				float32(column)*h.radius*1.5,
				float32(row)*cellHeight+float32(column&1)*cellHeight/2,
			)
			h.cells = append(h.cells, cell.New(column, row, centre, unlit))
		}
	}

	return h
}

// Radius returns the distance from the centre of a cell to a corner.
func (h *Hive) Radius() float32 {
	return h.radius
}

// Time returns the time since the hive began, in seconds.
func (h *Hive) Time() float64 {
	return h.time
}

// Cells returns every cell.
func (h *Hive) Cells() []*cell.Cell {
	return h.cells
}

// Iterate moves the hive along by the given number of seconds.
func (h *Hive) Iterate(dt float32) {
	dt = min(dt, maxFrameTime)
	h.time += float64(dt)

	if h.time >= h.nextWave {
		h.startWave()
		h.nextWave = h.time + minWaveGap + rand.Float64()*(maxWaveGap-minWaveGap)
	}

	h.shuffle(dt)

	for _, c := range h.cells {
		c.Iterate(h.time, dt)
	}
}

// startWave sends a new wave across the hive.
func (h *Hive) startWave() {
	// The first wave is always rain, falling onto a dark screen.
	sweep := wave.Rain
	if h.waves > 0 {
		sweep = wave.RandomSweep()
	}

	w := wave.New(sweep, h.cells, h.columns, h.width, h.height, h.radius, h.palettes.Next())
	for _, c := range h.cells {
		c.Queue(w.Plan(c, h.time))
	}

	h.waves++
}

// shuffle makes a few idle cells swap faces with a neighbour between waves,
// each spinning into the other's place.
func (h *Hive) shuffle(dt float32) {
	if h.waves == 0 {
		return
	}

	h.shuffles += float64(len(h.cells)) * shuffleRate * float64(dt)

	for ; h.shuffles >= 1; h.shuffles-- {
		a := h.cells[rand.Intn(len(h.cells))]
		b := h.neighbour(a)

		if b == nil || !a.IsIdle() || !b.IsIdle() || a.Colour() == b.Colour() {
			continue
		}

		turns := []int{-1, 1}[rand.Intn(2)]
		a.Queue(h.swap(b, turns))
		b.Queue(h.swap(a, -turns))
	}
}

// swap returns a transition that spins a cell into the face of another.
func (h *Hive) swap(other *cell.Cell, turns int) cell.Transition {
	return cell.Transition{
		Start:    h.time,
		Duration: shuffleTime,
		Motion:   cell.Spin,
		Colour:   other.Colour(),
		Relief:   other.Relief(),
		Turns:    turns,
	}
}

// neighbour returns a random neighbour of the given cell, or nil if it is on
// the edge of the hive and the one chosen is off it.
func (h *Hive) neighbour(c *cell.Cell) *cell.Cell {
	step := neighbours[c.Column()&1][rand.Intn(6)]
	column := c.Column() + step[0]
	row := c.Row() + step[1]

	if column < 0 || column >= h.columns || row < 0 || row >= h.rows {
		return nil
	}

	return h.cells[column*h.rows+row]
}
