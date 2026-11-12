package ui

import (
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

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
	*qtwidgets.QDialog
	tabs  *qtwidgets.QTabWidget
	theme *qtwidgets.QComboBox
	box   *qtwidgets.QDialogButtonBox
}

// newSettingsDialog shows the settings, and calls accepted with them if they
// are.
func newSettingsDialog(parent *qtwidgets.QWidget, current settings.Settings, accepted func(settings.Settings)) *settingsDialog {
	d := &settingsDialog{QDialog: qtwidgets.NewQDialog(parent)}
	d.SetWindowTitle("Settings")
	d.tabs = qtwidgets.NewQTabWidget(nil)

	ui := qtwidgets.NewQWidget(nil)
	uiLayout := qtwidgets.NewQVBoxLayout(ui)
	uiLayout.SetContentsMargins(margins(paneMargin, paneMargin, paneMargin, paneMargin))
	uiLayout.AddWidget(sectionHeading("Appearance"))
	form := qtwidgets.NewQFormLayout(nil)
	form.SetFieldGrowthPolicy(qtwidgets.QFormLayoutFieldsStayAtSizeHint)
	form.SetHorizontalSpacing(formSpacing)
	d.theme = qtwidgets.NewQComboBox(nil)
	for _, t := range themes {
		addItem(d.theme, t.label, t.name)
	}
	selectData(d.theme, current.Theme)
	form.AddRowText("Theme", d.theme)
	uiLayout.AddLayout(form)
	uiLayout.AddStretch()
	d.tabs.AddTab(plainPage(ui), "User interface")

	d.box = qtwidgets.NewQDialogButtonBoxWithButtonsOriented(
		qtwidgets.QDialogButtonBoxOk|qtwidgets.QDialogButtonBoxCancel, qtcore.Horizontal, nil)
	d.box.OnRejected(d.Reject)
	d.box.OnAccepted(func() {
		s := current
		s.Theme = currentData(d.theme)
		d.Accept()
		accepted(s)
	})

	layout := qtwidgets.NewQVBoxLayout(d)
	layout.AddWidgetFull(d.tabs, 1, 0)
	layout.AddWidget(d.box)
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
func plainPage(page *qtwidgets.QWidget) *qtwidgets.QWidget {
	page.SetBackgroundRole(qtgui.QPaletteWindow)
	page.SetAutoFillBackground(true)
	return page
}

// sectionHeading returns the heading of a section of the dialog, in bold.
func sectionHeading(title string) *qtwidgets.QLabel {
	label := qtwidgets.NewQLabelWithText(title, nil)
	// The font is a copy of the label's.
	font := label.Font()
	defer font.Delete()
	font.SetBold(true)
	label.SetFont(font)
	return label
}

// addItem adds an item to the box, with the data it stands for.
func addItem(box *qtwidgets.QComboBox, label, data string) {
	v := qtcore.NewQVariantWithString(data)
	defer v.Delete()
	box.AddItemFull(label, v)
}

// selectData makes the item of the box with the data the current one.
func selectData(box *qtwidgets.QComboBox, data string) {
	for i := range box.Count() {
		v := box.ItemData(i)
		if v.ToString() == data {
			box.SetCurrentIndex(i)
		}
		v.Delete()
	}
}

// currentData returns the data of the current item of the box.
func currentData(box *qtwidgets.QComboBox) string {
	v := box.CurrentData()
	defer v.Delete()
	return v.ToString()
}
