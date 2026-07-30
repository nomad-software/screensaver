package assets

import (
	"embed"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/assets"
)

var (
	//go:embed shaders/shader.fs
	fs embed.FS

	store = assets.New(fs)
)

// ShaderCollection contains all shaders used in the saver.
type ShaderCollection struct {
	shader rl.Shader
}

// NewShaderCollection creates a new shader collection.
func NewShaderCollection() *ShaderCollection {
	return &ShaderCollection{
		shader: store.LoadShader("shaders/shader.fs"),
	}
}

// Shader returns the shader.
func (s *ShaderCollection) Shader(width, height int) rl.Shader {
	resolutionLoc := rl.GetShaderLocation(s.shader, "u_resolution")
	resolution := []float32{float32(width), float32(height)}
	rl.SetShaderValue(s.shader, resolutionLoc, resolution, rl.ShaderUniformVec2)

	return s.shader
}
