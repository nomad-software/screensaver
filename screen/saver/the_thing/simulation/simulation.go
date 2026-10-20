package simulation

import (
	"math"
	"math/rand"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver/the_thing/display"
	"github.com/nomad-software/screensaver/screen/saver/the_thing/display/typewriter"
	"github.com/nomad-software/screensaver/screen/saver/the_thing/simulation/cell"
)

const (
	healthyCells     = 5    // Amount of healthy cells at the start.
	margin           = 32   // Distance from the screen edge cells stay within.
	driftSpeed       = 3.0  // Maximum speed of a drifting cell.
	returnSpeed      = 5.0  // Maximum speed of a cell returning to the screen.
	huntSpeed        = 4.5  // Speed of a cell moving in on its target.
	huntAcceleration = 0.5  // How quickly a hunter reaches its full speed.
	mergeRate        = 0.5  // How quickly the intruder slides into its first victim.
	dragSpeed        = 3.0  // Speed of a cell dragged in by the spear.
	spearTime        = 0.8  // Seconds taken for the spear to reach its target.
	spearGap         = 5    // Gap between cells the spear can reach across.
	assimilationTime = 10.0 // Seconds taken to assimilate a cell.
	readTime         = 2.5  // Seconds a finished label stays on screen.
	labelGap         = 14   // Distance between a cell and its label.
	species          = "DOG"
)

type phase int

const (
	intro      phase = iota // The intruder arrives.
	identify                // The next target is labelled.
	approach                // The hunter closes in on the target.
	lunge                   // An imitation spears the target.
	drag                    // The spear drags the target in.
	assimilate              // The target is taken over.
	complete                // The target is now an imitation.
	finished                // The simulation is over.
)

// label is text typed beside a cell, underlined with a line from the cell.
// It stays still while the cell moves, so it's placed where the cell is
// expected to be once the label has been typed.
type label struct {
	text  *typewriter.Typewriter
	line  *typewriter.Typewriter // Typed after the text to draw the underline.
	x     int                    // Left edge of the text.
	top   int                    // Top of the text.
	left  int                    // Left end of the underline.
	right int                    // Right end of the underline.
	y     int                    // Height of the underline.
	read  float32                // Time since the label and underline finished typing.
}

// Simulation is the cell assimilation scene.
type Simulation struct {
	width  int
	height int

	cells  []*cell.Cell
	hunter *cell.Cell
	target *cell.Cell
	label  *label

	phase phase
	timer float32 // Time spent in the current phase.
	spear float32 // How far the spear has reached from 0 to 1.
	time  float64 // Total time, used to animate the cells.
	round int
	calm  bool // True if cells don't wander, used when predicting movement.
}

// New creates a new simulation.
func New(width, height int) *Simulation {
	s := &Simulation{
		width:  width,
		height: height,
	}

	for len(s.cells) < healthyCells {
		pos := rl.NewVector2(
			float32(margin+rand.Intn(width-margin*2)),
			float32(margin+rand.Intn(height-margin*2)),
		)
		if s.crowded(pos) {
			continue
		}
		c := cell.New(pos, cell.Healthy)
		c.Vel = rl.NewVector2(rand.Float32()*2-1, rand.Float32()*2-1)
		s.cells = append(s.cells, c)
	}

	// The intruder drifts in from the screen edge nearest one of the cells.
	near := s.cells[rand.Intn(len(s.cells))].Pos
	pos := rl.NewVector2(-20, near.Y)
	if near.X > float32(width)/2 {
		pos.X = float32(width + 20)
	}
	s.hunter = cell.New(pos, cell.Intruder)
	s.hunter.Vel = rl.Vector2Scale(rl.Vector2Normalize(rl.Vector2Subtract(near, pos)), returnSpeed)
	s.cells = append(s.cells, s.hunter)

	return s
}

// crowded returns true if a position is too close to an existing cell.
func (s *Simulation) crowded(pos rl.Vector2) bool {
	minimum := float32(min(s.width, s.height)) / 4
	for _, c := range s.cells {
		if rl.Vector2Distance(c.Pos, pos) < minimum {
			return true
		}
	}
	return false
}

// inside returns true if a cell is within the screen margin.
func (s *Simulation) inside(c *cell.Cell) bool {
	return c.Pos.X > margin && c.Pos.X < float32(s.width-margin) &&
		c.Pos.Y > margin && c.Pos.Y < float32(s.height-margin)
}

