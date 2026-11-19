package qtwidgets_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// answerModal runs answer on the modal dialog once a static function such as
// QFileDialog::getOpenFileName shows it and runs its own event loop. If the
// dialog is still open a while after it was answered, it is rejected and t
// fails, so a test whose answer did not close it fails instead of hanging.
// Delete the returned timer when the function has returned.
func answerModal(t *testing.T, answer func(w *qtwidgets.QWidget)) *qtcore.QTimer {
	timer := qtcore.NewQTimer(nil)
	timer.SetInterval(10)
	var answered *qtwidgets.QWidget
	waited := 0
	timer.OnTimeout(func() {
		w := qtwidgets.QApplicationActiveModalWidget()
		switch {
		case w == nil:
		case answered == nil:
			answered = w
			answer(w)
		case w == answered:
			if waited++; waited > 300 {
				t.Errorf("the %s was still open 3s after it was answered", w.MetaObject().ClassName())
				qtwidgets.QDialogCast(w).Reject()
			}
		}
	})
	timer.Start()
	return timer
}

// fileNameEdit is the line edit of a (non-native) file dialog, where several
// quoted names select several files.
func fileNameEdit(d *qtwidgets.QFileDialog) *qtwidgets.QLineEdit {
	var find func(o *qtcore.QObject) *qtwidgets.QLineEdit
	find = func(o *qtcore.QObject) *qtwidgets.QLineEdit {
		for _, c := range o.Children() {
			if e := qtwidgets.QLineEditCast(c); e != nil && c.ObjectName() == "fileNameEdit" {
				return e
			}
			if e := find(c); e != nil {
				return e
			}
		}
		return nil
	}
	return find(d.AsQObject())
}

// The static file dialog functions are bound with every argument (their Full
// forms): the selected filter goes in, to choose the dialog's filter, and
// comes back out as the one the user chose.
func TestFileDialogStatics(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(n), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const filters = "Text (*.txt);;All (*)"
	qutetest.Run(t, func() {
		var shownFilter string
		timer := answerModal(t, func(w *qtwidgets.QWidget) {
			d := qtwidgets.QFileDialogCast(w)
			shownFilter = d.SelectedNameFilter()
			d.SelectNameFilter("Text (*.txt)")
			d.SelectFile(filepath.Join(dir, "a.txt"))
			d.Accept()
		})
		file, filter := qtwidgets.QFileDialogGetOpenFileNameFull(nil, "Open", dir, filters, "All (*)", qtwidgets.QFileDialogDontUseNativeDialog)
		timer.Delete()
		if shownFilter != "All (*)" {
			t.Errorf("dialog opened with filter %q, want the one passed in", shownFilter)
		}
		if file != filepath.Join(dir, "a.txt") || filter != "Text (*.txt)" {
			t.Errorf("GetOpenFileNameFull = %q, %q", file, filter)
		}

		timer = answerModal(t, func(w *qtwidgets.QWidget) {
			d := qtwidgets.QFileDialogCast(w)
			if e := fileNameEdit(d); e != nil {
				e.SetText(`"a.txt" "b.txt"`)
			} else {
				t.Error("no file name field in the file dialog")
			}
			d.Accept()
		})
		files, _ := qtwidgets.QFileDialogGetOpenFileNamesFull(nil, "Open", dir, filters, "", qtwidgets.QFileDialogDontUseNativeDialog)
		timer.Delete()
		slices.Sort(files)
		if want := []string{filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")}; !slices.Equal(files, want) {
			t.Errorf("GetOpenFileNamesFull = %q, want %q", files, want)
		}

		timer = answerModal(t, func(w *qtwidgets.QWidget) {
			d := qtwidgets.QFileDialogCast(w)
			if e := fileNameEdit(d); e != nil {
				e.SetText("new.txt")
			}
			d.Accept()
		})
		saved, _ := qtwidgets.QFileDialogGetSaveFileNameFull(nil, "Save", dir, filters, "", qtwidgets.QFileDialogDontUseNativeDialog)
		timer.Delete()
		if saved != filepath.Join(dir, "new.txt") {
			t.Errorf("GetSaveFileNameFull = %q", saved)
		}

		timer = answerModal(t, func(w *qtwidgets.QWidget) { qtwidgets.QFileDialogCast(w).Reject() })
		file, _ = qtwidgets.QFileDialogGetOpenFileNameFull(nil, "Open", dir, filters, "", qtwidgets.QFileDialogDontUseNativeDialog)
		timer.Delete()
		if file != "" {
			t.Errorf("cancelled GetOpenFileNameFull = %q, want nothing", file)
		}
	})
}

// QInputDialog::getText is bound with every argument, so a password can be
// asked for (an echo mode), and its ok out-parameter says whether the user
// accepted.
func TestInputDialogGetTextFull(t *testing.T) {
	qutetest.Run(t, func() {
		var echo qtwidgets.QLineEditEchoMode
		timer := answerModal(t, func(w *qtwidgets.QWidget) {
			d := qtwidgets.QInputDialogCast(w)
			echo = d.TextEchoMode()
			d.SetTextValue("secret")
			d.Accept()
		})
		text, ok := qtwidgets.QInputDialogGetTextFull(nil, "Password", "Password:", qtwidgets.QLineEditPassword, "", 0, 0)
		timer.Delete()
		if echo != qtwidgets.QLineEditPassword {
			t.Errorf("echo mode %v, want Password", echo)
		}
		if text != "secret" || !ok {
			t.Errorf("GetTextFull = %q, %v, want the text and true", text, ok)
		}

		timer = answerModal(t, func(w *qtwidgets.QWidget) { qtwidgets.QInputDialogCast(w).Reject() })
		_, ok = qtwidgets.QInputDialogGetTextFull(nil, "Password", "Password:", qtwidgets.QLineEditPassword, "", 0, 0)
		timer.Delete()
		if ok {
			t.Error("cancelled GetTextFull reported ok")
		}

		timer = answerModal(t, func(w *qtwidgets.QWidget) {
			d := qtwidgets.QInputDialogCast(w)
			d.SetIntValue(7)
			d.Accept()
		})
		n, ok := qtwidgets.QInputDialogGetIntFull(nil, "Count", "Count:", 1, 0, 10, 1, 0)
		timer.Delete()
		if n != 7 || !ok {
			t.Errorf("GetIntFull = %d, %v, want 7 and true", n, ok)
		}
	})
}
