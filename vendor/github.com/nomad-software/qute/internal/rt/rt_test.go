package rt

import (
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"
	"weak"
)

// These tests cover the runtime logic that needs no QApplication: handle
// states, kept references, dependencies, the cleanup queue and PostSync's
// failure paths. Objects are fake addresses; nothing here dereferences them.

func fakePtr(n int) unsafe.Pointer {
	// distinct, never dereferenced; backed by real memory so the GC is happy
	b := make([]byte, 8)
	return unsafe.Pointer(&b[n%8])
}

func newPlain(class string) *Handle { return Wrap(fakePtr(0), class, KindPlain, KeepOrBorrow, nil) }

func TestStates(t *testing.T) {
	h := Wrap(fakePtr(0), "X", KindValue, TakeOwned, func(unsafe.Pointer) {})
	if h.State() != Owned {
		t.Fatalf("state = %v, want Owned", h.State())
	}
	h.TransferToCpp()
	if h.State() != Parented {
		t.Fatalf("after TransferToCpp: %v, want Parented", h.State())
	}
	h.TransferToGo()
	if h.State() != Owned {
		t.Fatalf("after TransferToGo: %v, want Owned", h.State())
	}
	h.markDead("test")
	h.TransferToCpp()
	if h.State() != Dead {
		t.Fatalf("a Dead handle changed state: %v", h.State())
	}
	if p := newPlain("Y"); p.State() != Borrowed {
		t.Fatalf("plain handle without deleter = %v, want Borrowed", p.State())
	}
}

func TestDeleteBorrowedPanics(t *testing.T) {
	h := newPlain("X")
	defer func() {
		if p := recover(); p == nil || !strings.Contains(p.(string), "rule 4") {
			t.Errorf("Delete on Borrowed: panic %v, want rule 4", p)
		}
	}()
	h.Delete("X.Delete")
}

func TestKeepArgSlots(t *testing.T) {
	owner, a, b := newPlain("Owner"), newPlain("A"), newPlain("B")
	KeepArg(owner, "setModel", a, false)
	if !Keeps(owner, a) {
		t.Fatal("a not kept")
	}
	KeepArg(owner, "setModel", b, false) // replaces a
	if Keeps(owner, a) || !Keeps(owner, b) {
		t.Fatalf("after replacing: keeps a=%v b=%v, want false true", Keeps(owner, a), Keeps(owner, b))
	}
	KeepArg(owner, "other", a, false) // independent slot
	KeepArg(owner, "setModel", nil, false)
	if Keeps(owner, b) || !Keeps(owner, a) {
		t.Fatalf("after clearing setModel: keeps a=%v b=%v, want true false", Keeps(owner, a), Keeps(owner, b))
	}
}

func TestKeepEachAndRelease(t *testing.T) {
	owner, a, b := newPlain("Owner"), newPlain("A"), newPlain("B")
	KeepEach(owner, a, false)
	KeepEach(owner, b, false)
	KeepEach(owner, a, false) // twice is one entry
	if !Keeps(owner, a) || !Keeps(owner, b) {
		t.Fatal("KeepEach lost an argument")
	}
	Release(owner, a)
	if Keeps(owner, a) || !Keeps(owner, b) {
		t.Fatalf("after Release(a): keeps a=%v b=%v, want false true", Keeps(owner, a), Keeps(owner, b))
	}
	Release(owner, a) // idempotent
	Release(nil, b)
	KeepEach(owner, nil, false)
}

func TestKeepsDropWhenPlainOwnerDies(t *testing.T) {
	owner, a := newPlain("Owner"), newPlain("A")
	KeepArg(owner, "s", a, false)
	owner.markDead("test")
	if Keeps(owner, a) {
		t.Fatal("a dead owner still keeps its argument")
	}
	KeepArg(owner, "s", a, false)
	if Keeps(owner, a) {
		t.Fatal("a dead owner took a new reference")
	}
}

// A registered owner can outlive its Go variables, so while it keeps
// references its handle is anchored; its death notice drops both.
func TestKeepsOfRegisteredOwnerDropOnDestroyed(t *testing.T) {
	key := fakePtr(1)
	owner := Wrap(key, "Owner", KindNotify, KeepOrBorrow, nil)
	a := newPlain("A")
	KeepArg(owner, "s", a, false)
	if owner.Anchored() != "global" {
		t.Fatalf("owner with kept references anchored %q, want global", owner.Anchored())
	}
	Destroyed(key, "test")
	if Keeps(owner, a) {
		t.Fatal("references survived the owner's death notice")
	}
	if owner.Anchored() != "" || owner.Pins() != 0 {
		t.Fatalf("dead owner still anchored (%q, %d pins)", owner.Anchored(), owner.Pins())
	}
	if owner.State() != Dead {
		t.Fatalf("owner = %v, want Dead", owner.State())
	}
}

// Clearing the last kept reference drops the anchor it took.
func TestKeepsAnchorReleased(t *testing.T) {
	key := fakePtr(2)
	owner := Wrap(key, "Owner", KindNotify, KeepOrBorrow, nil)
	defer Destroyed(key, "test")
	a := newPlain("A")
	KeepEach(owner, a, false)
	if owner.Pins() != 1 {
		t.Fatalf("pins = %d, want 1 while keeping", owner.Pins())
	}
	Release(owner, a)
	if owner.Pins() != 0 || owner.Anchored() != "" {
		t.Fatalf("after Release: pins %d, anchored %q, want 0 and none", owner.Pins(), owner.Anchored())
	}
}

type fakeWrapper struct{ r Ref }

func TestKeepsArgWrappers(t *testing.T) {
	owner, a := newPlain("Owner"), newPlain("A")
	ow, aw := &fakeWrapper{Ref{H: owner}}, &fakeWrapper{Ref{H: a}}
	KeepEachArg(unsafe.Pointer(ow), unsafe.Pointer(aw))
	if !KeepsArg(unsafe.Pointer(ow), unsafe.Pointer(aw)) {
		t.Fatal("KeepsArg = false after KeepEachArg")
	}
	if KeepsArg(nil, unsafe.Pointer(aw)) || KeepsArg(unsafe.Pointer(ow), nil) {
		t.Fatal("KeepsArg with nil = true")
	}
}

