# CLAUDE.md

qute: Qt 6 bindings for Go, module `github.com/nomad-software/qute`. This file
is the source of truth for the design and for how to work in the repo. If code
and this file disagree, fix one of them; don't leave them diverged. The README
is the user-facing introduction, including why to use qute and how it suits
agent-driven development; keep its claims (numbers, flags, function names)
true when the code changes.

## Goals

- No double free or dangling pointer reachable from ordinary Go code. Misuse
  (a dead handle, the wrong thread, `Delete()` on Borrowed) is a clear panic,
  never a segfault.
- Automatic cleanup by default (GC), deterministic `Delete()` when needed.
- Ownership rules live in small YAML spec files, not in generator code. A
  binding bug is usually a one-line spec fix plus its generated test.
- Apps, including ones Claude writes, can be built, tested and visually
  checked headlessly.

Platform priority: Linux comes first for now. macOS and Windows support is
still wanted, but later; keep the code portable, and don't spend effort on
those platforms until asked.

Non-goals: Qt 5, a QML-first API (later), zero cgo overhead (correctness
first).

Background: miqt (Go) leaves QObjects to `Delete()` or opt-in `GoGC()` and
doesn't re-track ownership when it changes. PySide's typesystem files and
PyQt's `/Transfer/` annotations show ownership belongs in declarative
per-method data. cxx-qt and qmetaobject-rs use a queue to the Qt thread and
`QPointer`-style weak handles. gotk4 shows GC finalizers must not run Qt code
off the main thread. qute takes per-method ownership tracking, GC on by
default (made safe by pinning), PyQt's annotation vocabulary, and a GUI-thread
cleanup queue.

## Architecture

```
Qt headers --libclang (tools/irgen)--> spec/ir/*.json --+
                                       spec/classes/    +--> cmd/qutegen --> qtcore/ qtgui/ qtwidgets/ ... (one per module)
                                       spec/overlays/ --+                    COVERAGE.md, ownership tests

run time: app -> qute (root package) -> qtcore/qtgui/qtwidgets/... (generated Go + C++ shim) -> Qt
                                     \-> internal/rt (handles, registry, pins, cleanup queue)
```

Only `internal/rt` decides who frees what. The C++ shim calls back into it
when Qt destroys an object, emits a signal, or runs an overridden virtual.

One Go package per Qt module (`qtcore`, `qtgui`, `qtwidgets`, `qtopengl`,
`qtopenglwidgets`, `qtsvg`, `qtsvgwidgets`, `qtmultimedia`,
`qtmultimediawidgets`, `qtwebenginecore`, `qtwebenginewidgets`), each with
its own cgo shim built via `pkg-config`; plain `go build`, and an app needs
only the Qt modules it imports. Module dependencies are explicit in
`cmd/qutegen/main.go` (QtOpenGL needs only QtCore and QtGui), because each
shim may only include headers its pkg-config entry provides. Types from
modules qute doesn't bind (QtNetwork's `QSslCertificate`, `QPrinter`,
`QWebChannel`) are type-wide `deliberate` reasons in `types.go`. The root package
`qute` holds `Main`, `Post`, `PostSync`, `Screenshot`, `DumpTree` and
`ProcessEvents`; `qutetest` holds the test harness (`TestMain`, `Run`, GC
checks) and the input helpers (`Click`/`Type`/`Key`/`Wait`), which need
QtTest, so programs that import only `qute` don't link it. `cmd/qute` is the
companion tool.

Adding a module touches more than the generator. Every list of the generated
packages must include it: `mods` in `cmd/qutegen/main.go`, `libraryPkgs` in
`internal/rt/site.go` (debug call sites), `qtPkgs` in `cmd/qute/vet.go`
(otherwise `qute vet` silently ignores the package), `goKeywords` in
`cmd/qutegen/model.go`, `MODULES` in `tools/irgen/irgen.py` (plus
`NAMESPACES` for a module namespace with enums, like `QAudio`), and the
package lists in the README (including its "Qt modules" table) and below. A module needs a `doc_manual.go` with its
package doc.

## Ownership model

Every handle is in exactly one state, stored in the handle and changed only by
annotated methods. Compile-time `Owned[T]` types were rejected: `SetParent`
changes ownership at runtime, so a static type would lie.

| State    | Who frees it                  | GC cleanup | Example                          |
| -------- | ----------------------------- | ---------- | -------------------------------- |
| Owned    | Go (GC cleanup or `Delete()`) | yes        | `qtwidgets.NewQWidget(nil)`      |
| Parented | Qt parent / container         | no         | widget after `lay.AddWidget(w)`  |
| Borrowed | someone else; Go never frees  | no         | `QApplicationFocusWidget()`      |
| Dead     | already destroyed             | no         | any handle after its object died |

Rules (panic messages cite these numbers; keep them stable):

1. Constructors return Owned, or Parented when a non-nil parent is passed.
2. `transfer-to-cpp` arguments become Parented and their GC cleanup is cancelled.
3. `transfer-to-go` results and arguments become Owned and get a cleanup,
   unless Qt has given the QObject a parent: then they are Parented.
4. `Delete()` is idempotent and works on Owned and Parented; on Borrowed it
   panics. It also panics where Qt would keep a dangling pointer: on an object
   a keeper guards (see Kept references), and on a Parented object of a class
   with `parented-delete` (a `QSpacerItem` in a layout; take it out first).
5. Inside a signal or event handler use `DeleteLater()`; `Delete()` of a
   QObject there panics, in every build.
