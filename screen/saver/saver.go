package saver

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

var (
	mouseX int
	mouseY int
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
	if rl.GetKeyPressed() > 0 {
		return true
	}
	return false
}
