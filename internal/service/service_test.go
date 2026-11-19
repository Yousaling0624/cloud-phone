package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud-phone/internal/driver"
	"cloud-phone/internal/model"
	"cloud-phone/internal/service"
	"cloud-phone/internal/store"
)

func newSvc(t *testing.T) (*service.Service, store.Repository) {
	t.Helper()
	repo, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	sms := immediateSms{code: "123456"}
	return service.New(repo, driver.MockPhone{}, sms, driver.MockLogin{Sms: sms, FailRate: 0}), repo
}

type immediateSms struct{ code string }

func (s immediateSms) WaitCode(ctx context.Context, phone string) (string, error) {
	return s.code, nil
}

// manualLogin models a provider that hands the login to a human.
type manualLogin struct{}

func (manualLogin) Execute(context.Context, driver.LoginRequest) (driver.LoginResult, error) {
	return driver.LoginResult{OK: true, Manual: true, Message: "请使用 scrcpy 完成登录"}, nil
}

// syncPhone adds device discovery to the mock phone driver.
type syncPhone struct {
	driver.MockPhone
	devices []driver.DeviceInfo
}

func (s *syncPhone) ListDevices(context.Context) ([]driver.DeviceInfo, error) {
	return s.devices, nil
}

// gatedSms blocks until release is closed.
type gatedSms struct{ release chan struct{} }

