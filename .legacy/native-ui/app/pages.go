package app

import (
	"context"
	"fmt"
	"strconv"

	"cloud-phone/internal/model"

	"github.com/egoist/mygo/ui"
)

// dashboardPage is the landing: the four headline tiles plus a couple of
// quick links into the console.
func (a *App) dashboardPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		pageTitle(c, "工作台", "概览")
		ui.Row(c).Gap(16).Children(func() {
			statTile(c, iconAccount, "账号总数", len(a.accounts), ui.Hex("#12a594"))
			statTile(c, iconPhone, "云机总数", len(a.phones), ui.Hex("#3b82f6"))
			statTile(c, iconSms, "在线账号", a.countBy(func(x model.Account) bool { return x.Status == model.AccountOnline }), ui.Hex("#12a594"))
			statTile(c, iconStock, "异常预警", a.countBy(func(x model.Account) bool { return x.Status == model.AccountAbnormal }), ui.Hex("#e5484d"))
		})
		card(c, func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(40, 40).Radius(12).Background(accentSoft).Center().Children(func() {
					ui.Icon(c, iconAccount).FontSize(20).TextColor(ui.Hex("#12a594"))
				})
				ui.Column(c).Gap(2).Grow(1).Children(func() {
					ui.Text(c, "从这里开始").FontSize(14).Bold()
					ui.Text(c, "在左侧选择「账号管理」查看账号列表，或点某行的打开图标进入云手机控制台。").FontSize(12).TextColor(t.TextMuted)
				})
				if ui.PrimaryButton(c, "前往账号管理").Clicked() {
					a.page = pageAccounts
				}
			})
		})
	})
}

// phoneStateColor tints a phone state.
func phoneStateColor(s model.PhoneState) ui.Color {
	switch s {
	case model.PhoneRunning:
		return ui.Hex("#12a594")
	case model.PhoneIdle:
		return ui.Hex("#3b82f6")
	case model.PhoneStopped:
		return ui.Hex("#8a95a5")
	case model.PhoneStarting:
		return ui.Hex("#e2a03f")
	}
	return ui.Hex("#e5484d")
}

func phoneStateText(s model.PhoneState) string {
	switch s {
	case model.PhoneRunning:
		return "运行中"
	case model.PhoneIdle:
		return "空闲"
	case model.PhoneStopped:
		return "已关机"
	case model.PhoneStarting:
		return "启动中"
	case model.PhoneError:
		return "异常"
	}
	return string(s)
}

// phonesPage lists the cloud phones with boot controls.
func (a *App) phonesPage(c *ui.Context) {
	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		pageTitle(c, "云手机管理", itoa(len(a.phones))+" 台")
		ui.Column(c).Grow(1).Radius(12).Background(cardBg).Border(1, cardBorder).
			Shadow(0, 1, 2, 0, shadowCard).Clip().Children(func() {
			cols := []ui.TableColumn{
				{Title: "云机", Width: 150, Sortable: true},
				{Title: "区域", Width: 120},
				{Title: "镜像", Width: 120},
				{Title: "IP", MinWidth: 140},
				{Title: "状态", Width: 110},
				{Title: "操作", Width: 210},
			}
			ui.Table(c, &a.phoneTable, cols, len(a.phones), func(row, col int) {
				if row < 0 || row >= len(a.phones) {
					return
				}
				p := a.phones[row]
				switch col {
				case 0:
					ui.Text(c, p.Name).Bold().SingleLine()
				case 1:
					ui.Text(c, p.Region).SingleLine()
				case 2:
					ui.Text(c, p.Image).SingleLine()
				case 3:
					ui.Text(c, p.IP).SingleLine()
				case 4:
					pill(c, phoneStateText(p.State), phoneStateColor(p.State))
				case 5:
					ui.Row(c).Gap(6).Children(func() {
						if ui.Button(c, "开机").FontSize(11).Clicked() {
							a.runPhone(p.ID, "start", c)
						}
						if ui.Button(c, "重启").FontSize(11).Clicked() {
							a.runPhone(p.ID, "restart", c)
						}
						if ui.Button(c, "关机").FontSize(11).Clicked() {
							a.runPhone(p.ID, "stop", c)
						}
					})
				}
			}).Grow(1)
		})
	})
}

