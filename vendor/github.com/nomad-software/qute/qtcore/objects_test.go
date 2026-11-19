package qtcore_test

import (
	"os"
	"slices"
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qutetest"
)

// filter records the events an object receives and hands each to on.
type filter struct {
	qtcore.QObjectBase
	on func(e *qtcore.QEvent)
}

func (f *filter) EventFilter(watched *qtcore.QObject, e *qtcore.QEvent) bool {
	if f.on != nil {
		f.on(e)
	}
	return false
}

func TestMetaObject(t *testing.T) {
	qutetest.Run(t, func() {
		tm := qtcore.NewQTimer(nil)
		defer tm.Delete()
		mo := tm.MetaObject()
		if mo.ClassName() != "QTimer" || mo.SuperClass().ClassName() != "QObject" {
			t.Fatalf("class %q, super %q", mo.ClassName(), mo.SuperClass().ClassName())
		}
		obj := qtcore.NewQObject(nil)
		defer obj.Delete()
		if !mo.Inherits(obj.MetaObject()) || obj.MetaObject().Inherits(mo) {
			t.Error("Inherits")
		}
		if mo.Cast(tm) != tm.AsQObject() || mo.Cast(obj) != nil {
			t.Error("Cast")
		}

		prop := mo.Property(mo.IndexOfProperty("interval"))
		if !prop.IsValid() || prop.Name() != "interval" || !prop.IsWritable() || prop.EnclosingMetaObject().ClassName() != "QTimer" {
			t.Fatalf("property %q", prop.Name())
		}
		if !prop.Write(tm, qtcore.NewQVariantOf(77)) || tm.Interval() != 77 || prop.Read(tm).ToInt() != 77 {
			t.Errorf("write/read: %d", tm.Interval())
		}

		sig := qtcore.QMetaObjectNormalizedSignature("start( int )")
		if string(sig) != "start(int)" {
			t.Errorf("normalized %q", sig)
		}
		m := mo.Method(mo.IndexOfMethod(string(sig)))
		if string(m.Name()) != "start" || m.ParameterCount() != 1 || m.MethodType() != qtcore.QMetaMethodSlot || m.EnclosingMetaObject().ClassName() != "QTimer" {
			t.Errorf("method %q, %d parameters, type %v", m.Name(), m.ParameterCount(), m.MethodType())
		}
		if mo.IndexOfSignal("timeout()") < 0 || !qtcore.QMetaObjectCheckConnectArgs("timeout()", "f()") {
			t.Error("timeout signal")
		}

		anim := qtcore.NewQPauseAnimation(nil)
		defer anim.Delete()
		amo := anim.MetaObject()
		e := amo.Enumerator(amo.IndexOfEnumerator("State"))
		if !e.IsValid() || e.Scope() != "QAbstractAnimation" || e.KeyCount() != 3 || e.IsFlag() {
			t.Fatalf("enum %q in %q, %d keys", e.Name(), e.Scope(), e.KeyCount())
		}
		if e.ValueToKey(2) != "Running" || e.EnclosingMetaObject().ClassName() != "QAbstractAnimation" {
			t.Errorf("value 2 is %q", e.ValueToKey(2))
		}
		if v := e.KeyToValue("Paused"); v != 1 {
			t.Errorf("Paused is %d", v)
		}

		ci := qtcore.NewQMetaClassInfo()
		if ci.Name() != "" || ci.Value() != "" || ci.EnclosingMetaObject() != nil || mo.ClassInfoCount() != 0 {
			t.Error("class info of a class without any")
		}
		if qtcore.NewQMetaMethod().IsValid() || qtcore.NewQMetaProperty().IsValid() || qtcore.NewQMetaEnum().IsValid() {
			t.Error("default meta objects are valid")
		}
	})
}

func TestProxyModels(t *testing.T) {
	qutetest.Run(t, func() {
		a := qtcore.NewQStringListModelWithStrings([]string{"a", "b"}, nil)
		b := qtcore.NewQStringListModelWithStrings([]string{"c", "d", "e"}, nil)
		defer a.Delete()
		defer b.Delete()

		id := qtcore.NewQIdentityProxyModel(nil)
		id.SetSourceModel(a)
		if id.RowCount() != 2 || id.Data(id.Index(1, 0)).ToString() != "b" || !id.HandleSourceDataChanges() {
			t.Errorf("identity: %d rows", id.RowCount())
		}
		id.Delete()

		tr := qtcore.NewQTransposeProxyModel(nil)
		tr.SetSourceModel(b)
		if tr.RowCount() != 1 || tr.ColumnCount() != 3 || tr.Data(tr.Index(0, 2)).ToString() != "e" {
			t.Errorf("transpose: %dx%d", tr.RowCount(), tr.ColumnCount())
		}
		tr.Delete()

		cat := qtcore.NewQConcatenateTablesProxyModel(nil)
		cat.AddSourceModel(a)
		cat.AddSourceModel(b)
		if cat.RowCount() != 5 || len(cat.SourceModels()) != 2 {
			t.Errorf("concatenated rows %d", cat.RowCount())
		}
		src := cat.MapToSource(cat.Index(3, 0))
		if src.Model() != b.AsQAbstractItemModel() || src.Row() != 1 {
			t.Errorf("row 3 maps to row %d of %v", src.Row(), src.Model())
		}
		if cat.MapFromSource(b.Index(0, 0)).Row() != 2 {
			t.Error("MapFromSource")
		}
		cat.Delete()
	})
}

