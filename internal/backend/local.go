package backend

import (
	"context"

	"cloud-phone/internal/model"
	"cloud-phone/internal/service"
)

// Local runs the backend in this process over the service layer.
type Local struct {
	svc *service.Service
}

// NewLocal wraps a service as a Backend.
func NewLocal(svc *service.Service) *Local { return &Local{svc: svc} }

func (l *Local) Snapshot(ctx context.Context) (model.Snapshot, error) {
	return l.svc.Snapshot(ctx)
}

func (l *Local) ImportAccount(ctx context.Context, remark, number, display string, phoneID int64) error {
	return l.svc.ImportAccount(ctx, remark, number, display, phoneID)
}

func (l *Local) UpdateAccount(ctx context.Context, id int64, remark, display string, phoneID int64) error {
	return l.svc.UpdateAccount(ctx, id, remark, display, phoneID)
}

func (l *Local) DeleteAccount(ctx context.Context, id int64) error {
	return l.svc.DeleteAccount(ctx, id)
}

func (l *Local) DeleteAccounts(ctx context.Context, ids []int64) error {
	return l.svc.DeleteAccounts(ctx, ids)
}

func (l *Local) ImportCSV(ctx context.Context, text string) (model.ImportResult, error) {
	return l.svc.ImportCSV(ctx, text)
}

func (l *Local) AccountsCSV(ctx context.Context, ids []int64) ([]byte, int, error) {
	return l.svc.AccountsCSV(ctx, ids)
}

func (l *Local) StartLogin(ctx context.Context, id int64, opts model.LoginOptions) error {
	return l.svc.StartLogin(ctx, id, opts)
}

func (l *Local) ConfirmLogin(ctx context.Context, id int64, ok bool) error {
	_, err := l.svc.ConfirmLogin(ctx, id, ok)
	return err
}

func (l *Local) SetPhoneState(ctx context.Context, id int64, action string) error {
	return l.svc.SetPhoneState(ctx, id, action)
}

func (l *Local) SyncPhones(ctx context.Context) (model.PhoneSyncResult, error) {
	return l.svc.SyncPhones(ctx)
}

func (l *Local) SetUserEnabled(ctx context.Context, id int64, enabled bool) error {
	return l.svc.SetUserEnabled(ctx, id, enabled)
}

func (l *Local) TopUp(ctx context.Context, amount float64) error {
	return l.svc.TopUp(ctx, amount)
}

func (l *Local) Charge(ctx context.Context, amount float64, reason string) error {
	return l.svc.Charge(ctx, amount, reason)
}

func (l *Local) Screenshot(ctx context.Context, accountID int64) ([]byte, error) {
	return l.svc.Screenshot(ctx, accountID)
}

func (l *Local) Settings(ctx context.Context) (model.Settings, error) {
	return l.svc.Settings(ctx)
}

func (l *Local) SaveSettings(ctx context.Context, s model.Settings) error {
	return l.svc.SaveSettings(ctx, s)
}
