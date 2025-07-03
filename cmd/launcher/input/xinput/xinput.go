package xinput

/*
#cgo LDFLAGS: -lX11 -lXi

#include <stdlib.h>
#include <X11/Xlib.h>
#include <X11/extensions/XInput.h>
#include <X11/extensions/XInput2.h>

int Macro_XIMaskLen(int event) {
	return XIMaskLen(event);
}

int Macro_XISetMask(unsigned char* ptr, int event) {
	return XISetMask(ptr, event);
}

unsigned char* allocate_mask(int len) {
	return calloc(len, sizeof(char));
}
*/
import "C"
import (
	"unsafe"

	"github.com/nomad-software/screensaver/output"
)

// signal is the signal sent on the channel when an event is triggered.
type signal struct{}

// StartXInput checks for keyboard and mouse events and when triggered sends a
// signal on the returned channel.
func GetInput() chan signal {
	c := make(chan signal, 256)
	go getXInput(c)
	return c
}

// StartXInput checks for keyboard and mouse events and when triggered sends a
// signal on the provided channel.
func getXInput(c chan signal) {
	display := C.XOpenDisplay(nil)

	var majorOpcodeReturn C.int
	var firstEventReturn C.int
	var firstErrorReturn C.int

	if C.XQueryExtension(display, C.CString("XInputExtension"), &majorOpcodeReturn, &firstEventReturn, &firstErrorReturn) == 0 {
		C.XCloseDisplay(display)
		output.Fatal("X Input extension not available.\n")
	}

	screen := C.XDefaultScreen(display)
	window := C.XRootWindow(display, screen)
	masks := make([]C.XIEventMask, 2)

	// Mask 1
	masks[0].deviceid = C.XIAllDevices
	masks[0].mask_len = C.Macro_XIMaskLen(C.XI_LASTEVENT)
	masks[0].mask = C.allocate_mask(masks[0].mask_len)
	C.Macro_XISetMask(masks[0].mask, C.XI_ButtonPress)
	C.Macro_XISetMask(masks[0].mask, C.XI_ButtonRelease)
	C.Macro_XISetMask(masks[0].mask, C.XI_KeyPress)
	C.Macro_XISetMask(masks[0].mask, C.XI_KeyRelease)
	C.Macro_XISetMask(masks[0].mask, C.XI_Motion)
	C.Macro_XISetMask(masks[0].mask, C.XI_DeviceChanged)
	C.Macro_XISetMask(masks[0].mask, C.XI_Enter)
	C.Macro_XISetMask(masks[0].mask, C.XI_Leave)
	C.Macro_XISetMask(masks[0].mask, C.XI_FocusIn)
	C.Macro_XISetMask(masks[0].mask, C.XI_FocusOut)
	C.Macro_XISetMask(masks[0].mask, C.XI_TouchBegin)
	C.Macro_XISetMask(masks[0].mask, C.XI_TouchUpdate)
	C.Macro_XISetMask(masks[0].mask, C.XI_TouchEnd)
	C.Macro_XISetMask(masks[0].mask, C.XI_HierarchyChanged)
	C.Macro_XISetMask(masks[0].mask, C.XI_PropertyEvent)

	// Mask 2
	masks[1].deviceid = C.XIAllMasterDevices
	masks[1].mask_len = C.Macro_XIMaskLen(C.XI_LASTEVENT)
	masks[1].mask = C.allocate_mask(masks[1].mask_len)
	C.Macro_XISetMask(masks[1].mask, C.XI_RawKeyPress)
	C.Macro_XISetMask(masks[1].mask, C.XI_RawKeyRelease)
	C.Macro_XISetMask(masks[1].mask, C.XI_RawButtonPress)
	C.Macro_XISetMask(masks[1].mask, C.XI_RawButtonRelease)
	C.Macro_XISetMask(masks[1].mask, C.XI_RawMotion)
	C.Macro_XISetMask(masks[1].mask, C.XI_RawTouchBegin)
	C.Macro_XISetMask(masks[1].mask, C.XI_RawTouchUpdate)
	C.Macro_XISetMask(masks[1].mask, C.XI_RawTouchEnd)

	C.XISelectEvents(display, window, &masks[0], (C.int)(len(masks)))
	C.XSync(display, 0)

	C.free(unsafe.Pointer(masks[0].mask))
	C.free(unsafe.Pointer(masks[1].mask))

	for {
		ev := C.XEvent{}
		cookie := (*C.XGenericEventCookie)(unsafe.Pointer(&ev))

		C.XNextEvent(display, &ev)

		if C.XGetEventData(display, cookie) == 1 && cookie._type == C.GenericEvent && cookie.extension == majorOpcodeReturn {
			c <- signal{}
		}

		C.XFreeEventData(display, cookie)
	}
}
