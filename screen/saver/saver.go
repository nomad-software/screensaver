package saver

import (
	"errors"
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/nomad-software/screensaver/output"
)

var (
	errInputDetected = errors.New("detected")
)

// Saver is a generic screensaver with timer functionality that quits gracefully
// if input is detected.
type Saver struct {
	ebiten.Game // The ebiten game interface.

	mouseX int // The current horizontal mouse position.
	mouseY int // The current vertical mouse position.

	buffer *ebiten.Image // The primary drawing buffer.

	ScreenWidth  int // The current screen width.
	ScreenHeight int // The current screen height.
}

// CheckInput will return an error when any key is pressed or
// there is any mouse movement.
func (s *Saver) CheckInput() error {
	if len(inpututil.AppendPressedKeys([]ebiten.Key{})) > 0 {
		return fmt.Errorf("key press %w", errInputDetected)
	}

	x, y := ebiten.CursorPosition()
	if (s.mouseX != 0 && s.mouseY != 0) && (x != s.mouseX || y != s.mouseY) {
		return fmt.Errorf("mouse movement %w", errInputDetected)
	}

	s.mouseX = x
	s.mouseY = y

	return nil
}

// Update is called every tick. Tick is a time unit for logical updating. The
// default value is 1/60 [s], then Update is called 60 times per second by
// default. Update updates the game's logical state. Update returns an error
// value. In this code, In general, when the update function returns a non-nil
// error, the Ebitengine game suspends.
func (s *Saver) Update() error {
	if s.buffer == nil {
		s.buffer = ebiten.NewImage(s.ScreenWidth, s.ScreenHeight)
		output.ScreenInfo("initialised screen buffer")
	}

	return s.CheckInput()
}

// Draw is called every frame. Frame is a time unit for rendering and this
// depends on the display's refresh rate. If the monitor's refresh rate is 60
// [Hz], Draw is called 60 times per second. Draw takes an argument screen,
// which is a pointer to an ebiten.Image. This screen argument is the final
// destination of rendering. The window shows the final state of screen every
// frame.
func (s *Saver) Draw(screen *ebiten.Image) {
	ebitenutil.DebugPrint(screen, "Override the draw method")
}

// Layout accepts an outside size, which is a window size on desktop, and
// returns the game's logical screen size.
func (s *Saver) Layout(width, height int) (int, int) {
	s.ScreenWidth = width
	s.ScreenHeight = height

	return s.ScreenWidth, s.ScreenHeight
}

// Blit draws the buffer to the screen.
func (s *Saver) Blit(screen *ebiten.Image) *ebiten.Image {
	screen.DrawImage(s.buffer, nil)
	return s.buffer
}

// Run runs the screensaver.
func Run(g ebiten.Game) {
	output.ScreenInfo("running")

	if err := ebiten.RunGame(g); err != nil {

		if errors.Is(err, errInputDetected) {
			output.ScreenInfo("%s", err)

		} else {
			output.ScreenErr("%s", err)
		}
	}

	output.ScreenInfo("exiting")
}