// A value class with a dependency panics once the dependency is dead.
func TestDependency(t *testing.T) {
	model := newPlain("Model")
	RegisterDependency("Index", func(unsafe.Pointer) *Handle { return model })
	defer deps.Delete("Index")
	idx := Wrap(fakePtr(3), "Index", KindValue, TakeOwned, func(unsafe.Pointer) {})
	if d := idx.Dependencies(); len(d) != 1 || d[0] != model {
		t.Fatal("dependency not attached")
	}
	// checkDependency is the part of Use under test; Use itself also checks
	// the GUI thread in qute_debug builds, and tests run off it
	checkDependency(idx, "Index.Data") // fine while the model lives
	model.markDead("deleted by Delete()")
	defer func() {
		p := recover()
		msg, _ := p.(string)
		if !strings.Contains(msg, "Index.Data") || !strings.Contains(msg, "Model this Index belongs to was destroyed") ||
			!strings.Contains(msg, "deleted by Delete()") {
			t.Errorf("Use after the dependency died: panic %v", p)
		}
	}()
	checkDependency(idx, "Index.Data")
}

func TestNoDependencyForNil(t *testing.T) {
	RegisterDependency("Index2", func(unsafe.Pointer) *Handle { return nil })
	defer deps.Delete("Index2")
	if h := Wrap(fakePtr(4), "Index2", KindValue, TakeOwned, func(unsafe.Pointer) {}); h.Dependencies() != nil {
		t.Fatal("dependency attached for a nil result")
	}
}

// Several dependencies: the panic names whichever one died.
func TestSeveralDependencies(t *testing.T) {
	table, doc := newPlain("Table"), newPlain("Doc")
	RegisterDependency("Cell", func(unsafe.Pointer) *Handle { return table })
	RegisterDependency("Cell", func(unsafe.Pointer) *Handle { return doc })
	defer deps.Delete("Cell")
	cell := Wrap(fakePtr(5), "Cell", KindValue, TakeOwned, func(unsafe.Pointer) {})
	if d := cell.Dependencies(); len(d) != 2 || d[0] != table || d[1] != doc {
		t.Fatalf("dependencies = %v, want [table doc]", d)
	}
	checkDependency(cell, "Cell.Row")
	doc.markDead("deleted by Delete()")
	defer func() {
		msg, _ := recover().(string)
		if !strings.Contains(msg, "Doc this Cell belongs to was destroyed") {
			t.Errorf("Use after the document died: panic %q", msg)
		}
	}()
	checkDependency(cell, "Cell.Row")
}

// Connections are freed on the GUI thread through the cleanup queue, never
// from the GC cleanup goroutine, and are not counted as freed objects.
func TestConnectionFreedThroughQueue(t *testing.T) {
	queueHead.Store(nil)
	defer queueHead.Store(nil)
	p := fakePtr(5)
	freeConnectionLater(p)
	n := queueHead.Load()
	if n == nil {
		t.Fatal("connection free was not queued")
	}
	if n.arg.key != p || !n.arg.quiet || n.arg.registered {
		t.Fatalf("queued %+v, want a quiet unregistered free of the connection", n.arg)
	}
}

func TestDrainSkipsQuietCount(t *testing.T) {
	queueHead.Store(nil)
	freed := 0
	before := Drained.Load()
	enqueueCleanup(cleanupArg{key: fakePtr(6), kind: KindPlain, deleter: func(unsafe.Pointer) { freed++ }, quiet: true})
	enqueueCleanup(cleanupArg{key: fakePtr(6), kind: KindPlain, deleter: func(unsafe.Pointer) { freed++ }})
	Drain()
	if freed != 2 {
		t.Fatalf("Drain freed %d, want 2", freed)
	}
	if got := Drained.Load() - before; got != 1 {
		t.Fatalf("Drained counted %d, want 1 (quiet frees are not objects)", got)
	}
}

// A kept object collected together with its keeper is freed after it, in
// whatever order the GC queued them (a page must not outlive its profile).
func TestDrainFreesKeepersFirst(t *testing.T) {
	queueHead.Store(nil)
	defer queueHead.Store(nil)
	var order []string
	keeper := &Handle{ptr: fakePtr(1), class: "Page", kind: KindPlain, serial: serialCounter.Add(1)}
	kept := &Handle{ptr: fakePtr(2), class: "Profile", kind: KindPlain, serial: serialCounter.Add(1)}
	KeepArg(keeper, "profile", kept, false)
	// the kept object first: the order a LIFO queue would free them wrongly in
	enqueueCleanup(cleanupArg{key: keeper.ptr, serial: keeper.serial, kind: KindPlain, deleter: func(unsafe.Pointer) { order = append(order, "page") }})
	enqueueCleanup(cleanupArg{key: kept.ptr, serial: kept.serial, kind: KindPlain, deleter: func(unsafe.Pointer) { order = append(order, "profile") }})
	Drain()
	if len(order) != 2 || order[0] != "page" || order[1] != "profile" {
		t.Fatalf("freed %v, want [page profile]", order)
	}
	if isKept(kept.serial) || deferredN.Load() != 0 {
		t.Fatal("hold or deferred free left behind")
	}
}

// A kept object whose keeper Qt destroys later (a page inside a window being
// freed) waits for the keeper's death notice, even when the keeper's handle
// is already gone.
func TestDrainDefersUntilKeeperDies(t *testing.T) {
	queueHead.Store(nil)
	defer queueHead.Store(nil)
	freed := 0
	key := fakePtr(3)
	keeper := Wrap(key, "Page", KindNotify, KeepOrBorrow, nil)
	kept := &Handle{ptr: fakePtr(4), class: "Profile", kind: KindPlain, serial: serialCounter.Add(1)}
	KeepArg(keeper, "profile", kept, false)
	enqueueCleanup(cleanupArg{key: kept.ptr, serial: kept.serial, kind: KindPlain, deleter: func(unsafe.Pointer) { freed++ }})
	Drain()
	if freed != 0 {
		t.Fatal("kept object freed while its keeper lives")
	}
	Destroyed(key, "test")
	Drain()
	if freed != 1 || deferredN.Load() != 0 {
		t.Fatalf("freed %d after the keeper died, want 1", freed)
	}

	// the keeper's handle collected before Qt destroys it: the registry
	// entry's serial still releases the hold
	key2 := fakePtr(5)
	keeper2 := Wrap(key2, "Page", KindNotify, KeepOrBorrow, nil)
	KeepArg(keeper2, "profile", kept, false)
	releaseHolds(keeper2.serial)
	if isKept(kept.serial) {
		t.Fatal("hold survived releaseHolds")
	}
	Destroyed(key2, "test")
}

