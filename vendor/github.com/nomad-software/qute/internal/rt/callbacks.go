package rt

import (
	"fmt"
	"reflect"
	"runtime"
	"runtime/cgo"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"unsafe"
	"weak"
)

var callbackDepth atomic.Int32

// InCallback reports whether a signal or virtual handler is running.
func InCallback() bool { return callbackDepth.Load() > 0 }

// A panic in a Go handler called from Qt would otherwise unwind through the
// C++ frames between the handler and the Go code that entered Qt, skipping
// their destructors and leaving Qt in an inconsistent state. While a capture
// is active (qutetest.Run), handlers recover their own panic at the cgo boundary,
// return normally to Qt, and the first panic is re-raised once control is
// back in Go. Without a capture (qute.Main) a handler panic still crashes the
// program, with its stack.

// CallbackPanic is a panic recovered from a Go handler called by Qt.
type CallbackPanic struct {
	Value any
	Stack string
}

func (p *CallbackPanic) String() string {
	return fmt.Sprintf("panic in a Qt callback: %v\n%s", p.Value, p.Stack)
}

var (
	capturing    atomic.Int32
	pendingPanic atomic.Pointer[CallbackPanic]
)

// CapturePanics starts recovering handler panics on the GUI thread. The
// returned stop ends the capture and returns the first recovered panic, if any.
func CapturePanics() (stop func() *CallbackPanic) {
	capturing.Add(1)
	return func() *CallbackPanic {
		capturing.Add(-1)
		return pendingPanic.Swap(nil)
	}
}

// RethrowCallbackPanic re-raises a recovered handler panic in the caller. Call
// it after a call into Qt that may have run handlers (the input helpers).
func RethrowCallbackPanic() {
	if p := pendingPanic.Swap(nil); p != nil {
		panic(p)
	}
}

// catchPanic is deferred by every entry point from Qt into Go handlers.
func catchPanic() {
	if capturing.Load() == 0 {
		return // let the panic crash the program as usual
	}
	if p := recover(); p != nil {
		cp := &CallbackPanic{Value: p, Stack: string(debug.Stack())}
		if inner, ok := p.(*CallbackPanic); ok {
			cp = inner // a nested handler's panic, re-raised by a helper
		}
		pendingPanic.CompareAndSwap(nil, cp)
	}
}

// guiOnlyEntry panics when Qt calls into a Go handler from a thread other
// than the GUI thread; off is what the generated C++ found when it called
// (qute_off_gui), so the check costs no call back into C++. Handlers use Qt objects, and the runtime's callback
// state (InCallback, captured panics) belongs to the GUI thread, so such a
// call cannot work; the panic names the problem instead of a crash somewhere
// else. A binding whose callback Qt runs on a worker thread must be
// hand-written to convert its arguments to plain Go values first (see
// qtwebenginecore's SetCookieFilter).
func guiOnlyEntry(what string, off bool) {
	if off {
		panic("qute: Qt called a Go " + what + " from a thread other than the GUI thread; " +
			"qute runs Go handlers only on the GUI thread, so this binding cannot be used as generated. " +
			"Please report it: its callback needs a hand-written binding")
	}
}

// NewFunc stores a Go func passed to Qt as a std::function. Qt releases it
// (through the same release path as signals) when it drops the function.
func NewFunc(fn func(args, ret unsafe.Pointer)) uintptr {
	return uintptr(cgo.NewHandle(fn)) | funcBit
}

// InvokeFunc runs a std::function callback and lets it write its result. If
// the callback panics and the panic is recovered (inside qutetest.Run or
// PostSync), the result keeps the zero value the shim initialised it with,
// and the panic is re-raised once control is back in Go.
func InvokeFunc(id uintptr, args, ret unsafe.Pointer, off bool) {
	guiOnlyEntry("std::function callback", off)
	fn := cgo.Handle(id &^ funcBit).Value().(func(args, ret unsafe.Pointer))
	callbackDepth.Add(1)
	defer callbackDepth.Add(-1)
	defer catchPanic()
	fn(args, ret)
}

// NewCallback stores fn for a C++ lambda that captures only the returned id.
// The closure lives on the sender's handle, which is anchored (see anchor.go)
// while the connection is live: by the root of its object tree, or globally
// when the sender is itself a root.
func NewCallback(sender *Handle, fn func(args unsafe.Pointer)) uintptr {
	id := newSlot(sender)
	sender.mu.Lock()
	if sender.x().callbacks == nil {
		sender.x().callbacks = map[uintptr]func(args unsafe.Pointer){}
	}
	sender.x().callbacks[id] = fn
	sender.mu.Unlock()
	sender.anchorPin()
	return id
}

// funcBit marks a std::function callback id, a cgo.Handle, apart from the
// slot ids of connections: both come to ReleaseCallback. cgo handles start at
// 1 and grow by one, so the top bit is free.
const funcBit = uintptr(1) << (unsafe.Sizeof(uintptr(0))*8 - 1)

