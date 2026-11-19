package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const modPath = "github.com/nomad-software/qute/"

// qtPkgs are the generated binding packages.
var qtPkgs = map[string]bool{
	modPath + "qtcore": true, modPath + "qtgui": true, modPath + "qtwidgets": true,
	modPath + "qtopengl": true, modPath + "qtopenglwidgets": true,
	modPath + "qtsvg": true, modPath + "qtsvgwidgets": true,
	modPath + "qtmultimedia": true, modPath + "qtmultimediawidgets": true,
	modPath + "qtwebenginecore": true, modPath + "qtwebenginewidgets": true,
}

type finding struct {
	pos  token.Position
	rule string
	msg  string
}

// vet runs the qute static checks:
//
//	owned-leak      an Owned object with signal connections, or a parentless
//	                Go subclass, is never deleted, parented or shown (its
//	                connections or Go value pin it forever)
//	delete-handler  Delete() of a QObject inside a signal or event handler (use
//	                DeleteLater)
//	gui-goroutine   Qt calls inside a goroutine (use qute.Post)
//	scoped-escape   an event or painter argument stored past its callback
func vet(args []string) int {
	all, err := vetFindings(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "qute vet:", err)
		return 2
	}
	for _, f := range all {
		fmt.Printf("%s: %s: %s\n", f.pos, f.rule, f.msg)
	}
	if len(all) > 0 {
		return 1
	}
	return 0
}

func vetFindings(args []string) ([]finding, error) {
	if len(args) == 0 {
		args = []string{"./..."}
	}
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
		packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps, Tests: true}
	pkgs, err := packages.Load(cfg, args...)
	if err != nil {
		return nil, err
	}
	var all []finding
	seen := map[string]bool{}
	for _, p := range pkgs {
		for _, e := range p.Errors {
			fmt.Fprintln(os.Stderr, e)
		}
		if qtPkgs[p.PkgPath] || strings.HasPrefix(p.PkgPath, modPath+"internal") {
			continue
		}
		v := &vetter{pkg: p}
		for _, f := range p.Syntax {
			v.file(f)
		}
		for _, f := range v.out {
			k := f.pos.String() + f.rule
			if !seen[k] {
				seen[k] = true
				all = append(all, f)
			}
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].pos.Filename != all[j].pos.Filename {
			return all[i].pos.Filename < all[j].pos.Filename
		}
		return all[i].pos.Line < all[j].pos.Line
	})
	return all, nil
}

type vetter struct {
	pkg *packages.Package
	out []finding
}

func (v *vetter) report(n ast.Node, rule, format string, a ...any) {
	v.out = append(v.out, finding{v.pkg.Fset.Position(n.Pos()), rule, fmt.Sprintf(format, a...)})
}

// qtMethod returns the selected method if it belongs to a generated Qt type.
func (v *vetter) qtMethod(call *ast.CallExpr) (*types.Func, string) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, ""
	}
	obj := v.pkg.TypesInfo.Uses[sel.Sel]
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || !qtPkgs[fn.Pkg().Path()] {
		return nil, ""
	}
	return fn, sel.Sel.Name
}

func isQtFunc(info *types.Info, call *ast.CallExpr) (*types.Func, bool) {
	var id *ast.Ident
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		id = f.Sel
	case *ast.Ident:
		id = f
	default:
		return nil, false
	}
	fn, ok := info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return nil, false
	}
	return fn, qtPkgs[fn.Pkg().Path()]
}

func (v *vetter) file(f *ast.File) {
	v.checkGoroutines(f)
	v.checkHandlers(f)
	v.checkScoped(f)
	v.checkLeaks(f)
}

// rootAnyThread are the qute and qutetest functions any goroutine may call;
// the rest of those packages (Screenshot, DumpTree, Click, Type...) drives Qt.
var rootAnyThread = map[string]bool{"Post": true, "PostSync": true, "Run": true, "Quit": true,
	"IsGUIThread": true, "CollectGarbage": true, "Freed": true, "Main": true, "TestMain": true}

// helperPkg returns the name of the qute package with hand-written helpers
// (qute or qutetest) that fn belongs to, or "".
func helperPkg(fn *types.Func) string {
	switch fn.Pkg().Path() {
	case strings.TrimSuffix(modPath, "/"):
		return "qute"
	case modPath + "qutetest":
		return "qutetest"
	}
	return ""
}

