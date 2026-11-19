package wave

import (
	"math"
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver/hex_cascade/hive/cell"
	"github.com/nomad-software/screensaver/screen/saver/hex_cascade/hive/palette"
)

const (
	sweepTime    = 3.6  // How long a wave takes to cross the screen, in seconds.
	jitterTime   = 0.12 // The most a cell is randomly late, in seconds.
	flipDuration = 0.75 // How long a flip takes, in seconds.
	spinDuration = 0.65 // How long a spin takes, in seconds.
)

// Sweep is the way a wave travels across the screen.
type Sweep int

const (
	Rain     Sweep = iota // Falls down the screen, each column at its own pace.
	Ripple                // Spreads out in a circle from a point.
	Hexagon               // Spreads out in a hexagon from a cell.
	Wipe                  // Crosses the screen in a straight line.
	Spiral                // Winds out from a point in spiral arms.
	Dissolve              // Lands on cells at random.
	sweeps
)

// Pattern is the picture a wave paints.
type Pattern int

const (
	Solid    Pattern = iota // One colour.
	Triad                   // Three colours, no two neighbours alike.
	Blocks                  // Tumbling blocks, every cell a cube.
	Rings                   // Circular bands around a point.
	HexRings                // Hexagonal bands around a cell.
	Stripes                 // Straight bands.
	Arms                    // Spiral arms around a point.
	Gradient                // A smooth blend across the screen.
	patterns
)

// Wave is a single change of the whole hive, travelling across it and
// painting a new pattern as it goes.
type Wave struct {
	sweep   Sweep
	pattern Pattern
	motion  cell.Motion
	palette palette.Palette

	width  float32 // The width of the screen, in pixels.
	height float32 // The height of the screen, in pixels.

	origin     *cell.Cell // The cell a wave spreads from.
	focus      *cell.Cell // The cell a pattern is centred on.
	direction  rl.Vector2 // The unit direction a wipe or stripes run along.
	maxReach   float32    // The furthest any cell is from the origin.
	maxSteps   int        // The most steps any cell is from the origin.
	columnLag  []float32  // How late each column starts, for rain.
	columnPace []float32  // How fast each column falls, for rain.

	turns     int     // The sixths of a turn each cell spins.
	relief    float32 // How much each cell looks like a cube.
	colours   []int   // The palette colours used, in the order they appear.
	bandWidth float32 // The width of a band, in pixels.
	bandSteps int     // The width of a hexagonal band, in cells.
	arms      float32 // The number of spiral arms.
	mixed     bool    // Whether tumbling blocks are in three colours.
}

// New creates a new wave of the given sweep across the given cells, painting
// a random pattern from the palette.
func New(sweep Sweep, cells []*cell.Cell, columns int, width, height, radius float32, p palette.Palette) *Wave {
	w := &Wave{
		sweep:   sweep,
		pattern: Pattern(rand.Intn(int(patterns))),
		palette: p,
		width:   width,
		height:  height,
		origin:  cells[rand.Intn(len(cells))],
		turns:   []int{-2, -1, 1, 2}[rand.Intn(4)],
	}

	if rand.Intn(100) < 55 {
		w.motion = cell.Flip
	} else {
		w.motion = cell.Spin
	}

	// Patterns are centred on the origin of the wave as often as not, so that
	// the two line up.
	if rand.Intn(2) == 0 {
		w.focus = w.origin
	} else {
		w.focus = cells[rand.Intn(len(cells))]
	}

	// Directions are kept to the six of the hexagons' edges and the six of
	// their corners, so that wipes and stripes line up with the hive.
	angle := float64(rand.Intn(12)) * math.Pi / 6
	w.direction = rl.NewVector2(float32(math.Cos(angle)), float32(math.Sin(angle)))

	for _, c := range cells {
		w.maxReach = max(w.maxReach, rl.Vector2Distance(c.Centre(), w.origin.Centre()))
		w.maxSteps = max(w.maxSteps, c.Distance(w.origin))
	}

	w.columnLag = make([]float32, columns)
	w.columnPace = make([]float32, columns)
	for i := range columns {
		w.columnLag[i] = rand.Float32() * 0.9
		w.columnPace[i] = 0.75 + rand.Float32()*0.5
	}

	switch rand.Intn(20) {
	case 0, 1, 2, 3, 4, 5, 6:
		w.relief = 1.0
	case 7, 8, 9, 10, 11:
		w.relief = 0.35
	default:
		w.relief = 0.0
	}

	w.colours = rand.Perm(len(w.palette))
	w.bandWidth = radius * (2.0 + rand.Float32()*3.0)
	w.bandSteps = 1 + rand.Intn(3)
	w.arms = float32(2 + rand.Intn(4))
	w.mixed = rand.Intn(10) < 3

	switch w.pattern {
	case Blocks:
		w.relief = 1.0
	case Solid, Gradient:
		// The brightest colour comes first, so that a single colour is
		// never the darkest of the palette.
		w.colours[0] = rand.Intn(3)
	}

	return w
}

// RandomSweep returns any sweep at random.
func RandomSweep() Sweep {
	return Sweep(rand.Intn(int(sweeps)))
}

// Plan returns the transition this wave makes to the given cell, given the
// hive time the wave began.
func (w *Wave) Plan(c *cell.Cell, now float64) cell.Transition {
	colour, relief := w.paint(c)

	t := cell.Transition{
		Start:  now + float64(w.delay(c)) + rand.Float64()*jitterTime,
		Motion: w.motion,
		Colour: colour,
		Relief: relief,
		Turns:  w.turns,
		Axis:   w.axis(c),
	}

	if w.motion == cell.Flip {
		t.Duration = flipDuration
	} else {
		t.Duration = spinDuration
	}

	return t
}

