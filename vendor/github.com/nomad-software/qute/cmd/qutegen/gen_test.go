package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

// These tests load the checked-in IR and specs (no Qt needed) and check the
// decisions the generator makes for shapes that have gone wrong before.

var (
	loadOnce sync.Once
	loaded   *Gen
	loadErr  error
)

func gen(t *testing.T) *Gen {
	t.Helper()
	loadOnce.Do(func() { loaded, loadErr = load(filepath.Join("..", "..")) })
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	return loaded
}

func class(t *testing.T, name string) *Class {
	t.Helper()
	c := gen(t).classes[name]
	if c == nil {
		t.Fatalf("class %s is not bound", name)
	}
	return c
}

// method finds a bound constructor, method or static by its C++ signature prefix.
func method(t *testing.T, cls, sig string) *Method {
	t.Helper()
	c := class(t, cls)
	for _, mm := range append(append(append([]*Method{}, c.Ctors...), c.Methods...), c.Statics...) {
		if strings.Contains(mm.Sig, sig) {
			return mm
		}
	}
	t.Fatalf("%s: no bound method matching %q", cls, sig)
	return nil
}

func notBound(t *testing.T, cls, sig string) {
	t.Helper()
	c := class(t, cls)
	for _, mm := range append(append([]*Method{}, c.Ctors...), c.Methods...) {
		if strings.Contains(mm.Sig, sig) {
			t.Errorf("%s is bound as %s, want it skipped", mm.Sig, mm.GoName)
		}
	}
}

func TestOutConvShapes(t *testing.T) {
	g := gen(t)
	p := func(cpp string) irParam { return irParam{Name: "x", Type: irType{Spelling: cpp, Canonical: cpp}} }
	cases := []struct {
		cpp, rule string
		ctor      bool
		out       bool // recognised at all
		inout     bool
	}{
		{"int *", "", false, true, false},
		{"bool *", "", false, true, false},
		{"int &", "", false, true, true},
		{"QString &", "", false, true, true},
		{"const int *", "", false, false, false},
		{"unsigned char *", "", false, false, false}, // raw memory: unsafe.Pointer
		{"QByteArray *", "", true, false, false},     // a constructor keeps the pointer
		{"int *", "", true, false, false},
		{"QPalette &", "", false, true, false},      // inferred as out ...
		{"QPalette &", "in-out", false, true, true}, // ... unless the overlay says in-out
		{"float *", "out", false, true, false},
		{"QWidget *", "out", false, false, false}, // object pointers are never out-parameters
	}
	for _, c := range cases {
		oc, inout, _, _ := g.outConv(p(c.cpp), c.ctor, c.rule)
		if (oc != nil) != c.out || inout != c.inout {
			t.Errorf("outConv(%q, ctor=%v, rule=%q) = out %v inout %v, want %v %v", c.cpp, c.ctor, c.rule, oc != nil, inout, c.out, c.inout)
		}
	}
}

// Every out-parameter the type alone suggested must be confirmed in an
// overlay: the guess bound QMatrix4x4::copyDataTo to one float.
func TestEveryOutParameterIsConfirmed(t *testing.T) {
	g := gen(t)
	for _, m := range g.modules {
		for _, c := range m.Classes {
			for _, mm := range append(append(append([]*Method{}, c.Ctors...), c.Methods...), c.Statics...) {
				for _, p := range mm.Params {
					if (p.Out || p.InOut) && !p.Ruled {
						t.Errorf("%s: %s is an inferred %s; confirm it with an out or in-out rule", mm.Sig, p.GoName, p.Rule)
					}
				}
			}
		}
	}
}

func TestFixedBindings(t *testing.T) {
	notBound(t, "QMatrix4x4", "copyDataTo")
	notBound(t, "QDataStream", "QDataStream(QByteArray *")
	notBound(t, "QImage", "QImage(uchar *")

	if p := method(t, "QFileDevice", "unmap").Params[0]; p.Out || p.T.k != kRaw {
		t.Errorf("QFileDevice::unmap address: out=%v kind=%v, want an unsafe.Pointer input", p.Out, p.T.k)
	}
	if p := method(t, "QStyle", "polish(QPalette &").Params[0]; !p.InOut {
		t.Error("QStyle::polish(QPalette &) palette is not in-out")
	}
	if p := method(t, "QClipboard", "text(QString & subtype").Params[0]; !p.InOut {
		t.Error("QClipboard::text subtype is not in-out")
	}
	if mm := method(t, "QDataStream", "QDataStream(const QByteArray &"); mm.GoName != "NewQDataStreamWithBytes" {
		t.Errorf("read-only byte stream is named %s, want NewQDataStreamWithBytes", mm.GoName)
	}
}

func TestKeepRules(t *testing.T) {
	cases := []struct{ cls, sig, keep string }{
		{"QAbstractItemView", "setModel(", "setModel.model"},
		{"QDataStream", "QDataStream(QIODevice *", "device"},
		{"QDataStream", "setDevice(", "device"},
		{"QGraphicsView", "QGraphicsView(QGraphicsScene *", "scene"},
		{"QWidget", "addAction(QAction * action)", "each"},
		{"QWidget", "removeAction(", "release"},
	}
	for _, c := range cases {
		mm := method(t, c.cls, c.sig)
		var got string
		for _, p := range mm.Params {
			if p.T.k == kObj && p.Keep != "" {
				got = p.Keep
			}
		}
		if got != c.keep {
			t.Errorf("%s: keep %q, want %q", mm.Sig, got, c.keep)
		}
		if mm.Params[0].Rule != "borrowed" {
			t.Errorf("%s: rule %q, want borrowed (keep does not transfer ownership)", mm.Sig, mm.Params[0].Rule)
		}
	}
}

func TestKeepRuleWarnings(t *testing.T) {
	g := &Gen{report: &Report{}}
	c := &Class{Name: "QX", Cpp: "QX"}
	obj := &conv{k: kObj, cls: c}
	im := &irMethod{Name: "setThing", Static: true, Params: []irParam{{Name: "thing"}}}
	p := &Param{Name: "thing", GoName: "thing", T: obj}
	g.applyArgRule(c, im, &overlay{Args: map[string]string{"thing": "keep"}}, p, 0, false)
	if p.Keep != "" || len(g.report.warnings) != 1 {
		t.Errorf("keep on a static: Keep=%q warnings=%v, want none kept and one warning", p.Keep, g.report.warnings)
	}
	im.Static = false
	p = &Param{Name: "thing", GoName: "thing", T: obj}
	g.applyArgRule(c, im, &overlay{Args: map[string]string{"thing": "keep-forever"}}, p, 0, false)
	if len(g.report.warnings) != 2 {
		t.Errorf("unknown rule did not warn: %v", g.report.warnings)
	}
	p = &Param{Name: "thing", GoName: "thing", T: obj}
	g.applyArgRule(c, im, &overlay{Args: map[string]string{"thing": "keep:slot"}}, p, 0, false)
	if p.Keep != "slot" || p.Rule != "borrowed" || !p.Ruled {
		t.Errorf("keep:slot gave Keep=%q Rule=%q Ruled=%v", p.Keep, p.Rule, p.Ruled)
	}
}

