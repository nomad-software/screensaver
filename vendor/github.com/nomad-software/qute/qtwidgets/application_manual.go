// Hand-written; qutegen never overwrites *_manual.go files.

package qtwidgets

// #include "application_manual.h"
import "C"

import (
	"unsafe"

	rt "github.com/nomad-software/qute/internal/rt"
)

// NewQApplication creates the application object. Most programs call
// qute.Main instead, which creates it, runs the event loop and handles
// shutdown; code that calls NewQApplication itself must still run its event
// loop through qute.Main, which starts the runtime (cleanup queue, window
// pins). The application object is never freed by GC.
func NewQApplication(args []string) *QApplication {
	rt.CheckThread("NewQApplication")
	if rt.AppExists() {
		panic("NewQApplication: an application object already exists; use QApplicationInstance()")
	}
	d, l := quteStrListArg(args)
	p := C.qute_application_new((*C.char)(unsafe.Pointer(unsafe.SliceData(d))), (*C.ptrdiff_t)(unsafe.SliceData(l)), C.ptrdiff_t(len(args)))
	app := rt.View[QApplication](rt.WrapClass(p, "QApplication", rt.KeepOrBorrow), p)
	app.Keep()
	return app
}