// Replacing or clearing a slot drops the hold on the old argument.
func TestKeepSlotHolds(t *testing.T) {
	owner, a, b := newPlain("Owner"), newPlain("A"), newPlain("B")
	KeepArg(owner, "s", a, false)
	KeepArg(owner, "s", a, false) // same argument again: still one hold
	KeepArg(owner, "s", b, false)
	if isKept(a.serial) || !isKept(b.serial) {
		t.Fatalf("after replacing: a kept=%v b kept=%v, want false true", isKept(a.serial), isKept(b.serial))
	}
	KeepArg(owner, "s", nil, false)
	if isKept(b.serial) {
		t.Fatal("cleared slot still holds its argument")
	}
	KeepEach(owner, a, false)
	Release(owner, a)
	if isKept(a.serial) {
		t.Fatal("released argument still held")
	}
}

// PostSync must not wait forever when its function can never run.
func TestPostSyncPostFails(t *testing.T) {
	// No QApplication exists in this test binary, so the post itself fails.
	AppStarted()
	defer func() {
		appRunning.Store(false)
		appExited.Store(false)
	}()
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		PostSync(func() { t.Error("f ran without an application") })
	}()
	if p := <-done; p != errNotRun {
		t.Fatalf("PostSync panic = %v, want %q", p, errNotRun)
	}
}

func TestAppExitedWakesPostSyncWaiters(t *testing.T) {
	s := &syncCall{done: make(chan syncResult, 1)}
	syncMu.Lock()
	syncWaiters[s] = struct{}{}
	syncMu.Unlock()
	AppExited()
	defer appExited.Store(false)
	r := <-s.done
	if !r.notRun {
		t.Fatalf("waiter got %+v, want notRun", r)
	}
	if post(func() {}) {
		t.Fatal("post after exit reported success")
	}
}

func TestCallbackPanicCapture(t *testing.T) {
	stop := CapturePanics()
	func() {
		defer catchPanic()
		panic("handler failed")
	}()
	cp := stop()
	if cp == nil || cp.Value != "handler failed" || !strings.Contains(cp.Stack, "TestCallbackPanicCapture") {
		t.Fatalf("captured %+v, want the handler's panic with its stack", cp)
	}
	// without a capture the panic propagates as before
	defer func() {
		if p := recover(); p != "uncaptured" {
			t.Fatalf("panic = %v, want it to propagate", p)
		}
	}()
	func() {
		defer catchPanic()
		panic("uncaptured")
	}()
}

func TestRethrowCallbackPanic(t *testing.T) {
	stop := CapturePanics()
	defer stop()
	func() {
		defer catchPanic()
		panic("first")
	}()
	func() {
		defer catchPanic()
		panic("second") // only the first is kept
	}()
	defer func() {
		cp, _ := recover().(*CallbackPanic)
		if cp == nil || cp.Value != "first" {
			t.Fatalf("rethrown %+v, want the first panic", cp)
		}
	}()
	RethrowCallbackPanic()
}

// A post that saw the app not yet running, while AppStarted handed the
// pending list over, used to append to the list after the hand-over and
// never run. Now it sees the app running and posts directly.
func TestPostRacingAppStarted(t *testing.T) {
	defer func() {
		testHookPost = nil
		appRunning.Store(false)
		appExited.Store(false)
		pendingPosts = nil
	}()
	testHookPost = func() {
		testHookPost = nil
		AppStarted() // runs between post's check and its lock
	}
	post(func() {})
	pendingMu.Lock()
	n := len(pendingPosts)
	pendingMu.Unlock()
	if n != 0 {
		t.Fatalf("%d posts stranded in the pending list after the app started", n)
	}
}

// A plain object handed out by a method depends on the receiver and on what
// the receiver depends on; Alive and Use see that.
func TestWrapClassFromDependsOnReceiver(t *testing.T) {
	RegisterClass("PlainItem", KindPlain, nil)
	doc := newPlain("Doc")
	owner := newPlain("Layout")
	owner.deps = []*Handle{doc}
	it := WrapClassFrom(fakePtr(3), "PlainItem", KeepOrBorrow, owner)
	if len(it.deps) != 2 || it.deps[0] != owner || it.deps[1] != doc {
		t.Fatalf("deps = %v, want the receiver and its document", it.deps)
	}
	if !it.Alive() {
		t.Fatal("item not alive while its receiver lives")
	}
	doc.markDead("test")
	if it.Alive() {
		t.Fatal("item alive after what its receiver depends on died")
	}
	defer func() {
		if p, _ := recover().(string); !strings.Contains(p, "the Doc this PlainItem belongs to was destroyed") {
			t.Errorf("Use: panic %q, want the dependency message", p)
		}
	}()
	r := &fakeWrapper{Ref{H: it, P: fakePtr(3)}}
	Use(unsafe.Pointer(r), "PlainItem.Geometry")
}

// An owned plain result needs no dependency: Go frees it.
func TestWrapClassFromOwnedHasNoDependency(t *testing.T) {
	RegisterClass("OwnedItem", KindPlain, func(unsafe.Pointer) {})
	owner := newPlain("Layout")
	it := WrapClassFrom(fakePtr(4), "OwnedItem", TakeOwned, owner)
	if len(it.deps) != 0 {
		t.Fatalf("owned result depends on %v", it.deps)
	}
}

// fakeQObjects registers QObject handles without Qt: a fake tree for the
// anchoring logic.
func fakeQObject(t *testing.T, st State) *Handle {
	t.Helper()
	b := make([]byte, 8)
	h := newHandle(unsafe.Pointer(&b[0]), "QObject", KindQObject, st, nil)
	regMu.Lock()
	registry[h.ptr] = regEntry{w: weak.Make(h), serial: h.serial}
	regMu.Unlock()
	t.Cleanup(func() {
		regMu.Lock()
		delete(registry, h.ptr)
		regMu.Unlock()
		runtime.KeepAlive(b)
	})
	return h
}

func withFakeTree(t *testing.T, parent map[*Handle]*Handle) {
	t.Helper()
	oldRoot, oldSub := rootOfC, subtreeC
	t.Cleanup(func() { rootOfC, subtreeC = oldRoot, oldSub })
	byPtr := map[unsafe.Pointer]*Handle{}
	for c, p := range parent {
		byPtr[c.ptr] = c
		if p != nil {
			byPtr[p.ptr] = p
		}
	}
	rootOfC = func(p unsafe.Pointer) unsafe.Pointer {
		h := byPtr[p]
		for parent[h] != nil {
			h = parent[h]
		}
		return h.ptr
	}
	subtreeC = func(p unsafe.Pointer) []unsafe.Pointer {
		out := []unsafe.Pointer{p}
		for c, par := range parent {
			if par != nil && par.ptr == p {
				out = append(out, subtreeC(c.ptr)...)
			}
		}
		return out
	}
}

