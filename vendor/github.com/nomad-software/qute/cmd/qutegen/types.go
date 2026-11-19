package main

import (
	"fmt"
	"strings"
)

// conv describes how one C++ type crosses the boundary. Every kind knows six
// conversions: Go arg -> C, C -> C++ arg, C++ result -> C, C -> Go result,
// and for virtual returns Go -> C-owned C and C-owned C -> C++.

type tkind int

const (
	kVoid tkind = iota
	kPrim
	kEnum
	kFlags
	kString
	kBytes
	kStrList
	kList
	kStruct
	kValue
	kObj
	kCStr
	kChar
	kRaw      // void*, uchar*: unsafe.Pointer
	kDur      // std::chrono::duration: time.Duration
	kTime     // std::chrono::time_point<system_clock>: time.Time
	kPath     // std::filesystem::path: string
	kOpt      // std::optional<T> result: *T or nil
	kMap      // QMap/QHash: Go map
	kFunc     // std::function argument: Go func
	kStrArray // const char *const[]: []string
	kSelf     // T& returning *this: the receiver, for chaining
)

type primDef struct {
	c    string // C type in the header
	cgo  string // cgo name (C.int)
	goT  string // Go type
	cast string // C++ type for static_cast
}

var prims = map[string]primDef{
	"bool":               {"bool", "C.bool", "bool", "bool"},
	"int":                {"int", "C.int", "int", "int"},
	"unsigned int":       {"unsigned int", "C.uint", "uint", "unsigned int"},
	"short":              {"short", "C.short", "int16", "short"},
	"unsigned short":     {"unsigned short", "C.ushort", "uint16", "unsigned short"},
	"long":               {"long", "C.long", "int64", "long"},
	"unsigned long":      {"unsigned long", "C.ulong", "uint64", "unsigned long"},
	"long long":          {"long long", "C.longlong", "int64", "long long"},
	"unsigned long long": {"unsigned long long", "C.ulonglong", "uint64", "unsigned long long"},
	"signed char":        {"signed char", "C.schar", "int8", "signed char"},
	"unsigned char":      {"unsigned char", "C.uchar", "uint8", "unsigned char"},
	"char":               {"char", "C.char", "byte", "char"},
	"float":              {"float", "C.float", "float32", "float"},
	"double":             {"double", "C.double", "float64", "double"},
}

// qsizetype is "long long" canonically but reads better as Go int.
var qsizetypePrim = primDef{"long long", "C.longlong", "int", "long long"}

type structField struct {
	goName, cName, goType, cgo string
}

type structDef struct {
	name   string
	module *Module
	cType  string
	fields []structField
}

type conv struct {
	k        tkind
	cpp      string // canonical C++ type as written in the IR
	base     string // without const, &, *
	ptr      bool
	ref      bool
	cnst     bool
	prim     primDef
	enum     *Enum
	st       *structDef
	cls      *Class
	elem     *conv
	wrap     string  // QPolygon: a QList subclass built from the list
	key      *conv   // map key
	fparams  []*conv // std::function parameters (fret is elem)
	span     bool    // QSpan<T> instead of QList<T>
	bytes    bool    // QByteArrayView instead of QByteArray
	twoFlags bool    // QUrlTwoFlags
	qptr     bool    // QPointer<T> list element
	pairs    bool    // QList<std::pair<A, B>>: a kMap shaped as a slice of pairs
	inplace  bool    // value class passed by non-const reference: the object itself, never a copy
}

func unsupported(t irType, why string) (*conv, error) {
	return nil, fmt.Errorf("unsupported type %q (%s)", t.Canonical, why)
}

// onPurpose marks types that are left out by design; the report lists them
// apart from types still waiting for support.
func onPurpose(why string) (*conv, error) {
	return nil, fmt.Errorf("on purpose: %s", why)
}

var deliberateBases = map[string]string{
	"QThread":                     "threads are not supported yet: the runtime is GUI-thread-only (roadmap)",
	"QBindingStorage":             "Qt's property-binding system; use the getters, setters and change signals",
	"QAbstractEventDispatcher":    "event-loop internals",
	"QAbstractNativeEventFilter":  "native platform events",
	"QBackingStore":               "platform internals",
	"_IO_FILE":                    "C FILE*; use the QFile name or file-descriptor overloads",
	"QModelRoleDataSpan":          "a batching optimisation of data(); use Data",
	"QAbstractUndoItem":           "custom undo items need C++ subclassing",
	"QAbstractTextDocumentLayout": "custom text layouts need C++ subclassing",
	"QPermission":                 "a templated API",
	"QLatin1String":               "use the string overload",
	"QFunctionPointer":            "C function pointer",
	"QAccessibilityHints":         "the accessibility API is not bound",
	"QAccessibleInterface":        "the accessibility API is not bound",
	"QDebug":                      "debug streams; use fmt with the Go values",
	"QGraphicsEffectSource":       "custom graphics effects need C++ subclassing",
	"QItemEditorCreatorBase":      "custom editor creators need C++ subclassing",
	"QTextFrameLayoutData":        "text layout internals",
	"QTextCursorPrivate":          "private API",
	"QTextDocumentPrivate":        "private API",
	"QVulkanInstance":             "Vulkan is not bound",
	"QSslCertificate":             "QtNetwork is not bound",
	"QSslKey":                     "QtNetwork is not bound",
	"QNetworkCookie":              "QtNetwork is not bound",
	"QAuthenticator":              "QtNetwork is not bound",
	"QWebChannel":                 "QtWebChannel is not bound",
	"QPrinter":                    "QtPrintSupport is not bound",
	"QRhi":                        "Qt's rendering hardware interface is not bound",
	"QAbstractVideoBuffer":        "custom video buffers need C++ subclassing",
	"QRawFont":                    "low-level font internals",
	"Qt::Disambiguated_t":         "a tag type that only picks an overload",
	"QFontVariableAxis":           "low-level font internals",
	"QTextLayout":                 "low-level text layout; use QTextDocument and QTextCursor",
	"QTextLayout::FormatRange":    "low-level text layout; use QTextDocument and QTextCursor",
	"QPageLayout":                 "printing page setup is not bound yet",
	"QPageSize":                   "printing page setup is not bound yet",
	"QPageRanges":                 "printing page setup is not bound yet",
	"QPdfOutputIntent":            "printing page setup is not bound yet",
	"QPageLayout::Orientation":    "printing page setup is not bound yet",
	"QPageLayout::Unit":           "printing page setup is not bound yet",
	"QUuid::Id128Bytes":           "a C union; use the string and byte-array forms",
	"QFileDialogArgs":             "Qt's private internals",
	"Qt::partial_ordering":        "C++20 comparison types; use the comparison methods",
	"Qt::weak_ordering":           "C++20 comparison types; use the comparison methods",
	"Qt::strong_ordering":         "C++20 comparison types; use the comparison methods",
	"QJsonObject::iterator":       "iterator; use Keys and Value",
	"QJsonObject::const_iterator": "iterator; use Keys and Value",
	"QJsonArray::iterator":        "iterator; use At and Insert",
	"QJsonArray::const_iterator":  "iterator; use At and Insert",
	"QTextBlock::iterator":        "iterator; use the block and fragment methods",
	"QTextFrame::iterator":        "iterator; use ChildFrames",
	"QCborArray::Iterator":        "iterator; use At, Insert and RemoveAt",
	"QCborArray::ConstIterator":   "iterator; use At, Insert and RemoveAt",
	"QCborMap::Iterator":          "iterator; use Value, Contains, Keys and Remove",
	"QCborMap::ConstIterator":     "iterator; use Value, Contains, Keys and Remove",
	"QCborValueRef":               "a reference into a CBOR container; use the value getters",
	"QVersionNumber::It":          "iterator; use Segments and SegmentAt",
	"QCborSimpleType":             "a global enum the IR does not record; use the typed QCborValue constructors",
	"QCborTag":                    "a global enum the IR does not record; tagged CBOR values are not bound",
	"QCborKnownTags":              "a global enum the IR does not record; tagged CBOR values are not bound",
	"QCborNegativeInteger":        "a global enum the IR does not record; use ToInteger",
	"QtMsgType":                   "a global enum the IR does not record",
	"qfloat16":                    "half-precision floats; use the float32 and float64 forms",
	"std::nullptr_t":              "a null-only overload; use the Null type or value instead",
	"QGenericArgument":            "type-erased C++ argument pointers (Q_ARG); call the Go method instead",
	"QGenericReturnArgument":      "type-erased C++ argument pointers (Q_RETURN_ARG); call the Go method instead",
	"QUntypedBindable":            "Qt's property-binding system; use the getters, setters and change signals",
	"QMetaObject::Data":           "moc internals",
	"QBasicUtf8StringView<false>": "use the string overload",
	"QCollator":                   "collation is not bound; golang.org/x/text/collate covers it",
	"QAbstractOpenGLFunctions":    "the versioned OpenGL function tables are not in the IR; use QOpenGLFunctions and QOpenGLExtraFunctions",
}

