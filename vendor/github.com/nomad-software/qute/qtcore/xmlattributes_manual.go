// Hand-written; qutegen never overwrites *_manual.go files.

package qtcore

// #include "xmlattributes_manual.h"
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"

	rt "github.com/nomad-software/qute/internal/rt"
)

// QXmlStreamAttributes is a QList<QXmlStreamAttribute> in C++, and the list
// part is a template base the generator does not see. Without these two
// methods the attributes of an element could be looked up by name but not
// listed.

// Size returns the number of attributes.
//
//	C++: qsizetype QList<QXmlStreamAttribute>::size() const
//
// Qt docs: https://doc.qt.io/qt-6/qlist.html#size
func (this *QXmlStreamAttributes) Size() int {
	rt.GUIOnly("QXmlStreamAttributes.Size")
	self := rt.Use(unsafe.Pointer(this), "QXmlStreamAttributes.Size")
	n := int(C.qute_xmlattributes_size(self))
	runtime.KeepAlive(this)
	return n
}

// At returns the attribute at index i, in document order. It panics when i
// is out of range.
//
//	C++: const QXmlStreamAttribute & QList<QXmlStreamAttribute>::at(qsizetype i) const
//
// Ownership: the result is an Owned copy.
//
// Qt docs: https://doc.qt.io/qt-6/qlist.html#at
func (this *QXmlStreamAttributes) At(i int) *QXmlStreamAttribute {
	rt.GUIOnly("QXmlStreamAttributes.At")
	self := rt.Use(unsafe.Pointer(this), "QXmlStreamAttributes.At")
	if n := int(C.qute_xmlattributes_size(self)); i < 0 || i >= n {
		panic(fmt.Sprintf("QXmlStreamAttributes.At: index %d out of range; the list has %d attributes, check Size() first", i, n))
	}
	p := C.qute_xmlattributes_at(self, C.ptrdiff_t(i))
	runtime.KeepAlive(this)
	return QXmlStreamAttributeFromPtr(unsafe.Pointer(p), Owned)
}
