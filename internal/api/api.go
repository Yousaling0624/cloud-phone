// Package api is the only surface the web frontend can call. It delegates
// to a backend (in-process or remote), reads the whole state in one Snapshot
// for the page to render, and announces asynchronous changes (a login
// finishing, a code arriving) with the stateChanged event.
package api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cloud-phone/internal/backend"
	"cloud-phone/internal/clientconfig"
	"cloud-phone/internal/model"
	"cloud-phone/internal/service"

	"github.com/egoist/mygo"
)

// StateChanged is emitted whenever the page's snapshot is stale.
var StateChanged = mygo.NewEvent[model.Change]("stateChanged")

// EmitChange broadcasts a change to every open window. The remote backend's
// SSE subscription calls it; the API also calls it after its own mutations
// for immediate feedback.
func EmitChange(c model.Change) {
	_ = StateChanged.Broadcast(c)
}

// ExportResult is where an export was written and how many rows it holds.
type ExportResult struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// ConnectionInfo describes how this desktop client reaches its server. It
// is local to this machine, unlike the shared model.Settings.
type ConnectionInfo struct {
	Mode         string `json:"mode"` // embedded or remote
	ServerURL    string `json:"server_url"`
	ConfigPath   string `json:"config_path"`
	StartupError string `json:"startup_error"`
}

// Config is everything main tells the API about its environment at startup.
type Config struct {
	ConfigPath string
	Connection ConnectionInfo
}

// API is bound with mygo.Bind. `mygo generate` turns its methods into the
// typed client in src/mygo.ts.
type API struct {
	mu         sync.RWMutex
	be         backend.Backend
	startupErr error
	connection ConnectionInfo
	configPath string
}

// New returns an API with no backend yet.
func New() *API { return &API{} }

// Configure records where client.json lives and how the app connected.
// A function, not a method, so it stays out of the generated client.
func Configure(a *API, cfg Config) {
	a.mu.Lock()
	a.configPath = cfg.ConfigPath
	a.connection = cfg.Connection
	a.mu.Unlock()
}

// AttachBackend gives the API its data source. A function so the generated
// frontend client does not expose it.
func AttachBackend(a *API, be backend.Backend) {
	a.mu.Lock()
	a.be = be
	a.startupErr = nil
	a.connection.StartupError = ""
	a.mu.Unlock()
}

// SetStartupError records why no backend could be built, so the error page
// can explain it and offer the connection settings.
func SetStartupError(a *API, err error) {
	a.mu.Lock()
	a.startupErr = err
	if err != nil {
		a.connection.StartupError = err.Error()
	} else {
		a.connection.StartupError = ""
	}
	a.mu.Unlock()
}

func (a *API) backend() (backend.Backend, error) {
	a.mu.RLock()
	be, startupErr := a.be, a.startupErr
	a.mu.RUnlock()
	if startupErr != nil {
		return nil, fmt.Errorf("服务连接失败：%w", startupErr)
	}
	if be == nil {
		return nil, errors.New("数据服务尚未就绪，请稍后重试")
	}
	return be, nil
}

// Snapshot returns the whole state the console renders.
func (a *API) Snapshot(ctx context.Context) (model.Snapshot, error) {
	be, err := a.backend()
	if err != nil {
		return model.Snapshot{}, err
	}
	return be.Snapshot(ctx)
}

// ImportAccount adds one account, optionally bound to a cloud phone.
func (a *API) ImportAccount(ctx context.Context, remark, number, display string, boundPhoneID int64) error {
	be, err := a.backend()
	if err != nil {
		return err
	}
	if err := be.ImportAccount(ctx, remark, number, display, boundPhoneID); err != nil {
		return err
	}
	EmitChange(model.Change{Kind: "accounts"})
	if boundPhoneID != 0 {
		EmitChange(model.Change{Kind: "phones"})
	}
	return nil
}

// UpdateAccount saves the editable fields and the phone binding.
func (a *API) UpdateAccount(ctx context.Context, id int64, remark, display string, boundPhoneID int64) error {
	be, err := a.backend()
	if err != nil {
		return err
	}
	if err := be.UpdateAccount(ctx, id, remark, display, boundPhoneID); err != nil {
		return err
	}
	EmitChange(model.Change{Kind: "accounts", AccountID: id})
	EmitChange(model.Change{Kind: "phones"})
	return nil
}

