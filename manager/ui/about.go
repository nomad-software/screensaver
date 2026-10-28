package ui

import (
	"runtime"

	qt "github.com/mappu/miqt/qt6"
)

// aboutWidth is the narrowest the about box is, so that the notices of the
// licences are set out in lines of a comfortable length.
const aboutWidth = 640

// aboutIconSize is the size of the program's icon in the about box.
const aboutIconSize = 96

// aboutText returns what the about box says of the program.
func aboutText() string {
	return "<h2>Screensaver Manager</h2>" +
		"<p>Chooses and previews the saver the screensaver launcher shows, runs the launcher " +
		"as a service that starts when you log in, and lists the programs which, while they " +
		"run, keep the saver from being shown.</p>" +
		"<p>Copyright © 2026 Gary Willoughby. Released under the " +
		"<a href=\"https://opensource.org/license/mit\">MIT licence</a>.</p>"
}

// licenceText returns the notices of the licences of what the program is
// built with, which carry the notice the LGPL asks of programs that use Qt.
func licenceText() string {
	return "<p><b>Third party licences</b></p>" +
		"<p>This program uses Qt version " + qtVersion() + " (Qt Widgets).</p>" +
		"<p>Qt is a C++ toolkit for cross-platform application development, used here under the " +
		"terms of the <a href=\"https://www.gnu.org/licenses/lgpl-3.0.html\">GNU Lesser General " +
		"Public License version 3</a> (LGPLv3). Qt is dynamically linked, so it can be replaced " +
		"with a different or modified build of the library.</p>" +
		"<p>Qt and its source code are available from <a href=\"https://www.qt.io\">www.qt.io</a>. " +
		"See <a href=\"https://www.qt.io/licensing\">www.qt.io/licensing</a> for an overview of Qt " +
		"licensing.</p>" +
		"<p>Copyright (C) The Qt Company Ltd. and other contributors. Qt and the Qt logo are " +
		"trademarks of The Qt Company Ltd.</p>" +
		"<p>The Go bindings for Qt are provided by <a href=\"https://github.com/mappu/miqt\">miqt</a>, " +
		"used under the MIT licence.</p>"
}

// newAboutDialog returns the about box: the program's icon beside what it
// is, and the notices of the licences beneath in a frame of the style's own,
// in the colours of the theme.
func newAboutDialog(parent *qt.QWidget) *qt.QDialog {
	d := qt.NewQDialog(parent)
	d.SetWindowTitle("About Screensaver Manager")
	d.SetMinimumWidth(aboutWidth)

	ic := qt.NewQLabel2()
	icon := appIcon()
	defer icon.Delete()
	pixmap := icon.PixmapWithExtent(aboutIconSize)
	ic.SetPixmap(pixmap)
	runtime.KeepAlive(pixmap)
	ic.SetAlignment(qt.AlignTop)

	about := newRichLabel(aboutText())

	top := qt.NewQHBoxLayout2()
	top.SetSpacing(20)
	top.AddWidget(ic.QWidget)
	top.AddWidget2(about.QWidget, 1)

	// The notices are smaller than the rest, as on a label of the style.
	licences := newRichLabel(licenceText())
	font := licences.Font()
	font.SetPointSizeF(font.PointSizeF() * 0.9)
	licences.SetFont(font)

	notices := qt.NewQFrame2()
	notices.SetFrameShape(qt.QFrame__StyledPanel)
	notices.SetFrameShadow(qt.QFrame__Sunken)
	frameLayout := qt.NewQVBoxLayout(notices.QWidget)
	frameLayout.SetContentsMargins(12, 8, 12, 8)
	frameLayout.AddWidget(licences.QWidget)

	buttons := qt.NewQDialogButtonBox4(qt.QDialogButtonBox__Ok)
	buttons.OnAccepted(d.Accept)

	layout := qt.NewQVBoxLayout(d.QWidget)
	layout.SetContentsMargins(20, 20, 20, 16)
	layout.SetSpacing(16)
	layout.AddLayout(top.QLayout)
	layout.AddWidget(notices.QWidget)
	layout.AddWidget(buttons.QWidget)

	fitDialog(d, aboutWidth)
	return d
}

// newRichLabel returns a label of rich text which wraps, and whose links
// open in the browser.
func newRichLabel(text string) *qt.QLabel {
	label := qt.NewQLabel3(text)
	label.SetTextFormat(qt.RichText)
	label.SetWordWrap(true)
	label.SetOpenExternalLinks(true)
	label.SetTextInteractionFlags(qt.TextBrowserInteraction)
	return label
}

// fitDialog makes the dialog at least the width, and as high as its layout
// needs at that width, as a label that wraps is not given the height it
// needs by the dialog's size hint alone.
func fitDialog(d *qt.QDialog, width int) {
	layout := d.Layout()
	layout.Activate()
	least, hint := layout.MinimumSize(), d.SizeHint()
	width = max(width, least.Width())
	height := max(layout.HeightForWidth(width), hint.Height(), least.Height())
	runtime.KeepAlive(least)
	runtime.KeepAlive(hint)
	d.Resize(width, height)
}

// showAbout shows the about box, without waiting for it, and deletes it once
// it is closed.
func showAbout(parent *qt.QWidget) *qt.QDialog {
	d := newAboutDialog(parent)
	// A dialog is not destroyed by being closed.
	d.OnFinished(func(int) { d.DeleteLater() })
	d.Open()
	return d
}

// qtVersion returns the version of Qt the program runs with.
func qtVersion() string {
	v := qt.QLibraryInfo_Version()
	defer runtime.KeepAlive(v)
	return v.ToString()
}