// A virtual's non-const value-class pointer is Qt's own object: the override
// must get it in place (scoped), not a copy Qt never reads back.
func TestVirtualValuePointerInPlace(t *testing.T) {
	c := class(t, "QPushButton")
	var v *Method
	for _, x := range c.Virtuals {
		if x.IR.Name == "initStyleOption" {
			v = x
		}
	}
	if v == nil {
		t.Fatal("QPushButton::initStyleOption is not overridable")
	}
	if p := v.Params[0]; !p.View || p.Rule != "scoped" {
		t.Errorf("option: View=%v Rule=%q, want an in-place scoped argument", p.View, p.Rule)
	}
	var b buf
	gen(t).emitShimClass(c, &b)
	shim := b.String()
	i := strings.Index(shim, "void initStyleOption(")
	if i < 0 {
		t.Fatal("no initStyleOption override in the shim")
	}
	body := shim[i : i+strings.Index(shim[i:], "\n\t}")]
	if strings.Contains(body, "new QStyleOptionButton") || !strings.Contains(body, "static_cast<const void *>(a0)") {
		t.Errorf("initStyleOption override does not pass the option in place:\n%s", body)
	}
}

// A style virtual's const QStyleOption * is usually a derived option
// (QStyleOptionViewItem, QStyleOptionHeader): the override must get Qt's
// object in place (scoped), not a QStyleOption copy that slices it, or
// Super() hands the C++ style only the base part.
func TestVirtualConstDerivedValueInPlace(t *testing.T) {
	c := class(t, "QProxyStyle")
	for name, want := range map[string]bool{"QStyleOption": true, "QStyleOptionComplex": true, "QStyleOptionHeaderV2": false, "QPixmap": false, "QTextFormat": false} {
		if got := class(t, name).Sliced; got != want {
			t.Errorf("%s: Sliced = %v, want %v", name, got, want)
		}
	}
	var b buf
	gen(t).emitShimClass(c, &b)
	shim := b.String()
	for _, name := range []string{"drawPrimitive", "drawControl", "drawComplexControl", "subElementRect", "sizeFromContents", "pixelMetric", "styleHint"} {
		var v *Method
		for _, x := range c.Virtuals {
			if x.IR.Name == name {
				v = x
			}
		}
		if v == nil {
			t.Errorf("QProxyStyle::%s is not overridable", name)
			continue
		}
		for _, p := range v.Params {
			if p.T.k == kValue && strings.HasPrefix(p.T.cls.Name, "QStyleOption") && (!p.View || p.Rule != "scoped") {
				t.Errorf("%s(%s): View=%v Rule=%q, want an in-place scoped argument", name, p.GoName, p.View, p.Rule)
			}
		}
		i := strings.Index(shim, " "+name+"(")
		if i < 0 {
			t.Errorf("no %s override in the shim", name)
			continue
		}
		body := shim[i : i+strings.Index(shim[i:], "\n\t}")]
		if strings.Contains(body, "new QStyleOption") {
			t.Errorf("%s override copies the option:\n%s", name, body)
		}
	}
}

// The class option cast gives every class below the root a <Class>Cast
// from the root (QStyleOption's qstyleoption_cast), and the root none.
func TestValueCastEmitted(t *testing.T) {
	g := gen(t)
	src := string(g.emitClass(class(t, "QStyleOptionViewItem")))
	if !strings.Contains(src, "func QStyleOptionViewItemCast(o QStyleOptionLike) *QStyleOptionViewItem {") ||
		!strings.Contains(src, "rt.WrapClassFrom(p, \"QStyleOptionViewItem\", rt.KeepOrBorrow, rt.RefOf(unsafe.Pointer(o.AsQStyleOption())).H)") {
		t.Errorf("QStyleOptionViewItemCast missing or not tied to its option:\n%s", src)
	}
	if src := string(g.emitClass(class(t, "QStyleOption"))); strings.Contains(src, "func QStyleOptionCast(") {
		t.Error("the root QStyleOption has a cast to itself")
	}
	if !valueCast(class(t, "QStyleHintReturnMask")) || valueCast(class(t, "QPixmap")) {
		t.Error("valueCast: want QStyleHintReturnMask and not QPixmap")
	}
}

func TestDependsOnEmitted(t *testing.T) {
	src := string(gen(t).emitClass(class(t, "QModelIndex")))
	if !strings.Contains(src, `rt.RegisterDependency("QModelIndex"`) || !strings.Contains(src, `"QAbstractItemModel"`) {
		t.Error("QModelIndex does not register its model as a dependency")
	}
}

// depends-on also accepts a getter declared on a base class and C++
// expressions, and a class may list several.
func TestDependsOnForms(t *testing.T) {
	g := gen(t)
	item := string(g.emitClass(class(t, "QWidgetItem")))
	if !strings.Contains(item, `rt.RegisterDependency("QWidgetItem"`) || !strings.Contains(item, "qute_QLayoutItem_widget") {
		t.Error("QWidgetItem does not register its widget (getter on the base) as a dependency")
	}
	cell := class(t, "QTextTableCell")
	src := string(g.emitClass(cell))
	for _, want := range []string{"C.qute_QTextTableCell_dep0(p), \"QTextTable\"", "C.qute_QTextTableCell_dep1(p), \"QTextDocument\""} {
		if !strings.Contains(src, want) {
			t.Errorf("QTextTableCell: no %s", want)
		}
	}
	var dep1 string
	for _, f := range g.classCFuncs(cell) {
		if strings.Contains(f.decl, "qute_QTextTableCell_dep1(") {
			dep1 = f.body
		}
	}
	if !strings.Contains(dep1, "static_cast<const QTextTableCell *>(self)->firstCursorPosition().document()") {
		t.Errorf("dep1 body does not expand $self:\n%s", dep1)
	}
}

func TestDependsOnYAML(t *testing.T) {
	var o classOpts
	src := "depends-on:\n  - model\n  - { cpp: \"$self->x()\", class: QFoo }\n"
	if err := yaml.Unmarshal([]byte(src), &o); err != nil {
		t.Fatal(err)
	}
	want := depList{{Getter: "model"}, {Cpp: "$self->x()", Class: "QFoo"}}
	if fmt.Sprint(o.DependsOn) != fmt.Sprint(want) {
		t.Errorf("DependsOn = %v, want %v", o.DependsOn, want)
	}
	if err := yaml.Unmarshal([]byte("depends-on: { cpp: x }\n"), &o); err == nil {
		t.Error("a cpp entry without class was accepted")
	}
	o = classOpts{}
	if err := yaml.Unmarshal([]byte("depends-on: model\n"), &o); err != nil || len(o.DependsOn) != 1 || o.DependsOn[0].Getter != "model" {
		t.Errorf("plain string form: %v, %v", o.DependsOn, err)
	}
}

// The report counts an inferred out-parameter as open until it is confirmed.
func TestReportListsInferredOutParameters(t *testing.T) {
	g := gen(t)
	mm := method(t, "QDate", "getDate")
	saved := mm.Params[0].Ruled
	mm.Params[0].Ruled = false
	defer func() { mm.Params[0].Ruled = saved }()
	path := filepath.Join(t.TempDir(), "COVERAGE.md")
	if err := g.writeReport(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "`year`: out (inferred)") {
		t.Error("the report does not list an unconfirmed out-parameter")
	}
}

