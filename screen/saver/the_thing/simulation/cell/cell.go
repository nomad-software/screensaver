package cell

import (
	"image/color"
	"math"
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver/the_thing/display"
)

// Kind is the type of cell.
type Kind int

const (
	Healthy Kind = iota
	Intruder
	Imitation
)

// Mode is how a cell looks as it is assimilated.
type Mode int

const (
	Seeping        Mode = iota // Red seeps into the cell.
	Hollowing                  // The cell hollows out for an intruder to show through.
	Disintegrating             // The cell disintegrates from the centre.
)

// HollowEnd is the assimilation progress when a hollowing cell's wall has gone
// and the intruder has merged into it.
const HollowEnd = 0.25

// harmonic is one wave that makes up the wobbling outline of a cell.
type harmonic struct {
	freq  float64
	amp   float64
	phase float64
	speed float64
}

// Cell is a single organism in the simulation.
type Cell struct {
	Pos    rl.Vector2
	Vel    rl.Vector2
	Radius float64
	Kind   Kind

	// Infection is the progress of assimilation from 0 to 1.
	Infection float64

	// Mode is how the cell looks while it is assimilated.
	Mode Mode

	// From is the direction the infection came from.
	From float64

	// Morph blends the intruder's shape and colour from its own at 0 into
	// those of the infected MorphInto cell at 1, so that it can be swapped for
	// that cell without a jump.
	Morph     float64
	MorphInto *Cell

	// Absorbed is true once the intruder has completely merged into a cell.
	Absorbed bool

	harmonics []harmonic
	arms      [3]float64 // Length of each of the intruder's arms.
	seed      int
}

// New creates a new cell.
func New(pos rl.Vector2, kind Kind) *Cell {
	c := &Cell{
		Pos:    pos,
		Radius: 10 + rand.Float64()*3,
		Kind:   kind,
		seed:   rand.Intn(1 << 16),
	}

	// Lots of waves, the higher ones small and quick, keep the outline
	// lumpy and unstable.
	for freq := 2.0; freq <= 9; freq++ {
		c.harmonics = append(c.harmonics, harmonic{
			freq:  freq,
			amp:   (0.03 + rand.Float64()*0.07) * 3 / freq,
			phase: rand.Float64() * math.Pi * 2,
			speed: (rand.Float64() - 0.5) * (1 + freq*0.3),
		})
	}

	if kind == Intruder {
		c.Radius = 14 + rand.Float64()*2
		c.arms = [3]float64{1.17, 1.0 + rand.Float64()*0.1, 0.95 + rand.Float64()*0.1}
	}

	return c
}

// blob returns the radius of a cell's wobbling outline at an angle.
func (c *Cell) blob(theta, t, radius float64) float64 {
	r := 1.0
	for _, h := range c.harmonics {
		r += h.amp * math.Sin(h.freq*theta+h.phase+h.speed*t)
	}
	return radius * r
}

// edge returns the radius of the cell's outline at an angle.
func (c *Cell) edge(theta, t float64) float64 {
	if c.Kind == Intruder {
		// A caltrop: three sharp arms with concave sides between them,
		// gently rocking from side to side.
		rot := math.Pi/2 + 0.3*math.Sin(t*0.4)
		phi := theta - rot
		arm := int(math.Round(phi/(math.Pi*2/3))) % 3
		if arm < 0 {
			arm += 3
		}
		// Falls away linearly from each tip, which keeps the points sharp
		// and the sides between them concave.
		tip := math.Abs(math.Mod(math.Abs(phi)*3/(math.Pi*2), 1)*2 - 1)
		r := c.Radius * c.arms[arm] * (0.31 + 0.69*math.Pow(tip, 2.8))
		if c.Morph > 0 && c.MorphInto != nil {
			r += (c.MorphInto.blob(theta, t, c.MorphInto.Radius) - r) * c.Morph
		}
		return r
	}

	return c.blob(theta, t, c.Radius)
}

// noise returns a stable pseudo random value between 0 and 1 for a pixel.
func (c *Cell) noise(x, y int) float64 {
	h := uint32(x*374761393+y*668265263+c.seed*2246822519) ^ 0x5bd1e995
	h = (h ^ (h >> 13)) * 1274126177
	return float64(h^(h>>16)) / float64(math.MaxUint32)
}

// patch returns smooth noise between 0 and 1 that changes over the passed
// amount of pixels, so that changes happen in blobs rather than single pixels.
func (c *Cell) patch(x, y, size float64) float64 {
	gx, gy := math.Floor(x/size), math.Floor(y/size)
	fx, fy := x/size-gx, y/size-gy
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)

	ix, iy := int(gx), int(gy)
	top := c.noise(ix, iy) + (c.noise(ix+1, iy)-c.noise(ix, iy))*fx
	bottom := c.noise(ix, iy+1) + (c.noise(ix+1, iy+1)-c.noise(ix, iy+1))*fx
	return top + (bottom-top)*fy
}