// InvokeCallback runs the closure stored under id with the shim's argument
// array. A closure whose object tree GC already collected (its deletion is
// still queued) is skipped.
func InvokeCallback(id uintptr, args unsafe.Pointer, off bool) {
	guiOnlyEntry("signal handler", off)
	var fn func(args unsafe.Pointer)
	if h := slotHandle(id); h != nil {
		h.mu.Lock()
		fn = h.x().callbacks[id]
		h.mu.Unlock()
	}
	if fn == nil {
		return
	}
	callbackDepth.Add(1)
	defer callbackDepth.Add(-1)
	defer catchPanic()
	fn(args)
}

// ReleaseCallback is called when Qt destroys the slot object, on Disconnect or
// when the sender dies. It frees the closure and drops the sender's anchor.
// std::function callbacks (NewFunc) come here too.
func ReleaseCallback(id uintptr) {
	if id&funcBit != 0 {
		cgo.Handle(id &^ funcBit).Delete()
		return
	}
	v, ok := slots.LoadAndDelete(id)
	if !ok {
		return
	}
	if h := v.(weak.Pointer[Handle]).Value(); h != nil {
		h.mu.Lock()
		delete(h.x().callbacks, id)
		h.mu.Unlock()
		h.anchorUnpin()
	}
}

// Connection is a live signal connection.
type Connection struct {
	c *connState
}

type connState struct {
	conn unsafe.Pointer // QMetaObject::Connection*
	done atomic.Bool
}

// NewConnection wraps a heap QMetaObject::Connection*. When the Connection
// value becomes unreachable the C++ object is freed on the GUI thread, through
// the same queue as GC cleanups: GC cleanups run on their own goroutine, and
// freeing the last reference to a disconnected connection can destroy its slot
// object, which must not happen off the GUI thread.
func NewConnection(conn unsafe.Pointer) Connection {
	s := &connState{conn: conn}
	runtime.AddCleanup(s, freeConnectionLater, conn)
	return Connection{c: s}
}

func freeConnectionLater(conn unsafe.Pointer) {
	enqueueCleanup(cleanupArg{key: conn, kind: KindPlain, deleter: freeConnectionC, quiet: true})
}

// Disconnect breaks the connection and releases its closure. It is safe to
// call more than once, and after the sender has died.
func (c Connection) Disconnect() {
	if c.c == nil || c.c.done.Swap(true) {
		return
	}
	disconnectC(c.c.conn)
}

// Virtual is one overridable virtual method of a generated shim class.
type Virtual struct {
	Name string
	// Has reports whether the Go value overrides this virtual.
	Has func(self any) bool
	// Call converts the shim's arguments, calls the override and writes ret.
	Call func(self any, args, ret unsafe.Pointer)
}

// Dispatch is called by a shim for an overridden virtual. It reports false
// when the Go value is gone (its object tree was collected and is about to be
// deleted); the shim then runs the C++ implementation instead. It also
// reports false when the override panicked and the panic was recovered
// (inside qutetest.Run or PostSync): Qt then gets the C++ implementation's
// result, a sensible value, rather than whatever the override half wrote, and
// the panic is re-raised once control is back in Go.
func Dispatch(id uintptr, idx int, args, ret unsafe.Pointer, off bool) bool {
	guiOnlyEntry("virtual method override", off)
	h := slotHandle(id)
	if h == nil {
		return false
	}
	e := h.ext.Load()
	if e == nil || e.self == nil {
		return false
	}
	callbackDepth.Add(1)
	defer callbackDepth.Add(-1)
	defer catchPanic()
	if idx < 0 || idx >= len(e.vtable) {
		panic(fmt.Sprintf("qute: virtual index %d out of range for %s", idx, h.class))
	}
	e.vtable[idx].Call(e.self, args, ret)
	return true
}

// Mask computes the override bitmask for self against vt.
func Mask(self any, vt []Virtual) []uint64 {
	if len(vt) == 0 {
		return nil
	}
	// Which virtuals a Go type overrides is fixed by its methods, and
	// working it out takes reflection (UserDefined): it is done once per
	// type and table, as an application may make many objects of one type
	// (an icon engine per row).
	key := maskKey{reflect.TypeOf(self), &vt[0]}
	if m, ok := masks.Load(key); ok {
		return m.([]uint64)
	}
	m := make([]uint64, (len(vt)+63)/64)
	for i, v := range vt {
		if v.Has(self) && UserDefined(self, v.Name) {
			m[i/64] |= 1 << (i % 64)
		}
	}
	masks.Store(key, m)
	return m
}

// maskKey identifies a Go subclass type and the dispatch table of the class
// it subclasses (a type may embed the Base of more than one class).
type maskKey struct {
	t  reflect.Type
	vt *Virtual
}

var masks sync.Map // maskKey -> []uint64, read only once stored

// RequireOverride panics unless self defines the virtual called name. A
// pure virtual Go leaves out returns its zero value, which is fine for most
// (a layout with no items) but not where Qt uses the result unchecked
// (QIconEngine::clone); why says when Qt calls it.
func RequireOverride(self any, vt []Virtual, ctor, name, why string) {
	for _, v := range vt {
		if v.Name == name {
			if v.Has(self) && UserDefined(self, name) {
				return
			}
			break
		}
	}
	panic(fmt.Sprintf("%s: %T does not define %s, which Qt needs: %s; define %s on the type",
		ctor, self, name, why, name))
}
