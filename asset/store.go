package asset

import (
	"bytes"
	"embed"
	"image"
	_ "image/png"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/nomad-software/screensaver/output"
)

// Store is a helper for loading assets from an embedded filesystem.
type Store struct {
	fs embed.FS
}

// NewStore creates a new asset store.
func NewStore(fs embed.FS) *Store {
	return &Store{
		fs: fs,
	}
}

// LoadImage retrieves an asset from the store and creates an ebiten image from it.
func (s Store) LoadImage(name string) *ebiten.Image {
	st, err := s.fs.ReadFile(name)
	output.OnError(err, "cannot read image")

	i, _, err := image.Decode(bytes.NewReader(st))
	output.OnError(err, "cannot decode image")

	output.ScreenInfo("loaded %s", name)

	return ebiten.NewImageFromImage(i)
}
