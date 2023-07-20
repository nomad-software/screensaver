package assets

import (
	"embed"
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/assets"
	"github.com/nomad-software/screensaver/screen/saver/night_sky/sky"
)

var (
	//go:embed json/stars.json
	//go:embed images/star.png
	//go:embed images/circle.png
	//go:embed shaders/alpha_discard.fs
	fs embed.FS

	store = assets.New(fs)
)

// JsonCollection contains all json used in the saver.
type JsonCollection struct {
	stars []*sky.Star
}

// NewJsonCollection creates a new json collection.
func NewJsonCollection() *JsonCollection {
	stars := make([]*sky.Star, 0)
	store.LoadJson("json/stars.json", &stars)

	return &JsonCollection{
		stars: stars,
	}
}

func (c *JsonCollection) Prepare() {
	var maxX float64
	var maxY float64
	var maxZ float64
	var maxM float64
	var minM float64
	var maxO float64
	var minO float64 = 100.0
	var maxSize float64
	var minSize float64 = 100.0

	for _, s := range c.stars {
		if s.Coords.X > maxX {
			maxX = s.Coords.X
		}
		if s.Coords.Y > maxY {
			maxY = s.Coords.Y
		}
		if s.Coords.Z > maxZ {
			maxZ = s.Coords.Z
		}
		if s.Magnitude > maxM {
			maxM = s.Magnitude
		}
		if s.Magnitude < minM {
			minM = s.Magnitude
		}

		mag := math.Pow(1.33, (-s.Magnitude)+21)
		if mag > maxO {
			maxO = mag
		}
		if mag < minO {
			minO = mag
		}

		size := (mag / 120) + 0.2
		if size > maxSize {
			maxSize = size
		}
		if size < minSize {
			minSize = size
		}

		s.Color = rl.NewColor(s.ColorRGB.R, s.ColorRGB.G, s.ColorRGB.B, uint8(mag))
		s.CoordsV = rl.NewVector3(float32(s.Coords.X), float32(s.Coords.Y), float32(s.Coords.Z))
		s.Size = float32(size)
	}

	// slices.SortFunc(c.stars, func(a, b *sky.Star) bool {
	// 	return a.Size > b.Size
	// })

	fmt.Printf("x: %f, y: %f, z: %f\n", maxX, maxY, maxZ)
	fmt.Printf("mag min: %v, mag max: %v\n", minM, maxM)
	fmt.Printf("o min: %v, o max: %v\n", minO, maxO)
	fmt.Printf("size min: %v, size max: %v\n", minSize, maxSize)
}

// Stars returns the parsed stars.
func (c *JsonCollection) Stars() []*sky.Star {
	return c.stars
}

// TextureCollection contains all textures used in the saver.
type TextureCollection struct {
	star   rl.Texture2D
	circle rl.Texture2D
}

// NewTextureCollection creates a new texture collection.
func NewTextureCollection() *TextureCollection {
	return &TextureCollection{
		star:   store.LoadPngTexture("images/star.png"),
		circle: store.LoadPngTexture("images/circle.png"),
	}
}

// Star returns the normal star texture.
func (c *TextureCollection) Star() rl.Texture2D {
	return c.star
}

// Circle returns the circle texture.
func (c *TextureCollection) Circle() rl.Texture2D {
	return c.circle
}

// ShaderCollection contains all shaders used in the saver.
type ShaderCollection struct {
	alphaDiscard rl.Shader
}

// NewShaderCollection creates a new shader collection.
func NewShaderCollection() *ShaderCollection {
	return &ShaderCollection{
		alphaDiscard: store.LoadShader("shaders/alpha_discard.fs"),
	}
}

// AlphaDiscard returns the alpha discard shader.
func (s *ShaderCollection) AlphaDiscard() rl.Shader {
	return s.alphaDiscard
}
