package screen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSavers(t *testing.T) {
	savers := Savers()
	if len(savers) == 0 {
		t.Fatal("no savers")
	}
	for _, s := range savers {
		if !bytes.HasPrefix(s.Preview, []byte("GIF8")) {
			t.Errorf("%s: preview is not a GIF", s.Name)
		}
		// make install installs each saver from the file named for it in
		// its directory, under that name, which is the name listed.
		dir := strings.ReplaceAll(strings.TrimPrefix(s.Name, "screensaver-"), "-", "_")
		if _, err := os.Stat(filepath.Join("saver", dir, s.Name+".go")); err != nil {
			t.Errorf("%s: not installed under its name: %v", s.Name, err)
		}
	}
}

func TestTitle(t *testing.T) {
	for dir, want := range map[string]string{
		"digital_rain":     "Digital Rain",
		"shader_star_grid": "Shader Star Grid",
		"thing":            "Thing",
		"game_of_life":     "Game of Life",
		"the_thing":        "The Thing",
	} {
		if got := title(dir); got != want {
			t.Errorf("title(%q) = %q, want %q", dir, got, want)
		}
	}
}