// gui-goroutine: Qt calls in `go` statements, except inside qute.Post/PostSync.
func (v *vetter) checkGoroutines(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		g, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}
		var body ast.Node = g.Call
		if lit, ok := g.Call.Fun.(*ast.FuncLit); ok {
			body = lit.Body
		}
		ast.Inspect(body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if fn, _ := isQtFunc(v.pkg.TypesInfo, call); fn != nil && helperPkg(fn) != "" {
				if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
					return true
				}
				switch {
				case fn.Name() == "Post" || fn.Name() == "PostSync" || fn.Name() == "Run":
					return false // runs on the GUI thread
				case !rootAnyThread[fn.Name()] && fn.Exported():
					v.report(call, "gui-goroutine", "%s.%s called from a goroutine; it drives Qt on the GUI thread, wrap it in qute.Post(func() { ... })", helperPkg(fn), fn.Name())
					return false
				}
			}
			if fn, ok := isQtFunc(v.pkg.TypesInfo, call); ok {
				v.report(call, "gui-goroutine", "%s called from a goroutine; Qt objects live on the GUI thread, wrap it in qute.Post(func() { ... })", fn.Name())
				return false
			}
			return true
		})
		return false
	})
}

// handlerBodies yields closures passed to On* methods and bodies of methods
// that override a virtual on a type embedding a generated *Base. An override
// is told by its name (an event handler) or by a parameter Qt hands over for
// the call only: Paint(painter, option, widget) of a graphics item or a
// delegate, DrawControl of a style. An unexported method is a helper, never
// an override, and the painter it is given may be one the program made.
func (v *vetter) handlerBodies(f *ast.File, visit func(body *ast.BlockStmt, params *ast.FieldList, what string)) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if fn, name := v.qtMethod(x); fn != nil && strings.HasPrefix(name, "On") {
				for _, a := range x.Args {
					if lit, ok := a.(*ast.FuncLit); ok {
						visit(lit.Body, lit.Type.Params, name+" handler")
					}
				}
			}
		case *ast.FuncDecl:
			if x.Recv != nil && x.Body != nil && v.embedsBase(x) &&
				(strings.HasSuffix(x.Name.Name, "Event") || x.Name.IsExported() && v.hasScopedParam(x)) {
				visit(x.Body, x.Type.Params, x.Name.Name)
			}
		}
		return true
	})
}

// hasScopedParam reports whether a method takes an argument that is valid
// only during the call (an event, a painter, a style option).
func (v *vetter) hasScopedParam(fd *ast.FuncDecl) bool {
	if fd.Type.Params == nil {
		return false
	}
	for _, fld := range fd.Type.Params.List {
		if t := v.pkg.TypesInfo.TypeOf(fld.Type); t != nil && isScopedType(t) {
			return true
		}
	}
	return false
}

// embedsBase reports whether the method's receiver is a Go subclass: its
// type embeds a generated Base, directly or through a type it embeds.
func (v *vetter) embedsBase(fd *ast.FuncDecl) bool {
	if len(fd.Recv.List) == 0 {
		return false
	}
	return embedsBaseType(v.pkg.TypesInfo.TypeOf(fd.Recv.List[0].Type), map[types.Type]bool{})
}

func embedsBaseType(t types.Type, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Embedded() {
			continue
		}
		ft := f.Type()
		if p, ok := ft.(*types.Pointer); ok {
			ft = p.Elem()
		}
		n, ok := ft.(*types.Named)
		if !ok || n.Obj().Pkg() == nil {
			continue
		}
		if qtPkgs[n.Obj().Pkg().Path()] {
			if strings.HasSuffix(n.Obj().Name(), "Base") {
				return true
			}
			continue // a generated wrapper, not a subclass
		}
		if embedsBaseType(n, seen) {
			return true
		}
	}
	return false
}

