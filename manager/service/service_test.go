package service

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fake returns a manager whose unit file goes into a temporary directory, and
// which records the systemctl commands it runs rather than running them.
func fake(t *testing.T, show string) (*Manager, *[]string) {
	t.Helper()
	var ran []string
	m := &Manager{Dir: filepath.Join(t.TempDir(), "systemd", "user")}
	m.Systemctl = func(args ...string) (string, error) {
		ran = append(ran, strings.Join(args, " "))
		if args[0] == "show" {
			return show, nil
		}
		return "", nil
	}
	return m, &ran
}

func TestUnit(t *testing.T) {
	unit := Unit("/home/u/go/bin/screensaver-launcher", []string{"-saver", "screensaver-the-thing", "-inhibit", "vlc,mpv"})
	for _, want := range []string{
		"ExecStart=/home/u/go/bin/screensaver-launcher -saver screensaver-the-thing -inhibit vlc,mpv\n",
		"PartOf=graphical-session.target\n",
		"WantedBy=graphical-session.target\n",
		"Restart=on-failure\n",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("unit has no %q:\n%s", want, unit)
		}
	}
}

func TestQuote(t *testing.T) {
	for path, want := range map[string]string{
		"/usr/bin/x":       "/usr/bin/x",
		"/home/my files/x": `"/home/my files/x"`,
		`/a"b`:             `"/a\"b"`,
		"/100%/$HOME":      "/100%%/$$HOME",
		`/back\slash/x y`:  `"/back\\slash/x y"`,
	} {
		if got := quote(path); got != want {
			t.Errorf("quote(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestInstall(t *testing.T) {
	m, ran := fake(t, "")
	if err := m.Install("/bin/launcher", []string{"-saver", "x"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(m.Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != Unit("/bin/launcher", []string{"-saver", "x"}) {
		t.Fatalf("unit file is:\n%s", data)
	}
	want := []string{"daemon-reload", "enable " + Name, "restart " + Name}
	if !reflect.DeepEqual(*ran, want) {
		t.Fatalf("ran %q, want %q", *ran, want)
	}
}

func TestInstallFails(t *testing.T) {
	m, _ := fake(t, "")
	var ran []string
	m.Systemctl = func(args ...string) (string, error) {
		ran = append(ran, args[0])
		if args[0] == "enable" {
			return "", errors.New("no")
		}
		return "", nil
	}
	if err := m.Install("/bin/launcher", []string{"-saver", "x"}); err == nil {
		t.Fatal("no error")
	}
	// Nothing is started once a command has failed.
	if !reflect.DeepEqual(ran, []string{"daemon-reload", "enable"}) {
		t.Fatalf("ran %q", ran)
	}
}

func TestUninstall(t *testing.T) {
	m, ran := fake(t, "")
	// Nothing is done when there is nothing installed.
	if err := m.Uninstall(); err != nil || len(*ran) != 0 {
		t.Fatalf("got %v, ran %q", err, *ran)
	}
	if err := m.Install("/bin/launcher", []string{"-saver", "x"}); err != nil {
		t.Fatal(err)
	}
	*ran = nil
	if err := m.Uninstall(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(m.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unit file is still there: %v", err)
	}
	want := []string{"disable --now " + Name, "daemon-reload"}
	if !reflect.DeepEqual(*ran, want) {
		t.Fatalf("ran %q, want %q", *ran, want)
	}
}

func TestControl(t *testing.T) {
	m, ran := fake(t, "")
	for _, f := range []func() error{m.Start, m.Stop} {
		if err := f(); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"start " + Name, "stop " + Name}
	if !reflect.DeepEqual(*ran, want) {
		t.Fatalf("ran %q, want %q", *ran, want)
	}
}

func TestStatus(t *testing.T) {
	m, _ := fake(t, "MainPID=4242\nActiveState=active\nSubState=running\nUnitFileState=enabled\n")
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.Path(), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	want := Status{Installed: true, Enabled: true, State: "active", Sub: "running", PID: 4242}
	if s != want || !s.Running() {
		t.Fatalf("got %+v, want %+v", s, want)
	}
}

func TestStatusNotInstalled(t *testing.T) {
	m, _ := fake(t, "MainPID=0\nActiveState=inactive\nSubState=dead\nUnitFileState=\n")
	s, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	if s.Installed || s.Enabled || s.Running() {
		t.Fatalf("got %+v", s)
	}
}

func TestReconfigure(t *testing.T) {
	m, ran := fake(t, "")
	if err := m.Reconfigure("/bin/launcher", []string{"-saver", "y", "-timer", "5m"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(m.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ExecStart=/bin/launcher -saver y -timer 5m\n") {
		t.Fatalf("unit file is:\n%s", data)
	}
	// A launcher not running is not started by being reconfigured.
	want := []string{"daemon-reload", "try-restart " + Name}
	if !reflect.DeepEqual(*ran, want) {
		t.Fatalf("ran %q, want %q", *ran, want)
	}
}
