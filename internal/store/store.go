// Package store persists the platform's data. The concrete implementation
// uses the MyGo sqlite plugin, which embeds a SQLite written in Zig: no cgo,
// no C toolchain, and it works the same from a headless server as from the
// desktop app.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud-phone/internal/model"

	"github.com/egoist/mygo/plugins/sqlite"
)

// ErrNotFound is returned when a row the caller asked for does not exist.
var ErrNotFound = errors.New("not found")

// Repository is everything the services read and write.
type Repository interface {
	Close() error
	Seed(ctx context.Context) error // fills demo rows once, if empty

	ListAccounts(ctx context.Context) ([]model.Account, error)
	GetAccount(ctx context.Context, id int64) (model.Account, error)
	FindAccountByNumber(ctx context.Context, number string) (model.Account, bool, error)
	CreateAccount(ctx context.Context, a *model.Account) error
	UpdateAccount(ctx context.Context, a model.Account) error
	UpdateAccountProfile(ctx context.Context, id int64, remark, display string) error
	UpdateAccountLogin(ctx context.Context, id int64, status model.AccountStatus, login model.LoginState, rule, code, verifyPhone string) error
	SetAccountBinding(ctx context.Context, accountID, phoneID int64, phoneName string) error
	DeleteAccount(ctx context.Context, id int64) error

	ListPhones(ctx context.Context) ([]model.CloudPhone, error)
	GetPhone(ctx context.Context, id int64) (model.CloudPhone, error)
	FindPhoneByExternalID(ctx context.Context, externalID string) (model.CloudPhone, bool, error)
	CreatePhone(ctx context.Context, p *model.CloudPhone) error
	UpdatePhone(ctx context.Context, p model.CloudPhone) error
	UpdatePhoneInfo(ctx context.Context, id int64, name, region, image string, state model.PhoneState, ip string) error
	UpdatePhoneState(ctx context.Context, id int64, state model.PhoneState) error
	SetPhoneAccount(ctx context.Context, phoneID, accountID int64) error

	ListSms(ctx context.Context) ([]model.Sms, error)
	AddSms(ctx context.Context, s *model.Sms) error
	MarkSmsUsed(ctx context.Context, phone, code string) error

	ListSims(ctx context.Context) ([]model.SimSlot, error)

	ListUsers(ctx context.Context) ([]model.User, error)
	UpdateUser(ctx context.Context, u model.User) error

	Wallet(ctx context.Context) (model.Wallet, error)
	SetBalance(ctx context.Context, v float64) error // seed and tests
	ListLedger(ctx context.Context, limit int) ([]model.LedgerEntry, error)
	AddLedger(ctx context.Context, e *model.LedgerEntry) error // seed and tests
	ApplyWalletChange(ctx context.Context, newBalance float64, e *model.LedgerEntry) error

	GetSettings(ctx context.Context) (model.Settings, error)
	SaveSettings(ctx context.Context, s model.Settings) error
}

// SQLite is the Repository backed by the MyGo sqlite plugin.
type SQLite struct {
	db *sqlite.DB
}

