// Package rt is the qute runtime. It is the only layer that decides who frees
// a Qt object: every wrapped C++ object has exactly one Handle, and the
// handle's ownership state changes only through the transitions in this file.
package rt

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
	"weak"
)

// State is the ownership state of a handle.
type State uint32

const (
	// Owned objects are freed by Go: by a GC cleanup or by Delete().
	Owned State = iota
	// Parented objects are freed by their Qt parent or container.
	Parented
	// Borrowed objects belong to someone else; Go never frees them.
	Borrowed
	// Dead objects have already been destroyed.
	Dead
)

func (s State) String() string {
	switch s {
	case Owned:
		return "Owned"
	case Parented:
		return "Parented"
	case Borrowed:
		return "Borrowed"
	case Dead:
		return "Dead"
	}
	return fmt.Sprintf("State(%d)", uint32(s))
}

// Kind says how the runtime tracks an object's lifetime.
type Kind uint8

const (
	// KindQObject objects are registered and watched through QObject::destroyed.
	KindQObject Kind = iota
	// KindNotify objects are registered; a shim destructor reports their death.
	KindNotify
	// KindPlain objects have no death notice, so they are never registered and
	// every wrap yields a fresh handle.
	KindPlain
	// KindValue objects are heap copies of Qt value classes, always owned by
	// the handle that created them.
	KindValue
)

// Handle is the Go-side identity of one C++ object. It holds what every
// handle needs; the state only some objects have (pins, connections, kept
// references, anchoring, a Go subclass's slot, debug call sites) is in its
// handleExt, made the first time it is needed. A value handed to a model's
// override for every row then costs a small handle.
type Handle struct {
	ptr     unsafe.Pointer // registry key: QObject* or the object pointer
	class   string
	kind    Kind
	inline  bool      // an inline value: its bytes are in Go memory (inline.go)
	scoped  bool      // a scoped argument: Dead when its callback returns (keepMu)
	note    deathCode // why it died, when the reason is a common one (h.mu)
	state   atomic.Uint32
	pins    atomic.Int32
	serial  uint64
	deleter func(unsafe.Pointer)

	mu         sync.Mutex
	hasCleanup bool
	cleanup    runtime.Cleanup
	cache      []any
	cache1     [1]any // cache's first element, which most handles only need

	// deps are the objects this one only makes sense with (a QModelIndex's
	// model); set once at creation, so Use reads them without a lock.
	deps []*Handle
	dep1 [1]*Handle // deps' first element, all most values have (addDep)

	ext atomic.Pointer[handleExt]
}

// handleExt is the state of a handle that only some objects have. Each field
// keeps the lock it had on Handle.
type handleExt struct {
	// guarded by Handle.mu
	visiblePin bool
	keepPins   int32
	selfSlot   uintptr                               // the id the shim dispatches virtuals with
	deathNote  string                                // why it died, when note cannot say (noteOther)
	callbacks  map[uintptr]func(args unsafe.Pointer) // closures of the connections it sends
	memBytes   int64                                 // external memory counted for this value (memory.go)

	keeps map[string]any // kept references (guarded by keepMu)
	// release drops what an unregistered keeper holds once its handle is
	// collected, while Go does not free it (guarded by keepMu, see keep.go)
	release    runtime.Cleanup
	hasRelease bool

	// scope binding (keep.go), guarded by keepMu
	scope *Handle   // on a keeper: the scoped argument it must not outlive
	bound []*Handle // on a scoped argument: the keepers bound to it, oldest first

	// anchoring (anchor.go), guarded by anchorMu
	anchorN  int32                // reasons to hold the Go state: connections, Self, keeps
	placed   bool                 // held by holder, or globally when holder is nil
	holder   *Handle              // the root handle holding this one
	anchored map[*Handle]struct{} // handles this root holds

	// self is the Go value that overrides virtuals, vtable its generated
	// dispatch table (set once by SetSelf, before Qt can dispatch)
	self   any
	vtable []Virtual

	created string // call site of creation (debug builds)
	died    string // call site of death (debug builds)
}

// memBytesNow is the external memory counted for the handle (memory.go),
// read without making its extension.
func (h *Handle) memBytesNow() int64 {
	if e := h.ext.Load(); e != nil {
		return e.memBytes
	}
	return 0
}

