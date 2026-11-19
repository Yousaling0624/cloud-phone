package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"image/png"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cloud-phone/internal/backend"
	"cloud-phone/internal/clientconfig"
	"cloud-phone/internal/driver"
	"cloud-phone/internal/model"
	"cloud-phone/internal/service"
	"cloud-phone/internal/store"
)

// codeSms answers immediately, so tests do not wait for the mock's delay.
type codeSms struct{ code string }

func (s codeSms) WaitCode(ctx context.Context, phone string) (string, error) {
	return s.code, nil
}

// gatedSms blocks until the test releases it, to observe the waiting state.
type gatedSms struct{ release chan struct{} }

func (s gatedSms) WaitCode(ctx context.Context, phone string) (string, error) {
	select {
	case <-s.release:
		return "654321", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

type testLogin struct{ sms driver.SmsDriver }

func (l testLogin) Execute(ctx context.Context, req driver.LoginRequest) (driver.LoginResult, error) {
	code, err := l.sms.WaitCode(ctx, req.Account.VerifyPhone)
	if err != nil {
		return driver.LoginResult{}, err
	}
	return driver.LoginResult{OK: true, Code: code, Message: "登录成功"}, nil
}

// failLogin reports the rule failure the console shows as 登扣失败.
type failLogin struct{}

func (failLogin) Execute(ctx context.Context, req driver.LoginRequest) (driver.LoginResult, error) {
	return driver.LoginResult{OK: false, Message: "执行规则失败: " + req.Rule}, nil
}

func newAPI(t *testing.T, sms driver.SmsDriver) (*API, store.Repository, *service.Service) {
	return newAPIWith(t, sms, testLogin{sms: sms})
}

func newAPIWith(t *testing.T, sms driver.SmsDriver, login driver.LoginDriver) (*API, store.Repository, *service.Service) {
	t.Helper()
	repo, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	svc := service.New(repo, driver.MockPhone{}, sms, login)
	t.Cleanup(svc.Close)
	a := New()
	AttachBackend(a, backend.NewLocal(svc))
	return a, repo, svc
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

func TestSnapshotSeeded(t *testing.T) {
	a, repo, _ := newAPI(t, codeSms{code: "123456"})
	ctx := context.Background()
	if err := repo.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	s, err := a.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(s.Accounts) == 0 || len(s.Phones) == 0 || len(s.Sms) == 0 || len(s.Users) == 0 {
		t.Fatalf("seeded snapshot is incomplete: %+v", s)
	}
	if s.Wallet.Balance != 56.40 {
		t.Errorf("balance = %v, want 56.40", s.Wallet.Balance)
	}
	if s.Now.IsZero() {
		t.Error("Now is zero")
	}
	if s.Tasks == nil || s.Ledger == nil {
		t.Error("empty lists should be empty slices, not nil, to keep JSON arrays")
	}
	if s.Settings.LoginMethod != model.LoginMethodPassword {
		t.Errorf("settings default method = %q", s.Settings.LoginMethod)
	}
}

func TestAccountLifecycle(t *testing.T) {
	a, repo, svc := newAPI(t, codeSms{code: "123456"})
	ctx := context.Background()
	if err := repo.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	phones, _ := svc.Phones(ctx)
	var free []model.CloudPhone
	for _, p := range phones {
		if p.AccountID == 0 {
			free = append(free, p)
		}
	}
	if len(free) < 2 {
		t.Fatal("seed needs two free phones")
	}
	first, second := free[0], free[1]

	if err := a.ImportAccount(ctx, "备注A", "30001", "昵称A", first.ID); err != nil {
		t.Fatalf("import: %v", err)
	}
	if err := a.ImportAccount(ctx, "重复", "30001", "", 0); err == nil {
		t.Error("duplicate account should be rejected")
	}
	if err := a.ImportAccount(ctx, "", "  ", "", 0); err == nil {
		t.Error("empty account should be rejected")
	}

	accounts, err := svc.Accounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	for _, acc := range accounts {
		if acc.Number == "30001" {
			id = acc.ID
		}
	}
	if id == 0 {
		t.Fatal("imported account not found")
	}

	if err := a.UpdateAccount(ctx, id, "新备注", "新昵称", second.ID); err != nil {
		t.Fatalf("update: %v", err)
	}
	acc, err := repo.GetAccount(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Remark != "新备注" || acc.Display != "新昵称" || acc.BoundPhoneID != second.ID || acc.BoundPhone != second.Name {
		t.Errorf("update did not persist: %+v", acc)
	}
	oldPhone, _ := repo.GetPhone(ctx, first.ID)
	if oldPhone.AccountID == id {
		t.Error("old phone still points at the account after rebinding")
	}

	if err := a.ImportAccount(ctx, "B", "30002", "", 0); err != nil {
		t.Fatal(err)
	}
	accounts, _ = svc.Accounts(ctx)
	var secondID int64
	for _, x := range accounts {
		if x.Number == "30002" {
			secondID = x.ID
		}
	}
	if err := a.DeleteAccounts(ctx, []int64{id, secondID}); err != nil {
		t.Fatalf("delete selected: %v", err)
	}
	accounts, _ = svc.Accounts(ctx)
	for _, x := range accounts {
		if x.ID == id || x.ID == secondID {
			t.Errorf("account %d left after batch delete", x.ID)
		}
	}
}

func TestImportCSV(t *testing.T) {
	a, _, _ := newAPI(t, codeSms{code: "123456"})
	ctx := context.Background()
	res, err := a.ImportCSV(ctx, "# 注释\n备注A,40001\n40002\n备注A,40001\n")
	if err != nil {
		t.Fatalf("import csv: %v", err)
	}
	if res.Added != 2 {
		t.Errorf("added = %d, want 2", res.Added)
	}
	if len(res.Errors) != 0 {
		t.Errorf("unexpected errors: %v", res.Errors)
	}
}

func TestSetUserEnabledPersists(t *testing.T) {
	a, repo, _ := newAPI(t, codeSms{code: "123456"})
	ctx := context.Background()
	if err := repo.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	s, _ := a.Snapshot(ctx)
	if len(s.Users) == 0 {
		t.Fatal("no seeded users")
	}
	user := s.Users[0]
	want := !user.Enabled
	if err := a.SetUserEnabled(ctx, user.ID, want); err != nil {
		t.Fatalf("set enabled: %v", err)
	}
	s, _ = a.Snapshot(ctx)
	found := false
	for _, u := range s.Users {
		if u.ID == user.ID {
			found = true
			if u.Enabled != want {
				t.Errorf("enabled = %v, want %v", u.Enabled, want)
			}
		}
	}
	if !found {
		t.Fatal("user disappeared")
	}
	if err := a.SetUserEnabled(ctx, 999999, true); err == nil {
		t.Error("unknown user should fail")
	}
}

func TestWallet(t *testing.T) {
	a, repo, _ := newAPI(t, codeSms{code: "123456"})
	ctx := context.Background()
	if err := repo.SetBalance(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if err := a.TopUp(ctx, 5); err != nil {
		t.Fatalf("top up: %v", err)
	}
	s, _ := a.Snapshot(ctx)
	if s.Wallet.Balance != 15 {
		t.Errorf("balance after top up = %v, want 15", s.Wallet.Balance)
	}
	if err := a.Charge(ctx, 20, "超出余额"); err == nil {
		t.Error("charge over balance should fail")
	}
	if err := a.Charge(ctx, 4.5, "云机开机扣费"); err != nil {
		t.Fatalf("charge: %v", err)
	}
	s, _ = a.Snapshot(ctx)
	if s.Wallet.Balance != 10.5 {
		t.Errorf("balance after charge = %v, want 10.5", s.Wallet.Balance)
	}
	if len(s.Ledger) != 2 {
		t.Errorf("ledger entries = %d, want 2", len(s.Ledger))
	}
}

func TestLoginFlowAndSms(t *testing.T) {
	gated := gatedSms{release: make(chan struct{})}
	a, _, svc := newAPI(t, gated)
	ctx := context.Background()

	if err := a.ImportAccount(ctx, "登扣号", "50001", "小号", 0); err != nil {
		t.Fatal(err)
	}
	accounts, _ := svc.Accounts(ctx)
	if len(accounts) != 1 {
		t.Fatalf("accounts = %d, want 1", len(accounts))
	}
	id := accounts[0].ID

	if err := a.StartLogin(ctx, id, model.LoginOptions{}); err != nil {
		t.Fatalf("start login: %v", err)
	}
	waitFor(t, "waiting state", func() bool {
		acc, err := svc.Repo().GetAccount(ctx, id)
		return err == nil && acc.Login == model.LoginWaiting
	})
	if err := a.StartLogin(ctx, id, model.LoginOptions{}); err == nil {
		t.Error("a second login while waiting should be rejected")
	}
	if tasks := svc.Tasks(); len(tasks) != 1 {
		t.Errorf("running tasks = %d, want 1", len(tasks))
	}

	close(gated.release)
	waitFor(t, "ready state", func() bool {
		acc, err := svc.Repo().GetAccount(ctx, id)
		return err == nil && acc.Login == model.LoginReady && acc.VerifyCode == "654321"
	})
	waitFor(t, "task finished", func() bool { return len(svc.Tasks()) == 0 })

	s, _ := a.Snapshot(ctx)
	var smsID int64
	for _, m := range s.Sms {
		if m.Code == "654321" {
			smsID = m.ID
			if m.Used {
				t.Error("new code should be unused")
			}
		}
	}
	if smsID == 0 {
		t.Fatal("login code was not written to the inbox")
	}

	if err := a.ConfirmLogin(ctx, id, true); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	acc, _ := svc.Repo().GetAccount(ctx, id)
	if acc.Status != model.AccountOnline || acc.Login != model.LoginDone {
		t.Errorf("after confirmation: status=%s login=%s", acc.Status, acc.Login)
	}
	s, _ = a.Snapshot(ctx)
	for _, m := range s.Sms {
		if m.Code == "654321" && !m.Used {
			t.Error("confirmed code should be marked used")
		}
	}
}

func TestFailedLoginMarksAccountAbnormal(t *testing.T) {
	a, _, svc := newAPIWith(t, codeSms{code: "123456"}, failLogin{})
	ctx := context.Background()
	if err := a.ImportAccount(ctx, "", "50002", "", 0); err != nil {
		t.Fatal(err)
	}
	accounts, _ := svc.Accounts(ctx)
	id := accounts[0].ID
	if err := a.StartLogin(ctx, id, model.LoginOptions{Method: model.LoginMethodSMS}); err != nil {
		t.Fatalf("start login: %v", err)
	}
	waitFor(t, "failed state", func() bool {
		acc, err := svc.Repo().GetAccount(ctx, id)
		return err == nil && acc.Login == model.LoginFail
	})
	acc, _ := svc.Repo().GetAccount(ctx, id)
	if acc.Status != model.AccountAbnormal {
		t.Errorf("status = %s, want abnormal", acc.Status)
	}
	waitFor(t, "task finished", func() bool { return len(svc.Tasks()) == 0 })
}

func TestScreenshot(t *testing.T) {
	a, repo, svc := newAPI(t, codeSms{code: "123456"})
	ctx := context.Background()
	if err := repo.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	phones, _ := svc.Phones(ctx)
	var free model.CloudPhone
	for _, p := range phones {
		if p.AccountID == 0 {
			free = p
			break
		}
	}
	if free.ID == 0 {
		t.Fatal("no free phone")
	}
	if err := a.ImportAccount(ctx, "", "60001", "", free.ID); err != nil {
		t.Fatal(err)
	}
	accs, _ := svc.Accounts(ctx)
	var id int64
	for _, acc := range accs {
		if acc.Number == "60001" {
			id = acc.ID
		}
	}
	url, err := a.AccountScreenshot(ctx, id)
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(url, prefix) {
		t.Fatalf("screenshot is not a PNG data URL: %.40s", url)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, prefix))
	if err != nil {
		t.Fatalf("decode screenshot: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		t.Errorf("screenshot is not a valid PNG: %v", err)
	}

	if err := a.ImportAccount(ctx, "", "60002", "", 0); err != nil {
		t.Fatal(err)
	}
	accs, _ = svc.Accounts(ctx)
	for _, acc := range accs {
		if acc.Number == "60002" {
			if _, err := a.AccountScreenshot(ctx, acc.ID); err == nil {
				t.Error("unbound account should have no screenshot")
			}
		}
	}
}

func TestSettingsPersist(t *testing.T) {
	a, _, _ := newAPI(t, codeSms{code: "123456"})
	ctx := context.Background()
	want := model.Settings{
		ProviderBaseURL: "http://127.0.0.1:8790",
		ProviderToken:   "demo-token",
		LoginMethod:     model.LoginMethodSMS,
		AutoFill:        false,
		AutoBind:        true,
		SaveLog:         true,
	}
	if err := a.SaveSettings(ctx, want); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	s, _ := a.Snapshot(ctx)
	if s.Settings.ProviderBaseURL != want.ProviderBaseURL || s.Settings.ProviderToken != want.ProviderToken ||
		s.Settings.LoginMethod != want.LoginMethod || s.Settings.SaveLog != want.SaveLog {
		t.Errorf("settings not persisted: %+v", s.Settings)
	}
	if err := a.SaveSettings(ctx, model.Settings{ProviderBaseURL: "ftp://bad"}); err == nil {
		t.Error("invalid provider URL should be rejected")
	}
}

func TestSaveConnection(t *testing.T) {
	a := New()
	path := filepath.Join(t.TempDir(), "client.json")
	Configure(a, Config{ConfigPath: path})
	ctx := context.Background()

	if err := a.SaveConnection(ctx, " http://127.0.0.1:8787/ ", "secret"); err != nil {
		t.Fatalf("save connection: %v", err)
	}
	cfg, err := clientconfig.Load(path)
	if err != nil {
		t.Fatalf("load connection: %v", err)
	}
	if cfg.ServerURL != "http://127.0.0.1:8787" || cfg.Token != "secret" {
		t.Errorf("saved config = %+v", cfg)
	}
	if info, _ := a.Connection(ctx); info.Mode != "remote" || info.ServerURL != cfg.ServerURL {
		t.Errorf("connection info = %+v", info)
	}

	// The page never sees the saved token; saving with a blank field must
	// keep it and switch back to embedded mode without erasing the token.
	if err := a.SaveConnection(ctx, "", ""); err != nil {
		t.Fatalf("save blank connection: %v", err)
	}
	cfg, _ = clientconfig.Load(path)
	if cfg.ServerURL != "" || cfg.Token != "secret" {
		t.Errorf("blank token should be preserved, got %+v", cfg)
	}
	if info, _ := a.Connection(ctx); info.Mode != "embedded" {
		t.Errorf("mode = %q, want embedded", info.Mode)
	}

	if err := a.SaveConnection(ctx, "ftp://bad", "x"); err == nil {
		t.Error("invalid server URL should be rejected")
	}
	if err := a.SaveConnection(ctx, "", "new-token"); err != nil {
		t.Fatalf("replace token: %v", err)
	}
	cfg, _ = clientconfig.Load(path)
	if cfg.Token != "new-token" {
		t.Errorf("token = %q, want new-token", cfg.Token)
	}
}

func TestAccountsCSV(t *testing.T) {
	_, _, svc := newAPI(t, codeSms{code: "123456"})
	ctx := context.Background()
	if err := svc.ImportAccount(ctx, "带,逗号", "10001", "昵称", 0); err != nil {
		t.Fatal(err)
	}
	accs, _ := svc.Accounts(ctx)
	data, count, err := svc.AccountsCSV(ctx, []int64{accs[0].ID})
	if err != nil {
		t.Fatalf("csv: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	s := string(data)
	if len(data) < 3 || data[0] != 0xEF || data[1] != 0xBB || data[2] != 0xBF {
		t.Error("CSV should start with a UTF-8 BOM for Excel")
	}
	if !strings.Contains(s, "\"带,逗号\"") {
		t.Errorf("comma-bearing field should be quoted, got:\n%s", s)
	}
	if !strings.Contains(s, "\r\n") {
		t.Error("CSV should use CRLF line endings")
	}
	if !strings.Contains(s, "备注,账号") {
		t.Errorf("missing header, got:\n%s", s)
	}
}