// DeleteAccount removes one account.
func (a *API) DeleteAccount(ctx context.Context, id int64) error {
	be, err := a.backend()
	if err != nil {
		return err
	}
	if err := be.DeleteAccount(ctx, id); err != nil {
		return err
	}
	EmitChange(model.Change{Kind: "accounts", AccountID: id})
	EmitChange(model.Change{Kind: "phones"})
	EmitChange(model.Change{Kind: "sims"})
	return nil
}

// DeleteAccounts removes the selected accounts, reporting every id that
// could not be removed rather than stopping at the first one.
func (a *API) DeleteAccounts(ctx context.Context, ids []int64) error {
	be, err := a.backend()
	if err != nil {
		return err
	}
	if err := be.DeleteAccounts(ctx, ids); err != nil {
		return err
	}
	EmitChange(model.Change{Kind: "accounts"})
	EmitChange(model.Change{Kind: "phones"})
	EmitChange(model.Change{Kind: "sims"})
	return nil
}

// ImportCSV imports "备注,账号" lines.
func (a *API) ImportCSV(ctx context.Context, text string) (model.ImportResult, error) {
	be, err := a.backend()
	if err != nil {
		return model.ImportResult{}, err
	}
	res, err := be.ImportCSV(ctx, text)
	if err != nil {
		return model.ImportResult{}, err
	}
	if res.Added > 0 {
		EmitChange(model.Change{Kind: "accounts"})
	}
	if res.Errors == nil {
		res.Errors = []string{}
	}
	return res, nil
}

// StartLogin begins 登扣 for an account. The login keeps running on the
// server after this call returns; progress arrives as stateChanged events.
func (a *API) StartLogin(ctx context.Context, id int64, opts model.LoginOptions) error {
	be, err := a.backend()
	if err != nil {
		return err
	}
	if err := be.StartLogin(ctx, id, opts); err != nil {
		return err
	}
	EmitChange(model.Change{Kind: "accounts", AccountID: id})
	EmitChange(model.Change{Kind: "tasks", AccountID: id})
	return nil
}

// ConfirmLogin records whether the login succeeded.
func (a *API) ConfirmLogin(ctx context.Context, id int64, ok bool) error {
	be, err := a.backend()
	if err != nil {
		return err
	}
	if err := be.ConfirmLogin(ctx, id, ok); err != nil {
		return err
	}
	EmitChange(model.Change{Kind: "accounts", AccountID: id})
	EmitChange(model.Change{Kind: "sms", AccountID: id})
	return nil
}

// SetPhoneState powers a cloud phone on, off, or restarts it.
func (a *API) SetPhoneState(ctx context.Context, id int64, action string) error {
	be, err := a.backend()
	if err != nil {
		return err
	}
	if err := be.SetPhoneState(ctx, id, action); err != nil {
		return err
	}
	EmitChange(model.Change{Kind: "phones"})
	return nil
}

// SyncPhones imports the provider's current device list into the platform.
func (a *API) SyncPhones(ctx context.Context) (model.PhoneSyncResult, error) {
	be, err := a.backend()
	if err != nil {
		return model.PhoneSyncResult{}, err
	}
	res, err := be.SyncPhones(ctx)
	if err != nil {
		return model.PhoneSyncResult{}, err
	}
	EmitChange(model.Change{Kind: "phones"})
	return res, nil
}

// SetUserEnabled turns an operator account on or off.
func (a *API) SetUserEnabled(ctx context.Context, id int64, enabled bool) error {
	be, err := a.backend()
	if err != nil {
		return err
	}
	if err := be.SetUserEnabled(ctx, id, enabled); err != nil {
		return err
	}
	EmitChange(model.Change{Kind: "users"})
	return nil
}

// TopUp adds money to the platform wallet.
func (a *API) TopUp(ctx context.Context, amount float64) error {
	be, err := a.backend()
	if err != nil {
		return err
	}
	if err := be.TopUp(ctx, amount); err != nil {
		return err
	}
	EmitChange(model.Change{Kind: "wallet"})
	return nil
}

// Charge debits the platform wallet.
func (a *API) Charge(ctx context.Context, amount float64, reason string) error {
	be, err := a.backend()
	if err != nil {
		return err
	}
	if err := be.Charge(ctx, amount, strings.TrimSpace(reason)); err != nil {
		return err
	}
	EmitChange(model.Change{Kind: "wallet"})
	return nil
}