// x returns the handle's extension, making it the first time.
func (h *Handle) x() *handleExt {
	if e := h.ext.Load(); e != nil {
		return e
	}
	h.ext.CompareAndSwap(nil, &handleExt{})
	return h.ext.Load()
}

// Ref is the only field of every generated wrapper root struct. All wrapper
// structs in the primary-base chain start with it, so a pointer to any wrapper
// can be reinterpreted as *Ref.
type Ref struct {
	H *Handle
	P unsafe.Pointer // pointer to the C++ object as this wrapper's class
}

// RefOf reinterprets a pointer to a generated wrapper struct as its Ref.
func RefOf(wrapper unsafe.Pointer) *Ref { return (*Ref)(wrapper) }

var serialCounter atomic.Uint64

type regEntry struct {
	w       weak.Pointer[Handle]
	serial  uint64
	watched bool
	// prevSerial is the serial of a collected handle this entry's handle
	// replaced; a queued free of that one hands the object to the new handle.
	prevSerial uint64
}

var (
	regMu    sync.Mutex
	registry = map[unsafe.Pointer]regEntry{} // by value: no allocation per object
	strongMu sync.Mutex
	strong   = map[*Handle]struct{}{}
)

// State returns the handle's current ownership state.
func (h *Handle) State() State {
	if h == nil {
		return Dead
	}
	return State(h.state.Load())
}

// Class is the C++ class name the handle was created with.
func (h *Handle) Class() string { return h.class }

// Alive reports whether the C++ object still exists: it is not Dead, and no
// object it depends on (the model of an index, the layout of an item) is.
func (h *Handle) Alive() bool {
	if h == nil || h.State() == Dead {
		return false
	}
	for _, d := range h.deps {
		if d.State() == Dead {
			return false
		}
	}
	return true
}

// Lookup returns the live handle registered for key, or nil.
func Lookup(key unsafe.Pointer) *Handle {
	regMu.Lock()
	defer regMu.Unlock()
	if e, ok := registry[key]; ok {
		return e.w.Value()
	}
	return nil
}

// WrapMode says what to do with the state of an already-wrapped object.
type WrapMode uint8

const (
	// KeepOrBorrow keeps an existing handle's state; new handles are Borrowed.
	KeepOrBorrow WrapMode = iota
	// TakeOwned forces Owned (transfer-to-go returns and constructors).
	TakeOwned
	// TakeParented forces Parented.
	TakeParented
	// NewScoped makes an unregistered handle that dies when the callback returns.
	NewScoped
)

// Wrap returns the handle for the C++ object at key, creating it if needed.
// Wrapping the same pointer twice returns the same handle, so its state never
// diverges.
func Wrap(key unsafe.Pointer, class string, kind Kind, mode WrapMode, deleter func(unsafe.Pointer)) *Handle {
	return wrap(key, class, kind, mode, deleter, nil)
}

// wrap is Wrap; a new handle is made in fresh when it is not nil (WrapNew).
func wrap(key unsafe.Pointer, class string, kind Kind, mode WrapMode, deleter func(unsafe.Pointer), fresh *Handle) *Handle {
	if key == nil {
		return nil
	}
	if mode == NewScoped {
		h := newHandle(key, class, KindPlain, Borrowed, deleter)
		h.scoped = true
		return h
	}
	if kind == KindPlain || kind == KindValue {
		st := mode.state()
		if kind == KindPlain && st == Owned && deleter == nil {
			st = Borrowed
		}
		h := newHandleIn(fresh, key, class, kind, st, deleter)
		h.attachDependency()
		if st == Owned {
			h.attachCleanup()
		}
		return h
	}

	regMu.Lock()
	e, found := registry[key]
	if found {
		if h := e.w.Value(); h != nil && h.State() != Dead {
			regMu.Unlock()
			h.retake(mode)
			return h
		}
	}
	if kind == KindQObject && mode == KeepOrBorrow && isDying(key) {
		// the object is being destroyed: nothing may be registered (dying.go)
		h := wrapDyingLocked(key, class)
		regMu.Unlock()
		return h
	}
	st := mode.state()
	if st == Owned && kind == KindQObject && AppExists() && HasParent(key) {
		st = Parented // a transfer-to-go result Qt gave a parent (see TransferToGo)
	}
	h := newHandleIn(fresh, key, class, kind, st, deleter)
	needWatch := kind == KindQObject
	if found {
		needWatch = needWatch && !e.watched
		registry[key] = regEntry{w: weak.Make(h), serial: h.serial, watched: e.watched || needWatch, prevSerial: e.serial}
	} else {
		registry[key] = regEntry{w: weak.Make(h), serial: h.serial, watched: needWatch}
	}
	delete(dying, key) // a new object at the address of one destroyed earlier
	regMu.Unlock()
	if needWatch {
		watch(key)
	}
	if kind == KindNotify {
		h.attachDependency()
	}
	if st == Owned {
		h.attachCleanup()
	}
	return h
}