// A method whose trailing defaults are dropped gets a Full form taking every
// argument; the generated tests skip it (same rules as the short form).
func TestFullForms(t *testing.T) {
	for _, tc := range []struct{ cls, short, full string }{
		{"QAbstractItemModel", "Index", "IndexFull"},
		{"QAbstractItemModel", "Data", "DataFull"},
		{"QTextCursor", "MovePosition", "MovePositionFull"},
		{"QBoxLayout", "AddWidget", "AddWidgetFull"},
		{"QWidget", "SetWindowFlag", "SetWindowFlagFull"},
		{"QFont", "NewQFontWithFamily", "NewQFontWithFamilyFull"},
		{"QMessageBox", "QMessageBoxInformation", "QMessageBoxInformationFull"},
	} {
		c := class(t, tc.cls)
		var short, full *Method
		for _, mm := range append(append(append([]*Method{}, c.Ctors...), c.Methods...), c.Statics...) {
			switch mm.GoName {
			case tc.short:
				short = mm
			case tc.full:
				full = mm
			}
		}
		if short == nil || full == nil {
			t.Errorf("%s: short %s or full %s missing", tc.cls, tc.short, tc.full)
			continue
		}
		if full.FullOf != short || short.FullName != tc.full || len(goParams(full)) <= len(goParams(short)) {
			t.Errorf("%s.%s: not the Full form of %s (%d vs %d params)", tc.cls, tc.full, tc.short, len(goParams(full)), len(goParams(short)))
		}
	}
	// keep-defaults already passes everything: no Full form
	for _, mm := range class(t, "QCoreApplication").Statics {
		if mm.GoName == "QCoreApplicationExitFull" {
			t.Error("keep-defaults method got a Full form")
		}
	}
}

// Plain polymorphic bases of QObject classes look for the QObject's handle.
func TestQObjectBases(t *testing.T) {
	for _, n := range []string{"QPaintDevice", "QGraphicsItem", "QLayoutItem", "QSurface"} {
		if !class(t, n).QBase {
			t.Errorf("%s is not marked as a base of QObject classes", n)
		}
	}
	for _, n := range []string{"QStandardItem", "QSpacerItem", "QImage"} {
		if class(t, n).QBase {
			t.Errorf("%s is marked as a base of QObject classes", n)
		}
	}
	src := string(gen(t).emitClass(class(t, "QPaintDevice")))
	if !strings.Contains(src, `rt.RegisterBaseOfQObject("QPaintDevice"`) {
		t.Error("QPaintDevice does not register its QObject cast")
	}
}

// A plain object handed out by a receiver depends on it; one Go owns does not.
func TestPlainResultsDependOnReceiver(t *testing.T) {
	src := string(gen(t).emitClass(class(t, "QLayout")))
	if !strings.Contains(src, `rt.WrapClassFrom(r, "QLayoutItem", rt.KeepOrBorrow, rt.RefOf(unsafe.Pointer(this)).H)`) {
		t.Error("QLayout.ItemAt does not tie its item to the layout")
	}
	if !strings.Contains(src, `rt.WrapClass(r, "QLayoutItem", rt.TakeOwned)`) {
		t.Error("QLayout.TakeAt (Owned) should not depend on the layout")
	}
}

// A transfer-to-cpp argument records the container that took it.
func TestTransferRecordsContainer(t *testing.T) {
	src := string(gen(t).emitClass(class(t, "QBoxLayout")))
	if !strings.Contains(src, "rt.TransferTo(rt.RefOf(unsafe.Pointer(this)).H, widgetH)") {
		t.Error("QBoxLayout.AddWidget does not pass its layout to TransferTo")
	}
}

// The return rule of a virtual also governs what a Go override returns: the
// caller owns it, found on the base that declares it.
func TestVirtualReturnRule(t *testing.T) {
	c := class(t, "QSortFilterProxyModel")
	var v *Method
	for _, x := range c.Virtuals {
		if x.IR.Name == "mimeData" {
			v = x
		}
	}
	if v == nil || v.RetRule != "transfer-to-go" {
		t.Fatalf("QSortFilterProxyModel mimeData virtual: %+v, want the base's transfer-to-go rule", v)
	}
	src := string(gen(t).emitClass(c))
	if !strings.Contains(src, "rt.RefOf(unsafe.Pointer(r)).H.TransferToCpp()") {
		t.Error("the override's result is not transferred to C++")
	}
}

// A shim runs the C++ implementation when the Go value is gone, and every
// generated C function checks the thread.
func TestShimFallbackAndThreadCheck(t *testing.T) {
	g := gen(t)
	var b buf
	g.emitShimClass(class(t, "QWidget"), &b)
	shim := b.String()
	i := strings.Index(shim, "QSize sizeHint() const override {")
	if i < 0 {
		t.Fatal("no sizeHint override in the QWidget shim")
	}
	body := shim[i : i+strings.Index(shim[i:], "\n\t}")]
	if !strings.Contains(body, "if (qute_virtual(") || !strings.Contains(body, "return QWidget::sizeHint()") {
		t.Errorf("sizeHint does not fall back to the C++ implementation:\n%s", body)
	}
	var cpp buf
	g.emitCpp(g.byName["QtCore"], &cpp)
	src := cpp.String()
	funcs := strings.Count(src, ") {\n\tqute_gui_only();")
	if funcs < 1000 || !strings.Contains(src, "qute_qtcore_wrong_thread();") {
		t.Errorf("only %d C functions check the thread", funcs)
	}
	// Every entry into a Go handler goes through the wrappers that pass the
	// thread the call came from, so Go checks it without calling C++.
	for _, entry := range []string{"qute_qtcore_signal(", "qute_qtcore_virtual(", "qute_qtcore_call("} {
		if n := strings.Count(src, entry); n != 2 { // its declaration and its wrapper
			t.Errorf("%s appears %d times, want only the declaration and the wrapper", entry, n)
		}
	}
	for _, wrapper := range []string{"qute_signal(s->id", "if (qute_virtual(qute_id", "qute_call(s->id"} {
		if !strings.Contains(src, wrapper) {
			t.Errorf("no call through %s", wrapper)
		}
	}
}

// Value classes with a memory option report their size and are measured
// again after the methods that change it.
func TestMemoryOption(t *testing.T) {
	src := string(gen(t).emitClass(class(t, "QImage")))
	if !strings.Contains(src, `rt.RegisterMemory("QImage"`) {
		t.Error("QImage does not register its memory size")
	}
	i := strings.Index(src, "func (this *QImage) Load(")
	if i < 0 || !strings.Contains(src[i:i+strings.Index(src[i:], "\n}\n")], "rt.Remeasure(") {
		t.Error("QImage.Load does not measure the image again")
	}
}

