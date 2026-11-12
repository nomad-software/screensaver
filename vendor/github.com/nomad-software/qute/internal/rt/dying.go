package rt

import (
	"sync/atomic"
	"unsafe"
	"weak"
)

// A QObject is still handed to Go while it is being destroyed: the destroyed
// signal passes the object itself (OnDestroyed), and other signals Qt emits
// from a destructor can carry it too. By then the death notice has removed
// its registry entry, so wrapping it as usual would register a new handle
// whose own death notice can never come: Qt drops connections made during
// destruction. That stale entry outlives the object, and the next object Qt
// puts at the same address would be given the wrong handle, never watched,
// and used after it is freed.
//
// So Wrap asks Qt whether the object is being destroyed (QObjectData's
// wasDeleted, or a widget's in_destructor; see isDying) and, if it is, registers
// nothing: it returns the handle the object had, Dead, so a handler that
// compares it with the wrappers it holds finds the same Go pointer, or a
// fresh Dead handle when Go held none. The handles of objects being destroyed
// are remembered here from their death notice until the GUI thread is back
// in its event loop, when no destructor is running any more.

// dyingC reports whether the QObject at p is being destroyed (nil in tests
// without Qt). It reads QObjectData::wasDeleted, which ~QObject sets before
// it emits destroyed; a class whose own destructor emits it earlier (QWidget)
// adds a probe for its own flag.
var dyingC func(p unsafe.Pointer) bool

var dyingProbes []func(p unsafe.Pointer) bool // set in init functions only

// RegisterDyingProbe adds a check that the QObject at p is being destroyed,
// for a class whose destructor emits destroyed before ~QObject runs (QWidget,
// from qtwidgets). Call it from an init function.
func RegisterDyingProbe(f func(p unsafe.Pointer) bool) {
	dyingProbes = append(dyingProbes, f)
}

func isDying(p unsafe.Pointer) bool {
	if dyingC == nil {
		return false
	}
	if dyingC(p) {
		return true
	}
	for _, f := range dyingProbes {
		if f(p) {
			return true
		}
	}
	return false
}

type tombstone struct {
	w      weak.Pointer[Handle]
	serial uint64
}

var (
	dying          = map[unsafe.Pointer]tombstone{} // guarded by regMu
	dyingSweepPost atomic.Bool
)

// buryLocked remembers the handle of a QObject whose death notice came; the
// caller holds regMu.
func buryLocked(key unsafe.Pointer, h *Handle) {
	dying[key] = tombstone{w: weak.Make(h), serial: h.serial}
	if dyingSweepPost.CompareAndSwap(false, true) {
		// once the event loop runs again, every destructor has returned
		if !post(sweepDying) {
			dyingSweepPost.Store(false)
		}
	}
}

func sweepDying() {
	dyingSweepPost.Store(false)
	regMu.Lock()
	clear(dying)
	regMu.Unlock()
}

// wrapDyingLocked returns the Dead handle for a QObject that is being
// destroyed; the caller holds regMu and found no live registry entry.
func wrapDyingLocked(key unsafe.Pointer, class string) *Handle {
	if t, ok := dying[key]; ok {
		if h := t.w.Value(); h != nil {
			return h
		}
	}
	h := newHandle(key, class, KindQObject, Dead, DeleteQObject)
	h.note = noteDying
	return h
}
