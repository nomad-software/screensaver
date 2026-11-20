package assets

import (
	"embed"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/assets"
)

var (
	//go:embed shaders/glitch.fs
	fs embed.FS

	store = assets.New(fs)
)

// NewGlitchShader loads the failing crt monitor shader.
func NewGlitchShader() rl.Shader {
	return store.LoadShader("shaders/glitch.fs")
}
