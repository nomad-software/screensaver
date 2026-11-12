package rt

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// Qt often keeps a pointer to an object it does not own: a view's model, a
// stream's device, a button's menu. Go code that drops its last reference to
// such an object would let the GC free it underneath Qt, so the overlay rules
// `keep`, `keep-each` and `release` make the receiver hold a strong reference
// instead (PySide's reference-keeping). A kept reference lasts until the
// receiver dies, the slot is replaced, or a release rule drops it.
//
// The references live on the owner's handle. A registered owner (a QObject
// or notify-dtor object) can outlive every Go variable that refers to it, so
// while it keeps anything its handle is anchored (see anchor.go): held by the
// root of its object tree, or globally. The references then last exactly as
// long as the object, and a window whose view keeps a model that refers back
// to the window can still be collected as a whole.

var keepMu sync.Mutex

// A kept object and its keeper often become unreachable in the same GC cycle
// (a window whose page keeps a QWebEngineProfile), and Go runs their cleanups
// in no particular order. Qt may need the kept object while the keeper is
// destroyed (a page must not outlive its profile), so the cleanup queue
// frees a kept object only once no live keeper holds it (see Drain). Both
// maps are keyed by handle serial, so they outlive the handles: a keeper's
// handle may be collected long before Qt destroys the object.
var (
	keptBy = map[uint64]int32{}  // kept handle serial -> keepers holding it
	holds  = map[uint64][]hold{} // keeper serial -> what it keeps, one per slot

	// A guarded hold is one where Qt would follow a dangling pointer if the
	// kept object were deleted while the keeper lives (a QPainter's device),
	// so Delete() of the kept object panics instead (see guardingKeeper).
	guardedBy   = map[uint64]int32{}  // kept serial -> guarded holds on it
	keeperClass = map[uint64]string{} // keeper serial -> class, for messages
)

type hold struct {
	kept  uint64
	guard bool
}

// holdLocked records that keeper holds kept. The caller holds keepMu.
func holdLocked(keeper *Handle, kept uint64, guard bool) {
	holds[keeper.serial] = append(holds[keeper.serial], hold{kept, guard})
	keptBy[kept]++
	if guard {
		guardedBy[kept]++
		keeperClass[keeper.serial] = keeper.class
	}
}

// dropLocked forgets one hold on kept and reports whether kept is now free
// to go. The caller holds keepMu.
func dropLocked(hd hold) bool {
	if hd.guard {
		if guardedBy[hd.kept]--; guardedBy[hd.kept] <= 0 {
			delete(guardedBy, hd.kept)
		}
	}
	if keptBy[hd.kept]--; keptBy[hd.kept] <= 0 {
		delete(keptBy, hd.kept)
		return true
	}
	return false
}

// unholdLocked drops one hold of keeper on kept and reports whether kept is
// now free to go. The caller holds keepMu.
func unholdLocked(keeper, kept uint64) bool {
	hs := holds[keeper]
	var hd hold
	found := false
	for i, x := range hs {
		if x.kept == kept {
			hd, found = x, true
			hs[i] = hs[len(hs)-1]
			hs = hs[:len(hs)-1]
			break
		}
	}
	if len(hs) == 0 {
		delete(holds, keeper)
		delete(keeperClass, keeper)
	} else {
		holds[keeper] = hs
	}
	if !found {
		return false
	}
	return dropLocked(hd)
}

// releaseHolds drops every hold of a keeper that died or whose handle was
// collected, and wakes the cleanup queue if a deferred free may now run.
func releaseHolds(keeper uint64) {
	keepMu.Lock()
	freed := false
	for _, hd := range holds[keeper] {
		if dropLocked(hd) {
			freed = true
		}
	}
	delete(holds, keeper)
	delete(keeperClass, keeper)
	keepMu.Unlock()
	if freed {
		wakeDeferred()
	}
}

// A lend is a guarded use of an object that lasts one call (the device of
// the painter qtgui.PaintOn draws with): cheaper than a keep, since the
// caller's own reference keeps the object from the GC and the user is
// scoped, so there is nothing to anchor, bind or release later.
type lend struct {
	class string // the C++ user's class, for the panic message
	n     int
}

