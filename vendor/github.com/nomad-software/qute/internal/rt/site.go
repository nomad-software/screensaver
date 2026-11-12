package rt

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

const modulePrefix = "github.com/nomad-software/qute/"

// callSite returns the first caller outside this module and the Go runtime,
// formatted as file:line. Used by debug builds to say where an object was
// created or destroyed.
func callSite() string {
	pcs := make([]uintptr, 48)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		if userFrame(f.Function, f.File) {
			return fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line)
		}
		if !more {
			return ""
		}
	}
}

// userFrame reports whether a frame is the program's own code: not the
// binding, the Go runtime, cgo or the testing package, and not the main
// function "go test" generates (_testmain.go), which is all that is left when
// the library made the object itself. A program's own main is user code: most
// small programs build their window in the function they pass to qute.Main.
func userFrame(fn, file string) bool {
	if fn == "" || libraryFrame(fn) || strings.HasPrefix(fn, "runtime.") ||
		strings.HasPrefix(fn, "_cgo") || strings.HasPrefix(fn, "testing.") {
		return false
	}
	return filepath.Base(file) != "_testmain.go"
}

// libraryPkgs are the runtime and the generated packages. Their external test
// packages (qtcore_test) are user code for this purpose.
var libraryPkgs = []string{"internal/rt", "qtcore", "qtgui", "qtwidgets", "qtopengl", "qtopenglwidgets",
	"qtsvg", "qtsvgwidgets", "qtmultimedia", "qtmultimediawidgets", "qtwebenginecore", "qtwebenginewidgets", "qutetest"}

// libraryFrame reports frames inside the binding itself (not its tests or
// examples).
func libraryFrame(fn string) bool {
	if strings.HasPrefix(fn, strings.TrimSuffix(modulePrefix, "/")+".") {
		return true // the root package: qute.Main, qute.Screenshot, ...
	}
	for _, p := range libraryPkgs {
		if strings.HasPrefix(fn, modulePrefix+p+".") {
			return true
		}
	}
	return false
}
