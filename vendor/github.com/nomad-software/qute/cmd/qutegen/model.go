package main

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type Module struct {
	Name       string // QtWidgets
	Pkg        string // qtwidgets
	ImportPath string
	PkgConfig  string
	IR         *irModule
	Spec       *classSpec
	Overlays   overlayFile
	Classes    []*Class
	Enums      []*Enum
	Deps       []*Module

	imports  map[string]bool
	pkgNames map[string]string // package-level Go identifier -> what claimed it
	usedOver map[string]bool   // overlay keys that matched something
}

type ClassKind int

const (
	KindObject ClassKind = iota // QObject-derived
	KindPlain                   // polymorphic or plain non-QObject class
	KindValue                   // copyable value class held in an owned handle
)

type Class struct {
	Name      string // Go name and C symbol stem: QFontTag
	Cpp       string // C++ name: QFont::Tag
	Module    *Module
	IR        *irClass
	Opts      *classOpts
	Kind      ClassKind
	QObject   bool
	Event     bool
	Base      *Class
	Secondary []*Class
	Root      *Class

	Ctors     []*Method
	ShimCtors []*Method // constructors of an abstract class, used only by its shim
	// SubCtors are the constructors of a Go subclass (New<Ctor>With), the
	// primary one first (see subCtors)
	SubCtors  []subCtor
	Methods   []*Method
	Statics   []*Method
	Signals   []*Method
	Virtuals  []*Method // flattened, for the shim
	Protected []*Method // flattened protected non-virtuals, for the Base type
	Shim      bool
	Abstract  bool
	QBase     bool // a polymorphic plain base of a QObject class (QPaintDevice, QGraphicsItem)
	DefCtor   bool // has a public constructor callable with no arguments
	// Sliced: a bound class derived from it adds data members, so a pointer
	// or reference to it may be to a derived object (a QStyleOption that is
	// really a QStyleOptionViewItem) that a copy would slice. A QPixmap
	// (QBitmap) or QTextFormat (QTextCharFormat) loses nothing in a copy.
	Sliced  bool
	HasDtor bool
	Skipped []skipNote
	// NoFull are methods whose Full form cannot be built (a defaulted
	// argument with no Go type, such as an unconfirmed bool *ok).
	NoFull []skipNote
	// NoVirtuals are the virtuals this class declares that Go cannot override.
	NoVirtuals []skipNote
	protected  map[string]*Method // its own protected methods, built once (ownProtected)
	cNames     map[string]int
	goNames    map[string]string
}

// subCtor is one constructor of a Go subclass: the C++ constructor it
// forwards to, its Go name and the C function that builds the shim.
type subCtor struct {
	M      *Method
	GoName string
	CName  string
}

type skipNote struct {
	Sig, Why string
}

type Enum struct {
	cpp       string // Qt::AlignmentFlag, QLineEdit::EchoMode
	module    *Module
	ir        *irEnum
	goName    string
	flagsName string // Go name of the QFlags type, if any
	emitted   bool
	anon      bool
}

// goType is the Go type of the enum's values. An enum with a QFlags type
// is an alias of it, so the flags name is used for both.
func (e *Enum) goType() string {
	if e.flagsName != "" {
		return e.flagsName
	}
	return e.goName
}

type Param struct {
	Name   string // C++ name (may be empty)
	GoName string
	CName  string
	T      *conv
	Rule   string // transfer-to-cpp, transfer-this, transfer-to-go, borrowed, scoped
	Ruled  bool   // rule came from an overlay
	Out    bool   // a pointer or reference the call writes: a Go result
	InOut  bool   // a reference the call reads and writes: a Go parameter and result
	ByPtr  bool   // the C++ parameter is a pointer (pass &t), not a reference
	Pair   string // "array", "bytes" or "buffer": this parameter absorbs the next (its count)
	CppT   string // C++ type of the local used for an out or in-out parameter
	// Keep makes the receiver hold a reference to this object argument, for
	// pointers Qt stores without owning: a slot name (the reference replaces
	// the slot's previous one), "each" (one reference per distinct argument)
	// or "release" (drops a reference "each" took).
	Keep string
	// KeepGuard makes Delete() of the kept argument panic while the receiver
	// keeps it (see the overlay key kept-delete).
	KeepGuard bool
	View      bool // a virtual's non-const (or Sliced) value-class pointer or reference: passed in place, not copied
}

type Method struct {
	Class    *Class
	Owner    *Class // class that declares it (for flattened virtuals)
	IR       *irMethod
	GoName   string
	CName    string
	Params   []*Param
	Ret      *conv
	RetRule  string
	RetRuled bool
	Overlay  *overlay
	Error    bool
	Numbered bool
	Full     bool    // generated with all default args
	FullOf   *Method // for an automatic Full form: the short form it extends
	FullName string  // for a short form: the Go name of its Full form
	Emit     bool    // emits a signal
	Field    string  // "get" or "set": a public data member accessor
	PairRet  []*conv // a std::pair result, returned to Go as two results
	VIndex   int
	Sig      string
}

type Gen struct {
	root    string
	modules []*Module
	byName  map[string]*Module
	classes map[string]*Class
	enums   map[string]*Enum
	structs map[string]*structDef
	report  *Report
}

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true,
	"defer": true, "else": true, "fallthrough": true, "for": true, "func": true, "go": true, "goto": true,
	"if": true, "import": true, "interface": true, "map": true, "package": true, "range": true,
	"return": true, "select": true, "struct": true, "switch": true, "type": true, "var": true,
	// identifiers the generated code uses
	"C": true, "rt": true, "unsafe": true, "this": true, "qtcore": true, "qtgui": true, "qtwidgets": true,
	"runtime": true, "fmt": true, "self": true, "args": true, "ret": true, "string": true, "len": true,
	"qtopengl": true, "qtopenglwidgets": true, "qtsvg": true, "qtsvgwidgets": true, "qtmultimedia": true,
	"qtmultimediawidgets": true, "qtwebenginecore": true, "qtwebenginewidgets": true, "error": true, "bool": true, "int": true, "f": true, "e": true, "i": true, "v": true, "r": true, "p": true, "h": true,
	"new": true, "nil": true, "true": true, "false": true, "id": true, "mask": true, "b": true, "s": true, "a": true,
}

// Methods that never become bindings.
var globalSkip = map[string]bool{
	"tr": true, "trUtf8": true, "metaObject": true, "connect": true, "disconnect": true,
	"staticMetaObject": true, "d_func": true, "qt_metacast": true, "qt_metacall": true,
	"connectNotify": true, "disconnectNotify": true, "isSignalConnected": true, "receivers": true,
	"sender": true, "senderSignalIndex": true, "swap": true, "data_ptr": true, "detach": true,
	"isDetached": true, "devType": true, "paintEngine": true, "initPainter": true,
	"redirected": true, "sharedPainter": true, "metric": true, "nativeEvent": true,
	"inputMethodQuery": true, "setupUi": true, "dumpObjectTree": true, "dumpObjectInfo": true,
	"bindableObjectName": true, "qt_static_metacall": true, "qt_check_for_QGADGET_macro": true,
}

