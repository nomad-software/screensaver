package qute_test

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/nomad-software/qute"
	rt "github.com/nomad-software/qute/internal/rt"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// t.Fatal inside qutetest.Run used to kill the test binary with "panic called
// with nil argument". The child tests below run in a separate process: the
// first calls t.Fatal inside Run, the next ones must still run.
func TestFatalInsideRun(t *testing.T) {
	if os.Getenv("QUTE_FATAL_CHILD") != "" {
		t.Skip("parent only")
	}
	cmd := exec.Command(os.Args[0], "-test.run", "^TestFatalChild", "-test.v")
	cmd.Env = append(os.Environ(), "QUTE_FATAL_CHILD=1")
	out, err := cmd.CombinedOutput()
	s := string(out)
	if _, ok := err.(*exec.ExitError); !ok {
		t.Fatalf("child: err %v, want a failing exit; output:\n%s", err, s)
	}
	for _, want := range []string{
		"--- FAIL: TestFatalChild1", "fatal inside Run",
		"--- SKIP: TestFatalChild2", "skipped inside Run",
		"--- PASS: TestFatalChild3",
		"--- FAIL: TestFatalChild4", "second fatal",
		"--- PASS: TestFatalChild5",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("child output lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "panic:") || strings.Contains(s, "after Fatal") {
		t.Errorf("child panicked or ran code after t.Fatal:\n%s", s)
	}
}

func childOnly(t *testing.T) {
	if os.Getenv("QUTE_FATAL_CHILD") == "" {
		t.Skip("run by TestFatalInsideRun")
	}
}

func TestFatalChild1(t *testing.T) {
	childOnly(t)
	qutetest.Run(t, func() {
		t.Fatal("fatal inside Run")
		t.Log("after Fatal")
	})
	t.Log("after Fatal")
}

func TestFatalChild2(t *testing.T) {
	childOnly(t)
	qutetest.Run(t, func() { t.Skip("skipped inside Run") })
}

func TestFatalChild3(t *testing.T) {
	childOnly(t)
	qutetest.Run(t, func() {
		w := qtwidgets.NewQWidget(nil)
		w.Delete()
	})
}

func TestFatalChild4(t *testing.T) {
	childOnly(t)
	qutetest.Run(t, func() { t.Fatalf("second %s", "fatal") })
}

func TestFatalChild5(t *testing.T) {
	childOnly(t)
	qutetest.CollectGarbage()
	qutetest.Run(t, func() {})
}

// A handler panic under PostSync is recovered where Qt called the handler,
// so Qt's frames return normally, and re-raised in the caller with the
// handler's own stack.
func TestPostSyncHandlerPanic(t *testing.T) {
	var after bool
	var b *qtwidgets.QPushButton
	qutetest.Run(t, func() {
		b = qtwidgets.NewQPushButtonWithText("x", nil)
		b.OnClicked(func(bool) { handlerThatPanics() })
	})
	defer qutetest.Run(t, func() { b.Delete() })
	p := func() (p any) {
		defer func() { p = recover() }()
		qute.PostSync(func() {
			b.Click()
			after = true // Qt returned normally from the click
		})
		return nil
	}()
	cp, ok := p.(*rt.CallbackPanic)
	if !ok {
		t.Fatalf("PostSync raised %T %v, want the handler's *CallbackPanic", p, p)
	}
	if cp.Value != "handler boom" || !strings.Contains(cp.Stack, "handlerThatPanics") {
		t.Errorf("panic %v with stack:\n%s\nwant the handler's value and stack", cp.Value, cp.Stack)
	}
	if !after {
		t.Error("the code after the click did not run")
	}
}

func handlerThatPanics() { panic("handler boom") }

// Qt calls from another goroutine panic in release builds too: generated
// methods and the hand-written helpers alike.
func TestOffThreadCallPanics(t *testing.T) {
	var w *qtwidgets.QWidget
	qutetest.Run(t, func() { w = qtwidgets.NewQWidget(nil) })
	defer qutetest.Run(t, func() { w.Delete() })
	for name, f := range map[string]func(){
		"ObjectName":     func() { _ = w.ObjectName() },
		"qutetest.Click": func() { qutetest.Click(w) },
		"qute.Tree":      func() { qute.Tree(w) },
	} {
		func() {
			defer func() {
				p := recover()
				if msg, _ := p.(string); !strings.Contains(msg, "off the GUI thread") {
					t.Errorf("%s off the GUI thread: panic %v, want an off-thread message", name, p)
				}
			}()
			f() // the test goroutine is not the GUI thread
		}()
	}
}

// Arguments with C++ defaults can be passed through each method's Full form.
func TestFullForms(t *testing.T) {
	qutetest.Run(t, func() {
		// a tree model: below the top level needs the parent argument
		m := qtgui.NewQStandardItemModel(nil)
		defer m.Delete()
		top := qtgui.NewQStandardItemWithText("top")
		top.AppendRow(qtgui.NewQStandardItemWithText("child"))
		m.AppendRow(top)
		root := m.Index(0, 0)
		if n := m.RowCountFull(root); n != 1 {
			t.Errorf("RowCountFull(top) = %d, want 1", n)
		}
		if !m.HasChildrenFull(root) {
			t.Error("HasChildrenFull(top) = false")
		}
		child := m.IndexFull(0, 0, root)
		if got := m.Data(child).ToString(); got != "child" {
			t.Errorf("child = %q, want child", got)
		}
		// roles
		m.SetDataFull(child, qtcore.NewQVariantOf("tip"), int(qtcore.ToolTipRole))
		if got := m.DataFull(child, int(qtcore.ToolTipRole)).ToString(); got != "tip" {
			t.Errorf("tooltip role = %q, want tip", got)
		}
		// text selection: KeepAnchor and a count
		doc := qtgui.NewQTextDocument(nil)
		defer doc.Delete()
		doc.SetPlainText("hello world")
		c := qtgui.NewQTextCursorWithTextDocument(doc)
		c.MovePositionFull(qtgui.QTextCursorRight, qtgui.QTextCursorKeepAnchor, 5)
		if got := c.SelectedText(); got != "hello" {
			t.Errorf("selected %q, want hello", got)
		}
		// layouts and window flags
		w := qtwidgets.NewQWidget(nil)
		defer w.Delete()
		lay := qtwidgets.NewQHBoxLayout(w)
		lay.AddWidgetFull(qtwidgets.NewQLabelWithText("a", nil), 3, qtcore.AlignLeft)
		lay.AddStretchFull(2)
		if lay.Stretch(0) != 3 || lay.Stretch(1) != 2 {
			t.Errorf("stretches %d %d, want 3 2", lay.Stretch(0), lay.Stretch(1))
		}
		w.SetWindowFlag(qtcore.WindowStaysOnTopHint)
		w.SetWindowFlagFull(qtcore.WindowStaysOnTopHint, false)
		if w.WindowFlags().Has(qtcore.WindowStaysOnTopHint) {
			t.Error("SetWindowFlagFull(flag, false) did not clear the flag")
		}
		// user data and constructor defaults
		cb := qtwidgets.NewQComboBox(w)
		cb.AddItemFull("one", qtcore.NewQVariantOf(42))
		if got := cb.ItemData(0).ToInt(); got != 42 {
			t.Errorf("item data %d, want 42", got)
		}
		if f := qtgui.NewQFontWithFamilyFull("Sans", 17, -1, true); f.PointSize() != 17 || !f.Italic() {
			t.Errorf("font %d italic %v, want 17 true", f.PointSize(), f.Italic())
		}
	})
}

// Qt calling a Go handler from another thread is a clear panic, not a
// corrupted runtime.
func TestHandlerOffGUIThreadPanics(t *testing.T) {
	var wg sync.WaitGroup
	entries := map[string]func(){
		// off: what the generated C++ reports for a call from another thread
		"signal":   func() { rt.InvokeCallback(0, nil, true) },
		"virtual":  func() { rt.Dispatch(0, 0, nil, nil, true) },
		"function": func() { rt.InvokeFunc(0, nil, nil, true) },
	}
	for name, f := range entries {
		var msg string
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg = panicText(f)
		}()
		wg.Wait()
		if !strings.Contains(msg, "thread other than the GUI thread") {
			t.Errorf("off-thread %s entry: panic %q", name, msg)
		}
	}
}