// state is the state a new handle gets for mode.
func (mode WrapMode) state() State {
	switch mode {
	case TakeOwned:
		return Owned
	case TakeParented:
		return Parented
	}
	return Borrowed
}

// retake applies a forcing mode to an existing handle.
func (h *Handle) retake(mode WrapMode) {
	switch mode {
	case TakeOwned:
		h.TransferToGo()
	case TakeParented:
		h.TransferToCpp()
	}
}

func newHandle(key unsafe.Pointer, class string, kind Kind, st State, deleter func(unsafe.Pointer)) *Handle {
	return newHandleIn(nil, key, class, kind, st, deleter)
}

// newHandleIn sets up a new handle in h, or in a new one when h is nil.
func newHandleIn(h *Handle, key unsafe.Pointer, class string, kind Kind, st State, deleter func(unsafe.Pointer)) *Handle {
	if h == nil {
		h = new(Handle)
	}
	h.init(key, class, kind, st, deleter)
	return h
}

// init sets up a new handle in place (newHandle, and inline values made
// together with their wrapper).
func (h *Handle) init(key unsafe.Pointer, class string, kind Kind, st State, deleter func(unsafe.Pointer)) {
	h.ptr, h.class, h.kind, h.deleter, h.serial = key, class, kind, deleter, serialCounter.Add(1)
	h.state.Store(uint32(st))
	if debugEnabled {
		h.x().created = callSite()
	}
}

// addDep adds an object h depends on; the first needs no allocation. Only
// while h is new and not yet shared, as Use reads deps without a lock.
func (h *Handle) addDep(d *Handle) {
	if h.deps == nil {
		h.deps = h.dep1[:0]
	}
	h.deps = append(h.deps, d)
}

// View returns the cached wrapper of type T for (h, p), creating it once.
func View[T any](h *Handle, p unsafe.Pointer) *T {
	if h == nil || p == nil {
		return nil
	}
	if h.inline {
		p = h.ptr // the Go copy, not the C++ object it was copied from
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.cache) == 0 {
		// the first wrapper (most handles have only one), with no slice to
		// allocate: cache starts on the array inside the handle
		v := new(T)
		*RefOf(unsafe.Pointer(v)) = Ref{H: h, P: p}
		h.cache1[0] = v
		h.cache = h.cache1[:1]
		return v
	}
	for _, c := range h.cache {
		if v, ok := c.(*T); ok && RefOf(unsafe.Pointer(v)).P == p {
			return v
		}
	}
	// An upcast of a more-derived wrapper returns its embedded base, so
	// label.AsQWidget() and a QWidget* returned by Qt are the same Go pointer.
	want := reflect.TypeFor[T]()
	for _, c := range h.cache {
		if RefOf(reflect.ValueOf(c).UnsafePointer()).P != p {
			continue
		}
		f := reflect.ValueOf(c).Elem()
		for f.Kind() == reflect.Struct && f.NumField() > 0 && f.Type().Field(0).Anonymous {
			f = f.Field(0)
			if f.Type() == want {
				return (*T)(f.Addr().UnsafePointer())
			}
		}
	}
	v := new(T)
	*RefOf(unsafe.Pointer(v)) = Ref{H: h, P: p}
	h.cache = append(h.cache, v)
	return v
}

// TransferToCpp marks the object as freed by its Qt parent or container and
// cancels its GC cleanup.
func (h *Handle) TransferToCpp() {
	if h == nil {
		return
	}
	for {
		s := h.State()
		if s == Dead || s == Parented {
			return
		}
		if h.state.CompareAndSwap(uint32(s), uint32(Parented)) {
			h.stopCleanup()
			if s == Owned {
				h.releaseAnchored() // a Parented handle may be collected: it holds nothing
			}
			return
		}
	}
}

