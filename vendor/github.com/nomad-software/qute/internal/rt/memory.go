package rt

import (
	"runtime"
	"sync/atomic"
	"unsafe"
)

// A QImage of 16 MB costs the Go heap a few dozen bytes, so dropping images
// without Delete() never makes the GC run, and the pixels pile up outside the
// heap. Classes with a `memory` option report the bytes each Owned copy holds;
// once the bytes allocated since the last forced collection pass a threshold
// (32 MB, or the bytes that were live before them if that is more, like
// GOGC=100), qute runs the GC. Copies share pixels until written, so a copy
// is counted in full: that can only make collections more frequent, never
// rarer.
//
// GC cleanups only queue the deletion; the GUI thread frees the objects when
// it next runs the queue. A loop inside one handler would never get there, so
// tracked allocations on the GUI thread also free the value objects already
// queued, right away, and the plain objects that may keep them (see
// drainValues); never QObjects, one of which might be the object whose
// handler is running.

const memMin = 32 << 20

var (
	memLive  atomic.Int64 // bytes held by tracked value objects
	memSince atomic.Int64 // bytes tracked since the last forced collection

	// MemoryCollections counts the collections memory tracking forced (for tests).
	MemoryCollections atomic.Int64
)

// RegisterMemory gives class a function that reports how many bytes one
// object holds outside the Go heap.
func RegisterMemory(class string, size func(unsafe.Pointer) int64) {
	updateClass(class, func(ci *classInfo) { ci.memory = size })
}

// trackNew measures a value object Go just took ownership of.
func trackNew(h *Handle, ci classInfo) {
	if ci.memory != nil && h != nil && h.State() == Owned {
		setMemory(h, ci.memory(h.ptr))
	}
}

// Remeasure measures h again after a method that may have changed its size
// (QImage.Load, QPixmap.ConvertFromImage).
func Remeasure(h *Handle) {
	if h == nil || h.State() != Owned {
		return
	}
	if v, ok := classes.Load(h.class); ok {
		if ci := v.(classInfo); ci.memory != nil {
			setMemory(h, ci.memory(h.ptr))
		}
	}
}

func setMemory(h *Handle, n int64) {
	if n < 0 {
		n = 0
	}
	h.mu.Lock()
	old := h.x().memBytes
	h.x().memBytes = n
	h.mu.Unlock()
	if n == old {
		return
	}
	// the cleanup carries the byte count it gives back, so it is re-made
	if h.State() == Owned {
		h.stopCleanup()
		h.attachCleanup()
	}
	memLive.Add(n - old)
	if n > old {
		allocated(n - old)
	}
}

// untrackMemory forgets the bytes of a handle that died through Delete().
func untrackMemory(h *Handle) {
	e := h.ext.Load()
	if e == nil {
		return
	}
	h.mu.Lock()
	n := e.memBytes
	e.memBytes = 0
	h.mu.Unlock()
	if n != 0 {
		memLive.Add(-n)
	}
}

func allocated(n int64) {
	if IsGUIThread() && Running() {
		drainValues()
	}
	// like GOGC=100: collect once the bytes allocated since the last
	// collection match the bytes that were live before them
	since := memSince.Add(n)
	if since < max(int64(memMin), memLive.Load()-since) {
		return
	}
	memSince.Store(0)
	MemoryCollections.Add(1)
	runtime.GC()
}

// MemoryLive reports the bytes held by tracked value objects (for tests).
func MemoryLive() int64 { return memLive.Load() }

// drainValues frees the value and plain objects in the cleanup queue now and
// leaves everything else for the next Drain. It must run on the GUI thread.
// Plain objects are freed too because they are often what keeps a value (a
// QPainter keeps the QImage it draws on); a kept object still waits for its
// keeper, as in Drain. QObjects and notifying objects wait for Drain: one of
// them might be the object whose handler is running.
func drainValues() {
	n := queueHead.Swap(nil)
	var now []cleanupArg
	var rest []*qnode
	for ; n != nil; n = n.next {
		if (n.arg.kind != KindValue && n.arg.kind != KindPlain) || n.arg.registered || appExited.Load() {
			rest = append(rest, n)
			continue
		}
		now = append(now, n.arg)
	}
	for _, a := range freeReady(now) {
		rest = append(rest, &qnode{arg: a})
	}
	for i := len(rest) - 1; i >= 0; i-- {
		push(rest[i])
	}
}