func TestObjectHelpers(t *testing.T) {
	qutetest.Run(t, func() {
		// QSignalMapper: map senders to values
		m := qtcore.NewQSignalMapper(nil)
		s1, s2 := qtcore.NewQObject(m), qtcore.NewQObject(m)
		m.SetMappingText(s1, "one")
		m.SetMapping(s2, 2)
		var gotText string
		var gotInt int
		m.OnMappedString(func(s string) { gotText = s })
		m.OnMappedInt(func(i int) { gotInt = i })
		m.MapWithObject(s1)
		m.MapWithObject(s2)
		if gotText != "one" || gotInt != 2 || m.MappingText("one") != s1 || m.Mapping(2) != s2 {
			t.Errorf("mapped %q and %d", gotText, gotInt)
		}
		m.RemoveMappings(s1)
		if m.MappingText("one") != nil {
			t.Error("mapping not removed")
		}
		m.Delete()

		// QObjectCleanupHandler deletes what it holds
		h := qtcore.NewQObjectCleanupHandler()
		o := qtcore.NewQObject(nil)
		if h.Add(o) != o || h.IsEmpty() {
			t.Error("Add")
		}
		h.Clear()
		if o.Alive() || !h.IsEmpty() {
			t.Error("Clear did not delete the object")
		}
		h.Delete()

		// QEventLoopLocker: the loop quits when its last locker goes
		loop := qtcore.NewQEventLoop(nil)
		locker := qtcore.NewQEventLoopLockerWithEventLoop(loop)
		release := qtcore.NewQTimer(nil)
		release.SetSingleShot(true)
		release.SetInterval(10)
		release.OnTimeout(func() { locker.Delete() })
		release.Start()
		guard := qtcore.NewQTimer(nil)
		guard.SetSingleShot(true)
		guard.SetInterval(3000)
		timedOut := false
		guard.OnTimeout(func() { timedOut = true; loop.Exit() })
		guard.Start()
		loop.Exec()
		if timedOut || locker.Alive() {
			t.Error("the loop did not quit when its locker was deleted")
		}
		release.Delete()
		guard.Delete()
		loop.Delete()

		// a dynamic property change arrives as a QEvent
		obj := qtcore.NewQObject(nil)
		var seen *qtcore.QDynamicPropertyChangeEvent
		name := ""
		f := &filter{on: func(e *qtcore.QEvent) {
			if d := qtcore.QDynamicPropertyChangeEventCast(e); d != nil {
				seen, name = d, string(d.PropertyName())
			}
			if qtcore.QTimerEventCast(e) != nil {
				t.Error("a property change event cast to a timer event")
			}
		}}
		fo := qtcore.NewQObjectWith(f, nil)
		obj.InstallEventFilter(fo)
		obj.SetProperty("dyn", qtcore.NewQVariantOf(1))
		if name != "dyn" {
			t.Errorf("dynamic property event for %q", name)
		}
		if seen == nil || seen.Alive() {
			t.Error("the cast event outlived its handler")
		}
		obj.Delete()
		fo.Delete()
	})
}

func TestSocketNotifier(t *testing.T) {
	qutetest.Run(t, func() {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		defer w.Close()
		fd := int64(r.Fd())
		n := qtcore.NewQSocketNotifierWithSocket(fd, qtcore.QSocketNotifierRead, nil)
		if n.Socket() != fd || n.Type() != qtcore.QSocketNotifierRead || !n.IsValid() || !n.IsEnabled() {
			t.Fatal("notifier state")
		}
		var types []qtcore.QSocketNotifierType
		valid := false
		n.OnActivated(func(sd *qtcore.QSocketDescriptor, typ qtcore.QSocketNotifierType) {
			types = append(types, typ)
			valid = sd.IsValid()
			n.SetEnabled(false) // the data stays unread
		})
		w.Write([]byte("x"))
		if !waitFor(func() bool { return len(types) > 0 }) {
			t.Fatal("not activated")
		}
		if !slices.Equal(types, []qtcore.QSocketNotifierType{qtcore.QSocketNotifierRead}) || !valid {
			t.Errorf("activations %v, descriptor valid %v", types, valid)
		}
		n.Delete()
		if qtcore.NewQSocketDescriptor().IsValid() || !qtcore.NewQSocketDescriptorFull(int(fd)).IsValid() {
			t.Error("socket descriptors")
		}
	})
}