// TransferToGo makes Go responsible for freeing the object again. A QObject
// that has a Qt parent stays (or becomes) Parented instead: its parent frees
// it, whatever the rule said (the safety net of rule 7, applied up front).
func (h *Handle) TransferToGo() {
	if h == nil {
		return
	}
	if h.kind == KindQObject && h.State() != Dead && AppExists() && HasParent(h.ptr) {
		h.TransferToCpp()
		return
	}
	for {
		s := h.State()
		if s == Dead || s == Owned {
			return
		}
		if h.deleter == nil {
			return
		}
		if h.state.CompareAndSwap(uint32(s), uint32(Owned)) {
			h.attachCleanup()
			if h.kind == KindQObject && subtreeC != nil && !HasParent(h.ptr) {
				reanchorSubtree(h.ptr, false) // a new root: it holds its tree's Go state
			}
			return
		}
	}
}

// SetParented applies a transfer-this rule: Parented when the new parent is
// non-nil, Owned when it is nil.
func (h *Handle) SetParented(hasParent bool) {
	if hasParent {
		h.TransferToCpp()
	} else {
		h.TransferToGo()
	}
}

type cleanupArg struct {
	key        unsafe.Pointer
	serial     uint64
	class      string
	kind       Kind
	deleter    func(unsafe.Pointer)
	registered bool
	quiet      bool  // not an object handle: not counted in Drained
	mem        int64 // external memory freed with it (memory.go)
}

func (h *Handle) attachCleanup() {
	h.mu.Lock()
	if h.hasCleanup || h.deleter == nil {
		h.mu.Unlock()
		return
	}
	arg := cleanupArg{key: h.ptr, serial: h.serial, class: h.class, kind: h.kind, deleter: h.deleter,
		registered: h.registeredKind(), mem: h.memBytesNow()}
	h.cleanup = runtime.AddCleanup(h, enqueueCleanup, arg)
	h.hasCleanup = true
	h.mu.Unlock()
	h.syncRelease() // keepMu comes before h.mu, so only once h.mu is free
}

// hasCleanupNow reports whether Go will free the object through its GC cleanup.
func (h *Handle) hasCleanupNow() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hasCleanup
}

func (h *Handle) stopCleanup() {
	h.mu.Lock()
	stopped := h.hasCleanup
	if stopped {
		h.cleanup.Stop()
		h.hasCleanup = false
	}
	h.mu.Unlock()
	if stopped {
		h.syncRelease()
	}
}

// Pin holds the handle strongly so GC cannot free it or its Go state. The
// count and the strong set change under one lock, so they always agree.
func (h *Handle) Pin() {
	strongMu.Lock()
	if h.pins.Add(1) == 1 {
		if testHookPin != nil {
			testHookPin() // between the count and the set (tests only)
		}
		strong[h] = struct{}{}
	}
	strongMu.Unlock()
}

// testHookPin runs inside Pin after the count changed and before the strong
// set does, where a racing Unpin used to slip in (tests only).
var testHookPin func()

// Unpin releases one Pin.
func (h *Handle) Unpin() {
	strongMu.Lock()
	if n := h.pins.Add(-1); n == 0 {
		delete(strong, h)
	} else if n < 0 {
		h.pins.Store(0)
	}
	strongMu.Unlock()
}

// Pins returns the current pin count.
func (h *Handle) Pins() int { return int(h.pins.Load()) }

// Keep pins the object explicitly until Unkeep.
func (h *Handle) Keep() {
	if !h.Alive() {
		return
	}
	h.mu.Lock()
	h.x().keepPins++
	h.mu.Unlock()
	h.Pin()
}

// Unkeep releases one Keep.
func (h *Handle) Unkeep() {
	h.mu.Lock()
	e := h.ext.Load()
	if e == nil || e.keepPins == 0 {
		h.mu.Unlock()
		return
	}
	e.keepPins--
	h.mu.Unlock()
	h.Unpin()
}

// SetVisiblePin pins or unpins the handle for being a visible top-level window.
func (h *Handle) SetVisiblePin(visible bool) {
	h.mu.Lock()
	if h.x().visiblePin == visible {
		h.mu.Unlock()
		return
	}
	h.x().visiblePin = visible
	h.mu.Unlock()
	if visible {
		h.Pin()
	} else {
		h.Unpin()
	}
}

