package qutetest

// #cgo pkg-config: Qt6Widgets Qt6Test
// #cgo CXXFLAGS: -std=c++17 -fPIC -O1 -g0
// #include "input.h"
import "C"

import (
	"unsafe"

	rt "github.com/nomad-software/qute/internal/rt"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtwidgets"
)

func widgetPtr(w qtwidgets.QWidgetLike, method string) unsafe.Pointer {
	if w == nil {
		panic(method + ": widget is nil")
	}
	rt.GUIOnly(method)
	return rt.Use(unsafe.Pointer(w.AsQWidget()), method)
}

// Click sends a left mouse click to the centre of w, as a user would.
func Click(w qtwidgets.QWidgetLike) {
	C.qute_tools_click(widgetPtr(w, "qutetest.Click"), C.int(qtcore.LeftButton), 0, 0, false)
	rt.RethrowCallbackPanic()
}

// ClickAt sends a mouse click with button at pos (widget coordinates).
func ClickAt(w qtwidgets.QWidgetLike, button qtcore.MouseButton, pos qtcore.QPoint) {
	C.qute_tools_click(widgetPtr(w, "qutetest.ClickAt"), C.int(button), C.int(pos.X), C.int(pos.Y), true)
	rt.RethrowCallbackPanic()
}

// DoubleClick sends a left double click to the centre of w.
func DoubleClick(w qtwidgets.QWidgetLike) {
	C.qute_tools_dblclick(widgetPtr(w, "qutetest.DoubleClick"), C.int(qtcore.LeftButton))
	rt.RethrowCallbackPanic()
}

// Type sends text to w as key presses.
func Type(w qtwidgets.QWidgetLike, text string) {
	C.qute_tools_type(widgetPtr(w, "qutetest.Type"), (*C.char)(unsafe.Pointer(unsafe.StringData(text))), C.ptrdiff_t(len(text)))
	rt.RethrowCallbackPanic()
}

// Key sends one key press with modifiers, e.g. Key(edit, qtcore.Key_Return, 0).
func Key(w qtwidgets.QWidgetLike, key qtcore.Key, mods qtcore.KeyboardModifiers) {
	C.qute_tools_key(widgetPtr(w, "qutetest.Key"), C.int(key), C.int(mods))
	rt.RethrowCallbackPanic()
}

// Wait runs the event loop for ms milliseconds (timers, deferred deletes).
// Call it on the GUI thread.
func Wait(ms int) {
	rt.GUIOnly("qutetest.Wait")
	C.qute_tools_wait(C.int(ms))
	rt.RethrowCallbackPanic()
}