// deliberate returns why a type is left out by design, or "" if it is not.
func deliberate(can, base string, ptr, ref, rref, ret bool) string {
	has := func(p string) bool { return strings.HasPrefix(base, p) }
	switch {
	case rref:
		return "C++ move overload; the copying overload does the same in Go"
	case has("QBindable<"):
		return "Qt's property-binding system; use the getters, setters and change signals"
	case strings.Contains(can, "(*)"):
		return "C function pointer; a Go closure cannot become one"
	case has("std::initializer_list<"):
		return "std::initializer_list cannot be built at run time; use the list overload"
	case has("std::reverse_iterator<"), has("QList<QRect>::const_iterator"):
		return "iterator; use the method that returns a list"
	case has("QPlatform") && !has("QPlatformSurfaceEvent"): // the surface event is public API
		return "private platform API"
	case strings.HasSuffix(base, "Private"):
		return "private API"
	case has("QGenericMatrix<"):
		return "a template class; use QMatrix4x4"
	case has("QQuaternion::EulerAngles<"), base == "QQuaternion::Axes":
		return "a nested template or array struct; use the angle and vector overloads"
	case has("QtPrivate::"):
		return "Qt's private internals"
	case has("QRgbaFloat<"):
		return "a template class; use QColor or QRgba64"
	case has("QMap<QFont::Tag"):
		return "a map keyed by a value class; use the single-tag methods"
	case strings.Contains(can, "[") && (strings.Contains(can, "char") || strings.HasPrefix(can, "uchar")):
		return "a C array; use the string or byte-array form"
	case strings.Contains(can, "[") && strings.Contains(can, "float"):
		return "a C array; use the QMatrix4x4 overload (SetUniformMatrix4x4)"
	case strings.Contains(can, "["):
		return "a C array of objects; reimplement the per-item method instead (QGraphicsItem.Paint)"
	case base == "char" && ptr && ref:
		return "returns memory allocated with new[]; use the QByteArray overload"
	case has("QMap<") && strings.Contains(base, "QDate,"):
		return "a map keyed by a value class; use the single-key getter"
	case base == "QTransform::Affine":
		return "internal matrix access; use M11() … M33()"
	case has("std::function<") && ret:
		return "a C++ function cannot be turned back into a Go func"
	}
	return deliberateBases[base]
}

// special handles types that need more than the base-name lookup.
func (g *Gen) special(t irType, ret bool) (*conv, error, bool) {
	can := strings.TrimSpace(t.Canonical)
	base, cnst, ptr, ref, rref := parseCpp(can)
	if why := deliberate(can, base, ptr, ref, rref, ret); why != "" {
		c, err := onPurpose(why)
		return c, err, true
	}
	c := &conv{cpp: can, base: base, ptr: ptr, ref: ref, cnst: cnst}
	has := func(p string) bool { return strings.HasPrefix(base, p) }
	switch {
	case base == "__GLsync" && ptr:
		c.k = kRaw // an opaque GL sync handle
		return c, nil, true
	case base == "void *" && ptr:
		c.k = kRaw // void **: a GL out-pointer
		return c, nil, true
	case has("QSet<") && ret:
		e, err := g.resolve(irType{Canonical: base[len("QSet<") : len(base)-1]}, true)
		if err != nil || !(e.k == kString || e.k == kBytes || e.k == kPrim || e.k == kEnum) {
			c, err := unsupported(t, "set element")
			return c, err, true
		}
		c.k, c.elem = kList, e // iterated into a slice
		return c, nil, true
	}
	switch {
	case has("std::chrono::duration<"):
		c.k = kDur
	case has("std::chrono::time_point<std::chrono::system_clock"):
		c.k = kTime
	case base == "std::filesystem::path" || base == "std::filesystem::__cxx11::path":
		c.k = kPath
	case has("std::optional<") && ret:
		e, err := g.resolve(irType{Canonical: base[len("std::optional<") : len(base)-1]}, true)
		if err != nil || e.k != kValue {
			c, err := unsupported(t, "optional of non-value class")
			return c, err, true
		}
		c.k, c.elem = kOpt, e
	case has("std::function<") && !ret:
		sig := base[len("std::function<") : len(base)-1]
		i := strings.Index(sig, "(")
		r, err := g.resolve(irType{Canonical: strings.TrimSpace(sig[:i])}, true)
		if err != nil || !virtRetSupported(r) {
			c, err := unsupported(t, "callback result")
			return c, err, true
		}
		c.k, c.elem = kFunc, r
		inner := strings.TrimSuffix(sig[i+1:], ")")
		if strings.TrimSpace(inner) != "" {
			for _, a := range splitTopLevel(inner) {
				pc, err := g.resolve(irType{Canonical: a}, true)
				if err != nil {
					c, err := unsupported(t, "callback argument")
					return c, err, true
				}
				c.fparams = append(c.fparams, pc)
			}
		}
	case can == "const char *const[]" || can == "const char *const *":
		if ret {
			return nil, nil, false
		}
		c.k = kStrArray
	case base == "QByteArrayView" && !ptr:
		c.k, c.bytes = kBytes, true
	case (base == "QStringView" || base == "QAnyStringView") && ret && !ptr:
		c.k, c.bytes = kString, true // copied out with toString()
	case (base == "char32_t" || base == "char16_t") && !ptr && !ref:
		c.k = kChar
		c.wrap = base
	case has("QUrlTwoFlags<"):
		args := splitTopLevel(base[len("QUrlTwoFlags<") : len(base)-1])
		en := g.enums[args[0]]
		if en == nil || !en.emitted {
			c, err := unsupported(t, "flags of unbound enum")
			return c, err, true
		}
		c.k, c.enum, c.twoFlags = kFlags, en, true
	case has("QSpan<"):
		inner := strings.TrimSpace(strings.TrimPrefix(base[len("QSpan<"):len(base)-1], "const "))
		switch inner {
		case "char", "unsigned char", "std::byte":
			if ret {
				return nil, nil, false
			}
			c.k, c.span = kBytes, true
			c.elem = &conv{base: inner}
		default:
			e, err := g.resolve(irType{Canonical: inner}, ret)
			if err != nil || e.k != kStruct {
				c, err := unsupported(t, "span element")
				return c, err, true
			}
			c.k, c.elem, c.span = kList, e, true
		}
	case has("QMap<") || has("QHash<"):
		open := strings.Index(base, "<")
		kv := splitTopLevel(base[open+1 : len(base)-1])
		if len(kv) != 2 {
			return nil, nil, false
		}
		k, err1 := g.resolve(irType{Canonical: kv[0]}, ret)
		v, err2 := g.resolve(irType{Canonical: kv[1]}, ret)
		if err1 != nil || err2 != nil || !(k.k == kString || k.k == kPrim && k.prim.goT == "int") ||
			!(v.k == kValue || v.k == kBytes || v.k == kString || v.k == kPrim || v.k == kStruct) {
			c, err := unsupported(t, "map key or value")
			return c, err, true
		}
		c.k, c.key, c.elem = kMap, k, v
	case (base == "void" || base == "unsigned char" || base == "char" && !cnst) && ptr && !ref:
		if base == "char" {
			return nil, nil, false // char* is handled as an out-parameter or buffer
		}
		c.k = kRaw
	default:
		return nil, nil, false
	}
	return c, nil, true
}

