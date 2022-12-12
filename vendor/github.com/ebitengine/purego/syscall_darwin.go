// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

package purego

import (
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

var syscall9XABI0 uintptr

type syscall9Args struct{ fn, a1, a2, a3, a4, a5, a6, a7, a8, a9, r1, r2, err uintptr }

//go:nosplit
func syscall_syscall9X(fn, a1, a2, a3, a4, a5, a6, a7, a8, a9 uintptr) (r1, r2, err uintptr) {
	args := syscall9Args{fn, a1, a2, a3, a4, a5, a6, a7, a8, a9, r1, r2, err}
	runtime_cgocall(syscall9XABI0, unsafe.Pointer(&args))
	return args.r1, args.r2, args.err
}

// NewCallback converts a Go function to a function pointer conforming to the C calling convention.
// This is useful when interoperating with C code requiring callbacks. The argument is expected to be a
// function with zero or one uintptr-sized result. The function must not have arguments with size larger than the size
// of uintptr. Only a limited number of callbacks may be created in a single Go process, and any memory allocated
// for these callbacks is never released. At least 2000 callbacks can always be created. Although this function
// provides similar functionality to windows.NewCallback it is distinct.
func NewCallback(fn interface{}) uintptr {
	return compileCallback(fn)
}

// maxCb is the maximum number of callbacks
// only increase this if you have added more to the callbackasm function
const maxCB = 2000

var cbs struct {
	lock  sync.Mutex
	numFn int                  // the number of functions currently in cbs.funcs
	funcs [maxCB]reflect.Value // the saved callbacks
}

type callbackArgs struct {
	index uintptr
	// args points to the argument block.
	//
	// For cdecl and stdcall, all arguments are on the stack.
	//
	// For fastcall, the trampoline spills register arguments to
	// the reserved spill slots below the stack arguments,
	// resulting in a layout equivalent to stdcall.
	//
	// For arm, the trampoline stores the register arguments just
	// below the stack arguments, so again we can treat it as one
	// big stack arguments frame.
	args unsafe.Pointer
	// Below are out-args from callbackWrap
	result uintptr
}

func compileCallback(fn interface{}) uintptr {
	val := reflect.ValueOf(fn)
	if val.Kind() != reflect.Func {
		panic("purego: type is not a function")
	}
	ty := val.Type()
	for i := 0; i < ty.NumIn(); i++ {
		in := ty.In(i)
		switch in.Kind() {
		case reflect.Struct, reflect.Float32, reflect.Float64,
			reflect.Interface, reflect.Func, reflect.Slice,
			reflect.Chan, reflect.Complex64, reflect.Complex128,
			reflect.String, reflect.Map, reflect.Invalid:
			panic("purego: unsupported argument type: " + in.Kind().String())
		}
	}
	if ty.NumOut() > 1 || ty.NumOut() == 1 && ty.Out(0).Size() != ptrSize {
		panic("purego: callbacks can only have one pointer-sized return")
	}
	cbs.lock.Lock()
	defer cbs.lock.Unlock()
	if cbs.numFn >= maxCB {
		panic("purego: the maximum number of callbacks has been reached")
	}
	cbs.funcs[cbs.numFn] = val
	cbs.numFn++
	return callbackasmAddr(cbs.numFn - 1)
}

const ptrSize = unsafe.Sizeof((*int)(nil))

const callbackMaxFrame = 64 * ptrSize

// callbackasmABI0 is implemented in zcallback_GOOS_GOARCH.s
var callbackasmABI0 uintptr

// callbackWrapPicker gets whatever is on the stack and in the first register.
// Depending on which version of Go that uses stack or register-based
// calling it passes the respective argument to the real calbackWrap function.
// The other argument is therefore invalid and points to undefined memory so don't use it.
// This function is necessary since we can't use the ABIInternal selector which is only
// valid in the runtime.
func callbackWrapPicker(stack, register *callbackArgs) {
	if stackCallingConvention {
		callbackWrap(stack)
	} else {
		callbackWrap(register)
	}
}

// callbackWrap is called by assembly code which determines which Go function to call.
// This function takes the arguments and passes them to the Go function and returns the result.
func callbackWrap(a *callbackArgs) {
	cbs.lock.Lock()
	fn := cbs.funcs[a.index]
	cbs.lock.Unlock()
	fnType := fn.Type()
	args := make([]reflect.Value, fnType.NumIn())
	frame := (*[callbackMaxFrame]uintptr)(a.args)
	for i := range args {
		//TODO: support float32 and float64
		args[i] = reflect.NewAt(fnType.In(i), unsafe.Pointer(&frame[i])).Elem()
	}
	ret := fn.Call(args)
	if len(ret) > 0 {
		switch k := ret[0].Kind(); k {
		case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uintptr:
			a.result = uintptr(ret[0].Uint())
		case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
			a.result = uintptr(ret[0].Int())
		case reflect.Bool:
			if ret[0].Bool() {
				a.result = 1
			} else {
				a.result = 0
			}
		default:
			panic("purego: unsupported kind: " + k.String())
		}
	}
}

// callbackasmAddr returns address of runtime.callbackasm
// function adjusted by i.
// On x86 and amd64, runtime.callbackasm is a series of CALL instructions,
// and we want callback to arrive at
// correspondent call instruction instead of start of
// runtime.callbackasm.
// On ARM, runtime.callbackasm is a series of mov and branch instructions.
// R12 is loaded with the callback index. Each entry is two instructions,
// hence 8 bytes.
func callbackasmAddr(i int) uintptr {
	var entrySize int
	switch runtime.GOARCH {
	default:
		panic("purego: unsupported architecture")
	case "386", "amd64":
		entrySize = 5
	case "arm", "arm64":
		// On ARM and ARM64, each entry is a MOV instruction
		// followed by a branch instruction
		entrySize = 8
	}
	return callbackasmABI0 + uintptr(i*entrySize)
}
