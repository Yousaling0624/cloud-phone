package app

import (
	"context"
	"fmt"

	"cloud-phone/internal/model"

	"github.com/egoist/mygo/plugins/loading"
	"github.com/egoist/mygo/ui"
)

// openConsole opens the cloud-phone console dialog on acc.
func (a *App) openConsole(acc model.Account) {
	a.consoleAccount = acc
	a.consoleOpen = true
	a.consoleWebTab = 0
	a.loginSettings = false
}

// console is the "云手机控制台" dialog: a left column with account and
// security details, and a right column with the login tabs, the settings
// fold, the verification-code card with its spinner, and the result
// buttons. It refreshes a.consoleAccount from the store each frame so
// worker updates (a code arriving) show as soon as they land.
func (a *App) console(c *ui.Context) {
	if !a.consoleOpen {
		return
	}
	t := c.Theme()
	if cur, err := a.svc.Repo().GetAccount(context.Background(), a.consoleAccount.ID); err == nil {
		a.consoleAccount = cur
	}
	acc := a.consoleAccount

	ui.Modal(c, &a.consoleOpen, func() {
		ui.Column(c).Gap(14).Width(880).Height(580).Children(func() {
			// Title row: avatar + name + bound phone, close at the right.
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Avatar(c, acc.Display, nil).Size(36, 36)
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, "云手机控制台").FontSize(15).Bold()
					ui.Text(c, acc.Number+" · "+acc.Display).FontSize(11.5).TextColor(t.TextMuted)
				})
				pill(c, acc.BoundPhone, ui.Hex("#3b82f6"))
				ui.Box(c).Grow(1)
				if iconButton(c, iconTrash, t.TextMuted) {
					a.consoleOpen = false
				}
			})
			ui.Box(c).FillWidth().Height(1).Background(hairline).Shrink(0)
			ui.Split(c, &a.consoleSize, func() {
				a.consoleLeft(c, acc)
			}, func() {
				a.consoleRight(c, acc)
			}).Grow(1)
			ui.Box(c).FillWidth().Height(1).Background(hairline).Shrink(0)
			a.consoleToolbar(c, acc)
		})
	})
}

// consoleLeft is the "账号与安全" column.
func (a *App) consoleLeft(c *ui.Context, acc model.Account) {
	t := c.Theme()
	ui.Column(c).FillHeight().PaddingX(10).Gap(12).Children(func() {
		// Profile card.
		ui.Column(c).Padding(14).Gap(12).Radius(12).Background(t.Background).
			Border(1, t.Border).Children(func() {
			ui.Row(c).Gap(11).AlignItems(ui.Center).Children(func() {
				ui.Avatar(c, acc.Display, nil).Size(44, 44)
				ui.Column(c).Gap(2).Grow(1).Children(func() {
					ui.Text(c, acc.Number).FontSize(14).Bold().SingleLine()
					ui.Text(c, acc.Display).FontSize(12).TextColor(t.TextMuted).SingleLine()
				})
			})
			if ui.Button(c, "添加 / 注册账号").Clicked() {
				c.Toast("注册流程待接入")
			}
		})
		sectionLabel(c, "账号与安全")
		kv := func(k, v string) {
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				ui.Text(c, k).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, v).FontSize(12).SingleLine()
			})
		}
		kv("关联QQ", boolText(acc.LinkedQQ, "已关联", "未关联"))
		if acc.LinkedQID != "" {
			kv("QID", acc.LinkedQID)
		} else {
			kv("QID", "未开启")
		}
		kv("手机号", orDash(acc.VerifyPhone))
		kv("账号绑定与授权", "管理")
		ui.Box(c).FillWidth().Height(1).Background(hairline).Shrink(0)
		link := func(label string) {
			ui.Row(c).FillWidth().Padding(9, 10).Radius(8).Gap(8).AlignItems(ui.Center).
				Cursor(ui.CursorPointer).Background(t.Surface).Children(func() {
				ui.Box(c).Size(6, 6).Radius(999).Background(ui.Hex("#12a594")).Shrink(0)
				ui.Text(c, label).FontSize(12.5).TextColor(ui.Hex("#0c7c6f"))
			}).OnClick(func() { c.Toast(label) })
		}
		link("修改密码")
		link("登录设备管理")
		link("手势解锁")
	})
}