// SetSelf attaches a Go subclass value and its dispatch table, anchors the
// handle for as long as the object lives (globally for a parentless object,
// else by the root of its tree, see anchor.go), and returns the id the shim
// dispatches with.
func (h *Handle) SetSelf(self any, vt []Virtual) uintptr {
	h.mu.Lock()
	h.x().self = self
	h.x().vtable = vt
	h.x().selfSlot = newSlot(h)
	id := h.x().selfSlot
	h.mu.Unlock()
	// The death notice drops the id. A handle collected first (with its tree,
	// which the GUI thread then deletes) is gone by then, so its id goes with
	// it; the shim finds nothing under it and runs the C++ implementation.
	runtime.AddCleanup(h, dropSlot, id)
	h.anchorPin()
	return id
}

func dropSlot(id uintptr) { slots.Delete(id) }

// GoValue returns the Go value set by SetSelf (a Go subclass), or nil. It
// stays readable after the object dies.
func (h *Handle) GoValue() any {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if e := h.ext.Load(); e != nil {
		return e.self
	}
	return nil
}

// Subclass returns the Go value that overrides virtuals and its dispatch
// table (tests call an override as the shim does).
func (h *Handle) Subclass() (any, []Virtual) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if e := h.ext.Load(); e != nil {
		return e.self, e.vtable
	}
	return nil, nil
}

// Destroyed is called when Qt (or a shim destructor) reports that the object
// at key is gone. The registry entry is removed before the memory is freed, so
// a new object at the same address gets a fresh handle.
func Destroyed(key unsafe.Pointer, why string) {
	regMu.Lock()
	e, found := registry[key]
	delete(registry, key)
	var h *Handle
	if found {
		h = e.w.Value()
		if h != nil && h.kind == KindQObject {
			buryLocked(key, h) // handed back if Qt passes the dying object to Go
		}
	}
	regMu.Unlock()
	if found {
		if h != nil {
			h.markDead(why)
		} else {
			// the handle was collected first (its tree is being freed):
			// what it kept may go now
			releaseHolds(e.serial)
		}
		if e.prevSerial != 0 {
			// and what an earlier handle of the object kept, collected
			// before Qt handed the object out again
			releaseHolds(e.prevSerial)
		}
	}
	orphaned(key)
}

func (h *Handle) markDead(why string) {
	old := State(h.state.Swap(uint32(Dead)))
	if old == Dead {
		return
	}
	h.stopCleanup()
	h.mu.Lock()
	h.setNoteLocked(why)
	if debugEnabled {
		h.x().died = callSite()
	}
	if h.ext.Load() == nil {
		// no pins, connections, keeps, anchoring or memory (the usual
		// case): nothing to release. A keeper always has an extension
		// (slotsFor makes it before holdLocked), so it holds nothing either.
		h.mu.Unlock()
		return
	}
	visible, keeps, id := h.x().visiblePin, h.x().keepPins, h.x().selfSlot
	h.x().visiblePin, h.x().keepPins, h.x().selfSlot = false, 0, 0
	h.mu.Unlock()
	if visible {
		h.Unpin()
	}
	for ; keeps > 0; keeps-- {
		h.Unpin()
	}
	if id != 0 {
		slots.Delete(id)
		h.anchorUnpin()
	}
	h.dropHandleKeeps()
	// What this root held moves to the global set: its children are deleted
	// right after it, and their handlers may still run while they die. Their
	// release callbacks then drop it.
	h.releaseAnchored()
	untrackMemory(h)
}

// Delete frees the object now. It is idempotent and works on Owned and
// Parented objects; on Borrowed objects it panics, and so it does where Qt
// would be left with a dangling pointer: an object a keeper guards (see
// KeepArg), a Parented object of a class whose container does not notice
// (RegisterParentedDelete), a QObject inside a handler (rule 5).
func (h *Handle) Delete(method string) {
	if h == nil {
		return
	}
	switch h.State() {
	case Dead:
		return
	case Borrowed:
		panic(fmt.Sprintf("%s: object is Borrowed, so Go does not own it and may not delete it (rule 4); "+
			"let its owner delete it, or call DeleteLater() on a QObject", method))
	}
	if InCallback() && h.kind == KindQObject {
		panic(fmt.Sprintf("%s: Delete() inside a signal or event handler (rule 5); Qt may still be using the object, "+
			"call DeleteLater() instead", method))
	}
	if keeper := guardingKeeper(h.serial); keeper != "" {
		panic(fmt.Sprintf("%s: a %s still uses this %s, and deleting it would leave Qt with a dangling pointer (rule 4); "+
			"end or delete the %s first, or pass another object or nil to the method that gave it the %s",
			method, keeper, h.class, keeper, h.class))
	}
	if h.State() == Parented {
		if hint := parentedDeleteHint(h.class); hint != "" {
			panic(fmt.Sprintf("%s: the %s belongs to a container that would keep a dangling pointer to it (rule 4); %s",
				method, h.class, hint))
		}
	}
	if h.inline {
		h.markDead("deleted by Delete()") // the GC frees its memory
		return
	}
	if h.deleter == nil {
		panic(fmt.Sprintf("%s: %s has no public destructor", method, h.class))
	}
	h.stopCleanup()
	h.mu.Lock()
	h.setNoteLocked("deleted by Delete()")
	h.mu.Unlock()
	key, del := h.ptr, h.deleter
	if h.registeredKind() {
		del(key) // the death notice marks the handle Dead
		Destroyed(key, "deleted by Delete()")
		h.markDead("deleted by Delete()")
		return
	}
	h.markDead("deleted by Delete()")
	del(key)
}

