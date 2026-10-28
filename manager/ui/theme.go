package ui

import (
	qt "github.com/mappu/miqt/qt6"

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
		systemStyle = qt.QApplication_Style().Name()
	}
}

// applyTheme puts the theme with the name into effect. The light and dark
// themes are the Fusion style, which draws in whatever palette it is given
// on every desktop, with a palette of their own; the system's is the style
// and the palette the desktop gives, which Qt keeps up to date as the
// desktop's theme changes.
func applyTheme(name string) {
	rememberSystemStyle()
	var p *qt.QPalette
	switch name {
	case settings.ThemeLight:
		qt.QApplication_SetStyleWithStyle("Fusion")
		p = makePalette(lightColours)
	case settings.ThemeDark:
		qt.QApplication_SetStyleWithStyle("Fusion")
		p = makePalette(darkColours)
	default:
		qt.QApplication_SetStyleWithStyle(systemStyle)
		// A palette that sets nothing is the desktop's.
		p = qt.NewQPalette()
	}
	qt.QApplication_SetPalette(p)
	p.Delete()
}

// colours are the colours of a palette: those of each role, and those of
// some roles where what they are in is disabled.
type colours struct {
	roles, disabled map[qt.QPalette__ColorRole]string
}

// lightColours and darkColours are the palettes of the light and dark
// themes, as ApeMail has them. Each sets every colour Fusion draws with, as
// what is left out would be the desktop's.
var (
	lightColours = colours{
		roles: map[qt.QPalette__ColorRole]string{
			qt.QPalette__Window:          "#efefef",
			qt.QPalette__WindowText:      "#1a1a1a",
			qt.QPalette__Base:            "#ffffff",
			qt.QPalette__AlternateBase:   "#f5f5f5",
			qt.QPalette__Text:            "#1a1a1a",
			qt.QPalette__PlaceholderText: "#808080",
			qt.QPalette__Button:          "#efefef",
			qt.QPalette__ButtonText:      "#1a1a1a",
			qt.QPalette__BrightText:      "#ffffff",
			qt.QPalette__ToolTipBase:     "#ffffdc",
			qt.QPalette__ToolTipText:     "#1a1a1a",
			qt.QPalette__Highlight:       "#308cc6",
			qt.QPalette__HighlightedText: "#ffffff",
			qt.QPalette__Link:            "#0b57d0",
			qt.QPalette__LinkVisited:     "#6a1b9a",
			qt.QPalette__Light:           "#ffffff",
			qt.QPalette__Midlight:        "#e3e3e3",
			qt.QPalette__Mid:             "#b8b8b8",
			qt.QPalette__Dark:            "#9f9f9f",
			qt.QPalette__Shadow:          "#767676",
		},
		disabled: map[qt.QPalette__ColorRole]string{
			qt.QPalette__WindowText:      "#a0a0a0",
			qt.QPalette__Text:            "#a0a0a0",
			qt.QPalette__ButtonText:      "#a0a0a0",
			qt.QPalette__Highlight:       "#919191",
			qt.QPalette__HighlightedText: "#ffffff",
		},
	}
	darkColours = colours{
		roles: map[qt.QPalette__ColorRole]string{
			qt.QPalette__Window:          "#2d2d2d",
			qt.QPalette__WindowText:      "#e6e6e6",
			qt.QPalette__Base:            "#1e1e1e",
			qt.QPalette__AlternateBase:   "#262626",
			qt.QPalette__Text:            "#e6e6e6",
			qt.QPalette__PlaceholderText: "#8c8c8c",
			qt.QPalette__Button:          "#353535",
			qt.QPalette__ButtonText:      "#e6e6e6",
			qt.QPalette__BrightText:      "#ff6060",
			qt.QPalette__ToolTipBase:     "#353535",
			qt.QPalette__ToolTipText:     "#e6e6e6",
			qt.QPalette__Highlight:       "#2f6fb5",
			qt.QPalette__HighlightedText: "#ffffff",
			qt.QPalette__Link:            "#6aa9ff",
			qt.QPalette__LinkVisited:     "#b48cff",
			qt.QPalette__Light:           "#4a4a4a",
			qt.QPalette__Midlight:        "#3e3e3e",
			qt.QPalette__Mid:             "#262626",
			qt.QPalette__Dark:            "#1a1a1a",
			qt.QPalette__Shadow:          "#101010",
		},
		disabled: map[qt.QPalette__ColorRole]string{
			qt.QPalette__WindowText:      "#7a7a7a",
			qt.QPalette__Text:            "#7a7a7a",
			qt.QPalette__ButtonText:      "#7a7a7a",
			qt.QPalette__Highlight:       "#4a4a4a",
			qt.QPalette__HighlightedText: "#b0b0b0",
		},
	}
)

// makePalette returns a palette of the colours.
func makePalette(cs colours) *qt.QPalette {
	p := qt.NewQPalette()
	// The palette keeps a copy of each colour.
	for role, colour := range cs.roles {
		c := qt.NewQColor6(colour)
		p.SetColor2(role, c)
		c.Delete()
	}
	for role, colour := range cs.disabled {
		c := qt.NewQColor6(colour)
		p.SetColor(qt.QPalette__Disabled, role, c)
		c.Delete()
	}
	return p
}