// runPhone runs a phone action on a worker and refreshes when it lands.
func (a *App) runPhone(id int64, action string, c *ui.Context) {
	go func() {
		err := a.svc.SetPhoneState(context.Background(), id, action)
		if a.win == nil {
			return
		}
		a.win.Update(func() {
			a.reload()
			_ = err
		})
	}()
	c.Toast("已下发指令，处理中…")
}

// smsPage lists the received verification messages as cards.
func (a *App) smsPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		pageTitle(c, "短信管理", "收信箱")
		var tab int
		ui.Tabs(c, &tab, "全部短信", "未使用", "已使用")
		ui.Column(c).Grow(1).Gap(10).Children(func() {
			for _, m := range a.sms {
				ui.Row(c).FillWidth().Padding(14, 16).Radius(12).Gap(14).
					AlignItems(ui.Center).Background(cardBg).Border(1, cardBorder).
					Shadow(0, 1, 2, 0, shadowCard).Children(func() {
					ui.Box(c).Size(38, 38).Radius(11).Background(accentSoft).Center().Shrink(0).Children(func() {
						ui.Text(c, "码").FontSize(14).Bold().TextColor(ui.Hex("#0c7c6f"))
					})
					ui.Column(c).Gap(3).Grow(1).Children(func() {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Text(c, m.Phone).FontSize(13).Bold().SingleLine()
							if m.Used {
								pill(c, "已使用", ui.Hex("#8a95a5"))
							} else {
								pill(c, "未使用", ui.Hex("#12a594"))
							}
						})
						ui.Text(c, m.Content).FontSize(12).TextColor(t.TextMuted).SingleLine()
					})
					ui.Text(c, m.Code).FontSize(20).Bold().TextColor(ui.Hex("#0c7c6f"))
					ui.Text(c, m.ReceivedAt.Format("01-02 15:04")).FontSize(11).TextColor(t.TextMuted)
				})
			}
		})
	})
}

// slotsPage lists the SIM card slots.
func (a *App) slotsPage(c *ui.Context) {
	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		pageTitle(c, "卡位管理", itoa(len(a.sims))+" 个卡位")
		ui.Column(c).Grow(1).Radius(12).Background(cardBg).Border(1, cardBorder).
			Shadow(0, 1, 2, 0, shadowCard).Clip().Children(func() {
			cols := []ui.TableColumn{
				{Title: "手机号", MinWidth: 160},
				{Title: "运营商", Width: 140},
				{Title: "绑定账号", Width: 140},
				{Title: "状态", Width: 130},
			}
			var dummy ui.ListState
			var sel = -1
			dummy.Selected = &sel
			ui.Table(c, &dummy, cols, len(a.sims), func(row, col int) {
				if row < 0 || row >= len(a.sims) {
					return
				}
				s := a.sims[row]
				switch col {
				case 0:
					ui.Text(c, s.Phone).Bold().SingleLine()
				case 1:
					ui.Text(c, s.Carrier).SingleLine()
				case 2:
					if s.BoundAccount == 0 {
						ui.Text(c, "—").TextColor(textMuted2)
					} else {
						ui.Textf(c, "#%d", s.BoundAccount)
					}
				case 3:
					col := ui.Hex("#12a594")
					if s.Status == "停机" {
						col = ui.Hex("#e5484d")
					} else if s.Status == "空闲" {
						col = ui.Hex("#3b82f6")
					}
					pill(c, s.Status, col)
				}
			}).Grow(1)
		})
	})
}