func parseCpp(t string) (base string, cnst, ptr, ref, rref bool) {
	s := strings.TrimSpace(t)
	if strings.HasSuffix(s, "&&") {
		rref = true
		s = strings.TrimSpace(strings.TrimSuffix(s, "&&"))
	} else if strings.HasSuffix(s, "&") {
		ref = true
		s = strings.TrimSpace(strings.TrimSuffix(s, "&"))
	}
	if strings.HasSuffix(s, "*") {
		ptr = true
		s = strings.TrimSpace(strings.TrimSuffix(s, "*"))
	}
	if strings.HasSuffix(s, " const") {
		cnst = true
		s = strings.TrimSpace(strings.TrimSuffix(s, " const"))
	}
	if strings.HasPrefix(s, "const ") {
		cnst = true
		s = strings.TrimSpace(strings.TrimPrefix(s, "const "))
	}
	return s, cnst, ptr, ref, rref
}

// resolve maps an IR type to a conversion. ret says the type is a return
// value (or a signal/virtual argument travelling C++ -> Go).
func (g *Gen) resolve(t irType, ret bool) (*conv, error) {
	if c, err, ok := g.special(t, ret); ok {
		return c, err
	}
	can := t.Canonical
	base, cnst, ptr, ref, rref := parseCpp(can)
	if rref {
		return unsupported(t, "rvalue reference")
	}
	c := &conv{cpp: can, base: base, ptr: ptr, ref: ref, cnst: cnst}
	if base == "void" && !ptr {
		c.k = kVoid
		return c, nil
	}
	if p, ok := prims[base]; ok {
		if ptr && base == "char" && cnst && !ref {
			c.k = kCStr
			return c, nil
		}
		if ptr && !ref && (cnst || ret) && base != "char" {
			c.k = kRaw // a raw array of numbers, such as QMatrix4x4::data()
			return c, nil
		}
		if ptr || (ref && !cnst) {
			return unsupported(t, "pointer or out-parameter to primitive")
		}
		if base == "long long" && strings.Contains(t.Spelling, "qsizetype") {
			p = qsizetypePrim
		}
		c.k = kPrim
		c.prim = p
		return c, nil
	}
	if ptr || (ref && !cnst) {
		// only object pointers (and object references) from here on
		cls := g.classes[base]
		switch {
		case cls != nil && cls.Kind != KindValue:
			c.k, c.cls = kObj, cls
		case cls != nil && ptr && !ref:
			c.k, c.cls = kValue, cls // T* of a value class: may be nil; a result is copied
		case cls != nil && ref && ret:
			c.k, c.cls = kValue, cls // a non-const reference result is copied
		default:
			return unsupported(t, "pointer or out-parameter")
		}
		return c, nil
	}
	switch base {
	case "QString":
		c.k = kString
		return c, nil
	case "QStringView", "QAnyStringView":
		if ret {
			return unsupported(t, "string view result")
		}
		c.k = kString // a temporary QString converts implicitly for the call
		return c, nil
	case "QPolygon", "QPolygonF":
		el := "QPoint"
		if base == "QPolygonF" {
			el = "QPointF"
		}
		if st := g.structs[el]; st != nil {
			c.k = kList
			c.elem = &conv{k: kStruct, cpp: el, base: el, st: st}
			c.wrap = base
			return c, nil
		}
	case "QByteArray":
		c.k = kBytes
		return c, nil
	case "QList<QString>":
		c.k = kStrList
		return c, nil
	case "QChar":
		c.k = kChar
		return c, nil
	}
	if strings.HasPrefix(base, "QFlags<") && strings.HasSuffix(base, ">") {
		en := g.enums[base[len("QFlags<"):len(base)-1]]
		if en == nil || !en.emitted {
			return unsupported(t, "flags of unbound enum")
		}
		c.k = kFlags
		c.enum = en
		return c, nil
	}
	if en := g.enums[base]; en != nil {
		if !en.emitted {
			return unsupported(t, "unbound enum")
		}
		c.k = kEnum
		c.enum = en
		return c, nil
	}
	if st := g.structs[base]; st != nil {
		c.k = kStruct
		c.st = st
		return c, nil
	}
	if strings.HasPrefix(base, "QPointer<") && strings.HasSuffix(base, ">") {
		if cls := g.classes[base[len("QPointer<"):len(base)-1]]; cls != nil && cls.Kind == KindObject {
			c.k, c.cls, c.qptr = kObj, cls, true
			return c, nil
		}
	}
	if strings.HasPrefix(base, "QList<std::pair<") && strings.HasSuffix(base, ">>") {
		ab := splitTopLevel(base[len("QList<std::pair<") : len(base)-2])
		if len(ab) == 2 {
			a, err1 := g.resolve(irType{Canonical: ab[0]}, ret)
			b, err2 := g.resolve(irType{Canonical: ab[1]}, ret)
			okKind := func(x *conv) bool {
				return x.k == kString || x.k == kPrim || x.k == kStruct || x.k == kValue || x.k == kBytes
			}
			if err1 == nil && err2 == nil && okKind(a) && okKind(b) {
				c.k, c.key, c.elem, c.pairs = kMap, a, b, true
				return c, nil
			}
		}
		return unsupported(t, "pair list element")
	}
	if strings.HasPrefix(base, "QList<") && strings.HasSuffix(base, ">") {
		inner := base[len("QList<") : len(base)-1]
		e, err := g.resolve(irType{Spelling: inner, Canonical: inner}, ret)
		if err != nil {
			return nil, err
		}
		switch e.k {
		case kPrim, kEnum, kFlags, kStruct, kObj, kValue, kString, kBytes:
		case kList:
			if !ret || e.elem.k != kStruct {
				return unsupported(t, "list element")
			}
		default:
			return unsupported(t, "list element")
		}
		if e.k == kObj && e.cls.Kind == KindValue {
			return unsupported(t, "list element")
		}
		c.k = kList
		c.elem = e
		return c, nil
	}
	if cls := g.classes[base]; cls != nil {
		if cls.Kind == KindValue {
			c.k = kValue
			c.cls = cls
			return c, nil
		}
		if ref && cnst && !ret {
			c.k = kObj
			c.cls = cls
			return c, nil
		}
		return unsupported(t, "object class by value")
	}
	return unsupported(t, "unknown")
}

