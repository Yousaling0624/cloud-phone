package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud-phone/internal/model"
)

// Accounts lists accounts in creation order.
func (s *Service) Accounts(ctx context.Context) ([]model.Account, error) {
	return s.repo.ListAccounts(ctx)
}

// ImportAccount adds one account and optionally binds it to a cloud phone.
// Duplicates are rejected. The account is created first and removed again if
// the binding is refused, so a failed import leaves no half-made row.
func (s *Service) ImportAccount(ctx context.Context, remark, number, display string, phoneID int64) error {
	remark = strings.TrimSpace(remark)
	number = strings.TrimSpace(number)
	display = strings.TrimSpace(display)
	if number == "" {
		return ErrEmptyNumber
	}

	s.accountMu.Lock()
	defer s.accountMu.Unlock()

	if _, exists, err := s.repo.FindAccountByNumber(ctx, number); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("账号 %s: %w", number, ErrDuplicate)
	}

	s.bindMu.Lock()
	target, err := s.resolveBindingLocked(ctx, 0, phoneID)
	if err != nil {
		s.bindMu.Unlock()
		return err
	}
	acc := model.Account{
		Remark: remark, Number: number, Display: display,
		Status: model.AccountOffline,
	}
	if err := s.repo.CreateAccount(ctx, &acc); err != nil {
		s.bindMu.Unlock()
		return err
	}
	if err := s.repo.SetAccountBinding(ctx, acc.ID, target.phoneID, target.phoneName); err != nil {
		// Compensation: do not leave an account behind when its requested
		// phone could not be bound.
		_ = s.repo.DeleteAccount(ctx, acc.ID)
		s.bindMu.Unlock()
		return err
	}
	s.bindMu.Unlock()

	s.notify(model.Change{Kind: "accounts", AccountID: acc.ID})
	if target.phoneID != 0 {
		s.notify(model.Change{Kind: "phones"})
	}
	return nil
}

// DeleteAccount removes an account, cancels its running login, and clears
// the phone and SIM references in the store's transaction.
func (s *Service) DeleteAccount(ctx context.Context, id int64) error {
	if _, err := s.repo.GetAccount(ctx, id); err != nil {
		return wrapNotFound("账号", id, err)
	}
	s.cancelLogin(id)
	if err := s.repo.DeleteAccount(ctx, id); err != nil {
		return err
	}
	s.notify(model.Change{Kind: "accounts", AccountID: id})
	s.notify(model.Change{Kind: "phones"})
	s.notify(model.Change{Kind: "sims"})
	return nil
}

// DeleteAccounts removes every id it can and reports the ones it could not.
func (s *Service) DeleteAccounts(ctx context.Context, ids []int64) error {
	var errs []error
	removed := 0
	for _, id := range ids {
		s.cancelLogin(id)
		if _, err := s.repo.GetAccount(ctx, id); err != nil {
			errs = append(errs, wrapNotFound("账号", id, err))
			continue
		}
		if err := s.repo.DeleteAccount(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("账号 %d: %w", id, err))
			continue
		}
		removed++
	}
	if removed > 0 {
		s.notify(model.Change{Kind: "accounts"})
		s.notify(model.Change{Kind: "phones"})
		s.notify(model.Change{Kind: "sims"})
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("部分账号删除失败: %w", errors.Join(errs...))
}

// ImportCSV adds "备注,账号" lines (the remark is optional). Blank lines and
// "#" comments are skipped, and duplicates inside the file or already in the
// database are ignored silently.
func (s *Service) ImportCSV(ctx context.Context, text string) (model.ImportResult, error) {
	res := model.ImportResult{Errors: []string{}}
	text = strings.TrimPrefix(text, string([]byte{0xEF, 0xBB, 0xBF}))

	s.accountMu.Lock()
	defer s.accountMu.Unlock()

	seen := map[string]bool{}
	for i, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		remark, number := "", line
		if parts := strings.SplitN(line, ",", 2); len(parts) == 2 {
			remark = strings.TrimSpace(parts[0])
			number = strings.TrimSpace(parts[1])
		}
		if number == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行: 账号为空", i+1))
			continue
		}
		if seen[number] {
			continue
		}
		seen[number] = true
		if _, exists, err := s.repo.FindAccountByNumber(ctx, number); err != nil {
			return res, err
		} else if exists {
			continue
		}
		acc := model.Account{Remark: remark, Number: number, Status: model.AccountOffline}
		if err := s.repo.CreateAccount(ctx, &acc); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行: %v", i+1, err))
			continue
		}
		res.Added++
	}
	if res.Added > 0 {
		s.notify(model.Change{Kind: "accounts"})
	}
	return res, nil
}