// Open opens (or creates) the database at path and returns it ready to use.
// The parent directory must exist.
func Open(path string) (*SQLite, error) {
	db, err := sqlite.Open(context.Background(), path, sqlite.OpenOptions{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	s := &SQLite{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// OpenMemory opens a private in-memory database, for tests.
func OpenMemory() (*SQLite, error) {
	db, err := sqlite.Open(context.Background(), ":memory:", sqlite.OpenOptions{})
	if err != nil {
		return nil, fmt.Errorf("open memory database: %w", err)
	}
	s := &SQLite{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

func (s *SQLite) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			remark TEXT, number TEXT, display TEXT, bound_phone TEXT,
			bound_phone_id INTEGER NOT NULL DEFAULT 0,
			status TEXT, login TEXT, login_rule TEXT, verify_code TEXT,
			verify_phone TEXT, linked_qq INTEGER, linked_qid TEXT,
			has_password INTEGER, updated_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS phones (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT, external_id TEXT NOT NULL DEFAULT '', region TEXT, image TEXT,
			state TEXT, account_id INTEGER, ip TEXT)`,
		`CREATE TABLE IF NOT EXISTS sms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			phone TEXT, code TEXT, content TEXT, received_at TEXT, used INTEGER)`,
		`CREATE TABLE IF NOT EXISTS sims (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			phone TEXT, carrier TEXT, bound_account INTEGER, status TEXT)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT, nickname TEXT, role TEXT, enabled INTEGER)`,
		`CREATE TABLE IF NOT EXISTS ledger (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			at TEXT, amount REAL, reason TEXT)`,
		`CREATE TABLE IF NOT EXISTS wallet (
			id INTEGER PRIMARY KEY CHECK (id = 1), balance REAL)`,
		`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY CHECK (id = 1), json TEXT NOT NULL, updated_at TEXT)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Execute(ctx, q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	// Additive migrations for databases created before these columns existed.
	if err := s.ensureColumn(ctx, "accounts", "bound_phone_id", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "phones", "external_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_phones_account ON phones(account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_phones_external ON phones(external_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sms_phone_code ON sms(phone, code)`,
		`CREATE INDEX IF NOT EXISTS idx_accounts_bound_phone ON accounts(bound_phone_id)`,
	}
	for _, q := range indexes {
		if _, err := s.db.Execute(ctx, q); err != nil {
			return fmt.Errorf("migrate index: %w", err)
		}
	}
	backfills := []string{
		`UPDATE phones SET external_id = name WHERE external_id IS NULL OR external_id = ''`,
		`UPDATE accounts SET bound_phone_id = COALESCE(
			(SELECT id FROM phones WHERE phones.name = accounts.bound_phone LIMIT 1), 0)
		 WHERE bound_phone <> '' AND bound_phone_id = 0`,
	}
	for _, q := range backfills {
		if _, err := s.db.Execute(ctx, q); err != nil {
			return fmt.Errorf("migrate backfill: %w", err)
		}
	}
	return nil
}

// ensureColumn adds a column to an existing table if it is missing. table,
// column and definition are compile-time constants.
func (s *SQLite) ensureColumn(ctx context.Context, table, column, definition string) error {
	rows, err := s.db.Query(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return fmt.Errorf("inspect %s: %w", table, err)
	}
	for _, r := range rows.Values {
		if len(r) > 1 {
			if name, _ := r[1].(string); name == column {
				return nil
			}
		}
	}
	if _, err := s.db.Execute(ctx, "ALTER TABLE "+table+" ADD COLUMN "+column+" "+definition); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}
	return nil
}

// --- accounts ---

const accountColumns = `id, remark, number, display, bound_phone, bound_phone_id,
	status, login, login_rule, verify_code, verify_phone, linked_qq, linked_qid,
	has_password, updated_at`

func (s *SQLite) ListAccounts(ctx context.Context) ([]model.Account, error) {
	rows, err := s.db.Query(ctx, `SELECT `+accountColumns+` FROM accounts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var out []model.Account
	for _, r := range rows.Values {
		out = append(out, scanAccount(r))
	}
	return out, nil
}

func scanAccount(r []any) model.Account {
	str := func(i int) string {
		if r[i] == nil {
			return ""
		}
		v, _ := r[i].(string)
		return v
	}
	boo := func(i int) bool {
		if r[i] == nil {
			return false
		}
		switch v := r[i].(type) {
		case int64:
			return v != 0
		case bool:
			return v
		}
		return false
	}
	id, _ := r[0].(int64)
	phoneID, _ := r[5].(int64)
	a := model.Account{
		ID: id, Remark: str(1), Number: str(2), Display: str(3), BoundPhone: str(4),
		BoundPhoneID: phoneID,
		Status:       model.AccountStatus(str(6)), Login: model.LoginState(str(7)),
		LoginRule: str(8), VerifyCode: str(9), VerifyPhone: str(10),
		LinkedQQ: boo(11), LinkedQID: str(12), HasPassword: boo(13),
	}
	if t := str(14); t != "" {
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			a.UpdatedAt = ts
		}
	}
	return a
}

func (s *SQLite) GetAccount(ctx context.Context, id int64) (model.Account, error) {
	rows, err := s.db.Query(ctx, `SELECT `+accountColumns+` FROM accounts WHERE id = ?`, id)
	if err != nil {
		return model.Account{}, err
	}
	if len(rows.Values) == 0 {
		return model.Account{}, fmt.Errorf("account %d: %w", id, ErrNotFound)
	}
	return scanAccount(rows.Values[0]), nil
}

func (s *SQLite) FindAccountByNumber(ctx context.Context, number string) (model.Account, bool, error) {
	rows, err := s.db.Query(ctx, `SELECT `+accountColumns+` FROM accounts WHERE number = ? LIMIT 1`, number)
	if err != nil {
		return model.Account{}, false, err
	}
	if len(rows.Values) == 0 {
		return model.Account{}, false, nil
	}
	return scanAccount(rows.Values[0]), true, nil
}

func (s *SQLite) CreateAccount(ctx context.Context, a *model.Account) error {
	a.UpdatedAt = time.Now()
	res, err := s.db.Execute(ctx, `INSERT INTO accounts
		(remark, number, display, bound_phone, bound_phone_id, status, login, login_rule,
		verify_code, verify_phone, linked_qq, linked_qid, has_password, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.Remark, a.Number, a.Display, a.BoundPhone, a.BoundPhoneID, string(a.Status), string(a.Login),
		a.LoginRule, a.VerifyCode, a.VerifyPhone, a.LinkedQQ, a.LinkedQID, a.HasPassword,
		a.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return err
	}
	a.ID = res.LastInsertID
	return nil
}

func (s *SQLite) UpdateAccount(ctx context.Context, a model.Account) error {
	a.UpdatedAt = time.Now()
	_, err := s.db.Execute(ctx, `UPDATE accounts SET remark=?, number=?, display=?,
		bound_phone=?, bound_phone_id=?, status=?, login=?, login_rule=?, verify_code=?,
		verify_phone=?, linked_qq=?, linked_qid=?, has_password=?, updated_at=? WHERE id=?`,
		a.Remark, a.Number, a.Display, a.BoundPhone, a.BoundPhoneID, string(a.Status), string(a.Login),
		a.LoginRule, a.VerifyCode, a.VerifyPhone, a.LinkedQQ, a.LinkedQID, a.HasPassword,
		a.UpdatedAt.Format(time.RFC3339), a.ID)
	return err
}

func (s *SQLite) UpdateAccountProfile(ctx context.Context, id int64, remark, display string) error {
	_, err := s.db.Execute(ctx, `UPDATE accounts SET remark=?, display=?, updated_at=? WHERE id=?`,
		remark, display, time.Now().Format(time.RFC3339), id)
	return err
}

func (s *SQLite) UpdateAccountLogin(ctx context.Context, id int64, status model.AccountStatus,
	login model.LoginState, rule, code, verifyPhone string) error {
	_, err := s.db.Execute(ctx, `UPDATE accounts SET status=?, login=?, login_rule=?,
		verify_code=?, verify_phone=?, updated_at=? WHERE id=?`,
		string(status), string(login), rule, code, verifyPhone,
		time.Now().Format(time.RFC3339), id)
	return err
}

// SetAccountBinding writes both halves of the account-phone link in one
// transaction. The service validates the target before calling it.
func (s *SQLite) SetAccountBinding(ctx context.Context, accountID, phoneID int64, phoneName string) error {
	now := time.Now().Format(time.RFC3339)
	var stmts []sqlite.Statement
	if phoneID != 0 {
		stmts = []sqlite.Statement{
			{SQL: `UPDATE phones SET account_id=0 WHERE account_id=? AND id<>?`, Args: []any{accountID, phoneID}},
			{SQL: `UPDATE phones SET account_id=? WHERE id=?`, Args: []any{accountID, phoneID}},
			{SQL: `UPDATE accounts SET bound_phone_id=?, bound_phone=?, updated_at=? WHERE id=?`,
				Args: []any{phoneID, phoneName, now, accountID}},
		}
	} else {
		stmts = []sqlite.Statement{
			{SQL: `UPDATE phones SET account_id=0 WHERE account_id=?`, Args: []any{accountID}},
			{SQL: `UPDATE accounts SET bound_phone_id=0, bound_phone='', updated_at=? WHERE id=?`,
				Args: []any{now, accountID}},
		}
	}
	_, err := s.db.Transaction(ctx, stmts)
	return err
}

// DeleteAccount deletes the account and clears every reference to it in one
// transaction, so no orphan phone or SIM links remain.
func (s *SQLite) DeleteAccount(ctx context.Context, id int64) error {
	_, err := s.db.Transaction(ctx, []sqlite.Statement{
		{SQL: `UPDATE phones SET account_id=0 WHERE account_id=?`, Args: []any{id}},
		{SQL: `UPDATE sims SET bound_account=0, status='空闲' WHERE bound_account=?`, Args: []any{id}},
		{SQL: `DELETE FROM accounts WHERE id=?`, Args: []any{id}},
	})
	return err
}

// --- phones ---

func (s *SQLite) ListPhones(ctx context.Context) ([]model.CloudPhone, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, external_id, region, image, state, account_id, ip
		FROM phones ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var out []model.CloudPhone
	for _, r := range rows.Values {
		out = append(out, scanPhone(r))
	}
	return out, nil
}

func scanPhone(r []any) model.CloudPhone {
	id, _ := r[0].(int64)
	acc, _ := r[6].(int64)
	str := func(i int) string {
		if r[i] == nil {
			return ""
		}
		v, _ := r[i].(string)
		return v
	}
	return model.CloudPhone{
		ID: id, Name: str(1), ExternalID: str(2), Region: str(3), Image: str(4),
		State: model.PhoneState(str(5)), AccountID: acc, IP: str(7),
	}
}

func (s *SQLite) GetPhone(ctx context.Context, id int64) (model.CloudPhone, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, external_id, region, image, state, account_id, ip
		FROM phones WHERE id = ?`, id)
	if err != nil {
		return model.CloudPhone{}, err
	}
	if len(rows.Values) == 0 {
		return model.CloudPhone{}, fmt.Errorf("phone %d: %w", id, ErrNotFound)
	}
	return scanPhone(rows.Values[0]), nil
}

func (s *SQLite) FindPhoneByExternalID(ctx context.Context, externalID string) (model.CloudPhone, bool, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, external_id, region, image, state, account_id, ip
		FROM phones WHERE external_id = ? LIMIT 1`, externalID)
	if err != nil {
		return model.CloudPhone{}, false, err
	}
	if len(rows.Values) == 0 {
		return model.CloudPhone{}, false, nil
	}
	return scanPhone(rows.Values[0]), true, nil
}

func (s *SQLite) CreatePhone(ctx context.Context, p *model.CloudPhone) error {
	res, err := s.db.Execute(ctx, `INSERT INTO phones
		(name, external_id, region, image, state, account_id, ip)
		VALUES (?,?,?,?,?,?,?)`,
		p.Name, p.ExternalID, p.Region, p.Image, string(p.State), p.AccountID, p.IP)
	if err != nil {
		return err
	}
	p.ID = res.LastInsertID
	return nil
}

// UpdatePhoneInfo refreshes only the provider-owned display fields; the
// account binding (account_id) is deliberately left untouched. The account
// display-name copy is updated in the same transaction so a provider rename
// does not leave a stale name in the account list.
func (s *SQLite) UpdatePhoneInfo(ctx context.Context, id int64, name, region, image string, state model.PhoneState, ip string) error {
	_, err := s.db.Transaction(ctx, []sqlite.Statement{
		{SQL: `UPDATE phones SET name=?, region=?, image=?, state=?, ip=? WHERE id=?`,
			Args: []any{name, region, image, string(state), ip, id}},
		{SQL: `UPDATE accounts SET bound_phone=?, updated_at=? WHERE bound_phone_id=?`,
			Args: []any{name, time.Now().Format(time.RFC3339), id}},
	})
	return err
}

func (s *SQLite) UpdatePhone(ctx context.Context, p model.CloudPhone) error {
	_, err := s.db.Execute(ctx, `UPDATE phones SET name=?, external_id=?, region=?, image=?,
		state=?, account_id=?, ip=? WHERE id=?`,
		p.Name, p.ExternalID, p.Region, p.Image, string(p.State), p.AccountID, p.IP, p.ID)
	return err
}

func (s *SQLite) UpdatePhoneState(ctx context.Context, id int64, state model.PhoneState) error {
	_, err := s.db.Execute(ctx, `UPDATE phones SET state=? WHERE id=?`, string(state), id)
	return err
}

func (s *SQLite) SetPhoneAccount(ctx context.Context, phoneID, accountID int64) error {
	_, err := s.db.Execute(ctx, `UPDATE phones SET account_id=? WHERE id=?`, accountID, phoneID)
	return err
}

// --- sms ---

func (s *SQLite) ListSms(ctx context.Context) ([]model.Sms, error) {
	rows, err := s.db.Query(ctx, `SELECT id, phone, code, content, received_at, used
		FROM sms ORDER BY received_at DESC`)
	if err != nil {
		return nil, err
	}
	var out []model.Sms
	for _, r := range rows.Values {
		id, _ := r[0].(int64)
		str := func(i int) string {
			if r[i] == nil {
				return ""
			}
			v, _ := r[i].(string)
			return v
		}
		used := false
		if v, ok := r[5].(int64); ok {
			used = v != 0
		}
		m := model.Sms{ID: id, Phone: str(1), Code: str(2), Content: str(3), Used: used}
		if ts, err := time.Parse(time.RFC3339, str(4)); err == nil {
			m.ReceivedAt = ts
		}
		out = append(out, m)
	}
	return out, nil
}

// MarkSmsUsed flags the newest message for phone carrying code as consumed,
// so a code collision cannot spend another number's message.
func (s *SQLite) MarkSmsUsed(ctx context.Context, phone, code string) error {
	_, err := s.db.Execute(ctx, `UPDATE sms SET used = 1 WHERE id = (
		SELECT id FROM sms WHERE code = ? AND (? = '' OR phone = ?)
		ORDER BY id DESC LIMIT 1)`, code, phone, phone)
	return err
}

func (s *SQLite) AddSms(ctx context.Context, m *model.Sms) error {
	if m.ReceivedAt.IsZero() {
		m.ReceivedAt = time.Now()
	}
	res, err := s.db.Execute(ctx, `INSERT INTO sms (phone, code, content, received_at, used)
		VALUES (?,?,?,?,?)`, m.Phone, m.Code, m.Content, m.ReceivedAt.Format(time.RFC3339), m.Used)
	if err != nil {
		return err
	}
	m.ID = res.LastInsertID
	return nil
}

// --- sims ---

func (s *SQLite) ListSims(ctx context.Context) ([]model.SimSlot, error) {
	rows, err := s.db.Query(ctx, `SELECT id, phone, carrier, bound_account, status FROM sims ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var out []model.SimSlot
	for _, r := range rows.Values {
		id, _ := r[0].(int64)
		acc, _ := r[3].(int64)
		str := func(i int) string {
			if r[i] == nil {
				return ""
			}
			v, _ := r[i].(string)
			return v
		}
		out = append(out, model.SimSlot{ID: id, Phone: str(1), Carrier: str(2), BoundAccount: acc, Status: str(4)})
	}
	return out, nil
}

// --- users ---

func (s *SQLite) ListUsers(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.Query(ctx, `SELECT id, username, nickname, role, enabled FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var out []model.User
	for _, r := range rows.Values {
		id, _ := r[0].(int64)
		str := func(i int) string {
			if r[i] == nil {
				return ""
			}
			v, _ := r[i].(string)
			return v
		}
		en := false
		if v, ok := r[4].(int64); ok {
			en = v != 0
		}
		out = append(out, model.User{ID: id, Username: str(1), Nickname: str(2), Role: str(3), Enabled: en})
	}
	return out, nil
}

func (s *SQLite) UpdateUser(ctx context.Context, u model.User) error {
	_, err := s.db.Execute(ctx, `UPDATE users SET username=?, nickname=?, role=?, enabled=? WHERE id=?`,
		u.Username, u.Nickname, u.Role, u.Enabled, u.ID)
	return err
}

// --- wallet ---

func (s *SQLite) Wallet(ctx context.Context) (model.Wallet, error) {
	rows, err := s.db.Query(ctx, `SELECT balance FROM wallet WHERE id = 1`)
	if err != nil {
		return model.Wallet{}, err
	}
	if len(rows.Values) == 0 {
		return model.Wallet{}, nil
	}
	bal, _ := rows.Values[0][0].(float64)
	return model.Wallet{Balance: bal}, nil
}

func (s *SQLite) ListLedger(ctx context.Context, limit int) ([]model.LedgerEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `SELECT id, at, amount, reason FROM ledger
		ORDER BY id DESC LIMIT ?`, int64(limit))
	if err != nil {
		return nil, err
	}
	var out []model.LedgerEntry
	for _, r := range rows.Values {
		id, _ := r[0].(int64)
		amt, _ := r[2].(float64)
		str := func(i int) string {
			if r[i] == nil {
				return ""
			}
			v, _ := r[i].(string)
			return v
		}
		e := model.LedgerEntry{ID: id, Amount: amt, Reason: str(3)}
		if ts, err := time.Parse(time.RFC3339, str(1)); err == nil {
			e.At = ts
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *SQLite) AddLedger(ctx context.Context, e *model.LedgerEntry) error {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	res, err := s.db.Execute(ctx, `INSERT INTO ledger (at, amount, reason) VALUES (?,?,?)`,
		e.At.Format(time.RFC3339), e.Amount, e.Reason)
	if err != nil {
		return err
	}
	e.ID = res.LastInsertID
	return nil
}

// SetBalance writes the wallet's balance, creating the row if needed.
func (s *SQLite) SetBalance(ctx context.Context, v float64) error {
	_, err := s.db.Execute(ctx, `INSERT INTO wallet (id, balance) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET balance = excluded.balance`, v)
	return err
}

// ApplyWalletChange records one ledger entry and the wallet balance derived
// from it atomically. The caller computes newBalance under its wallet lock.
func (s *SQLite) ApplyWalletChange(ctx context.Context, newBalance float64, e *model.LedgerEntry) error {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	_, err := s.db.Transaction(ctx, []sqlite.Statement{
		{SQL: `INSERT INTO ledger (at, amount, reason) VALUES (?,?,?)`,
			Args: []any{e.At.Format(time.RFC3339), e.Amount, e.Reason}},
		{SQL: `INSERT INTO wallet (id, balance) VALUES (1, ?)
			ON CONFLICT(id) DO UPDATE SET balance = excluded.balance`, Args: []any{newBalance}},
	})
	return err
}
