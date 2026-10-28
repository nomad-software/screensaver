package process

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// fakeProc makes a proc file system with a process for each id, with its
// name and command line.
func fakeProc(t *testing.T, procs map[string][2]string) string {
	t.Helper()
	root := t.TempDir()
	for pid, p := range procs {
		dir := filepath.Join(root, pid)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(p[0]+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(p[1]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Files in /proc that are not processes are passed over.
	if err := os.WriteFile(filepath.Join(root, "uptime"), []byte("1 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sys"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunning(t *testing.T) {
	root := fakeProc(t, map[string][2]string{
		"10": {"vlc", "/usr/bin/vlc\x00film.mkv\x00"},
		"11": {"mpv", "mpv\x00a.mp4\x00"},
		"12": {"mpv", "mpv\x00b.mp4\x00"},
		// The kernel cuts names to 15 characters, so a longer one is matched
		// by the program's file name.
		"13": {"screensaver-lau", "/home/u/go/bin/screensaver-launcher\x00"},
		// A name within another is not a match.
		"14": {"steamwebhelper", "/opt/steam/steamwebhelper\x00"},
		// A kernel thread has no command line.
		"15": {"kworker/0:1", ""},
	})
	got, err := runningIn(root, []string{"vlc", "mpv", "steam", "screensaver-launcher", ""})
	if err != nil {
		t.Fatal(err)
	}
	for _, pids := range got {
		// The order of the ids depends on the order the directory is read in.
		if len(pids) == 2 && pids[0] > pids[1] {
			pids[0], pids[1] = pids[1], pids[0]
		}
	}
	want := map[string][]int{
		"vlc":                  {10},
		"mpv":                  {11, 12},
		"screensaver-launcher": {13},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRunningNoNames(t *testing.T) {
	got, err := runningIn("/does/not/exist", nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v, want nothing", got, err)
	}
}

func TestRunningSelf(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(self)
	got, err := Running([]string{name})
	if err != nil {
		t.Fatal(err)
	}
	pids := got[name]
	for _, pid := range pids {
		if pid == os.Getpid() {
			return
		}
	}
	t.Fatalf("this test's process %d is not among %v", os.Getpid(), pids)
}