// AccountScreenshot returns the bound phone's current screen as a data URL
// the page can put in an <img> without fetching cross-origin resources.
func (a *API) AccountScreenshot(ctx context.Context, id int64) (string, error) {
	be, err := a.backend()
	if err != nil {
		return "", err
	}
	data, err := be.Screenshot(ctx, id)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("云机未返回画面")
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data), nil
}

// Settings returns the shared platform settings.
func (a *API) Settings(ctx context.Context) (model.Settings, error) {
	be, err := a.backend()
	if err != nil {
		return model.Settings{}, err
	}
	return be.Settings(ctx)
}

// SaveSettings persists the shared platform settings.
func (a *API) SaveSettings(ctx context.Context, s model.Settings) error {
	be, err := a.backend()
	if err != nil {
		return err
	}
	if err := be.SaveSettings(ctx, s); err != nil {
		return err
	}
	EmitChange(model.Change{Kind: "settings"})
	return nil
}

// Connection returns this client's connection details.
func (a *API) Connection(ctx context.Context) (ConnectionInfo, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	info := a.connection
	info.ConfigPath = a.configPath
	return info, nil
}

// SaveConnection writes client.json. The change takes effect on the next
// launch; the page says so.
func (a *API) SaveConnection(ctx context.Context, serverURL, token string) error {
	a.mu.Lock()
	path := a.configPath
	a.mu.Unlock()
	if path == "" {
		return errors.New("未找到客户端配置文件位置")
	}
	serverURL = strings.TrimSpace(serverURL)
	token = strings.TrimSpace(token)
	if serverURL != "" {
		u, err := url.Parse(serverURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("服务端地址必须是 http:// 或 https:// 开头的完整 URL")
		}
		serverURL = strings.TrimRight(serverURL, "/")
	}
	// The token is never shown back to the page. An empty field therefore
	// means "keep the saved token", not "erase it", so opening settings and
	// pressing save cannot lock this client out.
	if token == "" {
		if prev, err := clientconfig.Load(path); err == nil {
			token = prev.Token
		}
	}
	if err := clientconfig.Save(path, clientconfig.Config{ServerURL: serverURL, Token: token}); err != nil {
		return err
	}
	a.mu.Lock()
	a.connection.ServerURL = serverURL
	if serverURL == "" {
		a.connection.Mode = "embedded"
	} else {
		a.connection.Mode = "remote"
	}
	a.mu.Unlock()
	return nil
}

// TestConnection checks a server's health endpoint without changing any
// configuration. It works even while no backend is attached.
func (a *API) TestConnection(ctx context.Context, serverURL, token string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	h, err := backend.Health(ctx, serverURL, token)
	if err != nil {
		return "", err
	}
	if h.Version == "" {
		return "连接成功", nil
	}
	return fmt.Sprintf("连接成功：%s（数据库 %s）", h.Version, h.Database), nil
}

// ExportAccountsCSV asks where to save, then writes the selected accounts
// (all of them when ids is empty) as UTF-8 CSV. An empty path means the user
// canceled.
func (a *API) ExportAccountsCSV(ctx context.Context, ids []int64) (ExportResult, error) {
	be, err := a.backend()
	if err != nil {
		return ExportResult{}, err
	}
	data, count, err := be.AccountsCSV(ctx, ids)
	if err != nil {
		return ExportResult{}, err
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      mygo.CallerWindow(ctx),
		Title:       "导出账号",
		DefaultPath: "云手机账号.csv",
		ButtonLabel: "导出",
		Filters:     []mygo.FileFilter{{Name: "CSV 文件", Extensions: []string{"csv"}}},
	})
	if err != nil {
		return ExportResult{}, err
	}
	if path == "" {
		return ExportResult{}, nil // canceled
	}
	if !strings.EqualFold(filepath.Ext(path), ".csv") {
		path += ".csv"
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return ExportResult{}, fmt.Errorf("写入导出文件: %w", err)
	}
	return ExportResult{Path: path, Count: count}, nil
}

// AccountStatusLabel is kept for compatibility; the canonical labels live in
// the service package (used by the CSV export).
func AccountStatusLabel(s model.AccountStatus) string { return service.AccountStatusLabel(s) }

// LoginStateLabel is the Chinese label of the finer login state.
func LoginStateLabel(s model.LoginState) string { return service.LoginStateLabel(s) }
