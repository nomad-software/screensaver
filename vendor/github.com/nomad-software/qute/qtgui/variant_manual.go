// Hand-written; qutegen never overwrites *_manual.go files.

package qtgui

// #include "variant_manual.h"
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"

	rt "github.com/nomad-software/qute/internal/rt"
	"github.com/nomad-software/qute/qtcore"
)

// Variant kinds; they match the enum in variant_manual.h.
const (
	gvIcon = iota + 1
	gvFont
	gvBrush
	gvPixmap
	gvImage
	gvPen
	gvKeySequence
	gvCursor
	gvPalette
	gvTransform
	gvRegion
	gvColor = 100
)

// NewQVariantOf converts a Go value to an Owned QVariant. It accepts what
// qtcore.NewQVariantOf accepts and the GUI types an item model returns for
// its roles: QColor (ForegroundRole, BackgroundRole), *QIcon
// (DecorationRole), *QFont (FontRole), *QBrush, *QPixmap, *QImage, *QPen,
// *QKeySequence, *QCursor, *QPalette, *QTransform and *QRegion. Handles are
// copied, so the variant does not depend on them; a nil handle gives an
// invalid variant.
func NewQVariantOf(v any) *qtcore.QVariant {
	rt.CheckThread("qtgui.NewQVariantOf")
	kind, ptr := 0, unsafe.Pointer(nil)
	switch x := v.(type) {
	case QColor:
		return qtcore.QVariantFromPtr(C.qute_gui_variant_color(cQColor(x)), qtcore.Owned)
	case *QIcon:
		kind, ptr = gvIcon, rt.UseArg(unsafe.Pointer(x), "qtgui.NewQVariantOf")
	case *QFont:
		kind, ptr = gvFont, rt.UseArg(unsafe.Pointer(x), "qtgui.NewQVariantOf")
	case *QBrush:
		kind, ptr = gvBrush, rt.UseArg(unsafe.Pointer(x), "qtgui.NewQVariantOf")
	case *QPixmap:
		kind, ptr = gvPixmap, rt.UseArg(unsafe.Pointer(x), "qtgui.NewQVariantOf")
	case *QImage:
		kind, ptr = gvImage, rt.UseArg(unsafe.Pointer(x), "qtgui.NewQVariantOf")
	case *QPen:
		kind, ptr = gvPen, rt.UseArg(unsafe.Pointer(x), "qtgui.NewQVariantOf")
	case *QKeySequence:
		kind, ptr = gvKeySequence, rt.UseArg(unsafe.Pointer(x), "qtgui.NewQVariantOf")
	case *QCursor:
		kind, ptr = gvCursor, rt.UseArg(unsafe.Pointer(x), "qtgui.NewQVariantOf")
	case *QPalette:
		kind, ptr = gvPalette, rt.UseArg(unsafe.Pointer(x), "qtgui.NewQVariantOf")
	case *QTransform:
		kind, ptr = gvTransform, rt.UseArg(unsafe.Pointer(x), "qtgui.NewQVariantOf")
	case *QRegion:
		kind, ptr = gvRegion, rt.UseArg(unsafe.Pointer(x), "qtgui.NewQVariantOf")
	default:
		return qtcore.NewQVariantOf(v)
	}
	if ptr == nil {
		return qtcore.NewQVariantOf(nil)
	}
	r := qtcore.QVariantFromPtr(C.qute_gui_variant_from(C.int(kind), ptr), qtcore.Owned)
	runtime.KeepAlive(v)
	return r
}

// VariantValue converts v to the closest Go value, like v.Value(), and also
// returns the GUI types NewQVariantOf accepts: a QColor, or an Owned copy of
// a *QIcon, *QFont, *QBrush, *QPixmap, *QImage, *QPen, *QKeySequence,
// *QCursor, *QPalette, *QTransform or *QRegion.
func VariantValue(v *qtcore.QVariant) any {
	self := rt.Use(unsafe.Pointer(v), "qtgui.VariantValue")
	kind := int(C.qute_gui_variant_kind(self))
	defer runtime.KeepAlive(v)
	switch kind {
	case 0:
		return v.Value()
	case gvColor:
		return goQColor(C.qute_gui_variant_to_color(self))
	}
	p := C.qute_gui_variant_take(self, C.int(kind))
	switch kind {
	case gvIcon:
		return QIconFromPtr(p, qtcore.Owned)
	case gvFont:
		return QFontFromPtr(p, qtcore.Owned)
	case gvBrush:
		return QBrushFromPtr(p, qtcore.Owned)
	case gvPixmap:
		return QPixmapFromPtr(p, qtcore.Owned)
	case gvImage:
		return QImageFromPtr(p, qtcore.Owned)
	case gvPen:
		return QPenFromPtr(p, qtcore.Owned)
	case gvKeySequence:
		return QKeySequenceFromPtr(p, qtcore.Owned)
	case gvCursor:
		return QCursorFromPtr(p, qtcore.Owned)
	case gvPalette:
		return QPaletteFromPtr(p, qtcore.Owned)
	case gvTransform:
		return QTransformFromPtr(p, qtcore.Owned)
	case gvRegion:
		return QRegionFromPtr(p, qtcore.Owned)
	}
	panic(fmt.Sprintf("qtgui.VariantValue: unknown variant kind %d", kind))
}

// NewQImageFromBytes builds an Owned image of the given size and format from
// a copy of data, which holds height rows of bytesPerLine bytes each (rows
// missing from a short slice are zero). Unlike NewQImageOverMemory
// the image does not refer to data afterwards, so data may be any Go slice.
func NewQImageFromBytes(data []byte, width, height, bytesPerLine int, format QImageFormat) *QImage {
	rt.CheckThread("NewQImageFromBytes")
	p := C.qute_gui_image_from_bytes((*C.char)(unsafe.Pointer(unsafe.SliceData(data))), C.ptrdiff_t(len(data)),
		C.int(width), C.int(height), C.longlong(bytesPerLine), C.int64_t(format))
	return rt.View[QImage](rt.WrapClass(p, "QImage", rt.NewOwned), p)
}
