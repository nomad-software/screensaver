// Package screen lists the savers in this repository, with their previews, so
// that tools managing the launcher can show them. Each saver is a directory of
// saver/ with a preview at assets/images/preview.gif, built into an executable
// named after the directory.
package screen

import (
	"embed"
	"io/fs"
	"path"
	"strings"
)

//go:embed saver/*/assets/images/preview.gif
var previews embed.FS

// Saver is one of the savers.
type Saver struct {
	// Name is the name of its executable, e.g. screensaver-digital-rain.
	Name string

	// Title is its name to show, e.g. Digital Rain.
	Title string

	// Preview is an animated GIF of it running.
	Preview []byte
}

// Savers returns every saver, in the order of their names.
func Savers() []Saver {
	files, _ := fs.Glob(previews, "saver/*/assets/images/preview.gif")
	savers := make([]Saver, 0, len(files))
	for _, f := range files {
		dir := strings.Split(f, "/")[1]
		data, _ := previews.ReadFile(f)
		savers = append(savers, Saver{
			Name:    "screensaver-" + strings.ReplaceAll(dir, "_", "-"),
			Title:   title(dir),
			Preview: data,
		})
	}
	return savers
}

// minor are the words left in lower case within a title.
var minor = map[string]bool{"a": true, "an": true, "and": true, "of": true, "the": true}

// title makes a directory name such as digital_rain into one to show, such as
// Digital Rain.
func title(dir string) string {
	words := strings.Split(path.Base(dir), "_")
	for i, w := range words {
		if w != "" && (i == 0 || !minor[w]) {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