// ---- Go side ----------------------------------------------------------------

func (g *Gen) goType(c *conv, ctx *Module, param bool) string {
	switch c.k {
	case kVoid:
		return ""
	case kPrim:
		return c.prim.goT
	case kEnum, kFlags:
		return g.qual(c.enum.module, ctx, c.enum.goType())
	case kString, kCStr:
		return "string"
	case kBytes:
		return "[]byte"
	case kStrList:
		return "[]string"
	case kChar:
		return "rune"
	case kStruct:
		return g.qual(c.st.module, ctx, c.st.name)
	case kValue:
		return "*" + g.qual(c.cls.Module, ctx, c.cls.Name)
	case kObj:
		if param {
			return g.qual(c.cls.Module, ctx, c.cls.Name+"Like")
		}
		return "*" + g.qual(c.cls.Module, ctx, c.cls.Name)
	case kList:
		if c.elem.k == kObj {
			return "[]*" + g.qual(c.elem.cls.Module, ctx, c.elem.cls.Name)
		}
		return "[]" + g.goType(c.elem, ctx, false)
	case kSelf:
		return "*" + g.qual(c.cls.Module, ctx, c.cls.Name)
	case kRaw:
		return "unsafe.Pointer"
	case kDur:
		return "time.Duration"
	case kTime:
		return "time.Time"
	case kPath:
		return "string"
	case kOpt:
		return "*" + g.qual(c.elem.cls.Module, ctx, c.elem.cls.Name)
	case kMap:
		if c.pairs {
			return "[]" + g.pairType(c, ctx)
		}
		return "map[" + g.goType(c.key, ctx, false) + "]" + g.goType(c.elem, ctx, false)
	case kStrArray:
		return "[]string"
	case kFunc:
		var ps []string
		for _, p := range c.fparams {
			ps = append(ps, g.goType(p, ctx, false))
		}
		return "func(" + strings.Join(ps, ", ") + ") " + g.goType(c.elem, ctx, false)
	}
	panic("goType")
}

func (g *Gen) qual(m, ctx *Module, name string) string {
	if m == ctx {
		return name
	}
	ctx.imports[m.ImportPath] = true
	return m.Pkg + "." + name
}

// cgoType is the cgo spelling of the C representation.
func (g *Gen) cgoType(c *conv) string {
	switch c.k {
	case kPrim:
		return c.prim.cgo
	case kEnum, kFlags:
		return "C.int64_t"
	case kString, kBytes:
		return "C.qute_str"
	case kStrList, kList:
		return "C.qute_list"
	case kChar:
		return "C.uint"
	case kStruct:
		return "C." + c.st.cType
	case kValue, kObj:
		return "unsafe.Pointer"
	case kCStr:
		return "*C.char"
	case kRaw, kOpt:
		return "unsafe.Pointer"
	case kDur, kTime:
		return "C.int64_t"
	case kPath:
		return "C.qute_str"
	case kMap:
		return "C.qute_map"
	}
	panic("cgoType")
}

// cType is the C spelling used in headers for a single-value representation.
func cType(c *conv) string {
	switch c.k {
	case kPrim:
		return c.prim.c
	case kEnum, kFlags:
		return "int64_t"
	case kString, kBytes:
		return "qute_str"
	case kStrList, kList:
		return "qute_list"
	case kChar:
		return "unsigned int"
	case kStruct:
		return c.st.cType
	case kValue, kObj:
		return "void *"
	case kCStr:
		return "const char *"
	case kVoid, kSelf:
		return "void"
	case kRaw, kOpt:
		return "void *"
	case kDur, kTime:
		return "int64_t"
	case kPath:
		return "qute_str"
	case kMap:
		return "qute_map"
	case kFunc:
		return "uintptr_t"
	case kStrArray:
		return "const void *"
	}
	panic("cType")
}

// cParams returns the C parameter declarations for an argument.
func cParams(c *conv, n string) []string {
	switch c.k {
	case kString, kBytes:
		return []string{"const char *" + n, "ptrdiff_t " + n + "_len"}
	case kStrList:
		return []string{"const char *" + n, "const ptrdiff_t *" + n + "_lens", "ptrdiff_t " + n + "_n"}
	case kList:
		return []string{"const void *" + n, "ptrdiff_t " + n + "_n"}
	case kPath:
		return []string{"const char *" + n, "ptrdiff_t " + n + "_len"}
	case kMap:
		return []string{"const void *" + n, "const void *" + n + "_v", "ptrdiff_t " + n + "_n"}
	}
	return []string{cType(c) + " " + n}
}

