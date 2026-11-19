package cell

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Motion is the way a cell moves from one face to the next.
type Motion int

const (
	// Flip tips the cell over like a card, revealing the new face on its back.
	Flip Motion = iota

	// Spin turns the cell in place by sixths of a turn, blending into the new
	// face as it turns.
	Spin
)

const (
	maxQueued   = 3    // The most transitions a cell keeps waiting.
	flashDecay  = 4.5  // How fast the flash of a landed cell fades, per second.
	shimmerRate = 1.3  // How fast the faint ripple of light across the hive moves.
	shimmerSize = 0.07 // How much the faint ripple of light changes brightness.
)

// Transition is a change of a cell from one face to another.
type Transition struct {
	Start    float64    // The hive time it begins at.
	Duration float64    // How long it takes, in seconds.
	Motion   Motion     // The way the cell moves.
	Colour   rl.Color   // The colour of the new face.
	Relief   float32    // How much the new face looks like a cube, from 0 to 1.
	Turns    int        // The sixths of a turn a spin makes, signed.
	Axis     rl.Vector2 // The unit direction a flip tips over towards.
}

// View is everything needed to draw a cell at one moment.
type View struct {
	Centre     rl.Vector2 // The centre, in pixels.
	Colour     rl.Color   // The colour of the face shown.
	Relief     float32    // How much the face looks like a cube, from 0 to 1.
	Rotation   float32    // The turn of the cell, in radians.
	Size       float32    // The scale of the cell.
	Squash     float32    // The scale of the cell along its axis, from 0 to 1.
	Axis       rl.Vector2 // The unit direction it is squashed along.
	Brightness float32    // The light falling on it.
	Flash      float32    // The white flash of a cell that has just landed.
}

// Cell is a single hexagon of the hive.
type Cell struct {
	centre rl.Vector2 // The centre, in pixels.

	column int // The column of the hive it is in.
	row    int // The row of the hive it is in.
	q      int // The axial q coordinate.
	r      int // The axial r coordinate.

	colour   rl.Color // The colour of the face shown when settled.
	relief   float32  // How much the face looks like a cube when settled.
	rotation float32  // The turn of the cell when settled, in radians.
	flash    float32  // The white flash of a cell that has just landed.

	moving  bool         // Whether the active transition is playing.
	active  Transition   // The transition playing.
	pending []Transition // Transitions waiting to play, in order of start.
}

// New creates a new cell for the given column and row, with its centre at
// the given position.
func New(column, row int, centre rl.Vector2, colour rl.Color) *Cell {
	return &Cell{
		centre: centre,
		column: column,
		row:    row,
		q:      column,
		r:      row - (column-(column&1))/2,
		colour: colour,
	}
}

// Centre returns the centre of the cell, in pixels.
func (c *Cell) Centre() rl.Vector2 {
	return c.centre
}

// Column returns the column of the hive the cell is in.
func (c *Cell) Column() int {
	return c.column
}

// Row returns the row of the hive the cell is in.
func (c *Cell) Row() int {
	return c.row
}

// Q returns the axial q coordinate of the cell.
func (c *Cell) Q() int {
	return c.q
}

// R returns the axial r coordinate of the cell.
func (c *Cell) R() int {
	return c.r
}

// Colour returns the colour the cell is, or is about to be.
func (c *Cell) Colour() rl.Color {
	if n := len(c.pending); n > 0 {
		return c.pending[n-1].Colour
	}
	if c.moving {
		return c.active.Colour
	}
	return c.colour
}

// Relief returns the relief the cell has, or is about to have.
func (c *Cell) Relief() float32 {
	if n := len(c.pending); n > 0 {
		return c.pending[n-1].Relief
	}
	if c.moving {
		return c.active.Relief
	}
	return c.relief
}

// Distance returns the number of steps between this cell and another.
func (c *Cell) Distance(other *Cell) int {
	dq := c.q - other.q
	dr := c.r - other.r
	return (abs(dq) + abs(dr) + abs(dq+dr)) / 2
}

// IsMoving returns true while the cell is part way through a transition.
func (c *Cell) IsMoving() bool {
	return c.moving
}

// IsIdle returns true if the cell is neither moving nor waiting to.
func (c *Cell) IsIdle() bool {
	return !c.moving && len(c.pending) == 0
}