// delay returns how long after the wave begins it reaches the given cell, in
// seconds.
func (w *Wave) delay(c *cell.Cell) float32 {
	pos := c.Centre()

	switch w.sweep {
	case Rain:
		fall := pos.Y / w.height * w.columnPace[c.Column()]
		return w.columnLag[c.Column()] + fall*(sweepTime-0.9)

	case Ripple:
		return rl.Vector2Distance(pos, w.origin.Centre()) / w.maxReach * sweepTime

	case Hexagon:
		return float32(c.Distance(w.origin)) / float32(w.maxSteps) * sweepTime

	case Wipe:
		return w.along(pos) * sweepTime

	case Spiral:
		turn, reach := w.polar(pos, w.origin)
		arm := fract(turn + reach/w.maxReach*1.5)
		return arm*sweepTime*0.7 + reach/w.maxReach*sweepTime*0.3

	default:
		return rand.Float32() * sweepTime
	}
}

// axis returns the direction the wave is travelling at the given cell, which
// is the way a flipping cell tips over.
func (w *Wave) axis(c *cell.Cell) rl.Vector2 {
	pos := c.Centre()

	switch w.sweep {
	case Rain:
		return rl.NewVector2(0, 1)

	case Ripple, Hexagon:
		return away(pos, w.origin.Centre())

	case Wipe:
		return w.direction

	case Spiral:
		out := away(pos, w.origin.Centre())
		return rl.NewVector2(-out.Y, out.X)

	default:
		angle := float64(rand.Intn(3)) * math.Pi / 3
		return rl.NewVector2(float32(math.Cos(angle)), float32(math.Sin(angle)))
	}
}

// paint returns the colour and relief this wave gives the given cell.
func (w *Wave) paint(c *cell.Cell) (rl.Color, float32) {
	pos := c.Centre()
	index := 0

	switch w.pattern {
	case Solid:
		return w.vary(w.palette[w.colours[0]]), w.relief

	case Triad:
		index = (((c.Q() - c.R()) % 3) + 3) % 3

	case Blocks:
		// Mostly a single colour, as the cubes are the pattern.
		if w.mixed {
			index = (((c.Q() - c.R()) % 3) + 3) % 3
		}

	case Rings:
		index = int(rl.Vector2Distance(pos, w.focus.Centre())/w.bandWidth) % 3

	case HexRings:
		index = (c.Distance(w.focus) / w.bandSteps) % 3

	case Stripes:
		index = int(w.along(pos)*w.width/w.bandWidth) % 3

	case Arms:
		turn, reach := w.polar(pos, w.focus)
		index = int(fract(turn*w.arms/2+reach/(w.bandWidth*4))*2) % 2

	case Gradient:
		t := w.along(pos) * 2
		lo := w.palette[w.colours[0]]
		mid := w.palette[w.colours[1]]
		hi := w.palette[w.colours[2]]
		if t < 1 {
			return w.vary(lerp(lo, mid, t)), w.relief
		}
		return w.vary(lerp(mid, hi, t-1)), w.relief
	}

	return w.vary(w.palette[w.colours[index]]), w.relief
}

// along returns how far along the direction of the wave the given position
// is, from 0 to 1 across the screen.
func (w *Wave) along(pos rl.Vector2) float32 {
	corners := [4]rl.Vector2{{X: 0, Y: 0}, {X: w.width, Y: 0}, {X: 0, Y: w.height}, {X: w.width, Y: w.height}}

	lo, hi := float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, corner := range corners {
		d := rl.Vector2DotProduct(corner, w.direction)
		lo = min(lo, d)
		hi = max(hi, d)
	}

	return (rl.Vector2DotProduct(pos, w.direction) - lo) / (hi - lo)
}

// polar returns the turn, from 0 to 1, and distance, in pixels, of the given
// position around the given cell.
func (w *Wave) polar(pos rl.Vector2, centre *cell.Cell) (float32, float32) {
	d := rl.Vector2Subtract(pos, centre.Centre())
	turn := float32(math.Atan2(float64(d.Y), float64(d.X))/(2*math.Pi)) + 0.5
	return turn, rl.Vector2Length(d)
}

// vary changes the brightness of a colour a little, so that a field of one
// colour has some life in it.
func (w *Wave) vary(c rl.Color) rl.Color {
	f := 0.9 + rand.Float32()*0.16
	scale := func(v uint8) uint8 {
		return uint8(min(255, float32(v)*f))
	}
	return rl.NewColor(scale(c.R), scale(c.G), scale(c.B), 255)
}

// away returns the unit direction from one position to another, or down if
// they are the same.
func away(pos, from rl.Vector2) rl.Vector2 {
	d := rl.Vector2Subtract(pos, from)
	if rl.Vector2Length(d) < 1 {
		return rl.NewVector2(0, 1)
	}
	return rl.Vector2Normalize(d)
}

func lerp(a, b rl.Color, t float32) rl.Color {
	mix := func(x, y uint8) uint8 {
		return uint8(float32(x) + (float32(y)-float32(x))*t)
	}
	return rl.NewColor(mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 255)
}

func fract(x float32) float32 {
	return x - float32(math.Floor(float64(x)))
}
