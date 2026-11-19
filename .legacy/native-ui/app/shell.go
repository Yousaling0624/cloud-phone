package app

import (
	"github.com/egoist/mygo/ui"
)

// View builds the whole console each frame: a deep slate sidebar on the
// left, the top tab bar and the content of the current page on the right,
// and the cloud-phone console dialog when it is open.
func (a *App) View(c *ui.Context) {
	ensureFonts()
	c.SetTheme(theme())
	ui.Row(c).Fill().Children(func() {
		a.sidebar(c)
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			a.topBar(c)
			ui.Box(c).Grow(1).FillWidth().Children(func() {
				a.pageContent(c)
			})
		})
	})
	a.console(c)
}

// sidebar paints the navigation column: the brand, the icon items, and the
// signed-in operator at the foot. ui.Sidebar keeps the chosen item's id in
// a.page and reports a change.
func (a *App) sidebar(c *ui.Context) {
	ui.Themed(c, sidebarTheme()).Key("sidebar-scope").
		Width(228).FillHeight().Background(sidebarBg).Shrink(0).Children(func() {
		ui.Column(c).Fill().Children(func() {
			// Brand: a teal rounded logo mark and the wordmark.
			ui.Row(c).Padding(18, 18, 14).Gap(11).AlignItems(ui.Center).Shrink(0).Children(func() {
				ui.Box(c).Size(34, 34).Radius(10).Center().Shrink(0).
					Background(ui.Hex("#2fd0bd")).Children(func() {
					ui.Text(c, "存").FontSize(17).Bold().TextColor(ui.Hex("#08221e"))
				})
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, "存号网络").FontSize(15).Bold().TextColor(ui.Hex("#eaf2fa"))
					ui.Text(c, "账号运营平台").FontSize(10.5).TextColor(sidebarMuted)
				})
			})
			ui.Sidebar(c, &a.page, func() {
				item := func(id string, icon *ui.SVG, label string) {
					ui.SidebarItem(c, id, icon, label)
				}
				item(pageDashboard, iconDashboard, "工作台")
				item(pageSms, iconSms, "短信管理")
				ui.SidebarSection(c, "账号管理", &a.sectionOpen, func() {
					item(pageAccounts, iconAccount, "账号管理")
					item(pageStock, iconStock, "账号库存管理")
					item(pageExtract, iconExtract, "提号管理")
				})
				item(pageSlots, iconSlot, "卡位管理")
				item(pagePhones, iconPhone, "云手机管理")
				item(pageMerchant, iconMerchant, "商家管理")
				item(pageWallet, iconWallet, "系统钱包")
				item(pageSystem, iconSystem, "系统管理")
			}).Grow(1)
			// Footer: the signed-in operator on a subtle panel.
			ui.Box(c).Margin(12, 12, 12, 12).Padding(10).Radius(12).
				Background(sidebarPanel).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Avatar(c, "喜", nil).Size(32, 32)
					ui.Column(c).Gap(1).Grow(1).Children(func() {
						ui.Text(c, "喜相逢").FontSize(12.5).Bold().TextColor(ui.Hex("#eaf2fa")).SingleLine()
						ui.Text(c, "管理员").FontSize(10.5).TextColor(sidebarMuted).SingleLine()
					})
					ui.Box(c).Size(8, 8).Radius(999).Background(ui.Hex("#2fd0bd")).Shrink(0)
				})
			})
		})
	})
}

// topBar is the horizontal header above the content: the four top tabs on
// the left and a right cluster with a search hint and a badge.
func (a *App) topBar(c *ui.Context) {
	t := c.Theme()
	labs := []string{"账号管理", "用户信息", "收信箱", "云手机管理"}
	if a.tab < 0 || a.tab >= len(labs) {
		a.tab = 0
	}
	ui.Row(c).FillWidth().Padding(16, 0).Height(56).AlignItems(ui.Center).Gap(14).
		Background(cardBg).Shrink(0).Children(func() {
		ui.Tabs(c, &a.tab, labs...)
		ui.Box(c).Grow(1)
		ui.Row(c).Padding(5, 10).Radius(8).Gap(6).AlignItems(ui.Center).
			Border(1, t.Border).Children(func() {
			ui.Icon(c, iconSearch).FontSize(13).TextColor(t.TextMuted)
			ui.Text(c, "搜索").FontSize(12).TextColor(t.TextMuted)
		})
		pill(c, "运营模式", ui.Hex("#12a594"))
	})
	// Hairline under the bar.
	ui.Box(c).FillWidth().Height(1).Background(hairline).Shrink(0)
}

// pageContent dispatches to the page for a.page.
func (a *App) pageContent(c *ui.Context) {
	switch a.page {
	case pageAccounts:
		a.accountsPage(c)
	case pagePhones:
		a.phonesPage(c)
	case pageSms:
		a.smsPage(c)
	case pageSlots:
		a.slotsPage(c)
	case pageWallet:
		a.walletPage(c)
	case pageSystem:
		a.systemPage(c)
	case pageStock, pageExtract, pageMerchant:
		a.placeholderPage(c)
	default:
		a.dashboardPage(c)
	}
}
