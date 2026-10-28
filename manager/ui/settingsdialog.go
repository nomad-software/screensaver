package ui

import (
	"runtime"

	qt "github.com/mappu/miqt/qt6"

	"github.com/nomad-software/screensaver/manager/settings"
)

// The size the settings dialog opens at, at least.
const (
	settingsWidth  = 600
	settingsHeight = 400
)

// settingsDialog shows the manager's settings, in tabs as Prism's are: the
// theme, on User interface.
type settingsDialog struct {
	*qt.QDialog
	tabs  *qt.QTabWidget
	theme *qt.QComboBox
	box   *qt.QDialogButtonBox
}

// newSettingsDialog shows the settings, and calls accepted with them if they
// are.
func newSettingsDialog(parent *qt.QWidget, current settings.Settings, accepted func(settings.Settings)) *settingsDialog {
	d := &settingsDialog{QDialog: qt.NewQDialog(parent)}
	d.SetWindowTitle("Settings")
	d.tabs = qt.NewQTabWidget2()

	ui := qt.NewQWidget2()
	uiLayout := qt.NewQVBoxLayout(ui)
	uiLayout.SetContentsMargins(paneMargin, paneMargin, paneMargin, paneMargin)
	uiLayout.AddWidget(sectionHeading("Appearance").QWidget)
	form := qt.NewQFormLayout2()
	form.SetFieldGrowthPolicy(qt.QFormLayout__FieldsStayAtSizeHint)
	form.SetHorizontalSpacing(formSpacing)
	d.theme = qt.NewQComboBox2()
	for _, t := range themes {
		addItem(d.theme, t.label, t.name)
	}
	selectData(d.theme, current.Theme)
	form.AddRow3("Theme", d.theme.QWidget)
	uiLayout.AddLayout(form.QLayout)
	uiLayout.AddStretch()
	d.tabs.AddTab(plainPage(ui), "User interface")

	d.box = qt.NewQDialogButtonBox5(qt.QDialogButtonBox__Ok|qt.QDialogButtonBox__Cancel, qt.Horizontal)
	d.box.OnRejected(d.Reject)
	d.box.OnAccepted(func() {
		s := current
		s.Theme = currentData(d.theme)
		d.Accept()
		accepted(s)
	})

	layout := qt.NewQVBoxLayout(d.QWidget)
	layout.AddWidget2(d.tabs.QWidget, 1)
	layout.AddWidget(d.box.QWidget)
	fitDialog(d.QDialog, settingsWidth)
	d.Resize(d.Width(), max(d.Height(), settingsHeight))
	// A dialog is not destroyed by being closed.
	d.OnFinished(func(int) { d.DeleteLater() })
	d.Open()
	return d
}

// plainPage gives the page of a tab widget the background of the window, as
// Prism does: some styles draw the pages of a tab widget lighter than the
// window, which in a dark theme is a slab of grey. The page is returned.
func plainPage(page *qt.QWidget) *qt.QWidget {
	page.SetBackgroundRole(qt.QPalette__Window)
	page.SetAutoFillBackground(true)
	return page
}

// sectionHeading returns the heading of a section of the dialog, in bold.
func sectionHeading(title string) *qt.QLabel {
	label := qt.NewQLabel3(title)
	font := qt.NewQFont5(label.Font())
	defer font.Delete()
	font.SetBold(true)
	label.SetFont(font)
	return label
}

// addItem adds an item to the box, with the data it stands for.
func addItem(box *qt.QComboBox, label, data string) {
	v := qt.NewQVariant11(data)
	box.AddItem3(label, v)
	v.Delete()
}

// selectData makes the item of the box with the data the current one.
func selectData(box *qt.QComboBox, data string) {
	for i := range box.Count() {
		v := box.ItemData(i)
		if v.ToString() == data {
			box.SetCurrentIndex(i)
		}
		runtime.KeepAlive(v)
	}
}

// currentData returns the data of the current item of the box.
func currentData(box *qt.QComboBox) string {
	v := box.CurrentData()
	defer runtime.KeepAlive(v)
	return v.ToString()
}