// Go state of an object in a tree is held by the tree's Owned root, so the
// whole tree can be collected; a root that stops being Owned hands it to the
// global set, and so does a move between parents.
func TestAnchoring(t *testing.T) {
	root := fakeQObject(t, Owned)
	child := fakeQObject(t, Parented)
	withFakeTree(t, map[*Handle]*Handle{root: nil, child: root})

	child.anchorPin()
	if got := child.Anchored(); got != "root" {
		t.Fatalf("child anchored %q, want root", got)
	}
	if child.Pins() != 0 {
		t.Fatalf("child anchored by its root is also pinned globally (%d)", child.Pins())
	}
	if _, ok := root.x().anchored[child]; !ok {
		t.Fatal("root does not hold the child")
	}

	root.anchorPin() // a root's own state is global: rule "pinned while connected"
	if got := root.Anchored(); got != "global" || root.Pins() != 1 {
		t.Fatalf("root anchored %q with %d pins, want global and 1", got, root.Pins())
	}
	root.anchorUnpin()

	reanchorSubtree(child.ptr, true) // between parents
	if got := child.Anchored(); got != "global" || child.Pins() != 1 {
		t.Fatalf("moving child anchored %q with %d pins, want global and 1", got, child.Pins())
	}
	reanchorSubtree(child.ptr, false) // arrived (back) under root
	if got := child.Anchored(); got != "root" || child.Pins() != 0 {
		t.Fatalf("re-added child anchored %q with %d pins, want root and 0", got, child.Pins())
	}

	root.TransferToCpp() // a Parented root may be collected: it holds nothing
	if got := child.Anchored(); got != "global" || child.Pins() != 1 {
		t.Fatalf("after root became Parented: child anchored %q with %d pins, want global", got, child.Pins())
	}
	child.anchorUnpin()
	if child.Anchored() != "" || child.Pins() != 0 {
		t.Fatalf("released child anchored %q with %d pins", child.Anchored(), child.Pins())
	}
}

// The id a shim dispatches virtuals with refers to its handle weakly. When
// the handle of a Go subclass is collected with its tree, before Qt destroys
// the object, the id's table entry must go too: it used to stay for good, one
// entry per collected Go subclass.
func TestSelfSlotDroppedWithCollectedHandle(t *testing.T) {
	// nothing but the test's variables may hold the handles, so no helpers
	mem := make([]byte, 16)
	rootP, childP := unsafe.Pointer(&mem[0]), unsafe.Pointer(&mem[8])
	root := newHandle(rootP, "QObject", KindQObject, Owned, nil)
	child := newHandle(childP, "QObject", KindQObject, Parented, nil)
	regMu.Lock()
	registry[rootP] = regEntry{w: weak.Make(root), serial: root.serial}
	registry[childP] = regEntry{w: weak.Make(child), serial: child.serial}
	regMu.Unlock()
	oldRoot := rootOfC
	rootOfC = func(unsafe.Pointer) unsafe.Pointer { return rootP }
	defer func() {
		rootOfC = oldRoot
		regMu.Lock()
		delete(registry, rootP)
		delete(registry, childP)
		regMu.Unlock()
	}()
	id := child.SetSelf(&struct{ n int }{}, nil)
	if child.Anchored() != "root" {
		t.Fatalf("child anchored %q, want root", child.Anchored())
	}
	if slotHandle(id) != child {
		t.Fatal("the id does not find its handle")
	}
	root, child = nil, nil
	has := func() bool { _, ok := slots.Load(id); return ok }
	for i := 0; i < 50 && has(); i++ {
		runtime.GC()
		time.Sleep(time.Millisecond)
	}
	if has() {
		t.Fatal("the id of a collected Go subclass is still in the table")
	}
	if Dispatch(id, 0, nil, nil, false) {
		t.Fatal("a virtual dispatched to a collected Go value")
	}
}

// A connection's closure lives on its sender's handle; the id table holds the
// handle weakly, and releasing the connection drops the closure and anchor.
func TestCallbackSlots(t *testing.T) {
	sender := Wrap(fakePtr(5), "Sender", KindNotify, KeepOrBorrow, nil)
	defer Destroyed(sender.ptr, "test")
	ran := 0
	id := NewCallback(sender, func(unsafe.Pointer) { ran++ })
	if sender.Pins() != 1 {
		t.Fatalf("connected sender pins = %d, want 1", sender.Pins())
	}
	InvokeCallback(id, nil, false)
	if ran != 1 {
		t.Fatalf("callback ran %d times, want 1", ran)
	}
	ReleaseCallback(id)
	InvokeCallback(id, nil, false) // released: nothing runs
	if ran != 1 || sender.Pins() != 0 || len(sender.x().callbacks) != 0 {
		t.Fatalf("after release: ran %d, pins %d, %d closures", ran, sender.Pins(), len(sender.x().callbacks))
	}
	// std::function ids never collide with slot ids
	fid := NewFunc(func(_, _ unsafe.Pointer) {})
	if fid&funcBit == 0 {
		t.Fatal("NewFunc id lacks the func bit")
	}
	ReleaseCallback(fid)
}

// A plain keeper that Qt takes over (Parented, so Go never frees it) releases
// what it keeps once its handle is collected; while Go frees it, only that
// free releases them (after the keeper is gone).
func TestPlainKeeperReleaseFollowsOwnership(t *testing.T) {
	owner := Wrap(fakePtr(1), "Keeper", KindPlain, TakeOwned, func(unsafe.Pointer) {})
	kept := newPlain("Kept")
	KeepArg(owner, "s", kept, false)
	if owner.x().hasRelease {
		t.Fatal("an Owned keeper releases on collection; its free should")
	}
	owner.TransferToCpp()
	if !owner.x().hasRelease {
		t.Fatal("a Parented keeper has no release cleanup")
	}
	owner.TransferToGo()
	if owner.x().hasRelease {
		t.Fatal("back to Owned, the release cleanup stayed")
	}
	owner.TransferToCpp()
	serial := kept.serial
	owner = nil
	for i := 0; i < 20 && isKept(serial); i++ {
		runtime.GC()
		time.Sleep(time.Millisecond)
	}
	if isKept(serial) {
		t.Fatal("the collected Parented keeper still holds its argument")
	}
}