// Use checks that the wrapper is usable and returns its C++ pointer. It is the
// one atomic load every generated method pays.
func Use(wrapper unsafe.Pointer, method string) unsafe.Pointer {
	if wrapper == nil {
		panic(fmt.Sprintf("%s: called on a nil pointer; create the object with its New function first", method))
	}
	r := (*Ref)(wrapper)
	if r.H == nil {
		panic(fmt.Sprintf("%s: called on a zero-value wrapper; create the object with its New function, "+
			"or for a Go subclass pass it to the NewXWith constructor first", method))
	}
	if State(r.H.state.Load()) == Dead {
		panic(deadMessage(r.H, method))
	}
	checkDependency(r.H, method)
	if debugEnabled {
		GUIOnly(method)
	}
	return r.P
}

// UseArg is Use for an argument that may be nil.
func UseArg(wrapper unsafe.Pointer, method string) unsafe.Pointer {
	if wrapper == nil {
		return nil
	}
	return Use(wrapper, method)
}

// GUIOnly panics when called off the GUI thread, in every build. Generated
// methods check in their C shim instead; hand-written helpers (qutetest.Click,
// qute.Screenshot) call this. Before the application exists nothing is
// checked, as in the shims: there is no GUI thread yet.
func GUIOnly(method string) {
	if AppExists() {
		checkThread(method)
	}
}

// CheckThread panics when called off the GUI thread in debug builds.
func CheckThread(method string) {
	if debugEnabled {
		GUIOnly(method)
	}
}

func deadMessage(h *Handle, method string) string {
	h.mu.Lock()
	why := h.noteLocked()
	var site, created string
	if e := h.ext.Load(); e != nil {
		site, created = e.died, e.created
	}
	if why == "" && h.scoped {
		why = scopedNote
	}
	h.mu.Unlock()
	var msg string
	if why == scopedNote {
		msg = fmt.Sprintf("%s: %s argument used after its callback returned (rule 6); "+
			"copy the values you need inside the handler", method, h.class)
	} else {
		if why == "" {
			why = "destroyed"
		}
		if site != "" {
			why += " at " + site
		}
		msg = fmt.Sprintf("%s: object destroyed (%s); keep its owner alive while you use it, "+
			"or check Alive() first", method, why)
	}
	if created != "" {
		msg += fmt.Sprintf(" [created at %s]", created)
	}
	return msg
}

const scopedNote = "scoped argument expired"

// deathCode is why a handle died, for the common reasons, so recording it
// needs no handleExt: Qt destroys many small objects (list rows) that never
// had any other state.
type deathCode uint8

const (
	noteNone  deathCode = iota
	noteOther           // the reason is in handleExt.deathNote
	noteQt
	noteDelete
	noteGC
	noteScoped
	noteDying
)

var deathNotes = [...]string{
	noteQt:     "destroyed by Qt",
	noteDelete: "deleted by Delete()",
	noteGC:     "freed by GC",
	noteScoped: scopedNote,
	noteDying:  "wrapped while Qt was destroying it",
}

// setNoteLocked records why h died, unless a reason is already recorded (Delete
// names itself before Qt's death notice arrives). The caller holds h.mu.
func (h *Handle) setNoteLocked(why string) {
	if h.note != noteNone || why == "" {
		return
	}
	for c := noteQt; int(c) < len(deathNotes); c++ {
		if deathNotes[c] == why {
			h.note = c
			return
		}
	}
	h.note = noteOther
	h.x().deathNote = why
}