var lent = map[uint64]lend{} // serial -> lend (keepMu)

// Lend marks obj as used by a C++ object of class user until Unlend: Delete()
// of obj panics meanwhile, naming user, as it does for a guarded keep. Calls
// nest.
func Lend(obj *Handle, user string) {
	if obj == nil {
		return
	}
	keepMu.Lock()
	l := lent[obj.serial]
	l.class, l.n = user, l.n+1
	lent[obj.serial] = l
	keepMu.Unlock()
}

// Unlend ends one Lend of obj.
func Unlend(obj *Handle) {
	if obj == nil {
		return
	}
	keepMu.Lock()
	if l, ok := lent[obj.serial]; ok {
		if l.n <= 1 {
			delete(lent, obj.serial)
		} else {
			l.n--
			lent[obj.serial] = l
		}
	}
	keepMu.Unlock()
}

// guardingKeeper returns the class of a live keeper whose guarded hold on
// the object with serial would make deleting it unsafe, or "".
func guardingKeeper(serial uint64) string {
	keepMu.Lock()
	defer keepMu.Unlock()
	if l, ok := lent[serial]; ok {
		return l.class
	}
	if guardedBy[serial] == 0 {
		return ""
	}
	for keeper, hs := range holds {
		for _, hd := range hs {
			if hd.kept == serial && hd.guard {
				return keeperClass[keeper]
			}
		}
	}
	return "keeper"
}

// isKept reports whether a live keeper still holds the object with serial.
func isKept(serial uint64) bool {
	keepMu.Lock()
	defer keepMu.Unlock()
	return keptBy[serial] > 0
}

func (h *Handle) registeredKind() bool { return h.kind == KindQObject || h.kind == KindNotify }

// slotsFor returns the keep slots of owner, creating them if create is set.
// The caller holds keepMu.
func slotsFor(owner *Handle, create bool) map[string]any {
	if owner.x().keeps == nil && create {
		owner.x().keeps = map[string]any{}
		if owner.registeredKind() {
			owner.anchorPin()
		} else {
			owner.syncReleaseLocked()
		}
	}
	return owner.x().keeps
}

// syncReleaseLocked keeps the release cleanup of an unregistered keeper in
// step with who frees it. When Go frees it (it has a GC cleanup), its free
// releases what it holds, after the keeper is gone. When Go does not (it is
// Borrowed, or Parented after a transfer), nothing would report its end, so
// what it holds is released once its handle is collected. The caller holds
// keepMu.
func (h *Handle) syncReleaseLocked() {
	if h.registeredKind() {
		return
	}
	e := h.ext.Load()
	if e == nil {
		return // keeps nothing, and so has no release cleanup
	}
	want := e.keeps != nil && h.State() != Dead && !h.hasCleanupNow()
	if want == e.hasRelease {
		return
	}
	if want {
		e.release = runtime.AddCleanup(h, releaseHolds, h.serial)
	} else {
		e.release.Stop()
	}
	e.hasRelease = want
}

// syncRelease is syncReleaseLocked for callers that do not hold keepMu.
func (h *Handle) syncRelease() {
	if h.registeredKind() || h.ext.Load() == nil {
		return // a handle with no extension keeps nothing
	}
	keepMu.Lock()
	h.syncReleaseLocked()
	keepMu.Unlock()
}

// unslotLocked drops one slot and the owner's anchor once nothing is kept.
// The caller holds keepMu.
func unslotLocked(owner *Handle, slot string) {
	if e := owner.ext.Load(); e == nil || e.keeps == nil {
		return
	}
	if old, ok := owner.x().keeps[slot].(*Handle); ok && unholdLocked(owner.serial, old.serial) {
		defer wakeDeferred()
	}
	delete(owner.x().keeps, slot)
	if len(owner.x().keeps) == 0 {
		owner.x().keeps = nil
		if owner.registeredKind() {
			owner.anchorUnpin()
		} else {
			owner.syncReleaseLocked()
		}
	}
}