// A guarded hold makes Delete of the kept object panic; clearing it allows it.
func TestGuardedKeepBlocksDelete(t *testing.T) {
	owner := newPlain("Painter")
	kept := Wrap(fakePtr(2), "Image", KindValue, TakeOwned, func(unsafe.Pointer) {})
	KeepArg(owner, "device", kept, true)
	func() {
		defer func() {
			if p, _ := recover().(string); !strings.Contains(p, "a Painter still uses this Image") {
				t.Errorf("Delete of a guarded object: panic %q", p)
			}
		}()
		kept.Delete("Image.Delete")
	}()
	ReleaseSlot(owner, "device")
	kept.Delete("Image.Delete")
	if kept.State() != Dead {
		t.Fatalf("kept = %v after Delete, want Dead", kept.State())
	}
}

// Qt handing an object out again after GC collected its Owned handle, before
// the queued free ran: the free hands the object to the new handle instead
// of deleting it underneath.
func TestQueuedFreeOfRewrappedObject(t *testing.T) {
	key := fakePtr(3)
	old := serialCounter.Add(1) // the collected Owned handle, whose free is queued
	regMu.Lock()
	registry[key] = regEntry{serial: old} // its weak pointer is empty: collected
	regMu.Unlock()
	h := Wrap(key, "Obj", KindNotify, KeepOrBorrow, func(unsafe.Pointer) { t.Error("object deleted under its new handle") })
	defer Destroyed(key, "test")
	if h.State() != Borrowed {
		t.Fatalf("re-wrapped: %v, want Borrowed", h.State())
	}
	freeNode(cleanupArg{key: key, serial: old, kind: KindNotify, registered: true, deleter: h.deleter})
	if h.State() != Owned {
		t.Fatalf("after the stale free: %v, want Owned (Go still owns it)", h.State())
	}
	h.stopCleanup()
}

// When the safety net refuses to free an object Qt has parented, it says so
// once per class instead of letting the tree's handlers stop silently.
func TestParentedFreeWarns(t *testing.T) {
	old, oldOut := hasParentC, warnOut
	var out strings.Builder
	hasParentC, warnOut = func(unsafe.Pointer) bool { return true }, &out
	defer func() { hasParentC, warnOut = old, oldOut }()
	for i := 0; i < 2; i++ {
		h := fakeQObject(t, Owned)
		freeNode(cleanupArg{key: h.ptr, serial: h.serial, class: "QWarnedThing", kind: KindQObject, registered: true,
			deleter: func(unsafe.Pointer) { t.Error("an object with a parent was deleted") }})
	}
	if n := strings.Count(out.String(), "QWarnedThing that Qt has given a parent"); n != 1 {
		t.Errorf("warned %d times, want once: %q", n, out.String())
	}
}

// An Unpin racing a Pin cannot slip in between the count and the strong
// set: the handle ends up held iff it has pins. (Pin used to change the
// count, then take the lock; an Unpin in that gap deleted the handle from
// the set, and Pin then added it back with no pins.)
func TestPinUnpinRace(t *testing.T) {
	h := newPlain("Raced")
	h.Pin() // the racing Unpin's pin
	var wg sync.WaitGroup
	testHookPin = func() {
		testHookPin = nil
		wg.Add(1)
		go func() { defer wg.Done(); h.Unpin(); h.Unpin() }()
		time.Sleep(20 * time.Millisecond) // the Unpins run now if nothing stops them
	}
	defer func() { testHookPin = nil }()
	h.Unpin() // back to 0, so the next Pin reaches the hook
	h.Pin()
	wg.Wait()
	strongMu.Lock()
	_, held := strong[h]
	strongMu.Unlock()
	if held != (h.Pins() > 0) {
		t.Fatalf("pins %d but held in the strong set = %v", h.Pins(), held)
	}
}

// Keepers Go built on a scoped argument and left alive end when it expires,
// newest first, while Qt's object is still there for their destructors; a
// keeper that let go of it, or that Go does not own, is left alone.
func TestScopedArgumentEndsItsKeepers(t *testing.T) {
	var order []string
	plain := func(class string) *Handle {
		return Wrap(fakePtr(0), class, KindPlain, TakeOwned, func(unsafe.Pointer) { order = append(order, class) })
	}
	device := Wrap(fakePtr(0), "Device", KindPlain, NewScoped, nil)
	painter := plain("Painter") // paints on the scoped device
	guard := plain("Guard")     // restores that painter
	ended := plain("Ended")     // let go of the device before the handler returned
	borrowed := newPlain("Borrowed")
	other := plain("Other") // keeps something else
	KeepArg(painter, "device", device, true)
	KeepArg(guard, "painter", painter, true)
	KeepArg(ended, "device", device, true)
	ReleaseSlot(ended, "device")
	KeepArg(borrowed, "device", device, true)
	KeepArg(other, "x", newPlain("X"), true)

	device.Expire()
	if got := strings.Join(order, ","); got != "Guard,Painter" {
		t.Errorf("deleted %q, want Guard then Painter", got)
	}
	if guard.Alive() || painter.Alive() || device.Alive() {
		t.Errorf("alive after Expire: guard %v, painter %v, device %v", guard.Alive(), painter.Alive(), device.Alive())
	}
	if !ended.Alive() || !borrowed.Alive() || !other.Alive() {
		t.Errorf("ended %v, borrowed %v, other %v; want all alive", ended.Alive(), borrowed.Alive(), other.Alive())
	}
	if device.x().bound != nil || painter.x().scope != nil {
		t.Error("scope binding left behind")
	}
	if msg := deadMessage(guard, "Guard.Restore"); !strings.Contains(msg, "the Device argument it used expired") {
		t.Errorf("dead message: %q", msg)
	}
	ended.Delete("Ended.Delete")
	other.Delete("Other.Delete")
}

// A container passed to its own transfer-to-cpp method (an anchor layout
// anchored to its own edges) does not come to own itself.
func TestTransferToSelf(t *testing.T) {
	lay := Wrap(fakePtr(0), "Layout", KindPlain, TakeOwned, func(unsafe.Pointer) {})
	TransferTo(lay, lay)
	if lay.State() != Owned {
		t.Errorf("layout passed to itself: %v, want Owned", lay.State())
	}
	lay.Delete("Layout.Delete")
}

// registerFakeQObject registers a QObject handle at key without the death
// notice connection a real one gets (there is no Qt here).
func registerFakeQObject(key unsafe.Pointer) *Handle {
	h := newHandle(key, "QObject", KindQObject, Borrowed, nil)
	regMu.Lock()
	registry[key] = regEntry{w: weak.Make(h), serial: h.serial, watched: true}
	regMu.Unlock()
	return h
}

