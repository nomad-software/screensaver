package ui

import (
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtwidgets"
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
		"<p>The Go bindings for Qt are provided by <a href=\"https://github.com/nomad-software/qute\">qute</a>, " +
		"used under the terms of the <a href=\"https://www.gnu.org/licenses/gpl-3.0.html\">GNU General " +
		"Public License version 3</a> (GPLv3).</p>"
}

// newAboutDialog returns the about box: the program's icon beside what it
// is, and the notices of the licences beneath in a frame of the style's own,
// in the colours of the theme.
func newAboutDialog(parent *qtwidgets.QWidget) *qtwidgets.QDialog {
	d := qtwidgets.NewQDialog(parent)
	d.SetWindowTitle("About Screensaver Manager")
	d.SetMinimumWidth(aboutWidth)

	ic := qtwidgets.NewQLabel(nil)
	icon := appIcon()
	defer icon.Delete()
	pixmap := icon.PixmapExtent(aboutIconSize)
	defer pixmap.Delete()
	// The label keeps a copy of the picture.
	ic.SetPixmap(pixmap)
	ic.SetAlignment(qtcore.AlignTop)

	about := newRichLabel(aboutText())

	top := qtwidgets.NewQHBoxLayout(nil)
	top.SetSpacing(20)
	top.AddWidget(ic)
	top.AddWidgetFull(about, 1, 0)

	// The notices are smaller than the rest, as on a label of the style.
	licences := newRichLabel(licenceText())
	font := licences.Font()
	defer font.Delete()
	font.SetPointSizeF(font.PointSizeF() * 0.9)
	licences.SetFont(font)

	notices := qtwidgets.NewQFrame(nil)
	notices.SetFrameShape(qtwidgets.QFrameStyledPanel)
	notices.SetFrameShadow(qtwidgets.QFrameSunken)
	frameLayout := qtwidgets.NewQVBoxLayout(notices)
	frameLayout.SetContentsMargins(margins(12, 8, 12, 8))
	frameLayout.AddWidget(licences)

	buttons := qtwidgets.NewQDialogButtonBoxWithStandardButton(qtwidgets.QDialogButtonBoxOk, nil)
	buttons.OnAccepted(d.Accept)

	layout := qtwidgets.NewQVBoxLayout(d)
	layout.SetContentsMargins(margins(20, 20, 20, 16))
	layout.SetSpacing(16)
	layout.AddLayout(top)
	layout.AddWidget(notices)
	layout.AddWidget(buttons)

	fitDialog(d, aboutWidth)
	return d
}

// newRichLabel returns a label of rich text which wraps, and whose links
// open in the browser.
func newRichLabel(text string) *qtwidgets.QLabel {
	label := qtwidgets.NewQLabelWithText(text, nil)
	label.SetTextFormat(qtcore.RichText)
	label.SetWordWrap(true)
	label.SetOpenExternalLinks(true)
	label.SetTextInteractionFlags(qtcore.TextBrowserInteraction)
	return label
}

// margins returns the margins at the left, top, right and bottom.
func margins(left, top, right, bottom int) qtcore.QMargins {
	return qtcore.QMargins{Left: left, Top: top, Right: right, Bottom: bottom}
}

// fitDialog makes the dialog at least the width, and as high as its layout
// needs at that width, as a label that wraps is not given the height it
// needs by the dialog's size hint alone.
func fitDialog(d *qtwidgets.QDialog, width int) {
	layout := d.Layout()
	layout.Activate()
	least, hint := layout.MinimumSize(), d.SizeHint()
	width = max(width, least.Width)
	height := max(layout.AsQLayoutItem().HeightForWidth(width), hint.Height, least.Height)
	d.Resize(width, height)
}

// showAbout shows the about box, without waiting for it, and deletes it once
// it is closed.
func showAbout(parent *qtwidgets.QWidget) *qtwidgets.QDialog {
	d := newAboutDialog(parent)
	// A dialog is not destroyed by being closed.
	d.OnFinished(func(int) { d.DeleteLater() })
	d.Open()
	return d
}

// qtVersion returns the version of Qt the program runs with.
func qtVersion() string {
	v := qtcore.QLibraryInfoVersion()
	defer v.Delete()
	return v.ToString()
}
