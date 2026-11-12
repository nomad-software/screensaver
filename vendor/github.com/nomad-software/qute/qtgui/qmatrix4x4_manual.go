// Hand-written; qutegen never overwrites *_manual.go files.

package qtgui

import (
	"runtime"
	"unsafe"
)

// CopyDataTo returns the 16 elements of the matrix in row-major order, like
// QMatrix4x4::copyDataTo. The generated binding is skipped on purpose: Qt
// writes 16 floats through the pointer, which the out-parameter mapping would
// point at a single float.
//
// Qt docs: https://doc.qt.io/qt-6/qmatrix4x4.html#copyDataTo
func (this *QMatrix4x4) CopyDataTo() [16]float32 {
	// ConstData is column-major: element (row, col) is at col*4 + row.
	src := unsafe.Slice((*float32)(this.ConstData()), 16)
	var out [16]float32
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			out[row*4+col] = src[col*4+row]
		}
	}
	runtime.KeepAlive(this)
	return out
}
