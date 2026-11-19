package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Every ownership annotation ships with a generated test: create, transfer,
// delete the owner, assert the handle is Dead. Run under -asan the sanitizer
// also proves nothing was freed twice. Rules the generator cannot exercise on
// its own take a `test:` snippet in the overlay, or are listed in
// COVERAGE.md as untested.

type testCtx struct {
	g       *Gen
	m       *Module
	imports map[string]bool
}

func (t *testCtx) q(m *Module, name string) string {
	t.imports[m.ImportPath] = true
	return m.Pkg + "." + name
}

// concreteFor returns the bound classes derived from c (c first, then by depth).
func (g *Gen) concreteFor(c *Class) []*Class {
	var out []*Class
	for _, m := range g.modules {
		for _, x := range m.Classes {
			for b := x; b != nil; b = b.Base {
				if b == c {
					out = append(out, x)
					break
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i] == c {
			return true
		}
		if out[j] == c {
			return false
		}
		return depth(out[i]) < depth(out[j]) || depth(out[i]) == depth(out[j]) && out[i].Name < out[j].Name
	})
	return out
}

func depth(c *Class) int {
	n := 0
	for ; c.Base != nil; c = c.Base {
		n++
	}
	return n
}

// zeroArg returns a Go expression usable as a harmless argument of type p.
func (t *testCtx) zeroArg(p *Param) (string, bool) {
	switch p.T.k {
	case kPrim:
		if p.T.prim.goT == "bool" {
			return "false", true
		}
		switch strings.ToLower(p.Name) {
		case "index", "row", "column", "pos", "i", "position", "idx":
			return "0", true
		}
		return "1", true // zero is not always harmless (digit counts, sizes)
	case kEnum, kFlags:
		v := int64(0)
		if p.T.k == kEnum {
			// zero is often a "none" value (NoToolBarArea); prefer the first real one
			for _, x := range p.T.enum.ir.Values {
				n := x.Name
				if strings.HasPrefix(n, "No") || strings.HasPrefix(n, "Invalid") || n == "None" || strings.HasPrefix(n, "Unknown") {
					continue
				}
				v = x.Value
				break
			}
		}
		return fmt.Sprintf("%s(%d)", t.q(p.T.enum.module, p.T.enum.goType()), v), true
	case kString, kCStr:
		return `"x"`, true
	case kBytes, kStrList:
		return "nil", true
	case kChar:
		return "'x'", true
	case kStruct:
		return t.q(p.T.st.module, p.T.st.name) + "{}", true
	case kValue:
		if p.T.cls.DefCtor {
			return "nil", true
		}
	case kList:
		return "nil", true
	case kObj:
		if p.Rule == "borrowed" || p.Rule == "" {
			return "nil", true // a borrowed object argument may be absent
		}
	}
	return "", false
}

// construct returns an expression that builds an object assignable to cls
// (with parent expression for a transfer-this argument), and the concrete
// class it builds.
func (t *testCtx) construct(cls *Class, parent string) (string, *Class, bool) {
	return t.constructPref(cls, parent, false)
}

// constructPref can prefer the constructor with the most arguments, which
// gives receivers such as QTableWidget(rows, columns) a usable size.
func (t *testCtx) constructPref(cls *Class, parent string, longest bool) (string, *Class, bool) {
	for _, cc := range t.g.concreteFor(cls) {
		var ctors []*Method
		for _, c := range cc.Ctors {
			if c.FullOf == nil { // a Full form only adds defaults zeroArg would guess
				ctors = append(ctors, c)
			}
		}
		if longest {
			sort.SliceStable(ctors, func(i, j int) bool { return len(ctors[i].Params) > len(ctors[j].Params) })
		}
		for _, ctor := range ctors {
			var args []string
			ok := true
			for _, p := range ctor.Params {
				if p.Out {
					continue
				}
				if p.Rule == "transfer-this" && p.T.k == kObj {
					if parent != "nil" && !acceptsWidget(p.T.cls) {
						args = append(args, "nil") // the root widget is not a valid parent here
						continue
					}
					args = append(args, parent)
					continue
				}
				a, good := t.zeroArg(p)
				if !good {
					ok = false
					break
				}
				args = append(args, a)
			}
			if ok {
				return fmt.Sprintf("%s(%s)", t.q(cc.Module, ctor.GoName), strings.Join(args, ", ")), cc, true
			}
		}
	}
	return "", nil, false
}

// protectedOf returns the protected methods c declares, each as bound on
// the Base of a class Go can subclass (c itself when it can be, else the
// nearest subclassable derived class, preferring c's own module), for tests.
func (g *Gen) protectedOf(c *Class) []*Method {
	var host *Class
	for _, d := range g.concreteFor(c) {
		if d.Opts.Subclass && (host == nil || d.Module == c.Module && host.Module != c.Module) {
			host = d
		}
	}
	if host == nil {
		return nil
	}
	var out []*Method
	for _, mm := range host.Protected {
		if mm.Owner == c {
			out = append(out, mm)
		}
	}
	return out
}

// receiver builds the receiver of mm. A protected method is only callable on
// a Go subclass, so its receiver is one: an empty struct embedding the Base
// of the class that binds it, constructed with New<Class>With.
func (t *testCtx) receiver(c *Class, mm *Method, parent string, longest bool) (string, *Class, bool) {
	if mm.IR.Access != "protected" {
		return t.constructPref(c, parent, longest)
	}
	host := mm.Class
	pc := primaryCtor(host)
	if pc == nil {
		return "", nil, false
	}
	base := t.q(host.Module, host.Name+"Base")
	args := []string{"v"}
	for _, p := range pc.Params {
		if p.Out {
			continue
		}
		if p.Rule == "transfer-this" && p.T.k == kObj {
			if parent != "nil" && !acceptsWidget(p.T.cls) {
				args = append(args, "nil")
				continue
			}
			args = append(args, parent)
			continue
		}
		a, ok := t.zeroArg(p)
		if !ok {
			return "", nil, false
		}
		args = append(args, a)
	}
	return fmt.Sprintf("func() *struct{ %s } {\n\t\t\tv := &struct{ %s }{}\n\t\t\t%s(%s)\n\t\t\treturn v\n\t\t}()",
		base, base, t.q(host.Module, "New"+host.Name+"With"), strings.Join(args, ", ")), host, true
}

// path returns the selector from the receiver to the method's binding: the
// embedded-field chain from concrete down to declaring, or nothing for a
// protected method, which is promoted from the receiver's Base.
func (t *testCtx) path(concrete, declaring *Class, mm *Method) string {
	if mm.IR.Access == "protected" {
		return ""
	}
	return path(concrete, declaring)
}

// path returns the embedded-field selector from concrete down to declaring.
func path(concrete, declaring *Class) string {
	var parts []string
	for c := concrete; c != nil && c != declaring; c = c.Base {
		if c.Base == nil {
			return ""
		}
		parts = append(parts, c.Base.Name)
	}
	if len(parts) == 0 {
		return ""
	}
	return "." + strings.Join(parts, ".")
}

func isWidget(c *Class) bool {
	for ; c != nil; c = c.Base {
		if c.Name == "QWidget" {
			return true
		}
	}
	return false
}

func registered(c *Class) bool { return c.QObject || c.Shim && c.Kind != KindValue }

func (g *Gen) emitTests(m *Module, dir string) error {
	t := &testCtx{g: g, m: m, imports: map[string]bool{}}
	var body buf
	b := &body
	core := g.byName["QtCore"]
	owned, parented := t.q(core, "Owned"), t.q(core, "Parented")

	b.p("func TestMain(m *testing.M) { qutetest.TestMain(m) }\n")
	b.p("// TestLayout checks the runtime's assumption that every primary base sits at")
	b.p("// offset 0, so one pointer serves the whole embedding chain.")
	b.p("func TestLayout(t *testing.T) {\n\tif c := %s.CheckLayout(); c != \"\" {\n\t\tt.Fatalf(\"%%s: primary base is not at offset 0\", c)\n\t}\n}\n", m.Pkg)

	for _, c := range m.Classes {
		if c.Opts.EnumsOnly {
			continue
		}
		// constructors with a transfer-this parent
		for _, ctor := range c.Ctors {
			if ctor.FullOf != nil {
				continue // the same rules and code path as its short form, tested there
			}
			if ctor.Overlay != nil && ctor.Overlay.Test != "" {
				g.emitSnippet(t, c, ctor, b)
				continue
			}
			g.emitKeepTest(t, c, ctor, b)
			var tp *Param
			for _, p := range ctor.Params {
				if p.Rule == "transfer-this" && p.T.k == kObj {
					tp = p
				}
			}
			if tp == nil {
				continue
			}
			rootExpr, rootCls, ok := t.construct(tp.T.cls, "nil")
			if !ok {
				g.report.untested(ctor, "cannot construct a parent")
				continue
			}
			var args, argsNil []string
			good := true
			for _, p := range ctor.Params {
				if p == tp {
					args = append(args, "root")
					argsNil = append(argsNil, "nil")
					continue
				}
				a, ok := t.zeroArg(p)
				if !ok {
					good = false
					break
				}
				args, argsNil = append(args, a), append(argsNil, a)
			}
			if !good || !registered(c) || !registered(rootCls) {
				g.report.untested(ctor, "arguments or lifetime not observable")
				continue
			}
			b.p("// %s: a non-nil parent makes the object Parented, nil makes it Owned.", ctor.Sig)
			b.p("func TestOwnership_%s(t *testing.T) {\n\tqutetest.Run(t, func() {", ctor.GoName)
			b.p("\t\troot := %s", rootExpr)
			b.p("\t\tx := %s(%s)", t.q(m, ctor.GoName), strings.Join(args, ", "))
			b.p("\t\tif got := x.Ownership(); got != %s {\n\t\t\tt.Errorf(\"with parent: %%v, want Parented\", got)\n\t\t}", parented)
			b.p("\t\troot.Delete()")
			b.p("\t\tif x.Alive() {\n\t\t\tt.Error(\"still alive after its parent was deleted\")\n\t\t}")
			b.p("\t\ty := %s(%s)", t.q(m, ctor.GoName), strings.Join(argsNil, ", "))
			b.p("\t\tif got := y.Ownership(); got != %s {\n\t\t\tt.Errorf(\"without parent: %%v, want Owned\", got)\n\t\t}", owned)
			b.p("\t\ty.Delete()\n\t\tif y.Alive() {\n\t\t\tt.Error(\"alive after Delete\")\n\t\t}")
			b.p("\t})\n}\n")
		}
		for _, mm := range c.Methods {
			g.emitMethodTest(t, c, mm, b)
		}
		for _, mm := range g.protectedOf(c) {
			g.emitMethodTest(t, c, mm, b)
		}
	}
	var out buf
	out.p("%s", strings.TrimSuffix(genHeader, "\n"))
	out.p("package %s_test\n", m.Pkg)
	out.p("import (\n\t\"testing\"\n")
	out.p("\t%q", modulePath+"/qutetest")
	t.imports[m.ImportPath] = true
	var ims []string
	for k := range t.imports {
		ims = append(ims, k)
	}
	sort.Strings(ims)
	for _, k := range ims {
		out.p("\t%q", k)
	}
	out.p(")\n")
	out.WriteString(body.String())
	if err := writeGo(filepath.Join(dir, "ownership_gen_test.go"), out.Bytes()); err != nil {
		return err
	}
	var ex buf
	ex.p("%s", strings.TrimSuffix(genHeader, "\n"))
	ex.p("package %s\n", m.Pkg)
	ex.p("// CheckLayout exposes the C++ layout check to the external test package.")
	ex.p("var CheckLayout = checkLayout")
	return writeGo(filepath.Join(dir, "export_gen_test.go"), ex.Bytes())
}

func (g *Gen) emitMethodTest(t *testCtx, c *Class, mm *Method, b *buf) {
	if mm.FullOf != nil {
		return // the same rules and code path as its short form, tested there
	}
	if mm.Overlay != nil && mm.Overlay.Test != "" {
		g.emitSnippet(t, c, mm, b)
		return
	}
	g.emitGenericTest(t, c, mm, b)
	g.emitKeepTest(t, c, mm, b)
}

// emitKeepTest checks keep, keep-each and release rules: after the call the
// receiver (or the new object) holds a reference to the argument, or no
// longer does.
func (g *Gen) emitKeepTest(t *testCtx, c *Class, mm *Method, b *buf) {
	var keeps []*Param
	for _, p := range mm.Params {
		if p.Keep != "" {
			keeps = append(keeps, p)
		}
	}
	if len(keeps) == 0 {
		return
	}
	ctor := mm.IR.Kind == "ctor"
	var pre, args []string
	recvCls := c
	if !ctor {
		// the first usable constructor: the longest one may want a real
		// argument that zeroArg would pass as nil (QMovie's device)
		ex, rc, ok := t.receiver(c, mm, "nil", false)
		if !ok {
			g.report.untested(mm, "cannot construct the receiver")
			return
		}
		recvCls = rc
		pre = append(pre, "recv := "+ex)
	}
	vars := map[*Param]string{}
	for i, p := range mm.Params {
		if p.Out {
			continue
		}
		v := fmt.Sprintf("a%d", i)
		switch {
		case p.Keep != "":
			ex, _, ok := t.construct(p.T.cls, "nil")
			if !ok {
				g.report.untested(mm, "cannot construct argument "+p.GoName)
				return
			}
			pre = append(pre, fmt.Sprintf("%s := %s", v, ex))
			args = append(args, v)
			vars[p] = v
		case p.T.k == kObj && p.Rule == "transfer-this":
			args = append(args, "nil")
		default:
			a, ok := t.zeroArg(p)
			if !ok {
				g.report.untested(mm, "cannot build argument "+p.GoName)
				return
			}
			args = append(args, a)
		}
	}
	t.imports["unsafe"] = true
	t.imports[modulePath+"/internal/rt"] = true
	b.p("// %s: keep rules", mm.Sig)
	b.p("func TestKeep_%s_%s(t *testing.T) {\n\tqutetest.Run(t, func() {", c.Name, strings.TrimPrefix(mm.GoName, "New"))
	for _, l := range pre {
		b.p("\t\t%s", l)
	}
	owner := "recv"
	if mm.IR.Access == "protected" {
		owner = "recv." + recvCls.Name // the wrapper inside the Go subclass
	}
	if ctor {
		owner = "x"
		b.p("\t\tx := %s(%s)", t.q(c.Module, mm.GoName), strings.Join(args, ", "))
	} else {
		for _, p := range keeps {
			if p.Keep == "release" {
				b.p("\t\trt.KeepEachArg(unsafe.Pointer(%s), unsafe.Pointer(%s))", owner, vars[p])
			}
		}
		call := fmt.Sprintf("recv%s.%s(%s)", t.path(recvCls, c, mm), mm.GoName, strings.Join(args, ", "))
		if n := results(mm); n > 0 {
			call = strings.TrimSuffix(strings.Repeat("_, ", n), ", ") + " = " + call
		}
		b.p("\t\t%s", call)
	}
	for _, p := range keeps {
		v := vars[p]
		if p.Keep == "release" {
			b.p("\t\tif rt.KeepsArg(unsafe.Pointer(%s), unsafe.Pointer(%s)) {\n\t\t\tt.Error(\"%s: still kept after the call\")\n\t\t}", owner, v, p.GoName)
		} else {
			who := "receiver"
			if ctor {
				who = "new object"
			}
			b.p("\t\tif !rt.KeepsArg(unsafe.Pointer(%s), unsafe.Pointer(%s)) {\n\t\t\tt.Error(\"%s: not kept by the %s\")\n\t\t}", owner, v, p.GoName, who)
		}
	}
	b.p("\t\t%s.Delete()", owner)
	b.p("\t})\n}\n")
}

// results counts the Go results of a generated method.
func results(mm *Method) int {
	n := len(mm.PairRet)
	if mm.Ret != nil && mm.Ret.k != kVoid && !mm.Error {
		n++
	}
	if mm.Error {
		n++
	}
	for _, p := range mm.Params {
		if p.Out || p.InOut {
			n++
		}
	}
	return n
}

// emitSnippet writes a test from an overlay's test: field.
func (g *Gen) emitSnippet(t *testCtx, c *Class, mm *Method, b *buf) {
	if why, ok := strings.CutPrefix(strings.TrimSpace(mm.Overlay.Test), "skip"); ok {
		g.report.untested(mm, "skipped in overlay"+why)
		return
	}
	for _, md := range g.modules {
		if strings.Contains(mm.Overlay.Test, md.Pkg+".") {
			t.imports[md.ImportPath] = true
		}
	}
	if strings.Contains(mm.Overlay.Test, "rt.") {
		t.imports[modulePath+"/internal/rt"] = true
	}
	if strings.Contains(mm.Overlay.Test, "unsafe.") {
		t.imports["unsafe"] = true
	}
	b.p("// %s (test from the overlay)", mm.Sig)
	b.p("func TestOwnership_%s_%s(t *testing.T) {\n\tqutetest.Run(t, func() {", c.Name, mm.GoName)
	for _, l := range strings.Split(strings.TrimRight(mm.Overlay.Test, "\n"), "\n") {
		b.p("\t\t%s", l)
	}
	b.p("\t})\n}\n")
}

func (g *Gen) emitGenericTest(t *testCtx, c *Class, mm *Method, b *buf) {
	var transfers []*Param
	thisRule := false
	for _, p := range mm.Params {
		if !p.Ruled {
			continue
		}
		switch p.Rule {
		case "transfer-to-cpp":
			transfers = append(transfers, p)
		case "transfer-this":
			thisRule = true
		case "transfer-to-go":
			g.report.untested(mm, "transfer-to-go argument needs a test snippet")
			return
		}
	}
	retRule := ""
	if mm.RetRuled && (mm.RetRule == "transfer-to-go" || mm.RetRule == "parented") {
		retRule = mm.RetRule
	}
	if len(transfers) == 0 && !thisRule && retRule == "" {
		return
	}
	core := g.byName["QtCore"]
	// receiver: a non-widget QObject (layout, model) gets a root widget as
	// parent so deleting the root shows what the receiver owned.
	var pre []string
	rootVar := "recv"
	parentForRecv := "nil"
	if !thisRule && c.QObject && !isWidget(c) {
		parentForRecv = "root"
	}
	recvExpr, recvCls, ok := t.receiver(c, mm, parentForRecv, true)
	if !ok {
		g.report.untested(mm, "cannot construct the receiver")
		return
	}
	if strings.Contains(recvExpr, "root") {
		rx, _, _ := t.construct(g.classes["QWidget"], "nil")
		pre = append(pre, "root := "+rx)
		rootVar = "root"
	}
	if !registered(recvCls) && recvCls.Kind != KindPlain {
		g.report.untested(mm, "receiver lifetime not observable")
		return
	}
	pre = append(pre, "recv := "+recvExpr)
	var args []string
	var checks []string
	for i, p := range mm.Params {
		v := fmt.Sprintf("a%d", i)
		if p.Out {
			continue
		}
		switch {
		case p.T.k == kObj && (p.Rule == "transfer-to-cpp" || p.Rule == "transfer-this"):
			ex, ac, ok := t.construct(p.T.cls, "nil")
			if !ok {
				g.report.untested(mm, "cannot construct argument "+p.GoName)
				return
			}
			if p.Rule == "transfer-this" {
				// the receiver is the child; the argument is its new parent
				pre = append(pre, fmt.Sprintf("%s := %s", v, ex))
				args = append(args, v)
				checks = append(checks, fmt.Sprintf("if got := recv.Ownership(); got != %s {\n\t\t\tt.Errorf(\"receiver: %%v, want Parented\", got)\n\t\t}", t.q(core, "Parented")))
				rootVar = v
				continue
			}
			pre = append(pre, fmt.Sprintf("%s := %s", v, ex))
			args = append(args, v)
			checks = append(checks, fmt.Sprintf("if got := %s.Ownership(); got != %s {\n\t\t\tt.Errorf(\"%s: %%v, want Parented\", got)\n\t\t}", v, t.q(core, "Parented"), p.GoName))
			if registered(ac) {
				checks = append(checks, "DEAD:"+v)
			}
		case p.T.k == kList && p.T.elem.k == kObj && p.Rule == "transfer-to-cpp":
			ex, ac, ok := t.construct(p.T.elem.cls, "nil")
			if !ok {
				g.report.untested(mm, "cannot construct argument "+p.GoName)
				return
			}
			pre = append(pre, fmt.Sprintf("%s := %s", v, ex))
			args = append(args, fmt.Sprintf("[]*%s{%s}", t.q(p.T.elem.cls.Module, p.T.elem.cls.Name), v))
			checks = append(checks, fmt.Sprintf("if got := %s.Ownership(); got != %s {\n\t\t\tt.Errorf(\"%s: %%v, want Parented\", got)\n\t\t}", v, t.q(core, "Parented"), p.GoName))
			if registered(ac) {
				checks = append(checks, "DEAD:"+v)
			}
		case p.T.k == kObj && p.Rule != "borrowed":
			g.report.untested(mm, "has another object argument ("+p.GoName+")")
			return
		default:
			a, ok := t.zeroArg(p)
			if !ok {
				g.report.untested(mm, "cannot build argument "+p.GoName)
				return
			}
			args = append(args, a)
		}
	}
	b.p("// %s", mm.Sig)
	b.p("func TestOwnership_%s_%s(t *testing.T) {\n\tqutetest.Run(t, func() {", c.Name, mm.GoName)
	for _, l := range pre {
		b.p("\t\t%s", l)
	}
	call := fmt.Sprintf("recv%s.%s(%s)", t.path(recvCls, c, mm), mm.GoName, strings.Join(args, ", "))
	if mm.Ret != nil && mm.Ret.k != kVoid {
		if retRule != "" {
			call = "res := " + call
		} else {
			call = "_ = " + call
		}
	}
	b.p("\t\t%s", call)
	// the returned object: Parented (freed with its owner) or Owned (ours)
	if retRule != "" {
		res := "res"
		if mm.Ret.k == kList {
			b.p("\t\tif len(res) == 0 {\n\t\t\tt.Errorf(\"returned an empty list; give this rule a test: snippet\")\n\t\t\treturn\n\t\t}")
			res = "res[0]"
		} else {
			b.p("\t\tif res == nil {\n\t\t\tt.Errorf(\"returned nil; give this rule a test: snippet\")\n\t\t\treturn\n\t\t}")
		}
		want := "Parented"
		if retRule == "transfer-to-go" {
			want = "Owned"
		}
		b.p("\t\tif got := %s.Ownership(); got != %s {\n\t\t\tt.Errorf(\"result: %%v, want %s\", got)\n\t\t}", res, t.q(core, want), want)
		if retRule == "transfer-to-go" {
			b.p("\t\t%s.Delete()\n\t\tif %s.Alive() {\n\t\t\tt.Error(\"result alive after Delete\")\n\t\t}", res, res)
		} else if mm.Ret.cls.QObject || mm.Overlay != nil && mm.Overlay.Created {
			checks = append(checks, "DEAD:"+res)
		}
	}
	var dead []string
	for _, ch := range checks {
		if strings.HasPrefix(ch, "DEAD:") {
			dead = append(dead, strings.TrimPrefix(ch, "DEAD:"))
			continue
		}
		b.p("\t\t%s", ch)
	}
	b.p("\t\t%s.Delete()", rootVar)
	for _, d := range dead {
		b.p("\t\tif %s.Alive() {\n\t\t\tt.Errorf(\"%s alive after its owner was deleted\")\n\t\t}", d, strings.Trim(d, "[]0"))
	}
	if rootVar != "recv" && registered(recvCls) && !thisRule {
		b.p("\t\tif recv.Alive() {\n\t\t\trecv.Delete()\n\t\t}")
	}
	b.p("\t})\n}\n")
}

// acceptsWidget reports whether a QWidget can be passed where cls is expected.
func acceptsWidget(cls *Class) bool {
	return cls.Name == "QWidget" || cls.Name == "QObject"
}
