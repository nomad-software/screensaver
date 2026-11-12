package rt

// #cgo pkg-config: Qt6Core
// #cgo CXXFLAGS: -std=c++17 -fPIC -O1 -g0
// #include <stdlib.h>
// #include "rt.h"
import "C"

import (
	"runtime"
	"runtime/cgo"
	"unsafe"
)

func init() {
	// Package init runs on the main goroutine while it is still on the main
	// OS thread. Locking here keeps it there for qute.Main and the event loop.
	runtime.LockOSThread()
	C.qute_rt_init()
	rootOfC = func(p unsafe.Pointer) unsafe.Pointer { return C.qute_rt_root(p) }
	subtreeC = subtree
	classNameC = func(p unsafe.Pointer) string { return C.GoString(C.qute_rt_class_name(p)) }
	dyingC = func(p unsafe.Pointer) bool { return bool(C.qute_rt_dying(p)) }
}

//export qute_rt_run
func qute_rt_run(id C.uintptr_t) {
	h := cgo.Handle(id)
	f := h.Value().(func())
	h.Delete()
	f()
}

//export qute_rt_drain
func qute_rt_drain() {
	drainScheduled.Store(false)
	Drain()
}

//export qute_rt_destroyed
func qute_rt_destroyed(obj unsafe.Pointer) {
	Destroyed(obj, "destroyed by Qt")
}

//export qute_rt_visibility
func qute_rt_visibility(obj unsafe.Pointer, visible C.bool) {
	if h := Lookup(obj); h != nil {
		h.SetVisiblePin(bool(visible))
	}
}

//export qute_rt_reparent
func qute_rt_reparent(obj unsafe.Pointer, what C.int) {
	reparented(obj, reparentKind(what))
}

// IsGUIThread reports whether the caller runs on the Qt GUI thread.
func IsGUIThread() bool { return bool(C.qute_rt_is_gui_thread()) }

// AppExists reports whether a Q*Application object exists.
func AppExists() bool { return bool(C.qute_rt_app_exists()) }

// HasParent reports whether a live QObject has a parent.
func HasParent(obj unsafe.Pointer) bool { return bool(C.qute_rt_has_parent(obj)) }

// DeleteQObject is the deleter used for every QObject-derived handle.
func DeleteQObject(obj unsafe.Pointer) { C.qute_rt_delete_qobject(obj) }

func subtree(obj unsafe.Pointer) []unsafe.Pointer {
	var n C.ptrdiff_t
	p := C.qute_rt_subtree(obj, &n)
	if p == nil {
		return nil
	}
	out := append([]unsafe.Pointer(nil), unsafe.Slice((*unsafe.Pointer)(unsafe.Pointer(p)), int(n))...)
	C.free(unsafe.Pointer(p))
	return out
}

func watch(obj unsafe.Pointer)            { C.qute_rt_watch(obj) }
func installFilterC()                     { C.qute_rt_install_filter() }
func nestedLoopC()                        { C.qute_rt_nested_loop() }
func postC(id uintptr) bool               { return bool(C.qute_rt_post(C.uintptr_t(id))) }
func postDrainC() bool                    { return bool(C.qute_rt_post_drain()) }
func disconnectC(conn unsafe.Pointer)     { C.qute_rt_disconnect(conn) }
func freeConnectionC(conn unsafe.Pointer) { C.qute_rt_free_connection(conn) }