// A QObject wrapped while Qt destroys it (the argument of its destroyed
// signal) gets the handle it had, Dead, or a fresh Dead one, and nothing is
// registered: a registered handle would never learn of its death, and would
// be handed out for the next object at the same address.
func TestWrapDyingQObject(t *testing.T) {
	oldDying := dyingC
	t.Cleanup(func() { dyingC = oldDying })
	key := fakePtr(2)
	h := registerFakeQObject(key)
	Destroyed(key, "destroyed by Qt")
	dyingC = func(p unsafe.Pointer) bool { return p == key }

	if got := Wrap(key, "QObject", KindQObject, KeepOrBorrow, nil); got != h {
		t.Fatal("the dying object got a new handle, not the one it had")
	}
	if h.State() != Dead {
		t.Fatalf("handle of the dying object = %v, want Dead", h.State())
	}
	if Lookup(key) != nil {
		t.Fatal("the dying object was registered")
	}

	// once the event loop runs again, no destructor is running: what is at
	// the address now is another object, and a dying one there is new to Go
	sweepDying()
	fresh := Wrap(key, "QObject", KindQObject, KeepOrBorrow, nil)
	if fresh == h || fresh.State() != Dead {
		t.Fatalf("a dying object Go never had: %p (old %p), %v, want a fresh Dead handle", fresh, h, fresh.State())
	}
	if Lookup(key) != nil {
		t.Fatal("the dying object was registered")
	}

	// a live object registered at the address forgets the dead one
	dyingC = func(unsafe.Pointer) bool { return false }
	regMu.Lock()
	buryLocked(key, h)
	regMu.Unlock()
	dyingSweepPost.Store(false)
	live := registerFakeQObject(key)
	regMu.Lock()
	delete(dying, key) // as Wrap does when it registers a new object
	regMu.Unlock()
	if Lookup(key) != live {
		t.Fatal("the live object's handle is not the registered one")
	}
	regMu.Lock()
	delete(registry, key)
	regMu.Unlock()
}

// An inline value is a copy in Go memory: wrapping it copies the bytes, no
// cleanup is attached, Delete only marks it Dead, and lists cross as a block
// of the values, a nil element being a default one.
func TestInlineValues(t *testing.T) {
	RegisterClass("Inline8", KindValue, func(unsafe.Pointer) { t.Error("an inline value was deleted through C++") })
	RegisterInline("Inline8", 8, func(p unsafe.Pointer) { *(*int64)(p) = -1 })
	src := int64(42)
	h := WrapClass(unsafe.Pointer(&src), "Inline8", CopyInline)
	if !h.inline || h.State() != Owned || h.hasCleanupNow() {
		t.Fatalf("inline handle: inline=%v state=%v cleanup=%v", h.inline, h.State(), h.hasCleanupNow())
	}
	src = 7
	if *(*int64)(h.ptr) != 42 {
		t.Fatal("the inline value is not a copy")
	}
	type wrapper struct{ Ref }
	w := View[wrapper](h, unsafe.Pointer(&src))
	if w.P != h.ptr {
		t.Fatal("the wrapper points at the C++ object, not the Go copy")
	}
	block := InlineBlock("Inline8", "test", 2, func(i int) unsafe.Pointer {
		if i == 0 {
			return unsafe.Pointer(w)
		}
		return nil
	}, func(n int) unsafe.Pointer { return unsafe.Pointer(unsafe.SliceData(make([]int64, n/8))) })
	if got := unsafe.Slice((*int64)(block), 2); got[0] != 42 || got[1] != -1 {
		t.Fatalf("block = %v, want [42 -1]", got)
	}
	var out int64
	CopyValueOut(unsafe.Pointer(&out), unsafe.Pointer(w), "Inline8", "test")
	if out != 42 {
		t.Fatalf("CopyValueOut wrote %d", out)
	}
	h.Delete("test")
	if h.State() != Dead {
		t.Fatalf("after Delete: %v", h.State())
	}
}

type maskPaints struct{}

func (maskPaints) Paint() {}

type maskPlain struct{}

// Which virtuals a Go type overrides is worked out once per type and table:
// later objects of the type reuse the answer, other types and tables get
// their own.
func TestMaskCachedPerType(t *testing.T) {
	has := func(v any) bool { _, ok := v.(interface{ Paint() }); return ok }
	vt := []Virtual{{Name: "Size"}, {Name: "Paint", Has: has}}
	vt[0].Has = func(any) bool { return false }
	a := Mask(maskPaints{}, vt)
	if len(a) != 1 || a[0] != 0b10 {
		t.Fatalf("mask of a type overriding Paint = %b, want 10", a)
	}
	if b := Mask(maskPaints{}, vt); &b[0] != &a[0] {
		t.Error("the mask of a type was worked out again, not reused")
	}
	if p := Mask(maskPlain{}, vt); p[0] != 0 {
		t.Errorf("mask of a type overriding nothing = %b", p)
	}
	other := []Virtual{{Name: "Paint", Has: has}}
	if o := Mask(maskPaints{}, other); o[0] != 0b1 {
		t.Errorf("mask for another table = %b, want 1", o)
	}
}

// An override's inline argument takes the objects it depends on from what the
// shim found in C++ (qute_inline_arg), with no call to ask again.
func TestInlineArgDependencies(t *testing.T) {
	RegisterClass("InlineDep", KindValue, func(unsafe.Pointer) { t.Error("an inline argument was deleted through C++") })
	RegisterInline("InlineDep", 8, func(p unsafe.Pointer) {})
	model := newPlain("Model")
	modelPtr := fakePtr(3)
	RegisterDependency("InlineDep", func(unsafe.Pointer) *Handle {
		t.Error("the dependency was looked up through C++")
		return nil
	})
	RegisterDependencyWrap("InlineDep", func(d unsafe.Pointer) *Handle {
		if d != modelPtr {
			t.Errorf("wrap got %p, want the model the shim passed", d)
		}
		return model
	})
	v := int64(5)
	arg := inlineArg{p: unsafe.Pointer(&v)}
	arg.deps[0] = modelPtr
	h := InlineArg(unsafe.Pointer(&arg), "InlineDep")
	if *(*int64)(h.ptr) != 5 || !h.inline {
		t.Fatal("the argument was not copied")
	}
	if len(h.deps) != 1 || h.deps[0] != model {
		t.Fatalf("deps = %v, want the model", h.deps)
	}
	model.markDead("test")
	if h.Alive() {
		t.Error("the argument is alive after its model died")
	}
}