// setLabel types new text beside a cell.
func (s *Simulation) setLabel(c *cell.Cell, text string) {
	t := typewriter.New(text)

	width := 0
	for _, line := range t.AllLines() {
		width = max(width, display.MeasureText(line))
	}

	// The underline is typed as a row of characters, so it takes as many
	// keystrokes as characters would fit along it.
	advance := display.MeasureText("__") - display.MeasureText("_")
	space := int(c.Radius) + labelGap + width
	line := typewriter.New(strings.Repeat("_", max(1, (space+advance/2)/advance)))

	pos := s.predict(c, t.Duration()+line.Duration())
	fitsRight := int(pos.X)+space < s.width-4
	fitsLeft := int(pos.X)-space > 4

	// Keep the label away from the other cell involved so it isn't covered.
	other := s.target
	if c == s.target {
		other = s.hunter
	}
	right := fitsRight
	if other != nil && other != c {
		right = other.Pos.X < pos.X
		if right && !fitsRight || !right && !fitsLeft {
			right = !right
		}
	}

	// Line the underline of the last line up with the middle of the cell.
	lines := len(t.AllLines())
	lh := display.LineHeight()
	underline := 9
	top := int(pos.Y) - underline - (lines-1)*lh
	top = min(max(top, 8), s.height-lines*lh-8)

	cx := int(pos.X)
	l := &label{
		text:  t,
		line:  line,
		top:   top,
		y:     top + (lines-1)*lh + underline,
		x:     cx + int(c.Radius) + labelGap,
		left:  cx,
		right: cx + space,
	}
	if !right {
		l.x = cx - space
		l.left = l.x
		l.right = cx
	}
	s.label = l
}

// predict returns roughly where a cell will be after the passed amount of
// seconds.
func (s *Simulation) predict(c *cell.Cell, seconds float32) rl.Vector2 {
	pos := c.Pos

	switch {
	case c == s.target && s.phase == assimilate && c.Mode == cell.Disintegrating:
		// The target eases towards its goal over the imitation.
		away := rl.Vector2Normalize(rl.Vector2Subtract(c.Pos, s.hunter.Pos))
		goal := rl.Vector2Add(s.hunter.Pos, rl.Vector2Scale(away, float32(s.hunter.Radius)*0.3))
		pos = rl.Vector2Lerp(pos, goal, 1-float32(math.Exp(float64(-0.5*seconds))))

	case s.phase == assimilate:
		// Held cells barely move.

	default:
		// Move copies of the cells without their random wandering, so cells
		// pushing apart are followed as well as cells drifting.
		ghost := *s
		ghost.calm = true
		ghost.cells = make([]*cell.Cell, len(s.cells))
		var follow *cell.Cell
		for i, other := range s.cells {
			copied := *other
			ghost.cells[i] = &copied
			if other == c {
				follow = &copied
			}
			switch other {
			case s.hunter:
				ghost.hunter = &copied
			case s.target:
				ghost.target = &copied
			}
		}
		if follow == nil {
			break
		}
		step := float32(1.0 / 30)
		for t := float32(0); t < seconds; t += step {
			ghost.move(step)
		}
		pos = follow.Pos
	}

	pos.X = min(max(pos.X, margin), float32(s.width-margin))
	pos.Y = min(max(pos.Y, margin), float32(s.height-margin))
	return pos
}

// nearestHealthy returns the closest healthy cell to the hunter.
func (s *Simulation) nearestHealthy() *cell.Cell {
	var nearest *cell.Cell
	for _, c := range s.cells {
		if c.Kind != cell.Healthy {
			continue
		}
		if nearest == nil || rl.Vector2Distance(c.Pos, s.hunter.Pos) < rl.Vector2Distance(nearest.Pos, s.hunter.Pos) {
			nearest = c
		}
	}
	return nearest
}

// countHealthy returns the amount of healthy cells left.
func (s *Simulation) countHealthy() int {
	count := 0
	for _, c := range s.cells {
		if c.Kind == cell.Healthy {
			count++
		}
	}
	return count
}

// gap returns the distance between the edges of the hunter and target.
func (s *Simulation) gap() float32 {
	return rl.Vector2Distance(s.hunter.Pos, s.target.Pos) - float32(s.hunter.Radius+s.target.Radius)
}

// steer gradually turns and speeds a cell up towards another.
func steer(c, to *cell.Cell, speed, dt float32) {
	dir := rl.Vector2Normalize(rl.Vector2Subtract(to.Pos, c.Pos))
	c.Vel = rl.Vector2Lerp(c.Vel, rl.Vector2Scale(dir, speed), min(1, dt*huntAcceleration))
}

