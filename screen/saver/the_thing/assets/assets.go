package assets

import (
	"embed"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/assets"
)

var (
	//go:embed shaders/crt.fs
	fs embed.FS

	store = assets.New(fs)
)

// ShaderCollection contains all shaders used in the saver.
type ShaderCollection struct {
	crt     rl.Shader
	timeLoc int32
}

// NewShaderCollection creates a new shader collection.
func NewShaderCollection() *ShaderCollection {
	return &ShaderCollection{
		crt: store.LoadShader("shaders/crt.fs"),
	}
}

// CRT returns the crt shader. The scale is the amount of screen pixels used to
// draw one virtual pixel.
func (s *ShaderCollection) CRT(width, height, scale int) rl.Shader {
	resolutionLoc := rl.GetShaderLocation(s.crt, "resolution")
	resolution := []float32{float32(width), float32(height)}
	rl.SetShaderValue(s.crt, resolutionLoc, resolution, rl.ShaderUniformVec2)

	glowSpreadLoc := rl.GetShaderLocation(s.crt, "glowSpread")
	glowSpread := []float32{float32(scale) * 0.5}
	rl.SetShaderValue(s.crt, glowSpreadLoc, glowSpread, rl.ShaderUniformFloat)

	s.timeLoc = rl.GetShaderLocation(s.crt, "time")

	return s.crt
}

// SetTime updates the time used by the crt shader for flicker.
func (s *ShaderCollection) SetTime(time float32) {
	rl.SetShaderValue(s.crt, s.timeLoc, []float32{time}, rl.ShaderUniformFloat)
}
