package app

import (
	"github.com/egoist/mygo/ui"
)

// card wraps body in the console's standard surface: a white, rounded
// panel with a hairline border and a soft shadow, on the gray page. Every
// page composes from these so the whole console shares one material.
func card(c *ui.Context, body func()) ui.Element {
	return ui.Column(c).Padding(16).Gap(12).Radius(12).
		Background(cardBg).Border(1, cardBorder).
		Shadow(0, 1, 2, 0, shadowCard).Children(body)
}

// statTile is one of the four headline numbers: a soft rounded square
// holding a colored icon, the label, and the big count in its color.
func statTile(c *ui.Context, icon *ui.SVG, label string, n int, color ui.Color) {
	t := c.Theme()
	ui.Row(c).Grow(1).Basis(0).Padding(16).Gap(14).AlignItems(ui.Center).
		Radius(12).Background(cardBg).Border(1, cardBorder).
		Shadow(0, 1, 2, 0, shadowCard).Children(func() {
		// Icon in a tinted rounded square.
		ui.Box(c).Size(44, 44).Radius(12).Shrink(0).
			Background(color.Alpha(0.12)).Center().Children(func() {
			ui.Icon(c, icon).FontSize(22).TextColor(color)
		})
		ui.Column(c).Gap(2).Grow(1).Children(func() {
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).SingleLine()
			ui.Textf(c, "%d", n).FontSize(26).Bold().TextColor(color)
		})
	})
}

// sectionLabel is a small uppercase-ish muted caption that heads a group.
func sectionLabel(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(12).Bold().TextColor(textMuted2).LetterSpacing(0.5)
}

// pageTitle draws the big page title with an optional muted subtitle.
func pageTitle(c *ui.Context, title, sub string) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Shrink(0).Children(func() {
		ui.Text(c, title).FontSize(20).Bold().TextColor(textStrong)
		if sub != "" {
			ui.Badge(c, sub)
		}
	})
}

// iconTextButton is a secondary button with a leading icon and a label,
// the console's standard action style. It returns whether it was clicked.
func iconTextButton(c *ui.Context, icon *ui.SVG, label string) bool {
	b := ui.Button(c, "").Padding(7, 12).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon).FontSize(14).TextColor(textBody)
			ui.Text(c, label).FontSize(12).TextColor(textBody)
		})
	})
	return b.Clicked()
}

// iconButton is a small square button showing only an icon, for row
// actions (打开/编辑/删除). tint is the icon color.
func iconButton(c *ui.Context, icon *ui.SVG, tint ui.Color) bool {
	return iconButtonLabel(c, icon, tint, "")
}

// iconButtonLabel is iconButton with an accessible name (Access), so screen
// readers and the headless tester can find it by that label.
func iconButtonLabel(c *ui.Context, icon *ui.SVG, tint ui.Color, label string) bool {
	b := ui.Button(c, "").Padding(6, 8).Children(func() {
		ui.Icon(c, icon).FontSize(15).TextColor(tint)
	})
	if label != "" {
		b.Label(label)
	}
	return b.Clicked()
}

// pill is a rounded status chip with a colored dot and label.
func pill(c *ui.Context, label string, color ui.Color) {
	ui.Row(c).Padding(3, 9).Radius(999).Gap(6).AlignItems(ui.Center).
		Background(color.Alpha(0.12)).Shrink(0).Children(func() {
		ui.Box(c).Size(7, 7).Radius(999).Background(color).Shrink(0)
		ui.Text(c, label).FontSize(12).TextColor(color).SingleLine()
	})
}