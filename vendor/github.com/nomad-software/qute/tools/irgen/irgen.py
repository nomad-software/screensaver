#!/usr/bin/env python3
"""Parse Qt 6 headers with libclang and write a JSON IR per module.

The IR is checked in (spec/ir/*.json) so the Go generator runs without Qt or
libclang installed. Re-run this only when moving to a new Qt version:

    python3 tools/irgen/irgen.py --qt-include /usr/include/qt6 --out spec/ir
"""
import argparse, glob, json, os, re, subprocess, sys

import clang.cindex as ci

MODULES = ["QtCore", "QtGui", "QtWidgets", "QtOpenGL", "QtOpenGLWidgets", "QtSvg", "QtSvgWidgets",
           "QtMultimedia", "QtMultimediaWidgets", "QtWebEngineCore", "QtWebEngineWidgets"]
# Module namespaces whose enums are bound (as QAudioState, QtVideoRotation, ...).
# Qt's own namespace is read in every module and its enums keep bare names.
NAMESPACES = ["QAudio", "QtVideo", "QtSvg"]
K = ci.CursorKind


def find_libclang():
    for p in ["/usr/lib/libclang.so", "/usr/lib64/libclang.so",
              "/usr/lib/x86_64-linux-gnu/libclang.so"] + sorted(glob.glob("/usr/lib/llvm-*/lib/libclang.so")):
        if os.path.exists(p):
            return p
    return None


def resource_dir():
    try:
        return subprocess.check_output(["clang", "-print-resource-dir"], text=True).strip()
    except Exception:
        return None


def has_default(param):
    # An "=" token marks a default; array bounds ([2][2]) are expressions too,
    # so the children alone cannot tell.
    depth = 0
    for t in param.get_tokens():
        if t.spelling in ("(", "[", "<"):
            depth += 1
        elif t.spelling in (")", "]", ">"):
            depth -= 1
        elif t.spelling == "=" and depth == 0:
            return True
    return False


def type_info(t):
    return {"spelling": t.spelling, "canonical": t.get_canonical().spelling}


def deprecated(c):
    return c.availability in (ci.AvailabilityKind.DEPRECATED, ci.AvailabilityKind.NOT_AVAILABLE)


def enum_info(c, scope):
    vals = []
    for v in c.get_children():
        if v.kind == K.ENUM_CONSTANT_DECL:
            if deprecated(v):
                continue
            vals.append({"name": v.spelling, "value": v.enum_value})
    return {
        "name": c.spelling,
        "scope": scope,
        "scoped": c.is_scoped_enum(),
        "underlying": c.enum_type.get_canonical().spelling,
        "values": vals,
    }


def flags_typedef(c):
    # Q_DECLARE_FLAGS(Flags, Enum) -> typedef QFlags<Enum> Flags;
    canon = c.underlying_typedef_type.get_canonical().spelling
    m = re.match(r"^QFlags<(.+)>$", canon)
    if m:
        return {"name": c.spelling, "enum": m.group(1)}
    return None


def method_info(c, access, section):
    kind = "method"
    if c.kind == K.CONSTRUCTOR:
        kind = "ctor"
    elif section == "qt_signal":
        kind = "signal"
    elif section == "qt_slot":
        kind = "slot"
    params = []
    for a in c.get_arguments():
        params.append({"name": a.spelling, "type": type_info(a.type), "default": has_default(a)})
    m = {
        "name": c.spelling,
        "kind": kind,
        "access": access,
        "params": params,
        "variadic": c.type.is_function_variadic() if c.type.kind == ci.TypeKind.FUNCTIONPROTO else False,
    }
    if c.kind != K.CONSTRUCTOR:
        m["return"] = type_info(c.result_type)
        m["static"] = c.is_static_method()
        m["const"] = c.is_const_method()
        m["virtual"] = c.is_virtual_method()
        m["pure"] = c.is_pure_virtual_method()
    else:
        m["copy"] = c.is_copy_constructor()
        m["move"] = c.is_move_constructor()
    if any(ch.kind == K.CXX_FINAL_ATTR for ch in c.get_children()):
        m["final"] = True
    if deprecated(c):
        m["deprecated"] = True
    for ch in c.get_children():
        if ch.kind == K.ANNOTATE_ATTR and ch.spelling == "qt_invokable":
            m["invokable"] = True
    return m