// noteLocked returns why h died, or "". The caller holds h.mu.
func (h *Handle) noteLocked() string {
	if h.note == noteOther {
		return h.ext.Load().deathNote
	}
	return deathNotes[h.note]
}

// Expire marks a scoped argument handle Dead when its callback returns.
// Objects Go built on it inside the handler and left alive (a
// QPainterStateGuard on the handler's painter) end first, while Qt's object
// is still there for their destructors (see endBound).
func (h *Handle) Expire() {
	if h != nil {
		h.endBound()
		h.markDead(scopedNote)
	}
}

// Constructor modes: the pointer is fresh from a Go constructor.
const (
	NewOwned WrapMode = iota + 10
	NewParented
)

type classInfo struct {
	kind     Kind
	deleter  func(unsafe.Pointer)
	toObject func(unsafe.Pointer) unsafe.Pointer
	memory   func(unsafe.Pointer) int64
	// parentedDelete, when set, is the fix Delete() names for a Parented
	// object of this class (see RegisterParentedDelete)
	parentedDelete string
	// inline, when set, is the size of an inline value class (inline.go)
	inline int
}

var classes sync.Map // name -> classInfo

// RegisterClass records how a generated class is tracked and freed.
func RegisterClass(name string, kind Kind, deleter func(unsafe.Pointer)) {
	classes.Store(name, classInfo{kind: kind, deleter: deleter})
}

// RegisterBaseOfQObject records that the plain class name is a base of some
// QObject class (QGraphicsItem of QGraphicsObject, QPaintDevice of QWidget,
// QLayoutItem of QLayout). toObject is a dynamic_cast to QObject: when a
// pointer of the base class is in fact a QObject, it is wrapped with that
// QObject's handle, so it shares the object's state and death notice instead
// of getting a second handle that never learns the object died.
func RegisterBaseOfQObject(name string, toObject func(unsafe.Pointer) unsafe.Pointer) {
	updateClass(name, func(ci *classInfo) { ci.toObject = toObject })
}

// RegisterParentedDelete says that a Parented object of class may not be
// deleted from Go: its container keeps a pointer to it and does not notice
// its destructor (a QSpacerItem in a layout). Delete() then panics with hint,
// which names the way to take it out first.
func RegisterParentedDelete(class, hint string) {
	updateClass(class, func(ci *classInfo) { ci.parentedDelete = hint })
}

func parentedDeleteHint(class string) string {
	if v, ok := classes.Load(class); ok {
		return v.(classInfo).parentedDelete
	}
	return ""
}

// updateClass changes the record of a class RegisterClass already made. It
// runs from the generated init functions, which register a class first.
func updateClass(name string, f func(*classInfo)) {
	v, ok := classes.Load(name)
	if !ok {
		panic("qute: class " + name + " is updated before RegisterClass")
	}
	ci := v.(classInfo)
	f(&ci)
	classes.Store(name, ci)
}

// WrapClass is Wrap for a registered class.
func WrapClass(p unsafe.Pointer, class string, mode WrapMode) *Handle {
	if p == nil {
		return nil
	}
	v, ok := classes.Load(class)
	if !ok {
		panic("qute: class " + class + " is not registered")
	}
	ci := v.(classInfo)
	kind := ci.kind
	if ci.inline > 0 {
		switch mode {
		case CopyInline:
			return newInline(p, class, ci.inline)
		case TakeOwned, NewOwned:
			// a heap copy that is Go's: keep its bytes, free it now
			h := newInline(p, class, ci.inline)
			ci.deleter(p)
			return h
		}
	}
	if mode == NewOwned || mode == NewParented {
		return wrapNew(p, class, ci, mode, nil)
	}
	if kind == KindValue {
		h := Wrap(p, class, kind, mode, ci.deleter)
		trackNew(h, ci)
		return h
	}
	if ci.toObject != nil && mode != NewScoped {
		if q := ci.toObject(p); q != nil {
			// the QObject's own handle; a fresh one carries its real class
			if h := Lookup(q); h != nil {
				h.retake(mode)
				return h
			}
			return Wrap(q, classNameC(q), KindQObject, mode, DeleteQObject)
		}
	}
	if kind == KindNotify && mode != NewScoped {
		// A scoped argument always gets a handle of its own (Wrap), even for
		// an object Go built and holds (an event it sent): expiring it when
		// the handler returns must not kill the caller's handle.
		//
		// Only objects built by our shim constructors report their death, so
		// only those may be registered. Others get a fresh plain handle.
		if h := Lookup(p); h != nil {
			h.retake(mode)
			return h
		}
		kind = KindPlain
	}
	return Wrap(p, class, kind, mode, ci.deleter)
}