// goArg converts Go value v for the C call. It returns statements to run
// before the call and the C argument expressions.
func (g *Gen) goArg(c *conv, v, method string, ctx *Module) (pre []string, args []string) {
	switch c.k {
	case kPrim:
		return nil, []string{fmt.Sprintf("%s(%s)", c.prim.cgo, v)}
	case kEnum, kFlags:
		return nil, []string{fmt.Sprintf("C.int64_t(%s)", v)}
	case kString:
		return nil, []string{fmt.Sprintf("(*C.char)(unsafe.Pointer(unsafe.StringData(%s)))", v), fmt.Sprintf("C.ptrdiff_t(len(%s))", v)}
	case kBytes:
		return nil, []string{fmt.Sprintf("(*C.char)(unsafe.Pointer(unsafe.SliceData(%s)))", v), fmt.Sprintf("C.ptrdiff_t(len(%s))", v)}
	case kStrList:
		d, l := v+"Data", v+"Lens"
		return []string{fmt.Sprintf("%s, %s := quteStrListArg(%s)", d, l, v)},
			[]string{fmt.Sprintf("(*C.char)(unsafe.Pointer(unsafe.SliceData(%s)))", d), fmt.Sprintf("(*C.ptrdiff_t)(unsafe.SliceData(%s))", l), fmt.Sprintf("C.ptrdiff_t(len(%s))", v)}
	case kList:
		tmp := v + "C"
		if inlineVal(c.elem) {
			// one block of the values in Go memory, which holds no pointers
			return []string{fmt.Sprintf("%s := rt.InlineBlock(%q, %q, len(%s), func(i int) unsafe.Pointer { return unsafe.Pointer(%s[i]) }, func(n int) unsafe.Pointer { return unsafe.Pointer(unsafe.SliceData(make([]uint64, (n+7)/8))) })",
					tmp, c.elem.cls.Name, method, v, v)},
				[]string{tmp, fmt.Sprintf("C.ptrdiff_t(len(%s))", v)}
		}
		et := g.cgoType(c.elem)
		var conv string
		switch c.elem.k {
		case kObj, kValue:
			conv = fmt.Sprintf("rt.UseArg(unsafe.Pointer(%s[i]), %q)", v, method)
		case kString:
			conv = fmt.Sprintf("quteCStr(%s[i])", v)
		case kBytes:
			conv = fmt.Sprintf("quteCBytes(%s[i])", v)
		default:
			_, a := g.goArg(c.elem, v+"[i]", method, ctx)
			conv = a[0]
		}
		pre = []string{
			fmt.Sprintf("%s := make([]%s, len(%s))", tmp, et, v),
			fmt.Sprintf("for i := range %s {\n\t\t%s[i] = %s\n\t}", v, tmp, conv),
		}
		if c.elem.k == kString || c.elem.k == kBytes {
			pre = append(pre, fmt.Sprintf("defer quteFreeStrs(%s)", tmp))
		}
		return pre, []string{fmt.Sprintf("unsafe.Pointer(unsafe.SliceData(%s))", tmp), fmt.Sprintf("C.ptrdiff_t(len(%s))", v)}
	case kChar:
		return nil, []string{fmt.Sprintf("C.uint(%s)", v)}
	case kStruct:
		return nil, []string{fmt.Sprintf("c%s(%s)", c.st.name, v)}
	case kValue:
		if !c.cls.DefCtor && !c.ptr {
			return nil, []string{fmt.Sprintf("rt.Use(unsafe.Pointer(%s), %q)", v, method)}
		}
		return nil, []string{fmt.Sprintf("rt.UseArg(unsafe.Pointer(%s), %q)", v, method)}
	case kObj:
		p := v + "Ptr"
		return []string{
			fmt.Sprintf("var %s unsafe.Pointer", p),
			fmt.Sprintf("if %s != nil {\n\t\t%s = rt.UseArg(unsafe.Pointer(%s.As%s()), %q)\n\t}", v, p, v, c.cls.Name, method),
		}, []string{p}
	case kCStr:
		p := v + "CStr"
		return []string{fmt.Sprintf("%s := C.CString(%s)", p, v), fmt.Sprintf("defer C.free(unsafe.Pointer(%s))", p)}, []string{p}
	case kRaw:
		return nil, []string{v}
	case kDur:
		return nil, []string{fmt.Sprintf("C.int64_t(%s)", v)}
	case kTime:
		return nil, []string{fmt.Sprintf("C.int64_t(%s.UnixMilli())", v)}
	case kPath:
		return nil, []string{fmt.Sprintf("(*C.char)(unsafe.Pointer(unsafe.StringData(%s)))", v), fmt.Sprintf("C.ptrdiff_t(len(%s))", v)}
	case kStrArray:
		a := v + "Arr"
		return []string{
			fmt.Sprintf("%s := make([]*C.char, len(%s)+1)", a, v),
			fmt.Sprintf("for i, x := range %s {\n\t\t%s[i] = C.CString(x)\n\t}", v, a),
			fmt.Sprintf("defer func() {\n\t\tfor _, x := range %s {\n\t\t\tC.free(unsafe.Pointer(x))\n\t\t}\n\t}()", a),
		}, []string{fmt.Sprintf("unsafe.Pointer(&%s[0])", a)}
	case kMap:
		kc, vc := v+"K", v+"V"
		kconv, vconv := g.elemArg(c.key, "k", method, ctx), g.elemArg(c.elem, "x", method, ctx)
		pre = []string{
			fmt.Sprintf("%s := make([]%s, 0, len(%s))", kc, g.cgoType(c.key), v),
			fmt.Sprintf("%s := make([]%s, 0, len(%s))", vc, g.cgoType(c.elem), v),
			fmt.Sprintf("for k, x := range %s {\n\t\t%s = append(%s, %s)\n\t\t%s = append(%s, %s)\n\t}", v, kc, kc, kconv, vc, vc, vconv),
		}
		if c.pairs {
			pre[2] = fmt.Sprintf("for _, kv := range %s {\n\t\tk, x := kv.First, kv.Second\n\t\t%s = append(%s, %s)\n\t\t%s = append(%s, %s)\n\t}", v, kc, kc, kconv, vc, vc, vconv)
		}
		if c.key.k == kString || c.key.k == kBytes {
			pre = append(pre, fmt.Sprintf("defer quteFreeStrs(%s)", kc))
		}
		if c.elem.k == kString || c.elem.k == kBytes {
			pre = append(pre, fmt.Sprintf("defer quteFreeStrs(%s)", vc))
		}
		return pre, []string{fmt.Sprintf("unsafe.Pointer(unsafe.SliceData(%s))", kc), fmt.Sprintf("unsafe.Pointer(unsafe.SliceData(%s))", vc), fmt.Sprintf("C.ptrdiff_t(len(%s))", v)}
	case kFunc:
		id := v + "ID"
		var b strings.Builder
		// a nil func is an empty std::function, which Qt checks for
		fmt.Fprintf(&b, "var %s uintptr\n\tif %s != nil {\n\t%s = rt.NewFunc(func(args, ret unsafe.Pointer) {\n", id, v, id)
		var names []string
		if len(c.fparams) > 0 {
			fmt.Fprintf(&b, "\t\ta := unsafe.Slice((*unsafe.Pointer)(args), %d)\n", len(c.fparams))
		} else {
			b.WriteString("\t\t_ = args\n")
		}
		for i, p := range c.fparams {
			n := fmt.Sprintf("p%d", i)
			fmt.Fprintf(&b, "\t\t%s := %s\n", n, g.goRet(p, fmt.Sprintf("*(*%s)(a[%d])", g.cgoType(p), i), "rt.KeepOrBorrow", ctx))
			names = append(names, n)
		}
		call := fmt.Sprintf("%s(%s)", v, strings.Join(names, ", "))
		if c.elem.k == kVoid {
			fmt.Fprintf(&b, "\t\t_ = ret\n\t\t%s\n", call)
		} else {
			ex, _ := g.goVirtRet(c.elem, "r", method, ctx)
			fmt.Fprintf(&b, "\t\tr := %s\n\t\t*(*%s)(ret) = %s\n", call, g.cgoType(c.elem), ex)
		}
		b.WriteString("\t})\n\t}")
		return []string{b.String()}, []string{fmt.Sprintf("C.uintptr_t(%s)", id)}
	}
	panic("goArg")
}