// KeepArg makes owner hold arg (a handle, or nil to clear) under slot,
// replacing whatever the slot held. Dead owners keep nothing. With guard set,
// Delete() of arg panics while owner keeps it: Qt would use it after it was
// freed (a QPainter's device, a QDataStream's device).
func KeepArg(owner *Handle, slot string, arg *Handle, guard bool) {
	if owner == nil || !owner.Alive() {
		return
	}
	keepMu.Lock()
	defer keepMu.Unlock()
	if arg == nil {
		unslotLocked(owner, slot)
		return
	}
	ks := slotsFor(owner, true)
	if old, ok := ks[slot].(*Handle); ok {
		if old == arg {
			return
		}
		if unholdLocked(owner.serial, old.serial) {
			defer wakeDeferred()
		}
	}
	ks[slot] = arg
	holdLocked(owner, arg.serial, guard)
	if guard {
		bindScopeLocked(owner, arg)
	}
}

// A guarded hold says the keeper's C++ object follows a pointer to what it
// keeps, and Delete() of the kept object panics for that reason. A scoped
// argument (a handler's QPainter) is not deleted by Go, though: Qt frees it
// soon after the handler returns, whatever still points at it. A keeper left
// alive would then run its destructor on freed memory when GC got to it (a
// QPainterStateGuard restores its painter). So a keeper with a guarded hold
// on a scoped argument, or on another such keeper, is bound to that argument,
// which holds it strongly until the handler returns; Expire then ends the
// bound keepers Go owns, newest first, as their C++ scope would have.

// bindScopeLocked binds keeper to the scoped argument that kept is, or is
// itself bound to. The caller holds keepMu.
func bindScopeLocked(keeper, kept *Handle) {
	root := kept.x().scope
	if kept.scoped {
		root = kept
	}
	if root == nil || !root.Alive() || keeper.x().scope == root {
		return
	}
	keeper.x().scope = root
	root.x().bound = append(root.x().bound, keeper)
}

// endBound deletes the keepers bound to the scoped argument h that still
// use it, directly or through one another. Only plain objects Go owns are
// deleted; anything else is left to its owner.
func (h *Handle) endBound() {
	if h.ext.Load() == nil {
		return // no keeper was ever bound to it
	}
	keepMu.Lock()
	bound := h.x().bound
	h.x().bound = nil
	if len(bound) == 0 {
		keepMu.Unlock()
		return // nearly every scoped argument: nothing was built on it
	}
	ending := map[uint64]bool{h.serial: true}
	var doomed []*Handle
	for _, k := range bound {
		if k.x().scope == h {
			k.x().scope = nil
		}
		for _, hd := range holds[k.serial] {
			if hd.guard && ending[hd.kept] {
				ending[k.serial] = true
				doomed = append(doomed, k)
				break
			}
		}
	}
	keepMu.Unlock()
	note := fmt.Sprintf("ended with the handler it was created in, because the %s argument it used expired (rule 6)", h.class)
	for i := len(doomed) - 1; i >= 0; i-- {
		k := doomed[i]
		if k.registeredKind() || k.State() != Owned || k.deleter == nil || guardingKeeper(k.serial) != "" {
			continue
		}
		k.markDead(note)
		k.deleter(k.ptr)
	}
}

// KeepEach makes owner hold arg until owner dies or Release drops it. Unlike
// KeepArg, each distinct argument gets its own entry (actions added to a widget).
func KeepEach(owner *Handle, arg *Handle, guard bool) {
	if arg == nil {
		return
	}
	KeepArg(owner, eachSlot(arg), arg, guard)
}

// ReleaseSlot drops what owner holds under slot (a QPainter's device once
// End returns, since an inactive painter no longer uses it).
func ReleaseSlot(owner *Handle, slot string) {
	if owner == nil {
		return
	}
	keepMu.Lock()
	defer keepMu.Unlock()
	unslotLocked(owner, slot)
}

// Release drops a reference KeepEach took.
func Release(owner *Handle, arg *Handle) {
	if owner == nil || arg == nil {
		return
	}
	keepMu.Lock()
	defer keepMu.Unlock()
	unslotLocked(owner, eachSlot(arg))
}

func eachSlot(arg *Handle) string { return fmt.Sprintf("each:%p", arg) }

