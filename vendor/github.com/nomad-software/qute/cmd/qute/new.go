package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"
)

// newApp writes a known-good starter: main.go, a window, a behaviour test and
// a screenshot test. If dir is not inside a Go module, it also writes go.mod.
func newApp(dir, module string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if entries, _ := os.ReadDir(dir); len(entries) > 0 {
		return fmt.Errorf("%s is not empty", dir)
	}
	abs, _ := filepath.Abs(dir)
	name := filepath.Base(abs)
	if module == "" {
		module = name
	}
	files := map[string]string{
		"main.go":            mainTmpl,
		"window.go":          windowTmpl,
		"window_test.go":     testTmpl,
		"screenshot_test.go": shotTmpl,
	}
	if !insideModule(abs) {
		files["go.mod"] = modTmpl
	}
	data := struct{ Module, Title string }{module, title(name)}
	for fn, src := range files {
		t := template.Must(template.New(fn).Parse(src))
		f, err := os.Create(filepath.Join(dir, fn))
		if err != nil {
			return err
		}
		if err := t.Execute(f, data); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	fmt.Printf("created %s\n\nnext:\n  cd %s\n", dir, dir)
	if files["go.mod"] != "" {
		fmt.Println("  go get github.com/nomad-software/qute@latest")
	}
	fmt.Println("  go test ./...        # behaviour + screenshot tests, headless")
	fmt.Println("  go run .")
	return nil
}

// title turns a directory name such as "my-app" into a window title, "My App".
func title(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i, w := range words {
		r, n := utf8.DecodeRuneInString(w)
		words[i] = string(unicode.ToUpper(r)) + w[n:]
	}
	return strings.Join(words, " ")
}

func insideModule(dir string) bool {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return true
		}
		if filepath.Dir(d) == d {
			return false
		}
	}
}

const modTmpl = `module {{.Module}}

go 1.26
`

const mainTmpl = `package main

import (
	"os"

	"github.com/nomad-software/qute"
)

func main() {
	os.Exit(qute.Main(func() {
		w := NewWindow()
		w.Show()
	}))
}
`

const windowTmpl = `package main

import (
	"fmt"

	"github.com/nomad-software/qute/qtwidgets"
)

// Window is the main window. Its widgets are exported to the tests.
type Window struct {
	*qtwidgets.QWidget
	Label  *qtwidgets.QLabel
	Button *qtwidgets.QPushButton
	clicks int
}

// NewWindow builds the window. Children are parented through the layout, so
// deleting the window frees them all.
func NewWindow() *Window {
	w := &Window{QWidget: qtwidgets.NewQWidget(nil)}
	w.SetWindowTitle("{{.Title}}")
	w.Resize(320, 120)

	lay := qtwidgets.NewQVBoxLayout(w.QWidget)
	w.Label = qtwidgets.NewQLabelWithText("Not clicked yet", nil)
	w.Button = qtwidgets.NewQPushButtonWithText("Click me", nil)
	lay.AddWidget(w.Label)
	lay.AddWidget(w.Button)

	// The handler refers to the window; that is fine: the window's own Go
	// handle holds it, so a closed (hidden) window nobody refers to is still
	// freed by GC, handler and all.
	w.Button.OnClicked(func(bool) {
		w.clicks++
		w.Label.SetText(fmt.Sprintf("Clicked %d times", w.clicks))
	})
	return w
}
`

const testTmpl = `package main

import (
	"testing"

	"github.com/nomad-software/qute/qutetest"
)

func TestMain(m *testing.M) { qutetest.TestMain(m) }

func TestClickUpdatesLabel(t *testing.T) {
	qutetest.Run(t, func() {
		w := NewWindow()
		defer w.Delete()
		qutetest.Click(w.Button)
		qutetest.Click(w.Button)
		if got := w.Label.Text(); got != "Clicked 2 times" {
			t.Errorf("label = %q", got)
		}
	})
}
`

const shotTmpl = `package main

import (
	"path/filepath"
	"testing"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qutetest"
)

// TestScreenshot renders the window offscreen and saves it, so the layout can
// be looked at without a display. Run with -v to see the file path.
func TestScreenshot(t *testing.T) {
	qutetest.Run(t, func() {
		w := NewWindow()
		defer w.Delete()
		img := qute.Screenshot(w)
		if b := img.Bounds(); b.Dx() < 100 || b.Dy() < 50 {
			t.Errorf("window too small: %v", b)
			return
		}
		path := filepath.Join(t.TempDir(), "window.png")
		if err := qute.SaveScreenshot(w, path); err != nil {
			t.Error(err)
			return
		}
		t.Log("screenshot:", path)
		t.Log(qute.DumpTree(w))
	})
}
`