// goRet converts a C result expression r to Go. mode is the rt wrap mode for
// object pointers.
func (g *Gen) goRet(c *conv, r string, mode string, ctx *Module) string {
	return g.goRetFrom(c, r, mode, ctx, "")
}

// goRetFrom is goRet for a result of a method: owner is the Go expression of
// the receiver's handle, which a plain object result depends on
// (rt.WrapClassFrom), or "" for none.
func (g *Gen) goRetFrom(c *conv, r string, mode string, ctx *Module, owner string) string {
	switch c.k {
	case kPrim:
		return fmt.Sprintf("%s(%s)", c.prim.goT, r)
	case kEnum, kFlags:
		return fmt.Sprintf("%s(%s)", g.goType(c, ctx, false), r)
	case kString:
		return fmt.Sprintf("quteStr(%s)", r)
	case kBytes:
		return fmt.Sprintf("quteBytes(%s)", r)
	case kStrList:
		return fmt.Sprintf("quteStrList(%s)", r)
	case kChar:
		return fmt.Sprintf("rune(%s)", r)
	case kCStr:
		return fmt.Sprintf("C.GoString(%s)", r)
	case kStruct:
		return fmt.Sprintf("go%s(%s)", c.st.name, r)
	case kValue:
		return g.wrapExpr(c.cls, r, "rt.TakeOwned", ctx, "")
	case kObj:
		return g.wrapExpr(c.cls, r, mode, ctx, owner)
	case kList:
		et := g.cgoType(c.elem)
		gt := g.goType(c.elem, ctx, false)
		inner := g.goRetFrom(c.elem, "e", mode, ctx, owner)
		if c.elem.k == kValue {
			inner = g.wrapExpr(c.elem.cls, "e", "rt.TakeOwned", ctx, "")
		}
		return fmt.Sprintf("quteList(%s, func(e %s) %s { return %s })", r, et, gt, inner)
	case kRaw:
		return r
	case kDur:
		return fmt.Sprintf("time.Duration(%s)", r)
	case kTime:
		return fmt.Sprintf("time.UnixMilli(int64(%s))", r)
	case kPath:
		return fmt.Sprintf("quteStr(%s)", r)
	case kOpt:
		return g.wrapExpr(c.elem.cls, r, "rt.TakeOwned", ctx, "")
	case kMap:
		kt, vt := g.goType(c.key, ctx, false), g.goType(c.elem, ctx, false)
		kin, vin := g.goRet(c.key, "k", mode, ctx), g.goRet(c.elem, "v", "rt.TakeOwned", ctx)
		if c.pairs {
			pt := g.pairType(c, ctx)
			return fmt.Sprintf("qutePairs(%s, func(k %s) %s { return %s }, func(v %s) %s { return %s }, func(a %s, b %s) %s { return %s{First: a, Second: b} })",
				r, g.cgoType(c.key), kt, kin, g.cgoType(c.elem), vt, vin, kt, vt, pt, pt)
		}
		return fmt.Sprintf("quteMap(%s, func(k %s) %s { return %s }, func(v %s) %s { return %s })",
			r, g.cgoType(c.key), kt, kin, g.cgoType(c.elem), vt, vin)
	}
	panic("goRet")
}

func (g *Gen) wrapExpr(cls *Class, p, mode string, ctx *Module, owner string) string {
	if cls.Opts.Inline && mode == "rt.TakeOwned" {
		// a heap copy that is Go's: its bytes are kept, the copy freed now
		return fmt.Sprintf("rt.InlineOwnedView[%s](%s, %q)", g.qual(cls.Module, ctx, cls.Name), p, cls.Name)
	}
	if owner != "" && cls.Kind == KindPlain && mode != "rt.TakeOwned" && mode != "rt.NewOwned" && mode != "rt.NewParented" {
		return fmt.Sprintf("rt.View[%s](rt.WrapClassFrom(%s, %q, %s, %s), %s)", g.qual(cls.Module, ctx, cls.Name), p, cls.Name, mode, owner, p)
	}
	return fmt.Sprintf("rt.View[%s](rt.WrapClass(%s, %q, %s), %s)", g.qual(cls.Module, ctx, cls.Name), p, cls.Name, mode, p)
}

// goVirtRet converts a Go value returned by a virtual override into C memory
// the shim takes ownership of.
func (g *Gen) goVirtRet(c *conv, v, method string, ctx *Module) (string, bool) {
	switch c.k {
	case kString:
		return fmt.Sprintf("quteCStr(%s)", v), true
	case kBytes:
		return fmt.Sprintf("quteCBytes(%s)", v), true
	case kStrList:
		return fmt.Sprintf("quteCStrList(%s)", v), true
	case kPrim, kEnum, kFlags, kChar, kStruct, kValue:
		_, a := g.goArg(c, v, method, ctx)
		return a[0], true
	case kObj:
		return fmt.Sprintf("rt.UseArg(unsafe.Pointer(%s), %q)", v, method), true
	case kDur, kTime, kRaw:
		_, a := g.goArg(c, v, method, ctx)
		return a[0], true
	case kList:
		if inlineVal(c.elem) {
			// a malloc'd block of the values the shim copies and frees
			return fmt.Sprintf("C.qute_list{data: rt.InlineBlock(%q, %q, len(%s), func(i int) unsafe.Pointer { return unsafe.Pointer(%s[i]) }, func(n int) unsafe.Pointer { return C.malloc(C.size_t(n)) }), len: C.ptrdiff_t(len(%s))}",
				c.elem.cls.Name, method, v, v, v), true
		}
		// a malloc'd array the shim copies into a QList and frees
		return fmt.Sprintf("quteCList(len(%s), func(d []%s) {\n\tfor i, e := range %s {\n\t\td[i] = %s\n\t}\n})",
			v, g.cgoType(c.elem), v, g.elemArg(c.elem, "e", method, ctx)), true
	case kMap:
		return fmt.Sprintf("quteCMap(%s, func(k %s) %s { return %s }, func(e %s) %s { return %s })",
			v, g.goType(c.key, ctx, false), g.cgoType(c.key), g.elemArg(c.key, "k", method, ctx),
			g.goType(c.elem, ctx, false), g.cgoType(c.elem), g.elemArg(c.elem, "e", method, ctx)), true
	}
	return "", false
}

// ---- C++ side ---------------------------------------------------------------