// delete-handler: Delete() of a QObject inside a handler.
func (v *vetter) checkHandlers(f *ast.File) {
	v.handlerBodies(f, func(body *ast.BlockStmt, _ *ast.FieldList, what string) {
		ast.Inspect(body, func(n ast.Node) bool {
			if lit, ok := n.(*ast.FuncLit); ok && lit.Body != body {
				return false // a nested closure may run later
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, name := v.qtMethod(call)
			if fn == nil || name != "Delete" {
				return true
			}
			if !isQObject(v.pkg.TypesInfo.TypeOf(call.Fun.(*ast.SelectorExpr).X)) {
				// rule 5 is about QObjects: a painter, an image or a state
				// guard made inside a handler is deleted there
				return true
			}
			v.report(call, "delete-handler", "Delete() inside %s; Qt may still be using the object, call DeleteLater() instead (rule 5)", what)
			return true
		})
	})
}

// isQObject reports whether t is a generated wrapper of a QObject class, or
// a type that embeds one: every such wrapper has AsQObject.
func isQObject(t types.Type) bool {
	if t == nil {
		return false
	}
	obj, _, _ := types.LookupFieldOrMethod(t, true, nil, "AsQObject")
	_, ok := obj.(*types.Func)
	return ok
}

func isScopedType(t types.Type) bool {
	s := t.String()
	if !strings.HasPrefix(s, "*"+modPath) {
		return false
	}
	return strings.HasSuffix(s, "Event") || strings.HasSuffix(s, ".QPainter") || strings.Contains(s, ".QStyleOption")
}

// scoped-escape: an event/painter parameter assigned to anything but a local,
// appended, sent on a channel, or captured by a goroutine or closure.
func (v *vetter) checkScoped(f *ast.File) {
	// qtgui.PaintOn runs its function before it returns, with a painter that
	// is Dead afterwards: the function is a callback with a scoped argument,
	// and a scoped argument of an enclosing handler may be used inside it.
	paintOn := map[*ast.FuncLit]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if fn, name := v.qtMethod(call); fn != nil && name == "PaintOn" && fn.Pkg().Path() == modPath+"qtgui" {
				for _, a := range call.Args {
					if lit, ok := a.(*ast.FuncLit); ok {
						paintOn[lit] = true
					}
				}
			}
		}
		return true
	})
	check := func(body *ast.BlockStmt, params *ast.FieldList, what string) {
		scoped := map[types.Object]bool{}
		if params == nil {
			return
		}
		for _, fld := range params.List {
			for _, nm := range fld.Names {
				if obj := v.pkg.TypesInfo.Defs[nm]; obj != nil && isScopedType(obj.Type()) {
					scoped[obj] = true
				}
			}
		}
		if len(scoped) == 0 {
			return
		}
		uses := func(e ast.Node) *ast.Ident {
			var hit *ast.Ident
			ast.Inspect(e, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && scoped[v.pkg.TypesInfo.Uses[id]] {
					hit = id
				}
				return hit == nil
			})
			return hit
		}
		bare := func(e ast.Expr) *ast.Ident {
			if id, ok := e.(*ast.Ident); ok && scoped[v.pkg.TypesInfo.Uses[id]] {
				return id
			}
			return nil
		}
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for i, r := range x.Rhs {
					id := bare(r)
					if id == nil || i >= len(x.Lhs) {
						continue
					}
					if lid, ok := x.Lhs[i].(*ast.Ident); ok {
						if obj := v.pkg.TypesInfo.ObjectOf(lid); obj != nil && obj.Parent() != nil && body.Pos() <= obj.Pos() && obj.Pos() <= body.End() {
							continue // a local variable
						}
					}
					v.report(x, "scoped-escape", "%s argument %s is stored past its callback; it is Dead once %s returns, copy the values you need (rule 6)", what, id.Name, what)
				}
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "append" {
					for _, a := range x.Args[1:] {
						if hit := bare(a); hit != nil {
							v.report(x, "scoped-escape", "%s argument %s is appended to a slice; it is Dead once %s returns (rule 6)", what, hit.Name, what)
						}
					}
				}
			case *ast.SendStmt:
				if hit := bare(x.Value); hit != nil {
					v.report(x, "scoped-escape", "%s argument %s is sent on a channel; it is Dead once %s returns (rule 6)", what, hit.Name, what)
				}
			case *ast.GoStmt:
				if hit := uses(x.Call); hit != nil {
					v.report(x, "scoped-escape", "%s argument %s is used by a goroutine; it is Dead once %s returns (rule 6)", what, hit.Name, what)
				}
				return false
			case *ast.FuncLit:
				if paintOn[x] {
					return true // runs now, inside the callback
				}
				if hit := uses(x.Body); hit != nil {
					v.report(x, "scoped-escape", "%s argument %s is captured by a closure that may run later (rule 6)", what, hit.Name)
				}
				return false
			}
			return true
		})
	}
	v.handlerBodies(f, check)
	for lit := range paintOn {
		check(lit.Body, lit.Type.Params, "PaintOn")
	}
}