// consoleRight is the login side: the tabs, the login-method select, the
// settings fold, the code card and the result buttons.
func (a *App) consoleRight(c *ui.Context, acc model.Account) {
	t := c.Theme()
	ui.Column(c).FillHeight().Gap(12).PaddingX(10).Children(func() {
		ui.Tabs(c, &a.consoleWebTab, "登扣", "快速扫码", "快速查询", "提号")
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "登录方式").FontSize(12).TextColor(t.TextMuted)
			method := "账号密码登录"
			ui.Select(c.Key("login-method"), &method, []string{"账号密码登录", "扫码登录", "短信验证码"}).Width(190)
			ui.Box(c).Grow(1)
			if ui.PrimaryButton(c, "开始登扣").Clicked() {
				a.startLogin(acc)
			}
		})
		ui.Collapsible(c, "登录设置", &a.loginSettings, func() {
			ui.Column(c).Gap(8).Padding(12).Radius(10).Background(t.Background).
				Border(1, t.Border).Children(func() {
				row := func(k string) {
					ui.Row(c).Justify(ui.SpaceBetween).FillWidth().Children(func() {
						ui.Text(c, k).FontSize(12).TextColor(t.TextMuted)
						var on = true
						ui.Switch(c, &on)
					})
				}
				row("自动获取验证码")
				row("登录后自动关闭云机")
				row("失败自动重试")
			})
		})
		a.codeCard(c, acc)
	})
}

// codeCard shows the verification code, the waiting spinner, and the rule
// error when the login failed, then the two result buttons.
func (a *App) codeCard(c *ui.Context, acc model.Account) {
	t := c.Theme()
	ui.Column(c).FillWidth().Grow(1).Padding(16).Radius(12).Background(t.Background).
		Border(1, t.Border).Children(func() {
		ui.Column(c).Gap(12).Fill().Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Text(c, "登录验证码").FontSize(12).Bold().TextColor(textMuted2)
				ui.Box(c).Grow(1)
				switch acc.Login {
				case model.LoginWaiting:
					loading.Arc(c, loading.Options{Size: 18, Color: t.Accent})
					ui.Text(c, "等待短信…").FontSize(12).TextColor(t.TextMuted)
				case model.LoginReady, model.LoginDone:
					ui.Box(c).Padding(6, 14).Radius(8).Background(accentSoft).Children(func() {
						ui.Text(c, acc.VerifyCode).FontSize(24).Bold().TextColor(ui.Hex("#0c7c6f"))
					})
					if ui.Button(c, "复制").Clicked() {
						c.Toast("验证码已复制")
					}
				default:
					ui.Text(c, "— — — —").FontSize(24).Bold().TextColor(t.TextMuted)
				}
			})
			if acc.Login == model.LoginFail {
				ui.Row(c).FillWidth().Padding(10, 12).Radius(10).
					Background(ui.Hex("#e5484d").Alpha(0.08)).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(7, 7).Radius(999).Background(ui.Hex("#e5484d")).Shrink(0)
					ui.Text(c, "执行规则失败: "+orDash(acc.LoginRule)).FontSize(12).TextColor(ui.Hex("#c83a3f"))
				})
			}
			if acc.Login == model.LoginWaiting {
				ui.Text(c, "正在等待接收短信验证码，请稍候…").FontSize(12).TextColor(t.TextMuted)
			}
			ui.Box(c).Grow(1)
			ui.Box(c).FillWidth().Height(1).Background(hairline).Shrink(0)
			ui.Text(c, "登录结果确认").FontSize(12).Bold().TextColor(textMuted2)
			ui.Row(c).Gap(10).FillWidth().Children(func() {
				if ui.PrimaryButton(c, "登录成功并关闭").Grow(1).Clicked() {
					a.confirmLogin(acc, true, c)
				}
				if ui.Button(c, "登录失败并关闭").Grow(1).Clicked() {
					a.confirmLogin(acc, false, c)
				}
			})
		})
	})
}

// consoleToolbar is the bottom strip of console tools.
func (a *App) consoleToolbar(c *ui.Context, acc model.Account) {
	tools := []string{"放大操作", "获取截图", "切换IP", "切真机网络", "验证码", "等待接收", "详情", "结束本次操作"}
	ui.Row(c).FillWidth().Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
		for _, label := range tools {
			label := label
			if ui.Button(c, label).FontSize(12).Clicked() {
				if label == "结束本次操作" {
					a.consoleOpen = false
					return
				}
				c.Toast(label)
			}
		}
	})
}

// startLogin runs the login rule in the background; the service calls back
// on a worker goroutine, and the callback publishes to the UI through
// win.Update so the code (or the rule error) shows when it lands.
func (a *App) startLogin(acc model.Account) {
	ctx := context.Background()
	id := acc.ID
	a.svc.StartLogin(ctx, id, func(updated model.Account, err error) {
		if a.win == nil {
			return
		}
		a.win.Update(func() {
			a.reload()
			if a.consoleOpen {
				a.consoleAccount = updated
			}
		})
	})
}

// confirmLogin records the user's verdict and closes the dialog.
func (a *App) confirmLogin(acc model.Account, ok bool, c *ui.Context) {
	updated, err := a.svc.ConfirmLogin(context.Background(), acc.ID, ok)
	if err != nil {
		c.Toast("操作失败")
		return
	}
	a.reload()
	a.consoleAccount = updated
	a.consoleOpen = false
	if ok {
		c.Toast(fmt.Sprintf("账号 %s 登录成功", acc.Number))
	} else {
		c.Toast(fmt.Sprintf("账号 %s 登录失败", acc.Number))
	}
}

func boolText(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