// cppArg builds the Qt argument from the C parameters named n.
func (g *Gen) cppArg(c *conv, n string) string {
	switch c.k {
	case kPrim:
		return fmt.Sprintf("static_cast<%s>(%s)", c.prim.cast, n)
	case kEnum:
		return fmt.Sprintf("static_cast<%s>(%s)", c.enum.cpp, n)
	case kFlags:
		if c.twoFlags {
			return fmt.Sprintf("%s(QFlag(int(%s)))", c.base, n)
		}
		return fmt.Sprintf("QFlags<%s>::fromInt(static_cast<QFlags<%s>::Int>(%s))", c.enum.cpp, c.enum.cpp, n)
	case kString:
		return fmt.Sprintf("QString::fromUtf8(%s, %s_len)", n, n)
	case kBytes:
		if c.span {
			e := c.elem.base
			return fmt.Sprintf("QSpan<%s>(reinterpret_cast<%s *>(const_cast<char *>(%s)), %s_len)", e, e, n, n)
		}
		if c.bytes {
			return fmt.Sprintf("QByteArrayView(%s, %s_len)", n, n)
		}
		return fmt.Sprintf("QByteArray(%s, %s_len)", n, n)
	case kStrList:
		return fmt.Sprintf("qute_qstrlist(%s, %s_lens, %s_n)", n, n, n)
	case kChar:
		if c.wrap != "" {
			return fmt.Sprintf("static_cast<%s>(%s)", c.wrap, n)
		}
		return fmt.Sprintf("QChar(static_cast<char32_t>(%s))", n)
	case kStruct:
		return fmt.Sprintf("qute_to_%s(%s)", c.st.name, n)
	case kValue:
		if c.ptr && c.cnst {
			return fmt.Sprintf("static_cast<const %s *>(%s)", c.cls.Cpp, n)
		}
		if c.ptr {
			return fmt.Sprintf("static_cast<%s *>(%s)", c.cls.Cpp, n) // Qt may update it in place
		}
		if !c.cls.DefCtor || c.inplace {
			return fmt.Sprintf("*static_cast<%s *>(%s)", c.cls.Cpp, n)
		}
		return fmt.Sprintf("(%s ? *static_cast<%s *>(%s) : %s())", n, c.cls.Cpp, n, c.cls.Cpp)
	case kObj:
		if c.ref {
			return fmt.Sprintf("*static_cast<%s *>(%s)", c.cls.Cpp, n)
		}
		return fmt.Sprintf("static_cast<%s *>(%s)", c.cls.Cpp, n)
	case kCStr:
		return n
	case kRaw:
		return fmt.Sprintf("(%s)(%s)", c.cpp, n)
	case kDur:
		return fmt.Sprintf("std::chrono::duration_cast<%s>(std::chrono::nanoseconds(%s))", c.base, n)
	case kTime:
		return fmt.Sprintf("%s(std::chrono::duration_cast<%s::duration>(std::chrono::milliseconds(%s)))", c.base, c.base, n)
	case kPath:
		return fmt.Sprintf("std::filesystem::path(std::string(%s, %s_len))", n, n)
	case kStrArray:
		return fmt.Sprintf("static_cast<const char *const *>(%s)", n)
	case kMap:
		kd, vd := g.mapElemCpp(c.key, "kd[i]"), g.mapElemCpp(c.elem, "vd[i]")
		op := "m.insert(%s, %s)"
		if c.pairs {
			op = "m.append({%s, %s})"
		}
		return fmt.Sprintf("[&] { %s m; auto kd = static_cast<%s const *>(%s); auto vd = static_cast<%s const *>(%s_v); for (ptrdiff_t i = 0; i < %s_n; i++) "+op+"; return m; }()",
			c.base, elemCType(c.key), n, elemCType(c.elem), n, n, kd, vd)
	case kFunc:
		var decl, conv, refs []string
		for i, p := range c.fparams {
			decl = append(decl, fmt.Sprintf("%s p%d", p.cpp, i))
			conv = append(conv, fmt.Sprintf("%s c%d = %s;", cType(p), i, g.cppRet(p, fmt.Sprintf("p%d", i))))
			refs = append(refs, fmt.Sprintf("&c%d", i))
		}
		args := "nullptr"
		pre := strings.Join(conv, " ")
		if len(refs) > 0 {
			pre += fmt.Sprintf(" void *args[] = {%s};", strings.Join(refs, ", "))
			args = "args"
		}
		body := fmt.Sprintf("%s qute_call(s->id, %s, nullptr);", pre, args)
		if c.elem.k != kVoid {
			init := "{}"
			if c.elem.k == kValue || c.elem.k == kObj {
				init = " = nullptr"
			}
			body = fmt.Sprintf("%s %s r%s; qute_call(s->id, %s, &r); return %s;", pre, cType(c.elem), init, args, g.cppVirtRet(c.elem, "r"))
		}
		// id 0 is a nil Go func: an empty std::function
		return fmt.Sprintf("(%s ? %s([s = std::make_shared<GoSlot>(%s)](%s) -> %s { %s }) : %s())", n, c.base, n, strings.Join(decl, ", "), c.elem.cpp, body, c.base)
	case kList:
		var dt, el string
		switch {
		case inlineVal(c.elem):
			dt, el = c.elem.cls.Cpp, "d[i]" // a block of the values
		case c.elem.k == kObj || c.elem.k == kValue:
			dt, el = "void *", g.cppArg(c.elem, "d[i]")
		case c.elem.k == kString:
			dt, el = "qute_str", "QString::fromUtf8(d[i].data, d[i].len)"
		case c.elem.k == kBytes:
			dt, el = "qute_str", "QByteArray(d[i].data, d[i].len)"
		default:
			dt, el = cType(c.elem), g.cppArg(c.elem, "d[i]")
		}
		ex := fmt.Sprintf("[&] { QList<%s> l; auto d = static_cast<%s const *>(%s); l.reserve(%s_n); for (ptrdiff_t i = 0; i < %s_n; i++) l.append(%s); return l; }()",
			c.elem.cppBase(), dt, n, n, n, el)
		if c.wrap != "" {
			ex = c.wrap + "(" + ex + ")"
		}
		if c.span {
			ex = fmt.Sprintf("QSpan<const %s>(%s)", c.elem.base, ex)
		}
		return ex
	}
	panic("cppArg")
}

func (c *conv) cppBase() string {
	switch c.k {
	case kObj:
		return c.cls.Cpp + " *"
	case kEnum:
		return c.enum.cpp
	case kFlags:
		return "QFlags<" + c.enum.cpp + ">"
	case kString:
		return "QString"
	}
	return c.base
}

