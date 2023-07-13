package assets

import (
	"embed"
	"encoding/json"
	_ "image/png"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/nomad-software/screensaver/output"
)

// Store is a helper for loading assets from an embedded filesystem.
type Store struct {
	fs embed.FS
}

// New creates a new asset store.
func New(fs embed.FS) *Store {
	return &Store{
		fs: fs,
	}
}

// LoadJson loads the specified json file from the store and unmarshals it to
// the passed arg.
func (s Store) LoadJson(name string, out any) {
	bytes, err := s.fs.ReadFile(name)
	output.OnError(err, "cannot read embedded json file")

	err = json.Unmarshal(bytes, out)
	output.OnError(err, "cannot unmarshal json")
}

// LoadPngImage retrieves a png from the store and creates an image from it.
func (s Store) LoadPngImage(name string) *rl.Image {
	bytes, err := s.fs.ReadFile(name)
	output.OnError(err, "cannot read embedded png image")

	image := rl.LoadImageFromMemory(".png", bytes, int32(len(bytes)))

	return image
}

// LoadPngTexture retrieves a png from the store and creates a texture from it.
func (s Store) LoadPngTexture(name string) rl.Texture2D {
	img := s.LoadPngImage(name)
	tex := rl.LoadTextureFromImage(img)

	rl.UnloadImage(img)

	return tex
}

// LoadShader retrieves a text file from the store and creates a shader from it.
func (s Store) LoadShader(name string) rl.Shader {
	bytes, err := s.fs.ReadFile(name)
	output.OnError(err, "cannot read embedded shader")

	return rl.LoadShaderFromMemory("", string(bytes))
}