// startAssimilation begins the assimilation of the target.
func (s *Simulation) startAssimilation() {
	dir := rl.Vector2Subtract(s.hunter.Pos, s.target.Pos)
	s.target.From = math.Atan2(float64(dir.Y), float64(dir.X))

	if s.hunter.Kind == cell.Intruder {
		s.target.Mode = cell.Hollowing
		s.hunter.MorphInto = s.target
	} else {
		// The target is pulled over its hunter, which is assimilated
		// along with it.
		s.target.Mode = cell.Disintegrating
		s.hunter.Mode = cell.Seeping
		s.hunter.From = math.Atan2(float64(-dir.Y), float64(-dir.X))
	}
	s.setPhase(assimilate)
	s.setLabel(s.target, ">ASSIMILATION")
}

// setPhase moves the simulation to the next phase.
func (s *Simulation) setPhase(p phase) {
	s.phase = p
	s.timer = 0
}

// labelRead returns true when the label has been typed and read.
func (s *Simulation) labelRead() bool {
	return s.label != nil && s.label.read > readTime
}

// Update advances the simulation by the passed amount of seconds.
func (s *Simulation) Update(dt float32) {
	s.time += float64(dt)
	s.timer += dt

	if s.label != nil {
		if s.label.line.Done() {
			s.label.read += dt
		}
		if s.label.text.Done() {
			s.label.line.Update(dt)
		}
		s.label.text.Update(dt)
	}

	switch s.phase {
	case intro:
		if s.label == nil && s.inside(s.hunter) {
			s.setLabel(s.hunter, ">CELL\n>INTRUDER")
		}
		if s.labelRead() {
			s.target = s.nearestHealthy()
			s.setLabel(s.target, ">CELL\n>"+species)
			s.setPhase(identify)
		}

	case identify:
		steer(s.hunter, s.target, driftSpeed, dt)
		if s.labelRead() {
			s.label = nil
			s.setPhase(approach)
		}

	case approach:
		steer(s.hunter, s.target, huntSpeed, dt)

		// The intruder makes contact itself, while imitations spear their
		// target once they are very close.
		if s.hunter.Kind == cell.Intruder {
			if s.gap() < -float32(s.hunter.Radius)*0.4 {
				s.startAssimilation()
			}
		} else if s.gap() < spearGap {
			s.setPhase(lunge)
		}

	case lunge:
		s.spear = min(1, s.timer/spearTime)
		if s.spear >= 1 {
			s.setPhase(drag)
		}

	case drag:
		dir := rl.Vector2Normalize(rl.Vector2Subtract(s.hunter.Pos, s.target.Pos))
		s.target.Vel = rl.Vector2Scale(dir, dragSpeed)

		if s.gap() < -1 {
			s.spear = 0
			s.startAssimilation()
		}

	case assimilate:
		s.target.Infection += float64(dt) / assimilationTime

		switch s.target.Mode {
		case cell.Hollowing:
			// The intruder slides into its first victim and takes on its
			// shape. Near the end it keeps a steady pace so that it reaches the
			// middle as it's absorbed, rather than jumping there.
			remaining := float32((cell.HollowEnd - s.target.Infection) * assimilationTime)
			rate := min(1, max(dt*mergeRate, dt/max(remaining, dt)))
			s.hunter.Pos = rl.Vector2Lerp(s.hunter.Pos, s.target.Pos, rate)
			s.hunter.Morph = min(1, s.target.Infection/cell.HollowEnd)
			s.hunter.Absorbed = s.target.Infection >= cell.HollowEnd

		case cell.Disintegrating:
			// The target is pulled over the top of the imitation, stopping
			// just short of its middle so they can split apart later.
			s.hunter.Infection = s.target.Infection
			away := rl.Vector2Normalize(rl.Vector2Subtract(s.target.Pos, s.hunter.Pos))
			goal := rl.Vector2Add(s.hunter.Pos, rl.Vector2Scale(away, float32(s.hunter.Radius)*0.3))
			s.target.Pos = rl.Vector2Lerp(s.target.Pos, goal, min(1, dt*0.5))
		}

		if s.target.Infection >= 1 {
			s.target.Infection = 0
			s.hunter.Infection = 0
			s.target.Kind = cell.Imitation
			s.setPhase(complete)
			s.setLabel(s.target, ">ASSIMILATION COMPLETE\n>CELL\n>"+species+"\n>IMITATION")
		}

	case complete:
		if s.labelRead() {
			s.label = nil
			s.round++

			if s.countHealthy() <= 1 {
				s.setPhase(finished)
				break
			}

			s.hunter = s.target
			s.target = s.nearestHealthy()
			s.setLabel(s.target, ">CELL\n>"+species)
			s.setPhase(identify)
		}
	}

	s.move(dt)
}