// A conversion constructor is named by its type, not by a cryptic C++
// parameter name; constructors with several named numbers keep the names.
func TestConstructorNames(t *testing.T) {
	c := class(t, "QVariant")
	names := map[string]bool{}
	for _, mm := range c.Ctors {
		names[mm.GoName] = true
	}
	for _, n := range []string{"NewQVariantWithInt", "NewQVariantWithInt64", "NewQVariantWithBool", "NewQVariantWithFloat64", "NewQVariantWithString", "NewQVariantWithRune"} {
		if !names[n] {
			t.Errorf("no %s", n)
		}
	}
	for n := range names {
		for _, bad := range []string{"IArg", "Ll", "WithD", "Qchar", "StringArg", "BArg"} {
			if strings.HasSuffix(n, bad) {
				t.Errorf("cryptic constructor name %s", n)
			}
		}
	}
	method(t, "QDate", "QDate::QDate(int y, int m, int d)") // still NewQDateWithY
}

// depends-on on a QObject class would do nothing: the generator says so.
func TestDependsOnQObjectWarns(t *testing.T) {
	g := gen(t)
	c := class(t, "QTimer")
	saved, warns := c.Opts.DependsOn, len(g.report.warnings)
	c.Opts.DependsOn = depList{{Getter: "parent"}}
	defer func() { c.Opts.DependsOn, g.report.warnings = saved, g.report.warnings[:warns] }()
	g.emitClass(c)
	found := false
	for _, w := range g.report.warnings[warns:] {
		found = found || strings.Contains(w, "depends-on on a QObject class")
	}
	if !found {
		t.Error("no warning for depends-on on a QObject class")
	}
}

// An object reference in a virtual (interceptRequest(QWebEngineUrlRequestInfo &))
// is Qt's object on its stack: the override gets it in place and scoped, and
// the class stays subclassable.
func TestVirtualObjectReferenceInPlace(t *testing.T) {
	c := class(t, "QWebEngineUrlRequestInterceptor")
	if !c.Opts.Subclass {
		t.Fatal("QWebEngineUrlRequestInterceptor is not subclassable")
	}
	var v *Method
	for _, x := range c.Virtuals {
		if x.IR.Name == "interceptRequest" {
			v = x
		}
	}
	if v == nil {
		t.Fatal("interceptRequest is not overridable")
	}
	if p := v.Params[0]; !p.View || p.Rule != "scoped" || !p.T.inplace {
		t.Errorf("info: View=%v Rule=%q inplace=%v, want an in-place scoped argument", p.View, p.Rule, p.T.inplace)
	}
	var b buf
	gen(t).emitShimClass(c, &b)
	if !strings.Contains(b.String(), "static_cast<const void *>(&a0)") {
		t.Error("the shim does not pass the address of Qt's object")
	}
}

// A declared copy constructor suppresses C++'s implicit default constructor,
// so the generator must not default-construct the class.
func TestCopyOnlyClassHasNoDefaultCtor(t *testing.T) {
	for _, n := range []string{"QWebEngineHistoryItem", "QWebEngineClientCertificateSelection"} {
		if c := class(t, n); c.DefCtor {
			t.Errorf("%s: DefCtor = true, but it declares only a copy constructor", n)
		}
	}
}

// A signal that shares its name with a method (QAudioDecoder::error() and the
// signal error(Error)) must be connected through an explicit member pointer.
func TestSignalSharingMethodName(t *testing.T) {
	c := class(t, "QAudioDecoder")
	for _, sg := range c.Signals {
		if sg.IR.Name == "error" {
			if f := gen(t).signalCFunc(c, sg); !strings.Contains(f.body, "static_cast<void (QAudioDecoder::*)(QAudioDecoder::Error)>") {
				t.Errorf("error signal connected without a cast:\n%s", f.body)
			}
			return
		}
	}
	t.Fatal("QAudioDecoder has no error signal")
}

// Enums of a module namespace take the namespace as their prefix; Qt's own
// namespace keeps bare names.
func TestModuleNamespaceEnums(t *testing.T) {
	g := gen(t)
	for cpp, want := range map[string]string{
		"QAudio::State":     "QAudioState",
		"QtVideo::Rotation": "QtVideoRotation",
		"QtSvg::Option":     "QtSvgOption",
		"Qt::AlignmentFlag": "AlignmentFlag",
	} {
		e := g.enums[cpp]
		if e == nil || !e.emitted {
			t.Errorf("%s is not bound", cpp)
			continue
		}
		if e.goName != want {
			t.Errorf("%s is %s, want %s", cpp, e.goName, want)
		}
	}
}

// A keep is guarded (Delete of the kept object panics) when the receiver or
// the argument is not a QObject, unless the overlay says otherwise.
func TestKeepGuards(t *testing.T) {
	cases := []struct {
		cls, sig string
		guard    bool
	}{
		{"QPainter", "QPainter(QPaintDevice *", true},                  // plain keeper, plain device
		{"QPainter", "begin(", true},                                   // same slot through begin
		{"QDataStream", "QDataStream(QIODevice *", true},               // plain keeper of a QObject
		{"QAbstractItemView", "setModel(", false},                      // QObject to QObject: Qt watches it
		{"QTextCursor", "QTextCursor(QTextDocument *", false},          // kept-delete: allow
		{"QWebEnginePage", "QWebEnginePage(QWebEngineProfile *", true}, // kept-delete: guard
	}
	for _, c := range cases {
		mm := method(t, c.cls, c.sig)
		var p *Param
		for _, x := range mm.Params {
			if x.Keep != "" {
				p = x
			}
		}
		if p == nil {
			t.Errorf("%s: no kept argument", mm.Sig)
			continue
		}
		if p.KeepGuard != c.guard {
			t.Errorf("%s: guard %v, want %v", mm.Sig, p.KeepGuard, c.guard)
		}
	}
	// the guard reaches the generated call
	g := gen(t)
	mm := method(t, "QPainter", "begin(")
	if post := strings.Join(g.prepCall(mm.Class, mm, "QPainter.Begin", false).post, "\n"); !strings.Contains(post, `rt.KeepArg(rt.RefOf(unsafe.Pointer(this)).H, "device", paintDeviceH, true)`) {
		t.Errorf("QPainter.Begin keeps without the guard: %s", post)
	}
}

func TestKeptDeleteWarnsOnUnknownValue(t *testing.T) {
	g := &Gen{report: &Report{}}
	c := &Class{Name: "QX", Cpp: "QX"}
	p := &Param{Name: "thing", GoName: "thing", T: &conv{k: kObj, cls: c}}
	g.applyArgRule(c, &irMethod{Name: "setThing", Params: []irParam{{Name: "thing"}}},
		&overlay{Args: map[string]string{"thing": "keep"}, KeptDelete: "sometimes"}, p, 0, false)
	if len(g.report.warnings) != 1 || !strings.Contains(g.report.warnings[0], "kept-delete") {
		t.Errorf("warnings %v, want one about kept-delete", g.report.warnings)
	}
}

// release-slot empties the slot after the call: an ended painter frees its
// device.
func TestReleaseSlotEmitted(t *testing.T) {
	g := gen(t)
	mm := method(t, "QPainter", "end()")
	post := strings.Join(g.prepCall(mm.Class, mm, "QPainter.End", false).post, "\n")
	if !strings.Contains(post, `rt.ReleaseSlot(rt.RefOf(unsafe.Pointer(this)).H, "device")`) {
		t.Errorf("QPainter.End does not drop its device: %q", post)
	}
}

