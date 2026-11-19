package host_test

import (
	"bytes"
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cloud-phone/internal/backend"
	"cloud-phone/internal/host"
	"cloud-phone/internal/model"
)

func TestHostServesSharedData(t *testing.T) {
	inst, err := host.Start(host.Options{DBPath: ":memory:", Seed: true, Demo: true})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}
	defer inst.Close()

	client, err := backend.NewRemote(inst.URL, inst.Token)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	snap, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Accounts) == 0 || len(snap.Phones) == 0 {
		t.Fatal("seed is missing")
	}
	if snap.Settings.LoginMethod != model.LoginMethodPassword {
		t.Errorf("default login method = %q", snap.Settings.LoginMethod)
	}

	var free model.CloudPhone
	for _, p := range snap.Phones {
		if p.AccountID == 0 {
			free = p
			break
		}
	}
	if free.ID == 0 {
		t.Fatal("no free phone in seed")
	}
	if err := client.ImportAccount(ctx, "集成测试", "90001", "集成", free.ID); err != nil {
		t.Fatalf("import account: %v", err)
	}
	snap, _ = client.Snapshot(ctx)
	var acc model.Account
	for _, a := range snap.Accounts {
		if a.Number == "90001" {
			acc = a
		}
	}
	if acc.ID == 0 {
		t.Fatal("imported account missing")
	}
	if acc.BoundPhoneID != free.ID || acc.BoundPhone != free.Name {
		t.Errorf("binding = %d/%q, want %d/%q", acc.BoundPhoneID, acc.BoundPhone, free.ID, free.Name)
	}
	found := false
	for _, p := range snap.Phones {
		if p.ID == free.ID {
			found = true
			if p.AccountID != acc.ID {
				t.Errorf("phone owner = %d, want %d", p.AccountID, acc.ID)
			}
		}
	}
	if !found {
		t.Fatal("phone disappeared")
	}

	// Settings round-trip; pointing the provider at the demo gateway's
	// explicit URL exercises the configured-provider path too.
	want := model.Settings{
		ProviderBaseURL: inst.Demo.URL,
		ProviderToken:   inst.Demo.Token,
		LoginMethod:     model.LoginMethodSMS,
		AutoFill:        false,
		AutoBind:        true,
		SaveLog:         true,
	}
	if err := client.SaveSettings(ctx, want); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	got, err := client.Settings(ctx)
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got.ProviderBaseURL != want.ProviderBaseURL || got.LoginMethod != want.LoginMethod || got.SaveLog != want.SaveLog {
		t.Errorf("settings = %+v, want %+v", got, want)
	}

	// Screenshot through HTTP.
	data, err := client.Screenshot(ctx, acc.ID)
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("screenshot is not a PNG: %v", err)
	}
	if cfg.Width != 480 || cfg.Height != 800 {
		t.Errorf("screenshot = %dx%d, want 480x800", cfg.Width, cfg.Height)
	}

	// CSV export carries the BOM and one header row.
	csvData, count, err := client.AccountsCSV(ctx, []int64{acc.ID})
	if err != nil {
		t.Fatalf("csv: %v", err)
	}
	if count != 1 {
		t.Errorf("csv count = %d, want 1", count)
	}
	if len(csvData) < 3 || csvData[0] != 0xEF || csvData[1] != 0xBB || csvData[2] != 0xBF {
		t.Error("csv missing UTF-8 BOM")
	}
	if !strings.Contains(string(csvData), "90001") {
		t.Error("csv missing the exported account")
	}

	// SSE pushes a change for a mutation made by another client.
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	changes := make(chan model.Change, 16)
	go func() {
		_ = client.Subscribe(subCtx, func(c model.Change) {
			select {
			case changes <- c:
			default:
			}
		})
	}()
	time.Sleep(300 * time.Millisecond)
	if err := client.ImportAccount(ctx, "", "90002", "", 0); err != nil {
		t.Fatalf("second import: %v", err)
	}
	deadline := time.After(4 * time.Second)
sseLoop:
	for {
		select {
		case c := <-changes:
			if c.Kind == "accounts" {
				break sseLoop
			}
		case <-deadline:
			t.Fatal("no accounts change arrived over SSE")
		}
	}

	// The wrong token is rejected.
	bad, err := backend.NewRemote(inst.URL, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bad.Snapshot(ctx); err == nil {
		t.Error("wrong token should fail")
	}
}

func TestHostSyncPhonesAndManualLogin(t *testing.T) {
	const gatewayToken = "gw-secret"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+gatewayToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/phones":
			_, _ = w.Write([]byte(`{"phones":[{
				"external_id":"USB-IT-1","name":"集成设备","region":"桌面",
				"image":"Android 13","state":"online","ip":""
			}]}`))
		case "/v1/login/execute":
			_, _ = w.Write([]byte(`{"ok":true,"manual":true,"message":"请使用 scrcpy"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	inst, err := host.Start(host.Options{DBPath: ":memory:", Demo: false})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}
	defer inst.Close()
	client, err := backend.NewRemote(inst.URL, inst.Token)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := client.SaveSettings(ctx, model.Settings{
		ProviderBaseURL: provider.URL, ProviderToken: gatewayToken,
		LoginMethod: model.LoginMethodPassword,
	}); err != nil {
		t.Fatal(err)
	}

	res, err := client.SyncPhones(ctx)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Added != 1 || res.Total != 1 {
		t.Fatalf("sync = %+v", res)
	}
	res, err = client.SyncPhones(ctx)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if res.Added != 0 || res.Updated != 1 {
		t.Fatalf("second sync = %+v", res)
	}

	snap, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Phones) != 1 || snap.Phones[0].ExternalID != "USB-IT-1" {
		t.Fatalf("phones = %+v", snap.Phones)
	}
	phone := snap.Phones[0]
	if err := client.ImportAccount(ctx, "", "91001", "集成人工", phone.ID); err != nil {
		t.Fatalf("import: %v", err)
	}
	snap, _ = client.Snapshot(ctx)
	var account model.Account
	for _, a := range snap.Accounts {
		if a.Number == "91001" {
			account = a
		}
	}
	if account.ID == 0 {
		t.Fatal("imported account missing")
	}
	if err := client.StartLogin(ctx, account.ID, model.LoginOptions{}); err != nil {
		t.Fatalf("start login: %v", err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for {
		snap, err = client.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range snap.Accounts {
			if a.ID == account.ID && a.Login == model.LoginReady {
				if a.VerifyCode != "" {
					t.Fatalf("manual login has code %q", a.VerifyCode)
				}
				goto ready
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("manual login did not become ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
ready:
	if len(snap.Sms) != 0 {
		t.Errorf("manual login created SMS rows: %+v", snap.Sms)
	}
	if err := client.ConfirmLogin(ctx, account.ID, true); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	snap, _ = client.Snapshot(ctx)
	for _, a := range snap.Accounts {
		if a.ID == account.ID {
			if a.Status != model.AccountOnline || a.Login != model.LoginDone {
				t.Errorf("after confirm: status=%s login=%s", a.Status, a.Login)
			}
		}
	}
}