// WrapNew is WrapClass for an object a constructor just made (mode NewOwned
// or NewParented): its handle is new, so the handle and its wrapper of type
// T are made in one allocation, and View[T] finds the wrapper cached.
func WrapNew[T any](p unsafe.Pointer, class string, mode WrapMode) *Handle {
	if p == nil {
		return nil
	}
	v, ok := classes.Load(class)
	if !ok {
		panic("qute: class " + class + " is not registered")
	}
	ci := v.(classInfo)
	if ci.inline > 0 || (mode != NewOwned && mode != NewParented) {
		return WrapClass(p, class, mode)
	}
	x := new(viewPair[T])
	h := wrapNew(p, class, ci, mode, &x.h)
	if h == &x.h {
		*RefOf(unsafe.Pointer(&x.v)) = Ref{H: h, P: p}
		h.cache1[0] = &x.v
		h.cache = h.cache1[:1]
	}
	return h
}

// wrapNew wraps an object a constructor just made, in fresh when it is not
// nil and the handle is new (WrapNew), and counts the memory of an Owned one.
func wrapNew(p unsafe.Pointer, class string, ci classInfo, mode WrapMode, fresh *Handle) *Handle {
	if mode == NewParented {
		return wrap(p, class, ci.kind, TakeParented, ci.deleter, fresh)
	}
	h := wrap(p, class, ci.kind, TakeOwned, ci.deleter, fresh)
	trackNew(h, ci)
	return h
}

// WrapScoped makes the handle of an object Go builds for the length of one
// call (the painter qtgui.PaintOn draws with) and its wrapper of type T, in
// one allocation. Like a scoped argument it is Borrowed, so Go code cannot
// delete it, and Expire makes it Dead when the call returns; the caller then
// frees the object. It has no GC cleanup and is never registered.
func WrapScoped[T any](p unsafe.Pointer, class string) *Handle {
	if p == nil {
		return nil
	}
	x := new(viewPair[T])
	h := &x.h
	h.init(p, class, KindPlain, Borrowed, nil)
	h.scoped = true
	*RefOf(unsafe.Pointer(&x.v)) = Ref{H: h, P: p}
	h.cache1[0] = &x.v
	h.cache = h.cache1[:1]
	return h
}

// WrapClassFrom is WrapClass for an object pointer returned by a method of
// owner. When the result is a plain object qute cannot watch (a QLayoutItem
// Qt made, the QPaintDevice a QPainter draws on) and Go does not own it, it
// depends on owner and on whatever owner depends on: Use panics once owner is
// destroyed, instead of following a pointer Qt may have freed with it.
func WrapClassFrom(p unsafe.Pointer, class string, mode WrapMode, owner *Handle) *Handle {
	h := WrapClass(p, class, mode)
	// a value handle Go does not own (a QStyleOptionViewItemCast of a scoped
	// option) is Qt's object too, and dies with owner the same way
	if h == nil || owner == nil || h == owner || (h.kind != KindPlain && h.kind != KindValue) || h.State() == Owned {
		return h
	}
	// a fresh plain handle, not yet shared: its deps may still be set
	h.addDep(owner)
	for _, d := range owner.deps {
		if d != h {
			h.addDep(d)
		}
	}
	return h
}

// ModeFor maps a public ownership request to a wrap mode.
func ModeFor(s State) WrapMode {
	switch s {
	case Owned:
		return TakeOwned
	case Parented:
		return TakeParented
	}
	return KeepOrBorrow
}

// Arg checks a wrapper argument and returns its pointer and handle.
func Arg(wrapper unsafe.Pointer, method string) (unsafe.Pointer, *Handle) {
	if wrapper == nil {
		return nil, nil
	}
	p := Use(wrapper, method)
	return p, (*Ref)(wrapper).H
}

// WrongThread is called by a generated shim that runs off the GUI thread.
func WrongThread() {
	panic("qute: a Qt method was called off the GUI thread; Qt objects may only be used on the GUI thread, " +
		"wrap the call in qute.Post(func() { ... }) (build with -tags qute_debug to have the method named)")
}