// parented-delete applies to the class and its non-QObject subclasses only:
// a QLayout's destructor takes it out of its parent layout.
func TestParentedDelete(t *testing.T) {
	for _, n := range []string{"QLayoutItem", "QSpacerItem", "QWidgetItem"} {
		if parentedDelete(class(t, n)) == "" {
			t.Errorf("%s: no parented-delete fix", n)
		}
	}
	for _, n := range []string{"QLayout", "QBoxLayout", "QWidget"} {
		if h := parentedDelete(class(t, n)); h != "" {
			t.Errorf("%s: parented-delete %q, want none (a QObject)", n, h)
		}
	}
}

// A T& result of a non-QObject class is the receiver, for chaining; a return
// rule on it is a warning, since nothing is handed over.
func TestReceiverResultChains(t *testing.T) {
	for _, c := range []struct{ cls, sig string }{
		{"QWebEngineProfileBuilder", "setCachePath("},
		{"QTransform", "translate("},
		{"QDataStream", "writeBytes("},
	} {
		if mm := method(t, c.cls, c.sig); mm.Ret.k != kSelf {
			t.Errorf("%s: result kind %v, want the receiver", mm.Sig, mm.Ret.k)
		}
	}
	g := gen(t)
	saved := g.report
	g.report = &Report{}
	defer func() { g.report = saved }()
	c := class(t, "QWebEngineProfileBuilder")
	var im *irMethod
	for i := range c.IR.Methods {
		if c.IR.Methods[i].Name == "setCachePath" {
			im = &c.IR.Methods[i]
		}
	}
	g.newMethod(c, im, &overlay{Return: "borrowed"}, false, false)
	if len(g.report.warnings) != 1 || !strings.Contains(g.report.warnings[0], "is the receiver") {
		t.Errorf("warnings %v, want one about the receiver", g.report.warnings)
	}
}

// Public fields named _… are reserved padding, not API.
func TestReservedFieldsSkipped(t *testing.T) {
	g := gen(t)
	boolT := irType{Spelling: "bool", Canonical: "bool"}
	c := &Class{Name: "QReserved", Cpp: "QReserved", Module: g.byName["QtCore"], Opts: &classOpts{},
		IR: &irClass{Name: "QReserved", Fields: []irField{{Name: "thirdParty", Type: boolT}, {Name: "_reservedFlag", Type: boolT}}}}
	for _, m := range g.fieldMethods(c) {
		if strings.Contains(m.Sig, "_reserved") {
			t.Errorf("reserved field bound: %s", m.Sig)
		}
	}
	if len(c.Skipped) != 1 || !strings.Contains(c.Skipped[0].Why, "reserved field") {
		t.Errorf("skipped %v, want the reserved field listed", c.Skipped)
	}
}

func TestCppNames(t *testing.T) {
	cases := []struct {
		code, name string
		want       bool
	}{
		{"QWidget *w;", "QWidget", true},
		{"(QWidget*)p", "QWidget", true},
		{"QWidgetItem *it;", "QWidget", false},
		{"qute_QWidget_new", "QWidget", false},
		{"QWebEngineCookieStore::FilterRequest", "QWebEngineCookieStore", true},
		{"QWidget", "QWidget", true},
		{"", "QWidget", false},
	}
	for _, c := range cases {
		if got := cppNames(c.code, c.name); got != c.want {
			t.Errorf("cppNames(%q, %q) = %v, want %v", c.code, c.name, got, c.want)
		}
	}
}

// A shim includes its own module's headers and only the dependency headers
// its code names.
func TestIncludesOnlyNamedDependencies(t *testing.T) {
	g := gen(t)
	inc := g.includes(g.byName["QtSvg"], "void f(QPainter *p, QObject *o);")
	has := map[string]bool{}
	for _, n := range inc {
		has[n] = true
	}
	for _, n := range []string{"QSvgRenderer", "QSvgGenerator", "QPainter", "QObject"} {
		if !has[n] {
			t.Errorf("includes lack %s: %v", n, inc)
		}
	}
	for _, n := range []string{"QAbstractItemModel", "QWidget", "QImage"} {
		if has[n] {
			t.Errorf("includes %s, which the code does not name", n)
		}
	}
}

