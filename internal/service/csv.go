package service

import (
	"bytes"
	"context"
	"encoding/csv"

	"cloud-phone/internal/model"
)

// AccountsCSV renders the selected accounts (all when ids is empty) as UTF-8
// CSV. It returns the bytes and how many rows were written; the transport
// decides where to save them.
func (s *Service) AccountsCSV(ctx context.Context, ids []int64) ([]byte, int, error) {
	accounts, err := s.repo.ListAccounts(ctx)
	if err != nil {
		return nil, 0, err
	}
	selected := accounts
	if len(ids) > 0 {
		wanted := make(map[int64]bool, len(ids))
		for _, id := range ids {
			wanted[id] = true
		}
		selected = make([]model.Account, 0, len(ids))
		for _, acc := range accounts {
			if wanted[acc.ID] {
				selected = append(selected, acc)
			}
		}
	}
	return buildAccountsCSV(selected), len(selected), nil
}

// buildAccountsCSV writes a BOM so Windows Excel detects UTF-8, and CRLF
// line endings for the same reason.
func buildAccountsCSV(accounts []model.Account) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	_ = w.Write([]string{"备注", "账号", "昵称", "绑定云机", "状态", "登录状态", "更新时间"})
	for _, acc := range accounts {
		updated := ""
		if !acc.UpdatedAt.IsZero() {
			updated = acc.UpdatedAt.Format("2006-01-02 15:04:05")
		}
		_ = w.Write([]string{
			acc.Remark, acc.Number, acc.Display, acc.BoundPhone,
			AccountStatusLabel(acc.Status), LoginStateLabel(acc.Login), updated,
		})
	}
	w.Flush()
	return buf.Bytes()
}

// AccountStatusLabel is the Chinese label of an account status, used by the
// CSV export and the page.
func AccountStatusLabel(s model.AccountStatus) string {
	switch s {
	case model.AccountOnline:
		return "在线"
	case model.AccountLoggingIn:
		return "登扣中"
	case model.AccountAbnormal:
		return "异常"
	case model.AccountOffline:
		return "离线"
	}
	return string(s)
}

// LoginStateLabel is the Chinese label of the finer login state.
func LoginStateLabel(s model.LoginState) string {
	switch s {
	case model.LoginWaiting:
		return "等待验证码"
	case model.LoginReady:
		return "待确认"
	case model.LoginDone:
		return "登录成功"
	case model.LoginFail:
		return "登录失败"
	}
	return "未登录"
}
