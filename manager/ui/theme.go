package ui

import (
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"github.com/nomad-software/screensaver/manager/settings"
)

// themes are the themes that can be chosen, by the name the settings keep
// them by, in the order they are offered: the system's, which is "", and
// light and dark ones.
var themes = []struct{ name, label string }{
	{"", "Follow the system"},
	{settings.ThemeLight, "Light"},
	{settings.ThemeDark, "Dark"},
}

// systemStyle is the name of the style the desktop gave the program, which
// following the system goes back to.
var systemStyle string

// rememberSystemStyle keeps the name of the style the program started with,
// before a theme of its own replaces it.
func rememberSystemStyle() {
	if systemStyle == "" {
		systemStyle = qtwidgets.QApplicationStyle().Name()
	}
}

// applyTheme puts the theme with the name into effect. The light and dark
// themes are the Fusion style, which draws in whatever palette it is given
// on every desktop, with a palette of their own; the system's is the style
// and the palette the desktop gives, which Qt keeps up to date as the
// desktop's theme changes.
func applyTheme(name string) {
	rememberSystemStyle()
	var p *qtgui.QPalette
	switch name {
	case settings.ThemeLight:
		qtwidgets.QApplicationSetStyleName("Fusion")
		p = makePalette(lightColours)
	case settings.ThemeDark:
		qtwidgets.QApplicationSetStyleName("Fusion")
		p = makePalette(darkColours)
	default:
		qtwidgets.QApplicationSetStyleName(systemStyle)
		// A palette that sets nothing is the desktop's.
		p = qtgui.NewQPalette()
	}
	qtwidgets.QApplicationSetPalette(p)
	p.Delete()
}

// colours are the colours of a palette: those of each role, and those of
// some roles where what they are in is disabled.
type colours struct {
	roles, disabled map[qtgui.QPaletteColorRole]string
}

// lightColours and darkColours are the palettes of the light and dark
// themes, as ApeMail has them. Each sets every colour Fusion draws with, as
// what is left out would be the desktop's.
var (
	lightColours = colours{
		roles: map[qtgui.QPaletteColorRole]string{
			qtgui.QPaletteWindow:          "#efefef",
			qtgui.QPaletteWindowText:      "#1a1a1a",
			qtgui.QPaletteBase:            "#ffffff",
			qtgui.QPaletteAlternateBase:   "#f5f5f5",
			qtgui.QPaletteText:            "#1a1a1a",
			qtgui.QPalettePlaceholderText: "#808080",
			qtgui.QPaletteButton:          "#efefef",
			qtgui.QPaletteButtonText:      "#1a1a1a",
			qtgui.QPaletteBrightText:      "#ffffff",
			qtgui.QPaletteToolTipBase:     "#ffffdc",
			qtgui.QPaletteToolTipText:     "#1a1a1a",
			qtgui.QPaletteHighlight:       "#308cc6",
			qtgui.QPaletteHighlightedText: "#ffffff",
			qtgui.QPaletteLink:            "#0b57d0",
			qtgui.QPaletteLinkVisited:     "#6a1b9a",
			qtgui.QPaletteLight:           "#ffffff",
			qtgui.QPaletteMidlight:        "#e3e3e3",
			qtgui.QPaletteMid:             "#b8b8b8",
			qtgui.QPaletteDark:            "#9f9f9f",
			qtgui.QPaletteShadow:          "#767676",
		},
		disabled: map[qtgui.QPaletteColorRole]string{
			qtgui.QPaletteWindowText:      "#a0a0a0",
			qtgui.QPaletteText:            "#a0a0a0",
			qtgui.QPaletteButtonText:      "#a0a0a0",
			qtgui.QPaletteHighlight:       "#919191",
			qtgui.QPaletteHighlightedText: "#ffffff",
		},
	}
	darkColours = colours{
		roles: map[qtgui.QPaletteColorRole]string{
			qtgui.QPaletteWindow:          "#2d2d2d",
			qtgui.QPaletteWindowText:      "#e6e6e6",
			qtgui.QPaletteBase:            "#1e1e1e",
			qtgui.QPaletteAlternateBase:   "#262626",
			qtgui.QPaletteText:            "#e6e6e6",
			qtgui.QPalettePlaceholderText: "#8c8c8c",
			qtgui.QPaletteButton:          "#353535",
			qtgui.QPaletteButtonText:      "#e6e6e6",
			qtgui.QPaletteBrightText:      "#ff6060",
			qtgui.QPaletteToolTipBase:     "#353535",
			qtgui.QPaletteToolTipText:     "#e6e6e6",
			qtgui.QPaletteHighlight:       "#2f6fb5",
			qtgui.QPaletteHighlightedText: "#ffffff",
			qtgui.QPaletteLink:            "#6aa9ff",
			qtgui.QPaletteLinkVisited:     "#b48cff",
			qtgui.QPaletteLight:           "#4a4a4a",
			qtgui.QPaletteMidlight:        "#3e3e3e",
			qtgui.QPaletteMid:             "#262626",
			qtgui.QPaletteDark:            "#1a1a1a",
			qtgui.QPaletteShadow:          "#101010",
		},
		disabled: map[qtgui.QPaletteColorRole]string{
			qtgui.QPaletteWindowText:      "#7a7a7a",
			qtgui.QPaletteText:            "#7a7a7a",
			qtgui.QPaletteButtonText:      "#7a7a7a",
			qtgui.QPaletteHighlight:       "#4a4a4a",
			qtgui.QPaletteHighlightedText: "#b0b0b0",
		},
	}
)

// makePalette returns a palette of the colours.
func makePalette(cs colours) *qtgui.QPalette {
	p := qtgui.NewQPalette()
	for role, colour := range cs.roles {
		p.SetColor(role, mustColour(colour))
	}
	for role, colour := range cs.disabled {
		p.SetGroupColor(qtgui.QPaletteDisabled, role, mustColour(colour))
	}
	return p
}

// mustColour returns the colour written as #rrggbb, which the colours here
// all are.
func mustColour(s string) qtgui.QColor {
	c, err := qtgui.ParseColor(s)
	if err != nil {
		panic(err)
	}
	return c
}