// holed returns true if a pixel is in one of the small holes that keep
// opening and closing in the cell, most often near its edge.
func (c *Cell) holed(x, y int, q, t float64) bool {
	// Each area of the cell changes at its own moment so the holes don't all
	// flicker together.
	slot := float64(int(t*2.5 + c.noise(x>>3, y>>3)*2))
	n := c.patch(float64(x)+slot*53, float64(y)-slot*29, 4)
	return n < 0.02+0.15*q*q*q
}

// lerp blends two colours.
func lerp(a, b color.RGBA, f float64) color.RGBA {
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x) + (float64(y)-float64(x))*f)
	}
	return color.RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 0xFF}
}

// colour returns the colour of a pixel relative to the cell. The bool is false
// if nothing is drawn.
func (c *Cell) colour(dx, dy, q float64) (color.RGBA, bool) {
	if c.Kind == Intruder {
		// The odd pixel is missing from the rough outline.
		if q > 0.8 && c.noise(int(dx), int(dy)) < 0.25 {
			return color.RGBA{}, false
		}
		col := lerp(display.IntruderCore, display.Intruder, math.Min(1, q*1.5))
		return lerp(col, display.Infection, c.Morph), true
	}

	// The middle of the cell glows slightly lighter.
	body := lerp(display.CellCore, display.Cell, math.Min(1, math.Max(0, (q-0.1)/0.6)))

	x, y := int(dx), int(dy)

	p := c.Infection
	switch {
	case p <= 0 || p >= 1:
		return body, true

	case c.Mode == Hollowing && p >= HollowEnd:
		// The intruder has merged and pixels of the imitation start
		// replacing it straight away, so it's given a head start.
		if c.dissolved(dx, dy, q, 0.15+(p-HollowEnd)/(1-HollowEnd)) {
			return body, true
		}
		return display.Infection, true

	case p < 0.35:
		// The cell wall holds while the inside changes, though for a
		// merging intruder it breaks up before the intruder is absorbed.
		if q > 0.72 {
			if c.Mode != Hollowing || p < 0.12 || c.noise(x, y) > (p-0.12)/(HollowEnd-0.12) {
				return display.Cell, true
			}
		}
		switch c.Mode {
		case Hollowing:
			// The inside hollows out for the merging intruder to show
			// through.
			if c.patch(dx, dy, 3) > p/0.12 {
				return body, true
			}
		case Disintegrating:
			// The inside falls away from the centre outwards.
			if q-c.patch(dx, dy, 3)*0.25 > p/0.35*0.75 {
				return body, true
			}
		default:
			// Red seeps in.
			if c.patch(dx, dy, 3) < p/0.35 {
				return display.Infection, true
			}
			return body, true
		}
		return color.RGBA{}, false

	case p < 0.45:
		// The cell is wholly the intruder and its old wall fades away.
		if q > 0.72 && c.noise(x, y) > (p-0.35)/0.1 {
			return display.Cell, true
		}
		return display.Infection, true

	default:
		// Pixels of the imitation slowly replace the intruder's.
		if c.dissolved(dx, dy, q, (p-0.45)/0.55) {
			return body, true
		}
		return display.Infection, true
	}
}

// dissolved returns true if a pixel has been replaced by the imitation, given
// the progress of the replacement from 0 to 1. It spreads in blobs from the
// far side of where the infection came from.
func (c *Cell) dissolved(dx, dy, q, f float64) bool {
	far := (math.Cos(math.Atan2(dy, dx)-c.From-math.Pi)*q + 1) / 2
	return c.patch(dx, dy, 3)*0.6+(1-far)*0.4 < f
}

// Draw draws the cell a pixel at a time.
func (c *Cell) Draw(t float64) {
	if c.Absorbed {
		return
	}

	bound := int(c.Radius*1.5) + 2
	if c.MorphInto != nil {
		bound = max(bound, int(c.MorphInto.Radius*1.5)+2)
	}
	cx := int(math.Round(float64(c.Pos.X)))
	cy := int(math.Round(float64(c.Pos.Y)))

	// The edge pixels flicker in and out as the outline is redrawn.
	frame := int(t * 8)

	for y := -bound; y <= bound; y++ {
		for x := -bound; x <= bound; x++ {
			dx := float64(x)
			dy := float64(y)
			d := math.Hypot(dx, dy)
			r := c.edge(math.Atan2(dy, dx), t)
			r += (c.noise(x+frame*31, y-frame*17) - 0.5) * 0.9

			if d > r || c.holed(x, y, d/r, t) {
				continue
			}

			col, ok := c.colour(dx, dy, d/r)
			if !ok {
				continue
			}

			rl.DrawPixel(int32(cx+x), int32(cy+y), col)
		}
	}
}
