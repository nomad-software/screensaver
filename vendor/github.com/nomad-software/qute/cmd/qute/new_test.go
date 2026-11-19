package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewAppFiles checks that `qute new` writes the starter app's files, that
// each is Go that parses, and that the window title comes from the directory
// name. Nothing is built.
func TestNewAppFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-app")
	if err := newApp(dir, "example.com/myapp"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Error("no go.mod written:", err)
	}
	fset := token.NewFileSet()
	for _, name := range []string{"main.go", "window.go", "window_test.go", "screenshot_test.go"} {
		if _, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "window.go")); !strings.Contains(string(b), `SetWindowTitle("My App")`) {
		t.Error("the window title is not taken from the directory name")
	}
	if err := newApp(dir, "example.com/myapp"); err == nil {
		t.Error("newApp wrote into a directory that is not empty")
	}
}