// owned-leak: x := qtX.NewY(..., nil) with x.OnZ(...) and no Delete,
// SetParent, Show, or use of x as an argument/return/stored value.
func (v *vetter) checkLeaks(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			return true
		}
		type cand struct {
			id       *ast.Ident
			conns    int
			released bool
			subclass bool // a parentless New...With: its Go value pins it
		}
		cands := map[types.Object]*cand{}
		origin := map[*ast.CallExpr]types.Object{} // the constructor call of a followed Go value
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.ExprStmt:
				// NewQWidgetWith(&Canvas{}, nil) with the result dropped
				if call, ok := x.X.(*ast.CallExpr); ok {
					if sub, owned := v.newOwned(call); owned && sub && len(call.Args) > 0 {
						// the Go value stands for the object: follow it if it is
						// a variable (row.AddWidget(swatch) gives it a parent)
						if id, ok := call.Args[0].(*ast.Ident); ok {
							if obj := v.pkg.TypesInfo.Uses[id]; obj != nil {
								cands[obj] = &cand{id: id, subclass: true}
								origin[call] = obj
							}
						} else {
							v.report(call, "owned-leak", "a parentless Go subclass stays pinned while it lives (its Go value must survive), so dropping it leaks it; keep it and call Delete() when done, or give it a parent")
						}
					}
				}
			case *ast.AssignStmt:
				if x.Tok != token.DEFINE {
					return true
				}
				for i, r := range x.Rhs {
					call, ok := r.(*ast.CallExpr)
					if !ok || i >= len(x.Lhs) {
						continue
					}
					sub, owned := v.newOwned(call)
					if id, ok := x.Lhs[i].(*ast.Ident); ok && owned {
						if obj := v.pkg.TypesInfo.Defs[id]; obj != nil {
							cands[obj] = &cand{id: id, subclass: sub}
						}
					}
				}
			}
			return true
		})
		if len(cands) == 0 {
			return true
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok {
						if c := cands[v.pkg.TypesInfo.Uses[id]]; c != nil {
							switch name := sel.Sel.Name; {
							case strings.HasPrefix(name, "On"):
								c.conns++
							case name == "Delete" || name == "DeleteLater" || name == "SetParent" || name == "Keep" ||
								strings.HasPrefix(name, "Show"):
								// a shown window is taken to live as long as the
								// program; a dialog's Exec or Open returns with it
								// hidden and still pinned by its connections
								c.released = true
							}
						}
					}
				}
				for _, a := range x.Args {
					ast.Inspect(a, func(n ast.Node) bool {
						if id, ok := n.(*ast.Ident); ok {
							if obj := v.pkg.TypesInfo.Uses[id]; origin[x] == obj && obj != nil {
								return true // its own constructor call
							}
							if c := cands[v.pkg.TypesInfo.Uses[id]]; c != nil {
								c.released = true // handed to something that may own it
							}
						}
						return true
					})
				}
			case *ast.ReturnStmt:
				for _, r := range x.Results {
					if c := cands[v.objectOf(r)]; c != nil {
						c.released = true
					}
				}
			case *ast.AssignStmt:
				for _, r := range x.Rhs {
					if c := cands[v.objectOf(r)]; c != nil {
						c.released = true
					}
				}
			case *ast.CompositeLit:
				for _, e := range x.Elts {
					if kv, ok := e.(*ast.KeyValueExpr); ok {
						e = kv.Value
					}
					if c := cands[v.objectOf(e)]; c != nil {
						c.released = true
					}
				}
			}
			return true
		})
		for _, c := range cands {
			switch {
			case c.released:
			case c.subclass:
				v.report(c.id, "owned-leak", "%s is a parentless Go subclass, which stays pinned while it lives, but it is never deleted, parented or shown; call %s.Delete() when done or give it a parent", c.id.Name, c.id.Name)
			case c.conns > 0:
				v.report(c.id, "owned-leak", "%s is Owned and has signal connections, which pin it, but it is never deleted, parented or shown; call %s.Delete() when done or give it a parent", c.id.Name, c.id.Name)
			}
		}
		return true
	})
}

// objectOf returns the variable an expression hands on: x, &x, (x), or one
// of x's embedded fields, which is x seen as one of its classes (a Go
// subclass's wrapper, return s.QSplitter, d.list = l.QTreeWidget). Nil for
// anything else.
func (v *vetter) objectOf(e ast.Expr) types.Object {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.UnaryExpr:
			if x.Op != token.AND {
				return nil
			}
			e = x.X
		case *ast.SelectorExpr:
			sel := v.pkg.TypesInfo.Selections[x]
			if sel == nil || sel.Kind() != types.FieldVal {
				return nil
			}
			if f, ok := sel.Obj().(*types.Var); !ok || !f.Embedded() {
				return nil
			}
			e = x.X
		case *ast.Ident:
			return v.pkg.TypesInfo.Uses[x]
		default:
			return nil
		}
	}
}

// newOwned reports whether call is a generated constructor that makes an
// Owned object (no parent passed), and whether it is a Go subclass one
// (NewXWith). A parent is any non-nil argument for an object parameter,
// whatever its static type: NewQPushButtonWithText("ok", dialog).
func (v *vetter) newOwned(call *ast.CallExpr) (subclass, owned bool) {
	fn, isQt := isQtFunc(v.pkg.TypesInfo, call)
	if !isQt || !strings.HasPrefix(fn.Name(), "New") {
		return false, false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false, false
	}
	subclass = strings.HasSuffix(fn.Name(), "With")
	for i, a := range call.Args {
		if subclass && i == 0 || i >= sig.Params().Len() {
			continue // the Go value, not a parent
		}
		if id, ok := a.(*ast.Ident); ok && id.Name == "nil" {
			continue
		}
		pt := sig.Params().At(i).Type()
		if _, ptr := pt.(*types.Pointer); ptr || types.IsInterface(pt) {
			return subclass, false // a parent (or another object) was passed
		}
	}
	return subclass, true
}