// COVERAGE.md lists what irgen left out of the IR.
func TestReportListsWhatIRLeavesOut(t *testing.T) {
	g := gen(t)
	path := filepath.Join(t.TempDir(), "COVERAGE.md")
	if err := g.writeReport(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"## Not in the IR: operators, conversions and member templates",
		"| Operators, conversions and member templates not in the IR |", "`operator QVariant()`"} {
		if !strings.Contains(s, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	if n := len(class(t, "QIcon").IR.Dropped); n == 0 {
		t.Error("the IR records nothing dropped for QIcon")
	}
}

// Subclassing is on by default: a class with virtuals Go can implement is
// subclassable without a spec entry, whatever its kind (QObject, plain
// polymorphic, abstract with only a shim constructor).
func TestSubclassByDefault(t *testing.T) {
	g := gen(t)
	for _, n := range []string{"QWidget", "QTreeView", "QStyledItemDelegate", "QLayout", "QValidator",
		"QProxyStyle", "QGraphicsItem", "QGraphicsObject", "QGraphicsEffect", "QLayoutItem", "QSyntaxHighlighter"} {
		c := class(t, n)
		if !c.Opts.Subclass || !c.Shim || c.Opts.NoSubclass != "" {
			t.Errorf("%s: Subclass=%v Shim=%v NoSubclass=%q, want subclassable", n, c.Opts.Subclass, c.Shim, c.Opts.NoSubclass)
		}
		if c.Opts.SubclassOpt != nil {
			t.Errorf("%s: the spec still sets subclass, which is the default", n)
		}
	}
	// no spec entry insists on the default any more, nor asks for the death
	// notice a subclass shim already gives (buildShim warns about it)
	for _, m := range g.modules {
		for _, c := range m.Classes {
			if o := c.Opts.SubclassOpt; o != nil && *o {
				t.Errorf("%s: subclass: true is the default; drop it", c.Name)
			}
			if c.Opts.NotifyDtor && c.Opts.Subclass {
				t.Errorf("%s: notify-dtor is redundant on a subclassable class", c.Name)
			}
		}
	}
	// an abstract class is built only through its shim
	c := class(t, "QGraphicsItem")
	if len(c.Ctors) != 0 || len(c.ShimCtors) == 0 {
		t.Errorf("QGraphicsItem: %d public and %d shim constructors, want only shim ones", len(c.Ctors), len(c.ShimCtors))
	}
}

// A class Go cannot subclass says why (COVERAGE.md lists it); values, events
// and classes without virtuals are not candidates at all.
func TestSubclassRefused(t *testing.T) {
	for n, why := range map[string]string{
		"QIODevice":    "pure virtual QIODevice::readData",
		"QApplication": "no-construct",
		"QTextObject":  "destructor is not public",
		"QPaintDevice": "pure virtual QPaintDevice::paintEngine",
	} {
		c := class(t, n)
		if c.Opts.Subclass || !strings.Contains(c.Opts.NoSubclass, why) {
			t.Errorf("%s: Subclass=%v NoSubclass=%q, want refused because %q", n, c.Opts.Subclass, c.Opts.NoSubclass, why)
		}
		if len(c.Virtuals) != 0 || len(c.Protected) != 0 {
			t.Errorf("%s: refused but still has %d virtuals and %d protected methods", n, len(c.Virtuals), len(c.Protected))
		}
	}
	for _, n := range []string{"QFont", "QMouseEvent", "QStyleOptionButton"} {
		if c := class(t, n); c.Opts.Subclass || c.Opts.NoSubclass != "" {
			t.Errorf("%s: Subclass=%v NoSubclass=%q, want not a candidate", n, c.Opts.Subclass, c.Opts.NoSubclass)
		}
	}
	// QIODevice's subclasses still are subclassable, without readData
	c := class(t, "QBuffer")
	if !c.Opts.Subclass {
		t.Fatal("QBuffer is not subclassable")
	}
	for _, v := range c.Virtuals {
		if v.IR.Name == "readData" {
			t.Error("QBuffer.ReadData is overridable, but Go cannot fill a char buffer")
		}
	}
}

// subclass: false turns subclassing off; subclass: true on a class that
// cannot be subclassed is a warning, the default silently lists why.
func TestSubclassOption(t *testing.T) {
	g := gen(t)
	no, yes := false, true
	warns := len(g.report.warnings)
	defer func() { g.report.warnings = g.report.warnings[:warns] }()
	off := &Class{Name: "QOff", QObject: true, HasDtor: true, IR: &irClass{}, Opts: &classOpts{SubclassOpt: &no}}
	g.chooseSubclass(off)
	if off.Opts.Subclass || !strings.Contains(off.Opts.NoSubclass, "subclass: false") {
		t.Errorf("subclass: false: Subclass=%v NoSubclass=%q", off.Opts.Subclass, off.Opts.NoSubclass)
	}
	def := &Class{Name: "QHidden", QObject: true, HasDtor: true, IR: &irClass{}, Opts: &classOpts{NoConstruct: true}}
	g.chooseSubclass(def)
	if len(g.report.warnings) != warns {
		t.Errorf("the default warned: %v", g.report.warnings[warns:])
	}
	insist := &Class{Name: "QInsist", QObject: true, HasDtor: true, IR: &irClass{}, Opts: &classOpts{SubclassOpt: &yes, NoConstruct: true}}
	g.chooseSubclass(insist)
	if insist.Opts.Subclass || len(g.report.warnings) != warns+1 {
		t.Errorf("subclass: true on a no-construct class: Subclass=%v, warnings %v", insist.Opts.Subclass, g.report.warnings[warns:])
	}
	g.noSubclass(insist, "it has no public constructor")
	if len(g.report.warnings) != warns+2 {
		t.Error("subclass: true on a class buildShim refuses does not warn")
	}
}

func virtualNamed(t *testing.T, cls, name string) *Method {
	t.Helper()
	for _, v := range class(t, cls).Virtuals {
		if v.IR.Name == name && (len(v.Params) == 0 || name != "indexOf" || strings.Contains(v.Sig, "QLayoutItem")) {
			return v
		}
	}
	t.Fatalf("%s: %s is not overridable", cls, name)
	return nil
}

// An overridden overload takes the name of its binding (QLayout.IndexOfItem),
// so no override gets a numbered name.
func TestVirtualNamesFollowBindings(t *testing.T) {
	if v := virtualNamed(t, "QBoxLayout", "indexOf"); v.GoName != "IndexOfItem" {
		t.Errorf("indexOf(const QLayoutItem *): %s, want IndexOfItem", v.GoName)
	}
	for _, m := range gen(t).modules {
		for _, c := range m.Classes {
			for _, v := range c.Virtuals {
				if v.Numbered {
					t.Errorf("%s: virtual %s has a numbered name %s", c.Name, v.Sig, v.GoName)
				}
			}
		}
	}
}

// A confirmed in-out parameter of a virtual (validate(QString &, int &)) is
// a parameter and a result of the override, as in the method's binding; a
// derived override that names the parameter differently (QSpinBox::fixup(str))
// finds the base's rule by position.
func TestVirtualInOut(t *testing.T) {
	g := gen(t)
	v := virtualNamed(t, "QValidator", "validate")
	if got, want := g.virtualSig(class(t, "QValidator"), v), "Validate(string, int) (QValidatorState, string, int)"; got != want {
		t.Errorf("override signature %q, want %q", got, want)
	}
	if got := g.virtualSig(class(t, "QSpinBox"), virtualNamed(t, "QSpinBox", "fixup")); got != "Fixup(string) string" {
		t.Errorf("QSpinBox fixup override %q", got)
	}
	var b buf
	g.emitShimClass(class(t, "QValidator"), &b)
	shim := b.String()
	for _, want := range []string{"qute_str c0 = qute_ret_str(a0);", "a0 = qute_take_qstr(c0);", "std::free(c0.data);"} {
		if !strings.Contains(shim, want) {
			t.Errorf("QValidator shim lacks %q", want)
		}
	}
	// an out-parameter that is a pointer may be null: no slot then
	if got := g.virtualSig(class(t, "QWebEnginePage"), virtualNamed(t, "QWebEnginePage", "javaScriptPrompt")); !strings.HasSuffix(got, "(bool, string)") {
		t.Errorf("javaScriptPrompt override %q, want results (bool, string)", got)
	}
}

// Lists and maps can come back from an override (roleNames, itemData,
// selectedIndexes).
func TestVirtualListAndMapResults(t *testing.T) {
	g := gen(t)
	for cls, name := range map[string]string{"QAbstractItemModel": "roleNames", "QStandardItemModel": "itemData", "QTreeView": "selectedIndexes"} {
		v := virtualNamed(t, cls, name)
		if _, ok := g.goVirtRet(v.Ret, "r", "x", class(t, cls).Module); !ok {
			t.Errorf("%s::%s: no Go conversion for the result", cls, name)
		}
	}
}

// Protected enums are bound for the protected methods that use them, and
// the shim re-exports them so its C functions may name them.
func TestProtectedEnums(t *testing.T) {
	g := gen(t)
	if e := g.enums["QAbstractItemView::State"]; e == nil || e.goName == "" {
		t.Fatal("QAbstractItemView::State is not bound")
	}
	c := class(t, "QTreeView")
	found := false
	for _, p := range c.Protected {
		found = found || p.GoName == "SetState"
	}
	if !found {
		t.Error("QTreeViewBase has no SetState")
	}
	var b buf
	g.emitShimClass(c, &b)
	if !strings.Contains(b.String(), "using QAbstractItemView::State;") {
		t.Error("the QTreeView shim does not re-export QAbstractItemView::State")
	}
	for _, f := range g.classCFuncs(c) {
		if strings.Contains(f.body, "QAbstractItemView::State") {
			t.Errorf("%s names the protected enum directly: %s", f.decl, f.body)
		}
	}
}

// The shim forwards to any base constructor, protected ones included, and a
// protected constructor taking Qt's d-pointer is left out.
func TestShimConstructors(t *testing.T) {
	g := gen(t)
	var b buf
	g.emitShimClass(class(t, "QWidget"), &b)
	if !strings.Contains(b.String(), "template <class... A> explicit qute_QWidget(A &&...a) : QWidget(std::forward<A>(a)...) {}") {
		t.Error("the QWidget shim has no forwarding constructor")
	}
	for _, m := range g.modules {
		for _, c := range m.Classes {
			for _, ctor := range c.ShimCtors {
				if privateCtor(ctor.IR) {
					t.Errorf("%s: shim constructor %s takes Qt's private data", c.Name, ctor.Sig)
				}
			}
		}
	}
}

// Protected methods take their declaring class's overlay: a skip there is
// honoured and a rule there gets a generated test (on a Go subclass).
func TestProtectedOverlays(t *testing.T) {
	g := gen(t)
	for _, p := range class(t, "QPlainTextEdit").Protected {
		if p.IR.Name == "getPaintContext" {
			t.Error("QPlainTextEdit.GetPaintContext is bound, but its overlay skips it")
		}
	}
	var add *Method
	for _, p := range g.protectedOf(class(t, "QLayout")) {
		if p.IR.Name == "addChildWidget" {
			add = p
		}
	}
	if add == nil || add.Params[0].Rule != "transfer-to-cpp" {
		t.Fatal("QLayout.AddChildWidget has no transfer-to-cpp rule")
	}
	tc := &testCtx{g: g, m: g.byName["QtWidgets"], imports: map[string]bool{}}
	var b buf
	g.emitGenericTest(tc, class(t, "QLayout"), add, &b)
	if s := b.String(); !strings.Contains(s, "func TestOwnership_QLayout_AddChildWidget") || !strings.Contains(s, "NewQLayoutWith(v, root)") {
		t.Errorf("no test that builds a Go subclass:\n%s", s)
	}
}

// The report lists the object arguments only a Full form takes
// (QProxyStyle(QStyle *style = nullptr) took ownership unnoticed), protected
// methods and virtual results on default rules, and the classes and virtuals
// Go cannot subclass or override.
func TestReportSubclassSections(t *testing.T) {
	g := gen(t)
	var full *Method
	for _, c := range class(t, "QProxyStyle").Ctors {
		if c.FullOf != nil && strings.Contains(c.Sig, "QStyle *") {
			full = c
		}
	}
	if full == nil {
		t.Fatal("QProxyStyle(QStyle *) has no Full form")
	}
	// as before its overlay entry: the argument on the default rule
	p := full.Params[len(full.Params)-1]
	saved, savedRule := p.Ruled, p.Rule
	p.Ruled, p.Rule = false, "borrowed"
	defer func() { p.Ruled, p.Rule = saved, savedRule }()
	path := filepath.Join(t.TempDir(), "COVERAGE.md")
	if err := g.writeReport(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"`QProxyStyle::QProxyStyle(QStyle * style = …)` (Full form) → `style`: borrowed",
		"## Classes Go cannot subclass", "- `QIODevice`: Go cannot implement its pure virtual QIODevice::readData",
		"## Virtuals Go cannot override", "| Classes Go can subclass |",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("report lacks %q", want)
		}
	}
}

// Event classes that reach Go only as a QEvent get a checked downcast that
// depends on the event, so it is Dead whenever the event is.
func TestEventCast(t *testing.T) {
	g := gen(t)
	src := string(g.emitClass(class(t, "QStatusTipEvent")))
	for _, want := range []string{
		"func QStatusTipEventCast(e qtcore.QEventLike) *QStatusTipEvent {",
		`rt.WrapClassFrom(p, "QStatusTipEvent", rt.KeepOrBorrow, rt.RefOf(unsafe.Pointer(e.AsQEvent())).H)`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("QStatusTipEvent: missing %q", want)
		}
	}
	if strings.Contains(string(g.emitClass(class(t, "QEvent"))), "func QEventCast(") {
		t.Error("QEvent has a cast to itself")
	}
	if strings.Contains(string(g.emitClass(class(t, "QImage"))), "Cast(e ") {
		t.Error("a value class has an event cast")
	}
}

