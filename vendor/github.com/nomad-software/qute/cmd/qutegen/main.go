// Command qutegen generates the binding packages (qtcore, qtgui, qtwidgets,
// qtopengl, qtopenglwidgets, qtsvg, qtsvgwidgets, qtmultimedia,
// qtmultimediawidgets, qtwebenginecore, qtwebenginewidgets), their ownership tests and COVERAGE.md from the
// checked-in IR (spec/ir) and the class and ownership specs (spec/classes,
// spec/overlays). It needs neither Qt nor libclang.
//
//	go run ./cmd/qutegen -root .
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	if err := run(*root); err != nil {
		fmt.Fprintln(os.Stderr, "qutegen:", err)
		os.Exit(1)
	}
}

func run(root string) error {
	g, err := load(root)
	if err != nil {
		return err
	}
	for _, m := range g.modules {
		if err := g.emitModule(m); err != nil {
			return err
		}
	}
	if err := g.writeReport(filepath.Join(root, "COVERAGE.md")); err != nil {
		return err
	}
	for _, w := range g.report.warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	n := 0
	for _, m := range g.modules {
		n += len(m.Classes)
	}
	fmt.Printf("qutegen: %d classes in %d packages\n", n, len(g.modules))
	return nil
}

// load reads the IR and specs and resolves every class, without writing
// anything.
func load(root string) (*Gen, error) {
	g := &Gen{root: root, byName: map[string]*Module{}, classes: map[string]*Class{},
		enums: map[string]*Enum{}, structs: map[string]*structDef{}, report: &Report{}}
	mods := []struct {
		name, pkg, pc string
		deps          []string
	}{
		{"QtCore", "qtcore", "Qt6Core", nil},
		{"QtGui", "qtgui", "Qt6Gui", []string{"QtCore"}},
		{"QtWidgets", "qtwidgets", "Qt6Widgets", []string{"QtCore", "QtGui"}},
		{"QtOpenGL", "qtopengl", "Qt6OpenGL", []string{"QtCore", "QtGui"}},
		{"QtOpenGLWidgets", "qtopenglwidgets", "Qt6OpenGLWidgets", []string{"QtCore", "QtGui", "QtWidgets", "QtOpenGL"}},
		{"QtSvg", "qtsvg", "Qt6Svg", []string{"QtCore", "QtGui"}},
		{"QtSvgWidgets", "qtsvgwidgets", "Qt6SvgWidgets", []string{"QtCore", "QtGui", "QtWidgets", "QtSvg"}},
		{"QtMultimedia", "qtmultimedia", "Qt6Multimedia", []string{"QtCore", "QtGui"}},
		{"QtMultimediaWidgets", "qtmultimediawidgets", "Qt6MultimediaWidgets", []string{"QtCore", "QtGui", "QtWidgets", "QtMultimedia"}},
		{"QtWebEngineCore", "qtwebenginecore", "Qt6WebEngineCore", []string{"QtCore", "QtGui"}},
		{"QtWebEngineWidgets", "qtwebenginewidgets", "Qt6WebEngineWidgets", []string{"QtCore", "QtGui", "QtWidgets", "QtWebEngineCore"}},
	}
	for _, md := range mods {
		ir, err := loadIR(filepath.Join(root, "spec", "ir", md.pkg+".json"))
		if err != nil {
			return nil, err
		}
		spec := &classSpec{}
		if err := loadYAML(filepath.Join(root, "spec", "classes", md.pkg+".yaml"), spec); err != nil {
			return nil, err
		}
		ov := overlayFile{}
		if err := loadYAML(filepath.Join(root, "spec", "overlays", md.pkg+".yaml"), &ov); err != nil {
			return nil, err
		}
		m := &Module{Name: md.name, Pkg: md.pkg, ImportPath: modulePath + "/" + md.pkg, PkgConfig: md.pc,
			IR: ir, Spec: spec, Overlays: ov, Deps: depsOf(g, md.deps),
			imports: map[string]bool{}, pkgNames: map[string]string{}, usedOver: map[string]bool{}}
		g.modules = append(g.modules, m)
		g.byName[md.name] = m
	}
	g.initStructs()
	if err := g.build(); err != nil {
		return nil, err
	}
	return g, nil
}

func depsOf(g *Gen, names []string) []*Module {
	var out []*Module
	for _, n := range names {
		out = append(out, g.byName[n])
	}
	return out
}
