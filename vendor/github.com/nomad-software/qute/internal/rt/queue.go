package rt

import (
	"fmt"
	"io"
	"os"
	"runtime/cgo"
	"sync"
	"sync/atomic"
)

// Cleanups never call delete directly: GC cleanups run on a GC goroutine, and
// Qt objects must be freed on the GUI thread. They push onto this lock-free
// stack, which the GUI thread drains from a posted event.

type qnode struct {
	arg  cleanupArg
	next *qnode
}

var (
	queueHead      atomic.Pointer[qnode]
	drainScheduled atomic.Bool
	appRunning     atomic.Bool
	appExited      atomic.Bool

	pendingMu    sync.Mutex
	pendingPosts []func()

	// Drained counts objects freed by the cleanup queue (for tests).
	Drained atomic.Int64
)

func enqueueCleanup(arg cleanupArg) {
	if appExited.Load() {
		return
	}
	push(&qnode{arg: arg})
	scheduleDrain()
}

func push(n *qnode) {
	for {
		old := queueHead.Load()
		n.next = old
		if queueHead.CompareAndSwap(old, n) {
			return
		}
	}
}

func scheduleDrain() {
	if !appRunning.Load() || appExited.Load() {
		return
	}
	if drainScheduled.CompareAndSwap(false, true) {
		if !postDrainC() {
			drainScheduled.Store(false)
		}
	}
}

// deferred holds frees of objects a live keeper still holds (see keep.go);
// only the GUI thread touches it. deferredN mirrors its length for wakeDeferred.
var (
	deferred  []cleanupArg
	deferredN atomic.Int32
)

// Drain frees every queued object. It must run on the GUI thread. An object
// another object keeps waits until its keepers are gone: freeing a keeper
// in this same drain releases it, and a keeper Qt destroys later wakes the
// queue again (wakeDeferred).
func Drain() {
	pending := deferred
	deferred = nil
	for n := queueHead.Swap(nil); n != nil; n = n.next {
		pending = append(pending, n.arg)
	}
	pending = freeReady(pending)
	// A destructor may run a Go handler that runs the event loop, and with it
	// a nested Drain, which leaves what it could not free in deferred: keep
	// those too.
	deferred = append(pending, deferred...)
	deferredN.Store(int32(len(deferred)))
}

// freeReady frees every object in pending that no live keeper holds, again
// and again while that frees keepers, and returns the ones still held. Every
// path that frees queued objects goes through it, so a kept object is never
// freed before its keeper (a QImage before the QPainter drawing on it).
func freeReady(pending []cleanupArg) []cleanupArg {
	for progress := true; progress && len(pending) > 0; {
		progress = false
		rest := pending[:0]
		for _, a := range pending {
			if appExited.Load() {
				return nil // deleting after the app is gone crashes inside Qt destructors
			}
			if a.serial != 0 && isKept(a.serial) {
				rest = append(rest, a)
				continue
			}
			freeNode(a)
			progress = true
		}
		pending = rest
	}
	return pending
}

// wakeDeferred schedules a drain when a free was waiting for a keeper.
func wakeDeferred() {
	if deferredN.Load() > 0 {
		scheduleDrain()
	}
}

func freeNode(a cleanupArg) {
	if a.registered {
		regMu.Lock()
		e, found := registry[a.key]
		ok := found && e.serial == a.serial
		var revived *Handle
		if found && !ok && e.prevSerial == a.serial {
			revived = e.w.Value()
		}
		regMu.Unlock()
		if revived != nil && revived.State() == Borrowed {
			// Qt handed the object out again after GC collected its Owned
			// handle and before this free ran: Go still owns it, so the new
			// handle takes over instead of the object being freed under it.
			revived.TransferToGo()
			return
		}
		if revived != nil {
			return // a rule since then said who owns it
		}
		if !ok {
			return // already destroyed, or the address was reused
		}
		if a.kind == KindQObject && hasParentC(a.key) {
			// Safety net: an ownership rule was missed and Qt now owns the
			// object. Never delete something with a parent. The Go state its
			// handle anchored was collected with it, so say so instead of
			// letting its handlers stop running without a trace.
			warnParentedFree(a.class)
			return
		}
	}
	a.deleter(a.key)
	if a.mem != 0 {
		memLive.Add(-a.mem)
	}
	if a.registered {
		Destroyed(a.key, "freed by GC")
	} else if a.serial != 0 {
		releaseHolds(a.serial) // what a plain object kept may go now
	}
	if !a.quiet {
		Drained.Add(1)
	}
}

