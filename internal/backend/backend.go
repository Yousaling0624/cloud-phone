// Package backend is the seam between the bound API surface and where the
// business logic actually lives. The local implementation calls the service
// in-process; the remote implementation talks HTTP to a standalone server.
package backend

import (
	"context"

	"cloud-phone/internal/model"
)

// Backend is every operation the console can perform.
type Backend interface {
	Snapshot(ctx context.Context) (model.Snapshot, error)

	ImportAccount(ctx context.Context, remark, number, display string, phoneID int64) error
	UpdateAccount(ctx context.Context, id int64, remark, display string, phoneID int64) error
	DeleteAccount(ctx context.Context, id int64) error
	DeleteAccounts(ctx context.Context, ids []int64) error
	ImportCSV(ctx context.Context, text string) (model.ImportResult, error)
	AccountsCSV(ctx context.Context, ids []int64) (data []byte, count int, err error)

	StartLogin(ctx context.Context, id int64, opts model.LoginOptions) error
	ConfirmLogin(ctx context.Context, id int64, ok bool) error

	SetPhoneState(ctx context.Context, id int64, action string) error
	SyncPhones(ctx context.Context) (model.PhoneSyncResult, error)
	SetUserEnabled(ctx context.Context, id int64, enabled bool) error

	TopUp(ctx context.Context, amount float64) error
	Charge(ctx context.Context, amount float64, reason string) error

	Screenshot(ctx context.Context, accountID int64) ([]byte, error)

	Settings(ctx context.Context) (model.Settings, error)
	SaveSettings(ctx context.Context, s model.Settings) error
}

// Subscriber is implemented by backends that can push changes from the
// server (the remote one, over SSE). The desktop API uses it to emit
// stateChanged events; the local backend notifies through the service.
type Subscriber interface {
	Subscribe(ctx context.Context, onChange func(model.Change)) error
}