6. Scoped arguments (events, `QPainter*` and style options in callbacks) are
   marked Dead when the callback returns. Long-lived arguments stay Borrowed.
   A plain object Go built on one inside the handler and left alive (a
   `QPainterStateGuard` on the handler's painter) is deleted first, as its
   C++ scope would have (see Kept references). `qtgui.PaintOn`'s painter is
   scoped to its function the same way.
7. Safety net: GC never deletes a QObject that has a parent, even if a rule
   was missed (it prints a warning naming the class, since the Go state its
   tree anchored was collected), and an Owned QObject that Qt gives a parent
   becomes Parented (the event filter sees the reparenting). In reverse, a `transfer-to-cpp`
   argument that Qt did not parent (a widget in a layout with no widget yet)
   is recorded as adopted by the receiver; if the receiver dies and it is
   still parentless, it goes back to Owned (`rt.TransferTo`, `adopt.go`). A container passed to its own
   transfer method (an anchor layout anchored to its own edges) is left as
   it is.

`SetParent(p)` / `SetParent(nil)` move between Parented and Owned
(`transfer-this`).

```go
w := qtwidgets.NewQWidget(nil)                // Owned
lay := qtwidgets.NewQVBoxLayout(w)            // Parented to w
lbl := qtwidgets.NewQLabelWithText("hi", nil) // Owned
lay.AddWidget(lbl)                            // lbl: Owned -> Parented
w.Show()                                      // w pinned while visible
// hidden + unreachable -> GC frees w, which frees lay and lbl
```

**Pinning and anchoring.** GC must never delete something the user is
looking at: a parentless widget or `QWindow` is pinned (held strongly in
`rt`'s strong set) while it is shown, seen by `rt`'s event filter on
QApplication (non-spontaneous Show/Hide; minimising does not unpin), and
anything is pinned after `Keep()` until `Unkeep()`.

Go state of an object, meaning the closures of the connections it sends, its
Go subclass value and the references it keeps, must live as long as the
object even when no Go variable refers to it. A global root would also keep
alive whatever that state refers to (a button handler that mentions its
dialog would pin the dialog forever: Go cannot see a cycle through C++). So
such a handle is *anchored* (`anchor.go`): held by the handle of its tree's
root when that root is an Owned QObject (GC frees the root and with it the
whole tree), otherwise globally. A root's own state is global, so a
parentless object with connections, or a parentless Go subclass, must be
`Delete()`d; `qute vet` flags ones that never are. The event filter reports
reparenting (ChildAdded/ChildRemoved, ParentChange), and the anchored state
of the moved subtree moves with it; a root that stops being Owned hands what
it holds to the global set. Qt calls back with ids that refer to handles
weakly; a callback or virtual whose tree was collected but not yet deleted
finds nothing and is skipped (a virtual then runs the C++ implementation).
GC frees an Owned object only when it is unpinned and unreachable.

**Kept references.** Qt often stores a pointer it does not own: a view's
model, a stream's device, a button's menu, actions added to a widget. The arg
rules `keep`, `keep-each` and `release` make the receiver (or, on a
constructor, the new object) hold a Go reference to the argument, so GC can't
free it underneath Qt (PySide's reference-keeping). `keep` fills one slot per
method (`keep:<slot>` shares one, e.g. `QDataStream` constructor and
`setDevice`); passing another argument or nil replaces it. `keep-each` holds
every distinct argument until a `release` method drops it. References live
on the receiver's handle; a QObject or notifying (shim) receiver is anchored while
it keeps anything, because its Parented handle could otherwise be collected
while the object lives. The death notice drops them.
A keeper and what it keeps are often collected in the same GC cycle, and Go
runs cleanups in no order, but Qt may need the kept object while the keeper
dies (a `QWebEnginePage` must not outlive its profile). So `keep.go` also
counts holds by handle serial, which outlives the handle, and `rt.Drain`
frees a kept object only once no keeper holds it: a keeper freed in the same
drain releases it at once, and one Qt destroys later (a page inside a
collected window) releases it through its death notice, even when its handle
is already gone, or was collected and replaced by a new one when Qt handed
the keeper out again (the registry entry's `prevSerial`). The memory path (`drainValues`) frees through the same check.
An unregistered keeper Go does not free (Parented after a transfer) releases
what it holds when its handle is collected instead.
GC is not the only way a kept object can go: `Delete()` of it while the
keeper still uses it would leave Qt with a dangling pointer. A keep is
*guarded* by default when the keeper or the kept object is not a QObject
(a `QPainter`'s device, a `QDataStream`'s device), since only QObject-to-
QObject references are usually watched by Qt; `Delete()` of a guarded object
panics, naming the keeper. The overlay key `kept-delete: allow|guard`
overrides it per method (`QTextCursor` allows, a page's profile guards), and
`release-slot: <slot>` empties a slot when a method ends the use
(`QPainter::end`). A use that lasts one call is a *lend* instead
(`rt.Lend`/`Unlend`): `qtgui.PaintOn` lends the device to a painter with a
scoped handle (`rt.WrapScoped`, Dead when its function returns), so
`Delete()` of the device panics meanwhile, with no keep, anchoring or GC
cleanup to undo; it costs well under half of `NewQPainterOn`, `End` and
`Delete` (`BenchmarkPainter`).
A scoped argument is not deleted by Go, so the guard cannot protect it: Qt
frees a handler's painter whatever still points at it, and a keeper left
alive would run its destructor on freed memory when GC got to it. A keeper
with a guarded hold on a scoped argument, or on another such keeper, is
*bound* to that argument (`bindScopeLocked`), which holds it strongly until
the handler returns; `Expire` then deletes the bound keepers Go owns (Owned,
not registered), newest first, while Qt's object is still there. Their death
note names the expired argument.
A value only valid with another object (`QModelIndex` and its model,
`QTextBlock` and its document, `QWidgetItem` and its widget) uses the class
option `depends-on`: each wrapper holds that object, and `rt.Use` panics once
it is destroyed instead of letting Qt follow a dangling pointer. An entry is a
getter name (also one declared on a base), or `{cpp: <expression over $self>,
class: <bound class>}` when Qt has no getter; a class may list several
(`QTextTableCell` depends on its table and its document).

## Object lifetime and handle safety

- **Handles are small.** What every handle needs is on `Handle`; state only
  some objects have (pins, connections, keeps, anchoring, a Go subclass,
  debug call sites) is in a `handleExt` made on first use, and the first
  wrapper is cached without a slice. Code that only asks (`Anchored`,
  `Keeps`, `Unkeep`, the reparenting walk's `anchorCount`) reads
  `h.ext.Load()` and must not call `h.x()`, which makes the extension; so
  does the death path, and why an object died is a one-byte code on
  `Handle` for the common reasons (`deathCode`), so a list row that Qt
  destroys allocates nothing. A constructor makes the handle and its
  wrapper in one allocation (`rt.WrapNew`), and registry entries are map
  values, not pointers. A value Qt hands an override for every
  row costs two allocations, its bytes and its handle and wrapper made
  together (`rt.InlineArgView`, `InlineOwnedView`), and no call into C++: the
  shim passes the objects it depends on (`qute_inline_arg`, filled from its
  `depends-on`; `rt.RegisterDependencyWrap`). `bench_test.go` measures it,
  and the cost of many small objects (`BenchmarkListRows`,
  `BenchmarkWidgetRows`).
- **Registry.** Object address to a `weak.Pointer[Handle]` plus a serial.
  Wrapping a pointer twice returns the same handle, and the same Go wrapper
  per type; an upcast of a derived wrapper returns its embedded base, so
  comparing wrappers of the same object with `==` works. An upcast to a
  secondary base (`QWidget.AsQPaintDevice`) adds an offset the shim works
  out once at init (`qute_<Class>_offset_<Base>`), with no call into C++;
  a `static_assert` (`qute_fixed_base`) keeps a virtual base, whose offset
  is not fixed, from compiling.
- **Death notice.** On first wrap of a QObject, `rt` connects
  `QObject::destroyed`, with a separate never-destroyed context object
  (`~QVideoSink` drops connections whose receiver is itself before
  `destroyed` is emitted, which would lose the notice); it marks the handle Dead, removes the registry entry
  and releases its Go callbacks. The entry goes before the memory is freed, so
  a new object at a reused address gets a fresh handle. Queued cleanups carry
  a serial number for the same reason.
- **Wrapped while dying.** Qt still hands a QObject to Go while destroying it
  (the argument of `destroyed`). `Wrap` asks Qt (`QObjectData::wasDeleted`,
  `qute_rt_dying`) and registers nothing then: a handle registered during
  destruction would never get its death notice, and the next object at the
  address would be given it. It returns the handle the object had, Dead
  (remembered from its death notice until the event loop runs again,
  `dying.go`), so `==` with the wrappers a handler holds still works, or a
  fresh Dead handle.
- **Checked calls.** Every generated method does one atomic load (and one per
  dependency); a Dead handle, or one whose dependency died, panics with class
  and method name and the fix. Its C shim checks the thread.
- **Kept alive for the call.** After the C call, generated methods call
  `runtime.KeepAlive` on the receiver and on every argument that holds
  wrappers. Once `rt.Use` has returned the raw pointer nothing else refers to
  the wrapper, and a call that runs a nested event loop (`QEventLoop::exec`,
  `QFontDialog::getFont`, a callback that calls `qute.ProcessEvents`) would
  otherwise let the cleanup queue free it mid-call (miqt #366).
  `keepalive_test.go` guards this. Values returned from Go overrides are safe
  without it: the shim copies them before returning to Qt.
- **Non-QObject classes.** The shim subclass (built for subclassing, or with
  `notify-dtor` where subclassing is off) reports death from its destructor;
  only objects built by our constructors are registered.
  Other non-QObjects (`KindPlain`) are never registered and get a fresh handle
  per wrap. Such a handle handed out by a method (a layout's `itemAt`, a
  painter's `device`) depends on the receiver and on what it depends on
  (`rt.WrapClassFrom`): `Use` panics once the receiver is gone, and `Alive()`
  is false. A plain polymorphic base of a QObject class (`QGraphicsItem`,
  `QPaintDevice`, `QLayoutItem`, `QSurface`) dynamic_casts to QObject when
  wrapped (`rt.RegisterBaseOfQObject`), so a QObject reached through it
  shares the QObject's handle instead of getting a second one that never
  learns of its death.
- **Value classes** (`QImage`, `QFont`, `QIcon`, `QVariant`, ...) are heap
  copies in Owned handles, freed by GC; Qt's implicit sharing keeps copies
  cheap. A class with a `memory` option (images, pixmaps, pictures) counts
  its bytes outside the Go heap (`memory.go`): enough of them run the GC
  (like GOGC=100), and on the GUI thread the value and plain objects already
  queued (a painter keeps its image) are freed at once, keepers first, so a
  loop inside one handler stays bounded.
- **Inline values.** A value class marked `inline` (`QModelIndex`; the shim
  `static_assert`s it is trivially copyable and destructible) keeps its bytes
  in pointer-free Go memory instead of a C++ heap copy (`inline.go`): no GC
  cleanup, no queued delete. An override's or signal's argument is copied
  from Qt's reference (`rt.CopyInline`), a method's result is copied and its
  C++ copy freed at once. Its bytes cross cgo, never a pointer C could keep:
  a list as one block of the values (`rt.InlineBlock`, a nil element is a
  default value), an override's result into storage the shim provides
  (`rt.CopyValueOut`). The generator warns where an inline class appears in
  a shape this does not cover (maps, out-parameters, `std::function`
  results). Qt asks a model about every row with an index, so this is the
  hot path of item views.
- **Re-wrapped before freed.** If Qt hands out an object after GC collected
  its Owned handle but before the queued free ran, the new handle is
  Borrowed; the registry entry remembers the old serial, and the stale free
  makes the new handle Owned instead of freeing the object under it.

## Marshalling

| Qt type                                    | Go type                 | Freed by                          |
| ------------------------------------------ | ----------------------- | --------------------------------- |
| `QString`, `QByteArray`                    | `string`, `[]byte`      | copied; C buffer freed in wrapper |
| `QStringList`, `QList<T>`, `QPolygon(F)`   | `[]string`, `[]T`       | copied                            |
| `QPoint(F)`, `QSize(F)`, `QRect(F)`, `QMargins`, `QColor` | Go structs | nothing to free             |
| value classes                              | Owned handle            | GC cleanup                        |
| object pointers                            | `*T`; args take `TLike` | per ownership state               |
| enums / `QFlags`                           | typed constants / bitmask type with `Has()` | —             |

Beyond the table, the generator maps (see `cmd/qutegen/types.go`):

- out-parameters (`int *row`, `bool *ok`, `QTransform &result`) to extra Go
  results, and in-out references (`QString &`, `int &`) to a parameter plus a
  result. The shape is guessed from the type, so every one needs an `out` or
  `in-out` overlay rule (which also overrides the guess: `QStyle::polish(QPalette
  &)` is in-out); until then COVERAGE.md lists it as open. A constructor's
  pointer (the object keeps it) and `uchar *` (raw memory) are never
  out-parameters. Anything that writes an array (`QMatrix4x4::copyDataTo`) is
  skipped and, if useful, hand-written;
- a pointer followed by its count (`const QPoint *, int`, `const uchar *, int`,
  `char *data, qint64 maxlen`) to one slice, and `QSpan<T>` to a slice. The
  slice is only valid for the call, so where Qt keeps reading the memory
  instead of copying it, an overlay makes the binding copy
  (`QTranslator::load`) or picks the copying overload (`QCborStreamReader`);
- `QMap`/`QHash` with string or int keys to Go maps;
- `std::chrono::duration` to `time.Duration`, `time_point<system_clock>` to
  `time.Time`, `std::filesystem::path` to `string`, `std::optional<T>` to a
  nil-able handle;
- `std::function` parameters to Go funcs (a nil func is an empty
  `std::function`, which Qt checks for), called back through
  `qute_<pkg>_call` and released when Qt drops the function;
- `void *`, `uchar *` to `unsafe.Pointer` (escape hatch);
- a virtual's non-const value-class pointer or reference
  (`initStyleOption(QStyleOptionButton *)`) to Qt's own object, in place and
  scoped, so what the override writes reaches Qt; an object reference in a
  virtual (`interceptRequest(QWebEngineUrlRequestInfo &)`) likewise; and a
  const one whose class a bound subclass extends with data members
  (`drawPrimitive(const QStyleOption *)`, `Class.Sliced`), since Qt passes a
  derived option a copy would slice and `Super()` would hand on cut down;
- a non-QObject class's `T &` result that is `*this` to the receiver, for
  chaining (`QTransform`, `QWebEngineProfileBuilder`); a return rule there
  is a warning;
- `const T *` of a value class to a nil-able handle;
- public data members to getter/setter methods, nested classes
  (`QFont::Tag`) to flat Go names (`QFontTag`), and classes without declared
  constructors to an implicit `NewX()`;
- `QList<std::pair<A, B>>` to `[]qtcore.Pair[A, B]`, a single `std::pair`
  result to two Go results (`major, minor := f.Version()`), `QSet<T>` results
  to slices, `char32_t`/`char16_t` to `rune`, `QStringView` results to
  `string`;
- a `const T *` of a value class with a plural name and no count (an array
  Qt would read past one element) is skipped, never bound as one handle;
- classes marked `raw-pointers` (the GL function tables, `QOpenGLTexture`)
  take every pointer-to-number argument as `unsafe.Pointer`, because GL
  decides the size from other arguments;
- the integer `GL_*` macros become untyped constants in `qtgui/gl_gen.go`
  (`irgen` reads them with libclang's macro records).

Deliberately unsupported (reported as "skipped on purpose", with the reason):
C++ move overloads, `QBindable`, `QThread`, iterators, Qt's private and
platform internals, C function pointers, returned `std::function`s,
`std::initializer_list`, fixed-size C arrays, C++20 ordering types, template
classes (`QGenericMatrix`, `QRgbaFloat`), printing page setup, accessibility,
Vulkan and low-level font and text-layout internals.

Go strings go in as `(ptr, len)` into `QString::fromUtf8`, which copies, so
cgo pointer rules hold. Results come back malloc'd and are copied and freed in
the same generated wrapper; C memory never reaches user code. `nil` maps to
`nullptr` both ways and a returned null is never wrapped.

## Threading

- The main goroutine is locked to the main OS thread in `internal/rt`'s init.
  `qute.Main(setup)` creates QApplication there and runs the event loop.
- `qute.Post(f)` / `qute.PostSync(f)` queue work via
  `QMetaObject::invokeMethod(qApp, ..., Qt::QueuedConnection)`. `PostSync` on
  the GUI thread runs `f` inline (no deadlock) and re-raises panics in the
  caller. While `f` runs, handler panics are recovered at the cgo boundary
  (so Qt's frames return normally) and re-raised as a `*rt.CallbackPanic`
  with the handler's stack. It panics instead of waiting forever when its
  function can never run (the post fails, or the event loop exits first). A
  `Post` before `Main` is held in a list that `AppStarted` hands over under
  the same lock, so none is lost.
- If `f` calls `runtime.Goexit` (`t.Fatal` inside `qutetest.Run`), the GUI
  goroutine is ending and nothing can stop it; its deferred code keeps
  serving Qt from a nested event loop until the app quits
  (`rescueGUIThread`), and `qutetest.TestMain` exits from there. `Run` then ends
  the test on its own goroutine, so later tests run.
- Every generated C function first checks it is on the GUI thread (a
  `QCoreApplication::instance()` and two thread lookups) and panics through
  `qute_<pkg>_wrong_thread` otherwise, in release builds too. Before the
  application exists nothing is checked, which is how init-time registration
  (`QWebEngineUrlScheme`) works. The hand-written helpers (`Tree`,
  `Screenshot`, `qutetest.Click`, `Wait`...) call `rt.GUIOnly`.
- Every entry from Qt into a Go handler (signal, virtual, `std::function`)
  checks it runs on the GUI thread (the generated C++ looks the thread up,
  `qute_off_gui`, and passes the answer in, so Go need not call C++ to ask) and panics otherwise: handlers use Qt
  objects, and the callback state (`InCallback`, captured panics) is the GUI
  thread's. A callback Qt runs on a worker thread gets a hand-written binding
  that converts its arguments to plain Go values and skips the runtime
  (`QWebEngineCookieStore.SetCookieFilter`, `cookiefilter_manual.go`).
- GC cleanups never call `delete`: they push onto a lock-free queue that the
  GUI thread drains from a posted event. `rt.Drain` may be re-entered (a
  destructor runs a Go handler that runs the event loop), so it merges what
  a nested drain left waiting instead of overwriting it. After the event loop exits, queued
  cleanups are dropped, since deleting after the app is gone crashes Qt.
  Unreachable `qtcore.Connection`s are freed through the same queue.
- Build tag `qute_debug` adds a Go-side GUI-thread check that names the
  method (like the shims, it checks nothing before the application exists),
  and records creation and death call sites: the first frame that
  is the program's own (`userFrame` in `site.go`; its `main` counts, only
  the `_testmain.go` main of a test binary does not).

## Signals and subclassing

- `btn.OnClicked(func(checked bool) {...})` returns a `qtcore.Connection`.
  The closure is stored on the sender's handle, which is anchored while the
  connection lives (see Pinning and anchoring); the C++ lambda captures only
  an id that refers to the handle weakly, and a `GoSlot` destructor releases
  it when Qt drops the connection (`Disconnect()` or sender destroyed). The
  sender is the context object. `std::function` callbacks are `cgo.Handle`s
  (marked by the top bit of the id).
- `model.EmitDataChanged(...)` emits a signal.
- Subclassing: embed `qtwidgets.QWidgetBase` (etc.), define virtual methods
  by name, construct with `NewQWidgetWith(v, parent)`; every other public
  constructor has a subclass form too, its name plus `With`
  (`NewQWebEnginePageWithProfileWith(v, profile, parent)`, `subCtors`). It is on by default
  for every bound class with virtuals (QObject or polymorphic plain class);
  `chooseSubclass` and `buildShim` turn it off, with the reason in
  `COVERAGE.md`, for values, events, `no-construct` classes, a non-public
  destructor, no constructor, or a pure virtual Go cannot implement.
  `subclass: false` opts out; `subclass: true` only insists (a warning if
  it cannot be). The shim subclass (`qute_QWidget`) has a forwarding
  constructor, so it also reaches protected constructors (those taking Qt's
  d-pointer are left out), and an abstract class is built only through it.
  It checks a per-instance bitmask, so non-overridden virtuals
  skip the cgo hop. Overrides are detected with interface assertions plus
  `rt.UserDefined`, which follows a promoted method to the type that declares
  it and rejects the generated wrappers and Base types, so a public virtual
  like `SizeHint` can't dispatch to itself, while an override declared on a
  user type that another user type embeds (`type Fancy struct{ Canvas }`)
  still counts. The answer is fixed by the type's methods, so `rt.Mask` works
  it out once per Go type and class and caches it (an icon engine per row
  pays the reflection once). `b.Super().X()`
  calls the C++ base. An override has the Go name and signature of the
  method's binding (`IndexOfItem`; a confirmed in-out parameter is a
  parameter plus a result, `Validate(input string, pos int) (State, string,
  int)`), and may return lists and maps. Protected methods are on the Base
  type, built and named once per declaring class (`ownProtected`) with its
  overlay, so their rules get generated tests on a Go subclass; protected
  enums are bound and the shim re-exports them (`using`) for its C
  functions. Pure virtuals not overridden return their zero value, except
  one an overlay marks `required` because Qt uses its result unchecked
  (`QIconEngine::clone`): `NewXWith` panics for a type that leaves it out. The Go
  value is anchored like a connection, and `GoValue()` on any non-value
  wrapper returns it (a view's delegate, a block's user data). The id the
  shim dispatches with is dropped by the death notice, or by a cleanup on the
  handle when GC collects it first (with its tree).
  An override's object result follows the method's return rule in the other
  direction: `transfer-to-go` or `parented` (the C++ caller owns it, e.g.
  `mimeData`, `createPopupMenu`) makes it Parented; the rule may sit on the
  base that declares the virtual.

```go
type Canvas struct{ qtwidgets.QWidgetBase }

func (c *Canvas) PaintEvent(e *qtgui.QPaintEvent) {
	qtgui.PaintOn(c, func(p *qtgui.QPainter) {
		p.FillRect(e.Rect(), qtgui.RGB(200, 30, 30))
	})
}

qtwidgets.NewQWidgetWith(&Canvas{}, nil)
```

## Generator and spec

- **IR** (`spec/ir/*.json`) from `tools/irgen/irgen.py`, checked in, so the
  generator needs neither Qt nor libclang. Its `qt_version` (6.12.0 today) is
  the oldest Qt the bindings compile against: they call that version's API.
  Re-running irgen on a newer Qt raises the minimum; update the README's
  requirements with it. It records public and protected
  members, signals and slots (via `QT_ANNOTATE_ACCESS_SPECIFIER`), enums
  (protected ones marked `protected`),
  `QFlags` typedefs, and private virtual overrides (which hide base virtuals
  from the shim), and lists (`dropped`) the public operators, conversion
  operators and member templates it leaves out. Public fields named `_…`
  (reserved) are skipped by the generator. Each shim includes the headers of
  its own module's classes and of the dependency classes its code names.
- **Classes** (`spec/classes/<module>.yaml`): which classes are bound, with
  options `kind: value|event` (an event Go builds is built as a notifying
  shim, so a posted event Qt deletes is seen to die), `inline` (see Inline
  values), `subclass: false` (see Signals and
  subclassing), `notify-dtor`, `no-construct` (only Qt creates it),
  `skip-virtuals`, `raw-pointers`, `enums-only` (enums but no wrapper, e.g.
  `QChar`, which is a `rune` in Go), `depends-on` (see Kept references; on a
  QObject class it does nothing and the generator warns), `memory` (`size:`
  a C++ expression over `$self`, `changed-by:` the methods after which it is
  measured again; see Value classes), `cast` (on the root of a value hierarchy, the C++ cast that gives every
  class below it an `XCast`: `qstyleoption_cast`), `parented-delete` (the fix Delete()
  names when it refuses a Parented object of the class or of a non-QObject
  subclass, because the container does not notice its destructor:
  `QLayoutItem`), `reviewed`. A class's primary base must
  be bound too. Don't use class-level `reviewed`: it hides methods nobody
  looked at (it hid `QTextCursor::insertList` and `QGraphicsWidget::setLayout`).
- **Overlays** (`spec/overlays/<module>.yaml`), keyed by class (a nested
  class by its Go name, `QCalendarYearMonthDay`) and then by method name (all
  overloads) or signature (`addWidget(QWidget*, int, int, Qt::Alignment)`):

  ```yaml
  QLayout:
    addWidget:  { args: { w: transfer-to-cpp } }
    takeAt:     { return: transfer-to-go }
  ```

  Arg rules: `transfer-to-cpp`, `transfer-to-go`, `transfer-this`,
  `borrowed`, `scoped`, `keep`, `keep:<slot>`, `keep-each`, `release`, and
  for non-object parameters `out` and `in-out` (keyed by name or index).
  Return rules:
  `transfer-to-go`, `parented`, `borrowed`; on a virtual they also govern
  what a Go override returns (see Signals and subclassing). An unknown rule,
  or a return rule on a result that is not an object pointer, is a generator
  warning.
  Other keys: `name`, `skip`, `keep-defaults`, `full`, `no-full: <why>` (the
  method has no Full form on purpose: a defaulted input array), `error`,
  `error-string`, `reviewed`, `doc`, `test`, `cpp` (a custom C++ call: `$self`, `$0`, `$1`…; on a
  constructor it builds the object, `$class` naming the class to construct,
  the shim when there is one: `QLoggingCategory` copies the name Qt keeps), `created: true` (the
  result is built by the binding, so it is registered and its death seen),
  `kept-delete: allow|guard` and `release-slot: <slot>` (see Kept references),
  `required: <when Qt calls it>` (a pure virtual a Go subclass must define).
  `QGraphicsScene::addRect` and friends use `cpp` + `created` to build
  qute's notifying item subclasses instead of Qt's, so those items report
  their death however they die.
- **Defaults** (Shiboken-style): a constructor argument named `parent` is
  `transfer-this`; other object pointers and returned objects are
  `borrowed`; value results are Owned copies; trailing default arguments are
  dropped (a constructor's `parent` is kept), and the method also gets a
  `Full` form, `<Name>Full`, that takes every argument (`IndexFull`,
  `NewQFontWithFamilyFull`). There is none when an overlay has
  `keep-defaults`, `full`, `no-full` or `cpp`, when a defaulted argument has
  no Go type (a defaulted `bool *ok` needs its `out` rule: an out-parameter is
  never guessed when it has a default), or when another overload already has
  its Go signature (out-parameters count: `ToIntFull` returns the `ok`
  `ToInt` leaves out). COVERAGE.md lists every method whose Full form cannot
  be built, open or on purpose. Full forms share the short form's rules and
  tests.
- **Naming**: methods mirror Qt (`setText` → `SetText`); constructors are
  `NewX` (parent-only form) and `NewXWith<FirstParam>` (a number or string
  first parameter by its name, except a one-argument conversion or a
  cryptic name, which are named by type: `NewQVariantWithInt`); statics are package
  functions `<Class><Method>`; signals `On<Signal>` / `Emit<Signal>`; enums
  `<Class><Value>` (Qt namespace: bare, e.g. `AlignLeft`; a module namespace
  is the prefix, e.g. `QAudioActiveState`, `QtSvgOptions`); unnamed C++
  parameters get names from setters or their type (`QTextCursor::MoveMode`
  gives `moveMode`).
- **Overloads** are named automatically, in this order: overloads identical
  in Go (a `QStringView` twin of a `QString` one) are dropped; plain-number
  twins of a struct overload (`setStart(x, y)` next to `setStart(QPointF)`)
  are dropped; the overload with the fewest parameters gets the plain name;
  the others get a suffix from what differs (`Year()` / `YearWithCalendar`,
  `ToStringFloat64`, `DrawLineSegmentsF`). A new overload never reuses a name
  the class inherits through an override (`QFile.Open` stays the
  `QIODevice` one). Overlay `name:` entries override all of this; whatever
  is still ambiguous gets a number and is listed in `COVERAGE.md`.
- **Coverage report** `COVERAGE.md` (summary table first; it also lists the
  operators, conversions and member templates irgen leaves out): pointer methods on default rules
  and out-parameters not yet confirmed by an `out`/`in-out` rule,
  numbered names, skipped methods (split into missing type support, counted
  by type, and skipped on purpose), rules without a test, and results whose
  death qute can't observe (a plain object from a function with no receiver
  to depend on, unless an overlay marks it `reviewed`), methods whose Full
  form cannot be built (open, or on purpose), the classes Go
  cannot subclass and the virtuals Go cannot override, each with its reason.
  Protected methods and virtual object results on default rules are listed
  once, under the declaring class, and so are object arguments only a Full
  form takes. The summary also
  counts Full forms and plain results tied to their receiver. Signal
  emitters count as reviewed, since emitting never changes ownership.
- **Escape hatches**: `.Ptr()`, `XFromPtr(p, qtcore.Borrowed)`, `XCast(obj)`
  (`qobject_cast`), `XCast(e)` for every event class (`dynamic_cast` from a
  `QEvent`, which is how most events reach an `event`, `eventFilter` or
  `changeEvent` handler; the result depends on `e`, so it is Dead when the
  handler returns, like the event), `XCast(opt)` for every style option and
  style hint result (`qstyleoption_cast`, from the class option `cast:` on
  `QStyleOption` and `QStyleHintReturn`; a style override's option is often
  a derived one, and the cast is Dead when the override returns), and hand-written `*_manual.{go,cpp,h}` files the
  generator never touches (`QObject.MetaObject` is one: read-only, because
  the generator skips the moc virtual `metaObject` everywhere;
  `QXmlStreamAttributes.Size` and `At` are two more: the list part of that
  class is a template base the IR does not record).

## Errors and ergonomics

- Programmer errors panic with what happened, the rule and the fix: dead
  handle, wrong thread, `Delete()` on Borrowed, nil receiver.
- Runtime failures return `error` where the overlay sets `error: true`
  (`QImage.Load`, `QPixmap.Save`).
- One obvious way: redundant overloads (int-coordinate twins of
  `QRect`/`QPoint` forms, `GlobalColor` twins) are skipped in overlays.
- Godoc on every generated method: the C++ signature, its ownership rule and
  a Qt docs link. Qt's summary text isn't included (no offline doc index was
  available).

## Built for AI-authored apps

- `qute.Screenshot(w)` returns an image under the offscreen platform;
  `qute.SaveScreenshot`; `go run ./examples/hello -screenshot out.png`.
- `qute.DumpTree(w)`: class, objectName, ownership, pins, anchoring
  (`root` or `global`, see Pinning and anchoring), geometry as JSON.
- `qutetest.Click`, `ClickAt`, `DoubleClick`, `Type` (handles non-ASCII), `Key`,
  `Wait`, over QTest, and `qute.ProcessEvents`.
- `qutetest.TestMain(m)` and `qutetest.Run(t, f)` run tests on the GUI thread,
  offscreen; `qutetest.CollectGarbage()` / `qutetest.Freed()` check that GC frees.
  `TestMain` turns off Qt's quit-on-last-window-closed, so a test that closes
  or deletes its last visible window does not stop the event loop under the
  tests after it (`qutetest/window_test.go`).
  Inside `Run`, a panic in a signal, virtual or callback handler is recovered
  at the cgo boundary (so Qt's C++ frames return normally), re-raised by the
  input helpers and `ProcessEvents`/`Wait`, and fails the test. Under
  `qute.Main` a handler panic still crashes the program, unless it ran inside
  `PostSync`, which re-raises it in its caller. A recovered panic leaves Qt a
  sensible result: an overridden virtual that panicked runs the C++
  implementation, and a `std::function` callback returns its zero value.
  `t.Fatal`, `t.FailNow` and `t.Skip` inside `Run` end the test (see
  Threading).
- `qute vet`: owned-leak (also a parentless Go subclass never deleted; a
  parent is any non-nil object argument; `Show` counts as keeping a window
  for the program's life, `Exec` does not; a variable handed on, also as
  one of its embedded fields, `return s.QSplitter`, is taken to be parented
  where it goes), delete-handler (QObjects only,
  as rule 5: a type with `AsQObject`), gui-goroutine
  (also the inspection helpers of `qute` and the input helpers of
  `qutetest`), scoped-escape. A handler is an `On*` closure or an override: a
  method of a type that embeds a Base (directly or through a type it embeds)
  whose name ends in `Event` or that takes a scoped argument (`Paint` of a
  graphics item or delegate). A function passed to `qtgui.PaintOn` is
  checked for scoped-escape too (its painter is scoped); it runs at once,
  so a handler's event used inside it does not escape.
- `qute new app <dir> [module]`: main.go, a window, a behaviour test and a
  screenshot test.

## Testing

- Every transfer rule gets a generated test (`*/ownership_gen_test.go`):
  create, transfer, delete the owner, assert Dead. Return rules are tested by
  calling the method and checking the result's state; borrowed object
  arguments are passed as nil. Keep rules get a `TestKeep_` test that the
  reference is held (or released). Rules the generator can't
  set up take a `test:` snippet (Go code run inside `qutetest.Run`), also on
  constructors. Tests that need a GL context skip with the reason when the
  driver can't provide one.
  `TestLayout` checks every primary base sits at offset 0.
- `lsan.supp` suppresses memory that GL and video drivers, FFmpeg and
  Chromium (`libQt6WebEngineCore`) keep until exit (third-party libraries
  only), for `LSAN_OPTIONS=suppressions=$PWD/lsan.supp`. On a CUDA machine
  set `QT_FFMPEG_DECODING_HW_DEVICE_TYPES=,` and
  `QT_FFMPEG_ENCODING_HW_DEVICE_TYPES=,` for `-asan`, and
  `QTWEBENGINE_DISABLE_SANDBOX=1` when running as root (a container).
- `opengl_test.go` draws a triangle (shader program, buffer, VAO) and checks
  the pixels; `scene_test.go` checks scene items die with the scene, with
  `Clear()` and with their parent item.
- `lifetime_test.go` covers object lifetimes that went wrong once: layout
  items and paint devices after their owner died, a QObject through its
  second base, visible `QWindow`s, windows kept alive by their handlers,
  subclass children or kept models, orphaned layout widgets, image memory,
  override results, and misuse that used to crash: the memory path freeing a
  painter's image first, `Delete()` of a guarded kept object or of a layout
  item in a layout, `Delete()` inside a handler, a transfer-to-go result Qt
  had parented, a state guard left on a handler's scoped painter, a signal
  mapper's object collected while mapped. The newer modules carry their own lifetime tests:
  `qtsvg/render_test.go`, `qtmultimedia/lifetime_test.go` (kept outputs
  through GC, `QVideoSink` death, frame memory) and
  `qtwebenginewidgets/lifetime_test.go` (a custom-scheme reply device kept
  through GC, pages and profiles collected together; it registers its scheme
  in `init`, before the application exists). Each newer module also has a
  behaviour test: `qtsvgwidgets/render_test.go` (a widget and a scene item
  draw the SVG), `qtmultimedia` (a sink delivers real frames, outputs and
  sinks share one kept slot), `qtmultimediawidgets/videowidget_test.go`,
  `qtopenglwidgets/lifetime_test.go` and `qtwebenginecore/behaviour_test.go`
  (the cookie filter on a local HTTP server, its panic recovery, builder
  chaining, a guarded profile, a Go page in its own profile, a script run
  with a nil callback). Behaviour tests cover the later qtcore,
  qtgui and qtwidgets classes: `qtcore/{animation,system,data,objects}_test.go`
  (animations, files, processes, command lines, MIME, XML and CBOR streams,
  meta-objects, proxy models, socket notifiers), `qtgui/undo_test.go`
  and `qtgui/newclasses_test.go` (drags, file system model, writers, the
  event casts on events Qt really sends), `qtwidgets/newclasses_test.go`
  (wizard, column view, mapper, anchors, scroller and its events, style
  painter); `subclass_test.go` drives a Go icon engine and a Go gesture
  recognizer. `bench_test.go` measures the reparenting walk
  (linear in the subtree). `run_test.go` covers `t.Fatal` inside `Run` (in a
  child process), `PostSync` handler panics, the off-thread checks (Qt calls
  and handler entries) and Full forms.
- There is no CI workflow. Before tagging, run the commands under "Working
  in the repo": the tests, `qute_debug`, `-asan`, `GOEXPERIMENT=cgocheck2`,
  `gofmt -l`, `qute vet ./examples/...` and a regenerate-and-diff check.

## Roadmap

1. Runtime: done.
2. Widgets: done for 553 classes in eleven modules, including OpenGL,
   SVG, multimedia and the web engine. No method waits for type support,
   every ownership rule is reviewed, every overload has a proper name, every
   out-parameter is confirmed by a rule, every method with defaulted
   arguments has its Full form unless COVERAGE.md says why not, and all
   rules but `QWebEngineUrlRequestJob::reply`, `QSyntaxHighlighter::setCurrentBlockUserData`,
   `QIconEngine::clone` and `QGestureRecognizer::create` have a generated test
   (`reply` is covered by `qtwebenginewidgets/lifetime_test.go`;
   `setCurrentBlockUserData`, valid only inside `highlightBlock`, and the
   two abstract classes' results, which need a Go override, by
   `subclass_test.go`).
3. Subclassing and models: done. Every class with virtuals is subclassable
   by default (237 classes; `COVERAGE.md` lists the 31 that are not, and
   the virtuals Go cannot override).
4. Beyond: `QThread` affinity (one cleanup queue per thread), QtQuick/QML,
   more modules (QtNetwork, QtPrintSupport and QtWebChannel would fill the
   gaps the web engine's skipped methods name), Qt's summary text in doc comments, then macOS and Windows.

Known limitations:

- A non-QObject that Qt itself creates and frees (with no notifying
  subclass qute could substitute) can't report its death. Handed out by a
  method, it depends on the receiver (and a `QLayoutItem` on its widget), so
  the usual ways it dies are caught; one Qt frees while both still live
  (`QLayout::removeWidget` deleting the item) is not. Functions with no
  receiver are listed in `COVERAGE.md` under "Results whose death qute cannot
  see" (none today).
- A Go override never runs from a C++ destructor (the object is already
  Dead to Go), so a Go `QLayout` subclass cannot delete its items there as a
  C++ layout does: the `QLayoutItem`s it holds outlive it (its widgets are
  still freed by their parent). Take them out with `TakeAt` and `Delete()`
  them before deleting such a layout.
- When GC frees a window, Go handlers of its children no longer run, even
  ones Qt would call while the tree is being destroyed (`OnDestroyed`): their
  closures were collected with the window. `Delete()` the window to have them
  run.
- A virtual or signal that Qt calls between GC collecting a tree and the
  GUI thread deleting it is skipped (the virtual runs its C++
  implementation); arguments the shim converted for it are not freed.
- The application object is never destroyed, and cleanups still queued when
  the event loop exits are dropped (deleting after the application is gone
  crashes Qt), so no Qt destructor runs at exit. Objects whose teardown
  matters (a web engine profile flushing storage) must be deleted while the
  loop runs; the README says so.
- `QIODevice` is not subclassable, and `readData` cannot be overridden on
  its subclasses (`QBuffer`, `QFile`), so there are no Go streams (pull-mode
  audio, custom media sources, streaming scheme replies). Qt Multimedia and
  the web engine read devices on their own threads, so one would need a
  thread-safe hand-written binding like the cookie filter.
- operators, conversion operators and member templates are left out by irgen
  (it records them in the IR's `dropped` lists); COVERAGE.md counts and lists
  them. Conversions to `QVariant` are covered by `qtgui.NewQVariantOf`.

Open questions: is one eager `destroyed` connection per wrapped QObject cheap
enough, or should it be lazy? The first build takes about two minutes; would
prebuilt shim archives help? Static linking and Qt's LGPL terms for
distributed binaries. Could miqt's IR or overlays save review time?

## Working in the repo

```sh
go test ./...                                   # all tests, headless
go test -tags qute_debug ./...                  # debug runtime checks
LSAN_OPTIONS=suppressions=$PWD/lsan.supp go test -asan . ./qtcore ./qtgui ./qtwidgets ./qtopengl ./qtopenglwidgets ./qtsvg ./qtsvgwidgets ./qtmultimedia ./qtmultimediawidgets ./qtwebenginecore ./qtwebenginewidgets ./qutetest
GOEXPERIMENT=cgocheck2 go test . ./qtcore ./qtgui ./qtwidgets ./qtopengl ./qtopenglwidgets ./qtsvg ./qtsvgwidgets ./qtmultimedia ./qtmultimediawidgets ./qtwebenginecore ./qtwebenginewidgets ./qutetest
go generate ./qtcore                            # = go run ./cmd/qutegen -root .
go vet ./... && gofmt -l .
staticcheck ./...                               # optional; staticcheck.conf allows `this` receivers
go run ./examples/hello -screenshot out.png     # render offscreen, then Read the PNG
python3 tools/irgen/irgen.py                    # only for a new Qt version (needs python3-clang)
```

- Go 1.26 is the minimum: `golang.org/x/tools` (used by `qute vet`) needs it,
  and `go mod tidy` raises the `go` line back if it is lowered. Keep the
  `go.mod` that `qute new` writes in step.
- A cold build compiles ~22k lines of C++ for qtwidgets alone, about two
  minutes for everything on 8 cores; `go test ./qtwidgets` takes about a
  minute more. Use long timeouts.
- To debug a C++ crash: `go test -c -o x.test ./pkg`, then
  `QT_QPA_PLATFORM=offscreen gdb -batch -ex run -ex bt --args ./x.test -test.run Name`.
  `qutetest.Run` failures caused by runtime errors include the Go stack.

Layout:

- Root package `qute`: `qute.go`, `tools.{go,cpp,h}`, tests
  (`tools_test.go` covers the inspection helpers, `SaveScreenshot`,
  `Keep`/`Unkeep` and `ProcessEvents`;
  `regression_test.go` covers bindings whose signature was once wrong:
  out-parameters, kept references, in-place virtual arguments, handler panics,
  the object a `destroyed` handler is given;
  `subclass_test.go` covers subclassing by default: a view's events and
  protected enums, in-out validator overrides and their Super, map results,
  a Go graphics item, a syntax highlighter's per-block Go state, `GoValue`,
  an abstract class with no overrides, a required override left out, and
  overrides reached through an embedded user type).
- `qutetest`: `qutetest.go` (TestMain, Run, CollectGarbage, Freed) and
  `input.{go,cpp,h}` (the QTest input helpers, tested in `input_test.go`).
- `internal/rt/rt_test.go` and `cmd/qutegen/gen_test.go` test the runtime and
  the generator directly (no QApplication; the generator test loads the real
  spec). The runtime tests use fake addresses and stub the Qt hooks
  (`rootOfC`, `subtreeC`, `hasParentC`), which is how the queue, keep and
  anchoring edge cases (a nested drain, a re-wrapped keeper) are reached.
- `internal/rt`: runtime (hand-written): `handle.go` (states, registry,
  wrapping), `anchor.go` (Go state held by tree roots), `keep.go` (kept
  references, depends-on), `adopt.go` (parentless transfers), `memory.go`
  (value-object bytes), `inline.go` (inline values), `dying.go` (objects
  wrapped while Qt destroys them), `queue.go` (cleanup queue, Post/PostSync),
  `callbacks.go`, and `rt.cpp` (the application event filter for window
  pins and reparenting).
- `qtcore`, `qtgui`, `qtwidgets`, `qtopengl`, `qtopenglwidgets`, `qtsvg`,
  `qtsvgwidgets`, `qtmultimedia`, `qtmultimediawidgets`, `qtwebenginecore`,
  `qtwebenginewidgets`: **generated**. Never edit `*_gen.*`,
  `qute_common.h`, `ownership_gen_test.go` or `export_gen_test.go`; change
  the generator or spec and regenerate. Hand-written code is in `*_manual.*`.
- `cmd/qutegen`: `types.go` (type mapping), `model.go` (resolution, naming,
  rules, out-parameters, pairs, fields), `emit.go` (Go/C/C++ output),
  `tests.go`, `report.go`.
- `qtgui/paint_manual_test.go` covers `PaintOn` (drawing, the device
  guard, a panic in its function, a state guard left on its painter), and
  `regression_test.go` calls a virtual through every secondary-base upcast.
- `features_test.go` checks each extra type kind end to end (out-parameters,
  chaining, buffers, maps, durations, fields, callbacks, GUI values in
  variants, images from bytes, defaulted out-parameters in Full forms).
- `events_test.go` sends and posts events made in Go (key, mouse, custom),
  checks an override's view of one is scoped while the event lives on, and
  that dropped events are collected. `qtwidgets/dialogs_test.go` answers the
  static file and input dialogs (`QFileDialogGetOpenFileNameFull`,
  `QInputDialogGetTextFull`) from a timer.
- `bench_test.go` also measures painters (`BenchmarkPainter`:
  `NewQPainterOn`/`End`/`Delete` against `PaintOn`, on an image and in a
  paint event), a process-list tree (`BenchmarkTreeRows`), and a Go table
  model under a view
  (`BenchmarkModelSelectAll`, `BenchmarkModelPaint`): Qt calls its overrides
  with an index for every row; and rows of list items and of widgets made
  and destroyed (`BenchmarkListRows`, `BenchmarkWidgetRows`).
- `keepalive_test.go` runs the GC inside a call (nested event loop, a
  synchronous event) and checks the receiver and arguments survive it.
- Inside `qutetest.Run`, prefer `t.Error` and `return`. `t.Fatal` works (it ends
  the test), but each one ends the GUI goroutine, which the runtime replaces
  with a nested event loop.
- `cmd/qute`: `vet` and `new`. `new_test.go`
  checks that `qute new` writes the starter app's files and that they parse;
  it builds nothing (tests here test the library, not apps built on it).
  `testdata/bad/bad.go` holds `// want <rule>`
  comments that `vet_test.go` checks.

Invariants:

- Every wrapper struct starts with `rt.Ref` (roots hold it; derived types
  embed their primary base first), and code reinterprets wrapper pointers as
  `*rt.Ref`.
- Only C types cross cgo. Each generated package exports its own
  `qute_<pkg>_signal/release/virtual/dead/call` callbacks; C code never calls a
  function exported by another Go package.
- All Qt calls happen on the GUI thread; only `rt.Drain` deletes for GC.
- Generation is idempotent and warning-free. Unmatched overlay keys warn.

Conventions:

- Fixing a binding: check Qt's docs for who owns what, edit the overlay,
  `go generate ./qtcore`, run that module's tests. The generated tests have
  caught wrong rules (e.g. `QDateTimeEdit::setCalendarWidget` does not take
  ownership).
- Per-method decisions (ownership, names, skips) go in overlays, never in
  generator code; `deliberate` in `cmd/qutegen/types.go` holds only
  type-wide reasons for leaving something out. A `skip:` string is the reason
  shown in `COVERAGE.md`.
- Panic messages name the method, what happened and the fix; keep that style.
- A new exported helper in the root package or a `*_manual.go` file gets a
  test (generated code is covered by `ownership_gen_test.go` and
  `features_test.go`).
- After generator changes: regenerate, `go vet ./...`, `gofmt -l .`, and the
  affected tests.
- Every fix and behaviour change lands with a test that guards it against
  regression (runtime, generator in `gen_test.go`, spec rules, hand-written
  bindings); where practical, check the test fails without the fix.
- Ask before committing. Commit messages end with the attribution line the
  harness supplies.
