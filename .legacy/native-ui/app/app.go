// Package app builds the cloud-phone operator console: a dark sidebar, a
// top tab bar, and a large account table whose rows open a "cloud phone
// console" dialog. The state in App is the data the view draws; the view
// rebuilds the interface from it every frame, on the main thread.
package app

import (
	"context"

	"cloud-phone/internal/model"
	"cloud-phone/internal/service"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Sidebar item ids, the value ui.Sidebar keeps in App.page.
const (
	pageDashboard = "dashboard" // 工作台
	pageSms       = "sms"       // 短信管理
	pageAccounts  = "accounts"  // 账号管理
	pageStock     = "stock"     // 账号库存管理
	pageExtract   = "extract"   // 提号管理
	pageSlots     = "slots"     // 卡位管理
	pagePhones    = "phones"    // 云手机管理
	pageMerchant  = "merchant"  // 商家管理
	pageWallet    = "wallet"    // 系统钱包
	pageSystem    = "system"    // 系统管理
)

// App is the console's state. It lives on the main thread: only the view,
// input callbacks and Window.Update closures touch it. Workers publish
// results through win.Update.
type App struct {
	svc *service.Service
	win *mygo.Window

	// Navigation.
	page        string
	tab         int
	sectionOpen bool // whether the 账号管理 section is expanded

	// Accounts page.
	accounts      []model.Account
	accountQuery  string // the search box
	accountState  string // the status filter ("全部状态" for none)
	phoneFilter   string
	accountTable  ui.ListState
	accountSel    int
	accountChecks ui.Selection[int64]

	// Console dialog.
	consoleOpen    bool
	consoleAccount model.Account
	consoleSize    float32 // width of the left column of the Split
	consoleWebTab  int     // right-hand tabs: 登扣/快速扫码/快速查询/提号
	loginSettings  bool    // "登录设置" fold

	// Cloud phones page.
	phones     []model.CloudPhone
	phoneTable ui.ListState
	phoneSel   int

	// SMS page.
	sms []model.Sms

	// Card slots page.
	sims []model.SimSlot

	// Users / system page.
	users []model.User

	// Wallet page.
	wallet      model.Wallet
	ledger      []model.LedgerEntry
	topUpAmount string
}

// New returns an App over svc, with sane defaults matching the reference:
// the accounts page open on its first tab.
func New(svc *service.Service) *App {
	a := &App{
		svc:          svc,
		page:         pageAccounts,
		sectionOpen:  true,
		accountState: "全部状态",
		phoneFilter:  "全部云机",
		consoleSize:  300,
		accountSel:   -1,
		phoneSel:     -1,
		topUpAmount:  "10",
	}
	a.accountTable.Selected = &a.accountSel
	a.accountTable.Selection = &a.accountChecks
	a.accountTable.Key = func(i int) any {
		if i < 0 || i >= len(a.accounts) {
			return i
		}
		return a.accounts[i].ID
	}
	a.phoneTable.Selected = &a.phoneSel
	a.phoneTable.Key = func(i int) any {
		if i < 0 || i >= len(a.phones) {
			return i
		}
		return a.phones[i].ID
	}
	a.reload()
	return a
}

// SetWindow records the window so worker goroutines can publish results.
func (a *App) SetWindow(w *mygo.Window) { a.win = w }

// reload reads every dataset the app shows from the service. It runs on
// the main thread (from the view under a reload request, or from a
// Window.Update closure); it never blocks on drivers.
func (a *App) reload() {
	ctx := context.Background()
	if accs, err := a.svc.Accounts(ctx); err == nil {
		a.accounts = accs
	}
	if phones, err := a.svc.Phones(ctx); err == nil {
		a.phones = phones
	}
	if sms, err := a.svc.Repo().ListSms(ctx); err == nil {
		a.sms = sms
	}
	if sims, err := a.svc.Repo().ListSims(ctx); err == nil {
		a.sims = sims
	}
	if users, err := a.svc.Repo().ListUsers(ctx); err == nil {
		a.users = users
	}
	if w, led, err := a.svc.Wallet(ctx); err == nil {
		a.wallet = w
		a.ledger = led
	}
}

// refreshUI asks for a rebuild after a worker changed state, when a window
// exists; in tests (no window) the tester drives frames itself.
func (a *App) refreshUI() {
	if a.win != nil {
		a.win.Update(func() {})
	}
}