// A constructor's cpp overlay builds the object: QLoggingCategory keeps the
// name pointer it is given, so the binding passes it an interned copy that lives on.
func TestConstructorCpp(t *testing.T) {
	g := gen(t)
	var body string
	for _, f := range g.classCFuncs(class(t, "QLoggingCategory")) {
		if strings.Contains(f.decl, "qute_QLoggingCategory_new(") {
			body = f.body
		}
	}
	if !strings.Contains(body, "new QLoggingCategory(names->insert(QByteArray(") {
		t.Errorf("QLoggingCategory constructor body: %q", body)
	}
}

// QPlatform* types are private platform API, but QPlatformSurfaceEvent and
// its enum are public.
func TestPlatformSurfaceEventBound(t *testing.T) {
	method(t, "QPlatformSurfaceEvent", "surfaceEventType()")
}

// A pure virtual whose result Qt uses unchecked is `required`: the subclass
// constructor refuses a Go type that leaves it out, before it builds the
// C++ object. Other pure virtuals keep their zero-value default.
func TestRequiredVirtual(t *testing.T) {
	g := gen(t)
	src := string(g.emitClass(class(t, "QIconEngine")))
	check := `rt.RequireOverride(v, qIconEngineVTable, "NewQIconEngineWith", "Clone", `
	i, j := strings.Index(src, check), strings.Index(src, "C.qute_QIconEngine_new_shim(")
	if i < 0 || j < 0 || i > j {
		t.Errorf("QIconEngine: the Clone check is missing or comes after the C++ constructor (%d, %d)", i, j)
	}
	if strings.Contains(string(g.emitClass(class(t, "QLayout"))), "rt.RequireOverride(") {
		t.Error("QLayout requires an override, but its pure virtuals default to zero values")
	}
}

// Every public constructor of a subclassable class has a subclass form, so a
// Go subclass can be made in any way the class can: a page in a profile of
// its own (it used to have only the parent-only form).
func TestSubclassConstructors(t *testing.T) {
	c := class(t, "QWebEnginePage")
	byName := map[string]subCtor{}
	for _, sc := range c.SubCtors {
		byName[sc.GoName] = sc
	}
	if sc, ok := byName["NewQWebEnginePageWith"]; !ok || sc.CName != "qute_QWebEnginePage_new_shim" {
		t.Errorf("primary subclass constructor: %+v", sc)
	}
	sc, ok := byName["NewQWebEnginePageWithProfileWith"]
	if !ok {
		t.Fatalf("no subclass constructor with a profile; have %v", byName)
	}
	src := string(gen(t).emitClass(c))
	i := strings.Index(src, "func NewQWebEnginePageWithProfileWith(v QWebEnginePageSubclass, profile QWebEngineProfileLike, parent qtcore.QObjectLike)")
	if i < 0 {
		t.Fatal("NewQWebEnginePageWithProfileWith is not emitted with the profile")
	}
	body := src[i : i+strings.Index(src[i:], "\n}\n")]
	for _, want := range []string{"C." + sc.CName + "(", `rt.KeepArg(h, "profile", profileH, true)`, "h.SetSelf(v, "} {
		if !strings.Contains(body, want) {
			t.Errorf("NewQWebEnginePageWithProfileWith lacks %s:\n%s", want, body)
		}
	}
	var cpp buf
	gen(t).emitCpp(c.Module, &cpp)
	if !strings.Contains(cpp.String(), "void * "+sc.CName+"(void * a0, void * a1) {") {
		t.Errorf("no C function %s", sc.CName)
	}
}

