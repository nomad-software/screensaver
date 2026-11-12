// Hand-written; qutegen never overwrites *_manual.go files.

package qtwidgets

// #include "dying_manual.h"
import "C"

import (
	"unsafe"

	rt "github.com/nomad-software/qute/internal/rt"
)

// A widget is destroyed by ~QWidget, which emits destroyed before ~QObject
// marks the object deleted: the runtime asks this too, so the object a
// destroyed handler is handed is not registered as a new one (rt/dying.go).
func init() {
	rt.RegisterDyingProbe(func(p unsafe.Pointer) bool { return bool(C.qute_widget_in_destructor(p)) })
}