// move moves all the cells.
func (s *Simulation) move(dt float32) {
	live := s.cells[:0]

	for _, c := range s.cells {
		if c.Absorbed {
			continue
		}
		live = append(live, c)

		hunting := c == s.hunter && (s.phase == identify || s.phase == approach) ||
			c == s.target && s.phase == drag
		held := c == s.target && (s.phase == lunge || s.phase == assimilate) ||
			c == s.hunter && (s.phase == lunge || s.phase == drag || s.phase == assimilate)

		switch {
		case hunting:
			// The velocity has already been set towards the other cell.

		case held:
			c.Vel = rl.Vector2Scale(c.Vel, 0.9)

		case !s.inside(c):
			centre := rl.NewVector2(float32(s.width)/2, float32(s.height)/2)
			dir := rl.Vector2Normalize(rl.Vector2Subtract(centre, c.Pos))
			c.Vel = limit(rl.Vector2Add(c.Vel, rl.Vector2Scale(dir, dt*4)), returnSpeed)

		default:
			wander := s.separation(c)
			if !s.calm {
				wander = rl.Vector2Add(wander, rl.NewVector2(rand.Float32()*2-1, rand.Float32()*2-1))
			}
			c.Vel = limit(rl.Vector2Add(c.Vel, rl.Vector2Scale(wander, dt*2)), driftSpeed)
		}

		c.Pos = rl.Vector2Add(c.Pos, rl.Vector2Scale(c.Vel, dt))
	}

	s.cells = live
}

// separation returns a push away from any cells that are too close.
func (s *Simulation) separation(c *cell.Cell) rl.Vector2 {
	push := rl.Vector2Zero()
	for _, other := range s.cells {
		if other == c {
			continue
		}
		// The target doesn't shy away from its hunter until they split
		// after assimilation.
		pair := c == s.target && other == s.hunter || c == s.hunter && other == s.target
		if pair && s.phase != complete {
			continue
		}
		space := float32(c.Radius+other.Radius) * 1.5
		away := rl.Vector2Subtract(c.Pos, other.Pos)
		if distance := rl.Vector2Length(away); distance > 0.01 && distance < space {
			push = rl.Vector2Add(push, rl.Vector2Scale(rl.Vector2Normalize(away), 2*(space-distance)/space))
		}
	}
	return push
}

// Done returns true when the simulation has finished.
func (s *Simulation) Done() bool {
	return s.phase == finished && s.timer > readTime
}

// Draw draws the simulation.
func (s *Simulation) Draw() {
	if s.phase == intro {
		clock := time.Now().Format("15:04:05")
		display.DrawText(clock, 8, 6)
	}

	if s.phase == lunge || s.phase == drag {
		s.drawSpear()
	}

	// The target is drawn last so its wall shows over a merging intruder.
	for _, c := range s.cells {
		if c != s.target {
			c.Draw(s.time)
		}
	}
	if s.target != nil && !s.target.Absorbed {
		s.target.Draw(s.time)
	}

	if s.label != nil {
		s.drawLabel()
	}
}

// drawSpear draws the spear reaching from the hunter into the target.
func (s *Simulation) drawSpear() {
	dir := rl.Vector2Normalize(rl.Vector2Subtract(s.target.Pos, s.hunter.Pos))
	from := rl.Vector2Add(s.hunter.Pos, rl.Vector2Scale(dir, float32(s.hunter.Radius)*0.8))
	to := rl.Vector2Subtract(s.target.Pos, rl.Vector2Scale(dir, float32(s.target.Radius)*0.8))
	tip := rl.Vector2Lerp(from, to, s.spear)

	rl.DrawLine(int32(from.X), int32(from.Y), int32(tip.X), int32(tip.Y), display.Infection)
}

// drawLabel draws the text and underline of the current label.
func (s *Simulation) drawLabel() {
	l := s.label
	lh := display.LineHeight()

	for i, line := range l.text.Lines() {
		display.DrawText(line, l.x, l.top+i*lh)
	}

	// The underline is typed from left to right whichever side of the cell
	// the label is on.
	typed := len(l.line.Lines()[0])
	if typed == 0 {
		return
	}
	end := l.left + (l.right-l.left)*typed/len(l.line.AllLines()[0])
	rl.DrawLine(int32(l.left), int32(l.y), int32(end), int32(l.y), display.Text)
}

// limit returns the vector shortened to the passed length if it is longer.
func limit(v rl.Vector2, length float32) rl.Vector2 {
	if rl.Vector2Length(v) > length {
		return rl.Vector2Scale(rl.Vector2Normalize(v), length)
	}
	return v
}
