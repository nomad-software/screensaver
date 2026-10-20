package typewriter

import (
	"strings"
	"unicode"

	"github.com/nomad-software/screensaver/screen/saver/the_thing/sound"
)

const (
	rate  = 10  // Characters typed per second.
	pause = 1.2 // Seconds to wait at each pause marker.
)

// Typewriter reveals text a character at a time. A '|' in the text is not
// displayed but makes the typewriter pause at that point.
type Typewriter struct {
	text    []rune
	pauses  map[int]int // Amount of pauses before the character at an index.
	typed   int
	elapsed float32
}

// New creates a new typewriter.
func New(text string) *Typewriter {
	t := &Typewriter{
		pauses: make(map[int]int),
	}

	for _, r := range text {
		if r == '|' {
			t.pauses[len(t.text)]++
			continue
		}
		t.text = append(t.text, r)
	}

	return t
}

// Update advances the typing by the passed amount of seconds.
func (t *Typewriter) Update(dt float32) {
	t.elapsed += dt

	for t.typed < len(t.text) {
		delay := float32(1.0/rate) + float32(t.pauses[t.typed])*pause
		if t.elapsed < delay {
			break
		}
		t.elapsed -= delay
		t.typed++

		if !unicode.IsSpace(t.text[t.typed-1]) {
			sound.Tick()
		}
	}
}

// Done returns true when all the text has been typed.
func (t *Typewriter) Done() bool {
	return t.typed >= len(t.text)
}

// Lines returns the lines of text typed so far.
func (t *Typewriter) Lines() []string {
	return strings.Split(string(t.text[:t.typed]), "\n")
}

// AllLines returns all lines of text, typed or not.
func (t *Typewriter) AllLines() []string {
	return strings.Split(string(t.text), "\n")
}
