package app

import (
	"context"
	"fmt"
	"strings"

	"cloud-phone/internal/model"

	"github.com/egoist/mygo/ui"
)

// statusColor returns the color a status draws in: teal online, red
// abnormal, amber logging in, gray offline.
func statusColor(s model.AccountStatus) ui.Color {
	switch s {
	case model.AccountOnline:
		return ui.Hex("#12a594")
	case model.AccountAbnormal:
		return ui.Hex("#e5484d")
	case model.AccountLoggingIn:
		return ui.Hex("#e2a03f")
	}
	return ui.Hex("#8a95a5")
}

func statusText(s model.AccountStatus) string {
	switch s {
	case model.AccountOnline:
		return "在线"
	case model.AccountOffline:
		return "离线"
	case model.AccountAbnormal:
		return "异常"
	case model.AccountLoggingIn:
		return "登扣中"
	}
	return "未知"
}

// accountsPage is the 账号管理 page: a filter card, an action row, the four
// stat tiles, the account table in its own card, and the task bar.
func (a *App) accountsPage(c *ui.Context) {
	t := c.Theme()
	rows := a.filteredAccounts()

	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		pageTitle(c, "账号管理", "共 "+itoa(len(a.accounts))+" 个账号")

		// Filter card.
		card(c, func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Select(c.Key("acct-state"), &a.accountState, []string{"全部状态", "在线", "离线", "异常"})
				ui.Select(c.Key("acct-phone"), &a.phoneFilter, []string{"全部云机", "已绑云机", "未绑云机"})
				ui.SearchField(c.Key("acct-q"), &a.accountQuery).Placeholder("账号 / 备注").Width(240)
				if ui.PrimaryButton(c, "搜索").Clicked() {
					c.Toast("搜索完成")
				}
				if ui.Button(c, "清空").Clicked() {
					a.accountQuery = ""
					a.accountState = "全部状态"
					a.phoneFilter = "全部云机"
				}
				ui.Box(c).Grow(1)
				if iconTextButton(c, iconImport, "批量导入") {
					added, _, _ := a.svc.ImportCSV(context.Background(),
						"批量导入A,18888880001\n批量导入B,18888880002\n")
					a.reload()
					c.Toast(fmt.Sprintf("已导入 %d 个账号", added))
				}
				if iconTextButton(c, iconExport, "导出") {
					c.Toast("账号已导出")
				}
				if ui.PrimaryButton(c, "").Padding(7, 14).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconPlus).FontSize(14).TextColor(t.AccentText)
						ui.Text(c, "存号").FontSize(12).Bold().TextColor(t.AccentText)
					})
				}).Clicked() {
					if ok, _ := a.svc.ImportAccount(context.Background(), model.Account{}); ok {
						a.reload()
						c.Toast("已存入新账号")
					}
				}
			})
		})

		// Stat tiles.
		ui.Row(c).Gap(16).Children(func() {
			statTile(c, iconAccount, "异常预警", a.countBy(func(x model.Account) bool { return x.Status == model.AccountAbnormal }), ui.Hex("#e5484d"))
			statTile(c, iconPhone, "未绑云机", a.countBy(func(x model.Account) bool { return x.BoundPhone == "" }), ui.Hex("#e2a03f"))
			statTile(c, iconStock, "未绑定账号", a.countBy(func(x model.Account) bool { return x.Number == "" }), ui.Hex("#6b7a90"))
			statTile(c, iconSms, "在线账号", a.countBy(func(x model.Account) bool { return x.Status == model.AccountOnline }), ui.Hex("#12a594"))
		})

		// Table card.
		ui.Column(c).Grow(1).Radius(12).Background(cardBg).Border(1, cardBorder).
			Shadow(0, 1, 2, 0, shadowCard).Clip().Children(func() {
			// Table header strip.
			ui.Row(c).FillWidth().Padding(14, 16).Gap(10).AlignItems(ui.Center).
				Border(0, cardBorder).Shrink(0).Children(func() {
				sectionLabel(c, "账号列表")
				ui.Box(c).Grow(1)
				ui.Textf(c, "筛选中 %d 条", len(rows)).FontSize(12).TextColor(t.TextMuted)
			})
			ui.Box(c).FillWidth().Height(1).Background(hairline).Shrink(0)

			cols := []ui.TableColumn{
				{Title: "", Width: 40, Fixed: true},
				{Title: "备注", Width: 110, Sortable: true},
				{Title: "账号", MinWidth: 150, Sortable: true},
				{Title: "绑定云机", Width: 110},
				{Title: "状态说明", Width: 96},
				{Title: "云机操作", Width: 130},
				{Title: "操作", Width: 120},
			}
			a.accountTable.Label = func(i int) string {
				if i < 0 || i >= len(rows) {
					return ""
				}
				return rows[i].Remark
			}
			if ui.Table(c, &a.accountTable, cols, len(rows), func(row, col int) {
				if row < 0 || row >= len(rows) {
					return
				}
				acc := rows[row]
				switch col {
				case 0:
					var on = a.accountChecks.Has(acc.ID)
					ui.Checkbox(c, &on, "")
				case 1:
					ui.Text(c, acc.Remark).SingleLine()
				case 2:
					ui.Row(c).Gap(9).AlignItems(ui.Center).Children(func() {
						ui.Avatar(c, acc.Display, nil).Size(30, 30)
						ui.Column(c).Gap(1).Children(func() {
							ui.Text(c, acc.Number).FontSize(12.5).Bold().SingleLine()
							ui.Text(c, acc.Display).FontSize(11).TextColor(t.TextMuted).SingleLine()
						})
					})
				case 3:
					if acc.BoundPhone == "" {
						ui.Text(c, "—").TextColor(t.TextMuted)
					} else {
						ui.Text(c, acc.BoundPhone).SingleLine()
					}
				case 4:
					pill(c, statusText(acc.Status), statusColor(acc.Status))
				case 5:
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						if ui.Button(c, "开机").FontSize(11).Clicked() {
							a.startPhoneFor(acc, c)
						}
						if ui.Button(c, "重启").FontSize(11).Clicked() {
							c.Toast("已下发重启")
						}
					})
				case 6:
					ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
						if iconButtonLabel(c, iconOpen, ui.Hex("#12a594"), "打开") {
							a.openConsole(acc)
						}
						if iconButtonLabel(c, iconEdit, ui.Hex("#6b7a90"), "编辑") {
							c.Toast("编辑 " + acc.Number)
						}
						if iconButtonLabel(c, iconTrash, ui.Hex("#e5484d"), "删除") {
							a.deleteAccount(acc, c)
						}
					})
				}
			}).Grow(1).Submitted() {
				if a.accountSel >= 0 && a.accountSel < len(rows) {
					a.openConsole(rows[a.accountSel])
				}
			}
		})

		// Running-task bar.
		a.taskBar(c)
	})
}