// A nil Go func passed for a std::function is an empty std::function, which
// Qt checks before calling (RunJavaScript's result callback).
func TestNilFuncArgument(t *testing.T) {
	g := gen(t)
	var mm *Method
	for _, x := range class(t, "QWebEnginePage").Methods {
		if x.GoName == "RunJavaScriptFull" {
			mm = x
		}
	}
	if mm == nil {
		t.Fatal("no RunJavaScriptFull")
	}
	var cb *Param
	for _, p := range mm.Params {
		if p.T.k == kFunc {
			cb = p
		}
	}
	if cb == nil {
		t.Fatal("no callback parameter")
	}
	pre, _ := g.goArg(cb.T, "f", "test", mm.Class.Module)
	if !strings.Contains(strings.Join(pre, "\n"), "if f != nil {") {
		t.Errorf("a nil func still gets a callback id:\n%s", strings.Join(pre, "\n"))
	}
	if cpp := g.cppArg(cb.T, "a2"); !strings.HasPrefix(cpp, "(a2 ? ") || !strings.HasSuffix(cpp, ": std::function<void (const QVariant &)>())") {
		t.Errorf("id 0 is not an empty std::function: %s", cpp)
	}
}

// A defaulted out-parameter needs its rule for the method to have a Full
// form; one without is listed in COVERAGE.md, not silently dropped, and
// no-full gives the reason one is left out on purpose.
func TestFullFormsNotBuilt(t *testing.T) {
	text := method(t, "QInputDialog", "QString QInputDialog::getText(").FullName
	if text != "QInputDialogGetTextFull" {
		t.Errorf("getText Full form: %q", text)
	}
	// a Full form that only adds an out-parameter is still a Full form
	if mm := method(t, "QLocale", "int QLocale::toInt(const QString & s"); mm.FullName != "ToIntFull" {
		t.Errorf("toInt Full form: %q", mm.FullName)
	}
	if mm := method(t, "QFileDialog", "QString QFileDialog::getOpenFileName("); mm.FullName == "" {
		t.Error("getOpenFileName has no Full form")
	}
	for _, m := range gen(t).modules {
		for _, c := range m.Classes {
			for _, n := range c.NoFull {
				if !strings.HasPrefix(n.Why, "on purpose") {
					t.Errorf("open: no Full form of %s: %s", n.Sig, n.Why)
				}
			}
		}
	}
	found := false
	for _, n := range class(t, "QFontMetrics").NoFull {
		found = found || strings.Contains(n.Why, "tabArray")
	}
	if !found {
		t.Error("the no-full reason of QFontMetrics::size is not recorded")
	}
}

// Events can be constructed; each is built as a shim whose destructor tells
// Go that Qt deleted it (a posted event).
func TestEventConstructors(t *testing.T) {
	for _, n := range []string{"QEvent", "QKeyEvent", "QMouseEvent"} {
		c := class(t, n)
		if len(c.Ctors) == 0 || !c.Shim || kindConst(c) != "rt.KindNotify" {
			t.Errorf("%s: %d constructors, shim %v, kind %s", n, len(c.Ctors), c.Shim, kindConst(c))
		}
	}
	if mm := method(t, "QKeyEvent", "QKeyEvent::QKeyEvent(Type type, int key, Qt::KeyboardModifiers modifiers, const QString"); mm.GoName != "NewQKeyEvent" {
		t.Errorf("QKeyEvent constructor named %s", mm.GoName)
	}
}

// An inline value class crosses cgo as bytes: an override's or signal's
// argument by its address (Go copies it), an override's result into storage
// the shim provides, and a list as one block of the values.
func TestInlineValueCode(t *testing.T) {
	g := gen(t)
	c := class(t, "QAbstractItemModel")
	var shim buf
	g.emitShimClass(c, &shim)
	s := shim.String()
	i := strings.Index(s, "QModelIndex index(int a0, int a1, const QModelIndex & a2) const override {")
	if i < 0 {
		t.Fatal("no index override in the shim")
	}
	body := s[i : i+strings.Index(s[i:], "\n\t}")]
	// the index and its model, found here so Go need not ask C++
	for _, want := range []string{"qute_inline_arg c2 = {const_cast<void *>(static_cast<const void *>(&a2)), {const_cast<void *>(static_cast<const void *>(a2.model()))}};", "QModelIndex r{};", "return r;"} {
		if !strings.Contains(body, want) {
			t.Errorf("index override lacks %s:\n%s", want, body)
		}
	}
	src := string(g.emitClass(c))
	for _, want := range []string{`rt.InlineArgView[QModelIndex](a[0], "QModelIndex")`, `rt.CopyValueOut(ret, unsafe.Pointer(r), "QModelIndex"`, `rt.InlineBlock("QModelIndex"`} {
		if !strings.Contains(src, want) {
			t.Errorf("QAbstractItemModel's Go code lacks %s", want)
		}
	}
	mi := string(g.emitClass(class(t, "QModelIndex")))
	if !strings.Contains(mi, `rt.RegisterInline("QModelIndex"`) || !strings.Contains(mi, `rt.RegisterDependencyWrap("QModelIndex"`) {
		t.Error("QModelIndex does not register as inline with a dependency wrap")
	}
}

// A method name is never skipped everywhere just because some classes use it
// for platform internals: QSplitter::handle(int) was hidden that way, while
// the QPlatform* handles are left out by their type, with the reason listed.
func TestHandleMethods(t *testing.T) {
	if mm := method(t, "QSplitter", "handle(int"); mm.GoName != "Handle" || mm.RetRule != "borrowed" || !mm.RetRuled {
		t.Errorf("QSplitter::handle bound as %s returning %q, want Handle, borrowed by rule", mm.GoName, mm.RetRule)
	}
	method(t, "QFileDevice", "handle()")
	notBound(t, "QWindow", "handle()")
	for _, s := range class(t, "QWindow").Skipped {
		if strings.Contains(s.Sig, "handle()") && strings.Contains(s.Why, "on purpose") {
			return
		}
	}
	t.Error("QWindow::handle is not listed as skipped on purpose")
}

// An upcast to a secondary base adds an offset worked out once in C++,
// instead of calling C++ for each static_cast (it ran on every
// NewQPainterOn(widget)); the C++ side asserts at compile time that the base
// is not virtual, whose offset would not be fixed.
func TestSecondaryBaseOffset(t *testing.T) {
	g := gen(t)
	src := string(g.emitClass(class(t, "QWidget")))
	for _, want := range []string{
		"var quteOffsetQWidgetQPaintDevice = uintptr(C.qute_QWidget_offset_QPaintDevice())",
		"unsafe.Add(self, quteOffsetQWidgetQPaintDevice)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("QWidget.AsQPaintDevice: missing %q", want)
		}
	}
	if strings.Contains(src, "_as_QPaintDevice") {
		t.Error("QWidget.AsQPaintDevice still calls C++")
	}
	var cpp buf
	g.emitCpp(g.byName["QtWidgets"], &cpp)
	c := cpp.String()
	for _, want := range []string{
		"static_assert(qute_fixed_base<QPaintDevice, QWidget>::value",
		"ptrdiff_t qute_QWidget_offset_QPaintDevice(void) {",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("C++: missing %q", want)
		}
	}
}