var (
	warnedMu sync.Mutex
	warned   = map[string]bool{}
)

// warnParentedFree reports, once per class, that GC collected the Owned
// handle of an object Qt had given a parent (rule 7).
var warnOut io.Writer = os.Stderr // tests capture it

func warnParentedFree(class string) {
	warnedMu.Lock()
	seen := warned[class]
	warned[class] = true
	warnedMu.Unlock()
	if !seen {
		fmt.Fprintf(warnOut, "qute: GC collected the Owned handle of a %s that Qt has given a parent; "+
			"the object is left to its parent (rule 7), but the Go handlers of its tree no longer run. "+
			"An ownership rule is missing: the method that gave it the parent needs transfer-to-cpp\n", class)
	}
}

// AppStarted is called once the Q*Application exists and the GUI thread can
// receive posted events.
func AppStarted() {
	if AppExists() {
		installFilterC()
	}
	pendingMu.Lock()
	appExited.Store(false)
	appRunning.Store(true)
	p := pendingPosts
	pendingPosts = nil
	// posted under the lock, so work queued before the start runs first and
	// none can slip in between the check in post and this hand-over
	for _, f := range p {
		postNow(f)
	}
	pendingMu.Unlock()
	if queueHead.Load() != nil {
		scheduleDrain()
	}
}

// AppExited is called after the event loop returns. Queued cleanups are
// dropped, not run, from now on.
func AppExited() {
	appRunning.Store(false)
	appExited.Store(true)
	queueHead.Store(nil)
	// Work posted but never run (the loop exited first) would leave its
	// PostSync caller waiting forever; wake every waiter instead.
	syncMu.Lock()
	ws := syncWaiters
	syncWaiters = map[*syncCall]struct{}{}
	syncMu.Unlock()
	for w := range ws {
		w.finish(syncResult{notRun: true})
	}
}

// Running reports whether the application event loop can take posted work.
func Running() bool { return appRunning.Load() && !appExited.Load() }

// Post queues f to run on the GUI thread.
func Post(f func()) { post(f) }

// post queues f and reports whether it was accepted. Work queued before the
// application starts counts as accepted; work after it exits does not.
func post(f func()) bool {
	if !appRunning.Load() {
		if testHookPost != nil {
			testHookPost()
		}
		pendingMu.Lock()
		if !appRunning.Load() {
			// checked again under the lock AppStarted hands the list over in,
			// so f either joins the list before the hand-over or sees the app
			// running
			defer pendingMu.Unlock()
			if appExited.Load() {
				return false
			}
			pendingPosts = append(pendingPosts, f)
			return true
		}
		pendingMu.Unlock()
	}
	return postNow(f)
}

// testHookPost runs between post's first look at appRunning and the lock
// (tests only), where AppStarted used to slip in and strand the work.
var testHookPost func()

func postNow(f func()) bool {
	h := cgo.NewHandle(f)
	if !postC(uintptr(h)) {
		h.Delete()
		return false
	}
	return true
}

type syncResult struct {
	panicked bool
	val      any
	notRun   bool
	goexit   bool
}

type syncCall struct {
	once sync.Once
	done chan syncResult
}

func (s *syncCall) finish(r syncResult) { s.once.Do(func() { s.done <- r }) }