// walletPage shows the balance, top-up controls, and the ledger.
func (a *App) walletPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		pageTitle(c, "系统钱包", "")
		ui.Row(c).Gap(16).Children(func() {
			// Balance card with an accent wash.
			ui.Box(c).Grow(1).Basis(0).Padding(20).Radius(12).Border(1, cardBorder).
				Background(ui.Hex("#0c7c6f")).Shadow(0, 6, 18, 0, ui.RGBA(12, 124, 111, 0.28)).Children(func() {
				ui.Column(c).Gap(8).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconWallet).FontSize(16).TextColor(ui.Hex("#c9f2ec"))
						ui.Text(c, "云币余额").FontSize(12.5).TextColor(ui.Hex("#c9f2ec"))
					})
					ui.Textf(c, "￥%.2f", a.wallet.Balance).FontSize(34).Bold().TextColor(ui.Hex("#ffffff"))
				})
			})
			// Top-up card.
			card(c, func() {
				ui.Column(c).Gap(12).Children(func() {
					sectionLabel(c, "在线充值")
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.DecimalInput(c.Key("topup"), &a.topUpAmount, "0.01", "", "0.01").Width(170)
						if ui.PrimaryButton(c, "充值").Clicked() {
							a.doTopUp(c)
						}
					})
					if ui.Button(c, "扣费 1.00（测试）").Clicked() {
						a.doCharge(c)
					}
				})
			}).Grow(1).Basis(0)
		})
		sectionLabel(c, "账单流水")
		ui.Column(c).Grow(1).Gap(8).Children(func() {
			for _, e := range a.ledger {
				col := ui.Hex("#0c7c6f")
				if e.Amount < 0 {
					col = ui.Hex("#e5484d")
				}
				ui.Row(c).FillWidth().Padding(12, 16).Radius(10).AlignItems(ui.Center).
					Gap(12).Background(cardBg).Border(1, cardBorder).Children(func() {
					ui.Box(c).Size(34, 34).Radius(9).Background(col.Alpha(0.10)).Center().Shrink(0).Children(func() {
						ui.Text(c, "币").FontSize(12).Bold().TextColor(col)
					})
					ui.Text(c, e.Reason).FontSize(12.5).Grow(1).SingleLine()
					ui.Text(c, e.At.Format("2006-01-02 15:04")).FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, "￥"+fmt.Sprintf("%.2f", e.Amount)).FontSize(14).Bold().TextColor(col)
				})
			}
		})
	})
}

func (a *App) doTopUp(c *ui.Context) {
	amount, err := strconv.ParseFloat(a.topUpAmount, 64)
	if err != nil || amount <= 0 {
		c.Toast("请输入有效的充值金额")
		return
	}
	if err := a.svc.TopUp(context.Background(), amount); err != nil {
		c.Toast("充值失败: " + err.Error())
		return
	}
	a.reload()
	c.Toast("充值成功")
}

func (a *App) doCharge(c *ui.Context) {
	if err := a.svc.Charge(context.Background(), 1.00, "手动扣费"); err != nil {
		c.Toast("扣费失败: " + err.Error())
		return
	}
	a.reload()
	c.Toast("扣费成功")
}

// systemPage lists the operators and their roles.
func (a *App) systemPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		pageTitle(c, "系统管理", "用户与权限")
		ui.Column(c).Gap(10).Children(func() {
			for _, u := range a.users {
				ui.Row(c).FillWidth().Padding(14, 16).Radius(12).Gap(14).
					AlignItems(ui.Center).Background(cardBg).Border(1, cardBorder).
					Shadow(0, 1, 2, 0, shadowCard).Children(func() {
					ui.Avatar(c, u.Nickname, nil).Size(38, 38)
					ui.Column(c).Gap(2).Grow(1).Children(func() {
						ui.Text(c, u.Nickname).FontSize(13.5).Bold().SingleLine()
						ui.Text(c, u.Username).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
					})
					pill(c, u.Role, ui.Hex("#12a594"))
					var on = u.Enabled
					ui.Switch(c, &on)
				})
			}
		})
	})
}

// placeholderPage is shown for the sections not built out yet.
func (a *App) placeholderPage(c *ui.Context) {
	t := c.Theme()
	label := map[string]string{
		pageStock:    "账号库存管理",
		pageExtract:  "提号管理",
		pageMerchant: "商家管理",
	}[a.page]
	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		pageTitle(c, label, "")
		card(c, func() {
			ui.Column(c).Fill().Center().Gap(10).Padding(40).Children(func() {
				ui.Box(c).Size(56, 56).Radius(16).Background(accentSoft).Center().Children(func() {
					ui.Icon(c, iconStock).FontSize(26).TextColor(ui.Hex("#12a594"))
				})
				ui.Text(c, label).FontSize(16).Bold()
				ui.Text(c, "该模块正在建设中").FontSize(12.5).TextColor(t.TextMuted)
			})
		}).Grow(1)
	})
}