// cppRet converts a Qt result expression e to its C representation.
func (g *Gen) cppRet(c *conv, e string) string {
	switch c.k {
	case kPrim:
		return fmt.Sprintf("static_cast<%s>(%s)", c.prim.c, e)
	case kEnum:
		return fmt.Sprintf("static_cast<int64_t>(%s)", e)
	case kFlags:
		if c.twoFlags {
			return fmt.Sprintf("static_cast<int64_t>(int(%s))", e)
		}
		return fmt.Sprintf("static_cast<int64_t>((%s).toInt())", e)
	case kString:
		if c.bytes {
			return fmt.Sprintf("qute_ret_str((%s).toString())", e)
		}
		return fmt.Sprintf("qute_ret_str(%s)", e)
	case kBytes:
		if c.bytes {
			return fmt.Sprintf("qute_ret_bytes((%s).toByteArray())", e)
		}
		return fmt.Sprintf("qute_ret_bytes(%s)", e)
	case kStrList:
		return fmt.Sprintf("qute_ret_strlist(%s)", e)
	case kChar:
		if c.wrap != "" {
			return fmt.Sprintf("static_cast<unsigned int>(%s)", e)
		}
		return fmt.Sprintf("static_cast<unsigned int>((%s).unicode())", e)
	case kStruct:
		return fmt.Sprintf("qute_from_%s(%s)", c.st.name, e)
	case kValue:
		if c.ptr {
			return fmt.Sprintf("[&]() -> void * { auto p = %s; return p ? static_cast<void *>(new %s(*p)) : nullptr; }()", e, c.cls.Cpp)
		}
		return fmt.Sprintf("static_cast<void *>(new %s(%s))", c.cls.Cpp, e)
	case kObj:
		if c.qptr {
			return fmt.Sprintf("(void *)((%s).data())", e)
		}
		if c.ref {
			return fmt.Sprintf("(void *)(&(%s))", e)
		}
		return fmt.Sprintf("(void *)(%s)", e)
	case kCStr:
		return e
	case kRaw:
		return fmt.Sprintf("(void *)(%s)", e)
	case kDur:
		return fmt.Sprintf("static_cast<int64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(%s).count())", e)
	case kTime:
		return fmt.Sprintf("static_cast<int64_t>(std::chrono::duration_cast<std::chrono::milliseconds>((%s).time_since_epoch()).count())", e)
	case kPath:
		return fmt.Sprintf("qute_ret_bytes(QByteArray::fromStdString((%s).string()))", e)
	case kOpt:
		return fmt.Sprintf("[&]() -> void * { auto o = %s; return o ? static_cast<void *>(new %s(*o)) : nullptr; }()", e, c.elem.cls.Cpp)
	case kMap:
		if c.pairs {
			return fmt.Sprintf("qute_ret_pairs<%s, %s>(%s, [](const auto &v) { return %s; }, [](const auto &v) { return %s; })",
				elemCType(c.key), elemCType(c.elem), e, g.cppRet(c.key, "v"), g.cppRet(c.elem, "v"))
		}
		return fmt.Sprintf("qute_ret_map<%s, %s>(%s, [](const auto &v) { return %s; }, [](const auto &v) { return %s; })",
			elemCType(c.key), elemCType(c.elem), e, g.cppRet(c.key, "v"), g.cppRet(c.elem, "v"))
	case kList:
		ec := cType(c.elem)
		return fmt.Sprintf("qute_ret_list<%s>(%s, [](const auto &v) { return %s; })", ec, e, g.cppRet(c.elem, "v"))
	}
	panic("cppRet")
}

// cppVirtRet converts the C value the Go override wrote into the Qt return.
func (g *Gen) cppVirtRet(c *conv, n string) string {
	switch c.k {
	case kString:
		return fmt.Sprintf("qute_take_qstr(%s)", n)
	case kBytes:
		return fmt.Sprintf("qute_take_qbytes(%s)", n)
	case kStrList:
		return fmt.Sprintf("qute_take_qstrlist(%s)", n)
	case kObj:
		return fmt.Sprintf("static_cast<%s *>(%s)", c.cls.Cpp, n)
	case kList:
		ex := fmt.Sprintf("qute_take_list<%s, %s>(%s, [](%s e) { return %s; })",
			c.elem.cppBase(), elemCType(c.elem), n, elemCType(c.elem), g.takeElem(c.elem, "e"))
		if c.wrap != "" {
			ex = c.wrap + "(" + ex + ")"
		}
		return ex
	case kMap:
		return fmt.Sprintf("qute_take_map<%s, %s, %s>(%s, [](%s k) { return %s; }, [](%s e) { return %s; })",
			c.base, elemCType(c.key), elemCType(c.elem), n, elemCType(c.key), g.takeElem(c.key, "k"), elemCType(c.elem), g.takeElem(c.elem, "e"))
	}
	return g.cppArg(c, n)
}

// takeElem builds one Qt list or map element from the C element a Go
// override wrote, freeing a string's buffer.
func (g *Gen) takeElem(c *conv, e string) string {
	if inlineVal(c) {
		return e
	}
	switch c.k {
	case kString:
		return fmt.Sprintf("qute_take_qstr(%s)", e)
	case kBytes:
		return fmt.Sprintf("qute_take_qbytes(%s)", e)
	}
	return g.cppArg(c, e)
}

// virtRetSupported reports whether a type can be returned from a Go override.
func virtRetSupported(c *conv) bool {
	switch c.k {
	case kVoid, kPrim, kEnum, kFlags, kChar, kStruct, kValue, kObj, kString, kBytes, kStrList, kDur, kTime, kRaw:
		return !(c.k == kObj && c.ref) && !c.span && !c.bytes
	case kList:
		return !c.span && virtElemSupported(c.elem)
	case kMap:
		return !c.pairs && virtElemSupported(c.key) && virtElemSupported(c.elem)
	}
	return false
}

// virtElemSupported reports whether a list or map element can come from a
// Go override.
func virtElemSupported(c *conv) bool {
	switch c.k {
	case kPrim, kEnum, kFlags, kStruct, kValue, kObj, kString, kBytes:
		return !c.span && !c.bytes && !(c.k == kObj && c.ref) && !(c.k == kValue && c.ptr)
	}
	return false
}

// elemArg converts one Go element for a C array (map keys and values).
func (g *Gen) elemArg(c *conv, v, method string, ctx *Module) string {
	switch c.k {
	case kString:
		return fmt.Sprintf("quteCStr(%s)", v)
	case kBytes:
		return fmt.Sprintf("quteCBytes(%s)", v)
	case kValue, kObj:
		return fmt.Sprintf("rt.UseArg(unsafe.Pointer(%s), %q)", v, method)
	}
	_, a := g.goArg(c, v, method, ctx)
	return a[0]
}

// inlineVal reports whether c is an inline value class (spec option
// inline): its bytes cross cgo, never a pointer to Go memory C could keep.
func inlineVal(c *conv) bool {
	return c != nil && c.k == kValue && c.cls != nil && c.cls.Opts.Inline && !c.ptr
}

func elemCType(c *conv) string {
	if inlineVal(c) {
		return c.cls.Cpp // the values themselves, in one block
	}
	switch c.k {
	case kString, kBytes:
		return "qute_str"
	case kValue, kObj:
		return "void *"
	}
	return cType(c)
}

// mapElemCpp builds a Qt map key or value from its C representation.
func (g *Gen) mapElemCpp(c *conv, e string) string {
	switch c.k {
	case kString:
		return fmt.Sprintf("QString::fromUtf8(%s.data, %s.len)", e, e)
	case kBytes:
		return fmt.Sprintf("QByteArray(%s.data, %s.len)", e, e)
	}
	return g.cppArg(c, e)
}

// pairType is the Go type of one std::pair element: qtcore.Pair[A, B].
func (g *Gen) pairType(c *conv, ctx *Module) string {
	return g.qual(g.byName["QtCore"], ctx, "Pair") + "[" + g.goType(c.key, ctx, false) + ", " + g.goType(c.elem, ctx, false) + "]"
}
