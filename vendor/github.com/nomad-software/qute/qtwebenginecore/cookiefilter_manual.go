// Hand-written; qutegen never overwrites *_manual.go files.

package qtwebenginecore

// #include "cookiefilter_manual.h"
import "C"

import (
	"fmt"
	"os"
	"runtime"
	"runtime/cgo"
	"runtime/debug"
	"unsafe"

	rt "github.com/nomad-software/qute/internal/rt"
)

// CookieFilterRequest describes a cookie access the filter decides on. It
// holds plain values: the filter runs on a network thread, where no Qt
// object may be used.
type CookieFilterRequest struct {
	FirstPartyURL string // the URL of the page that is loaded
	Origin        string // the URL of the resource that sets or reads the cookie
	ThirdParty    bool   // whether Origin is a third party to FirstPartyURL
}

// SetCookieFilter installs filter, which decides whether a cookie may be set
// or sent: it returns true to allow the access. nil removes the filter.
//
// Qt calls the filter on a network thread, not the GUI thread, and possibly
// from several at once. It must not use Qt objects or qute functions, and
// what it shares with the rest of the program must be safe for concurrent
// use (a mutex, an atomic, or data that never changes). A panic in the
// filter is printed and blocks that cookie.
//
//	C++: void QWebEngineCookieStore::setCookieFilter(const std::function<bool (const QWebEngineCookieStore::FilterRequest &)> &filterCallback)
//
// Hand-written, because the generated form would run the filter as an
// ordinary Go callback, which qute only allows on the GUI thread.
//
// Qt docs: https://doc.qt.io/qt-6/qwebenginecookiestore.html#setCookieFilter
func (this *QWebEngineCookieStore) SetCookieFilter(filter func(r CookieFilterRequest) bool) {
	self := rt.Use(unsafe.Pointer(this), "QWebEngineCookieStore.SetCookieFilter")
	var id uintptr
	if filter != nil {
		id = uintptr(cgo.NewHandle(filter))
	}
	C.qute_webengine_set_cookie_filter(self, C.uintptr_t(id))
	runtime.KeepAlive(this)
}

//export qute_webengine_cookie_filter
func qute_webengine_cookie_filter(id C.uintptr_t, fp *C.char, fpn C.int, or *C.char, orn C.int, third C.bool) (allow C.bool) {
	f := cgo.Handle(id).Value().(func(CookieFilterRequest) bool)
	r := CookieFilterRequest{
		FirstPartyURL: C.GoStringN(fp, fpn),
		Origin:        C.GoStringN(or, orn),
		ThirdParty:    bool(third),
	}
	defer func() {
		// a panic would unwind into Chromium's thread and end the process
		if p := recover(); p != nil {
			fmt.Fprintf(os.Stderr, "qute: panic in a cookie filter (the cookie is blocked): %v\n%s", p, debug.Stack())
			allow = false
		}
	}()
	return C.bool(f(r))
}

//export qute_webengine_cookie_filter_release
func qute_webengine_cookie_filter_release(id C.uintptr_t) { cgo.Handle(id).Delete() }