// Keeps reports whether owner holds a reference to arg (for tests).
func Keeps(owner, arg *Handle) bool {
	if owner == nil || arg == nil {
		return false
	}
	keepMu.Lock()
	defer keepMu.Unlock()
	e := owner.ext.Load()
	if e == nil {
		return false
	}
	for _, v := range e.keeps {
		if v == arg {
			return true
		}
	}
	return false
}

// KeepsArg is Keeps for two generated wrappers (for generated tests).
func KeepsArg(owner, arg unsafe.Pointer) bool {
	if owner == nil || arg == nil {
		return false
	}
	return Keeps(RefOf(owner).H, RefOf(arg).H)
}

// KeepEachArg is KeepEach for two generated wrappers (for generated tests).
func KeepEachArg(owner, arg unsafe.Pointer) {
	if owner != nil && arg != nil {
		KeepEach(RefOf(owner).H, RefOf(arg).H, false)
	}
}

// dropHandleKeeps forgets the references a dead handle held.
func (h *Handle) dropHandleKeeps() {
	keepMu.Lock()
	had := false
	if e := h.ext.Load(); e != nil {
		had = e.keeps != nil
		e.keeps = nil
	}
	h.syncReleaseLocked()
	keepMu.Unlock()
	if had && h.registeredKind() {
		h.anchorUnpin()
	}
	releaseHolds(h.serial)
}

// A value that only means something while another object lives (a
// QModelIndex and its model) depends on that object. Use panics once the
// dependency is dead instead of letting Qt follow a dangling pointer, and the
// strong reference keeps the GC from freeing it first.

type depFinder func(p unsafe.Pointer) *Handle

var (
	depMu sync.Mutex
	deps  sync.Map // class name -> []depFinder (copy on write under depMu)
)

// RegisterDependency says that every wrapped object of class depends on the
// object find returns for it (nil for none). A class may register several
// (a QTextTableCell depends on its table and on its document).
func RegisterDependency(class string, find func(p unsafe.Pointer) *Handle) {
	depMu.Lock()
	defer depMu.Unlock()
	var fs []depFinder
	if v, ok := deps.Load(class); ok {
		fs = v.([]depFinder)
	}
	deps.Store(class, append(fs[:len(fs):len(fs)], find))
}

// RegisterDependencyWrap gives, for the dependency of class registered in the
// same position by RegisterDependency, how a pointer to it becomes its
// handle. The shim of an override finds the object in C++ and passes it with
// an inline argument (InlineArg), so Go need not call C++ to ask.
func RegisterDependencyWrap(class string, wrap func(dep unsafe.Pointer) *Handle) {
	depMu.Lock()
	defer depMu.Unlock()
	var ws []func(unsafe.Pointer) *Handle
	if v, ok := depWraps.Load(class); ok {
		ws = v.([]func(unsafe.Pointer) *Handle)
	}
	depWraps.Store(class, append(ws[:len(ws):len(ws)], wrap))
}

var depWraps sync.Map // class name -> []func(unsafe.Pointer) *Handle, as deps

func (h *Handle) attachDependency() {
	v, ok := deps.Load(h.class)
	if !ok {
		return
	}
	for _, find := range v.([]depFinder) {
		if d := find(h.ptr); d != nil && d != h {
			h.addDep(d)
		}
	}
}

// Dependencies returns the objects h depends on (for tests).
func (h *Handle) Dependencies() []*Handle {
	if h == nil {
		return nil
	}
	return h.deps
}

// checkDependency panics when an object h depends on has been destroyed.
func checkDependency(h *Handle, method string) {
	for _, d := range h.deps {
		if State(d.state.Load()) == Dead {
			panic(depMessage(h, d, method))
		}
	}
}

func depMessage(h, dep *Handle, method string) string {
	return fmt.Sprintf("%s: the %s this %s belongs to was destroyed (%s); "+
		"a %s is only valid while its owner lives, so keep the owner alive or fetch a fresh one",
		method, dep.class, h.class, deathNote(dep), h.class)
}

func deathNote(h *Handle) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if why := h.noteLocked(); why != "" {
		return why
	}
	return "destroyed"
}
