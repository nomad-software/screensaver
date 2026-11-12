package rt

import (
	"sync"
	"sync/atomic"
	"unsafe"
	"weak"
)

// Go state attached to a Qt object — the closures of its signal connections,
// its Go subclass value, the references it keeps — must live as long as the
// object does, even when no Go variable refers to the object any more. A
// global root (a cgo.Handle, the strong set) would do that, but it also keeps
// alive everything the state refers to: a button's clicked handler that
// mentions its dialog would keep the dialog from ever being collected, since
// Go cannot see a cycle that runs through C++.
//
// So a handle with such state is anchored instead: held strongly by the Go
// handle of the root of its Qt object tree when that root is an Owned QObject
// (the object GC frees, and with it the whole tree), and by the global strong
// set otherwise. The cycle window -> handler -> window then lies entirely in
// the Go heap, and an unreachable hidden window is collected with its
// handlers. When an object moves to another tree, the event filter reports it
// and its anchored state moves with it.
//
// A handle that is itself a root (a parentless window with its own
// connections) is anchored globally: rule "pinned while it has connections".
//
// Qt calls back with integer ids. The id table holds handles weakly, so it
// is no GC root; a callback whose tree was collected but not yet deleted (the
// cleanup queue has not run) finds nothing and is skipped.

var (
	anchorMu sync.Mutex
	slotIDs  atomic.Uint64
	slots    sync.Map // uintptr -> weak.Pointer[Handle]

	// Hooks into Qt, replaced in tests that have no QApplication.
	rootOfC    func(unsafe.Pointer) unsafe.Pointer
	hasParentC = HasParent // stubbed by tests that have no Qt
	subtreeC   func(unsafe.Pointer) []unsafe.Pointer
	classNameC func(unsafe.Pointer) string
)

// newSlot returns a fresh callback id that refers to h weakly.
func newSlot(h *Handle) uintptr {
	id := uintptr(slotIDs.Add(1))
	slots.Store(id, weak.Make(h))
	return id
}

// slotHandle returns the live handle of a callback id, or nil.
func slotHandle(id uintptr) *Handle {
	v, ok := slots.Load(id)
	if !ok {
		return nil
	}
	return v.(weak.Pointer[Handle]).Value()
}

// anchorPin holds h for one more reason (a connection, its Go subclass value,
// its kept references) until the matching anchorUnpin.
func (h *Handle) anchorPin() {
	anchorMu.Lock()
	defer anchorMu.Unlock()
	h.x().anchorN++
	if h.x().anchorN == 1 {
		h.placeLocked(true, holderFor(h))
	}
}

// anchorUnpin releases one anchorPin.
func (h *Handle) anchorUnpin() {
	anchorMu.Lock()
	defer anchorMu.Unlock()
	if h.anchorCount() == 0 {
		return
	}
	h.x().anchorN--
	if h.x().anchorN == 0 {
		h.placeLocked(false, nil)
	}
}

// Anchored reports how h is held for its Go state: "" (not at all), "global",
// or "root" (by the handle of its tree's root). For DumpTree and tests.
func (h *Handle) Anchored() string {
	anchorMu.Lock()
	defer anchorMu.Unlock()
	e := h.ext.Load()
	switch {
	case e == nil || !e.placed:
		return ""
	case e.holder == nil:
		return "global"
	}
	return "root"
}

// anchorCount is the number of reasons h is anchored for, read without
// making its extension: the reparenting walk asks every wrapped object in a
// subtree, and most have none. The caller holds anchorMu.
func (h *Handle) anchorCount() int32 {
	if e := h.ext.Load(); e != nil {
		return e.anchorN
	}
	return 0
}

// placeLocked moves h to a new holder: the global strong set (holder nil) or
// a root handle. The new holder takes h before the old one lets go, so a
// concurrent GC never sees it unheld. The caller holds anchorMu.
func (h *Handle) placeLocked(placed bool, holder *Handle) {
	if !placed {
		holder = nil
	}
	wasGlobal := h.x().placed && h.x().holder == nil
	nowGlobal := placed && holder == nil
	if placed && holder != nil {
		if holder.x().anchored == nil {
			holder.x().anchored = map[*Handle]struct{}{}
		}
		holder.x().anchored[h] = struct{}{}
	}
	if nowGlobal && !wasGlobal {
		h.Pin()
	}
	if h.x().placed && h.x().holder != nil && (h.x().holder != holder || !placed) {
		delete(h.x().holder.x().anchored, h)
	}
	if wasGlobal && !nowGlobal {
		h.Unpin()
	}
	h.x().placed, h.x().holder = placed, holder
}

// rootHolder returns the handle of the tree root at root if it can hold the
// Go state of its tree: an Owned QObject, which GC frees together with
// everything it holds. Otherwise nil, for the global set.
func rootHolder(root unsafe.Pointer) *Handle {
	if r := Lookup(root); r != nil && r.kind == KindQObject && r.State() == Owned {
		return r
	}
	return nil
}

// holderFor picks the holder of h's Go state: the Owned root of its tree, or
// nil for the global set (also when h is the root). The caller holds anchorMu.
func holderFor(h *Handle) *Handle {
	if h.kind != KindQObject || h.State() == Dead || rootOfC == nil {
		return nil
	}
	if root := rootOfC(h.ptr); root != h.ptr {
		return rootHolder(root)
	}
	return nil
}

// releaseAnchoredLocked moves everything r holds to the global set, because r
// can no longer hold it (it became Parented, or died). The caller holds
// anchorMu.
func (r *Handle) releaseAnchoredLocked() {
	e := r.ext.Load()
	if e == nil {
		return // it never held anything
	}
	for a := range e.anchored {
		a.placeLocked(true, nil)
	}
	e.anchored = nil
}

func (r *Handle) releaseAnchored() {
	anchorMu.Lock()
	defer anchorMu.Unlock()
	r.releaseAnchoredLocked()
}

type reparentKind int

const (
	childRemoved  reparentKind = iota // the child still has its old parent
	childAdded                        // the child has its new parent
	parentChanged                     // a widget's parent changed (it may now be nil)
)

// reparented runs when Qt moves a wrapped object (or one with children) to a
// new parent. An Owned object that Qt gives a parent is Parented from now on:
// its parent frees it. The Go state anchored in the moved subtree follows it
// to its new tree; while the object is between parents it is held globally.
func reparented(obj unsafe.Pointer, what reparentKind) {
	if what != childRemoved && HasParent(obj) {
		if h := Lookup(obj); h != nil && h.State() == Owned {
			h.TransferToCpp()
		}
	}
	reanchorSubtree(obj, what == childRemoved)
}

// reanchorSubtree recomputes the holder of every anchored handle in obj's
// subtree, or moves them all to the global set.
func reanchorSubtree(obj unsafe.Pointer, toGlobal bool) {
	objs := subtreeC(obj)
	if len(objs) == 0 {
		return
	}
	root := rootOfC(obj)
	anchorMu.Lock()
	defer anchorMu.Unlock()
	var rootH *Handle
	if !toGlobal {
		rootH = rootHolder(root)
	}
	for _, p := range objs {
		h := Lookup(p)
		if h == nil || h.anchorCount() == 0 {
			continue
		}
		holder := rootH
		if p == root || toGlobal {
			holder = nil
		}
		h.placeLocked(true, holder)
	}
}