// An override's inline argument costs two allocations: its bytes, and its
// handle and wrapper made together (with the first dependency kept inside
// the handle). A heap copy C++ made for Go is copied and freed at once.
func TestInlineViews(t *testing.T) {
	freed := 0
	RegisterClass("InlineView", KindValue, func(unsafe.Pointer) { freed++ })
	RegisterInline("InlineView", 8, func(p unsafe.Pointer) {})
	model := newPlain("Model")
	RegisterDependency("InlineView", func(unsafe.Pointer) *Handle { return model })
	RegisterDependencyWrap("InlineView", func(unsafe.Pointer) *Handle { return model })
	type wrapper struct{ Ref }
	v := int64(9)
	arg := inlineArg{p: unsafe.Pointer(&v)}
	arg.deps[0] = fakePtr(4)
	w := InlineArgView[wrapper](unsafe.Pointer(&arg), "InlineView")
	if w.H == nil || *(*int64)(w.P) != 9 || w.P != w.H.ptr || len(w.H.deps) != 1 || w.H.deps[0] != model {
		t.Fatalf("argument view: %+v", w.Ref)
	}
	if View[wrapper](w.H, unsafe.Pointer(&v)) != w {
		t.Error("View does not return the wrapper made with the handle")
	}
	// (a debug build also records each handle's call site)
	if n := testing.AllocsPerRun(100, func() { InlineArgView[wrapper](unsafe.Pointer(&arg), "InlineView") }); n > 2 && !debugEnabled {
		t.Errorf("an inline argument costs %v allocations, want 2", n)
	}
	o := InlineOwnedView[wrapper](unsafe.Pointer(&v), "InlineView")
	if freed != 1 || *(*int64)(o.P) != 9 || o.H.State() != Owned || len(o.H.deps) != 1 {
		t.Errorf("owned view: freed %d, state %v, deps %d", freed, o.H.State(), len(o.H.deps))
	}
}

// A destructor may run a Go handler that runs the event loop, and with it a
// nested Drain. What the nested one could not free yet (its keeper lives)
// must still be waiting when the outer one returns. (The outer Drain used to
// overwrite the list, so such an object was never freed.)
func TestNestedDrainKeepsDeferred(t *testing.T) {
	queueHead.Store(nil)
	defer queueHead.Store(nil)
	freed := 0
	key := fakePtr(3)
	keeper := Wrap(key, "Page", KindNotify, KeepOrBorrow, nil)
	kept := &Handle{ptr: fakePtr(4), class: "Profile", kind: KindPlain, serial: serialCounter.Add(1)}
	KeepArg(keeper, "profile", kept, false)
	enqueueCleanup(cleanupArg{key: fakePtr(5), kind: KindPlain, quiet: true, deleter: func(unsafe.Pointer) {
		// queued while the outer Drain runs, and drained inside it
		enqueueCleanup(cleanupArg{key: kept.ptr, serial: kept.serial, kind: KindPlain, deleter: func(unsafe.Pointer) { freed++ }})
		Drain()
	}})
	Drain()
	if freed != 0 {
		t.Fatal("kept object freed while its keeper lives")
	}
	if deferredN.Load() != 1 {
		t.Fatalf("%d frees waiting after a nested Drain, want 1", deferredN.Load())
	}
	Destroyed(key, "test")
	Drain()
	if freed != 1 || deferredN.Load() != 0 {
		t.Fatalf("freed %d after the keeper died (%d waiting), want 1 (0)", freed, deferredN.Load())
	}
}

// A keeper whose handle GC collected and that Qt then hands out again gets a
// new handle, with a new serial. Its death releases what the first handle
// kept as well, or that object's free would wait forever.
func TestDeathReleasesHoldsOfEarlierHandle(t *testing.T) {
	key := fakePtr(6)
	first := Wrap(key, "View", KindNotify, KeepOrBorrow, nil)
	kept := &Handle{ptr: fakePtr(7), class: "Model", kind: KindPlain, serial: serialCounter.Add(1)}
	KeepArg(first, "model", kept, false)
	regMu.Lock()
	e := registry[key]
	e.w = weak.Pointer[Handle]{} // as if the first handle were collected
	registry[key] = e
	regMu.Unlock()
	second := Wrap(key, "View", KindNotify, KeepOrBorrow, nil)
	if second == first {
		t.Fatal("the collected handle was handed out again")
	}
	Destroyed(key, "test")
	if isKept(kept.serial) {
		t.Fatal("the hold of the keeper's first handle outlived the keeper")
	}
	runtime.KeepAlive(first)
}

// Debug call sites name the program's code, its main function included:
// most small programs build their window inside the function they pass to
// qute.Main. Only the main that "go test" generates is passed over. (Every
// frame of main.main used to be skipped, so such objects had no call site.)
func TestUserFrame(t *testing.T) {
	for _, c := range []struct {
		fn, file string
		want     bool
	}{
		{"main.main", "/app/main.go", true},
		{"main.main.func1", "/app/main.go", true},
		{"main.build", "/app/window.go", true},
		{"main.main", "_testmain.go", false},
		{"example.com/app_test.TestWindow.func1", "/app/window_test.go", true},
		{modulePrefix + "qtwidgets.NewQWidget", "/qute/qtwidgets/qwidget_gen.go", false},
		{modulePrefix + "qtwidgets_test.TestLayout", "/qute/qtwidgets/layout_test.go", true},
		{strings.TrimSuffix(modulePrefix, "/") + ".Main", "/qute/qute.go", false},
		{modulePrefix + "internal/rt.Wrap", "/qute/internal/rt/handle.go", false},
		{"runtime.main", "/go/src/runtime/proc.go", false},
		{"testing.tRunner", "/go/src/testing/testing.go", false},
		{"_cgoexp_1234", "_cgo_export.c", false},
		{"", "", false},
	} {
		if got := userFrame(c.fn, c.file); got != c.want {
			t.Errorf("userFrame(%q, %q) = %v, want %v", c.fn, c.file, got, c.want)
		}
	}
}

// Asking about a handle makes no extension: the reparenting walk asks every
// wrapped object in a subtree whether it is anchored, and DumpTree every
// object of a tree.
func TestQueriesMakeNoExtension(t *testing.T) {
	if debugEnabled {
		t.Skip("a debug build records every handle's call site in its extension")
	}
	h, other := newPlain("Asked"), newPlain("Other")
	h.Unkeep()
	h.anchorUnpin()
	h.releaseAnchored()
	ReleaseSlot(h, "slot")
	Release(h, other)
	if h.Anchored() != "" || Keeps(h, other) || h.GoValue() != nil {
		t.Fatal("a fresh handle reports state it does not have")
	}
	if h.ext.Load() != nil {
		t.Fatal("a query made the handle's extension")
	}
}

