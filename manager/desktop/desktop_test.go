package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEntry(t *testing.T) {
	entry := Entry("/home/me/bin/screensaver-manager", "screensaver-manager", "/home/me/.local/share/icons/hicolor/256x256/apps/screensaver-manager.png")
	for _, line := range []string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Screensaver Manager",
		"Exec=/home/me/bin/screensaver-manager",
		"TryExec=/home/me/bin/screensaver-manager",
		"Icon=/home/me/.local/share/icons/hicolor/256x256/apps/screensaver-manager.png",
		"StartupWMClass=screensaver-manager",
	} {
		if !strings.Contains(entry, line+"\n") {
			t.Errorf("no %q in\n%s", line, entry)
		}
	}
	if !strings.HasPrefix(entry, "[Desktop Entry]\n") {
		t.Errorf("entry does not start with its group:\n%s", entry)
	}
}

func TestExecQuoting(t *testing.T) {
	for path, want := range map[string]string{
		"/opt/screensaver-manager":                     "/opt/screensaver-manager",
		"/home/me/My Apps/ScreensaverManager.AppImage": `"/home/me/My Apps/ScreensaverManager.AppImage"`,
		`/tmp/a"b`:        `"/tmp/a\\"b"`,
		"/tmp/$HOME/x":    `"/tmp/\\$HOME/x"`,
		"/tmp/100%/x":     "/tmp/100%%/x",
		`/tmp/back\slash`: `"/tmp/back\\\\slash"`,
	} {
		entry := Entry(path, "x", "/x.png")
		if !strings.Contains(entry, "\nExec="+want+"\n") {
			t.Errorf("Exec for %q: want %q in\n%s", path, want, entry)
		}
	}
}

func TestExecutable(t *testing.T) {
	// An AppImage is started again through the AppImage itself.
	t.Setenv("APPIMAGE", "/home/me/Apps/ScreensaverManager-x86_64.AppImage")
	if got, err := Executable(); err != nil || got != "/home/me/Apps/ScreensaverManager-x86_64.AppImage" {
		t.Errorf("in an AppImage: %q, %v", got, err)
	}

	// Otherwise it is the program itself, with any links followed.
	t.Setenv("APPIMAGE", "")
	got, err := Executable()
	if err != nil || !filepath.IsAbs(got) {
		t.Fatalf("Executable() = %q, %v", got, err)
	}
	if resolved, _ := filepath.EvalSymlinks(got); resolved != got {
		t.Errorf("links not followed: %q is %q", got, resolved)
	}
}

func TestDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/data/home")
	if got, err := Dir(); err != nil || got != "/data/home" {
		t.Errorf("with XDG_DATA_HOME: %q, %v", got, err)
	}
	// A relative one is ignored, as the specification says.
	t.Setenv("XDG_DATA_HOME", "relative")
	t.Setenv("HOME", "/home/me")
	if got, err := Dir(); err != nil || got != "/home/me/.local/share" {
		t.Errorf("without XDG_DATA_HOME: %q, %v", got, err)
	}
}

func TestWrite(t *testing.T) {
	data := t.TempDir()
	entryPath := filepath.Join(data, "applications", "screensaver-manager.desktop")
	iconPath := filepath.Join(data, "icons", "hicolor", "256x256", "apps", "screensaver-manager.png")

	if err := Write(data, "/opt/one/screensaver-manager", "screensaver-manager", []byte("png")); err != nil {
		t.Fatal(err)
	}
	entry, err := os.ReadFile(entryPath)
	if err != nil || !strings.Contains(string(entry), "Exec=/opt/one/screensaver-manager\n") {
		t.Fatalf("entry %q, %v", entry, err)
	}
	if icon, err := os.ReadFile(iconPath); err != nil || string(icon) != "png" {
		t.Errorf("icon %q, %v", icon, err)
	}
	if info, err := os.Stat(entryPath); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("entry mode %v, %v", info.Mode(), err)
	}

	// The same again leaves the files alone.
	old := time.Now().Add(-time.Hour)
	os.Chtimes(entryPath, old, old)
	if err := Write(data, "/opt/one/screensaver-manager", "screensaver-manager", []byte("png")); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(entryPath); !info.ModTime().Equal(old) {
		t.Error("an entry that was already right was written again")
	}

	// Run from elsewhere, or under another name, it is rewritten.
	if err := Write(data, "/home/me/Apps/ScreensaverManager.AppImage", "screensaver-manager", []byte("png")); err != nil {
		t.Fatal(err)
	}
	entry, _ = os.ReadFile(entryPath)
	if !strings.Contains(string(entry), "Exec=/home/me/Apps/ScreensaverManager.AppImage\n") || strings.Contains(string(entry), "/opt/one") {
		t.Errorf("entry not rewritten:\n%s", entry)
	}
	// No temporary files are left behind.
	if files, _ := os.ReadDir(filepath.Dir(entryPath)); len(files) != 1 {
		t.Errorf("files left in applications: %v", files)
	}
}

func TestInstall(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("APPIMAGE", "/home/me/Apps/My ScreensaverManager.AppImage")

	icon := []byte("\x89PNG icon")
	if err := Install(icon); err != nil {
		t.Fatal(err)
	}

	// The entry starts the AppImage, quoted as its name has a space.
	entry, err := os.ReadFile(filepath.Join(data, "applications", "screensaver-manager.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	self, _ := os.Executable()
	// The icon is named by its path, not looked for in the theme.
	image := filepath.Join(data, "icons", "hicolor", "256x256", "apps", "screensaver-manager.png")
	for _, line := range []string{`Exec="/home/me/Apps/My ScreensaverManager.AppImage"`, "Icon=" + image, "StartupWMClass=" + filepath.Base(self)} {
		if !strings.Contains(string(entry), line+"\n") {
			t.Errorf("no %q in\n%s", line, entry)
		}
	}

	// The icon is the one given.
	if got, err := os.ReadFile(image); err != nil || string(got) != string(icon) {
		t.Errorf("icon %q, %v", got, err)
	}
}

func TestWriteFileEmpty(t *testing.T) {
	// Nothing to write writes nothing, leaving what is there.
	path := filepath.Join(t.TempDir(), "file")
	if err := writeFile(path, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a file was written of nothing: %v", err)
	}
}

func TestWriteFileErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// A folder cannot be made under a file.
	if err := writeFile(filepath.Join(file, "below", "x"), []byte("x")); err == nil {
		t.Error("writeFile under a file succeeded")
	}
	// A folder in the way cannot be replaced, and nothing is left behind.
	folder := filepath.Join(dir, "folder")
	if err := os.MkdirAll(filepath.Join(folder, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(folder, []byte("x")); err == nil {
		t.Error("writeFile over a folder succeeded")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Errorf("files left behind: %v", entries)
	}
}

func TestWriteFileUnwritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write into any folder")
	}
	// A folder that cannot be written to has no room for the temporary file.
	dir := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	if err := writeFile(filepath.Join(dir, "x"), []byte("x")); err == nil {
		t.Error("writeFile into a folder that cannot be written to succeeded")
	}
}

func TestWriteErrors(t *testing.T) {
	// Data under a file has neither folder, and both are said to fail.
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	err := Write(file, "/opt/screensaver-manager", "screensaver-manager", []byte("png"))
	if err == nil {
		t.Fatal("Write under a file succeeded")
	}
	if n := strings.Count(err.Error(), "\n") + 1; n != 2 {
		t.Errorf("Write said %d errors, want 2: %v", n, err)
	}
}

func TestInstallWithoutHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")
	if err := Install([]byte("png")); err == nil {
		t.Error("Install with nowhere to install succeeded")
	}
}
