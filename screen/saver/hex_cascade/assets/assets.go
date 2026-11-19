package assets

import (
	"embed"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/assets"
)

var (
	//go:embed shaders/glow.fs
	fs embed.FS

	store = assets.New(fs)
)

// ShaderCollection contains all shaders used in the saver.
type ShaderCollection struct {
	glow rl.Shader
}

// NewShaderCollection creates a new shader collection.
func NewShaderCollection() *ShaderCollection {
	return &ShaderCollection{
		glow: store.LoadShader("shaders/glow.fs"),
	}
}

// Glow returns the glow shader, which makes bright tiles glow by the given
// radius, in pixels.
func (s *ShaderCollection) Glow(width, height int, radius float32) rl.Shader {
	texelSizeLoc := rl.GetShaderLocation(s.glow, "texelSize")
	texelSize := []float32{1.0 / float32(width), 1.0 / float32(height)}
	rl.SetShaderValue(s.glow, texelSizeLoc, texelSize, rl.ShaderUniformVec2)

	glowRadiusLoc := rl.GetShaderLocation(s.glow, "glowRadius")
	glowRadius := []float32{radius}
	rl.SetShaderValue(s.glow, glowRadiusLoc, glowRadius, rl.ShaderUniformFloat)

	return s.glow
}