func (s gatedSms) WaitCode(ctx context.Context, phone string) (string, error) {
	select {
	case <-s.release:
		return "654321", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestImportAccountDedupes(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	if err := svc.ImportAccount(ctx, "", "10001", "", 0); err != nil {
		t.Fatalf("first import: %v", err)
	}
	if err := svc.ImportAccount(ctx, "", "10001", "", 0); !errors.Is(err, service.ErrDuplicate) {
		t.Fatalf("duplicate import err = %v, want ErrDuplicate", err)
	}
	accs, _ := svc.Accounts(ctx)
	n := 0
	for _, a := range accs {
		if a.Number == "10001" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("got %d copies of 10001, want 1", n)
	}
}

func TestImportCSV(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	res, err := svc.ImportCSV(ctx, "# comment\n备注A,20001\n20002\n\n备注A,20001\n")
	if err != nil {
		t.Fatalf("import csv: %v", err)
	}
	if res.Added != 2 {
		t.Errorf("added = %d, want 2", res.Added)
	}
	if len(res.Errors) != 0 {
		t.Errorf("unexpected line errors: %v", res.Errors)
	}
}

func TestWalletTopUpAndCharge(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	if err := svc.Repo().SetBalance(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if err := svc.TopUp(ctx, 5); err != nil {
		t.Fatalf("top up: %v", err)
	}
	w, _, _ := svc.Wallet(ctx)
	if w.Balance != 15 {
		t.Errorf("after top up balance = %v, want 15", w.Balance)
	}
	if err := svc.Charge(ctx, 20, "too much"); !errors.Is(err, service.ErrInsufficientFunds) {
		t.Errorf("charge over balance err = %v, want ErrInsufficientFunds", err)
	}
	if err := svc.Charge(ctx, 4.5, "ok"); err != nil {
		t.Fatalf("charge: %v", err)
	}
	w, _, _ = svc.Wallet(ctx)
	if w.Balance != 10.5 {
		t.Errorf("after charge balance = %v, want 10.5", w.Balance)
	}
}

func TestSetPhoneState(t *testing.T) {
	svc, repo := newSvc(t)
	ctx := context.Background()
	if err := repo.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	phones, _ := svc.Phones(ctx)
	if len(phones) == 0 {
		t.Fatal("no seeded phones")
	}
	id := phones[0].ID
	if err := svc.SetPhoneState(ctx, id, "stop"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	phones, _ = svc.Phones(ctx)
	for _, p := range phones {
		if p.ID == id && p.State != model.PhoneStopped {
			t.Errorf("state = %s, want stopped", p.State)
		}
	}
}

func TestBindingLifecycle(t *testing.T) {
	svc, repo := newSvc(t)
	ctx := context.Background()
	if err := repo.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	accs, _ := svc.Accounts(ctx)
	phones, _ := svc.Phones(ctx)
	if len(accs) < 2 || len(phones) < 2 {
		t.Fatal("seed needs at least two accounts and phones")
	}
	a, b := accs[0], accs[1]
	// Pick two unbound phones so the test does not depend on seed bindings.
	var p1, p2 model.CloudPhone
	for _, p := range phones {
		if p.AccountID == 0 {
			if p1.ID == 0 {
				p1 = p
			} else if p2.ID == 0 {
				p2 = p
			}
		}
	}
	if p1.ID == 0 || p2.ID == 0 {
		t.Fatal("need two unbound phones")
	}

	if err := svc.SetAccountPhone(ctx, a.ID, p1.ID); err != nil {
		t.Fatalf("bind a: %v", err)
	}
	got, _ := repo.GetAccount(ctx, a.ID)
	if got.BoundPhoneID != p1.ID || got.BoundPhone != p1.Name {
		t.Errorf("account binding = %d/%q, want %d/%q", got.BoundPhoneID, got.BoundPhone, p1.ID, p1.Name)
	}
	phone, _ := repo.GetPhone(ctx, p1.ID)
	if phone.AccountID != a.ID {
		t.Errorf("phone account = %d, want %d", phone.AccountID, a.ID)
	}

	if err := svc.SetAccountPhone(ctx, b.ID, p1.ID); !errors.Is(err, service.ErrPhoneBusy) {
		t.Errorf("binding a busy phone err = %v, want ErrPhoneBusy", err)
	}

	if err := svc.SetAccountPhone(ctx, a.ID, p2.ID); err != nil {
		t.Fatalf("rebind a: %v", err)
	}
	old, _ := repo.GetPhone(ctx, p1.ID)
	if old.AccountID != 0 {
		t.Errorf("old phone still points at %d", old.AccountID)
	}

	if err := svc.SetAccountPhone(ctx, a.ID, 0); err != nil {
		t.Fatalf("unbind: %v", err)
	}
	got, _ = repo.GetAccount(ctx, a.ID)
	if got.BoundPhoneID != 0 || got.BoundPhone != "" {
		t.Errorf("after unbind account = %d/%q", got.BoundPhoneID, got.BoundPhone)
	}
}

func TestDeleteAccountClearsReferences(t *testing.T) {
	svc, repo := newSvc(t)
	ctx := context.Background()
	if err := repo.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	accs, _ := svc.Accounts(ctx)
	a := accs[0]
	phones, _ := svc.Phones(ctx)
	var bound model.CloudPhone
	for _, p := range phones {
		if p.AccountID == a.ID {
			bound = p
		}
	}
	if bound.ID == 0 {
		t.Fatal("seeded account has no phone")
	}
	if err := svc.DeleteAccount(ctx, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	p, err := repo.GetPhone(ctx, bound.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.AccountID != 0 {
		t.Errorf("deleted account left phone.account_id = %d", p.AccountID)
	}
	sims, _ := repo.ListSims(ctx)
	for _, s := range sims {
		if s.BoundAccount == a.ID {
			t.Errorf("sim %d still bound to deleted account", s.ID)
		}
	}
}

func TestConfirmFailureStopsLogin(t *testing.T) {
	repo, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	gated := gatedSms{release: make(chan struct{})}
	svc := service.New(repo, driver.MockPhone{}, gated, driver.MockLogin{Sms: gated})
	ctx := context.Background()

	if err := svc.ImportAccount(ctx, "", "70001", "", 0); err != nil {
		t.Fatal(err)
	}
	accs, _ := svc.Accounts(ctx)
	id := accs[0].ID

	if err := svc.StartLogin(ctx, id, model.LoginOptions{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, "waiting", func() bool {
		a, err := repo.GetAccount(ctx, id)
		return err == nil && a.Login == model.LoginWaiting
	})
	if _, err := svc.ConfirmLogin(ctx, id, false); err != nil {
		t.Fatalf("confirm failure: %v", err)
	}
	close(gated.release)
	time.Sleep(100 * time.Millisecond)

	a, _ := repo.GetAccount(ctx, id)
	if a.Status != model.AccountAbnormal || a.Login != model.LoginFail || a.VerifyCode != "" {
		t.Errorf("after failed confirm: status=%s login=%s code=%q", a.Status, a.Login, a.VerifyCode)
	}
	if tasks := svc.Tasks(); len(tasks) != 0 {
		t.Errorf("tasks left: %+v", tasks)
	}
}

func TestRecoverStuckLoginsKeepsReady(t *testing.T) {
	svc, repo := newSvc(t)
	ctx := context.Background()
	if err := repo.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	accs, _ := svc.Accounts(ctx)
	waiting, ready := accs[0], accs[1]
	if err := repo.UpdateAccountLogin(ctx, waiting.ID, model.AccountLoggingIn, model.LoginWaiting, "rule", "111111", "135******81"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateAccountLogin(ctx, ready.ID, model.AccountLoggingIn, model.LoginReady, "rule", "222222", "135******82"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecoverStuckLogins(ctx); err != nil {
		t.Fatal(err)
	}
	w, _ := repo.GetAccount(ctx, waiting.ID)
	if w.Login != model.LoginFail || w.VerifyCode != "" {
		t.Errorf("waiting login not recovered: login=%s code=%q", w.Login, w.VerifyCode)
	}
	r, _ := repo.GetAccount(ctx, ready.ID)
	if r.Login != model.LoginReady || r.VerifyCode != "222222" {
		t.Errorf("ready login should be preserved: login=%s code=%q", r.Login, r.VerifyCode)
	}
}

func TestRepairBindingsPrefersPhoneOwner(t *testing.T) {
	svc, repo := newSvc(t)
	ctx := context.Background()
	if err := repo.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	accs, _ := svc.Accounts(ctx)
	phones, _ := svc.Phones(ctx)
	a, b := accs[0], accs[1]
	var free model.CloudPhone
	for _, p := range phones {
		if p.AccountID == 0 {
			free = p
			break
		}
	}
	if free.ID == 0 {
		t.Fatal("need a free phone")
	}
	// Both accounts claim the same free phone; the store makes b the owner
	// last, leaving a's reverse key stale.
	if err := repo.SetAccountBinding(ctx, a.ID, free.ID, free.Name); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAccountBinding(ctx, b.ID, free.ID, free.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RepairBindings(ctx); err != nil {
		t.Fatalf("repair: %v", err)
	}
	gotA, _ := repo.GetAccount(ctx, a.ID)
	gotB, _ := repo.GetAccount(ctx, b.ID)
	gotP, _ := repo.GetPhone(ctx, free.ID)
	if gotA.BoundPhoneID != 0 || gotA.BoundPhone != "" {
		t.Errorf("stale claimant not cleared: %d/%q", gotA.BoundPhoneID, gotA.BoundPhone)
	}
	if gotB.BoundPhoneID != free.ID || gotP.AccountID != b.ID {
		t.Errorf("owner binding wrong: account=%d phoneOwner=%d", gotB.BoundPhoneID, gotP.AccountID)
	}
}

func TestManualLoginReadyWithoutCode(t *testing.T) {
	repo, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	svc := service.New(repo, driver.MockPhone{}, immediateSms{code: "123456"}, manualLogin{})
	ctx := context.Background()

	if err := svc.ImportAccount(ctx, "", "30001", "人工号", 0); err != nil {
		t.Fatal(err)
	}
	accounts, _ := svc.Accounts(ctx)
	id := accounts[0].ID
	if err := svc.StartLogin(ctx, id, model.LoginOptions{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, "manual ready", func() bool {
		a, err := repo.GetAccount(ctx, id)
		return err == nil && a.Login == model.LoginReady && a.VerifyCode == ""
	})
	sms, _ := repo.ListSms(ctx)
	if len(sms) != 0 {
		t.Errorf("manual login inserted SMS rows: %+v", sms)
	}
	if tasks := svc.Tasks(); len(tasks) != 0 {
		t.Errorf("tasks left: %+v", tasks)
	}

	a, err := svc.ConfirmLogin(ctx, id, true)
	if err != nil {
		t.Fatalf("confirm success: %v", err)
	}
	if a.Status != model.AccountOnline || a.Login != model.LoginDone {
		t.Errorf("after confirm: status=%s login=%s", a.Status, a.Login)
	}

	// A second manual attempt can be failed from ready without a code.
	if err := svc.StartLogin(ctx, id, model.LoginOptions{}); err != nil {
		t.Fatalf("restart: %v", err)
	}
	waitFor(t, "manual ready again", func() bool {
		a, err := repo.GetAccount(ctx, id)
		return err == nil && a.Login == model.LoginReady
	})
	a, err = svc.ConfirmLogin(ctx, id, false)
	if err != nil {
		t.Fatalf("confirm failure: %v", err)
	}
	if a.Status != model.AccountAbnormal || a.Login != model.LoginFail || a.VerifyCode != "" {
		t.Errorf("after failure: status=%s login=%s code=%q", a.Status, a.Login, a.VerifyCode)
	}
	if sms, _ := repo.ListSms(ctx); len(sms) != 0 {
		t.Errorf("manual flow inserted SMS rows: %+v", sms)
	}
}

func TestSyncPhones(t *testing.T) {
	repo, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	runner := &syncPhone{devices: []driver.DeviceInfo{{
		ExternalID: "USB-1", Name: "QQ-01", Region: "桌面", Image: "Android 13",
		State: driver.DeviceOnline,
	}}}
	svc := service.New(repo, runner, immediateSms{}, manualLogin{})
	ctx := context.Background()

	res, err := svc.SyncPhones(ctx)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Added != 1 || res.Updated != 0 || res.Total != 1 {
		t.Fatalf("first sync = %+v", res)
	}
	phones, _ := svc.Phones(ctx)
	if len(phones) != 1 || phones[0].ExternalID != "USB-1" || phones[0].State != model.PhoneRunning {
		t.Fatalf("phones = %+v", phones)
	}
	phoneID := phones[0].ID

	if err := svc.ImportAccount(ctx, "", "40001", "", phoneID); err != nil {
		t.Fatal(err)
	}
	accounts, _ := svc.Accounts(ctx)
	accID := accounts[0].ID

	// A repeated sync updates instead of duplicating, and a provider rename
	// follows into the account display name.
	runner.devices[0].Name = "QQ-Renamed"
	res, err = svc.SyncPhones(ctx)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if res.Added != 0 || res.Updated != 1 {
		t.Fatalf("second sync = %+v", res)
	}
	phones, _ = svc.Phones(ctx)
	if len(phones) != 1 || phones[0].Name != "QQ-Renamed" || phones[0].AccountID != accID {
		t.Fatalf("phone after rename = %+v", phones)
	}
	a, _ := repo.GetAccount(ctx, accID)
	if a.BoundPhone != "QQ-Renamed" {
		t.Errorf("account display name = %q, want QQ-Renamed", a.BoundPhone)
	}
	if phones[0].State != model.PhoneRunning {
		t.Errorf("bound online phone state = %s, want running", phones[0].State)
	}

	// A deliberate stop survives a sync that still sees the device online.
	if err := svc.SetPhoneState(ctx, phoneID, "stop"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SyncPhones(ctx); err != nil {
		t.Fatal(err)
	}
	phones, _ = svc.Phones(ctx)
	if phones[0].State != model.PhoneStopped {
		t.Errorf("state after sync = %s, want stopped", phones[0].State)
	}

	// An offline device becomes an error but keeps its binding.
	runner.devices[0].State = driver.DeviceOffline
	if _, err := svc.SyncPhones(ctx); err != nil {
		t.Fatal(err)
	}
	phones, _ = svc.Phones(ctx)
	if phones[0].State != model.PhoneError || phones[0].AccountID != accID {
		t.Errorf("offline phone = %+v", phones[0])
	}
}

func TestSyncPhonesUnsupported(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.SyncPhones(context.Background()); !errors.Is(err, driver.ErrProvider) {
		t.Fatalf("err = %v, want ErrProvider", err)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