// Queue adds a transition for the cell to play when its time comes.
func (c *Cell) Queue(t Transition) {
	i := len(c.pending)
	for i > 0 && c.pending[i-1].Start > t.Start {
		i--
	}
	c.pending = append(c.pending, Transition{})
	copy(c.pending[i+1:], c.pending[i:])
	c.pending[i] = t

	// Too many waiting means the cell has fallen behind, so the oldest are
	// skipped rather than played late.
	if len(c.pending) > maxQueued {
		c.pending = c.pending[len(c.pending)-maxQueued:]
	}
}

// Iterate moves the cell along to the given hive time.
func (c *Cell) Iterate(now float64, dt float32) {
	c.flash *= float32(math.Exp(float64(-flashDecay * dt)))

	if c.moving && now >= c.active.Start+c.active.Duration {
		c.colour = c.active.Colour
		c.relief = c.active.Relief
		c.rotation = wrap(c.rotation + float32(c.active.Turns)*math.Pi/3)
		c.flash = 1.0
		c.moving = false
	}

	if !c.moving && len(c.pending) > 0 && now >= c.pending[0].Start {
		c.active = c.pending[0]
		c.pending = c.pending[1:]
		// One that had to wait for the last to finish starts now, so that it
		// plays in full.
		c.active.Start = now
		c.moving = true
	}
}

// View returns how the cell looks at the given hive time.
func (c *Cell) View(now float64) View {
	v := View{
		Centre:     c.centre,
		Colour:     c.colour,
		Relief:     c.relief,
		Rotation:   c.rotation,
		Size:       1.0,
		Squash:     1.0,
		Axis:       rl.NewVector2(0, 1),
		Brightness: 1.0,
		Flash:      c.flash,
	}

	if c.moving {
		t := c.active
		linear := clamp(float32((now-t.Start)/t.Duration), 0, 1)

		switch t.Motion {
		case Flip:
			angle := easeInOutCubic(linear) * math.Pi
			cos := float32(math.Cos(float64(angle)))
			sin := float32(math.Sin(float64(angle)))

			// The back is the new face, seen once the cell is past edge on.
			if cos < 0 {
				v.Colour = t.Colour
				v.Relief = t.Relief
			}
			v.Squash = abs32(cos)
			v.Axis = t.Axis
			v.Size = 1.0 + 0.06*sin
			v.Brightness = 0.4 + 0.6*abs32(cos)

		case Spin:
			p := easeInOutBack(linear)
			blend := smoothstep(0.2, 0.8, linear)

			v.Rotation = c.rotation + float32(t.Turns)*math.Pi/3*p
			v.Colour = lerpColour(c.colour, t.Colour, blend)
			v.Relief = c.relief + (t.Relief-c.relief)*blend
			// It shrinks as it turns, so that its corners stay clear of its
			// neighbours.
			v.Size = 1.0 - 0.2*float32(math.Sin(float64(linear*math.Pi)))
		}
	}

	// A faint ripple of light rolls across the whole hive, so that it is never
	// quite still.
	shimmer := math.Sin(now*shimmerRate + float64(c.centre.X)*0.004 + float64(c.centre.Y)*0.006)
	v.Brightness *= 1.0 - shimmerSize + shimmerSize*float32(shimmer)

	return v
}

// lerpColour blends from one colour to another.
func lerpColour(a, b rl.Color, t float32) rl.Color {
	mix := func(x, y uint8) uint8 {
		return uint8(float32(x) + (float32(y)-float32(x))*t)
	}
	return rl.NewColor(mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 255)
}

func easeInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	f := -2*t + 2
	return 1 - f*f*f/2
}

// easeInOutBack overshoots a little at each end, so that a spin snaps into
// place.
func easeInOutBack(t float32) float32 {
	const c1 = 1.70158
	const c2 = c1 * 1.525
	if t < 0.5 {
		return (2 * t) * (2 * t) * ((c2+1)*2*t - c2) / 2
	}
	f := 2*t - 2
	return (f*f*((c2+1)*f+c2) + 2) / 2
}

func smoothstep(edge0, edge1, x float32) float32 {
	t := clamp((x-edge0)/(edge1-edge0), 0, 1)
	return t * t * (3 - 2*t)
}

func clamp(x, lo, hi float32) float32 {
	return float32(math.Max(float64(lo), math.Min(float64(hi), float64(x))))
}

// wrap keeps a rotation within a whole turn. A hexagon looks the same every
// sixth of a turn but its relief does not, so a whole turn is kept.
func wrap(r float32) float32 {
	return float32(math.Mod(float64(r)+2*math.Pi, 2*math.Pi))
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
