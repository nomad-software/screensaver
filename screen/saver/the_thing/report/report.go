package report

import (
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/screen/saver/the_thing/display"
	"github.com/nomad-software/screensaver/screen/saver/the_thing/display/typewriter"
)

const (
	hold    = 6.0 // Seconds the finished report stays on screen.
	margin  = 10  // Gap between the screen edge and the border in pixels.
	padding = 4   // Gap between the border and the text in pixels.
)

// Report is a page of text typed out inside a border.
type Report struct {
	width  int
	height int

	text    *typewriter.Typewriter
	centred map[int]bool // Lines that are drawn centred.
	elapsed float32      // Time since typing finished.
}

// Probability creates the infection probability report.
func Probability(width, height int) *Report {
	return New(width, height, ""+
		"PROBABILITY THAT ONE OR MORE\n"+
		"TEAM MEMBERS MAY BE INFECTED\n"+
		"BY INTRUDER ORGANISM:|\n"+
		"^-75%-",
	)
}

// Projection creates the infection projection report.
func Projection(width, height int) *Report {
	return New(width, height, ""+
		"PROJECTION:\n"+
		"IF INTRUDER ORGANISM REACHES\n"+
		"CIVILIZED AREAS . . .||\n"+
		"\n"+
		"\n"+
		"ENTIRE WORLD POPULATION INFECTED\n"+
		"27,000 HOURS FROM FIRST CONTACT.",
	)
}

// New creates a new report. Lines starting with '^' are drawn centred and a
// '|' makes the typing pause.
func New(width, height int, text string) *Report {
	r := &Report{
		width:   width,
		height:  height,
		centred: make(map[int]bool),
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "^") {
			r.centred[i] = true
			lines[i] = line[1:]
		}
	}

	r.text = typewriter.New(strings.Join(lines, "\n"))

	return r
}

// Update advances the report by the passed amount of seconds.
func (r *Report) Update(dt float32) {
	if r.text.Done() {
		r.elapsed += dt
	}
	r.text.Update(dt)
}

// Done returns true when the report has finished being shown.
func (r *Report) Done() bool {
	return r.elapsed > hold
}

// Draw draws the report.
func (r *Report) Draw() {
	// The border is roughly the shape of an old 4:3 monitor.
	h := r.height - margin*2
	w := min(max(h*4/3, 240), r.width-margin*2)
	x := (r.width - w) / 2
	y := margin

	rl.DrawRectangleLines(int32(x), int32(y), int32(w), int32(h), display.Border)

	all := r.text.AllLines()
	textWidth := 0
	for _, line := range all {
		textWidth = max(textWidth, display.MeasureText(line))
	}

	// The text block is kept left aligned but centred overall within the
	// border, then starts a third of the way down like the film.
	left := x + max(padding, (w-textWidth)/2)
	top := y + h/4

	for i, line := range r.text.Lines() {
		lx := left
		if r.centred[i] {
			lx = left + (textWidth-display.MeasureText(all[i]))/2
		}
		display.DrawText(line, lx, top+i*display.LineHeight())
	}
}