def class_info(c, module, header, name=None, nested_out=None):
    name = name or c.spelling
    bases = []
    access = "private" if c.kind == K.CLASS_DECL else "public"
    section = ""
    methods, enums, flags, fields, dropped = [], [], [], [], []
    has_qobject_macro = False
    gadget = False
    dtor = None
    for ch in c.get_children():
        if ch.kind == K.CXX_BASE_SPECIFIER:
            if ch.access_specifier == ci.AccessSpecifier.PUBLIC:
                bases.append(ch.type.get_canonical().spelling)
            continue
        if ch.kind == K.CXX_ACCESS_SPEC_DECL:
            access = ch.access_specifier.name.lower()
            section = ""
            for a in ch.get_children():
                if a.kind == K.ANNOTATE_ATTR:
                    section = a.spelling
            continue
        if ch.kind == K.VAR_DECL and ch.spelling == "staticMetaObject":
            continue
        if ch.kind == K.CXX_METHOD and ch.spelling == "metaObject" and ch.is_virtual_method():
            has_qobject_macro = True
        if ch.kind == K.CXX_METHOD and ch.spelling == "qt_check_for_QGADGET_macro":
            gadget = True
        if access not in ("public", "protected"):
            if ch.kind == K.CXX_METHOD and ch.is_virtual_method():
                # private overrides still hide the base virtual from subclasses
                methods.append({"name": ch.spelling, "kind": "method", "access": access, "virtual": True,
                                "const": ch.is_const_method(), "pure": ch.is_pure_virtual_method(),
                                "params": [{"name": a.spelling, "type": type_info(a.type), "default": False}
                                           for a in ch.get_arguments()],
                                "return": type_info(ch.result_type)})
            if ch.kind == K.DESTRUCTOR:
                dtor = {"access": access, "virtual": ch.is_virtual_method()}
            if ch.kind == K.CONSTRUCTOR:
                if ch.is_copy_constructor():
                    methods.append({"name": c.spelling, "kind": "ctor", "access": access, "copy": True, "params": []})
                elif ch.is_default_constructor():
                    methods.append({"name": c.spelling, "kind": "ctor", "access": access, "default_ctor": True, "params": []})
            continue
        if ch.kind == K.DESTRUCTOR:
            dtor = {"access": access, "virtual": ch.is_virtual_method()}
        elif ch.kind in (K.CXX_METHOD, K.CONSTRUCTOR):
            if ch.spelling.startswith("qt_") or ch.spelling in ("d_func",):
                continue
            if ch.spelling.startswith("operator"):
                # not bound; recorded so the coverage report can say so
                if access == "public" and not deprecated(ch):
                    dropped.append({"kind": "operator", "name": ch.displayname})
                continue
            methods.append(method_info(ch, access, section))
        elif ch.kind == K.CONVERSION_FUNCTION and access == "public" and not deprecated(ch):
            dropped.append({"kind": "conversion", "name": ch.displayname})
        elif ch.kind == K.FUNCTION_TEMPLATE and access == "public" and not deprecated(ch) \
                and not ch.spelling.startswith("qt_"):
            dropped.append({"kind": "template", "name": ch.displayname})
        elif ch.kind == K.ENUM_DECL and ch.spelling and access in ("public", "protected"):
            # protected enums type the protected methods Go subclasses call
            # (QAbstractItemView::State for state() and setState())
            e = enum_info(ch, name)
            if access == "protected":
                e["protected"] = True
            enums.append(e)
        elif ch.kind == K.FIELD_DECL and access == "public" and not deprecated(ch):
            fields.append({"name": ch.spelling, "type": type_info(ch.type)})
        elif ch.kind in (K.CLASS_DECL, K.STRUCT_DECL) and access == "public" and ch.is_definition() \
                and ch.spelling and nested_out is not None:
            # public nested classes become "Outer::Inner"
            nested_out.append(class_info(ch, module, header, name + "::" + ch.spelling, nested_out))
        elif ch.kind == K.TYPEDEF_DECL and access in ("public", "protected"):
            f = flags_typedef(ch)
            if f:
                flags.append(f)
    return {
        "name": name,
        "fields": fields,
        "module": module,
        "header": header,
        "kind": "struct" if c.kind == K.STRUCT_DECL else "class",
        "bases": bases,
        "abstract": c.is_abstract_record(),
        "qobject": has_qobject_macro,
        "gadget": gadget,
        "dtor": dtor,
        "enums": enums,
        "flags": flags,
        "methods": methods,
        "dropped": dropped,
    }


