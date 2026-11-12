// Hand-written; qutegen never overwrites *_manual.go files.

package qtcore

// #include "metaobject_manual.h"
import "C"

import (
	"runtime"
	"unsafe"

	rt "github.com/nomad-software/qute/internal/rt"
)

// MetaObject returns the meta-object of the object's most derived class:
// its class name, superclasses, properties, methods, signals and enums.
// Qt's meta-objects are static and live as long as the program, so the
// result is Borrowed and stays valid after the object dies.
//
//	C++: virtual const QMetaObject * QObject::metaObject() const
//
// Hand-written, because the generator skips metaObject everywhere: moc
// implements it in every QObject subclass, and Go cannot override it. This
// binding only reads it.
//
// Qt docs: https://doc.qt.io/qt-6/qobject.html#metaObject
func (this *QObject) MetaObject() *QMetaObject {
	rt.GUIOnly("QObject.MetaObject")
	self := rt.Use(unsafe.Pointer(this), "QObject.MetaObject")
	p := C.qute_qobject_meta_object(self)
	runtime.KeepAlive(this)
	return QMetaObjectFromPtr(unsafe.Pointer(p), Borrowed)
}
