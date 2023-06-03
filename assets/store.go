package assets

import (
	"embed"
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

// LoadPngImage retrieves a png from the store and creates an image from it.
func (s Store) LoadPngImage(name string) *rl.Image {
	st, err := s.fs.ReadFile(name)
	output.OnError(err, "cannot read embedded png image")

	image := rl.LoadImageFromMemory(".png", st, int32(len(st)))

	return image
}

// LoadPngTexture retrieves a png from the store and creates a texture from it.
func (s Store) LoadPngTexture(name string) rl.Texture2D {
	return rl.LoadTextureFromImage(s.LoadPngImage(name))
}