// Before the application exists there is no GUI thread to be off: a debug
// build checks no more than the generated shims do, so a goroutine may use a
// value then (a test goroutine here). The debug check used to compare with
// the main OS thread, and panicked where a release build does not.
func TestUseBeforeApplication(t *testing.T) {
	type wrapper struct{ Ref }
	h := Wrap(fakePtr(1), "Early", KindValue, TakeOwned, func(unsafe.Pointer) {})
	defer h.stopCleanup()
	w := View[wrapper](h, h.ptr)
	if Use(unsafe.Pointer(w), "Early.Method") != h.ptr {
		t.Fatal("Use returned another pointer")
	}
	CheckThread("Early.Method")
	GUIOnly("Early.Method")
}

// Handles of objects with no other state (list rows) make no extension,
// neither when Go takes them nor when they die, and still say why they died:
// Delete's reason stands over the death notice that follows it, and an
// uncommon reason is kept too.
func TestDeathMakesNoExtension(t *testing.T) {
	if debugEnabled {
		t.Skip("debug builds record call sites in the extension")
	}
	key := fakePtr(3)
	h := Wrap(key, "Row", KindNotify, TakeOwned, func(unsafe.Pointer) {})
	v := Wrap(fakePtr(4), "Value", KindValue, TakeOwned, func(unsafe.Pointer) {})
	Destroyed(key, "destroyed by Qt")
	v.markDead("freed by GC")
	for _, x := range []*Handle{h, v} {
		if x.ext.Load() != nil {
			t.Errorf("%s: dying made an extension", x.class)
		}
	}
	if msg := deadMessage(h, "Row.Text"); !strings.Contains(msg, "destroyed by Qt") {
		t.Errorf("dead message %q does not name the reason", msg)
	}

	key = fakePtr(5)
	d := Wrap(key, "Row", KindNotify, TakeOwned, func(p unsafe.Pointer) { Destroyed(p, "destroyed by Qt") })
	d.Delete("Row.Delete")
	if msg := deadMessage(d, "Row.Text"); !strings.Contains(msg, "deleted by Delete()") {
		t.Errorf("dead message %q, want Delete's reason", msg)
	}

	o := newPlain("Other")
	o.markDead("ended with its handler")
	o.markDead("freed by GC")
	if why := deathNote(o); why != "ended with its handler" {
		t.Errorf("death note %q, want the uncommon reason kept", why)
	}
}

// A constructor's handle and wrapper are made together and View finds the
// wrapper; the state follows the mode. An address that already has a live
// handle (only a plain object's own: it is never registered) keeps it, and
// View makes the wrapper as usual.
func TestWrapNew(t *testing.T) {
	type wrapper struct{ Ref }
	RegisterClass("NewRow", KindNotify, func(p unsafe.Pointer) { Destroyed(p, "destroyed by Qt") })
	if WrapNew[wrapper](nil, "NewRow", NewOwned) != nil {
		t.Error("a nil pointer was wrapped")
	}
	key := fakePtr(1)
	h := WrapNew[wrapper](key, "NewRow", NewOwned)
	defer h.Delete("NewRow.Delete")
	if h.State() != Owned || Lookup(key) != h {
		t.Fatalf("state %v, registered %v; want an Owned registered handle", h.State(), Lookup(key) == h)
	}
	w := View[wrapper](h, key)
	if pair := (*viewPair[wrapper])(unsafe.Pointer(h)); w != &pair.v || w.H != h || w.P != key {
		t.Error("the wrapper is not the one made with the handle")
	}
	if WrapClass(key, "NewRow", KeepOrBorrow) != h {
		t.Error("wrapping the object again gave another handle")
	}

	pkey := fakePtr(2)
	ph := WrapNew[wrapper](pkey, "NewRow", NewParented)
	if ph.State() != Parented || ph.hasCleanupNow() {
		t.Errorf("parented constructor: %v, cleanup %v; want Parented without cleanup", ph.State(), ph.hasCleanupNow())
	}
	Destroyed(pkey, "destroyed by Qt")

	// already registered and alive: that handle stands, with its own wrapper
	reg := Wrap(fakePtr(3), "NewRow", KindNotify, KeepOrBorrow, nil)
	defer Destroyed(reg.ptr, "test")
	if got := WrapNew[wrapper](reg.ptr, "NewRow", NewParented); got != reg || View[wrapper](got, reg.ptr) == nil {
		t.Error("an existing handle was replaced")
	}
}

// A lend guards an object for one call (PaintOn's device): Delete() panics
// naming the user while any lend is open, and works again after the last.
func TestLendGuardsDelete(t *testing.T) {
	freed := 0
	dev := Wrap(fakePtr(1), "QImage", KindValue, TakeOwned, func(unsafe.Pointer) { freed++ })
	Lend(dev, "QPainter")
	Lend(dev, "QPainter") // nested
	mustPanic := func() {
		t.Helper()
		defer func() {
			if p, _ := recover().(string); !strings.Contains(p, "a QPainter still uses this QImage") {
				t.Errorf("Delete of a lent object: panic %q, want it to name the QPainter", p)
			}
		}()
		dev.Delete("QImage.Delete")
	}
	mustPanic()
	Unlend(dev)
	mustPanic()
	Unlend(dev)
	dev.Delete("QImage.Delete")
	if freed != 1 || dev.State() != Dead {
		t.Errorf("after the last Unlend: freed %d, state %v; want 1, Dead", freed, dev.State())
	}
	Lend(nil, "QPainter") // a nil handle is ignored
	Unlend(nil)
}

// WrapScoped makes a scoped, Borrowed handle with its wrapper cached; Expire
// makes it Dead with the scoped message.
func TestWrapScoped(t *testing.T) {
	type wrapper struct{ Ref }
	p := fakePtr(2)
	h := WrapScoped[wrapper](p, "QPainter")
	if h.State() != Borrowed || !h.scoped {
		t.Fatalf("state %v scoped %v, want Borrowed true", h.State(), h.scoped)
	}
	w := View[wrapper](h, p)
	if w != h.cache1[0] || RefOf(unsafe.Pointer(w)).H != h {
		t.Error("the wrapper made with the handle is not the one View returns")
	}
	if Lookup(p) != nil {
		t.Error("a scoped handle was registered")
	}
	h.Expire()
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "rule 6") {
			t.Errorf("use after Expire: panic %q, want rule 6", r)
		}
	}()
	Use(unsafe.Pointer(w), "QPainter.End")
}
