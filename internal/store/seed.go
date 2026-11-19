package store

import (
	"context"
	"time"

	"cloud-phone/internal/model"
)

// Seed fills the database with the demo rows shown in the screenshot the
// first time it runs (when there are no accounts yet), so the platform
// opens with something to look at.
func (s *SQLite) Seed(ctx context.Context) error {
	accounts, err := s.ListAccounts(ctx)
	if err != nil {
		return err
	}
	if len(accounts) > 0 {
		return nil // already seeded
	}

	// Accounts matching the screenshot: account 2441864785 nicknamed 祈,
	// bound to cloud phone 24418647, plus a handful for the table to show.
	demo := []model.Account{
		{Remark: "点击编辑", Number: "2441864785", Display: "祈", BoundPhone: "24418647",
			Status: model.AccountAbnormal, Login: model.LoginNone,
			LoginRule: "qq_account_management_login_rule", VerifyPhone: "135******81",
			LinkedQQ: true, LinkedQID: "已开启", HasPassword: true},
		{Remark: "游戏小号A", Number: "18812340001", Display: "阿杰", BoundPhone: "24418648",
			Status: model.AccountOnline, Login: model.LoginDone, LinkedQQ: true,
			LinkedQID: "已开启", HasPassword: true},
		{Remark: "备用", Number: "18812340002", Display: "小美", Status: model.AccountOffline},
		{Remark: "直播号", Number: "18812340003", Display: "老王", BoundPhone: "24418649",
			Status: model.AccountOnline, Login: model.LoginDone, LinkedQQ: true, HasPassword: true},
		{Remark: "", Number: "18812340004", Display: "新号", Status: model.AccountOffline},
	}
	for i := range demo {
		if err := s.CreateAccount(ctx, &demo[i]); err != nil {
			return err
		}
	}

	// Cloud phones.
	phones := []model.CloudPhone{
		{Name: "24418647", ExternalID: "demo-24418647", Region: "华东-1", Image: "Android 12", State: model.PhoneIdle, AccountID: demo[0].ID, IP: "10.0.0.11"},
		{Name: "24418648", ExternalID: "demo-24418648", Region: "华东-1", Image: "Android 12", State: model.PhoneRunning, AccountID: demo[1].ID, IP: "10.0.0.12"},
		{Name: "24418649", ExternalID: "demo-24418649", Region: "华南-2", Image: "Android 13", State: model.PhoneRunning, AccountID: demo[3].ID, IP: "10.0.0.13"},
		{Name: "24418650", ExternalID: "demo-24418650", Region: "华南-2", Image: "Android 13", State: model.PhoneIdle, IP: "10.0.0.14"},
		{Name: "24418651", ExternalID: "demo-24418651", Region: "华北-1", Image: "Android 14", State: model.PhoneStopped, IP: "10.0.0.15"},
	}
	for _, p := range phones {
		if _, err := s.db.Execute(ctx, `INSERT INTO phones (name, external_id, region, image, state, account_id, ip)
			VALUES (?,?,?,?,?,?,?)`, p.Name, p.ExternalID, p.Region, p.Image, string(p.State), p.AccountID, p.IP); err != nil {
			return err
		}
	}
	// The accounts were inserted before their phones; fill the reverse
	// binding key now that the phone rows exist.
	if _, err := s.db.Execute(ctx, `UPDATE accounts SET bound_phone_id = COALESCE(
		(SELECT id FROM phones WHERE phones.name = accounts.bound_phone LIMIT 1), 0)
		WHERE bound_phone <> ''`); err != nil {
		return err
	}

	// SMS.
	sms := []model.Sms{
		{Phone: "135******81", Code: "482913", Content: "登录验证码：482913", ReceivedAt: time.Now().Add(-3 * time.Minute)},
		{Phone: "135******82", Code: "770156", Content: "登录验证码：770156", ReceivedAt: time.Now().Add(-20 * time.Minute), Used: true},
	}
	for i := range sms {
		if err := s.AddSms(ctx, &sms[i]); err != nil {
			return err
		}
	}

	// SIM slots.
	sims := []struct {
		phone, carrier string
		acc            int64
		status         string
	}{
		{"135******81", "移动", demo[0].ID, "占用"},
		{"135******82", "联通", demo[1].ID, "占用"},
		{"135******83", "电信", 0, "空闲"},
		{"135******84", "移动", 0, "空闲"},
		{"135******85", "联通", 0, "停机"},
	}
	for _, m := range sims {
		if _, err := s.db.Execute(ctx, `INSERT INTO sims (phone, carrier, bound_account, status)
			VALUES (?,?,?,?)`, m.phone, m.carrier, m.acc, m.status); err != nil {
			return err
		}
	}

	// Users.
	users := []model.User{
		{Username: "admin", Nickname: "喜相逢", Role: "管理员", Enabled: true},
		{Username: "ops01", Nickname: "运营小张", Role: "运营", Enabled: true},
		{Username: "guest", Nickname: "访客", Role: "只读", Enabled: false},
	}
	for _, u := range users {
		if _, err := s.db.Execute(ctx, `INSERT INTO users (username, nickname, role, enabled)
			VALUES (?,?,?,?)`, u.Username, u.Nickname, u.Role, u.Enabled); err != nil {
			return err
		}
	}

	// Wallet: the screenshot shows 云币余额 56.40.
	if err := s.SetBalance(ctx, 56.40); err != nil {
		return err
	}
	if err := s.AddLedger(ctx, &model.LedgerEntry{Amount: 100, Reason: "首次充值"}); err != nil {
		return err
	}
	if err := s.AddLedger(ctx, &model.LedgerEntry{Amount: -43.60, Reason: "云机开机扣费"}); err != nil {
		return err
	}

	return nil
}