def namespace_info(c, module, header, out):
    for ch in c.get_children():
        if ch.kind == K.ENUM_DECL and ch.spelling:
            out["enums"].append(enum_info(ch, c.spelling))
        elif ch.kind == K.TYPEDEF_DECL:
            f = flags_typedef(ch)
            if f:
                out["flags"].append(f)


def gl_constants(tu):
    """Integer GL_* macros from the GL headers Qt includes (gl.h, glext.h,
    qopenglext.h). They are C #defines, so they never appear as declarations."""
    out = {}
    for c in tu.cursor.get_children():
        if c.kind != K.MACRO_DEFINITION or not c.spelling.startswith("GL_"):
            continue
        toks = [t.spelling for t in c.get_tokens()]
        if len(toks) != 2:
            continue
        v = toks[1].rstrip("uUlL")
        try:
            n = int(v, 0)
        except ValueError:
            continue
        if c.spelling not in out:
            out[c.spelling] = n
    return dict(sorted(out.items()))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--qt-include", default="/usr/include/qt6")
    ap.add_argument("--out", default="spec/ir")
    args = ap.parse_args()
    lib = find_libclang()
    if lib:
        ci.Config.set_library_file(lib)
    inc = args.qt_include
    cargs = ["-x", "c++", "-std=c++17", "-fPIC", "-I" + inc]
    for m in MODULES:
        cargs.append("-I" + os.path.join(inc, m))
    rd = resource_dir()
    if rd:
        cargs += ["-isystem", os.path.join(rd, "include")]
    cargs += [
        "-DQT_ANNOTATE_ACCESS_SPECIFIER(x)=__attribute__((annotate(#x)))",
        "-DQT_ANNOTATE_FUNCTION(x)=__attribute__((annotate(#x)))",
        "-DQT_DISABLE_DEPRECATED_UP_TO=0x060C00",
        "-DQT_NO_KEYWORDS",
    ]
    src = "".join("#include <%s/%s>\n" % (m, m) for m in MODULES)
    idx = ci.Index.create()
    tu = idx.parse("qute_ir.cpp", args=cargs, unsaved_files=[("qute_ir.cpp", src)],
                   options=ci.TranslationUnit.PARSE_SKIP_FUNCTION_BODIES |
                   ci.TranslationUnit.PARSE_DETAILED_PROCESSING_RECORD)
    errs = [d for d in tu.diagnostics if d.severity >= ci.Diagnostic.Error]
    for d in errs[:20]:
        print(d, file=sys.stderr)
    if errs:
        sys.exit("libclang reported errors; fix include paths")

    version = None
    vh = os.path.join(inc, "QtCore", "qtcoreversion.h")
    if os.path.exists(vh):
        m = re.search(r'QTCORE_VERSION_STR "([^"]+)"', open(vh).read())
        if m:
            version = m.group(1)

    mods = {m: {"module": m, "qt_version": version, "classes": [], "enums": [], "flags": []} for m in MODULES}
    mods["QtGui"]["gl_constants"] = gl_constants(tu)
    seen = set()
    for c in tu.cursor.get_children():
        loc = c.location.file
        if loc is None:
            continue
        path = os.path.realpath(loc.name)
        mod = None
        for m in MODULES:
            if path.startswith(os.path.realpath(os.path.join(inc, m)) + os.sep):
                mod = m
        if mod is None:
            continue
        header = os.path.basename(path)
        if header.endswith("_impl.h") or "/private/" in path:
            continue
        if c.kind in (K.CLASS_DECL, K.STRUCT_DECL) and c.is_definition() and c.spelling.startswith("Q"):
            if c.spelling in seen:
                continue
            seen.add(c.spelling)
            nested = []
            mods[mod]["classes"].append(class_info(c, mod, header, None, nested))
            mods[mod]["classes"].extend(nested)
        elif c.kind == K.NAMESPACE and (c.spelling == "Qt" or c.spelling in NAMESPACES):
            namespace_info(c, mod, header, mods[mod])
        elif c.kind == K.ENUM_DECL and c.spelling.startswith("Q"):
            mods[mod]["enums"].append(enum_info(c, ""))

    os.makedirs(args.out, exist_ok=True)
    for m, data in mods.items():
        data["classes"].sort(key=lambda x: x["name"])
        p = os.path.join(args.out, m.lower() + ".json")
        with open(p, "w") as f:
            json.dump(data, f, indent=1, sort_keys=True)
            f.write("\n")
        print("%s: %d classes, %d namespace enums -> %s" % (m, len(data["classes"]), len(data["enums"]), p))


if __name__ == "__main__":
    main()
