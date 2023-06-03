package saver

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

var (
	mouseDelta = 0
)

// CreateWindow creates a simple screensaver window.
func CreateWindow(title string) (int, int) {
	rl.InitWindow(0, 0, title)

	monitor := rl.GetCurrentMonitor()
	width := rl.GetMonitorWidth(monitor)
	height := rl.GetMonitorHeight(monitor)

	rl.SetWindowState(rl.FlagWindowUndecorated | rl.FlagWindowTopmost)
	rl.SetWindowSize(width, height)
	rl.SetWindowPosition(0, 0)
	rl.ToggleFullscreen()
	rl.HideCursor()

	rl.SetMousePosition(width/2, height/2)

	return width, height
}

// CloseWindow closes the screensaver window.
func CloseWindow() {
	rl.CloseWindow()
}

// InputDetected will return true when any key is pressed or there is any mouse
// movement.
func InputDetected() bool {
	// Check all supported keyboard keys.
	if rl.GetKeyPressed() > 0 {
		return true
	}

	// Check all supported mouse buttons.
	for i := int32(0); i <= 6; i++ {
		if rl.IsMouseButtonPressed(i) {
			return true
		}
	}

	// Check the mouse wheel.
	if rl.GetMouseWheelMove() != 0 {
		return true
	}

	// Check for mouse movement.
	md := rl.GetMouseDelta()
	if md.X != 0 || md.Y != 0 {
		mouseDelta++
		if mouseDelta > 5 {
			return true
		}
	} else {
		mouseDelta = 0
	}

	return false
}