func exportName(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// paramNames names a method's parameters, inventing readable names for the
// ones the header leaves unnamed: setText(const QString &) gets "text",
// addWidget(QWidget *) gets "widget".
func paramNames(m *irMethod) []string {
	out := make([]string, len(m.Params))
	used := map[string]bool{}
	for i, p := range m.Params {
		n := p.Name
		if n == "" {
			if len(m.Params) == 1 && len(m.Name) > 3 && strings.HasPrefix(m.Name, "set") && unicode.IsUpper(rune(m.Name[3])) {
				n = lowerFirst(m.Name[3:])
			} else {
				for _, t := range []string{p.Type.Canonical, p.Type.Spelling} {
					base, _, _, _, _ := parseCpp(t)
					if i := strings.LastIndex(base, "::"); i >= 0 {
						base = base[i+2:]
					}
					switch {
					case strings.ContainsAny(base, "<> ") || len(base) < 2:
					case strings.HasPrefix(base, "Q") && unicode.IsUpper(rune(base[1])):
						n = lowerFirst(base[1:]) // QWidget -> widget
					case unicode.IsUpper(rune(base[0])):
						n = lowerFirst(base) // QTextCursor::MoveMode -> moveMode
					}
					if n != "" {
						break
					}
				}
			}
		}
		if n == "" || used[n] {
			n = fmt.Sprintf("arg%d", i)
		}
		used[n] = true
		out[i] = goParamName(n, i)
	}
	return out
}

func lowerFirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func goParamName(s string, i int) string {
	if s == "" {
		return fmt.Sprintf("arg%d", i)
	}
	if goKeywords[s] {
		return s + "Arg"
	}
	return s
}

// ---------------------------------------------------------------------------

func (g *Gen) build() error {
	// 1. classes
	for _, m := range g.modules {
		irc := map[string]*irClass{}
		for i := range m.IR.Classes {
			irc[m.IR.Classes[i].Name] = &m.IR.Classes[i]
		}
		names := make([]string, 0, len(m.Spec.Classes))
		for n := range m.Spec.Classes {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			ic := irc[n]
			if ic == nil {
				return fmt.Errorf("%s: class %s not found in IR", m.Name, n)
			}
			opts := m.Spec.Classes[n]
			if opts == nil {
				opts = &classOpts{}
			}
			c := &Class{Name: strings.ReplaceAll(n, "::", ""), Cpp: n, Module: m, IR: ic, Opts: opts, cNames: map[string]int{}, goNames: map[string]string{}}
			g.classes[n] = c
			m.Classes = append(m.Classes, c)
		}
	}
	// 2. hierarchy and kinds
	for _, m := range g.modules {
		for _, c := range m.Classes {
			for i, b := range c.IR.Bases {
				bc := g.classes[b]
				if bc == nil {
					if i == 0 && !strings.Contains(b, "<") {
						return fmt.Errorf("%s: primary base %s is not bound; add it to spec/classes", c.Name, b)
					}
					continue
				}
				if i == 0 {
					c.Base = bc
				} else {
					c.Secondary = append(c.Secondary, bc)
				}
			}
		}
	}
	for _, m := range g.modules {
		for _, c := range m.Classes {
			if len(c.IR.Fields) > 0 {
				for b := c.Base; b != nil; b = b.Base {
					b.Sliced = true
				}
			}
		}
	}
	for _, m := range g.modules {
		for _, c := range m.Classes {
			r := c
			for r.Base != nil {
				r = r.Base
			}
			c.Root = r
			c.QObject = r.Name == "QObject"
			for x := c; x != nil; x = x.Base {
				if x.Name == "QEvent" {
					c.Event = true
				}
			}
			switch {
			case c.Opts.Kind == "value":
				c.Kind = KindValue
			case c.QObject:
				c.Kind = KindObject
			default:
				c.Kind = KindPlain
			}
			if c.Opts.Kind == "event" {
				c.Event = true
			}
			c.Abstract = c.IR.Abstract
			for _, im := range c.IR.Methods {
				if im.Kind == "ctor" && im.Access == "public" && !im.Copy && !im.Move {
					all := true
					for _, p := range im.Params {
						all = all && p.Default
					}
					if all {
						c.DefCtor = true
					}
				}
			}
			if !hasCtor(c.IR) {
				c.DefCtor = true // implicit default constructor
			}
			c.HasDtor = c.IR.Dtor == nil || c.IR.Dtor.Access == "public"
		}
	}
	for _, m := range g.modules {
		for _, c := range m.Classes {
			if c.Kind == KindValue && c.Base != nil && c.Base.Kind == KindObject {
				return fmt.Errorf("%s: value class with object base", c.Name)
			}
			if c.Kind == KindObject {
				markQBases(c)
			}
		}
	}
	for _, m := range g.modules {
		for _, c := range m.Classes {
			g.chooseSubclass(c)
		}
	}
	// 3. enums
	for _, m := range g.modules {
		g.collectEnums(m)
	}
	// 4. members
	for _, m := range g.modules {
		for _, c := range m.Classes {
			g.buildMembers(c)
		}
	}
	for _, m := range g.modules {
		for _, c := range m.Classes {
			g.buildShim(c)
		}
	}
	for _, m := range g.modules {
		for _, c := range m.Classes {
			g.checkInline(c)
		}
	}
	// 5. unused overlays
	for _, m := range g.modules {
		for cls, entries := range m.Overlays {
			if g.classes[cls] == nil && !g.boundGoName(cls) {
				g.report.warn("%s: overlay for unbound class %s", m.Name, cls)
				continue
			}
			for k := range entries {
				if !m.usedOver[cls+"::"+k] {
					g.report.warn("%s: overlay %s::%s matched no method", m.Name, cls, k)
				}
			}
		}
	}
	return nil
}

// chooseSubclass decides whether Go may subclass c, before its members are
// built (an abstract class's constructors are only bound for its shim).
// Subclassing is on wherever it can work; buildShim turns it off again when
// the class has no constructor or a pure virtual Go cannot implement.
func (g *Gen) chooseSubclass(c *Class) {
	o := c.Opts
	o.Subclass, o.NoSubclass = false, ""
	if o.EnumsOnly || c.Kind == KindValue || c.Event {
		if o.SubclassOpt != nil && *o.SubclassOpt {
			g.report.warn("%s: subclass: true on a value, event or enums-only class", c.Name)
		}
		return // values and events are not objects Go code would subclass
	}
	if !c.QObject && !polymorphic(c.IR) {
		if o.SubclassOpt != nil && *o.SubclassOpt {
			g.report.warn("%s: subclass: true on a class without virtuals", c.Name)
		}
		return
	}
	why := ""
	switch {
	case o.SubclassOpt != nil && !*o.SubclassOpt:
		why = "turned off in the spec (subclass: false)"
	case o.NoConstruct:
		why = "only Qt creates these objects (no-construct)"
	case !c.HasDtor:
		why = "its destructor is not public"
	}
	if why != "" {
		if o.SubclassOpt != nil && *o.SubclassOpt {
			g.report.warn("%s: subclass: true, but %s", c.Name, why)
		}
		o.NoSubclass = why
		return
	}
	o.Subclass = true
}

// noSubclass turns subclassing off after buildShim found why it cannot work.
// Only a class the spec insists on warns; the others are listed in COVERAGE.md.
func (g *Gen) noSubclass(c *Class, why string) {
	if c.Opts.SubclassOpt != nil && *c.Opts.SubclassOpt {
		g.report.warn("%s: %s, so the class cannot be subclassed from Go", c.Name, why)
	}
	c.Opts.Subclass = false
	c.Opts.NoSubclass = why
	c.Virtuals = nil
	c.Protected = nil
	c.ShimCtors = nil
	c.Shim = c.Opts.NotifyDtor && !c.Opts.NoConstruct
}

// markQBases marks the plain, polymorphic classes a QObject class derives
// from. A pointer of such a class may be a QObject (a QGraphicsItem * that is
// a QGraphicsTextItem), so its wrapper looks for the QObject's handle first.
func markQBases(c *Class) {
	var walk func(x *Class)
	walk = func(x *Class) {
		for _, b := range append([]*Class{x.Base}, x.Secondary...) {
			if b == nil {
				continue
			}
			if b.Kind == KindPlain && polymorphic(b.IR) {
				b.QBase = true
			}
			walk(b)
		}
	}
	walk(c)
}

func polymorphic(c *irClass) bool {
	if c.Dtor != nil && c.Dtor.Virtual {
		return true
	}
	for _, m := range c.Methods {
		if m.Virtual {
			return true
		}
	}
	return false
}

// boundGoName reports whether a bound class has this Go name; overlays of
// nested classes use it (QCalendarYearMonthDay for QCalendar::YearMonthDay).
func (g *Gen) boundGoName(name string) bool {
	for _, c := range g.classes {
		if c.Name == name {
			return true
		}
	}
	return false
}

func (m *Module) claim(name, what string) bool {
	if prev, ok := m.pkgNames[name]; ok && prev != what {
		return false
	}
	m.pkgNames[name] = what
	return true
}

func (g *Gen) collectEnums(m *Module) {
	add := func(e *irEnum, scope string, flags []irFlags, visible bool) {
		cpp := e.Name
		if scope != "" {
			cpp = scope + "::" + e.Name
		}
		en := &Enum{cpp: cpp, module: m, ir: e}
		if strings.Contains(e.Name, "(") {
			// anonymous enum: its values become untyped constants
			en.anon = true
			en.goName = ""
			en.emitted = true
			m.Enums = append(m.Enums, en)
			return
		}
		g.enums[cpp] = en
		if !visible {
			return
		}
		prefix := strings.ReplaceAll(scope, "::", "")
		if scope == "Qt" {
			prefix = ""
		}
		en.goName = prefix + e.Name
		for _, f := range flags {
			if f.Enum == cpp {
				en.flagsName = prefix + f.Name
			}
		}
		if !m.claim(en.goName, "enum "+cpp) {
			en.goName = prefix + "Enum" + e.Name
			if !m.claim(en.goName, "enum "+cpp) {
				return
			}
		}
		if en.flagsName != "" && !m.claim(en.flagsName, "flags "+cpp) {
			en.flagsName = ""
		}
		en.emitted = true
		m.Enums = append(m.Enums, en)
	}
	for i := range m.IR.Enums {
		e := &m.IR.Enums[i]
		if e.Scope != "" {
			// Qt's namespace keeps bare names; a module namespace
			// (QAudio, QtVideo) becomes the prefix.
			add(e, e.Scope, m.IR.Flags, true)
		}
	}
	for _, c := range m.Classes {
		for i := range c.IR.Enums {
			add(&c.IR.Enums[i], c.Cpp, c.IR.Flags, true)
		}
	}
}

func (g *Gen) methodSig(c *Class, m *irMethod) string {
	var ps []string
	for _, p := range m.Params {
		s := p.Type.Spelling
		if p.Name != "" {
			s += " " + p.Name
		}
		if p.Default {
			s += " = …"
		}
		ps = append(ps, s)
	}
	name := c.Name + "::" + m.Name
	ret := ""
	if m.Kind != "ctor" {
		ret = m.Return.Spelling + " "
	}
	cst := ""
	if m.Const {
		cst = " const"
	}
	return fmt.Sprintf("%s%s(%s)%s", ret, name, strings.Join(ps, ", "), cst)
}

// paramKey identifies a method's C++ signature for override/dup detection.
func paramKey(m *irMethod) string {
	var ps []string
	for _, p := range m.Params {
		ps = append(ps, p.Type.Canonical)
	}
	c := ""
	if m.Const {
		c = "const"
	}
	return m.Name + "(" + strings.Join(ps, ",") + ")" + c
}

func (g *Gen) skip(c *Class, m *irMethod, why string) {
	c.Skipped = append(c.Skipped, skipNote{Sig: g.methodSig(c, m), Why: why})
}

func (g *Gen) buildMembers(c *Class) {
	if c.Opts.EnumsOnly {
		return
	}
	m := c.Module
	ancestorKeys := map[string]bool{}
	for b := c.Base; b != nil; b = b.Base {
		for i := range b.IR.Methods {
			im := &b.IR.Methods[i]
			if im.Access == "public" && im.Kind != "ctor" {
				ancestorKeys[paramKey(im)] = true
			}
		}
	}
	var ctors, methods, statics, signals, emits []*Method
	var ctorTwins, methodTwins, staticTwins []*Method // Full forms of methods with dropped defaults
	inherited := map[string]bool{}                    // names whose inherited binding must stay visible
	for i := range c.IR.Methods {
		im := &c.IR.Methods[i]
		if im.Access == "protected" && im.Kind == "ctor" && c.Opts.Subclass && !im.Copy && !im.Move && !privateCtor(im) {
			// a protected constructor (QTextObject(QTextDocument *)) is only
			// callable by a subclass, so it builds the shim
			if mm := g.newMethod(c, im, nil, true, false); mm != nil {
				mm.CName = c.cName("new")
				c.ShimCtors = append(c.ShimCtors, mm)
			}
			continue
		}
		if im.Access != "public" {
			continue // protected members are bound on the Base type by buildShim
		}
		if globalSkip[im.Name] || strings.HasPrefix(im.Name, "operator") || strings.HasPrefix(im.Name, "~") {
			continue
		}
		ov, used := m.Overlays.overlayFor(c.Name, im)
		for _, u := range used {
			m.usedOver[c.Name+"::"+u] = true
		}
		if im.Deprecated || im.Variadic {
			g.skip(c, im, "deprecated or variadic")
			continue
		}
		if stringSlot(im) {
			g.skip(c, im, "string-based SLOT() connection; use the On* signal methods instead")
			continue
		}
		if iteratorNames[im.Name] && returnsIterator(im) {
			g.skip(c, im, "on purpose: iterator; use the method that returns a list")
			continue
		}
		if ov.skipped() {
			g.skip(c, im, fmt.Sprintf("skipped by overlay (%v)", ov.Skip))
			continue
		}
		switch im.Kind {
		case "ctor":
			if im.Copy || im.Move {
				continue
			}
			if c.Opts.NoConstruct || !c.HasDtor && c.Kind != KindObject {
				continue
			}
			if c.Abstract {
				// abstract classes are only constructible through their shim
				if c.Opts.Subclass {
					if mm := g.newMethod(c, im, ov, true, false); mm != nil {
						mm.CName = c.cName("new")
						c.ShimCtors = append(c.ShimCtors, mm)
					}
				}
				continue
			}
			if mm := g.newMethod(c, im, ov, true, false); mm != nil {
				ctors = append(ctors, mm)
				if tw := g.fullTwin(c, im, ov, true, mm); tw != nil {
					ctorTwins = append(ctorTwins, tw)
				}
			}
		case "signal":
			if mm := g.newSignal(c, im, ov); mm != nil {
				signals = append(signals, mm)
			}
			private := false
			for _, p := range im.Params {
				private = private || strings.HasSuffix(p.Type.Canonical, "QPrivateSignal")
			}
			if !private {
				if mm := g.newMethod(c, im, ov, false, false); mm != nil {
					mm.Emit = true
					emits = append(emits, mm)
				}
			}
		default:
			if !im.Static && im.Virtual && ancestorKeys[paramKey(im)] {
				inherited[exportName(im.Name)] = true
				continue // an override of a bound base method: the base binding dispatches virtually
			}
			if !im.Static && ancestorKeys[paramKey(im)] {
				continue
			}
			if mm := g.newMethod(c, im, ov, false, false); mm != nil {
				tw := g.fullTwin(c, im, ov, false, mm)
				if im.Static {
					statics = append(statics, mm)
					if tw != nil {
						staticTwins = append(staticTwins, tw)
					}
				} else {
					methods = append(methods, mm)
					if tw != nil {
						methodTwins = append(methodTwins, tw)
					}
				}
			}
			if ov != nil && ov.Full != "" {
				if mm := g.newMethod(c, im, ov, false, true); mm != nil {
					mm.GoName = ov.Full
					if im.Static {
						statics = append(statics, mm)
					} else {
						methods = append(methods, mm)
					}
				}
			}
		}
	}
	fields := g.fieldMethods(c)
	if !hasCtor(c.IR) && !c.Abstract && !c.Opts.NoConstruct && !c.Event && c.HasDtor {
		// an implicit default constructor (plain structs such as ExtraSelection)
		im := &irMethod{Name: lastName(c.Cpp), Kind: "ctor", Access: "public"}
		ctors = append(ctors, &Method{Class: c, Owner: c, IR: im, Sig: c.Cpp + "::" + lastName(c.Cpp) + "() [implicit]"})
	}
	c.Ctors = g.nameCtors(c, ctors)
	c.Ctors = append(c.Ctors, g.nameTwins(c, ctorTwins, c.Ctors, true)...)
	var keepVisible []*Method
	for n := range inherited {
		keepVisible = append(keepVisible, &Method{GoName: n})
	}
	c.Methods = g.nameMethods(c, methods, "", keepVisible)
	c.Statics = g.nameStatics(c, statics)
	c.Statics = append(c.Statics, g.nameTwins(c, staticTwins, c.Statics, true)...)
	c.Signals = g.nameMethods(c, signals, "On", c.Methods)
	c.Methods = append(c.Methods, g.nameMethods(c, emits, "Emit", append(append([]*Method{}, c.Methods...), c.Signals...))...)
	c.Methods = append(c.Methods, g.nameTwins(c, methodTwins, append(append([]*Method{}, c.Methods...), c.Signals...), false)...)
	taken := map[string]bool{}
	for _, mm := range append(append([]*Method{}, c.Methods...), c.Signals...) {
		taken[mm.GoName] = true
	}
	for b := c.Base; b != nil; b = b.Base {
		for _, mm := range b.Methods {
			taken[mm.GoName] = true
		}
	}
	for _, f := range fields {
		name := exportName(f.IR.Name)
		if f.Field == "set" {
			name = "Set" + name
		} else if taken[name] {
			name = "Get" + name
		}
		if taken[name] || reservedMethods[name] {
			g.skip(c, f.IR, "field accessor name is taken by a method")
			continue
		}
		taken[name] = true
		f.GoName = name
		f.CName = c.cName(f.Field + "_" + f.IR.Name)
		c.Methods = append(c.Methods, f)
	}
}

// fullTwin builds the Full form of a method whose trailing default arguments
// short leaves out: the same call taking every argument (Index(row, column)
// and IndexFull(row, column, parent)). It returns nil when nothing was left
// out, when an overlay already decides (keep-defaults, full), or when a
// defaulted argument has no Go type; the short form stands either way.
func (g *Gen) fullTwin(c *Class, im *irMethod, ov *overlay, ctor bool, short *Method) *Method {
	if ov != nil && ov.NoFull != "" {
		c.NoFull = append(c.NoFull, skipNote{short.Sig, "on purpose: " + ov.NoFull})
		return nil
	}
	if ov != nil && (ov.KeepDefs || ov.Full != "" || ov.Cpp != "") || short.PairRet != nil {
		return nil
	}
	skipped, warns := len(c.Skipped), len(g.report.warnings)
	full := g.newMethod(c, im, ov, ctor, true)
	if full == nil && len(c.Skipped) > skipped {
		// the short form is bound, so the method is not skipped, but its
		// Full form is a gap COVERAGE.md lists
		c.NoFull = append(c.NoFull, skipNote{short.Sig, c.Skipped[len(c.Skipped)-1].Why})
	}
	c.Skipped, g.report.warnings = c.Skipped[:skipped], g.report.warnings[:warns]
	if full == nil || len(full.Params) == len(short.Params) {
		return nil // no defaulted argument has a Go form
	}
	full.FullOf = short
	return full
}

// nameTwins names the Full forms whose short form survived naming: the short
// form's name plus "Full". A twin whose Go signature another overload already
// has, or whose name is taken, is dropped.
func (g *Gen) nameTwins(c *Class, twins, named []*Method, pkgLevel bool) []*Method {
	m := c.Module
	live := map[*Method]bool{}
	used := map[string]bool{}
	sigs := map[string]bool{}
	for _, mm := range named {
		live[mm] = true
		used[mm.GoName] = true
		if mm.IR != nil {
			sigs[mm.IR.Name+"|"+goSigOut(mm)] = true
		}
	}
	for k := range reservedMethods {
		used[k] = true
	}
	var out []*Method
	for _, tw := range twins {
		short := tw.FullOf
		if !live[short] || short.GoName == "" || sigs[tw.IR.Name+"|"+goSigOut(tw)] {
			continue
		}
		name := short.GoName + "Full"
		if pkgLevel {
			if !m.claim(name, tw.Sig+" [full]") {
				continue
			}
		} else if used[name] {
			continue
		}
		used[name] = true
		sigs[tw.IR.Name+"|"+goSigOut(tw)] = true
		tw.GoName = name
		short.FullName = name
		if tw.IR.Kind == "ctor" {
			tw.CName = c.cName("new")
		} else {
			tw.CName = c.cName(tw.IR.Name)
		}
		out = append(out, tw)
	}
	return out
}

func (g *Gen) newMethod(c *Class, im *irMethod, ov *overlay, ctor, full bool) *Method {
	mm := &Method{Class: c, Owner: c, IR: im, Overlay: ov, Sig: g.methodSig(c, im), Full: full}
	// Drop trailing default args. Constructors keep a defaulted parent.
	n := len(im.Params)
	if !full && !(ov != nil && ov.KeepDefs) {
		keep := 0
		for i, p := range im.Params {
			if !p.Default {
				keep = i + 1
			}
			if ctor && p.Default && (p.Name == "parent" || ruleFor(ov, p.Name, i) == "transfer-this") {
				keep = i + 1
			}
		}
		n = keep
	}
	names := paramNames(im)
	for i := 0; i < n; i++ {
		p := im.Params[i]
		if c.Opts.RawPointers {
			if base, _, ptr, ref, _ := parseCpp(p.Type.Canonical); (ptr || strings.Contains(p.Type.Canonical, "*")) && !ref &&
				(prims[base].goT != "" || base == "void" || strings.HasPrefix(base, "char")) {
				mm.Params = append(mm.Params, &Param{Name: p.Name, GoName: names[i], CName: fmt.Sprintf("a%d", i),
					T: &conv{k: kRaw, cpp: p.Type.Canonical, base: base, ptr: true}, Rule: "borrowed"})
				continue
			}
		}
		if i+1 < len(im.Params) {
			if pc, kind := g.pairConv(p, im.Params[i+1]); pc != nil {
				pp := &Param{Name: p.Name, GoName: names[i], CName: fmt.Sprintf("a%d", i), T: pc, Pair: kind, Rule: "borrowed"}
				mm.Params = append(mm.Params, pp)
				i++ // the count is implied by the slice
				if i >= n {
					n = i + 1
				}
				continue
			}
		}
		if arrayWithoutCount(g, p) {
			g.skip(c, im, "on purpose: a pointer to an array with no count; Qt would read past one element")
			return nil
		}
		outRule := ruleFor(ov, p.Name, i)
		if outRule == "" {
			outRule = ruleFor(ov, names[i], i)
		}
		if oc, inout, byPtr, cppT := g.outConv(p, ctor, outRule); oc != nil {
			pp := &Param{Name: p.Name, GoName: names[i], CName: fmt.Sprintf("a%d", i), T: oc,
				Out: !inout, InOut: inout, ByPtr: byPtr, CppT: cppT, Rule: "out", Ruled: outRule != ""}
			if inout {
				pp.Rule = "in-out"
			}
			mm.Params = append(mm.Params, pp)
			continue
		} else if outRule == "out" || outRule == "in-out" {
			g.report.warn("%s: %s rule on %s, which is not a non-const pointer or reference to a supported type",
				g.methodSig(c, im), outRule, names[i])
		}
		t, err := g.resolve(p.Type, false)
		if err != nil {
			g.skip(c, im, err.Error())
			return nil
		}
		if t.k == kVoid {
			g.skip(c, im, "void parameter")
			return nil
		}
		pp := &Param{Name: p.Name, GoName: names[i], CName: fmt.Sprintf("a%d", i), T: t}
		g.applyArgRule(c, im, ov, pp, i, ctor)
		mm.Params = append(mm.Params, pp)
	}
	if ov != nil && !full {
		g.checkArgKeys(c, im, ov, names)
	}
	if !ctor {
		if base, _, ptr, ref, _ := parseCpp(im.Return.Canonical); !ptr && !ref && strings.HasPrefix(base, "std::pair<") {
			ab := splitTopLevel(base[len("std::pair<") : len(base)-1])
			if len(ab) == 2 {
				a, err1 := g.resolve(irType{Canonical: ab[0]}, true)
				b, err2 := g.resolve(irType{Canonical: ab[1]}, true)
				if err1 == nil && err2 == nil {
					mm.Ret = &conv{k: kVoid}
					mm.PairRet = []*conv{a, b}
					return mm
				}
			}
		}
		if base, cnst, ptr, ref, _ := parseCpp(im.Return.Canonical); ref && !cnst && !ptr && base == c.Cpp && !c.QObject {
			mm.Ret = &conv{k: kSelf, cpp: im.Return.Canonical, base: base, cls: c}
			if ov != nil && ov.Return != "" {
				g.report.warn("%s: return rule on a result that is the receiver", mm.Sig)
			}
			return mm
		}
		t, err := g.resolve(im.Return, true)
		if err != nil {
			g.skip(c, im, "return: "+err.Error())
			return nil
		}
		mm.Ret = t
		if ov != nil && ov.Error {
			if t.k != kPrim || t.prim.goT != "bool" {
				g.report.warn("%s: error flag on non-bool return", mm.Sig)
			} else {
				mm.Error = true
			}
		}
		if t.k == kObj || (t.k == kList && t.elem.k == kObj) {
			mm.RetRule = "borrowed"
			if ov != nil && ov.Return != "" {
				mm.RetRule = ov.Return
				mm.RetRuled = true
				switch ov.Return {
				case "transfer-to-go", "parented", "borrowed":
				default:
					g.report.warn("%s: unknown return rule %q", mm.Sig, ov.Return)
				}
			}
		} else if ov != nil && ov.Return != "" {
			g.report.warn("%s: return rule on a result that is not an object pointer", mm.Sig)
		}
	}
	return mm
}

func ruleFor(ov *overlay, name string, idx int) string {
	if ov == nil {
		return ""
	}
	if r, ok := ov.Args[name]; ok && name != "" {
		return r
	}
	if r, ok := ov.Args[fmt.Sprint(idx)]; ok {
		return r
	}
	return ""
}

func (g *Gen) applyArgRule(c *Class, im *irMethod, ov *overlay, p *Param, i int, ctor bool) {
	if p.T.k != kObj && !(p.T.k == kList && p.T.elem.k == kObj) {
		return
	}
	r := ruleFor(ov, p.Name, i)
	if r == "" {
		r = ruleFor(ov, p.GoName, i)
	}
	if r != "" {
		p.Rule, p.Ruled = r, true
		switch {
		case r == "transfer-to-cpp", r == "transfer-this", r == "transfer-to-go", r == "borrowed", r == "scoped":
		case r == "keep" || strings.HasPrefix(r, "keep:") || r == "keep-each" || r == "release":
			// Qt stores the pointer but does not own it: still borrowed, but
			// the receiver (or the new object) holds a Go reference to it
			p.Rule = "borrowed"
			switch {
			case r == "keep":
				owner := im.Name
				if ctor {
					owner = c.Name
				}
				p.Keep = owner + "." + p.GoName
			case r == "keep-each":
				p.Keep = "each"
			case r == "release":
				p.Keep = "release"
			default:
				p.Keep = strings.TrimPrefix(r, "keep:")
			}
			if im.Static {
				g.report.warn("%s: %s rule on a static function, which has no receiver to hold the reference", g.methodSig(c, im), r)
				p.Keep = ""
			}
			if p.T.k == kObj {
				p.KeepGuard = !c.QObject || !p.T.cls.QObject
			}
			switch ov.KeptDelete {
			case "allow":
				p.KeepGuard = false
			case "guard":
				p.KeepGuard = true
			case "":
			default:
				g.report.warn("%s: unknown kept-delete value %q (allow or guard)", g.methodSig(c, im), ov.KeptDelete)
			}
			if p.T.k != kObj {
				g.report.warn("%s: %s rule on a list argument is not supported", g.methodSig(c, im), r)
				p.Keep = ""
			}
		default:
			g.report.warn("%s: unknown rule %q", g.methodSig(c, im), r)
		}
		return
	}
	switch {
	case ctor && p.Name == "parent":
		p.Rule = "transfer-this"
	case !ctor && im.Name == "setParent" && p.Name == "parent":
		p.Rule = "transfer-this"
	default:
		p.Rule = "borrowed"
	}
}

func (g *Gen) newSignal(c *Class, im *irMethod, ov *overlay) *Method {
	mm := &Method{Class: c, Owner: c, IR: im, Overlay: ov, Sig: g.methodSig(c, im)}
	for i, p := range im.Params {
		if strings.HasSuffix(p.Type.Canonical, "QPrivateSignal") {
			continue
		}
		t, err := g.resolve(p.Type, true)
		if err != nil {
			g.skip(c, im, "signal: "+err.Error())
			return nil
		}
		pp := &Param{Name: p.Name, GoName: paramNames(im)[i], CName: fmt.Sprintf("a%d", i), T: t, Rule: "borrowed"}
		if r := ruleFor(ov, p.Name, i); r != "" {
			pp.Rule, pp.Ruled = r, true
		} else if t.k == kObj && (t.cls.Event || t.cls.Name == "QPainter") {
			pp.Rule = "scoped"
		}
		mm.Params = append(mm.Params, pp)
	}
	return mm
}

// goSig is the Go-visible parameter list used to spot exact duplicates.
func goSig(m *Method) string {
	var ps []string
	for _, p := range m.Params {
		if p.Out {
			continue
		}
		ps = append(ps, goKey(p.T))
	}
	return strings.Join(ps, ",")
}

// goSigOut is goSig with the out-parameters, which are Go results: a Full
// form that only adds one (toInt's bool *ok) differs from its short form.
func goSigOut(m *Method) string {
	var outs []string
	for _, p := range m.Params {
		if p.Out {
			outs = append(outs, goKey(p.T))
		}
	}
	return goSig(m) + "|" + strings.Join(outs, ",")
}

var reservedMethods = map[string]bool{
	"Ptr": true, "Alive": true, "Delete": true, "Keep": true, "Unkeep": true, "Ownership": true,
	"Super": true, "String": true,
}

func (g *Gen) nameMethods(c *Class, ms []*Method, prefix string, taken []*Method) []*Method {
	used := map[string]bool{}
	for _, t := range taken {
		used[t.GoName] = true
	}
	for k := range reservedMethods {
		used[k] = true
	}
	for b := c; b != nil; b = b.Base {
		used["As"+b.Name] = true
	}
	for _, s := range c.Secondary {
		used["As"+s.Name] = true
	}
	seen := map[string]bool{}
	// named by overlay first
	for _, m := range ms {
		if m.Overlay != nil && m.Overlay.Name != "" && !m.Full {
			m.GoName = prefix + m.Overlay.Name // signals: name: X gives OnX and EmitX
			if used[m.GoName] {
				g.report.warn("%s: overlay name %s already used", m.Sig, m.GoName)
			}
			used[m.GoName] = true
		} else if m.Full {
			used[m.GoName] = true
		}
	}
	var kept []*Method
	// hand-named overloads claim their Go signature first
	for _, m := range ms {
		if m.Overlay != nil && m.Overlay.Name != "" && !m.Full {
			key := m.IR.Name + "|" + goSig(m)
			if m.Emit {
				key = "emit|" + key
			}
			seen[key] = true
		}
	}
	for _, m := range ms {
		key := m.IR.Name + "|" + goSig(m)
		if m.Overlay != nil && m.Overlay.Name != "" && !m.Full {
			kept = append(kept, m)
			continue
		}
		if m.Emit {
			key = "emit|" + key
		}
		if !m.Full && seen[key] {
			g.skip(c, m.IR, "on purpose: same Go signature as another overload (often a QStringView twin)")
			continue
		}
		seen[key] = true
		kept = append(kept, m)
	}
	var kept2 []*Method
	for _, m := range kept {
		if m.GoName == "" && prefix == "" && isNumberTwin(m, kept) {
			g.skip(c, m.IR, "on purpose: plain-number twin of the struct overload")
			continue
		}
		kept2 = append(kept2, m)
	}
	g.nameOverloads(kept2, func(m *Method) string { return prefix + exportName(m.IR.Name) },
		func(n string) bool { return used[n] }, func(n string) { used[n] = true })
	for _, m := range kept2 {
		m.CName = c.cName(m.IR.Name)
	}
	return kept2
}

func (c *Class) cName(n string) string {
	k := c.cNames[n]
	c.cNames[n] = k + 1
	if k == 0 {
		return fmt.Sprintf("qute_%s_%s", c.Name, n)
	}
	return fmt.Sprintf("qute_%s_%s_%d", c.Name, n, k)
}

func (g *Gen) nameStatics(c *Class, ms []*Method) []*Method {
	m := c.Module
	var out []*Method
	seen := map[string]bool{}
	for _, mm := range ms {
		key := mm.IR.Name + "|" + goSig(mm)
		if !mm.Full && seen[key] {
			g.skip(c, mm.IR, "on purpose: same Go signature as another overload (often a QStringView twin)")
			continue
		}
		seen[key] = true
		var name string
		if mm.Overlay != nil && mm.Overlay.Name != "" && !mm.Full {
			name = mm.Overlay.Name
			if !m.claim(name, mm.Sig) {
				g.report.warn("%s: overlay name %s already used", mm.Sig, name)
			}
		} else if mm.Full {
			name = c.Name + mm.GoName
			m.claim(name, mm.Sig)
		}
		mm.GoName = name
		out = append(out, mm)
	}
	var rest []*Method
	for _, mm := range out {
		if mm.GoName == "" && !isNumberTwin(mm, out) {
			rest = append(rest, mm)
		} else if mm.GoName == "" {
			g.skip(c, mm.IR, "on purpose: plain-number twin of the struct overload")
			mm.GoName = "-"
		}
	}
	g.nameOverloads(rest, func(mm *Method) string { return c.Name + exportName(mm.IR.Name) },
		func(n string) bool { _, ok := m.pkgNames[n]; return ok }, func(n string) { m.claim(n, "static") })
	var final []*Method
	for _, mm := range out {
		if mm.GoName == "-" {
			continue
		}
		mm.CName = c.cName(mm.IR.Name)
		final = append(final, mm)
	}
	return final
}

func (g *Gen) nameCtors(c *Class, ms []*Method) []*Method {
	m := c.Module
	var out []*Method
	seen := map[string]bool{}
	primary := "New" + c.Name
	firstWith := map[string]*Method{}
	// A constructor taking only the parent is the primary one; a zero-argument
	// constructor next to it is redundant (pass nil).
	hasParentOnly := false
	for _, mm := range ms {
		if len(mm.Params) == 1 && mm.Params[0].Rule == "transfer-this" && (mm.Overlay == nil || mm.Overlay.Name == "") {
			hasParentOnly = true
		}
	}
	if hasParentOnly {
		var keep []*Method
		for _, mm := range ms {
			if len(mm.Params) == 0 && (mm.Overlay == nil || mm.Overlay.Name == "") {
				g.skip(c, mm.IR, "redundant: call New"+c.Name+"(nil)")
				continue
			}
			keep = append(keep, mm)
		}
		sort.SliceStable(keep, func(i, j int) bool {
			pi := len(keep[i].Params) == 1 && keep[i].Params[0].Rule == "transfer-this"
			pj := len(keep[j].Params) == 1 && keep[j].Params[0].Rule == "transfer-this"
			return pi && !pj
		})
		ms = keep
	}
	for _, mm := range ms {
		key := goSig(mm)
		if (mm.Overlay == nil || mm.Overlay.Name == "") && isNumberTwin(mm, ms) {
			g.skip(c, mm.IR, "on purpose: plain-number twin of the struct overload")
			continue
		}
		if seen[key] {
			g.skip(c, mm.IR, "on purpose: same Go signature as another overload (often a QStringView twin)")
			continue
		}
		seen[key] = true
		var name string
		switch {
		case mm.Overlay != nil && mm.Overlay.Name != "":
			name = mm.Overlay.Name
			if !m.claim(name, mm.Sig) {
				g.report.warn("%s: overlay name %s already used", mm.Sig, name)
			}
		default:
			onlyParent := len(mm.Params) == 0 || (len(mm.Params) == 1 && mm.Params[0].Rule == "transfer-this")
			if onlyParent && m.claim(primary, mm.Sig) {
				name = primary
				break
			}
			base := primary
			if len(mm.Params) > 0 {
				base = primary + "With" + word(mm.Params[0])
				// a number or string names the constructor by its parameter
				// (NewQColorWithName), except a one-argument conversion
				// (QVariant(int i)) or a name the header leaves cryptic:
				// those are named by type (NewQVariantWithInt)
				if p0 := mm.Params[0]; p0.Name != "" && (p0.T.k == kString || p0.T.k == kPrim) &&
					!strings.HasSuffix(p0.GoName, "Arg") && (len(goParams(mm)) > 1 || wordName(p0.GoName)) {
					base = primary + "With" + exportName(p0.GoName)
				}
			}
			name = base
			if o, ok := firstWith[base]; ok {
				name = ""
				for n := 1; n <= 3 && name == ""; n++ {
					if sfx := strings.TrimPrefix(suffixFor(mm, o, n), "With"); sfx != "" {
						if _, taken := m.pkgNames[base+sfx]; !taken {
							name = base + sfx
						}
					}
				}
			} else {
				firstWith[base] = mm
			}
			if name == "" || !m.claim(name, mm.Sig) {
				for n := 2; ; n++ {
					name = fmt.Sprintf("%s%d", base, n)
					if m.claim(name, mm.Sig) {
						break
					}
				}
				mm.Numbered = true
			}
		}
		mm.GoName = name
		mm.CName = c.cName("new")
		out = append(out, mm)
	}
	return out
}

// ---------------------------------------------------------------------------
// Shims: Go-overridable virtuals and destructor notices.

func (g *Gen) buildShim(c *Class) {
	if c.Opts.EnumsOnly {
		return
	}
	// An event Go builds is built as its shim, whose destructor reports its
	// death: Qt deletes a posted event once it is delivered.
	c.Shim = (c.Opts.Subclass || c.Opts.NotifyDtor || c.Event && len(c.Ctors) > 0) && !c.Opts.NoConstruct
	if !c.Shim {
		return
	}
	if !c.Opts.Subclass {
		return
	}
	skipV := map[string]bool{}
	for _, s := range c.Opts.SkipVirtuals {
		skipV[s] = true
	}
	seen := map[string]bool{}
	var chain []*Class
	var walk func(x *Class)
	walk = func(x *Class) {
		chain = append(chain, x)
		if x.Base != nil {
			walk(x.Base)
		}
		for _, s := range x.Secondary {
			walk(s)
		}
	}
	walk(c)
	// Names declared in more-derived classes hide base overloads in C++.
	hidden := map[string]*Class{}
	pureMissing := "" // the first pure virtual Go cannot implement
	for _, x := range chain {
		for i := range x.IR.Methods {
			im := &x.IR.Methods[i]
			if im.Kind == "ctor" || im.Kind == "signal" {
				continue
			}
			key := paramKey(im)
			if im.Access == "private" {
				if im.Virtual {
					seen[key] = true // a private override: the base virtual is not ours to override
				}
				continue
			}
			if !im.Virtual {
				if im.Access == "protected" && !im.Static {
					if !seen["p:"+key] && !globalSkip[im.Name] && !strings.HasPrefix(im.Name, "operator") && !im.Deprecated {
						seen["p:"+key] = true
						if h, ok := hidden[im.Name]; ok && h != x {
							continue
						}
						if mm := g.newProtected(c, x, im); mm != nil {
							c.Protected = append(c.Protected, mm)
						}
					}
				}
				if _, ok := hidden[im.Name]; !ok {
					hidden[im.Name] = x
				}
				continue
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			if _, ok := hidden[im.Name]; !ok {
				hidden[im.Name] = x
			}
			if im.Final || globalSkip[im.Name] || strings.HasPrefix(im.Name, "operator") || skipV[im.Name] || im.Deprecated {
				if im.Pure && pureMissing == "" {
					pureMissing = x.Cpp + "::" + im.Name
				}
				continue
			}
			mm, why := g.newVirtual(c, x, im)
			if mm == nil {
				g.noVirtual(x, im, why)
				if im.Pure && pureMissing == "" {
					pureMissing = x.Cpp + "::" + im.Name
				}
				continue
			}
			mm.VIndex = len(c.Virtuals)
			c.Virtuals = append(c.Virtuals, mm)
		}
	}
	if pureMissing != "" {
		g.noSubclass(c, "Go cannot implement its pure virtual "+pureMissing)
		return
	}
	if primaryCtor(c) == nil {
		g.noSubclass(c, "it has no public constructor")
		return
	}
	// Name the Go override methods. A public virtual keeps the name of its
	// binding, so an overload is overridden under the name it is called by
	// (QLayout.IndexOfItem); the others take the C++ name.
	bound := map[string]string{}
	for _, x := range chain {
		for _, m := range x.Methods {
			if !m.Full && !m.Emit {
				if _, ok := bound[paramKey(m.IR)]; !ok {
					bound[paramKey(m.IR)] = m.GoName
				}
			}
		}
	}
	used := map[string]int{}
	for _, v := range c.Virtuals {
		if n, ok := bound[paramKey(v.IR)]; ok {
			v.GoName = n
			used[n]++
		}
	}
	for _, v := range c.Virtuals {
		v.CName = fmt.Sprintf("qute_%s_super_%d", c.Name, v.VIndex)
		if v.GoName != "" {
			continue
		}
		n := exportName(v.IR.Name)
		used[n]++
		if used[n] > 1 {
			n = fmt.Sprintf("%s%d", n, used[n])
			v.Numbered = true
		}
		v.GoName = n
	}
	for _, v := range c.Protected {
		v.CName = c.cName("prot_" + v.IR.Name)
	}
	c.SubCtors = g.subCtors(c)
	if c.Opts.NotifyDtor {
		g.report.warn("%s: notify-dtor is redundant: its subclass shim reports its death", c.Name)
	}
}

// checkInline warns where an inline value class (spec option inline) is
// used in a way its byte-copy protocol does not cover: where a pointer to Go
// memory would have to be stored in C memory.
func (g *Gen) checkInline(c *Class) {
	if c.Opts.Inline && (c.Kind != KindValue || c.Opts.Memory != nil) {
		g.report.warn("%s: inline needs kind: value and no memory option", c.Name)
	}
	var bad func(t *conv) bool
	bad = func(t *conv) bool {
		if t == nil {
			return false
		}
		switch t.k {
		case kMap:
			return inlineVal(t.key) || inlineVal(t.elem) || bad(t.key) || bad(t.elem)
		case kFunc:
			return inlineVal(t.elem) || bad(t.elem)
		case kOpt:
			return false // a result: Go copies it
		case kList:
			return t.elem != nil && t.elem.k == kValue && t.elem.cls.Opts.Inline && t.elem.ptr
		}
		return false
	}
	all := append(append(append(append(append([]*Method{}, c.Ctors...), c.Methods...), c.Statics...), c.Signals...), c.Virtuals...)
	for _, mm := range append(all, c.Protected...) {
		for _, p := range mm.Params {
			if bad(p.T) || (p.Out || p.InOut) && inlineVal(p.T) {
				g.report.warn("%s: an inline value as %s, which the inline protocol does not cover", mm.Sig, p.GoName)
			}
		}
		if mm.Ret != nil && bad(mm.Ret) {
			g.report.warn("%s: an inline value in its result, which the inline protocol does not cover", mm.Sig)
		}
	}
}

// subCtors lists the constructors a Go subclass is built with: the primary
// one as New<Class>With, and every other public constructor as its own name
// plus With (NewQWebEnginePageWithProfileWith), so a subclass can be built in
// any way the class can. Full forms take the short form's subclass
// constructor; a constructor an overlay builds with cpp is left out.
func (g *Gen) subCtors(c *Class) []subCtor {
	pc := primaryCtor(c)
	if pc == nil {
		return nil
	}
	out := []subCtor{{M: pc, GoName: "New" + c.Name + "With", CName: "qute_" + c.Name + "_new_shim"}}
	for _, mm := range c.Ctors {
		if mm == pc || mm.FullOf != nil || mm.IR == nil || len(mm.IR.Params) == 0 || mm.Overlay != nil && mm.Overlay.Cpp != "" {
			continue
		}
		name := mm.GoName + "With"
		if !c.Module.claim(name, mm.Sig+" [subclass]") {
			g.report.warn("%s: subclass constructor name %s is taken", c.Name, name)
			continue
		}
		out = append(out, subCtor{M: mm, GoName: name, CName: fmt.Sprintf("qute_%s_new_shim_%d", c.Name, len(out))})
	}
	return out
}

// noVirtual records, once per declaring class, a virtual Go cannot override.
func (g *Gen) noVirtual(owner *Class, im *irMethod, why string) {
	sig := g.methodSig(owner, im)
	for _, n := range owner.NoVirtuals {
		if n.Sig == sig {
			return
		}
	}
	owner.NoVirtuals = append(owner.NoVirtuals, skipNote{Sig: sig, Why: why})
}

func (g *Gen) newVirtual(c, owner *Class, im *irMethod) (*Method, string) {
	mm := &Method{Class: c, Owner: owner, IR: im, Sig: g.methodSig(owner, im)}
	// The overlays of the declaring class and its bases rule the virtual: a
	// derived class's override has no binding of its own (the base's
	// dispatches), so its rules sit on the base. A virtual that is only
	// protected has no public binding to claim its overlay entry, so every
	// entry is claimed here.
	var ovs []*overlay
	var ovDecl []*irMethod // each overlay's class's own declaration, for its parameter names
	for x := owner; x != nil; x = x.Base {
		ov, used := x.Module.Overlays.overlayFor(x.Name, im)
		for _, u := range used {
			x.Module.usedOver[x.Name+"::"+u] = true
		}
		if ov != nil {
			ovs = append(ovs, ov)
			ovDecl = append(ovDecl, declIn(x, im))
		}
	}
	names := paramNames(im)
	for i, p := range im.Params {
		// a confirmed out or in-out parameter (validate(QString &, int &)):
		// the override takes the value and returns the new one, as the
		// method's binding does. A base may name the parameter differently
		// (QAbstractSpinBox::fixup(input), QSpinBox::fixup(str)).
		outRule := ""
		for k, ov := range ovs {
			d := ovDecl[k]
			if outRule = ruleFor(ov, d.Params[i].Name, i); outRule == "" {
				outRule = ruleFor(ov, paramNames(d)[i], i)
			}
			if outRule != "" {
				break
			}
		}
		if outRule == "out" || outRule == "in-out" {
			oc, inout, byPtr, cppT := g.outConv(p, false, outRule)
			if oc == nil || oc.k == kValue {
				return nil, fmt.Sprintf("parameter %s: an override cannot fill in %s", names[i], p.Type.Canonical)
			}
			pp := &Param{Name: p.Name, GoName: names[i], CName: fmt.Sprintf("a%d", i), T: oc,
				Out: !inout, InOut: inout, ByPtr: byPtr, CppT: cppT, Rule: outRule, Ruled: true}
			mm.Params = append(mm.Params, pp)
			continue
		}
		t, err := g.resolve(p.Type, true)
		if err != nil {
			return nil, fmt.Sprintf("parameter %s: %v", names[i], err)
		}
		pp := &Param{Name: p.Name, GoName: names[i], CName: fmt.Sprintf("a%d", i), T: t, Rule: "borrowed"}
		if t.k == kObj && t.ref {
			// interceptRequest(QWebEngineUrlRequestInfo &): Qt's object on
			// its stack, so Go gets that object, valid for the call only
			tc := *t
			tc.inplace = true
			pp.T, pp.View, pp.Rule = &tc, true, "scoped"
			mm.Params = append(mm.Params, pp)
			continue
		}
		if t.k == kObj && (t.cls.Event || t.cls.Name == "QPainter" || strings.HasPrefix(t.cls.Name, "QStyleOption")) {
			pp.Rule = "scoped"
		}
		if _, cnst, ptr, ref, _ := parseCpp(p.Type.Canonical); t.k == kValue && (ptr || ref) && (!cnst || t.cls.Sliced) && !inlineVal(t) {
			// initStyleOption(QStyleOptionButton *), polish(QPalette &): the
			// override fills in Qt's object, so Go gets that object itself
			// (valid for the call only), never a copy Qt would not see.
			// drawPrimitive(const QStyleOption *): the option is often a
			// derived one (QStyleOptionViewItem, QStyleOptionHeader), so a
			// copy would slice it and Super() would hand the base part to
			// the C++ style; Go gets Qt's object itself here too.
			pp.View, pp.Rule = true, "scoped"
			if ref {
				tc := *t
				tc.inplace = true
				pp.T = &tc
			}
		}
		mm.Params = append(mm.Params, pp)
	}
	t, err := g.resolve(im.Return, true)
	if err != nil {
		return nil, fmt.Sprintf("result: %v", err)
	}
	if !virtRetSupported(t) {
		return nil, "result: a Go override cannot return " + im.Return.Canonical
	}
	mm.Ret = t
	// A result the caller owns (mimeData, clone, createPopupMenu): from a Go
	// override it goes the other way, to the C++ caller, which frees it. The
	// overlay's return rule says so for both directions.
	for _, ov := range ovs {
		if ov.Return == "" && !ov.Reviewed {
			continue
		}
		mm.Overlay = ov
		if ov.Return == "transfer-to-go" || ov.Return == "parented" {
			mm.RetRule = ov.Return
		}
		mm.RetRuled = ov.Return != ""
		break
	}
	return mm, ""
}

// privateCtor reports a constructor that takes Qt's private d-pointer
// (QTextObject(QTextObjectPrivate &, QTextDocument *)), for Qt's own subclasses.
func privateCtor(im *irMethod) bool {
	for _, p := range im.Params {
		if strings.Contains(p.Type.Canonical, "Private") {
			return true
		}
	}
	return false
}

// declIn returns x's own declaration of the method im overrides or declares
// (same parameter types), or im when x declares none.
func declIn(x *Class, im *irMethod) *irMethod {
	key := paramKey(im)
	for i := range x.IR.Methods {
		if d := &x.IR.Methods[i]; d.Kind != "ctor" && paramKey(d) == key {
			return d
		}
	}
	return im
}

// newProtected returns c's copy of a protected method its ancestor owner
// declares, or nil when it is not bound. The owner's protected methods are
// built and named once (protectedOf), so every subclass's Base calls them the same.
func (g *Gen) newProtected(c, owner *Class, im *irMethod) *Method {
	tmpl := g.ownProtected(owner)[paramKey(im)]
	if tmpl == nil {
		return nil
	}
	mm := *tmpl
	mm.Class = c
	return &mm
}

// ownProtected builds and names the protected non-virtual methods x
// declares, keyed by parameter key. They are named together, like public
// overloads (SetFormat, SetFormatWithColor), and never take the name of a
// public method x or a base binds, which they would hide on the Base type.
func (g *Gen) ownProtected(x *Class) map[string]*Method {
	if x.protected != nil {
		return x.protected
	}
	x.protected = map[string]*Method{}
	var ms []*Method
	for i := range x.IR.Methods {
		im := &x.IR.Methods[i]
		if im.Access != "protected" || im.Kind != "method" && im.Kind != "slot" || im.Virtual || im.Static ||
			globalSkip[im.Name] || strings.HasPrefix(im.Name, "operator") || im.Deprecated {
			continue
		}
		// the overlay of the declaring class rules every subclass's copy
		ov, used := x.Module.Overlays.overlayFor(x.Name, im)
		for _, u := range used {
			x.Module.usedOver[x.Name+"::"+u] = true
		}
		if ov.skipped() {
			g.skip(x, im, fmt.Sprintf("skipped by overlay (%v)", ov.Skip))
			continue
		}
		mm := g.newMethod(x, im, ov, false, false)
		if mm == nil {
			continue
		}
		ok := true
		for _, p := range mm.Params {
			ok = ok && !p.Out && !p.InOut && p.Pair == ""
		}
		if ok {
			ms = append(ms, mm)
		}
	}
	var taken []*Method
	for b := x; b != nil; b = b.Base {
		taken = append(taken, b.Methods...)
	}
	cnames := x.cNames
	x.cNames = map[string]int{} // nameMethods numbers C names; the shim names its own
	for _, mm := range g.nameMethods(x, ms, "", taken) {
		x.protected[paramKey(mm.IR)] = mm
	}
	x.cNames = cnames
	return x.protected
}

func (m *Module) dependsOn(x *Module) bool {
	for _, d := range m.Deps {
		if d == x {
			return true
		}
	}
	return false
}

// hasCtor reports whether the class declares any constructor. A declared
// copy or move constructor counts too: C++ then gives it no implicit
// default constructor (QWebEngineHistoryItem).
func hasCtor(c *irClass) bool {
	for _, m := range c.Methods {
		if m.Kind == "ctor" {
			return true
		}
	}
	return false
}

// stringSlot reports methods that take a receiver plus a SLOT() string, which
// Go code cannot produce.
func stringSlot(m *irMethod) bool {
	for _, p := range m.Params {
		if !p.Default && (p.Name == "member" || p.Name == "slot" || p.Name == "amember") && strings.Contains(p.Type.Canonical, "char *") {
			return true
		}
	}
	return false
}

// checkArgKeys warns about overlay argument keys that match no parameter, so
// a rule keyed by a name the header leaves out can't be silently ignored.
func (g *Gen) checkArgKeys(c *Class, im *irMethod, ov *overlay, names []string) {
	for k := range ov.Args {
		ok := false
		for i, p := range im.Params {
			if k == p.Name && k != "" || k == names[i] || k == fmt.Sprint(i) {
				ok = true
			}
		}
		if !ok {
			g.report.warn("%s: overlay argument %q matches no parameter (use its index)", g.methodSig(c, im), k)
		}
	}
}

var countNames = map[string]bool{"num": true, "count": true, "len": true, "length": true, "size": true, "maxlen": true,
	"maxSize": true, "n": true, "pointCount": true, "lineCount": true, "rectCount": true, "number": true}

// pairConv recognises a pointer followed by its element count, which Go
// passes as one slice: drawPoints(const QPoint *points, int pointCount),
// loadFromData(const uchar *buf, int len), read(char *data, qint64 maxlen).
func (g *Gen) pairConv(p, q irParam) (*conv, string) {
	qb, _, qptr, qref, _ := parseCpp(q.Type.Canonical)
	if qptr || qref || prims[qb].goT == "" || qb == "bool" || qb == "double" || qb == "float" {
		return nil, ""
	}
	base, cnst, ptr, ref, _ := parseCpp(p.Type.Canonical)
	if !ptr || ref {
		return nil, ""
	}
	switch {
	case cnst && (base == "unsigned char" || base == "char") && (countNames[q.Name] || q.Name == ""):
		return &conv{k: kBytes, cpp: p.Type.Canonical, base: base}, "bytes"
	case !cnst && base == "char" && (countNames[q.Name] || q.Name == ""):
		return &conv{k: kBytes, cpp: p.Type.Canonical, base: base}, "buffer"
	case cnst:
		if !(countNames[q.Name] || strings.HasSuffix(q.Name, "Count")) {
			break
		}
		if st := g.structs[base]; st != nil {
			return &conv{k: kList, cpp: p.Type.Canonical, base: "QList<" + base + ">",
				elem: &conv{k: kStruct, cpp: base, base: base, st: st}}, "array"
		}
		if cls := g.classes[base]; cls != nil && cls.Kind == KindValue {
			return &conv{k: kList, cpp: p.Type.Canonical, base: "QList<" + base + ">",
				elem: &conv{k: kValue, cpp: base, base: base, cls: cls}}, "array"
		}
	}
	return nil, ""
}

// outConv recognises parameters the call writes: int *row, bool *ok,
// ItemRole *role (out), and QString &input, int &pos (in-out), and
// QTransform &result (out, a value class). The guess is by type alone, so
// every inferred one is listed in COVERAGE.md until an overlay confirms it
// with an `out` or `in-out` rule, which also overrides the guess
// (QStyle::polish(QPalette &) reads the palette it updates).
//
// Two shapes are never out-parameters: a constructor's pointer (the object
// keeps it, so a local in the shim would dangle) and uchar * (raw memory,
// bound as unsafe.Pointer).
func (g *Gen) outConv(p irParam, ctor bool, rule string) (c *conv, inout, byPtr bool, cppT string) {
	if p.Default && rule == "" {
		return nil, false, false, ""
	}
	base, cnst, ptr, ref, rref := parseCpp(p.Type.Canonical)
	if cnst || rref || ptr == ref {
		return nil, false, false, ""
	}
	if ctor && ptr || base == "unsigned char" && rule == "" {
		return nil, false, false, ""
	}
	e, err := g.resolve(irType{Spelling: base, Canonical: base}, true)
	if err != nil {
		return nil, false, false, ""
	}
	switch rule {
	case "out", "in-out":
		switch e.k {
		case kPrim, kEnum, kFlags, kString, kStruct, kBytes:
		case kValue:
			if !e.cls.DefCtor || ptr {
				return nil, false, false, ""
			}
		default:
			return nil, false, false, ""
		}
		return e, rule == "in-out", ptr, base
	case "":
	default:
		return nil, false, false, ""
	}
	switch e.k {
	case kPrim, kEnum, kFlags:
		return e, ref, ptr, base
	case kString, kStruct:
		if ref {
			return e, e.k == kString, false, base
		}
	case kBytes:
		return e, false, ptr, base
	case kValue:
		if ref && e.cls.DefCtor {
			return e, false, false, base
		}
	}
	return nil, false, false, ""
}

// fieldMethods turns public data members (QStyleOption::rect,
// QTextEdit::ExtraSelection::cursor) into getter and setter methods.
func (g *Gen) fieldMethods(c *Class) []*Method {
	var out []*Method
	for _, f := range c.IR.Fields {
		get := &irMethod{Name: f.Name, Kind: "method", Access: "public", Return: f.Type, Const: true}
		if strings.HasPrefix(f.Name, "_") {
			// public only for layout reasons (QWebEngineCookieStore::FilterRequest::_reservedFlag)
			g.skip(c, get, "on purpose: reserved field")
			continue
		}
		t, err := g.resolve(f.Type, true)
		if err != nil {
			g.skip(c, get, "field: "+err.Error())
			continue
		}
		gm := &Method{Class: c, Owner: c, IR: get, Ret: t, Field: "get", Sig: fmt.Sprintf("%s %s::%s [field]", f.Type.Spelling, c.Cpp, f.Name)}
		if t.k == kObj {
			gm.RetRule, gm.RetRuled = "borrowed", true
		}
		out = append(out, gm)
		if _, cnst, _, _, _ := parseCpp(f.Type.Canonical); cnst && !t.ptr {
			continue
		}
		at, err := g.resolve(f.Type, false)
		if err != nil {
			continue
		}
		set := &irMethod{Name: f.Name, Kind: "method", Access: "public", Return: irType{Canonical: "void", Spelling: "void"},
			Params: []irParam{{Name: f.Name, Type: f.Type}}}
		sm := &Method{Class: c, Owner: c, IR: set, Ret: &conv{k: kVoid}, Field: "set",
			Sig:    fmt.Sprintf("void %s::%s = … [field]", c.Cpp, f.Name),
			Params: []*Param{{Name: f.Name, GoName: goParamName(lowerFirst(f.Name), 0), CName: "a0", T: at, Rule: "borrowed", Ruled: true}}}
		out = append(out, sm)
	}
	return out
}

var iteratorNames = map[string]bool{"begin": true, "end": true, "cbegin": true, "cend": true,
	"rbegin": true, "rend": true, "crbegin": true, "crend": true, "constBegin": true, "constEnd": true,
	"keyValueBegin": true, "keyValueEnd": true, "constKeyValueBegin": true, "constKeyValueEnd": true, "asKeyValueRange": true}

// goKey describes a type as Go sees it, so overloads that differ only in C++
// (QString and QStringView) count as duplicates.
func goKey(c *conv) string {
	switch c.k {
	case kString, kCStr, kPath:
		return "string"
	case kBytes:
		return "[]byte"
	case kStrList, kStrArray:
		return "[]string"
	case kPrim:
		return c.prim.goT
	case kEnum, kFlags:
		return c.enum.goType()
	case kStruct:
		return c.st.name
	case kValue, kObj, kOpt:
		if c.cls != nil {
			return c.cls.Name
		}
		if c.elem != nil && c.elem.cls != nil {
			return c.elem.cls.Name
		}
	case kList:
		return "[]" + goKey(c.elem)
	case kMap:
		return "map[" + goKey(c.key) + "]" + goKey(c.elem)
	}
	return fmt.Sprint(c.k, c.base)
}

// wordName reports whether a parameter name reads as a word worth putting in
// a Go name (NewQColorWithName), not a C++ abbreviation (ll, d) or a keyword
// that had to be renamed (string, i): those name the overload by type.
func wordName(n string) bool {
	return len(n) >= 3 && !strings.HasSuffix(n, "Arg") && !strings.HasPrefix(n, "arg")
}

// word is the suffix an overload gets for one of its parameters:
// QCalendar -> Calendar, int64 -> Int64, FormatType -> FormatType.
func word(p *Param) string {
	t := p.T
	switch t.k {
	case kStruct:
		return strings.TrimPrefix(t.st.name, "Q")
	case kValue, kObj:
		return strings.TrimPrefix(t.cls.Name, "Q")
	case kPrim:
		if n := strings.TrimSuffix(p.GoName, "Arg"); len(n) >= 3 && !strings.HasPrefix(n, "arg") {
			return exportName(n)
		}
		return exportName(t.prim.goT)
	case kEnum, kFlags:
		return exportName(t.enum.ir.Name)
	case kString:
		if n := p.GoName; wordName(n) {
			return exportName(n)
		}
		return "String"
	case kChar:
		return "Rune"
	case kDur:
		return "Duration"
	case kTime:
		return "Time"
	case kList:
		if t.elem != nil && t.elem.k != kString {
			return word(&Param{T: t.elem, GoName: p.GoName}) + "s"
		}
	case kBytes:
		return "Bytes" // the []byte twin of a string overload
	case kRaw:
		return "Ptr"
	case kFunc:
		return "Func"
	case kMap:
		return "Map"
	}
	return exportName(strings.TrimSuffix(p.GoName, "Arg"))
}

func goParams(m *Method) []*Param {
	var out []*Param
	for _, p := range m.Params {
		if !p.Out {
			out = append(out, p)
		}
	}
	return out
}

// suffixFor names an overload after what distinguishes it from the primary
// one: extra trailing parameters get "With" + their word, a parameter of a
// different type gets its word.
func suffixFor(m, primary *Method, n int) string {
	mp, pp := goParams(m), goParams(primary)
	var words []string
	replaced := false
	for i, p := range mp {
		if i >= len(pp) {
			words = append(words, word(p))
		} else if goKey(p.T) != goKey(pp[i].T) {
			replaced = true
			words = append(words, word(p))
		}
		if len(words) >= n {
			break
		}
	}
	if len(words) == 0 {
		return ""
	}
	if !replaced {
		return "With" + strings.Join(words, "")
	}
	return strings.Join(words, "")
}

var twinArity = map[string]int{"QPoint": 2, "QPointF": 2, "QSize": 2, "QSizeF": 2,
	"QRect": 4, "QRectF": 4, "QLine": 4, "QLineF": 4, "QMargins": 4}

// isNumberTwin reports whether m spells out, as plain numbers, a struct
// parameter another overload takes: setStart(x, y) next to setStart(QPointF).
func isNumberTwin(m *Method, all []*Method) bool {
	mp := goParams(m)
	for _, o := range all {
		if o == m || o.IR.Name != m.IR.Name {
			continue
		}
		op := goParams(o)
		for i, p := range op {
			n := 0
			if p.T.k == kStruct {
				n = twinArity[p.T.st.name]
			} else if p.T.k == kValue {
				n = twinArity[p.T.cls.Cpp]
			}
			if n == 0 || i+n > len(mp) || len(mp) != len(op)-1+n {
				continue
			}
			ok := true
			for j := 0; j < n; j++ {
				if mp[i+j].T.k != kPrim || mp[i+j].T.prim.goT == "bool" {
					ok = false
				}
			}
			for j := 0; j < i && ok; j++ {
				ok = goKey(mp[j].T) == goKey(op[j].T)
			}
			for j := i + 1; j < len(op) && ok; j++ {
				ok = goKey(mp[j-1+n].T) == goKey(op[j].T)
			}
			if ok {
				return true
			}
		}
	}
	return false
}

// nameOverloads gives unnamed methods their Go names: the overload with the
// fewest parameters gets the plain name, the rest a descriptive suffix, and
// only what is still ambiguous a number.
func (g *Gen) nameOverloads(ms []*Method, base func(*Method) string, used func(string) bool, claim func(string)) {
	groups := map[string][]*Method{}
	var order []string
	named := map[string]*Method{} // hand-named methods that already hold a plain name
	for _, m := range ms {
		if m.GoName != "" {
			named[m.GoName] = m
			continue
		}
		b := base(m)
		if _, ok := groups[b]; !ok {
			order = append(order, b)
		}
		groups[b] = append(groups[b], m)
	}
	for _, b := range order {
		grp := groups[b]
		sort.SliceStable(grp, func(i, j int) bool { return len(goParams(grp[i])) < len(goParams(grp[j])) })
		var primary *Method
		for _, m := range grp {
			name := ""
			if primary == nil && !used(b) {
				name, primary = b, m
			} else {
				ref := primary
				if ref == nil {
					ref = named[b]
				}
				if ref == nil {
					ref = grp[0]
				}
				for n := 1; n <= 3 && name == ""; n++ {
					if sfx := suffixFor(m, ref, n); sfx != "" && !used(b+sfx) {
						name = b + sfx
					}
				}
				if name == "" {
					for k := 2; ; k++ {
						if cand := fmt.Sprintf("%s%d", b, k); !used(cand) {
							name = cand
							break
						}
					}
					m.Numbered = true
				}
			}
			m.GoName = name
			claim(name)
		}
	}
}

func returnsIterator(m *irMethod) bool {
	r := m.Return.Canonical
	return strings.Contains(r, "iterator") || strings.Contains(r, "Iterator") || strings.HasSuffix(r, "*") && len(m.Params) == 0
}

// arrayWithoutCount spots "const QVector2D *values": a pointer to a value
// class or struct whose plural name says it is an array, with no count after
// it. Binding it as one nil-able handle would let Qt read past the element.
func arrayWithoutCount(g *Gen, p irParam) bool {
	base, _, ptr, ref, _ := parseCpp(p.Type.Canonical)
	if !ptr || ref {
		return false
	}
	if cls := g.classes[base]; cls == nil || cls.Kind != KindValue {
		if g.structs[base] == nil {
			return false
		}
	}
	n := strings.ToLower(p.Name)
	return strings.HasSuffix(n, "s") && len(n) > 3 || strings.Contains(n, "array")
}
