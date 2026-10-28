package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	if s := Load(filepath.Join(t.TempDir(), "none.json")); !reflect.DeepEqual(s, Settings{}) {
		t.Fatalf("got %+v, want the defaults", s)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "screensaver-manager", "settings.json")
	for _, theme := range []string{ThemeDark, ThemeLight, ""} {
		if err := Save(path, Settings{Theme: theme}); err != nil {
			t.Fatal(err)
		}
		if s := Load(path); s.Theme != theme {
			t.Fatalf("saved theme %q, loaded %q", theme, s.Theme)
		}
	}
}

func TestLoadUnusable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	for _, data := range []string{`{`, `{"theme":"purple"}`} {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		if s := Load(path); s.Theme != "" {
			t.Errorf("%s gave theme %q, want the system's", data, s.Theme)
		}
	}
}

func TestSaveAndLoadAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	want := Settings{Theme: ThemeDark, Saver: "screensaver-the-thing", Timer: "5m", Inhibitors: []string{"vlc", "mpv"}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got := Load(path); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestLauncherArgs(t *testing.T) {
	for _, c := range []struct {
		s    Settings
		want []string
	}{
		{Settings{Saver: "screensaver-the-thing"}, []string{"-saver", "screensaver-the-thing"}},
		{Settings{Saver: "s", Timer: "5m"}, []string{"-saver", "s", "-timer", "5m"}},
		{Settings{Saver: "s", Inhibitors: []string{"vlc", "mpv"}}, []string{"-saver", "s", "-inhibit", "vlc,mpv"}},
	} {
		if got := c.s.LauncherArgs(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%+v gives %q, want %q", c.s, got, c.want)
		}
	}
}
