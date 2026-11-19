package app

import (
	"context"
	"image/png"
	"os"
	"testing"

	"cloud-phone/internal/driver"
	"cloud-phone/internal/model"
	"cloud-phone/internal/service"
	"cloud-phone/internal/store"

	"github.com/egoist/mygo/ui"
)

// TestDumpScreenshots renders the main screens to PNGs for visual review.
// It is skipped unless CLOUDPHONE_SHOTS is set, so normal runs stay fast.
func TestDumpScreenshots(t *testing.T) {
	if os.Getenv("CLOUDPHONE_SHOTS") == "" {
		t.Skip("set CLOUDPHONE_SHOTS=1 to dump screenshot PNGs")
	}
	repo, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	if err := repo.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	sms := driver.MockSms{}
	svc := service.New(repo, driver.MockPhone{}, sms, driver.MockLogin{Sms: sms, FailRate: 0})
	a := New(svc)

	render := func(name string) {
		tt := ui.NewTester(a.View, 1280, 800)
		tt.Frame()
		img := tt.Image()
		f, err := os.Create("../../.shots/" + name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}

	render("shot-accounts")

	a.page = pagePhones
	render("shot-phones")

	a.page = pageWallet
	render("shot-wallet")

	a.page = pageSms
	render("shot-sms")

	a.page = pageSlots
	render("shot-slots")

	a.page = pageSystem
	render("shot-system")

	a.page = pageDashboard
	render("shot-dashboard")

	a.page = pageAccounts
	a.openConsole(a.accounts[0])
	render("shot-console")

	// A failing login shows the rule error; persist it so the console's
	// per-frame re-read from the store keeps it.
	acc0 := a.consoleAccount
	acc0.Login = model.LoginFail
	acc0.LoginRule = "qq_account_management_login_rule"
	_ = repo.UpdateAccount(context.Background(), acc0)
	render("shot-console-fail")

	// A code arrived: the ready state.
	acc0.Login = model.LoginReady
	acc0.VerifyCode = "482913"
	_ = repo.UpdateAccount(context.Background(), acc0)
	render("shot-console-ready")

	// Waiting: the spinner shows while a code is on its way.
	acc0.Login = model.LoginWaiting
	_ = repo.UpdateAccount(context.Background(), acc0)
	render("shot-console-wait")
}