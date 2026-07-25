package assets

import (
	"embed"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/assets"
)

var (
	//go:embed shaders/blur.fs
	fs embed.FS

	store = assets.New(fs)
)

// ShaderCollection contains all shaders used in the saver.
type ShaderCollection struct {
	blur rl.Shader
}

// NewShaderCollection creates a new shader collection.
func NewShaderCollection() *ShaderCollection {
	return &ShaderCollection{
		blur: store.LoadShader("shaders/blur.fs"),
	}
}

// Blur returns the blur shader.
func (s *ShaderCollection) Blur(width, height int) rl.Shader {
	texelSizeLoc := rl.GetShaderLocation(s.blur, "texelSize")
	texelSize := []float32{1.0 / float32(width), 1.0 / float32(height)}
	rl.SetShaderValue(s.blur, texelSizeLoc, texelSize, rl.ShaderUniformVec2)

	blurAmountLoc := rl.GetShaderLocation(s.blur, "blurAmount")
	blurAmount := []float32{1.0}
	rl.SetShaderValue(s.blur, blurAmountLoc, blurAmount, rl.ShaderUniformFloat)

	return s.blur
}