var (
	syncMu      sync.Mutex
	syncWaiters = map[*syncCall]struct{}{}
)

// errNotRun is the panic PostSync raises when f never ran.
const errNotRun = "qute.PostSync: the application exited before the function could run on the GUI thread"

// ErrGoexit is the panic PostSync raises when f called runtime.Goexit.
const ErrGoexit = "qute.PostSync: the function called runtime.Goexit (t.Fatal, t.FailNow or t.Skip) on the GUI thread; " +
	"inside qutetest.Run that ends the test, elsewhere return from the function instead"

// AfterRescue, when set, runs on the GUI thread once the event loop that
// replaced one ended by runtime.Goexit has quit (qutetest.TestMain exits there).
var AfterRescue func()

// PostSync runs f on the GUI thread and waits for it. Called on the GUI
// thread it runs f directly. A panic in f is re-raised in the caller, and so
// is a panic in a signal or virtual handler that f triggered: it is recovered
// where Qt called the handler, so Qt's C++ frames return normally, and
// re-raised with the handler's own stack. If the application exits (or the
// post fails) before f runs, PostSync panics instead of waiting forever.
func PostSync(f func()) {
	switch r := postSync(f); {
	case r.goexit:
		panic(ErrGoexit)
	case r.notRun:
		panic(errNotRun)
	case r.panicked:
		panic(r.val)
	}
}

// PostSyncExit is PostSync that reports a runtime.Goexit in f (a t.Fatal
// inside qutetest.Run) instead of panicking, so the caller can end its own
// goroutine the same way.
func PostSyncExit(f func()) (goexit bool) {
	switch r := postSync(f); {
	case r.goexit:
		return true
	case r.notRun:
		panic(errNotRun)
	case r.panicked:
		panic(r.val)
	}
	return false
}

func postSync(f func()) syncResult {
	if IsGUIThread() {
		stop := CapturePanics()
		returned := false
		defer func() {
			// a panic of f itself keeps unwinding; a handler's is raised here
			if cp := stop(); returned && cp != nil {
				panic(cp)
			}
		}()
		f()
		returned = true
		return syncResult{}
	}
	if !Running() {
		panic("qute.PostSync: the application is not running; call it from inside qute.Main or a qutetest.TestMain test")
	}
	s := &syncCall{done: make(chan syncResult, 1)}
	syncMu.Lock()
	syncWaiters[s] = struct{}{}
	syncMu.Unlock()
	ok := post(func() {
		syncMu.Lock()
		delete(syncWaiters, s)
		syncMu.Unlock()
		r := syncResult{panicked: true}
		stop := CapturePanics()
		defer func() {
			cp := stop()
			if r.panicked {
				r.val = recover()
				if r.val == nil {
					// recover returns nil only during runtime.Goexit: f
					// called t.Fatal, which would end the GUI goroutine and
					// with it every later test
					r.panicked, r.goexit = false, true
				}
			} else if cp != nil {
				r.panicked, r.val = true, cp
			}
			s.finish(r)
			if r.goexit {
				rescueGUIThread()
			}
		}()
		f()
		r.panicked = false
	})
	if !ok {
		syncMu.Lock()
		delete(syncWaiters, s)
		syncMu.Unlock()
		s.finish(syncResult{notRun: true})
	}
	return <-s.done
}

// rescueGUIThread runs while runtime.Goexit unwinds the GUI goroutine, which
// it cannot stop. It keeps that goroutine serving Qt from a nested event loop
// until the application quits, so later work (the next test) still runs.
func rescueGUIThread() {
	nestedLoopC()
	if AfterRescue != nil {
		AfterRescue()
	}
}

func checkThread(method string) {
	if !IsGUIThread() {
		panic(fmt.Sprintf("%s: called off the GUI thread; Qt objects may only be used on the GUI thread, "+
			"wrap the call in qute.Post(func() { ... })", method))
	}
}
