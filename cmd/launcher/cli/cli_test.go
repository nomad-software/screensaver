package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestOptions(t *testing.T) {
	opt := Options{Saver: "screensaver-the-thing", Timer: "5m", Inhibit: "vlc, mpv,,steam "}
	if err := opt.valid(); err != nil {
		t.Fatal(err)
	}
	if opt.TimerDuration != 5*time.Minute {
		t.Errorf("timer is %s", opt.TimerDuration)
	}
	if want := []string{"vlc", "mpv", "steam"}; !reflect.DeepEqual(opt.Inhibitors, want) {
		t.Errorf("inhibitors are %q, want %q", opt.Inhibitors, want)
	}
}

func TestOptionsSaverOnly(t *testing.T) {
	// The launcher runs with nothing but a saver, and inhibits nothing.
	opt := Options{Saver: "screensaver-the-thing", Timer: "15m"}
	if err := opt.valid(); err != nil {
		t.Fatal(err)
	}
	if len(opt.Inhibitors) != 0 {
		t.Errorf("inhibitors are %q, want none", opt.Inhibitors)
	}
}

func TestOptionsInvalid(t *testing.T) {
	if err := (&Options{Timer: "1m"}).valid(); err == nil {
		t.Fatal("no error with no saver")
	}
	if err := (&Options{Saver: "x", Timer: "soon"}).valid(); err == nil {
		t.Fatal("no error for a timer that cannot be parsed")
	}
}

func TestInhibited(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	name, ok := Inhibited([]string{"no-such-program", filepath.Base(self)})
	if !ok || name != filepath.Base(self) {
		t.Fatalf("got %q, %v, want this test", name, ok)
	}
	if _, ok := Inhibited([]string{"no-such-program"}); ok {
		t.Fatal("inhibited by a program not running")
	}
}
