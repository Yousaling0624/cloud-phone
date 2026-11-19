package app

import "github.com/egoist/mygo/ui"

// Accent is the platform's signature teal (#12a594), used for primary
// buttons, the chosen tab underline, sidebar icons and focus rings.
var Accent = ui.Hex("#12a594")

// Palette tokens the whole console draws from, so a tweak here reaches
// every page. The look is a light, airy business dashboard: a soft gray
// page, white cards that float on it with a gentle shadow, hairline
// borders, and the teal accent carrying every call to action. The
// sidebar stays a deep slate panel for contrast, as most admin consoles
// do, with teal-highlighted icons.
var (
	pageBg      = ui.Hex("#f3f5f9")
	cardBg      = ui.Hex("#ffffff")
	cardBorder  = ui.Hex("#e8ecf2")
	hairline    = ui.Hex("#eef1f6")
	textStrong  = ui.Hex("#0f1a24")
	textBody    = ui.Hex("#33414f")
	textMuted2  = ui.Hex("#8a95a5")
	accentSoft  = ui.RGBA(18, 165, 148, 0.10)
	accentSoft2 = ui.RGBA(18, 165, 148, 0.16)

	shadowCard = ui.RGBA(20, 34, 56, 0.06)
	shadowRaise = ui.RGBA(20, 34, 56, 0.12)
)

// theme returns the light theme the operator console uses. It is built
// fresh each call (the view calls it once per frame) so a field change can
// never leak into a window that is already open.
func theme() *ui.Theme {
	t := ui.LightTheme()
	t.Background = pageBg
	t.Surface = cardBg
	t.SurfaceHover = ui.Hex("#f0f3f8")
	t.SurfacePressed = ui.Hex("#e6ebf2")
	t.Border = cardBorder
	t.Text = textStrong
	t.TextMuted = textMuted2
	t.Accent = Accent
	t.AccentHover = ui.Hex("#0f9283")
	t.AccentPressed = ui.Hex("#0c7c6f")
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#e5484d")
	t.Warning = ui.Hex("#e2a03f")
	t.Success = ui.Hex("#12a594")
	t.Selection = ui.RGBA(18, 165, 148, 0.16)
	t.Focus = ui.RGBA(18, 165, 148, 0.50)
	t.FocusRingWidth = 2
	t.FocusRingOffset = 1
	t.Radius = 8
	t.FontSize = 13
	t.Font = uiFontFamily
	return t
}

// --- sidebar palette: a deep slate panel with teal icons ---

var (
	sidebarBg      = ui.Hex("#111c27")
	sidebarPanel   = ui.Hex("#16232f")
	sidebarText    = ui.Hex("#93a3b5")
	sidebarMuted   = ui.Hex("#5a6b7d")
	sidebarActive  = ui.Hex("#20303f")
	sidebarHover   = ui.Hex("#1a2937")
)

// sidebarTheme derives the app theme with a dark slate palette for the
// sidebar's scope (ui.Themed), keeping the teal accent.
func sidebarTheme() *ui.Theme {
	t := theme()
	t.Dark = true
	t.Background = sidebarBg
	t.Surface = sidebarBg
	t.SurfaceHover = sidebarHover
	t.SurfacePressed = sidebarActive
	t.Border = ui.Hex("#26384a")
	t.Text = ui.Hex("#e7eef6")
	t.TextMuted = sidebarText
	t.Accent = ui.Hex("#2fd0bd")
	t.AccentText = ui.Hex("#08221e")
	t.Selection = ui.RGBA(47, 208, 189, 0.28)
	t.Focus = ui.RGBA(47, 208, 189, 0.6)
	return t
}
