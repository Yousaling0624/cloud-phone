package app

import (
	"context"
	"testing"
	"time"

	"cloud-phone/internal/driver"
	"cloud-phone/internal/model"
	"cloud-phone/internal/service"
	"cloud-phone/internal/store"

	"github.com/egoist/mygo/ui"
)

// newTestApp returns an App over a seeded in-memory database with a login
// driver that always succeeds, so the console test is deterministic.
func newTestApp(t *testing.T) *App {
	t.Helper()
	repo, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	if err := repo.Seed(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	sms := driver.MockSms{}
	svc := service.New(repo, driver.MockPhone{}, sms, driver.MockLogin{Sms: sms, FailRate: 0})
	return New(svc)
}

// TestShellShowsSidebarAndTable checks the basic chrome is present on the
// accounts page: the sidebar item and the table's column headers.
func TestShellShowsSidebarAndTable(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1280, 800)
	if !tt.HasText("账号管理") {
		t.Fatalf("sidebar item missing; texts %q", tt.Texts())
	}
	for _, want := range []string{"备注", "账号", "绑定云机", "状态说明", "云机操作", "操作"} {
		if !tt.HasText(want) {
			t.Errorf("table header %q missing", want)
		}
	}
	if !tt.HasText("异常预警") || !tt.HasText("在线账号") {
		t.Errorf("stat strip missing")
	}
}

// TestOpenConsole opens the console dialog from a row's 打开 button and
// checks the dialog's landmarks show.
func TestOpenConsole(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1280, 800)
	if err := tt.Click("打开"); err != nil {
		t.Fatalf("click 打开: %v", err)
	}
	tt.Frame()
	if !a.consoleOpen {
		t.Fatalf("console did not open")
	}
	for _, want := range []string{"云手机控制台", "账号与安全", "登录验证码", "登录成功并关闭", "登录失败并关闭"} {
		if !tt.HasText(want) {
			t.Errorf("console landmark %q missing; texts %q", want, tt.Texts())
		}
	}
}

// TestLoginSuccessFlow drives 登扣 then 登录成功并关闭, then asserts the
// account became online. The mock login's SMS wait is real time, so the
// test waits for the code via the service's store state.
func TestLoginSuccessFlow(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1280, 800)
	if err := tt.Click("打开"); err != nil {
		t.Fatalf("click 打开: %v", err)
	}
	tt.Frame()
	id := a.consoleAccount.ID

	if err := tt.Click("开始登扣"); err != nil {
		t.Fatalf("click 开始登扣: %v", err)
	}
	// The mock login sleeps real time (3–8s) before delivering the code,
	// so poll the store for up to 12 seconds.
	var code string
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		cur, err := a.svc.Repo().GetAccount(context.Background(), id)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if cur.Login == model.LoginReady && cur.VerifyCode != "" {
			code = cur.VerifyCode
			break
		}
		if cur.Login == model.LoginFail {
			t.Fatalf("login failed unexpectedly")
		}
		time.Sleep(100 * time.Millisecond)
		tt.Frame()
	}
	if code == "" {
		t.Fatalf("no verification code arrived in time")
	}
	tt.Frame()

	// The view re-reads the account from the store each frame; a frame
	// now must show the code and the result buttons.
	if !tt.HasText(code) {
		t.Errorf("code %q not shown; texts %q", code, tt.Texts())
	}
	if err := tt.Click("登录成功并关闭"); err != nil {
		t.Fatalf("click success: %v", err)
	}
	tt.Frame()
	cur, err := a.svc.Repo().GetAccount(context.Background(), id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if cur.Status != model.AccountOnline {
		t.Errorf("status = %q, want online", cur.Status)
	}
	if a.consoleOpen {
		t.Errorf("console still open after confirm")
	}
}

// TestFilterAccounts narrows the list by the status filter.
func TestFilterAccounts(t *testing.T) {
	a := newTestApp(t)
	a.accountState = "在线"
	got := a.filteredAccounts()
	for _, acc := range got {
		if acc.Status != model.AccountOnline {
			t.Errorf("filtered row %s has status %s", acc.Number, acc.Status)
		}
	}
	if len(got) == 0 {
		t.Errorf("no online accounts; seed should include some")
	}
}
// TestSidebarNavigation switches pages by clicking a sidebar item.
func TestSidebarNavigation(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.View, 1280, 800)
	if err := tt.Click("系统钱包"); err != nil {
		t.Fatalf("click 系统钱包: %v", err)
	}
	tt.Frame()
	if a.page != pageWallet {
		t.Fatalf("page = %q, want %q", a.page, pageWallet)
	}
	if !tt.HasText("云币余额") {
		t.Errorf("wallet page not shown; texts %q", tt.Texts())
	}
	if err := tt.Click("云手机管理"); err != nil {
		t.Fatalf("click 云手机管理: %v", err)
	}
	tt.Frame()
	if a.page != pagePhones {
		t.Fatalf("page = %q, want %q", a.page, pagePhones)
	}
}
