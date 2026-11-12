// Hand-written; qutegen never overwrites *_manual.go files.

package qtgui

// #include "qtgui_gen.h"
import "C"

import (
	"runtime"
	"unsafe"

	rt "github.com/nomad-software/qute/internal/rt"
)

// PaintOn draws on device with a QPainter that lasts for the call to f, as
// `QPainter p(device);` does in a C++ paintEvent: f draws, and when it
// returns (or panics) the painter is ended and freed. It is the cheap way to
// paint: the painter is scoped (rule 6), so it needs no GC cleanup and keeps
// no reference to the device, and it is Dead once PaintOn returns; keep
// nothing of it. Use NewQPainterOn for a painter that must outlive one call.
//
// As in C++, f runs even when the painter could not begin (a widget outside
// its paint event); p.IsActive() tells. While f runs, Delete() of device
// panics, since the painter still draws on it.
func PaintOn(device QPaintDeviceLike, f func(p *QPainter)) {
	rt.CheckThread("PaintOn")
	if device == nil {
		panic("PaintOn: device is nil; pass the widget, window, image or pixmap to draw on")
	}
	devPtr, devH := rt.Arg(unsafe.Pointer(device.AsQPaintDevice()), "PaintOn")
	ptr := C.qute_QPainter_new_1(devPtr)
	h := rt.WrapScoped[QPainter](ptr, "QPainter")
	rt.Lend(devH, "QPainter")
	defer func() {
		// Objects built on the painter (a QPainterStateGuard) end first,
		// while it is still there for their destructors.
		h.Expire()
		C.qute_QPainter_delete(ptr)
		rt.Unlend(devH)
		runtime.KeepAlive(device)
	}()
	f(rt.View[QPainter](h, ptr))
}