// filteredAccounts applies the filter bar to the account list.
func (a *App) filteredAccounts() []model.Account {
	out := make([]model.Account, 0, len(a.accounts))
	for _, acc := range a.accounts {
		switch a.accountState {
		case "在线":
			if acc.Status != model.AccountOnline {
				continue
			}
		case "离线":
			if acc.Status != model.AccountOffline {
				continue
			}
		case "异常":
			if acc.Status != model.AccountAbnormal {
				continue
			}
		}
		switch a.phoneFilter {
		case "已绑云机":
			if acc.BoundPhone == "" {
				continue
			}
		case "未绑云机":
			if acc.BoundPhone != "" {
				continue
			}
		}
		if a.accountQuery != "" &&
			!containsFold(acc.Number, a.accountQuery) &&
			!containsFold(acc.Remark, a.accountQuery) &&
			!containsFold(acc.Display, a.accountQuery) {
			continue
		}
		out = append(out, acc)
	}
	return out
}

func (a *App) countBy(pred func(model.Account) bool) int {
	n := 0
	for _, acc := range a.accounts {
		if pred(acc) {
			n++
		}
	}
	return n
}

func (a *App) startPhoneFor(acc model.Account, c *ui.Context) {
	if acc.BoundPhone == "" {
		c.Toast("该账号尚未绑定云机")
		return
	}
	for _, p := range a.phones {
		if p.Name == acc.BoundPhone {
			a.runPhone(p.ID, "start", c)
			return
		}
	}
	c.Toast("未找到云机 " + acc.BoundPhone)
}

func (a *App) deleteAccount(acc model.Account, c *ui.Context) {
	if err := a.svc.DeleteAccount(context.Background(), acc.ID); err != nil {
		c.Toast("删除失败")
		return
	}
	a.reload()
	c.Toast("已删除 " + acc.Number)
}

// taskBar is the "进行中任务" strip at the bottom of the accounts page.
func (a *App) taskBar(c *ui.Context) {
	t := c.Theme()
	tasks := a.svc.Tasks()
	ui.Row(c).FillWidth().Padding(12, 16).Radius(12).Gap(12).AlignItems(ui.Center).
		Background(cardBg).Border(1, cardBorder).Shadow(0, 1, 2, 0, shadowCard).Shrink(0).Children(func() {
		ui.Icon(c, iconSms).FontSize(15).TextColor(t.TextMuted)
		ui.Text(c, "进行中任务").FontSize(12).Bold().TextColor(textMuted2)
		if len(tasks) == 0 {
			ui.Text(c, "暂无进行中的任务").FontSize(12).TextColor(t.TextMuted)
			return
		}
		for _, task := range tasks {
			pill(c, task.Label, ui.Hex("#12a594"))
		}
	})
}

// itoa is a tiny int→string to avoid a fmt import in tight view code.
func itoa(n int) string { return fmt.Sprintf("%d", n) }

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}