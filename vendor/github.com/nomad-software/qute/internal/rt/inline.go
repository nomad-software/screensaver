package rt

import (
	"fmt"
	"sync"
	"unsafe"
)

// Inline values. A value class that is trivially copyable and destructible
// (QModelIndex: two ints, an id and a model pointer) needs no C++ heap copy:
// its bytes are kept in Go memory that holds no Go pointers, which C++ may
// read and write during a call like any other object of the class. The GC
// frees it with its handle, so such a value needs no cleanup, no queued
// delete and no call into C to free it. Qt hands them to Go overrides and
// signal handlers for every row it asks about, so this is the hot path of
// models.
//
// The bytes never go where C keeps them: a list of inline values crosses as
// one block of the values themselves, and an override's result is copied
// into storage the shim provides (CopyValueOut), never stored as a pointer.

// CopyInline wraps a value Qt lent for the call (an override's or signal's
// const reference argument) by copying it into Go memory: the C++ object is
// neither kept nor freed.
const CopyInline WrapMode = 20

// RegisterInline records that class is an inline value of size bytes;
// construct builds a default value at p (what a nil element of a list means).
func RegisterInline(class string, size int, construct func(p unsafe.Pointer)) {
	zero := make([]uint64, (size+7)/8)
	construct(unsafe.Pointer(unsafe.SliceData(zero)))
	inlineZero.Store(class, unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(zero))), size))
	updateClass(class, func(ci *classInfo) { ci.inline = size })
}

var inlineZero sync.Map // class -> the bytes of a default-constructed value

// newInline returns an Owned handle of a copy of the value at p, in Go memory.
func newInline(p unsafe.Pointer, class string, size int) *Handle {
	h := newInlineCopy(p, class, size)
	h.attachDependency()
	return h
}

// newInlineCopy is newInline without its dependencies.
func newInlineCopy(p unsafe.Pointer, class string, size int) *Handle {
	buf := make([]uint64, (size+7)/8) // pointer-free and 8-byte aligned
	dst := unsafe.Pointer(unsafe.SliceData(buf))
	copy(unsafe.Slice((*byte)(dst), size), unsafe.Slice((*byte)(p), size))
	h := newHandle(dst, class, KindValue, Owned, nil)
	h.inline = true
	return h
}

// maxInlineDeps is how many dependencies an inline argument carries
// (qute_inline_arg in the shim).
const maxInlineDeps = 4

// inlineArg is the shim's qute_inline_arg: an override's or signal's inline
// argument and the objects it depends on, which C++ found (a QModelIndex's
// model), in the order of the class's depends-on.
type inlineArg struct {
	p    unsafe.Pointer
	deps [maxInlineDeps]unsafe.Pointer
}

// viewPair is a handle and its first wrapper in one allocation: an inline
// value's (its bytes are apart: C++ may only be given memory without Go
// pointers), or a new object's (WrapNew).
type viewPair[T any] struct {
	h Handle
	v T
}

// newInlineView copies the value at p into Go memory and returns its handle
// and wrapper, made together, with the wrapper cached as View would.
func newInlineView[T any](p unsafe.Pointer, class string, size int) (*Handle, *T) {
	buf := make([]uint64, (size+7)/8) // pointer-free and 8-byte aligned
	dst := unsafe.Pointer(unsafe.SliceData(buf))
	copy(unsafe.Slice((*byte)(dst), size), unsafe.Slice((*byte)(p), size))
	x := new(viewPair[T])
	h, v := &x.h, &x.v
	h.init(dst, class, KindValue, Owned, nil)
	h.inline = true
	*RefOf(unsafe.Pointer(v)) = Ref{H: h, P: dst}
	h.cache1[0] = v
	h.cache = h.cache1[:1]
	return h, v
}

// InlineArgView is View(InlineArg(arg, class)) with the handle and wrapper
// made in one allocation: the shape of an override's index argument, which
// Qt passes for every row it asks about.
func InlineArgView[T any](arg unsafe.Pointer, class string) *T {
	a := (*inlineArg)(arg)
	h, v := newInlineView[T](a.p, class, InlineSize(class))
	h.argDeps(a)
	return v
}

// InlineOwnedView wraps a heap copy C++ made for Go (a method's result, a
// list element) as an inline value: its bytes are kept, the copy is freed
// now, and the handle and wrapper are made in one allocation.
func InlineOwnedView[T any](p unsafe.Pointer, class string) *T {
	if p == nil {
		return nil
	}
	v, ok := classes.Load(class)
	if !ok {
		panic("qute: class " + class + " is not registered")
	}
	ci := v.(classInfo)
	h, w := newInlineView[T](p, class, ci.inline)
	ci.deleter(p)
	h.attachDependency()
	return w
}

// argDeps takes h's dependencies from what the shim found (InlineArg).
func (h *Handle) argDeps(a *inlineArg) {
	if v, ok := depWraps.Load(h.class); ok {
		for i, wrap := range v.([]func(unsafe.Pointer) *Handle) {
			if i >= maxInlineDeps || a.deps[i] == nil {
				continue
			}
			if d := wrap(a.deps[i]); d != nil && d != h {
				h.addDep(d)
			}
		}
	}
}

// InlineArg wraps an inline value Qt lent for the call (as CopyInline) from
// the shim's qute_inline_arg, taking its dependencies from what the shim
// found rather than asking C++ again.
func InlineArg(arg unsafe.Pointer, class string) *Handle {
	a := (*inlineArg)(arg)
	h := newInlineCopy(a.p, class, InlineSize(class))
	h.argDeps(a)
	return h
}

// InlineSize returns the size of an inline value class, or 0.
func InlineSize(class string) int {
	if v, ok := classes.Load(class); ok {
		return v.(classInfo).inline
	}
	return 0
}

// CopyValueOut copies the inline value a Go override returned into the
// storage the shim provides (dst). A nil result leaves dst as the shim made
// it, a default-constructed value.
func CopyValueOut(dst, wrapper unsafe.Pointer, class, method string) {
	if wrapper == nil {
		return
	}
	src := Use(wrapper, method)
	n := InlineSize(class)
	copy(unsafe.Slice((*byte)(dst), n), unsafe.Slice((*byte)(src), n))
}

// InlineBlock copies inline values into one block of n values, which C++
// reads as an array of the class (a list argument, or a list an override
// returns). alloc provides the block: Go memory for an argument, C memory for
// a result the shim takes over.
func InlineBlock(class, method string, n int, at func(i int) unsafe.Pointer, alloc func(bytes int) unsafe.Pointer) unsafe.Pointer {
	size := InlineSize(class)
	if size == 0 {
		panic(fmt.Sprintf("qute: %s is not an inline value class", class))
	}
	block := alloc(max(n, 1) * size)
	var zero []byte
	for i := range n {
		dst := unsafe.Slice((*byte)(unsafe.Add(block, i*size)), size)
		w := at(i)
		if w == nil {
			// a nil element is a default value, as a nil argument is
			if zero == nil {
				v, _ := inlineZero.Load(class)
				zero = v.([]byte)
			}
			copy(dst, zero)
			continue
		}
		copy(dst, unsafe.Slice((*byte)(Use(w, method)), size))
	}
	return block
}
